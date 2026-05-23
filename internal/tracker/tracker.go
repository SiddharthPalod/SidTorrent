package tracker

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

type Peer struct {
	IP   net.IP
	Port uint16
}

const (
	udpProtocolID     int64 = 0x41727101980
	udpActionConnect        = 0
	udpActionAnnounce       = 1
	udpActionError          = 3
	defaultPort             = 6881
	defaultNumWant          = 50
)

func GetPeers(tf *torrent.TorrentFile) ([]Peer, error) {

	peerID := randomPeerID()
	trackerTiers := tf.Trackers
	if len(trackerTiers) == 0 && tf.Announce != "" {
		trackerTiers = [][]string{{tf.Announce}}
	}
	if len(trackerTiers) == 0 {
		return nil, fmt.Errorf("torrent has no announce trackers")
	}

	// Prioritize HTTP/HTTPS trackers first for easier initial debugging
	var httpTiers [][]string
	var udpTiers [][]string

	for _, tier := range trackerTiers {
		var httpTier []string
		var udpTier []string
		for _, announce := range tier {
			u, err := url.Parse(announce)
			if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
				httpTier = append(httpTier, announce)
			} else {
				udpTier = append(udpTier, announce)
			}
		}
		if len(httpTier) > 0 {
			httpTiers = append(httpTiers, httpTier)
		}
		if len(udpTier) > 0 {
			udpTiers = append(udpTiers, udpTier)
		}
	}

	orderedTiers := append(httpTiers, udpTiers...)

	var allPeers []Peer
	seenPeers := make(map[string]bool)
	var errs []error

	for tierIndex, tier := range orderedTiers {
		for _, announce := range tier {
			peers, err := getPeersFromAnnounce(announce, tf, peerID)
			if err != nil {
				errs = append(errs, fmt.Errorf("tier %d %s: %w", tierIndex, announce, err))
				continue
			}
			for _, p := range peers {
				key := fmt.Sprintf("%s:%d", p.IP.String(), p.Port)
				if !seenPeers[key] {
					seenPeers[key] = true
					allPeers = append(allPeers, p)
				}
			}
		}
	}

	if len(allPeers) > 0 {
		return allPeers, nil
	}

	return nil, fmt.Errorf("all trackers failed: %w", errors.Join(errs...))
}

func getPeersFromAnnounce(announce string, tf *torrent.TorrentFile, peerID string) ([]Peer, error) {
	announceURL, err := url.Parse(announce)
	if err != nil {
		return nil, err
	}

	switch announceURL.Scheme {
	case "http", "https":
		return getHTTPPeers(announceURL, tf, peerID)
	case "udp":
		return getUDPPeers(announceURL, tf, peerID, 15*time.Second)
	default:
		return nil, fmt.Errorf("unsupported tracker scheme: %s", announceURL.Scheme)
	}
}

func getHTTPPeers(base *url.URL, tf *torrent.TorrentFile, peerID string) ([]Peer, error) {
	trackerURL := *base

	params := url.Values{
		"info_hash":  []string{string(tf.InfoHash[:])},
		"peer_id":    []string{peerID},
		"port":       []string{strconv.Itoa(defaultPort)},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"left":       []string{strconv.FormatInt(tf.Length, 10)},
		"compact":    []string{"1"},
		"numwant":    []string{strconv.Itoa(defaultNumWant)},
	}

	trackerURL.RawQuery = params.Encode()

	fmt.Println("tracker url:")
	fmt.Println(trackerURL.String())

	httpClient := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := httpClient.Get(trackerURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	decoded, err := bencode.DecodeBytes(body)
	if err != nil {
		return nil, err
	}

	root, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tracker response")
	}

	// tracker returned failure
	if failure, ok := root["failure reason"].([]byte); ok {

		return nil, fmt.Errorf(
			"tracker failure: %s",
			string(failure),
		)
	}

	peerBytes, ok := root["peers"].([]byte)
	if !ok {
		return nil, fmt.Errorf("tracker response missing peers")
	}

	peers := parseCompactPeers(peerBytes)

	fmt.Println("tracker returned peers:", len(peers))

	return peers, nil
}

func getUDPPeers(announceURL *url.URL, tf *torrent.TorrentFile, peerID string, timeout time.Duration) ([]Peer, error) {
	address := announceURL.Host
	if _, _, err := net.SplitHostPort(address); err != nil {
		address = net.JoinHostPort(announceURL.Host, "80")
	}

	var conn net.Conn
	var err error
	maxAttempts := 3

	// Attempt dialing with backoff
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		dialTimeout := time.Duration(attempt*5) * time.Second
		fmt.Printf("[STAGE] UDP Tracker: Dialing %s (attempt %d/%d, timeout: %v)\n", address, attempt, maxAttempts, dialTimeout)
		
		conn, err = net.DialTimeout("udp", address, dialTimeout)
		if err == nil {
			break
		}
		fmt.Printf("[STAGE] UDP Tracker: Dial failed for %s: %v\n", address, err)
	}
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Apply a generous active transaction deadline
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))

	var connectionID int64
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		fmt.Printf("[STAGE] UDP Tracker: Sending connect request to %s (attempt %d/%d)\n", address, attempt, maxAttempts)
		connectionID, err = udpConnect(conn)
		if err == nil {
			fmt.Printf("[STAGE] UDP Tracker: Received connect response (connection ID: %x)\n", connectionID)
			break
		}
		fmt.Printf("[STAGE] UDP Tracker: Connect request failed on attempt %d: %v\n", attempt, err)
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return nil, fmt.Errorf("udp connect error: %w", err)
	}

	var peers []Peer
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		fmt.Printf("[STAGE] UDP Tracker: Sending announce request to %s (attempt %d/%d)\n", address, attempt, maxAttempts)
		peers, err = udpAnnounce(conn, connectionID, tf, peerID)
		if err == nil {
			fmt.Printf("[STAGE] UDP Tracker: Received announce response with %d peers from %s\n", len(peers), address)
			break
		}
		fmt.Printf("[STAGE] UDP Tracker: Announce request failed on attempt %d: %v\n", attempt, err)
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return nil, fmt.Errorf("udp announce error: %w", err)
	}

	return peers, nil
}

func udpConnect(conn net.Conn) (int64, error) {
	transactionID, err := randomUint32()
	if err != nil {
		return 0, err
	}

	var req bytes.Buffer
	_ = binary.Write(&req, binary.BigEndian, udpProtocolID)
	_ = binary.Write(&req, binary.BigEndian, uint32(udpActionConnect))
	_ = binary.Write(&req, binary.BigEndian, transactionID)

	fmt.Printf("[STAGE] UDP Tracker (Connect): Transaction ID generated: %x\n", transactionID)

	if _, err := conn.Write(req.Bytes()); err != nil {
		return 0, err
	}

	// Read datagram fully (io.ReadFull causes hangs on UDP short/error packets)
	resp := make([]byte, 100)
	n, err := conn.Read(resp)
	if err != nil {
		return 0, err
	}
	if n < 16 {
		if n >= 8 {
			action := binary.BigEndian.Uint32(resp[0:4])
			if action == udpActionError {
				return 0, fmt.Errorf("udp tracker error: %s", string(resp[8:n]))
			}
		}
		return 0, fmt.Errorf("short udp connect response: %d bytes", n)
	}

	action := binary.BigEndian.Uint32(resp[0:4])
	gotTransactionID := binary.BigEndian.Uint32(resp[4:8])
	if action == udpActionError {
		return 0, fmt.Errorf("udp tracker error: %s", string(resp[8:n]))
	}
	if action != udpActionConnect || gotTransactionID != transactionID {
		return 0, fmt.Errorf("invalid udp connect response (action=%d, gotTxID=%x, wantTxID=%x)", action, gotTransactionID, transactionID)
	}

	return int64(binary.BigEndian.Uint64(resp[8:16])), nil
}

func udpAnnounce(conn net.Conn, connectionID int64, tf *torrent.TorrentFile, peerID string) ([]Peer, error) {
	transactionID, err := randomUint32()
	if err != nil {
		return nil, err
	}
	key, err := randomUint32()
	if err != nil {
		return nil, err
	}

	var req bytes.Buffer
	_ = binary.Write(&req, binary.BigEndian, uint64(connectionID))
	_ = binary.Write(&req, binary.BigEndian, uint32(udpActionAnnounce))
	_ = binary.Write(&req, binary.BigEndian, transactionID)
	req.Write(tf.InfoHash[:])
	req.WriteString(peerID) // peerID is exactly 20 bytes now!
	_ = binary.Write(&req, binary.BigEndian, uint64(0))
	_ = binary.Write(&req, binary.BigEndian, uint64(tf.Length))
	_ = binary.Write(&req, binary.BigEndian, uint64(0))
	_ = binary.Write(&req, binary.BigEndian, uint32(0))
	_ = binary.Write(&req, binary.BigEndian, uint32(0))
	_ = binary.Write(&req, binary.BigEndian, key)
	_ = binary.Write(&req, binary.BigEndian, int32(defaultNumWant))
	_ = binary.Write(&req, binary.BigEndian, uint16(defaultPort))

	// Verify exact 98-byte packet length requirement for UDP announce requests
	if req.Len() != 98 {
		return nil, fmt.Errorf("announce request packet has invalid length: %d bytes (expected exactly 98)", req.Len())
	}

	fmt.Printf("[STAGE] UDP Tracker (Announce): Sending 98-byte announce packet, transaction ID: %x\n", transactionID)

	if _, err := conn.Write(req.Bytes()); err != nil {
		return nil, err
	}

	resp := make([]byte, 1500)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, err
	}
	if n < 8 {
		return nil, fmt.Errorf("short udp announce response")
	}

	action := binary.BigEndian.Uint32(resp[0:4])
	gotTransactionID := binary.BigEndian.Uint32(resp[4:8])
	if action == udpActionError {
		return nil, fmt.Errorf("udp tracker error: %s", string(resp[8:n]))
	}
	if action != udpActionAnnounce || gotTransactionID != transactionID {
		return nil, fmt.Errorf("invalid udp announce response (action=%d, gotTxID=%x, wantTxID=%x)", action, gotTransactionID, transactionID)
	}
	if n < 20 {
		return nil, fmt.Errorf("short udp announce response")
	}

	peers := parseCompactPeers(resp[20:n])
	fmt.Println("tracker returned peers:", len(peers))
	return peers, nil
}

func parseCompactPeers(data []byte) []Peer {

	if len(data)%6 != 0 {
		fmt.Println("warning: malformed compact peer list")
	}

	var peers []Peer

	for i := 0; i+6 <= len(data); i += 6 {

		ip := net.IP(data[i : i+4])

		port := binary.BigEndian.Uint16(
			data[i+4 : i+6],
		)

		peers = append(peers, Peer{
			IP:   ip,
			Port: port,
		})
	}

	return peers
}

func randomPeerID() string {

	return fmt.Sprintf(
		"-SD0001-%012d",
		time.Now().UnixNano()%999999999999,
	)
}

func randomUint32() (uint32, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(b[:]), nil
}
