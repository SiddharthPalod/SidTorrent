package piece

import (
	"bytes"
	"crypto/sha1"
	"fmt"

	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

func VerifyPiece(
	tf *torrent.TorrentFile,
	pieceIndex int,
	data []byte,
) error {

	expectedHash, err := PieceHash(tf, pieceIndex)
	if err != nil {
		return err
	}

	actualHash := sha1.Sum(data)
	if !bytes.Equal(expectedHash, actualHash[:]) {
		return fmt.Errorf("piece %d failed SHA1 verification", pieceIndex)
	}

	return nil
}

func PieceHash(
	tf *torrent.TorrentFile,
	pieceIndex int,
) ([]byte, error) {
	start := pieceIndex * 20
	end := start + 20
	if end > len(tf.Pieces) {
		return nil, fmt.Errorf(
			"invalid piece index: %d",
			pieceIndex,
		)
	}
	return tf.Pieces[start:end], nil
}
