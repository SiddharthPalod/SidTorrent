package piece

import "sync"

type AvailabilityManager struct {
	Counts []int
	mu     sync.Mutex
}

func NewAvailabilityManager(totalPieces int) *AvailabilityManager {
	return &AvailabilityManager{
		Counts: make([]int, totalPieces),
	}
}

func (am *AvailabilityManager) AddBitField(
	bitfield []bool) {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i, has := range bitfield {
		if has {
			am.Counts[i]++
		}
	}
}

func (am *AvailabilityManager) Count(piece int) int {
	am.mu.Lock()
	defer am.mu.Unlock()
	return am.Counts[piece]
}
