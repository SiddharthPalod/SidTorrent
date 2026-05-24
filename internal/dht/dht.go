package dht

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
	"github.com/SiddharthPalod/SidTorrent/internal/tracker"
)

type DHTNode struct {
	ID           [20]byte
	routingTable *RoutingTable
	conn         *net.UDPConn
	peersMu      sync.Mutex
	peersMap     map[[20]byte][]tracker.Peer
	closed       chan struct{}
	wg           sync.WaitGroup

	// Transaction response routing
	waitersMu    sync.Mutex
	waiters      map[string]chan map[string]interface{}
	txCounter    uint32
}

func NewDHTNode(port int) (*DHTNode, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	id := GenerateRandomID()
	node := &DHTNode{
		ID:           id,
		routingTable: NewRoutingTable(id),
		conn:         conn,
		peersMap:     make(map[[20]byte][]tracker.Peer),
		closed:       make(chan struct{}),
		waiters:      make(map[string]chan map[string]interface{}),
	}
	node.wg.Add(1)
	go node.listenLoop()
	return node, nil
}

func (dht *DHTNode) listenLoop() {
	defer dht.wg.Done()
	buf := make([]byte, 2048)
	for {
		select {
		case <-dht.closed:
			return
		default:
		}

		_ = dht.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, raddr, err := dht.conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		dht.handleMessage(buf[:n], raddr)
	}
}

func (dht *DHTNode) handleMessage(data []byte, raddr *net.UDPAddr) {
	decoded, err := bencode.DecodeBytes(data)
	if err != nil {
		fmt.Printf("[DEBUG] DHT: Failed to decode incoming bencoded UDP packet from %s: %v\n", raddr, err)
		return
	}
	m, ok := decoded.(map[string]interface{})
	if !ok {
		return
	}

	y, ok := m["y"].([]byte)
	if !ok {
		return
	}

	tBytes, ok := m["t"].([]byte)
	if !ok {
		return
	}
	t := string(tBytes)

	// Intercept responses ("r") matching pending waiters
	if string(y) == "r" {
		dht.waitersMu.Lock()
		ch, exists := dht.waiters[t]
		if exists {
			delete(dht.waiters, t)
			dht.waitersMu.Unlock()
			select {
			case ch <- m:
			default:
			}
			dht.handleResponse(m, raddr)
			return
		}
		dht.waitersMu.Unlock()
	}

	switch string(y) {
	case "q":
		dht.handleQuery(m, raddr)
	case "r":
		dht.handleResponse(m, raddr)
	}
}

func (dht *DHTNode) handleQuery(m map[string]interface{}, raddr *net.UDPAddr) {
	q, _ := m["q"].([]byte)
	t, _ := m["t"].([]byte)
	a, _ := m["a"].(map[string]interface{})

	if len(q) == 0 || len(t) == 0 || a == nil {
		return
	}

	if senderID, ok := a["id"].([]byte); ok && len(senderID) == 20 {
		var id [20]byte
		copy(id[:], senderID)

		dht.routingTable.Add(Contact{ID: id, IP: raddr.IP, Port: uint16(raddr.Port)})

		switch string(q) {
		case "ping":
			resp := map[string]interface{}{
				"t": t, "y": "r", "r": map[string]interface{}{
					"id": dht.ID[:],
				},
			}
			dht.writeUDP(bencode.Encode(resp), raddr)
		case "find_node":
			target, _ := a["target"].([]byte)
			if len(target) != 20 {
				return
			}
			var targetID [20]byte
			copy(targetID[:], target)
			closest := dht.routingTable.ClosestNodes(targetID, K)
			resp := map[string]interface{}{
				"t": t,
				"y": "r",
				"r": map[string]interface{}{
					"id":    dht.ID[:],
					"nodes": packCompactNodes(closest),
				},
			}
			dht.writeUDP(bencode.Encode(resp), raddr)
		case "get_peers":
			info, _ := a["info_hash"].([]byte)
			if len(info) != 20 {
				return
			}
			var infoHash [20]byte
			copy(infoHash[:], info)
			dht.peersMu.Lock()
			peers, found := dht.peersMap[infoHash]
			dht.peersMu.Unlock()
			rDict := map[string]interface{}{
				"id": dht.ID[:],
			}
			if found && len(peers) > 0 {
				var peerList []interface{}
				for _, p := range peers {
					var b [6]byte
					copy(b[0:4], p.IP.To4())
					binary.BigEndian.PutUint16(b[4:6], p.Port)
					peerList = append(peerList, b[:])
				}
				rDict["values"] = peerList
			} else {
				closest := dht.routingTable.ClosestNodes(infoHash, K)
				rDict["nodes"] = packCompactNodes(closest)
			}
			resp := map[string]interface{}{
				"t": t,
				"y": "r",
				"r": rDict,
			}
			dht.writeUDP(bencode.Encode(resp), raddr)
		}
	}
}

func (dht *DHTNode) handleResponse(m map[string]interface{}, raddr *net.UDPAddr) {
	r, _ := m["r"].(map[string]interface{})
	if r == nil {
		return
	}
	senderID, ok := r["id"].([]byte)
	if !ok || len(senderID) != 20 {
		return
	}
	var id [20]byte
	copy(id[:], senderID)

	fmt.Printf("[DEBUG] DHT: Received bootstrap/find_node response from node %x at %s\n", id, raddr)

	dht.routingTable.Add(Contact{
		ID:   id,
		IP:   raddr.IP,
		Port: uint16(raddr.Port),
	})
}

func (dht *DHTNode) Bootstrap() {
	routers := []string{
		"router.bittorrent.com",
		"router.utorrent.com",
		"dht.transmissionbt.com",
	}
	for _, host := range routers {
		ips, err := net.LookupIP(host)
		if err != nil {
			fmt.Printf("[DEBUG] DHT Bootstrap: DNS lookup failed for %s: %v\n", host, err)
			continue
		}
		for _, ip := range ips {
			raddr := &net.UDPAddr{IP: ip, Port: 6881}
			fmt.Printf("[DEBUG] DHT Bootstrap: Sending find_node query to %s (%s)\n", host, raddr)

			query := map[string]interface{}{
				"t": "aa",
				"y": "q",
				"q": "find_node",
				"a": map[string]interface{}{
					"id":     dht.ID[:],
					"target": dht.ID[:],
				},
			}
			dht.writeUDP(bencode.Encode(query), raddr)
		}
	}
	// Sleep briefly to accumulate bootstrap responses
	time.Sleep(1500 * time.Millisecond)
}

func (dht *DHTNode) SearchPeers(infoHash [20]byte) []tracker.Peer {
	closest := dht.routingTable.ClosestNodes(infoHash, K)
	if len(closest) == 0 {
		return nil
	}
	var discovered []tracker.Peer
	seenPeers := make(map[string]bool)
	visited := make(map[[20]byte]bool)
	// Bounded recursive traversal to locate closest peers
	for i := 0; i < 3 && len(closest) > 0; i++ {
		var next []Contact
		for _, contact := range closest {
			if visited[contact.ID] {
				continue
			}
			visited[contact.ID] = true
			peers, nodes, err := dht.queryGetPeers(contact, infoHash)
			if err != nil {
				continue
			}
			for _, p := range peers {
				key := fmt.Sprintf("%s:%d", p.IP.String(), p.Port)
				if !seenPeers[key] {
					seenPeers[key] = true
					discovered = append(discovered, p)
				}
			}
			next = append(next, nodes...)
		}
		closest = next
	}
	return discovered
}
func (dht *DHTNode) queryGetPeers(contact Contact, infoHash [20]byte) ([]tracker.Peer, []Contact, error) {
	raddr := &net.UDPAddr{IP: contact.IP, Port: int(contact.Port)}

	// Generate unique 2-byte transaction ID using atomic counter
	id := atomic.AddUint32(&dht.txCounter, 1)
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(id))
	txID := string(b[:])

	// Register channel waiter
	ch := make(chan map[string]interface{}, 1)
	dht.waitersMu.Lock()
	dht.waiters[txID] = ch
	dht.waitersMu.Unlock()

	defer func() {
		dht.waitersMu.Lock()
		delete(dht.waiters, txID)
		dht.waitersMu.Unlock()
	}()

	query := map[string]interface{}{
		"t": txID,
		"y": "q",
		"q": "get_peers",
		"a": map[string]interface{}{
			"id":        dht.ID[:],
			"info_hash": infoHash[:],
		},
	}

	_, err := dht.conn.WriteToUDP(bencode.Encode(query), raddr)
	if err != nil {
		return nil, nil, err
	}

	// Dynamic wait for response
	select {
	case m := <-ch:
		r, _ := m["r"].(map[string]interface{})
		if r == nil {
			return nil, nil, errors.New("missing dict")
		}
		// Parse returned peer addresses (supports both 6-byte IPv4 and 18-byte IPv6 compact formats!)
		var peers []tracker.Peer
		if values, ok := r["values"].([]interface{}); ok {
			for _, v := range values {
				b, ok := v.([]byte)
				if ok {
					if len(b) == 6 {
						ip := net.IP(b[0:4])
						port := binary.BigEndian.Uint16(b[4:6])
						peers = append(peers, tracker.Peer{IP: ip, Port: port})
					} else if len(b) == 18 {
						ip := net.IP(b[0:16])
						port := binary.BigEndian.Uint16(b[16:18])
						peers = append(peers, tracker.Peer{IP: ip, Port: port})
					}
				}
			}
		}
		// Parse closer contact nodes
		var nodes []Contact
		if nodeBytes, ok := r["nodes"].([]byte); ok {
			nodes = parseCompactNodes(nodeBytes)
		}
		return peers, nodes, nil

	case <-time.After(2 * time.Second):
		return nil, nil, errors.New("query get_peers timeout")
	}
}

// Close gracefully closes the listener connection.
func (dht *DHTNode) Close() {
	close(dht.closed)
	_ = dht.conn.Close()
	dht.wg.Wait()
}

func (dht *DHTNode) writeUDP(data []byte, raddr *net.UDPAddr) {
	_, err := dht.conn.WriteToUDP(data, raddr)
	if err != nil {
		fmt.Printf("[DEBUG] DHT: Failed to write UDP packet to %s: %v\n", raddr, err)
	}
}

func parseCompactNodes(data []byte) []Contact {
	var contacts []Contact
	// Dynamically detect contact node size: 38 bytes for IPv6, 26 bytes for IPv4.
	nodeSize := 26
	if len(data)%38 == 0 && len(data)%26 != 0 {
		nodeSize = 38
	}

	for i := 0; i+nodeSize <= len(data); i += nodeSize {
		var id [20]byte
		copy(id[:], data[i:i+20])
		var ip net.IP
		var port uint16
		if nodeSize == 38 {
			ip = net.IP(data[i+20 : i+36])
			port = binary.BigEndian.Uint16(data[i+36 : i+38])
		} else {
			ip = net.IP(data[i+20 : i+24])
			port = binary.BigEndian.Uint16(data[i+24 : i+26])
		}
		contacts = append(contacts, Contact{ID: id, IP: ip, Port: port})
	}
	return contacts
}

func packCompactNodes(contacts []Contact) []byte {
	var buf bytes.Buffer
	for _, c := range contacts {
		buf.Write(c.ID[:])
		ip4 := c.IP.To4()
		if ip4 != nil {
			buf.Write(ip4)
		} else {
			buf.Write(c.IP)
		}
		var p [2]byte
		binary.BigEndian.PutUint16(p[:], c.Port)
		buf.Write(p[:])
	}
	return buf.Bytes()
}
