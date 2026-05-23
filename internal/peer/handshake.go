package peer

import (
	"bytes"
	"fmt"
)

type Handshake struct {
	Pstr     string
	InfoHash [20]byte
	PeerID   [20]byte
}

func NewHandshake(infoHash [20]byte, peerID [20]byte) *Handshake {
	return &Handshake{
		Pstr:     "BitTorrent protocol",
		InfoHash: infoHash,
		PeerID:   peerID,
	}
}

func (h *Handshake) Serialize() []byte {
	buf := make([]byte, len(h.Pstr)+49)
	buf[0] = byte(len(h.Pstr))
	curr := 1
	curr += copy(buf[curr:], h.Pstr)
	curr += copy(buf[curr:], make([]byte, 8))
	curr += copy(buf[curr:], h.InfoHash[:])
	curr += copy(buf[curr:], h.PeerID[:])
	return buf
}

func ReadHandshake(buf []byte) (*Handshake, error) {
	pstrlen := int(buf[0])
	if len(buf) < pstrlen+49 {
		return nil, fmt.Errorf("invalid handshake")
	}
	pstr := string(buf[1 : pstrlen+1])
	var infoHash [20]byte
	copy(infoHash[:], buf[pstrlen+9:pstrlen+29])
	var peerID [20]byte
	copy(peerID[:], buf[pstrlen+29:pstrlen+49])
	return &Handshake{
		Pstr:     pstr,
		InfoHash: infoHash,
		PeerID:   peerID,
	}, nil
}

func VerifyHandshake(sent, recv *Handshake) bool {
	return bytes.Equal(sent.InfoHash[:], recv.InfoHash[:])
}
