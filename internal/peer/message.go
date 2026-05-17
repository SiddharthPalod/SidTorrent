package peer

import (
	"encoding/binary"
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
)

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
