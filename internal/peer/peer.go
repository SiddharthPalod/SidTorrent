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
	if timeout > 0 {
		if err = conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return nil, err
		}
	}

	var peerID [20]byte
	copy(peerID[:], []byte("-SD001-1234567890"))
	hs := NewHandshake(infoHash, peerID)
	_, err = conn.Write(hs.Serialize())
	if err != nil {
		return nil, err
	}

	resp := make([]byte, 68)
	_, err = io.ReadFull(conn, resp)
	if err != nil {
		return nil, err
	}

	recvHs, err := ReadHandshake(resp)
	if err != nil {
		return nil, err
	}
	if !VerifyHandshake(hs, recvHs) {
		return nil, fmt.Errorf("handshake verify fail")
	}
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
