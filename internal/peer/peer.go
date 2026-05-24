	package peer

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

type Client struct {
	Conn     net.Conn
	State    PeerState
	PeerChan chan<- string
}

func Connect(address string, infoHash [20]byte) (*Client, error) {
	return ConnectTimeout(address, infoHash, 10*time.Second)
}

func ConnectTimeout(address string, infoHash [20]byte, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()

	fmt.Printf("[STAGE] peer.Connect: dial success to %s\n", address)

	if timeout > 0 {
		if err = conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return nil, err
		}
	}

	var peerID [20]byte
	copy(peerID[:], []byte("-SD001-1234567890"))
	hs := NewHandshake(infoHash, peerID)

	fmt.Printf("[STAGE] peer.Connect: sending handshake to %s\n", address)
	_, err = conn.Write(hs.Serialize())
	if err != nil {
		return nil, err
	}
	fmt.Printf("[STAGE] peer.Connect: handshake sent to %s\n", address)

	// Read the first byte to determine protocol string length
	pstrLenBuf := make([]byte, 1)
	_, err = io.ReadFull(conn, pstrLenBuf)
	if err != nil {
		return nil, err
	}
	pstrlen := int(pstrLenBuf[0])

	// Read the remaining handshake bytes (pstrlen + 48 bytes)
	remainingLen := pstrlen + 48
	remainingBuf := make([]byte, remainingLen)
	_, err = io.ReadFull(conn, remainingBuf)
	if err != nil {
		return nil, err
	}

	// Reassemble the complete handshake message
	resp := make([]byte, 1+remainingLen)
	resp[0] = pstrLenBuf[0]
	copy(resp[1:], remainingBuf)

	recvHs, err := ReadHandshake(resp)
	if err != nil {
		return nil, err
	}
	if !VerifyHandshake(hs, recvHs) {
		return nil, fmt.Errorf("handshake verify fail")
	}

	fmt.Printf("[STAGE] peer.Connect: handshake received from %s (Pstr: %q)\n", address, recvHs.Pstr)

	if timeout > 0 {
		if err = conn.SetDeadline(time.Time{}); err != nil {
			return nil, err
		}
	}

	return &Client{
		Conn: conn,
		State: PeerState{
			Choked:     true,
			LastActive: time.Now(),
		},
	}, nil
}

func (c *Client) WriteMessage(msg *Message) error {
	_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer c.Conn.SetWriteDeadline(time.Time{})
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

func (c *Client) SendInterested() error {
	msg := &Message{ID: MsgInterested}
	return c.WriteMessage(msg)
}

func (c *Client) SendNotInterested() error {
	msg := &Message{ID: MsgNotInterested}
	return c.WriteMessage(msg)
}

func (c *Client) SendHave(pieceIndex int) error {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(pieceIndex))
	msg := &Message{ID: MsgHave, Payload: payload}
	return c.WriteMessage(msg)
}

func (c *Client) ReadBitField(totalPieces int) error {
	// Initialize an empty bitfield in case the peer doesn't send one (e.g. they have no pieces yet)
	c.State.Bitfield = make([]bool, totalPieces)

	for {
		msg, err := ReadMessage(c.Conn)
		if err != nil {
			return err
		}
		if msg == nil {
			// Keepalive message, just continue waiting
			continue
		}

		switch msg.ID {
		case MsgBitfield:
			bitfield, err := ParseBitField(msg.Payload, totalPieces)
			if err != nil {
				return err
			}
			c.State.Bitfield = bitfield
			fmt.Printf("[STAGE] peer.Connect: bitfield received from %s (%d pieces)\n", c.Conn.RemoteAddr(), len(bitfield))
			return nil

		case MsgExtended:
			// Peer supports extensions and immediately sent their extended handshake response.
			// Handle it (advertises ut_pex, etc.)
			_ = c.HandleExtended(msg, c.PeerChan)
			continue

		case MsgHave:
			// Peer is sending standard Have messages, meaning they skipped the initial Bitfield
			err = c.HandleHave(msg)
			if err != nil {
				return err
			}
			return nil

		case MsgChoke:
			c.State.Choked = true
			return nil

		case MsgUnchoke:
			c.State.Choked = false
			return nil

		default:
			// For any other unexpected message ID, we can assume the initial setup/handshake
			// is finished and the peer has chosen to skip the bitfield. We return nil to
			// proceed with normal communication.
			fmt.Printf("[STAGE] peer.Connect: received message ID %d from %s during startup, proceeding\n", msg.ID, c.Conn.RemoteAddr())
			return nil
		}
	}
}

func (c *Client) HandleHave(msg *Message) error {

	pieceIndex, err := ParseHave(msg.Payload)
	if err != nil {
		return err
	}

	if pieceIndex < 0 ||
		pieceIndex >= len(c.State.Bitfield) {

		return fmt.Errorf(
			"invalid piece index: %d",
			pieceIndex,
		)
	}
	fmt.Println("peer announced piece:", pieceIndex)
	c.State.Bitfield[pieceIndex] = true

	return nil
}

func (c *Client) ReadNextMessage() error {
	msg, err := ReadMessage(c.Conn)
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}

	switch msg.ID {
	case MsgHave:
		return c.HandleHave(msg)
	}

	return nil
}

func (c *Client) SendChoke() error {
	msg := &Message{ID: MsgChoke}
	return c.WriteMessage(msg)
}

func (c *Client) SendUnchoke() error {
	msg := &Message{ID: MsgUnchoke}
	return c.WriteMessage(msg)
}
