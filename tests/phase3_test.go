package tests

import (
	"net"
	"testing"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/piece"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/util"
)

func TestRarestFirstPieceSelection(t *testing.T) {
	tf := &torrent.TorrentFile{
		Pieces:      make([]byte, 5*20), // 5 pieces total
		Length:      5 * 16384,
		PieceLength: 16384,
	}
	pm := piece.NewPieceManager(tf)

	// Artificially establish swarm availabilities:
	pm.Availability = []int{3, 1, 2, 4, 3} // Piece at index 1 is rarest (count = 1)

	available := []bool{true, true, true, true, true}
	pIndex, err := pm.NextRarestPiece(available)
	if err != nil {
		t.Fatalf("unexpected rarest piece search error: %v", err)
	}

	if pIndex != 1 {
		t.Fatalf("rarest piece selection got index %d, expected index 1", pIndex)
	}
}

func TestTokenBucketRateLimiterThrottling(t *testing.T) {
	rl := util.NewRateLimiter(5000) // 5KB/s rate cap
	start := time.Now()

	rl.Wait(10000) // Consume 10KB (needs ~1 second worth of tokens)

	elapsed := time.Since(start)
	if elapsed < 900*time.Millisecond {
		t.Fatalf("rate limiter did not throttle execution long enough, elapsed time: %v", elapsed)
	}
}

func TestChokeManagerTitForTatRanking(t *testing.T) {
	// Mock connections
	conn1, serverConn1 := net.Pipe()
	conn2, serverConn2 := net.Pipe()
	defer conn1.Close()
	defer serverConn1.Close()
	defer conn2.Close()
	defer serverConn2.Close()

	// Drain goroutines to prevent blocking on writes
	go func() {
		for {
			buf := make([]byte, 1024)
			_, err := serverConn1.Read(buf)
			if err != nil {
				return
			}
		}
	}()
	go func() {
		for {
			buf := make([]byte, 1024)
			_, err := serverConn2.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	c1 := &peer.Client{
		Conn:  conn1,
		State: peer.PeerState{InterestedInUs: true, ChokedByUs: true},
	}
	c2 := &peer.Client{
		Conn:  conn2,
		State: peer.PeerState{InterestedInUs: true, ChokedByUs: true},
	}

	// c1 is faster than c2:
	c1.State.IntervalBytes = 20000
	c1.State.LastActive = time.Now().Add(-1 * time.Second)

	c2.State.IntervalBytes = 5000
	c2.State.LastActive = time.Now().Add(-1 * time.Second)

	cm := piece.NewChokeManager([]*peer.Client{c1, c2}, 2) // 1 TFT slot + 1 optimistic slot
	cm.Evaluate()

	// Fastest peer (c1) should be unchoked by Tit-for-Tat slot
	if c1.State.ChokedByUs {
		t.Fatalf("fastest peer was incorrectly choked")
	}
}
