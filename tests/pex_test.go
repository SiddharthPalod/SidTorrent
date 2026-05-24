package tests

import (
	"testing"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
)

func TestPexExtensionBencodeHandshake(t *testing.T) {
	// Verify that handshakes correctly indicate extension support by setting the 20th reserved bit.
	var infoHash [20]byte
	var peerID [20]byte
	h := peer.NewHandshake(infoHash, peerID)

	serialized := h.Serialize()

	// Protocol string "BitTorrent protocol" length is 19.
	// Reserved bytes block starts at index 20. Byte 5 is at index 25.
	if serialized[25]&0x10 == 0 {
		t.Fatalf("extension protocol reserved bit was not set correctly in handshake payload")
	}

	parsed, err := peer.ReadHandshake(serialized)
	if err != nil {
		t.Fatalf("failed to parse handshake payload: %v", err)
	}
	if !parsed.SupportsExtensions {
		t.Fatalf("parsed handshake supportsExtensions flag was false, expected true")
	}
}
