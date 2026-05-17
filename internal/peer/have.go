package peer

import (
	"encoding/binary"
	"fmt"
)

func ParseHave(payload []byte) (int, error) {
	if len(payload) != 4 {
		return 0, fmt.Errorf(
			"invalid have payload length: %d",
			len(payload),
		)
	}

	pieceIndex := binary.BigEndian.Uint32(payload)
	return int(pieceIndex), nil
}
