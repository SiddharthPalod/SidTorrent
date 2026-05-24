package tests

import (
	"testing"

	"github.com/SiddharthPalod/SidTorrent/internal/piece"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

func TestHybridStreamingPieceSelection(t *testing.T) {
	tf := &torrent.TorrentFile{
		Pieces:      make([]byte, 10*20), // 10 pieces total
		Length:      10 * 16384,
		PieceLength: 16384,
	}
	pm := piece.NewPieceManager(tf)

	// Enable streaming mode and set the sequential window to 4 pieces
	pm.StreamingMode = true
	pm.StreamingWindowSize = 4

	// Swarm piece availability:
	// Let's set index 5 to be extremely rare (count = 1) and index 1 to be highly available (count = 5)
	pm.Availability = []int{3, 5, 2, 4, 3, 1, 3, 3, 3, 3}

	// Case 1: Sequential downloading in the streaming window
	// Peers have all pieces available
	available := []bool{true, true, true, true, true, true, true, true, true, true}

	// We expect the first piece selected to be index 0 (lowest sequential index in window 0-3),
	// even though index 5 is the rarest in the swarm!
	pIndex, err := pm.NextPiece(available)
	if err != nil {
		t.Fatalf("unexpected piece selection error: %v", err)
	}
	if pIndex != 0 {
		t.Fatalf("expected sequential piece index 0 in streaming mode, got %d", pIndex)
	}

	// Case 2: Skip unavailable piece in sequential window
	// Mark index 1 as unavailable from this peer (available[1] = false)
	available = []bool{true, false, true, true, true, true, true, true, true, true}

	// The next expected piece in the window (0-3) is index 2, since index 0 is already in-progress and index 1 is unavailable.
	pIndex, err = pm.NextPiece(available)
	if err != nil {
		t.Fatalf("unexpected piece selection error: %v", err)
	}
	if pIndex != 2 {
		t.Fatalf("expected sequential piece index 2 (skipping unavailable index 1), got %d", pIndex)
	}

	// Case 3: Sequential window finished, fallback to rarest-first
	// Let's mark pieces 0, 2, 3 as completed, and piece 1 is in-progress (delete pending entries for them)
	delete(pm.Pending, 0)
	delete(pm.Pending, 1)
	delete(pm.Pending, 2)
	delete(pm.Pending, 3)

	// At this point, the streaming window (indices 0, 1, 2, 3) has no pending pieces.
	// The selector must fall back to Rarest-First for remaining pending pieces (indices 4-9).
	// Swarm counts: index 5 has count=1 (rarest), others have counts >= 3.
	available = []bool{true, true, true, true, true, true, true, true, true, true}

	pIndex, err = pm.NextPiece(available)
	if err != nil {
		t.Fatalf("unexpected piece selection error: %v", err)
	}
	if pIndex != 5 {
		t.Fatalf("expected rarest-first fallback to select rarest index 5, got %d", pIndex)
	}
}
