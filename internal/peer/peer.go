package peer

import (
	"bufio"
	"bytes"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/config"
)

type PeekConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *PeekConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

type EncryptedConn struct {
	net.Conn
	enc *rc4.Cipher
	dec *rc4.Cipher
}

func NewEncryptedConn(conn net.Conn, infoHash [20]byte, isInitiator bool) (*EncryptedConn, error) {
	h1 := sha1.New()
	h1.Write([]byte("SiddTorrentKeyA"))
	h1.Write(infoHash[:])
	keyA := h1.Sum(nil)

	h2 := sha1.New()
	h2.Write([]byte("SiddTorrentKeyB"))
	h2.Write(infoHash[:])
	keyB := h2.Sum(nil)

	var encKey, decKey []byte
	if isInitiator {
		encKey = keyA
		decKey = keyB
	} else {
		encKey = keyB
		decKey = keyA
	}

	encCipher, err := rc4.NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	decCipher, err := rc4.NewCipher(decKey)
	if err != nil {
		return nil, err
	}

	return &EncryptedConn{
		Conn: conn,
		enc:  encCipher,
		dec:  decCipher,
	}, nil
}

func (c *EncryptedConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if err == nil {
		c.dec.XORKeyStream(b[:n], b[:n])
	}
	return
}

func (c *EncryptedConn) Write(b []byte) (n int, err error) {
	buf := make([]byte, len(b))
	c.enc.XORKeyStream(buf, b)
	return c.Conn.Write(buf)
}

func AcceptIncoming(conn net.Conn, infoHash [20]byte) (*Client, error) {
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{})

	reader := bufio.NewReader(conn)
	firstByte, err := reader.Peek(1)
	if err != nil {
		return nil, err
	}

	var wrappedConn net.Conn = &PeekConn{Conn: conn, r: reader}

	if firstByte[0] != 0x13 {
		fmt.Printf("[STAGE] AcceptIncoming: Encrypted incoming connection detected from %s\n", conn.RemoteAddr())
		encConn, err := NewEncryptedConn(wrappedConn, infoHash, false)
		if err != nil {
			return nil, err
		}
		wrappedConn = encConn
	}

	pstrLenBuf := make([]byte, 1)
	if _, err := io.ReadFull(wrappedConn, pstrLenBuf); err != nil {
		return nil, err
	}
	pstrlen := int(pstrLenBuf[0])

	remainingLen := pstrlen + 48
	remainingBuf := make([]byte, remainingLen)
	if _, err := io.ReadFull(wrappedConn, remainingBuf); err != nil {
		return nil, err
	}

	resp := make([]byte, 1+remainingLen)
	resp[0] = pstrLenBuf[0]
	copy(resp[1:], remainingBuf)

	recvHs, err := ReadHandshake(resp)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(recvHs.InfoHash[:], infoHash[:]) {
		return nil, fmt.Errorf("info hash mismatch")
	}

	var peerID [20]byte
	copy(peerID[:], []byte("-SD001-1234567890"))
	hs := NewHandshake(infoHash, peerID)
	_, err = wrappedConn.Write(hs.Serialize())
	if err != nil {
		return nil, err
	}

	return &Client{
		Conn: wrappedConn,
		State: PeerState{
			Choked:     true,
			LastActive: time.Now(),
		},
	}, nil
}

type Client struct {
	Conn     net.Conn
	State    PeerState
	PeerChan chan<- string
}

func Connect(address string, infoHash [20]byte) (*Client, error) {
	return ConnectTimeout(address, infoHash, 10*time.Second)
}

func ConnectTimeout(address string, infoHash [20]byte, timeout time.Duration) (*Client, error) {
	if config.GlobalConfig.EnableEncryption {
		encTimeout := timeout
		if encTimeout > 3*time.Second {
			encTimeout = 3 * time.Second
		}
		client, err := connectTimeoutWithEnc(address, infoHash, encTimeout, true)
		if err == nil {
			return client, nil
		}
		fmt.Printf("[STAGE] peer.Connect: Encrypted connection to %s failed (%v), falling back to plaintext...\n", address, err)
	}
	return connectTimeoutWithEnc(address, infoHash, timeout, false)
}

func connectTimeoutWithEnc(address string, infoHash [20]byte, timeout time.Duration, useEncryption bool) (*Client, error) {
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

	var wrappedConn net.Conn = conn
	if useEncryption {
		fmt.Printf("[STAGE] peer.Connect: Wrapping connection to %s in Encryption\n", address)
		encConn, err := NewEncryptedConn(conn, infoHash, true)
		if err != nil {
			return nil, err
		}
		wrappedConn = encConn
	}

	var peerID [20]byte
	copy(peerID[:], []byte("-SD001-1234567890"))
	hs := NewHandshake(infoHash, peerID)

	_, err = wrappedConn.Write(hs.Serialize())
	if err != nil {
		return nil, err
	}

	pstrLenBuf := make([]byte, 1)
	_, err = io.ReadFull(wrappedConn, pstrLenBuf)
	if err != nil {
		return nil, err
	}
	pstrlen := int(pstrLenBuf[0])

	remainingLen := pstrlen + 48
	remainingBuf := make([]byte, remainingLen)
	_, err = io.ReadFull(wrappedConn, remainingBuf)
	if err != nil {
		return nil, err
	}

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

	if timeout > 0 {
		if err = wrappedConn.SetDeadline(time.Time{}); err != nil {
			return nil, err
		}
	}

	return &Client{
		Conn: wrappedConn,
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
		if c.Conn.RemoteAddr().Network() != "pipe" {
			_ = c.Conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		}
		msg, err := ReadMessage(c.Conn)
		if c.Conn.RemoteAddr().Network() != "pipe" {
			_ = c.Conn.SetReadDeadline(time.Time{})
		}
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
