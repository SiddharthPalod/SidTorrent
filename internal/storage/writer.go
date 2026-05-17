package storage

import (
	"fmt"
	"os"
	"sync"

	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

type Writer struct {
	file *os.File
	tf   *torrent.TorrentFile
	mu   sync.Mutex
}

func NewWriter(
	tf *torrent.TorrentFile,
	outputPath string,
) (*Writer, error) {

	file, err := os.OpenFile(
		outputPath,
		os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	err = file.Truncate(tf.Length)
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	return &Writer{file: file, tf: tf}, nil

}

func (w *Writer) WritePiece(pieceIndex int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	offset := int64(pieceIndex) * w.tf.PieceLength
	expected := w.tf.PieceLengthAt(pieceIndex)

	if int64(len(data)) != expected {
		return fmt.Errorf(
			"invalid piece size: got=%d expected=%d",
			len(data),
			expected,
		)
	}

	_, err := w.file.WriteAt(data, offset)
	return err
}

func (w *Writer) Close() error {
	return w.file.Close()
}
