package piece

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"sort"
	"sync"

	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

const DefaultMaxRetries = 3

type PieceManager struct {
	TotalPieces int
	TotalLength int64

	Pending           map[int]bool
	InProgress        map[int]bool
	Completed         map[int]bool
	FailedAttempts    map[int]int
	PermanentlyFailed map[int]bool
	MaxRetries        int
	Downloaded        int64
	Assemblers        map[int]*PieceAssembler
	Availability      []int // Counts how many connected peers own each piece

	// Checksum to detect direct external mutations on pm.Availability
	availabilitySum int

	// Rarest-first buckets optimization: buckets[count] contains a set of piece indices
	buckets map[int]map[int]bool

	// Phase 7: Streaming Engine fields
	StreamingMode       bool
	StreamingWindowSize int

	// Phase 9: Blacklisted peers (Corrupt Peer Detection)
	BlacklistedPeers   map[string]bool
	BlacklistedPeersMu sync.Mutex
	mu                 sync.Mutex
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
		PermanentlyFailed:   make(map[int]bool),
		MaxRetries:          DefaultMaxRetries,
		Assemblers:          make(map[int]*PieceAssembler),
		Availability:        make([]int, totalPieces),
		buckets:             make(map[int]map[int]bool),
		StreamingMode:       true, // Enable Streaming Mode by default
		StreamingWindowSize: 15,   // Download the first 15 pieces sequentially
		BlacklistedPeers:    make(map[string]bool),
	}

	pm.buckets[0] = make(map[int]bool)
	for i := 0; i < totalPieces; i++ {
		size := tf.PieceLengthAt(i)
		pm.Assemblers[i] = NewPieceAssembler(i, int(size))
		pm.Pending[i] = true
		pm.buckets[0][i] = true
	}
	return pm
}

func (pm *PieceManager) ensureBucketsSync() {
	totalInBuckets := 0
	for _, bk := range pm.buckets {
		totalInBuckets += len(bk)
	}
	
	actualSum := 0
	for _, v := range pm.Availability {
		actualSum += v
	}

	if totalInBuckets != len(pm.Pending) || actualSum != pm.availabilitySum {
		pm.buckets = make(map[int]map[int]bool)
		for pieceIndex := range pm.Pending {
			count := pm.Availability[pieceIndex]
			if pm.buckets[count] == nil {
				pm.buckets[count] = make(map[int]bool)
			}
			pm.buckets[count][pieceIndex] = true
		}
		pm.availabilitySum = actualSum
	}
}

func (pm *PieceManager) LoadExistingState(outputPath string, tf *torrent.TorrentFile) error {
	file, err := os.Open(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No file exists yet, start fresh
		}
		return err
	}
	defer file.Close()

	fmt.Println("[STAGE] Resume: Scanning and verifying existing download file...")
	buf := make([]byte, tf.PieceLength)
	for i := 0; i < pm.TotalPieces; i++ {
		pieceLen := tf.PieceLengthAt(i)
		_, err := file.ReadAt(buf[:pieceLen], int64(i)*tf.PieceLength)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				// Rest of file hasn't been downloaded or allocated yet
				break
			}
			return err
		}
		if VerifyPiece(tf, i, buf[:pieceLen]) == nil {
			pm.MarkComplete(i, pieceLen)
			fmt.Printf("[STAGE] Resume: Piece %d verified on disk. Skipping download.\n", i)
		}
	}
	return nil
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

func (pm *PieceManager) moveBucket(pieceIndex, oldCount, newCount int) {
	if pm.buckets[oldCount] != nil {
		delete(pm.buckets[oldCount], pieceIndex)
	}
	if pm.buckets[newCount] == nil {
		pm.buckets[newCount] = make(map[int]bool)
	}
	pm.buckets[newCount][pieceIndex] = true
}

func (pm *PieceManager) claimPiece(pieceIndex int) {
	delete(pm.Pending, pieceIndex)
	pm.InProgress[pieceIndex] = true

	count := pm.Availability[pieceIndex]
	if pm.buckets[count] != nil {
		delete(pm.buckets[count], pieceIndex)
	}
}

func (pm *PieceManager) RegisterPeerBitfield(bf []bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for i, has := range bf {
		if has && i < pm.TotalPieces {
			oldCount := pm.Availability[i]
			newCount := oldCount + 1
			pm.Availability[i] = newCount
			pm.availabilitySum++
			if pm.Pending[i] {
				pm.moveBucket(i, oldCount, newCount)
			}
		}
	}
}

func (pm *PieceManager) UnregisterPeerBitfield(bf []bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for i, has := range bf {
		if has && i < pm.TotalPieces {
			oldCount := pm.Availability[i]
			newCount := oldCount - 1
			if newCount < 0 {
				newCount = 0
			}
			pm.Availability[i] = newCount
			if oldCount > 0 {
				pm.availabilitySum--
				if pm.availabilitySum < 0 {
					pm.availabilitySum = 0
				}
			}
			if pm.Pending[i] {
				pm.moveBucket(i, oldCount, newCount)
			}
		}
	}
}

func (pm *PieceManager) IncrementAvailability(pieceIndex int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pieceIndex >= 0 && pieceIndex < pm.TotalPieces {
		oldCount := pm.Availability[pieceIndex]
		newCount := oldCount + 1
		pm.Availability[pieceIndex] = newCount
		pm.availabilitySum++
		if pm.Pending[pieceIndex] {
			pm.moveBucket(pieceIndex, oldCount, newCount)
		}
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

	// Ensure our buckets are synchronized in case of manual availability/pending modifications in tests
	pm.ensureBucketsSync()

	// 1. Prioritize sequential downloading of the first N pieces
	for i := 0; i < pm.StreamingWindowSize && i < pm.TotalPieces; i++ {
		if pm.Pending[i] {
			if i < len(available) && available[i] {
				pm.claimPiece(i)
				fmt.Printf("[STAGE] Streaming Engine: Prioritizing sequential piece %d over rarest-first\n", i)
				return i, nil
			}
		}
	}

	// 2. Fallback to optimized Rarest-First using buckets
	var counts []int
	for c, bk := range pm.buckets {
		if len(bk) > 0 {
			counts = append(counts, c)
		}
	}
	sort.Ints(counts)

	for _, c := range counts {
		var candidates []int
		for idx := range pm.buckets[c] {
			if idx < len(available) && available[idx] {
				candidates = append(candidates, idx)
			}
		}
		if len(candidates) > 0 {
			pieceIndex := candidates[rand.Intn(len(candidates))]
			pm.claimPiece(pieceIndex)
			return pieceIndex, nil
		}
	}

	return -1, fmt.Errorf("no available pieces matching this peer")
}

func (pm *PieceManager) NextRarestPiece(available []bool) (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Ensure our buckets are synchronized in case of manual availability/pending modifications in tests
	pm.ensureBucketsSync()

	var counts []int
	for c, bk := range pm.buckets {
		if len(bk) > 0 {
			counts = append(counts, c)
		}
	}
	sort.Ints(counts)

	for _, c := range counts {
		var candidates []int
		for idx := range pm.buckets[c] {
			if idx < len(available) && available[idx] {
				candidates = append(candidates, idx)
			}
		}
		if len(candidates) > 0 {
			pieceIndex := candidates[rand.Intn(len(candidates))]
			pm.claimPiece(pieceIndex)
			return pieceIndex, nil
		}
	}

	return -1, fmt.Errorf("no available pieces matching this peer")
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
	pm.claimPiece(pieceIndex)
	return pieceIndex, nil
}

func (pm *PieceManager) MarkComplete(pieceIndex int, bytes int64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.InProgress, pieceIndex)
	pm.Completed[pieceIndex] = true
	pm.Downloaded += bytes

	// Ensure it is removed from buckets
	count := pm.Availability[pieceIndex]
	if pm.buckets[count] != nil {
		delete(pm.buckets[count], pieceIndex)
	}
}

func (pm *PieceManager) MarkFailed(pieceIndex int) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.InProgress, pieceIndex)
	pm.FailedAttempts[pieceIndex]++
	if pm.FailedAttempts[pieceIndex] > pm.MaxRetries {
		pm.PermanentlyFailed[pieceIndex] = true
		return false
	}
	pm.Pending[pieceIndex] = true
	
	// Add back to bucket
	count := pm.Availability[pieceIndex]
	if pm.buckets[count] == nil {
		pm.buckets[count] = make(map[int]bool)
	}
	pm.buckets[count][pieceIndex] = true
	return true
}

func (pm *PieceManager) IsComplete() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if len(pm.PermanentlyFailed) > 0 {
		return false // Cannot complete if pieces permanently failed
	}
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
