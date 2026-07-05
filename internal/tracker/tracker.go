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
	"sync"
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

	var allAnnounces []string
	for _, tier := range trackerTiers {
		for _, announce := range tier {
			if announce != "" {
				allAnnounces = append(allAnnounces, announce)
			}
		}
	}

	var mu sync.Mutex
	var allPeers []Peer
	seenPeers := make(map[string]bool)
	var errs []error
	var wg sync.WaitGroup

	for _, announce := range allAnnounces {
		wg.Add(1)
		go func(ann string) {
			defer wg.Done()
			peers, err := getPeersFromAnnounce(ann, tf, peerID)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", ann, err))
				mu.Unlock()
				return
			}
			mu.Lock()
			for _, p := range peers {
				key := fmt.Sprintf("%s:%d", p.IP.String(), p.Port)
				if !seenPeers[key] {
					seenPeers[key] = true
					allPeers = append(allPeers, p)
				}
			}
			mu.Unlock()
		}(announce)
	}

	wg.Wait()

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

	var peers []Peer
	if peerBytes, ok := root["peers"].([]byte); ok {
		peers = append(peers, parseCompactPeers(peerBytes, false)...)
	} else if peerList, ok := root["peers"].([]interface{}); ok {
		for _, item := range peerList {
			if peerDict, ok := item.(map[string]interface{}); ok {
				var ipStr string
				if ipBytes, ok := peerDict["ip"].([]byte); ok {
					ipStr = string(ipBytes)
				} else if ipS, ok := peerDict["ip"].(string); ok {
					ipStr = ipS
				}
				var port int64
				if pVal, ok := peerDict["port"].(int64); ok {
					port = pVal
				}
				if ipStr != "" && port > 0 && port <= 65535 {
					ip := net.ParseIP(ipStr)
					if ip != nil {
						peers = append(peers, Peer{
							IP:   ip,
							Port: uint16(port),
						})
					}
				}
			}
		}
	}

	if peerBytes6, ok := root["peers6"].([]byte); ok {
		fmt.Printf("[DEBUG] getHTTPPeers: found peers6, parsing compact IPv6 peers...\n")
		peers = append(peers, parseCompactPeers(peerBytes6, true)...)
	} else if peerList6, ok := root["peers6"].([]interface{}); ok {
		fmt.Printf("[DEBUG] getHTTPPeers: found peers6 list, parsing IPv6 dictionary peers...\n")
		for _, item := range peerList6 {
			if peerDict, ok := item.(map[string]interface{}); ok {
				var ipStr string
				if ipBytes, ok := peerDict["ip"].([]byte); ok {
					ipStr = string(ipBytes)
				} else if ipS, ok := peerDict["ip"].(string); ok {
					ipStr = ipS
				}
				var port int64
				if pVal, ok := peerDict["port"].(int64); ok {
					port = pVal
				}
				if ipStr != "" && port > 0 && port <= 65535 {
					ip := net.ParseIP(ipStr)
					if ip != nil {
						peers = append(peers, Peer{
							IP:   ip,
							Port: uint16(port),
						})
					}
				}
			}
		}
	}

	if len(peers) == 0 {
		var keys []string
		for k, v := range root {
			keys = append(keys, fmt.Sprintf("%s (%T)", k, v))
		}
		return nil, fmt.Errorf("tracker response missing both peers and peers6 fields. Keys present: %v", keys)
	}

	fmt.Println("tracker returned peers:", len(peers))
	return peers, nil
}

type UDPTrackerConfig struct {
	InitialTimeout time.Duration
	MaxRetries     int
	BackoffFactor  float64
}

var defaultUDPConfig = UDPTrackerConfig{
	InitialTimeout: 1500 * time.Millisecond,
	MaxRetries:     3,
	BackoffFactor:  2.0,
}

func tryUDPTrackerOnIP(address string, tf *torrent.TorrentFile, peerID string, cfg UDPTrackerConfig) ([]Peer, error) {
	conn, err := net.DialTimeout("udp", address, cfg.InitialTimeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Connect attempt - with retries and exponential backoff
	var connectionID int64
	var connectErr error
	timeout := cfg.InitialTimeout
	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		_ = conn.SetDeadline(time.Now().Add(timeout))
		connectionID, connectErr = udpConnect(conn)
		if connectErr == nil {
			break
		}
		timeout = time.Duration(float64(timeout) * cfg.BackoffFactor)
	}
	if connectErr != nil {
		return nil, connectErr
	}

	// Announce attempt - with retries and exponential backoff
	var peers []Peer
	var announceErr error
	timeout = cfg.InitialTimeout
	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		_ = conn.SetDeadline(time.Now().Add(timeout))
		peers, announceErr = udpAnnounce(conn, connectionID, tf, peerID)
		if announceErr == nil {
			break
		}
		timeout = time.Duration(float64(timeout) * cfg.BackoffFactor)
	}
	if announceErr != nil {
		return nil, announceErr
	}

	return peers, nil
}

func getUDPPeers(announceURL *url.URL, tf *torrent.TorrentFile, peerID string, timeout time.Duration) ([]Peer, error) {
	host, portStr, err := net.SplitHostPort(announceURL.Host)
	if err != nil {
		host = announceURL.Host
		portStr = "80"
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("lookup host %s failed: %w", host, err)
	}

	// Support both IPv4 and IPv6, preferring IPv6
	var orderedIPs []net.IP
	for _, ip := range ips {
		if ip.To4() == nil {
			orderedIPs = append(orderedIPs, ip)
		}
	}
	for _, ip := range ips {
		if ip.To4() != nil {
			orderedIPs = append(orderedIPs, ip)
		}
	}

	var lastErr error
	for _, ip := range orderedIPs {
		address := net.JoinHostPort(ip.String(), portStr)
		fmt.Printf("[STAGE] UDP Tracker: Trying IP %s for %s\n", address, announceURL.Host)

		peers, err := tryUDPTrackerOnIP(address, tf, peerID, defaultUDPConfig)
		if err == nil {
			return peers, nil
		}
		fmt.Printf("[STAGE] UDP Tracker: IP %s failed: %v\n", address, err)
		lastErr = err
	}

	return nil, fmt.Errorf("all IPs for %s failed: %w", announceURL.Host, lastErr)
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

	// [DEBUG] Announce response hex dump diagnostics
	fmt.Printf("[DEBUG] announce response len=%d from %s\n", n, conn.RemoteAddr())
	fmt.Printf("[DEBUG] raw=%x\n", resp[:n])

	action := binary.BigEndian.Uint32(resp[0:4])
	gotTransactionID := binary.BigEndian.Uint32(resp[4:8])

	if action == udpActionError {
		return nil, fmt.Errorf("udp tracker error: %s", string(resp[8:n]))
	}
	if action != udpActionAnnounce {
		return nil, fmt.Errorf("invalid udp announce response action (got=%d, want=%d)", action, udpActionAnnounce)
	}
	if gotTransactionID != transactionID {
		return nil, fmt.Errorf("invalid udp announce response transaction ID (got=%x, want=%x)", gotTransactionID, transactionID)
	}
	if n < 20 {
		return nil, fmt.Errorf("short udp announce response: %d bytes (expected at least 20)", n)
	}

	isIPv6 := false
	if udpAddr, ok := conn.RemoteAddr().(*net.UDPAddr); ok {
		isIPv6 = udpAddr.IP.To4() == nil
	}
	peers := parseCompactPeers(resp[20:n], isIPv6)
	fmt.Println("tracker returned peers:", len(peers))
	return peers, nil
}

func parseCompactPeers(data []byte, isIPv6 bool) []Peer {
	peerSize := 6
	if isIPv6 {
		peerSize = 18
	}

	fmt.Printf("[DEBUG] parseCompactPeers: len(data)=%d, isIPv6=%t, peerSize=%d\n", len(data), isIPv6, peerSize)

	if len(data)%peerSize != 0 {
		fmt.Printf("[DEBUG] warning: malformed compact peer list (length %d, expected multiple of %d)\n", len(data), peerSize)
	}

	var peers []Peer

	for i := 0; i+peerSize <= len(data); i += peerSize {
		var ip net.IP
		var port uint16

		if isIPv6 {
			ip = net.IP(data[i : i+16])
			port = binary.BigEndian.Uint16(data[i+16 : i+18])
		} else {
			ip = net.IP(data[i : i+4])
			port = binary.BigEndian.Uint16(data[i+4 : i+6])
		}

		// Skip invalid or private/unspecified multicast ranges
		if ip.IsUnspecified() || ip.IsMulticast() {
			continue
		}

		peers = append(peers, Peer{
			IP:   ip,
			Port: port,
		})
	}

	fmt.Printf("[DEBUG] parseCompactPeers: successfully parsed %d peers\n", len(peers))
	for _, p := range peers {
		fmt.Printf("  [DEBUG] parsed peer endpoint: %s:%d\n", p.IP, p.Port)
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
