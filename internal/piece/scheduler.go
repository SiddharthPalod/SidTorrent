package piece

import (
	"fmt"
	"sync"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

func StartScheduler(
	tf *torrent.TorrentFile,
	pm *PieceManager,
	writer *storage.Writer,
	clients []*peer.Client,
) error {

	var wg sync.WaitGroup
	errCh := make(chan error, len(clients))

	for _, client := range clients {
		wg.Add(1)

		go func(c *peer.Client) {
			defer wg.Done()
			if err := StartWorker(c, tf, pm, writer); err != nil {
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
