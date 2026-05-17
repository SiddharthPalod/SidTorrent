package piece

import "fmt"

const BlockSize = 16384

type PieceAssembler struct {
	PieceIndex  int
	PieceSize   int
	Data        []byte
	Received    []bool
	TotalBlocks int
}

func NewPieceAssembler(pieceIndex int, pieceSize int) *PieceAssembler {

	totalBlocks := (pieceSize + BlockSize - 1) / BlockSize

	return &PieceAssembler{
		PieceIndex:  pieceIndex,
		PieceSize:   pieceSize,
		Data:        make([]byte, pieceSize),
		Received:    make([]bool, totalBlocks),
		TotalBlocks: totalBlocks,
	}
}

func (pa *PieceAssembler) BlockLength(blockIndex int) int {
	begin := blockIndex * BlockSize
	remaining := pa.PieceSize - begin
	if remaining <= BlockSize {
		return remaining
	}
	return BlockSize
}

func (pa *PieceAssembler) AddBlock(offset int, block []byte) error {
	if offset < 0 || offset >= pa.PieceSize {
		return fmt.Errorf("Offset out of bounds: %d", offset)
	}

	end := offset + len(block)
	if end > pa.PieceSize {
		return fmt.Errorf("block exceeds piece size")
	}

	copy(pa.Data[offset:end], block)
	blockIndex := offset / BlockSize
	if blockIndex >= len(pa.Received) {
		return fmt.Errorf("invalid block index")
	}

	pa.Received[blockIndex] = true
	return nil
}

func (pa *PieceAssembler) IsComplete() bool {
	for _, received := range pa.Received {
		if !received {
			return false
		}
	}
	return true
}
