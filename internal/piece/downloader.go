package piece

import (
	"encoding/binary"
	"fmt"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
)

const BlockSize = 16384

func DownloadPiece(client *peer.Client, pieceIndex int, pieceLength int) ([]byte, error) {
	buffer := make([]byte, pieceLength)
	downloaded := 0
	for downloaded < pieceLength {
		blockSize := BlockSize
		if pieceLength-downloaded < blockSize {
			blockSize = pieceLength - downloaded
		}
		requestPayload := make([]byte, 12)

		binary.BigEndian.PutUint32(requestPayload[0:4], uint32(pieceIndex))
		binary.BigEndian.PutUint32(requestPayload[4:8], uint32(downloaded))
		binary.BigEndian.PutUint32(requestPayload[8:12], uint32(blockSize))
		req := &peer.Message{
			ID:      peer.MsgRequest,
			Payload: requestPayload,
		}
		_, err := client.Conn.Write(req.Serialize())
		if err != nil {
			return nil, err
		}

		msg, err := peer.ReadMessage(client.Conn)
		if err != nil {
			return nil, err
		}
		if msg.ID != peer.MsgPiece {
			continue
		}
		begin := binary.BigEndian.Uint32(msg.Payload[4:8])
		copy(buffer[begin:], msg.Payload[8:])
		downloaded += len(msg.Payload[8:])
		fmt.Printf("downloaded %d/%d bytes\n", downloaded, pieceLength)
	}
	return buffer, nil
}
