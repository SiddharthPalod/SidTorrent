package peer

import (
	"fmt"
	"io"
	"net"
	"time"
)

type Client struct {
	Conn  net.Conn
	State PeerState
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

func (c *Client) SendInterested() error {
	msg := &Message{
		ID: MsgInterested,
	}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

func (c *Client) ReadBitField(totalPieces int) error {
	msg, err := ReadMessage(c.Conn)
	if err != nil {
		return err
	}
	if msg == nil {
		return fmt.Errorf("received keepalive instead of bitfield")
	}
	if msg.ID != MsgBitfield {
		return fmt.Errorf(
			"expected bitfield message got=%d",
			msg.ID,
		)
	}

	bitfield, err := ParseBitField(msg.Payload, totalPieces)
	if err != nil {
		return err
	}
	c.State.Bitfield = bitfield

	fmt.Printf("[STAGE] peer.Connect: bitfield received from %s (%d pieces)\n", c.Conn.RemoteAddr(), len(bitfield))

	return nil
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
