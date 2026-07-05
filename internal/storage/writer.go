package storage

import (
	"fmt"

	"github.com/SiddharthPalod/SidTorrent/internal/disk"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

type Writer struct {
	store      Storage
	tf         *torrent.TorrentFile
	diskWriter *disk.DiskWriter
}

func NewWriter(
	tf *torrent.TorrentFile,
	outputPath string,
) (*Writer, error) {

	store, err := NewTorrentStorage(tf, outputPath)
	if err != nil {
		return nil, err
	}

	diskWriter := disk.NewDiskWriter(tf, store)

	return &Writer{store: store, tf: tf, diskWriter: diskWriter}, nil

}

func (w *Writer) WritePiece(pieceIndex int, data []byte) error {
	expected := w.tf.PieceLengthAt(pieceIndex)
	if int64(len(data)) != expected {
		return fmt.Errorf("invalid piece size: got=%d expected=%d", len(data), expected)
	}
	return w.diskWriter.WritePiece(pieceIndex, data)
}

func (w *Writer) ReadAt(p []byte, off int64) (n int, err error) {
	return w.store.ReadAt(p, off)
}

func (w *Writer) Close() error {
	err := w.diskWriter.Close()
	storeErr := w.store.Close()
	if err != nil {
		return err
	}
	return storeErr
}
