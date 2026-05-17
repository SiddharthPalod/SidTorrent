package piece

import (
	"fmt"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

func StartWorker(
	client *peer.Client,
	tf *torrent.TorrentFile,
	pm *PieceManager,
	writer *storage.Writer,
) error {
	defer client.Conn.Close()

	for {
		pieceIndex, err := pm.NextRandomPiece(client.State.Bitfield)
		if err != nil {
			fmt.Println("worker: no available pieces")
			return nil
		}

		pieceLength := int(tf.PieceLengthAt(pieceIndex))
		fmt.Printf("worker downloading piece %d\n", pieceIndex)

		data, err := DownloadPiece(client, pieceIndex, pieceLength)
		if err != nil {
			fmt.Printf(
				"piece %d failed: %v\n",
				pieceIndex,
				err,
			)
			pm.MarkFailed(pieceIndex)
			continue
		}

		err = VerifyPiece(tf, pieceIndex, data)
		if err != nil {
			fmt.Printf(
				"piece %d verification failed\n",
				pieceIndex,
			)
			pm.MarkFailed(pieceIndex)
			continue
		}

		if err := writer.WritePiece(pieceIndex, data); err != nil {
			fmt.Printf(
				"piece %d write failed: %v\n",
				pieceIndex,
				err,
			)
			pm.MarkFailed(pieceIndex)
			continue
		}

		pm.MarkComplete(pieceIndex, int64(len(data)))
		fmt.Printf(
			"piece %d completed\n",
			pieceIndex,
		)
		PrintProgress(pm)
		if pm.IsComplete() {
			fmt.Println("torrent download complete")
			return nil
		}
	}
}
