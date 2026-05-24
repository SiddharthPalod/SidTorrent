package peer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	MsgChoke         = 0
	MsgUnchoke       = 1
	MsgInterested    = 2
	MsgNotInterested = 3
	MsgHave          = 4
	MsgBitfield      = 5
	MsgRequest       = 6
	MsgPiece         = 7
	MsgExtended      = 20
)

var ErrMalformedBitfield = errors.New("malformed bitfield")

type Message struct {
	ID      uint8
	Payload []byte
}

func ReadMessage(r io.Reader) (*Message, error) {
	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(r, lengthBuf)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBuf)
	if length == 0 {
		return nil, nil
	}
	messageBuf := make([]byte, length)
	_, err = io.ReadFull(r, messageBuf)
	if err != nil {
		return nil, err
	}

	msg := &Message{
		ID:      messageBuf[0],
		Payload: messageBuf[1:],
	}
	return msg, nil
}

func (m *Message) Serialize() []byte {
	length := uint32(len(m.Payload) + 1)
	buf := make([]byte, 4+length)

	binary.BigEndian.PutUint32(buf[0:4], length)
	buf[4] = m.ID
	copy(buf[5:], m.Payload)
	return buf
}

func ValidateBitfield(payload []byte, pieceCount int) error {
	if pieceCount < 0 {
		return fmt.Errorf("%w: negative piece count", ErrMalformedBitfield)
	}
	expectedLen := (pieceCount + 7) / 8
	if len(payload) != expectedLen {
		return fmt.Errorf("%w: got %d bytes, want %d", ErrMalformedBitfield, len(payload), expectedLen)
	}
	if pieceCount == 0 || pieceCount%8 == 0 {
		return nil
	}

	spareBits := 8 - (pieceCount % 8)
	mask := byte((1 << spareBits) - 1)
	if payload[len(payload)-1]&mask != 0 {
		return fmt.Errorf("%w: spare bits set", ErrMalformedBitfield)
	}
	return nil
}
