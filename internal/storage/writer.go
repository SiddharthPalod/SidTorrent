package storage

import (
	"fmt"
	"os"

	"github.com/SiddharthPalod/SidTorrent/internal/disk"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

type Writer struct {
	file       *os.File
	tf         *torrent.TorrentFile
	diskWriter *disk.DiskWriter
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

	diskWriter := disk.NewDiskWriter(tf, file)

	return &Writer{file: file, tf: tf, diskWriter: diskWriter}, nil

}

func (w *Writer) WritePiece(pieceIndex int, data []byte) error {
	expected := w.tf.PieceLengthAt(pieceIndex)
	if int64(len(data)) != expected {
		return fmt.Errorf("invalid piece size: got=%d expected=%d", len(data), expected)
	}
	return w.diskWriter.WritePiece(pieceIndex, data)
}

func (w *Writer) Close() error {
	err := w.diskWriter.Close()
	fileErr := w.file.Close()
	if err != nil {
		return err
	}
	return fileErr
}
