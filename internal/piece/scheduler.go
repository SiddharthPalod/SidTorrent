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

	// Dynamic PEX peer discovery channel
	peerChan := make(chan string, 100)
	peerPoolMu := sync.Mutex{}
	peerPool := make(map[string]bool)
	var wg sync.WaitGroup
	errCh := make(chan error, 100)
	var activeClients []*peer.Client
	var activeClientsMu sync.Mutex

	startPeerWorker := func(c *peer.Client) {
		c.PeerChan = peerChan
		activeClientsMu.Lock()
		activeClients = append(activeClients, c)
		activeClientsMu.Unlock()

		wg.Add(1)
		go func() {
			defer func() {
				activeClientsMu.Lock()
				for i, ac := range activeClients {
					if ac == c {
						activeClients = append(activeClients[:i], activeClients[i+1:]...)
						break
					}
				}
				activeClientsMu.Unlock()
				wg.Done()
			}()

			if err := c.SendExtensionHandshake(); err != nil {
				fmt.Printf("[STAGE] PEX: Handshake failed to %s: %v\n", c.Conn.RemoteAddr(), err)
			}
			if err := StartWorker(c, tf, pm, writer, rl); err != nil {
				errCh <- err
			}
		}()
	}
	for _, client := range clients {
		addr := client.Conn.RemoteAddr().String()
		peerPool[addr] = true
		startPeerWorker(client)
	}

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

	// Periodic PEX broadcast loop
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				activeClientsMu.Lock()
				var endpoints []string
				for _, c := range activeClients {
					endpoints = append(endpoints, c.Conn.RemoteAddr().String())
				}

				for _, c := range activeClients {
					if c.State.RemotePexID > 0 {
						var added []string
						selfAddr := c.Conn.RemoteAddr().String()
						for _, ep := range endpoints {
							if ep != selfAddr {
								added = append(added, ep)
							}
						}
						if len(added) > 0 {
							_ = c.SendPexMessage(added, c.State.RemotePexID)
						}
					}
				}
				activeClientsMu.Unlock()
			case <-stopCh:
				return
			}
		}
	}()
	// Reactive PEX Dialer Thread: Listens on peerChan and dials new peers on-the-fly!
	go func() {
		for {
			select {
			case addr := <-peerChan:
				peerPoolMu.Lock()
				if peerPool[addr] {
					peerPoolMu.Unlock()
					continue
				}
				peerPool[addr] = true
				peerPoolMu.Unlock()
				fmt.Printf("[STAGE] PEX Dialer: Dynamically dialing newly discovered peer %s...\n", addr)
				go func(address string) {
					if pm.IsBlacklisted(address) {
						return
					}
					client, err := peer.ConnectTimeout(address, tf.InfoHash, 10*time.Second)
					if err != nil {
						return
					}
					// Read bitfield from new peer
					if err := client.ReadBitField(tf.PieceCount()); err != nil {
						client.Conn.Close()
						return
					}
					_ = client.SendInterested()
					startPeerWorker(client)
				}(addr)
			case <-stopCh:
				return
			}
		}
	}()

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
