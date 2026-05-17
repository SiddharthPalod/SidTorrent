package torrent

import (
	"bytes"
	"crypto/sha1"
	"os"
	"path/filepath"
	"testing"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
)

func TestOpenUbuntuTorrent(t *testing.T) {
	tf, err := Open(filepath.Join("..", "..", "testdata", "ubuntu.torrent"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if tf.Announce != "https://torrent.ubuntu.com/announce" {
		t.Fatalf("Announce = %q, want Ubuntu tracker", tf.Announce)
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
	path := filepath.Join("..", "..", "testdata", "ubuntu.torrent")
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

	tf, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if !bytes.Equal(tf.RawInfo, wantRaw) {
		t.Fatal("RawInfo does not match original info dictionary bytes")
	}
}
