package piece

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/metrics"
	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/util"
)

type SchedulerOptions struct {
	EnablePEX        bool
	EnableChoking    bool
	EnableMetrics    bool
	IncomingClients  chan *peer.Client
}

func StartScheduler(
	ctx context.Context,
	tf *torrent.TorrentFile,
	pm *PieceManager,
	writer *storage.Writer,
	clients []*peer.Client,
	rl *util.RateLimiter,
	opts SchedulerOptions,
) error {
	stopCh := make(chan struct{})
	defer close(stopCh)

	peerChan := make(chan string, 100)
	peerPoolMu := sync.Mutex{}
	peerPool := make(map[string]bool)
	var wg sync.WaitGroup
	errCh := make(chan error, 100)
	var activeClients []*peer.Client
	var activeClientsMu sync.Mutex
	completeChan := make(chan struct{}, 100)



	startPeerWorker := func(c *peer.Client) {
		if opts.EnablePEX {
			c.PeerChan = peerChan
		}
		activeClientsMu.Lock()
		activeClients = append(activeClients, c)
		if opts.EnableMetrics {
			metrics.GlobalMetrics.SetActivePeers(len(activeClients))
		}
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
				if opts.EnableMetrics {
					metrics.GlobalMetrics.SetActivePeers(len(activeClients))
				}
				activeClientsMu.Unlock()
				wg.Done()
			}()

			if opts.EnablePEX {
				if err := c.SendExtensionHandshake(); err != nil {
					fmt.Printf("[STAGE] PEX: Handshake failed to %s: %v\n", c.Conn.RemoteAddr(), err)
				}
			}
			if err := StartWorker(ctx, c, tf, pm, writer, rl, completeChan); err != nil {
				// Don't send context.Canceled as error to errCh to allow silent exit
				if err != context.Canceled {
					errCh <- err
				}
			}
		}()
	}

	for _, client := range clients {
		addr := client.Conn.RemoteAddr().String()
		peerPool[addr] = true
		startPeerWorker(client)
	}

	if opts.IncomingClients != nil {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-stopCh:
					return
				case client, ok := <-opts.IncomingClients:
					if !ok {
						return
					}
					addr := client.Conn.RemoteAddr().String()
					peerPoolMu.Lock()
					if peerPool[addr] {
						peerPoolMu.Unlock()
						client.Conn.Close()
						continue
					}
					peerPool[addr] = true
					peerPoolMu.Unlock()

					go func(c *peer.Client) {
						if err := c.ReadBitField(tf.PieceCount()); err != nil {
							c.Conn.Close()
							return
						}
						if err := c.SendInterested(); err != nil {
							c.Conn.Close()
							return
						}
						startPeerWorker(c)
					}(client)
				}
			}
		}()
	}


	if opts.EnableChoking {
		cm := NewChokeManager(clients, 4) // Max 4 upload slots (3 Tit-for-Tat + 1 Optimistic)
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					cm.Evaluate()
				case <-ctx.Done():
					return
				case <-stopCh:
					return
				}
			}
		}()
	}

	if opts.EnablePEX {
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
				case <-ctx.Done():
					return
				case <-stopCh:
					return
				}
			}
		}()
	}

	if opts.EnableMetrics {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					metrics.GlobalMetrics.Report()
				case <-ctx.Done():
					return
				case <-stopCh:
					return
				}
			}
		}()
	}

	if opts.EnablePEX {
		go func() {
			pexSem := make(chan struct{}, 5) // Limit concurrent dynamic PEX dials to 5

			for {
				select {
				case addr, ok := <-peerChan:
					if !ok {
						return
					}
					peerPoolMu.Lock()
					if peerPool[addr] {
						peerPoolMu.Unlock()
						continue
					}
					peerPool[addr] = true
					peerPoolMu.Unlock()

					select {
					case pexSem <- struct{}{}:
					case <-ctx.Done():
						return
					case <-stopCh:
						return
					}

					go func(address string) {
						defer func() { <-pexSem }()

						if pm.IsBlacklisted(address) {
							return
						}
						fmt.Printf("[STAGE] PEX Dialer: Dynamically dialing newly discovered peer %s...\n", address)

						client, err := peer.ConnectTimeout(address, tf.InfoHash, 10*time.Second)
						if err != nil {
							return
						}
						if err := client.ReadBitField(tf.PieceCount()); err != nil {
							client.Conn.Close()
							return
						}
						_ = client.SendInterested()

						select {
						case <-ctx.Done():
							client.Conn.Close()
							return
						default:
						}

						startPeerWorker(client)
					}(addr)

				case <-ctx.Done():
					return
				case <-stopCh:
					return
				}
			}
		}()
	}

	// Wait for context cancellation, error, or workers to complete
	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
	case <-completeChan:
		// Close all active connections to cancel workers immediately
		activeClientsMu.Lock()
		for _, c := range activeClients {
			_ = c.Conn.Close()
		}
		activeClientsMu.Unlock()
		wg.Wait()
	case <-ctx.Done():
		// Close all active connections to cancel startPeerWorker's read loop
		activeClientsMu.Lock()
		for _, c := range activeClients {
			_ = c.Conn.Close()
		}
		activeClientsMu.Unlock()
		wg.Wait()
	}

	close(errCh)

	var lastErr error
	for err := range errCh {
		if !isConnectionError(err) {
			lastErr = err
		}
	}
	if pm.IsComplete() {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if lastErr != nil {
		return fmt.Errorf("download stopped before completion: %w", lastErr)
	}
	return fmt.Errorf("no progress possible: no connected peer has remaining pieces")
}
