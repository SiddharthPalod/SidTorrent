package peer

import (
	"fmt"
	"io"
	"net"
	"time"
)

type Client struct {
	Conn   net.Conn
	Choked bool
}

func Connect(address string, infoHash [20]byte) (*Client, error) {
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return nil, err
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

	return &Client{
		Conn:   conn,
		Choked: true,
	}, nil
}

func (c *Client) SendInterested() error {
	msg := &Message{
		ID: MsgInterested,
	}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}
