package piece

import (
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
)

type ChokeManager struct {
	Clients           []*peer.Client
	MaxUploads        int
	mu                sync.Mutex
	optimisticTimer   int
	currentOptimistic *peer.Client
}

func NewChokeManager(clients []*peer.Client, maxUploads int) *ChokeManager {
	return &ChokeManager{
		Clients:    clients,
		MaxUploads: maxUploads,
	}
}

func (cm *ChokeManager) Evaluate() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.optimisticTimer++
	// 1. Calculate EWMA throughput per client
	for _, client := range cm.Clients {
		elapsed := time.Since(client.State.LastActive).Seconds()
		if elapsed > 0 {
			instantSpeed := float64(client.State.IntervalBytes) / elapsed
			// EWMA: 80% history weight, 20% sample weight
			client.State.DownloadRate = (0.8 * client.State.DownloadRate) + (0.2 * instantSpeed)
			client.State.IntervalBytes = 0
			client.State.LastActive = time.Now()
		}
	}
	// 2. Identify interested candidates
	var interested []*peer.Client
	for _, client := range cm.Clients {
		if client.State.InterestedInUs {
			interested = append(interested, client)
		}
	}
	if len(interested) == 0 {
		return
	}
	// 3. Tit-for-Tat: Rank interested peers descending by score
	sort.Slice(interested, func(i, j int) bool {
		return interested[i].State.CalculateScore() > interested[j].State.CalculateScore()
	})
	activeSlots := cm.MaxUploads - 1 // 1 slot kept for Optimistic Unchoke
	if len(interested) < activeSlots {
		activeSlots = len(interested)
	}
	unchokedPeers := make(map[*peer.Client]bool)
	for i := 0; i < activeSlots; i++ {
		unchokedPeers[interested[i]] = true
	}
	// 4. Optimistic Unchoking (Pick 1 random client, rotated every 30s)
	var remaining []*peer.Client
	for _, client := range interested {
		if !unchokedPeers[client] {
			remaining = append(remaining, client)
		}
	}
	if len(remaining) > 0 {
		if cm.optimisticTimer%3 == 0 || cm.currentOptimistic == nil || !cm.currentOptimistic.State.InterestedInUs {
			cm.currentOptimistic = remaining[rand.Intn(len(remaining))]
		}
		unchokedPeers[cm.currentOptimistic] = true
	}
	// 5. Send CHOKE/UNCHOKE states
	for _, client := range cm.Clients {
		shouldUnchoke := unchokedPeers[client]
		if shouldUnchoke && client.State.ChokedByUs {
			client.State.ChokedByUs = false
			_ = client.SendUnchoke()
		} else if !shouldUnchoke && !client.State.ChokedByUs {
			client.State.ChokedByUs = true
			_ = client.SendChoke()
		}
	}
}
