package piece

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

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
		fmt.Printf("[STAGE] DownloadPiece: waiting for unchoke from peer %s\n", client.Conn.RemoteAddr())
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

	fmt.Printf("[STAGE] DownloadPiece: starting download for piece %d (%d blocks, %d bytes) from peer %s\n",
		pieceIndex, assembler.TotalBlocks, pieceLength, client.Conn.RemoteAddr())

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

		fmt.Printf("[STAGE] DownloadPiece: requesting block %d/%d (offset %d, len %d) from peer %s\n",
			blockIndex+1, assembler.TotalBlocks, offset, length, client.Conn.RemoteAddr())

		_, err := client.Conn.Write(
			req.Serialize(),
		)

		if err != nil {
			return nil, err
		}

		for {
			_ = client.Conn.SetReadDeadline(time.Now().Add(45 * time.Second))
			msg, err := peer.ReadMessage(
				client.Conn,
			)
			_ = client.Conn.SetReadDeadline(time.Time{})

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
				fmt.Printf("[STAGE] peer.Connect: unchoked by peer %s\n", client.Conn.RemoteAddr())

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

				if blockIndex == 0 && receivedOffset == 0 {
					fmt.Printf("[STAGE] peer.Connect: first block received from peer %s (piece %d, offset %d, size %d)\n",
						client.Conn.RemoteAddr(), pieceIndex, receivedOffset, len(block))
				}

				fmt.Printf(
					"[STAGE] DownloadPiece: received block %d/%d (%d bytes) from peer %s\n",
					blockIndex+1,
					assembler.TotalBlocks,
					len(block),
					client.Conn.RemoteAddr(),
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
			fmt.Printf("[STAGE] peer.Connect: unchoked by peer %s\n", client.Conn.RemoteAddr())

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
