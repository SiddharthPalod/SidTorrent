package peer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
)

const UtPexExtensionName = "ut_pex"
const LocalPexExtensionID = 1

type ExtensionHandshake struct {
	M map[string]int64 `bencode:"m"`
	V string           `bencode:"v"`
}

func (c *Client) SendExtensionHandshake() error {
	payloadDict := map[string]interface{}{
		"m": map[string]interface{}{
			UtPexExtensionName: int64(LocalPexExtensionID),
		},
		"v": "SidTorrent 1.0",
	}
	bencoded := bencode.Encode(payloadDict)
	extendedPayload := make([]byte, len(bencoded)+1)
	extendedPayload[0] = 0
	copy(extendedPayload[1:], bencoded)

	msg := &Message{
		ID:      MsgExtended,
		Payload: extendedPayload,
	}
	return c.WriteMessage(msg)
}

func (c *Client) SendPexMessage(added []string, remotePexID int) error {
	var add4Buf bytes.Buffer
	var add6Buf bytes.Buffer

	for _, addr := range added {
		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			continue
		}
		ip := net.ParseIP(host)
		var port uint16
		_, _ = fmt.Sscanf(portStr, "%d", &port)

		if ip4 := ip.To4(); ip4 != nil {
			add4Buf.Write(ip4)
			var p [2]byte
			binary.BigEndian.PutUint16(p[:], uint16(port))
			add4Buf.Write(p[:])
		} else {
			add6Buf.Write(ip)
			var p [2]byte
			binary.BigEndian.PutUint16(p[:], uint16(port))
			add6Buf.Write(p[:])
		}
	}

	pexDict := map[string]interface{}{
		"added": add4Buf.Bytes(),
	}
	if add6Buf.Len() > 0 {
		pexDict["added6"] = add6Buf.Bytes()
	}
	bencoded := bencode.Encode(pexDict)
	extendedPayload := make([]byte, 1+len(bencoded))
	extendedPayload[0] = byte(remotePexID)
	copy(extendedPayload[1:], bencoded)
	msg := &Message{
		ID:      MsgExtended,
		Payload: extendedPayload,
	}
	return c.WriteMessage(msg)
}

func (c *Client) HandleExtended(msg *Message, peerChan chan<- string) error {
	if len(msg.Payload) < 1 {
		return fmt.Errorf("empty extended message payload")
	}
	extID := msg.Payload[0]
	rawPayload := msg.Payload[1:]
	decoded, err := bencode.DecodeBytes(rawPayload)
	if err != nil {
		return fmt.Errorf("bencode decode extended failed: %w", err)
	}
	dict, ok := decoded.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid extended message payload format")
	}
	// 1. Extended Handshake Response
	if extID == 0 {
		m, ok := dict["m"].(map[string]interface{})
		if !ok {
			return nil
		}
		if pexID, exists := m[UtPexExtensionName].(int64); exists {
			c.State.RemotePexID = int(pexID)
			fmt.Printf("[STAGE] PEX: Peer %s supports ut_pex (Remote ID: %d)\n", c.Conn.RemoteAddr(), pexID)
		}
		return nil
	}
	// 2. Incoming PEX updates (extID matches our local extension PEX ID)
	if extID == LocalPexExtensionID {
		var addedPeersCount int
		// Parse compact IPv4 peers
		if addedBytes, ok := dict["added"].([]byte); ok {
			for i := 0; i+6 <= len(addedBytes); i += 6 {
				ip := net.IP(addedBytes[i : i+4])
				port := binary.BigEndian.Uint16(addedBytes[i+4 : i+6])
				peerChan <- net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port))
				addedPeersCount++
			}
		}
		// Parse compact IPv6 peers
		if addedBytes6, ok := dict["added6"].([]byte); ok {
			for i := 0; i+18 <= len(addedBytes6); i += 18 {
				ip := net.IP(addedBytes6[i : i+16])
				port := binary.BigEndian.Uint16(addedBytes6[i+16 : i+18])
				peerChan <- net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port))
				addedPeersCount++
			}
		}
		if addedPeersCount > 0 {
			fmt.Printf("[STAGE] PEX: Discovered %d new peers from %s via ut_pex\n", addedPeersCount, c.Conn.RemoteAddr())
		}
	}
	return nil
}
