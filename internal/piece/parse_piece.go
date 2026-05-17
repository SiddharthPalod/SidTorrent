package piece

import (
	"encoding/binary"
	"fmt"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
)

func ParsePiece(
	msg *peer.Message,
) (
	offset int,
	block []byte,
	err error,
) {

	if msg == nil {

		return 0,
			nil,
			fmt.Errorf(
				"nil message",
			)
	}

	if msg.ID != peer.MsgPiece {

		return 0,
			nil,
			fmt.Errorf(
				"not a piece message",
			)
	}

	// piece payload:
	// [piece index(4)][begin(4)][block]

	if len(msg.Payload) < 8 {

		return 0,
			nil,
			fmt.Errorf(
				"%w: payload too short",
				ErrInvalidPieceBlock,
			)
	}

	pieceIndex := int(
		binary.BigEndian.Uint32(
			msg.Payload[0:4],
		),
	)

	_ = pieceIndex
	// useful later for validation

	offset = int(
		binary.BigEndian.Uint32(
			msg.Payload[4:8],
		),
	)

	block = msg.Payload[8:]

	if len(block) == 0 {

		return 0,
			nil,
			fmt.Errorf(
				"%w: empty block",
				ErrInvalidPieceBlock,
			)
	}

	return offset, block, nil
}
