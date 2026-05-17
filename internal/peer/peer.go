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
