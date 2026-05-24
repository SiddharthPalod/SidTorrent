package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/dht"
	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/piece"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/tracker"
	"github.com/SiddharthPalod/SidTorrent/internal/util"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var outputPath string
	flag.StringVar(&outputPath, "out", "", "output file path")
	var maxDownloadKB int64
	flag.Int64Var(&maxDownloadKB, "max-download", 0, "maximum download rate in KB/s (0 for unlimited)")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: siddtorrent [-out output-file] <torrent-file>\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		return errors.New("missing torrent file")
	}

	torrentPath := flag.Arg(0)
	tf, err := torrent.Open(
		torrentPath,
	)

	if err != nil {
		return fmt.Errorf("load torrent: %w", err)
	}

	fmt.Println(
		"torrent loaded:",
		tf.Name,
	)

	// DEBUG TRACKERS
	fmt.Println("trackers:")
	for i, tier := range tf.Trackers {
		fmt.Printf(
			"tier %d:\n",
			i,
		)

		for _, tr := range tier {

			fmt.Println("  ", tr)
		}
	}

	// Initialize and bootstrap the Kademlia DHT Node for trackerless discovery
	var dhtNode *dht.DHTNode
	dhtNode, err = dht.NewDHTNode(6881)
	if err != nil {
		fmt.Printf("[STAGE] Main: Failed to bind DHT on port 6881 (%v), attempting dynamic port...\n", err)
		dhtNode, err = dht.NewDHTNode(0)
	}

	if err == nil {
		defer dhtNode.Close()
		fmt.Printf("[STAGE] Main: DHT Node successfully initialized (ID: %x). Bootstrapping...\n", dhtNode.ID)
		dhtNode.Bootstrap()
	} else {
		fmt.Printf("[STAGE] Main: Failed to initialize DHT Node: %v\n", err)
	}

	// Fetch peers from trackers and DHT, merging uniquely
	var peers []tracker.Peer
	seenPeers := make(map[string]bool)

	addPeer := func(p tracker.Peer) {
		key := fmt.Sprintf("%s:%d", p.IP.String(), p.Port)
		if !seenPeers[key] {
			seenPeers[key] = true
			peers = append(peers, p)
		}
	}

	// 1. Fetch from trackers
	trackerPeers, trackerErr := tracker.GetPeers(tf)
	if trackerErr == nil {
		for _, p := range trackerPeers {
			addPeer(p)
		}
		fmt.Printf("[STAGE] Main: Trackers returned %d unique peers\n", len(trackerPeers))
	} else {
		fmt.Printf("[STAGE] Main: Tracker discovery failed/warned: %v\n", trackerErr)
	}

	// 2. Fetch from Kademlia DHT
	if dhtNode != nil {
		fmt.Println("[STAGE] Main: Querying Kademlia DHT for trackerless peers...")
		dhtPeers := dhtNode.SearchPeers(tf.InfoHash)
		for _, p := range dhtPeers {
			addPeer(p)
		}
		fmt.Printf("[STAGE] Main: Kademlia DHT returned %d unique peers\n", len(dhtPeers))
	}

	fmt.Println("total unique peer count:", len(peers))
	if len(peers) == 0 {
		if trackerErr != nil {
			return fmt.Errorf("no usable peers found (tracker error: %w)", trackerErr)
		}
		return errors.New("no usable peers found")
	}

	// piece manager
	pm := piece.NewPieceManager(tf)

	pending,
		inProgress,
		completed := pm.Stats()

	fmt.Printf(
		"piece manager => pending=%d inprogress=%d completed=%d\n",
		pending,
		inProgress,
		completed,
	)

	if outputPath == "" {
		outputPath = filepath.Join("downloads", tf.Name)
	}

	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		err = os.MkdirAll(
			outputDir,
			os.ModePerm,
		)
		if err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	writer, err := storage.NewWriter(
		tf,
		outputPath,
	)

	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer writer.Close()

	// connect multiple peers concurrently
	var clients []*peer.Client
	var mu sync.Mutex
	totalPieces := tf.PieceCount()

	var wg sync.WaitGroup
	// Limit maximum concurrent connection attempts to 20 to avoid descriptor exhaustion
	sem := make(chan struct{}, 20)

	for _, p := range peers {
		mu.Lock()
		if len(clients) >= 10 {
			mu.Unlock()
			break
		}
		mu.Unlock()

		wg.Add(1)
		go func(ip net.IP, port uint16) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// Check again if we already have enough before dialing
			mu.Lock()
			if len(clients) >= 10 {
				mu.Unlock()
				return
			}
			mu.Unlock()

			address := net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port))
			fmt.Printf("[STAGE] Main: Attempting to connect to peer %s...\n", address)

			client, err := peer.ConnectTimeout(address, tf.InfoHash, 15*time.Second)
			if err != nil {
				fmt.Printf("[STAGE] Main: Connection or handshake failed with peer %s: %v\n", address, err)
				return
			}
			fmt.Printf("[STAGE] Main: Handshake successful with peer %s\n", address)

			err = client.ReadBitField(totalPieces)
			if err != nil {
				fmt.Printf("[STAGE] Main: Bitfield read failed from peer %s: %v\n", address, err)
				_ = client.Conn.Close()
				return
			}
			fmt.Printf("[STAGE] Main: Bitfield successfully read from peer %s\n", address)

			err = client.SendInterested()
			if err != nil {
				fmt.Printf("[STAGE] Main: Sending 'interested' failed to peer %s: %v\n", address, err)
				_ = client.Conn.Close()
				return
			}
			fmt.Printf("[STAGE] Main: Sent 'interested' message to peer %s\n", address)

			mu.Lock()
			if len(clients) < 10 {
				clients = append(clients, client)
				fmt.Printf("[STAGE] Main: Peer %s successfully added to pool! (Total active: %d)\n", address, len(clients))
			} else {
				// We already reached 10, close this one
				_ = client.Conn.Close()
			}
			mu.Unlock()
		}(p.IP, p.Port)
	}

	wg.Wait()

	fmt.Printf("[STAGE] Main: Connection phase complete. Total active usable peers: %d\n", len(clients))
	if len(clients) == 0 {
		return errors.New("no usable peers found")
	}
	fmt.Printf("[STAGE] Main: Confirmed at least ONE successful peer connection! (Total: %d)\n", len(clients))

	var rl *util.RateLimiter
	if maxDownloadKB > 0 {
		rl = util.NewRateLimiter(maxDownloadKB * 1024)
		fmt.Printf("[STAGE] Main: Enabled download speed cap at %d KB/s\n", maxDownloadKB)
	}

	// START DOWNLOAD LOOP
	if err := piece.DownloadLoop(tf, pm, writer, clients, rl); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	pending, inProgress, completed = pm.Stats()
	fmt.Printf(
		"FINAL => pending=%d inprogress=%d completed=%d\n",
		pending,
		inProgress,
		completed,
	)

	for _, client := range clients {
		_ = client.Conn.Close()
	}

	fmt.Println(
		"download finished:",
		outputPath,
	)
	return nil
}
