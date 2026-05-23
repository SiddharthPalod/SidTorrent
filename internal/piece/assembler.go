package piece

import (
	"fmt"
	"sync"
)

const BlockSize = 16384

type PieceAssembler struct {
	PieceIndex  int
	PieceSize   int
	Data        []byte
	Received    []bool
	Requested   []bool
	TotalBlocks int
	mu          sync.Mutex
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

func (pa *PieceAssembler) NextMissingBlock() (offset int, length int, ok bool) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	for i := 0; i < pa.TotalBlocks; i++ {
		if pa.Received[i] {
			continue
		}
		if pa.Requested[i] {
			continue
		}
		pa.Requested[i] = true
		offset = i * BlockSize
		length = pa.BlockLength(i)
		return offset, length, true
	}

	return 0, 0, false
}

func (pa *PieceAssembler) ResetBlock(offset int) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	index := offset / BlockSize

	if index >= len(pa.Requested) {
		return
	}
	pa.Requested[index] = false
}
