package piece

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/config"
	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/util"
)

var ErrInvalidPieceBlock = errors.New("invalid piece block")

func DownloadPiece(
	client *peer.Client,
	pieceIndex int,
	pieceLength int,
	rl *util.RateLimiter,
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

	maxOutstanding := config.GlobalConfig.PipelineQueueSize
	if maxOutstanding <= 0 {
		maxOutstanding = 8
	}

	// Function to queue up missing requests up to the maxOutstanding limit
	queueRequests := func() error {
		for {
			outstanding := 0
			for i := 0; i < assembler.TotalBlocks; i++ {
				if assembler.Requested[i] && !assembler.Received[i] {
					outstanding++
				}
			}
			if outstanding >= maxOutstanding {
				break
			}
			offset, length, ok := assembler.NextMissingBlock()
			if !ok {
				break
			}
			if rl != nil {
				rl.Wait(int64(length))
			}
			req := RequestMessage(pieceIndex, offset, length)
			err := client.WriteMessage(req)
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Send initial batch of requests
	if err := queueRequests(); err != nil {
		return nil, err
	}

	for !assembler.IsComplete() {
		_ = client.Conn.SetReadDeadline(time.Now().Add(45 * time.Second))
		msg, err := peer.ReadMessage(
			client.Conn,
		)
		_ = client.Conn.SetReadDeadline(time.Time{})

		if err != nil {
			client.State.TotalDownloads++
			// Reset all outstanding requested blocks on error so they can be rescheduled
			for i := 0; i < assembler.TotalBlocks; i++ {
				if assembler.Requested[i] && !assembler.Received[i] {
					assembler.ResetBlock(i * BlockSize)
				}
			}
			return nil, err
		}

		if msg == nil {
			continue
		}

		switch msg.ID {

		case peer.MsgChoke:
			client.State.Choked = true
			client.State.TotalDownloads++
			// Reset all outstanding requested blocks
			for i := 0; i < assembler.TotalBlocks; i++ {
				if assembler.Requested[i] && !assembler.Received[i] {
					assembler.ResetBlock(i * BlockSize)
				}
			}
			return nil, fmt.Errorf(
				"peer choked during download",
			)

		case peer.MsgUnchoke:
			client.State.Choked = false
			fmt.Printf("[STAGE] peer.Connect: unchoked by peer %s\n", client.Conn.RemoteAddr())
			if err := queueRequests(); err != nil {
				return nil, err
			}

		case peer.MsgHave:
			err := client.HandleHave(msg)
			if err != nil {
				return nil, err
			}

		case peer.MsgExtended:
			_ = client.HandleExtended(msg, client.PeerChan)

		case peer.MsgPiece:
			receivedOffset, block, err := ParsePiece(msg)
			if err != nil {
				client.State.TotalDownloads++
				return nil, err
			}

			if receivedOffset%BlockSize != 0 {
				client.State.TotalDownloads++
				return nil, fmt.Errorf("%w: offset %d is not aligned to BlockSize", ErrInvalidPieceBlock, receivedOffset)
			}

			err = assembler.AddBlock(
				receivedOffset,
				block,
			)
			if err != nil {
				client.State.TotalDownloads++
				return nil, fmt.Errorf("%w: %v", ErrInvalidPieceBlock, err)
			}


			// Update score stats upon successful block download
			client.State.SuccessfulDownloads++
			client.State.TotalDownloads++
			client.State.IntervalBytes += int64(len(block))
			client.State.LastActive = time.Now()

			fmt.Printf(
				"[STAGE] DownloadPiece: received block (offset %d, size %d) from peer %s\n",
				receivedOffset,
				len(block),
				client.Conn.RemoteAddr(),
			)

			// Pipeline next blocks
			if err := queueRequests(); err != nil {
				return nil, err
			}
		}
	}


	return assembler.Data, nil
}

func waitForUnchoke(
	client *peer.Client,
) error {

	for client.State.Choked {
		if client.Conn.RemoteAddr().Network() != "pipe" {
			_ = client.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		}
		msg, err := peer.ReadMessage(
			client.Conn,
		)
		if client.Conn.RemoteAddr().Network() != "pipe" {
			_ = client.Conn.SetReadDeadline(time.Time{})
		}

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
