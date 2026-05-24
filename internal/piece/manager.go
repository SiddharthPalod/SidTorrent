package piece

import (
	"fmt"
	"math/rand"
	"net"
	"sync"

	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

const DefaultMaxRetries = 3

type PieceManager struct {
	TotalPieces int
	TotalLength int64

	Pending        map[int]bool
	InProgress     map[int]bool
	Completed      map[int]bool
	FailedAttempts map[int]int
	MaxRetries     int
	Downloaded     int64
	Assemblers     map[int]*PieceAssembler
	mu             sync.Mutex
	Availability   []int // Counts how many connected peers own each piece

	// Phase 7: Streaming Engine fields
	StreamingMode       bool
	StreamingWindowSize int

	// Phase 9: Blacklisted peers (Corrupt Peer Detection)
	BlacklistedPeers   map[string]bool
	BlacklistedPeersMu sync.Mutex
}

func NewPieceManager(tf *torrent.TorrentFile) *PieceManager {
	totalPieces := tf.PieceCount()
	pm := &PieceManager{
		TotalPieces:         totalPieces,
		TotalLength:         tf.Length,
		Pending:             make(map[int]bool),
		InProgress:          make(map[int]bool),
		Completed:           make(map[int]bool),
		FailedAttempts:      make(map[int]int),
		MaxRetries:          DefaultMaxRetries,
		Assemblers:          make(map[int]*PieceAssembler),
		Availability:        make([]int, totalPieces), // Allocate counts slice
		StreamingMode:       true,                     // Enable Streaming Mode by default
		StreamingWindowSize: 15,                       // Download the first 15 pieces sequentially
		BlacklistedPeers:    make(map[string]bool),
	}
	for i := 0; i < totalPieces; i++ {
		size := tf.PieceLengthAt(i)
		pm.Assemblers[i] = NewPieceAssembler(i, int(size))
		pm.Pending[i] = true
	}
	return pm
}

func (pm *PieceManager) BlacklistPeer(address string) {
	pm.BlacklistedPeersMu.Lock()
	defer pm.BlacklistedPeersMu.Unlock()

	// Strip port to blacklist the whole IP address
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	pm.BlacklistedPeers[host] = true
	fmt.Printf("[STAGE] Blacklist: Banned IP %s from future connections (corrupt data detected)\n", host)
}

func (pm *PieceManager) IsBlacklisted(address string) bool {
	pm.BlacklistedPeersMu.Lock()
	defer pm.BlacklistedPeersMu.Unlock()

	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	return pm.BlacklistedPeers[host]
}
func (pm *PieceManager) NextBlock(pieceIndex int) (offset int, length int, ok bool) {
	assembler, exists := pm.Assemblers[pieceIndex]
	if !exists {
		return 0, 0, false
	}
	return assembler.NextMissingBlock()
}
func (pm *PieceManager) RegisterPeerBitfield(bf []bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for i, has := range bf {
		if has && i < pm.TotalPieces {
			pm.Availability[i]++
		}
	}
}
func (pm *PieceManager) UnregisterPeerBitfield(bf []bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for i, has := range bf {
		if has && i < pm.TotalPieces {
			pm.Availability[i]--
			if pm.Availability[i] < 0 {
				pm.Availability[i] = 0
			}
		}
	}
}
func (pm *PieceManager) IncrementAvailability(pieceIndex int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pieceIndex >= 0 && pieceIndex < pm.TotalPieces {
		pm.Availability[pieceIndex]++
	}
}
func (pm *PieceManager) NextPiece(available []bool) (int, error) {
	if pm.StreamingMode {
		return pm.NextStreamingPiece(available)
	}
	return pm.NextRarestPiece(available)
}

func (pm *PieceManager) NextStreamingPiece(available []bool) (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 1. Prioritize sequential downloading of the first N pieces
	for i := 0; i < pm.StreamingWindowSize && i < pm.TotalPieces; i++ {
		if pm.Pending[i] {
			if i < len(available) && available[i] {
				delete(pm.Pending, i)
				pm.InProgress[i] = true
				fmt.Printf("[STAGE] Streaming Engine: Prioritizing sequential piece %d over rarest-first\n", i)
				return i, nil
			}
		}
	}

	// 2. Fallback to Rarest-First for the remaining pieces
	var candidates []int
	minCount := int(^uint(0) >> 1) // max int
	for pieceIndex := range pm.Pending {
		if pieceIndex >= len(available) || !available[pieceIndex] {
			continue
		}
		count := pm.Availability[pieceIndex]
		if count < minCount {
			minCount = count
			candidates = []int{pieceIndex}
		} else if count == minCount {
			candidates = append(candidates, pieceIndex)
		}
	}
	if len(candidates) == 0 {
		return -1, fmt.Errorf("no available pieces matching this peer")
	}
	// Pick a rarest piece randomly (tie-breaker)
	pieceIndex := candidates[rand.Intn(len(candidates))]
	delete(pm.Pending, pieceIndex)
	pm.InProgress[pieceIndex] = true
	return pieceIndex, nil
}
func (pm *PieceManager) NextRarestPiece(available []bool) (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	var candidates []int
	minCount := int(^uint(0) >> 1) // max int
	for pieceIndex := range pm.Pending {
		if pieceIndex >= len(available) || !available[pieceIndex] {
			continue
		}
		count := pm.Availability[pieceIndex]
		if count < minCount {
			minCount = count
			candidates = []int{pieceIndex}
		} else if count == minCount {
			candidates = append(candidates, pieceIndex)
		}
	}
	if len(candidates) == 0 {
		return -1, fmt.Errorf("no available pieces matching this peer")
	}
	// Pick a rarest piece randomly (tie-breaker)
	pieceIndex := candidates[rand.Intn(len(candidates))]
	delete(pm.Pending, pieceIndex)
	pm.InProgress[pieceIndex] = true
	return pieceIndex, nil
}
func (pm *PieceManager) NextRandomPiece(available []bool) (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	var candidates []int
	for pieceIndex := range pm.Pending {
		if pieceIndex >= len(available) || !available[pieceIndex] {
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
func (pm *PieceManager) MarkComplete(pieceIndex int, bytes int64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.InProgress, pieceIndex)
	pm.Completed[pieceIndex] = true
	pm.Downloaded += bytes
}
func (pm *PieceManager) MarkFailed(pieceIndex int) bool {
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
