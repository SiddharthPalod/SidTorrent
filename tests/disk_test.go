package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/disk"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

func TestDiskWriterAsyncAndSequential(t *testing.T) {
	tf := &torrent.TorrentFile{
		Length:      16,
		PieceLength: 4,
	}
	path := filepath.Join(t.TempDir(), "out.bin")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	err = file.Truncate(tf.Length)
	if err != nil {
		t.Fatalf("failed to truncate: %v", err)
	}

	dw := disk.NewDiskWriter(tf, file)

	// Write pieces out of order: 3, 1, 2, 0
	dw.WritePiece(3, []byte("dddd"))
	dw.WritePiece(1, []byte("bbbb"))
	dw.WritePiece(2, []byte("cccc"))
	dw.WritePiece(0, []byte("aaaa"))

	// Close forces a flush and waits for background loop to finish
	err = dw.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	file.Close()

	// Verify pieces are correctly written in order at their corresponding offsets
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	expected := []byte("aaaabbbbccccdddd")
	if !bytes.Equal(data, expected) {
		t.Fatalf("file content mismatch: got %q, want %q", string(data), string(expected))
	}
}

func TestDiskWriterCacheThresholdFlush(t *testing.T) {
	tf := &torrent.TorrentFile{
		Length:      20,
		PieceLength: 5,
	}
	path := filepath.Join(t.TempDir(), "out2.bin")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	_ = file.Truncate(tf.Length)

	dw := disk.NewDiskWriter(tf, file)
	dw.SetMaxCacheSize(8) // Flush threshold at 8 bytes

	// Write first piece (5 bytes) - shouldn't trigger flush immediately
	err = dw.WritePiece(0, []byte("12345"))
	if err != nil {
		t.Fatalf("WritePiece failed: %v", err)
	}

	// Verify file is empty/unwritten yet
	data, _ := os.ReadFile(path)
	if len(data) > 0 && bytes.Count(data, []byte{0}) != len(data) {
		t.Fatalf("file unexpectedly contained data before flush triggered")
	}

	// Write second piece (5 bytes) - total is now 10 bytes, exceeding 8 bytes.
	// This should trigger the flush.
	err = dw.WritePiece(1, []byte("67890"))
	if err != nil {
		t.Fatalf("WritePiece failed: %v", err)
	}

	// Let background writer flush
	time.Sleep(100 * time.Millisecond)

	// Close the writer
	_ = dw.Close()
	file.Close()

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	expected := []byte("1234567890\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00")
	if !bytes.Equal(data, expected) {
		t.Fatalf("file content mismatch: got %q, want %q", string(data), string(expected))
	}
}

func TestDiskWriterErrorHandling(t *testing.T) {
	tf := &torrent.TorrentFile{
		Length:      10,
		PieceLength: 5,
	}
	path := filepath.Join(t.TempDir(), "out3.bin")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}

	dw := disk.NewDiskWriter(tf, file)

	// Close the underlying file immediately. This forces the background
	// flusher's write operation to fail deterministically with a closed file error.
	file.Close()

	// Attempt writing to the closed file
	_ = dw.WritePiece(0, []byte("12345"))

	// Wait a bit for the background write to fail
	time.Sleep(100 * time.Millisecond)

	// Any subsequent write or Close should return the error!
	err = dw.WritePiece(1, []byte("abcde"))
	if err == nil {
		err = dw.Close()
	}
	if err == nil {
		t.Fatalf("expected write error, got nil")
	}
}
