package piece

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
)

const BlockSize = 16384

var ErrInvalidPieceBlock = errors.New("invalid piece block")

func DownloadPiece(client *peer.Client, pieceIndex int, pieceLength int) ([]byte, error) {
	if client.State.Choked {
		if err := client.SendInterested(); err != nil {
			return nil, err
		}
		if err := waitForUnchoke(client); err != nil {
			return nil, err
		}
	}

	buffer := make([]byte, pieceLength)
	downloaded := 0
	for downloaded < pieceLength {
		blockSize := BlockSize
		if pieceLength-downloaded < blockSize {
			blockSize = pieceLength - downloaded
		}
		req := RequestMessage(pieceIndex, downloaded, blockSize)
		_, err := client.Conn.Write(req.Serialize())
		if err != nil {
			return nil, err
		}

		msg, err := peer.ReadMessage(client.Conn)
		if err != nil {
			return nil, err
		}
		if msg == nil {
			continue
		}
		if msg.ID != peer.MsgPiece {
			continue
		}
		if len(msg.Payload) < 8 {
			return nil, fmt.Errorf("%w: piece payload too short", ErrInvalidPieceBlock)
		}
		begin := binary.BigEndian.Uint32(msg.Payload[4:8])
		block := msg.Payload[8:]
		if int(begin) != downloaded {
			return nil, fmt.Errorf("%w: got offset %d, want %d", ErrInvalidPieceBlock, begin, downloaded)
		}
		if int(begin)+len(block) > len(buffer) {
			return nil, fmt.Errorf("%w: block exceeds piece length", ErrInvalidPieceBlock)
		}
		copy(buffer[begin:], block)
		downloaded += len(block)
		fmt.Printf("downloaded %d/%d bytes\n", downloaded, pieceLength)
	}
	return buffer, nil
}

func waitForUnchoke(client *peer.Client) error {
	for client.State.Choked {
		msg, err := peer.ReadMessage(client.Conn)
		if err != nil {
			return err
		}
		if msg == nil {
			continue
		}
		switch msg.ID {
		case peer.MsgUnchoke:
			client.State.Choked = false
		case peer.MsgChoke:
			client.State.Choked = true
		}
	}
	return nil
}

func RequestMessage(pieceIndex, begin, length int) *peer.Message {
	requestPayload := make([]byte, 12)
	binary.BigEndian.PutUint32(requestPayload[0:4], uint32(pieceIndex))
	binary.BigEndian.PutUint32(requestPayload[4:8], uint32(begin))
	binary.BigEndian.PutUint32(requestPayload[8:12], uint32(length))
	return &peer.Message{
		ID:      peer.MsgRequest,
		Payload: requestPayload,
	}
}
