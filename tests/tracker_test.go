package tests

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/tracker"
)

func TestGetPeersUsesUDPTrackerProtocol(t *testing.T) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error = %v", err)
	}
	defer conn.Close()

	var infoHash [20]byte
	copy(infoHash[:], []byte("01234567890123456789"))

	serverErr := make(chan error, 1)
	go fakeUDPTracker(t, conn, infoHash, serverErr)

	tf := &torrent.TorrentFile{
		Announce: "udp://" + conn.LocalAddr().String() + "/announce",
		Length:   12345,
		InfoHash: infoHash,
	}
	peers, err := tracker.GetPeers(tf)
	if err != nil {
		t.Fatalf("GetPeers() error = %v", err)
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("fake tracker error = %v", err)
	}

	if len(peers) != 1 {
		t.Fatalf("len(peers) = %d, want 1", len(peers))
	}
	if got, want := peers[0].IP.String(), "1.2.3.4"; got != want {
		t.Fatalf("peer IP = %s, want %s", got, want)
	}
	if got, want := peers[0].Port, uint16(6881); got != want {
		t.Fatalf("peer port = %d, want %d", got, want)
	}
}

func TestGetPeersFallsBackAcrossAnnounceList(t *testing.T) {
	var firstHits int
	failingTracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstHits++
		_, _ = w.Write(bencode.Encode(map[string]interface{}{
			"failure reason": []byte("not today"),
		}))
	}))
	defer failingTracker.Close()

	var secondHits int
	workingTracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits++
		_, _ = w.Write(bencode.Encode(map[string]interface{}{
			"interval": int64(1800),
			"peers":    compactPeer(5, 6, 7, 8, 51413),
		}))
	}))
	defer workingTracker.Close()

	tf := &torrent.TorrentFile{
		Announce: failingTracker.URL,
		Trackers: [][]string{{failingTracker.URL}, {workingTracker.URL}},
		Length:   12345,
	}

	peers, err := tracker.GetPeers(tf)
	if err != nil {
		t.Fatalf("GetPeers() error = %v", err)
	}
	if firstHits != 1 {
		t.Fatalf("first tracker hits = %d, want 1", firstHits)
	}
	if secondHits != 1 {
		t.Fatalf("second tracker hits = %d, want 1", secondHits)
	}
	if len(peers) != 1 {
		t.Fatalf("len(peers) = %d, want 1", len(peers))
	}
	if got, want := peers[0].IP.String(), "5.6.7.8"; got != want {
		t.Fatalf("peer IP = %s, want %s", got, want)
	}
	if got, want := peers[0].Port, uint16(51413); got != want {
		t.Fatalf("peer port = %d, want %d", got, want)
	}
}

func TestGetPeersFallsBackAcrossMixedTrackerTier(t *testing.T) {
	failingTracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bencode.Encode(map[string]interface{}{
			"failure reason": []byte("http tracker unavailable"),
		}))
	}))
	defer failingTracker.Close()

	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error = %v", err)
	}
	defer conn.Close()

	var infoHash [20]byte
	copy(infoHash[:], []byte("abcdefghijklmnopqrst"))
	serverErr := make(chan error, 1)
	go fakeUDPTracker(t, conn, infoHash, serverErr)

	tf := &torrent.TorrentFile{
		Trackers: [][]string{{
			failingTracker.URL,
			"udp://" + conn.LocalAddr().String() + "/announce",
		}},
		Length:   12345,
		InfoHash: infoHash,
	}

	peers, err := tracker.GetPeers(tf)
	if err != nil {
		t.Fatalf("GetPeers() error = %v", err)
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("fake tracker error = %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("len(peers) = %d, want 1", len(peers))
	}
	if got, want := peers[0].IP.String(), "1.2.3.4"; got != want {
		t.Fatalf("peer IP = %s, want %s", got, want)
	}
}

func fakeUDPTracker(t *testing.T, conn net.PacketConn, infoHash [20]byte, errc chan<- error) {
	t.Helper()
	defer close(errc)

	buf := make([]byte, 1500)
	n, addr, err := conn.ReadFrom(buf)
	if err != nil {
		errc <- err
		return
	}
	if n != 16 {
		errc <- fmt.Errorf("connect request length = %d, want 16", n)
		return
	}
	if got, want := binary.BigEndian.Uint64(buf[0:8]), uint64(0x41727101980); got != want {
		errc <- fmt.Errorf("protocol ID = %x, want %x", got, want)
		return
	}
	if got, want := binary.BigEndian.Uint32(buf[8:12]), uint32(0); got != want {
		errc <- fmt.Errorf("connect action = %d, want %d", got, want)
		return
	}
	connectTxID := binary.BigEndian.Uint32(buf[12:16])
	connectionID := uint64(0x1122334455667788)

	connectResp := make([]byte, 16)
	binary.BigEndian.PutUint32(connectResp[0:4], 0)
	binary.BigEndian.PutUint32(connectResp[4:8], connectTxID)
	binary.BigEndian.PutUint64(connectResp[8:16], connectionID)
	if _, err := conn.WriteTo(connectResp, addr); err != nil {
		errc <- err
		return
	}

	n, addr, err = conn.ReadFrom(buf)
	if err != nil {
		errc <- err
		return
	}
	if n != 98 {
		errc <- fmt.Errorf("announce request length = %d, want 98", n)
		return
	}
	if got := binary.BigEndian.Uint64(buf[0:8]); got != connectionID {
		errc <- fmt.Errorf("connection ID = %x, want %x", got, connectionID)
		return
	}
	if got, want := binary.BigEndian.Uint32(buf[8:12]), uint32(1); got != want {
		errc <- fmt.Errorf("announce action = %d, want %d", got, want)
		return
	}
	announceTxID := binary.BigEndian.Uint32(buf[12:16])
	if !bytes.Equal(buf[16:36], infoHash[:]) {
		errc <- fmt.Errorf("announce info hash did not match")
		return
	}
	if got, want := binary.BigEndian.Uint64(buf[64:72]), uint64(12345); got != want {
		errc <- fmt.Errorf("left = %d, want %d", got, want)
		return
	}
	if got, want := int32(binary.BigEndian.Uint32(buf[92:96])), int32(50); got != want {
		errc <- fmt.Errorf("numwant = %d, want %d", got, want)
		return
	}
	if got, want := binary.BigEndian.Uint16(buf[96:98]), uint16(6881); got != want {
		errc <- fmt.Errorf("port = %d, want %d", got, want)
		return
	}

	announceResp := make([]byte, 26)
	binary.BigEndian.PutUint32(announceResp[0:4], 1)
	binary.BigEndian.PutUint32(announceResp[4:8], announceTxID)
	binary.BigEndian.PutUint32(announceResp[8:12], 1800)
	binary.BigEndian.PutUint32(announceResp[12:16], 1)
	binary.BigEndian.PutUint32(announceResp[16:20], 2)
	copy(announceResp[20:24], net.IPv4(1, 2, 3, 4).To4())
	binary.BigEndian.PutUint16(announceResp[24:26], 6881)
	if _, err := conn.WriteTo(announceResp, addr); err != nil {
		errc <- err
		return
	}
}

func compactPeer(a, b, c, d byte, port uint16) []byte {
	peer := []byte{a, b, c, d, 0, 0}
	binary.BigEndian.PutUint16(peer[4:6], port)
	return peer
}

func TestGetPeersHTTPTrackerIPv6(t *testing.T) {
	ipv6Addr := net.ParseIP("2001:db8::1")
	if ipv6Addr == nil {
		t.Fatalf("failed to parse test IPv6 address")
	}

	ipv6Compact := make([]byte, 18)
	copy(ipv6Compact[0:16], ipv6Addr)
	binary.BigEndian.PutUint16(ipv6Compact[16:18], 51413)

	workingTracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bencode.Encode(map[string]interface{}{
			"interval": int64(1800),
			"peers6":   ipv6Compact,
		}))
	}))
	defer workingTracker.Close()

	tf := &torrent.TorrentFile{
		Announce: workingTracker.URL,
		Length:   12345,
	}

	peers, err := tracker.GetPeers(tf)
	if err != nil {
		t.Fatalf("GetPeers() with IPv6 error = %v", err)
	}

	if len(peers) != 1 {
		t.Fatalf("len(peers) = %d, want 1", len(peers))
	}
	if got, want := peers[0].IP.String(), "2001:db8::1"; got != want {
		t.Fatalf("peer IP = %s, want %s", got, want)
	}
	if got, want := peers[0].Port, uint16(51413); got != want {
		t.Fatalf("peer port = %d, want %d", got, want)
	}
}

func TestGetPeersUDPTrackerIPv6(t *testing.T) {
	conn, err := net.ListenPacket("udp6", "[::1]:0")
	if err != nil {
		t.Fatalf("ListenPacket() error = %v", err)
	}
	defer conn.Close()

	var infoHash [20]byte
	copy(infoHash[:], []byte("01234567890123456789"))

	serverErr := make(chan error, 1)
	go fakeUDPTrackerIPv6(t, conn, infoHash, serverErr)

	tf := &torrent.TorrentFile{
		Announce: "udp://" + conn.LocalAddr().String() + "/announce",
		Length:   12345,
		InfoHash: infoHash,
	}
	peers, err := tracker.GetPeers(tf)
	if err != nil {
		t.Fatalf("GetPeers() error = %v", err)
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("fake tracker error = %v", err)
	}

	if len(peers) != 1 {
		t.Fatalf("len(peers) = %d, want 1", len(peers))
	}
	if got, want := peers[0].IP.String(), "2001:db8::1"; got != want {
		t.Fatalf("peer IP = %s, want %s", got, want)
	}
	if got, want := peers[0].Port, uint16(6881); got != want {
		t.Fatalf("peer port = %d, want %d", got, want)
	}
}

func fakeUDPTrackerIPv6(t *testing.T, conn net.PacketConn, infoHash [20]byte, errc chan<- error) {
	t.Helper()
	defer close(errc)

	buf := make([]byte, 1500)
	n, addr, err := conn.ReadFrom(buf)
	if err != nil {
		errc <- err
		return
	}
	if n != 16 {
		errc <- fmt.Errorf("connect request length = %d, want 16", n)
		return
	}
	connectTxID := binary.BigEndian.Uint32(buf[12:16])
	connectionID := uint64(0x1122334455667788)

	connectResp := make([]byte, 16)
	binary.BigEndian.PutUint32(connectResp[0:4], 0)
	binary.BigEndian.PutUint32(connectResp[4:8], connectTxID)
	binary.BigEndian.PutUint64(connectResp[8:16], connectionID)
	if _, err := conn.WriteTo(connectResp, addr); err != nil {
		errc <- err
		return
	}

	n, addr, err = conn.ReadFrom(buf)
	if err != nil {
		errc <- err
		return
	}
	if n != 98 {
		errc <- fmt.Errorf("announce request length = %d, want 98", n)
		return
	}
	announceTxID := binary.BigEndian.Uint32(buf[12:16])

	announceResp := make([]byte, 38)
	binary.BigEndian.PutUint32(announceResp[0:4], 1)
	binary.BigEndian.PutUint32(announceResp[4:8], announceTxID)
	binary.BigEndian.PutUint32(announceResp[8:12], 1800)
	binary.BigEndian.PutUint32(announceResp[12:16], 1)
	binary.BigEndian.PutUint32(announceResp[16:20], 2)
	
	ipv6Addr := net.ParseIP("2001:db8::1")
	copy(announceResp[20:36], ipv6Addr)
	binary.BigEndian.PutUint16(announceResp[36:38], 6881)

	if _, err := conn.WriteTo(announceResp, addr); err != nil {
		errc <- err
		return
	}
}
