package piece

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
)

var ErrInvalidPieceBlock = errors.New("invalid piece block")

func DownloadPiece(
	client *peer.Client,
	pieceIndex int,
	pieceLength int,
) ([]byte, error) {

	// wait until peer unchokes us
	if client.State.Choked {

		if err := client.SendInterested(); err != nil {
			return nil, err
		}

		if err := waitForUnchoke(client); err != nil {
			return nil, err
		}
	}

	assembler := NewPieceAssembler(
		pieceIndex,
		pieceLength,
	)

	for blockIndex := 0; blockIndex < assembler.TotalBlocks; blockIndex++ {

		offset := blockIndex * BlockSize

		length := assembler.BlockLength(
			blockIndex,
		)

		req := RequestMessage(
			pieceIndex,
			offset,
			length,
		)

		_, err := client.Conn.Write(
			req.Serialize(),
		)

		if err != nil {
			return nil, err
		}

		for {

			msg, err := peer.ReadMessage(
				client.Conn,
			)

			if err != nil {
				return nil, err
			}

			if msg == nil {
				continue
			}

			switch msg.ID {

			case peer.MsgChoke:

				client.State.Choked = true

				return nil, fmt.Errorf(
					"peer choked during download",
				)

			case peer.MsgUnchoke:

				client.State.Choked = false

				continue

			case peer.MsgHave:

				err := client.HandleHave(msg)

				if err != nil {
					return nil, err
				}

				continue

			case peer.MsgPiece:

				receivedOffset,
					block,
					err := ParsePiece(msg)

				if err != nil {
					return nil, err
				}

				if receivedOffset != offset {

					return nil, fmt.Errorf(
						"%w: got offset %d want %d",
						ErrInvalidPieceBlock,
						receivedOffset,
						offset,
					)
				}

				err = assembler.AddBlock(
					receivedOffset,
					block,
				)

				if err != nil {
					return nil, err
				}

				fmt.Printf(
					"received block %d/%d (%d bytes)\n",
					blockIndex+1,
					assembler.TotalBlocks,
					len(block),
				)

				goto nextBlock
			}
		}

	nextBlock:
	}

	if !assembler.IsComplete() {

		return nil, fmt.Errorf(
			"piece incomplete",
		)
	}

	return assembler.Data, nil
}

func waitForUnchoke(
	client *peer.Client,
) error {

	for client.State.Choked {

		msg, err := peer.ReadMessage(
			client.Conn,
		)

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

		case peer.MsgHave:

			err := client.HandleHave(msg)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func RequestMessage(
	pieceIndex,
	begin,
	length int,
) *peer.Message {

	requestPayload := make([]byte, 12)

	binary.BigEndian.PutUint32(
		requestPayload[0:4],
		uint32(pieceIndex),
	)

	binary.BigEndian.PutUint32(
		requestPayload[4:8],
		uint32(begin),
	)

	binary.BigEndian.PutUint32(
		requestPayload[8:12],
		uint32(length),
	)

	return &peer.Message{
		ID:      peer.MsgRequest,
		Payload: requestPayload,
	}
}
