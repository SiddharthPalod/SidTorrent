package piece

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

const DefaultMaxRetries = 3

type PieceManager struct {

	// torrent metadata
	TotalPieces int
	TotalLength int64

	// scheduling state
	Pending        map[int]bool
	InProgress     map[int]bool
	Completed      map[int]bool
	FailedAttempts map[int]int
	MaxRetries     int
	Downloaded     int64

	mu sync.Mutex
}

func NewPieceManager(tf *torrent.TorrentFile) *PieceManager {
	totalPieces := tf.PieceCount()

	pm := &PieceManager{
		TotalPieces:    totalPieces,
		TotalLength:    tf.Length,
		Pending:        make(map[int]bool),
		InProgress:     make(map[int]bool),
		Completed:      make(map[int]bool),
		FailedAttempts: make(map[int]int),
		MaxRetries:     DefaultMaxRetries,
	}

	for i := 0; i < totalPieces; i++ {
		pm.Pending[i] = true
	}
	return pm
}

func (pm *PieceManager) NextPiece(
	available []bool,
) (int, error) {
	return pm.NextRandomPiece(available)
}

func (pm *PieceManager) NextRandomPiece(
	available []bool,
) (int, error) {

	pm.mu.Lock()
	defer pm.mu.Unlock()

	var candidates []int
	for pieceIndex := range pm.Pending {

		// peer doesn't have piece
		if pieceIndex >= len(available) ||
			!available[pieceIndex] {

			continue
		}
		candidates = append(candidates, pieceIndex)
	}

	if len(candidates) == 0 {
		return -1, fmt.Errorf("no available pieces")
	}

	pieceIndex := candidates[rand.Intn(len(candidates))]
	delete(pm.Pending, pieceIndex)
	pm.InProgress[pieceIndex] = true

	return pieceIndex, nil
}

func (pm *PieceManager) MarkComplete(
	pieceIndex int,
	bytes int64,
) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.InProgress, pieceIndex)
	pm.Completed[pieceIndex] = true
	pm.Downloaded += bytes
}

func (pm *PieceManager) MarkFailed(
	pieceIndex int,
) bool {

	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.InProgress, pieceIndex)
	pm.FailedAttempts[pieceIndex]++
	if pm.FailedAttempts[pieceIndex] > pm.MaxRetries {
		return false
	}
	pm.Pending[pieceIndex] = true
	return true
}

func (pm *PieceManager) IsComplete() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return len(pm.Completed) == pm.TotalPieces
}

func (pm *PieceManager) Stats() (pending int, inProgress int, completed int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return len(pm.Pending), len(pm.InProgress), len(pm.Completed)
}

func (pm *PieceManager) Progress() (downloaded int64, total int64, percent float64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.TotalLength == 0 {
		return pm.Downloaded, pm.TotalLength, 0
	}
	return pm.Downloaded, pm.TotalLength, float64(pm.Downloaded) / float64(pm.TotalLength) * 100
}
