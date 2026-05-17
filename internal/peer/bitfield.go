package peer

import (
	"fmt"
)

func ParseBitField(data []byte, totalPieces int) ([]bool, error) {
	expectedBytes := (totalPieces + 7) / 8
	if len(data) != expectedBytes {
		return nil, fmt.Errorf(
			"invalid bitfield length: got=%d expected=%d",
			len(data),
			expectedBytes,
		)
	}

	bitfield := make([]bool, totalPieces)

	for i := 0; i < totalPieces; i++ {
		byteindex := i / 8
		offset := 7 - (i % 8)
		hasPiece := (data[byteindex] & (1 << offset)) != 0
		bitfield[i] = hasPiece
	}
	return bitfield, nil
}
