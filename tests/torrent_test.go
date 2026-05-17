package tests

import (
	"bytes"
	"crypto/sha1"
	"os"
	"path/filepath"
	"testing"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

func TestOpenUbuntuTorrent(t *testing.T) {
	tf, err := torrent.Open(filepath.Join("..", "testdata", "ubuntu.torrent"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if tf.Announce != "https://torrent.ubuntu.com/announce" {
		t.Fatalf("Announce = %q, want Ubuntu tracker", tf.Announce)
	}
	if len(tf.Trackers) != 2 {
		t.Fatalf("len(Trackers) = %d, want 2", len(tf.Trackers))
	}
	if len(tf.Trackers[0]) != 1 {
		t.Fatalf("len(Trackers[0]) = %d, want 1", len(tf.Trackers[0]))
	}
	if got, want := tf.Trackers[0][0], "https://torrent.ubuntu.com/announce"; got != want {
		t.Fatalf("Trackers[0][0] = %q, want %q", got, want)
	}
	if len(tf.Trackers[1]) != 1 {
		t.Fatalf("len(Trackers[1]) = %d, want 1", len(tf.Trackers[1]))
	}
	if got, want := tf.Trackers[1][0], "https://ipv6.torrent.ubuntu.com/announce"; got != want {
		t.Fatalf("Trackers[1][0] = %q, want %q", got, want)
	}
	if tf.Name != "ubuntu-26.04-desktop-amd64.iso" {
		t.Fatalf("Name = %q, want ubuntu-26.04-desktop-amd64.iso", tf.Name)
	}
	if tf.Length != 6518974464 {
		t.Fatalf("Length = %d, want 6518974464", tf.Length)
	}
	if tf.PieceLength != 262144 {
		t.Fatalf("PieceLength = %d, want 262144", tf.PieceLength)
	}
	if len(tf.Pieces) != 497360 {
		t.Fatalf("len(Pieces) = %d, want 497360", len(tf.Pieces))
	}
	if len(tf.RawInfo) == 0 {
		t.Fatal("RawInfo is empty")
	}
	if got, want := tf.RawInfo[0], byte('d'); got != want {
		t.Fatalf("RawInfo starts with %q, want %q", got, want)
	}
	if got, want := tf.RawInfo[len(tf.RawInfo)-1], byte('e'); got != want {
		t.Fatalf("RawInfo ends with %q, want %q", got, want)
	}
	if tf.InfoHash == [20]byte{} {
		t.Fatal("InfoHash is zero")
	}
	if got, want := tf.InfoHash, sha1.Sum(tf.RawInfo); got != want {
		t.Fatalf("InfoHash = %x, want sha1(raw info) %x", got, want)
	}
}

func TestOpenUsesOriginalRawInfoForHash(t *testing.T) {
	path := filepath.Join("..", "testdata", "ubuntu.torrent")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	rootNode, err := bencode.DecodeWithRaw(data)
	if err != nil {
		t.Fatalf("DecodeWithRaw() error = %v", err)
	}
	infoNode := rootNode.Dict["info"]
	wantRaw := data[infoNode.Start:infoNode.End]

	tf, err := torrent.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if !bytes.Equal(tf.RawInfo, wantRaw) {
		t.Fatal("RawInfo does not match original info dictionary bytes")
	}
}

func TestPieceLengthAtHandlesLastPiece(t *testing.T) {
	tests := []struct {
		name          string
		totalLength   int64
		pieceLength   int64
		pieceIndex    int
		wantPieceSize int64
	}{
		{name: "full first piece", totalLength: 10, pieceLength: 4, pieceIndex: 0, wantPieceSize: 4},
		{name: "full middle piece", totalLength: 10, pieceLength: 4, pieceIndex: 1, wantPieceSize: 4},
		{name: "short last piece", totalLength: 10, pieceLength: 4, pieceIndex: 2, wantPieceSize: 2},
		{name: "exact final piece", totalLength: 8, pieceLength: 4, pieceIndex: 1, wantPieceSize: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := torrent.PieceLengthAt(tt.totalLength, tt.pieceLength, tt.pieceIndex)
			if err != nil {
				t.Fatalf("PieceLengthAt() error = %v", err)
			}
			if got != tt.wantPieceSize {
				t.Fatalf("PieceLengthAt() = %d, want %d", got, tt.wantPieceSize)
			}
		})
	}
}

func TestPieceLengthAtRejectsOutOfRangePiece(t *testing.T) {
	if _, err := torrent.PieceLengthAt(10, 4, 3); err == nil {
		t.Fatal("PieceLengthAt() succeeded for out-of-range piece, want error")
	}
}
