package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

func TestStorageWriterTruncatesToTorrentLength(t *testing.T) {
	tf := &torrent.TorrentFile{
		Length:      10,
		PieceLength: 4,
	}
	path := filepath.Join(t.TempDir(), "out.bin")

	w, err := storage.NewWriter(tf, path)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Size() != 10 {
		t.Fatalf("output size = %d, want 10", info.Size())
	}
}

func TestStorageWriterWritesPieceAtCorrectOffset(t *testing.T) {
	tf := &torrent.TorrentFile{
		Length:      10,
		PieceLength: 4,
	}
	path := filepath.Join(t.TempDir(), "out.bin")
	w, err := storage.NewWriter(tf, path)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}

	if err := w.WritePiece(1, []byte("abcd")); err != nil {
		w.Close()
		t.Fatalf("WritePiece() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want := []byte{0, 0, 0, 0, 'a', 'b', 'c', 'd', 0, 0}
	if !bytes.Equal(data, want) {
		t.Fatalf("file data = %v, want %v", data, want)
	}
}

func TestStorageWriterAcceptsLastPieceSize(t *testing.T) {
	tf := &torrent.TorrentFile{
		Length:      10,
		PieceLength: 4,
	}
	path := filepath.Join(t.TempDir(), "out.bin")
	w, err := storage.NewWriter(tf, path)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	defer w.Close()

	if err := w.WritePiece(2, []byte("xy")); err != nil {
		t.Fatalf("WritePiece() error = %v", err)
	}
}

func TestStorageWriterRejectsWrongPieceSize(t *testing.T) {
	tf := &torrent.TorrentFile{
		Length:      10,
		PieceLength: 4,
	}
	path := filepath.Join(t.TempDir(), "out.bin")
	w, err := storage.NewWriter(tf, path)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	defer w.Close()

	if err := w.WritePiece(2, []byte("toolong")); err == nil {
		t.Fatal("WritePiece() succeeded with wrong piece size, want error")
	}
}
