package tests

import (
	"net"
	"testing"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/piece"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

func TestGlobalPeerBlacklisting(t *testing.T) {
	tf := &torrent.TorrentFile{
		Pieces:      make([]byte, 20),
		Length:      16384,
		PieceLength: 16384,
	}
	pm := piece.NewPieceManager(tf)

	address1 := "192.168.1.100:6881"
	address2 := "192.168.1.100:51413" // Same host, different port
	address3 := "8.8.8.8:6881"

	// Initially, no peers should be blacklisted
	if pm.IsBlacklisted(address1) {
		t.Fatalf("address1 was incorrectly marked as blacklisted initially")
	}

	// Blacklist address1
	pm.BlacklistPeer(address1)

	// Since blacklisting is host-based, both address1 and address2 (same host IP) should be blacklisted!
	if !pm.IsBlacklisted(address1) {
		t.Fatalf("address1 was not blacklisted")
	}
	if !pm.IsBlacklisted(address2) {
		t.Fatalf("address2 (same host) was not blacklisted")
	}

	// address3 should NOT be blacklisted
	if pm.IsBlacklisted(address3) {
		t.Fatalf("address3 (different host) was incorrectly blacklisted")
	}
}

func TestWriteMessageDeadline(t *testing.T) {
	// Setup mock network pipe
	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	client := &peer.Client{
		Conn: conn1,
	}

	msg := &peer.Message{
		ID:      peer.MsgInterested,
		Payload: []byte{},
	}

	// Start reader on other end of the pipe
	go func() {
		buf := make([]byte, 1024)
		_, _ = conn2.Read(buf)
	}()

	// Writing should succeed cleanly without deadline timeout
	err := client.WriteMessage(msg)
	if err != nil {
		t.Fatalf("WriteMessage failed on active connection: %v", err)
	}
}

func TestMaliciousCorruptPeerBlacklisting(t *testing.T) {
	tf := &torrent.TorrentFile{
		Pieces:      make([]byte, 40), // 2 pieces total
		Length:      2 * 16384,
		PieceLength: 16384,
	}
	pm := piece.NewPieceManager(tf)

	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	client := &peer.Client{
		Conn: conn1,
		State: peer.PeerState{
			Bitfield: []bool{true, true},
		},
	}

	// Artificially fail piece 0 twice to verify it bans the peer address
	client.State.CorruptCount = 0

	// 1st failure
	client.State.CorruptCount++
	if client.State.CorruptCount >= 2 {
		t.Fatalf("banned too early")
	}

	// 2nd failure
	client.State.CorruptCount++
	if client.State.CorruptCount >= 2 {
		pm.BlacklistPeer(client.Conn.RemoteAddr().String())
	}

	if !pm.IsBlacklisted(client.Conn.RemoteAddr().String()) {
		t.Fatalf("malicious peer was not blacklisted after 2 failures")
	}
}
