package piece

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/util"
)

func StartWorker(
	client *peer.Client,
	tf *torrent.TorrentFile,
	pm *PieceManager,
	writer *storage.Writer,
	rl *util.RateLimiter,
) error {
	defer client.Conn.Close()
	pm.RegisterPeerBitfield(client.State.Bitfield)
	defer pm.UnregisterPeerBitfield(client.State.Bitfield)
	fmt.Printf("[STAGE] StartWorker: starting worker goroutine for peer %s\n", client.Conn.RemoteAddr())

	for {
		pieceIndex, err := pm.NextPiece(client.State.Bitfield)
		if err != nil {
			fmt.Printf("[STAGE] StartWorker: no available pieces for peer %s to download\n", client.Conn.RemoteAddr())
			return nil
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
			pm.MarkFailed(pieceIndex)

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
			pm.MarkFailed(pieceIndex)

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
			pm.MarkFailed(pieceIndex)
			continue
		}

		fmt.Printf("[STAGE] StartWorker: piece %d successfully written to disk storage!\n", pieceIndex)

		pm.MarkComplete(pieceIndex, int64(len(data)))
		fmt.Printf(
			"[STAGE] StartWorker: piece %d completely processed!\n",
			pieceIndex,
		)
		PrintProgress(pm)
		if pm.IsComplete() {
			fmt.Println("[STAGE] StartWorker: torrent download completely finished!")
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
