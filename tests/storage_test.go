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

func TestMultiFileStorageReadWrite(t *testing.T) {
	tf := &torrent.TorrentFile{
		Name:        "multi",
		Length:      15,
		PieceLength: 5,
		IsMultiFile: true,
		Files: []torrent.FileEntry{
			{Length: 5, Path: []string{"a.txt"}},
			{Length: 10, Path: []string{"sub", "b.txt"}},
		},
	}

	tempDir := t.TempDir()
	store, err := storage.NewTorrentStorage(tf, tempDir)
	if err != nil {
		t.Fatalf("failed to create TorrentStorage: %v", err)
	}
	defer store.Close()

	// Write 15 bytes crossing file boundaries
	data := []byte("12345abcdefghij")
	n, err := store.WriteAt(data, 0)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if n != 15 {
		t.Fatalf("expected to write 15 bytes, wrote %d", n)
	}

	// Verify file content directly
	aContent, err := os.ReadFile(filepath.Join(tempDir, "a.txt"))
	if err != nil {
		t.Fatalf("failed to read a.txt: %v", err)
	}
	if string(aContent) != "12345" {
		t.Fatalf("a.txt = %q, expected '12345'", string(aContent))
	}

	bContent, err := os.ReadFile(filepath.Join(tempDir, "sub", "b.txt"))
	if err != nil {
		t.Fatalf("failed to read sub/b.txt: %v", err)
	}
	if string(bContent) != "abcdefghij" {
		t.Fatalf("sub/b.txt = %q, expected 'abcdefghij'", string(bContent))
	}

	// Read back through storage
	readBuf := make([]byte, 15)
	rn, err := store.ReadAt(readBuf, 0)
	if err != nil {
		t.Fatalf("failed to read back: %v", err)
	}
	if rn != 15 {
		t.Fatalf("expected to read 15 bytes, read %d", rn)
	}
	if string(readBuf) != "12345abcdefghij" {
		t.Fatalf("read back = %q, expected '12345abcdefghij'", string(readBuf))
	}
}
