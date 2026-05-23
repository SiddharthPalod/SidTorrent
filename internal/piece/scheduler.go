package piece

import (
	"fmt"
	"sync"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/util"
)

func StartScheduler(
	tf *torrent.TorrentFile,
	pm *PieceManager,
	writer *storage.Writer,
	clients []*peer.Client,
	rl *util.RateLimiter,
) error {
	cm := NewChokeManager(clients, 4) // Max 4 upload slots (3 Tit-for-Tat + 1 Optimistic)
	stopCh := make(chan struct{})
	defer close(stopCh)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cm.Evaluate()
			case <-stopCh:
				return
			}
		}
	}()

	var wg sync.WaitGroup
	errCh := make(chan error, len(clients))

	for _, client := range clients {
		wg.Add(1)

		go func(c *peer.Client) {
			defer wg.Done()
			if err := StartWorker(c, tf, pm, writer, rl); err != nil {
				errCh <- err
			}
		}(client)
	}
	wg.Wait()
	close(errCh)

	var lastErr error
	for err := range errCh {
		lastErr = err
	}
	if pm.IsComplete() {
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("download stopped before completion: %w", lastErr)
	}
	return fmt.Errorf("no progress possible: no connected peer has remaining pieces")
}
