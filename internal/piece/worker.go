package piece

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/metrics"
	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/util"
)

func StartWorker(
	ctx context.Context,
	client *peer.Client,
	tf *torrent.TorrentFile,
	pm *PieceManager,
	writer *storage.Writer,
	rl *util.RateLimiter,
	completeChan chan struct{},
) error {
	defer client.Conn.Close()
	pm.RegisterPeerBitfield(client.State.Bitfield)
	defer pm.UnregisterPeerBitfield(client.State.Bitfield)
	fmt.Printf("[STAGE] StartWorker: starting worker goroutine for peer %s\n", client.Conn.RemoteAddr())


	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[STAGE] StartWorker: worker for %s exiting due to context cancellation\n", client.Conn.RemoteAddr())
			return ctx.Err()
		default:
		}

		pieceIndex, err := pm.NextPiece(client.State.Bitfield)
		if err != nil {
			pending, inProgress, _ := pm.Stats()
			if pending == 0 && inProgress == 0 {
				fmt.Printf("[STAGE] StartWorker: no available pieces for peer %s to download\n", client.Conn.RemoteAddr())
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
				continue
			}
		}

		pieceLength := int(tf.PieceLengthAt(pieceIndex))
		fmt.Printf("[STAGE] StartWorker: worker assigned piece %d (len %d bytes) from peer %s\n",
			pieceIndex, pieceLength, client.Conn.RemoteAddr())

		data, err := DownloadPiece(client, pieceIndex, pieceLength, rl)
		if err != nil {
			fmt.Printf(
				"[STAGE] StartWorker: piece %d failed download from peer %s: %v\n",
				pieceIndex,
				client.Conn.RemoteAddr(),
				err,
			)
			
			metrics.GlobalMetrics.IncFailedPieces()
			pm.ReturnToPending(pieceIndex)

			if isConnectionError(err) {
				fmt.Printf("[STAGE] StartWorker: worker exiting, connection to peer %s lost\n", client.Conn.RemoteAddr())
				return err
			}

			continue
		}

		fmt.Printf("[STAGE] StartWorker: piece %d successfully downloaded from peer %s\n", pieceIndex, client.Conn.RemoteAddr())

		err = VerifyPiece(tf, pieceIndex, data)
		if err != nil {
			fmt.Printf(
				"[STAGE] StartWorker: piece %d verification failed from peer %s!\n",
				pieceIndex,
				client.Conn.RemoteAddr(),
			)
			
			metrics.GlobalMetrics.IncFailedPieces()
			if !pm.MarkFailed(pieceIndex) {
				return fmt.Errorf("piece %d exceeded max retries (%d), download incomplete", pieceIndex, pm.MaxRetries)
			}

			client.State.CorruptCount++
			if client.State.CorruptCount >= 2 {
				pm.BlacklistPeer(client.Conn.RemoteAddr().String())
				return fmt.Errorf("malicious peer %s: consistently sent corrupt data", client.Conn.RemoteAddr())
			}
			continue
		}

		fmt.Printf("[STAGE] StartWorker: piece %d SHA-1 hash verification PASSED!\n", pieceIndex)

		if err := writer.WritePiece(pieceIndex, data); err != nil {
			fmt.Printf(
				"[STAGE] StartWorker: piece %d write failed: %v\n",
				pieceIndex,
				err,
			)
			metrics.GlobalMetrics.IncFailedPieces()
			if !pm.MarkFailed(pieceIndex) {
				return fmt.Errorf("piece %d exceeded max retries (%d), download incomplete", pieceIndex, pm.MaxRetries)
			}
			continue
		}

		fmt.Printf("[STAGE] StartWorker: piece %d successfully written to disk storage!\n", pieceIndex)

		pm.MarkComplete(pieceIndex, int64(len(data)))
		metrics.GlobalMetrics.IncSuccessPieces()
		metrics.GlobalMetrics.AddDownloaded(int64(len(data)))

		fmt.Printf(
			"[STAGE] StartWorker: piece %d completely processed!\n",
			pieceIndex,
		)
		PrintProgress(pm)
		if pm.IsComplete() {
			fmt.Println("[STAGE] StartWorker: torrent download completely finished!")
			select {
			case completeChan <- struct{}{}:
			default:
			}
			return nil
		}
	}
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	return false
}
