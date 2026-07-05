package session

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/config"
	"github.com/SiddharthPalod/SidTorrent/internal/dht"
	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/piece"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/tracker"
	"github.com/SiddharthPalod/SidTorrent/internal/util"
	"gitlab.com/NebulousLabs/go-upnp"
)

type Request struct {
	TorrentPath          string `json:"torrentPath"`
	OutputPath           string `json:"outputPath"`
	MaxDownloadKB        int64  `json:"maxDownloadKB"`
	MaxUploadKB          int64  `json:"maxUploadKB"`
	MaxActiveConnections int    `json:"maxActiveConnections"`
	IncomingPort         int    `json:"incomingPort"`
	EnableDHT            *bool  `json:"enableDHT,omitempty"`
	EnablePEX            *bool  `json:"enablePEX,omitempty"`
	EnableStreaming      *bool  `json:"enableStreaming,omitempty"`
	EnableChoking        *bool  `json:"enableChoking,omitempty"`
	EnableMetrics        *bool  `json:"enableMetrics,omitempty"`
	EnableEncryption     *bool  `json:"enableEncryption,omitempty"`
	EnableUPnP           *bool  `json:"enableUPnP,omitempty"`
}

type features struct {
	DHT        bool
	PEX        bool
	Streaming  bool
	Choking    bool
	Metrics    bool
	Encryption bool
	UPnP       bool
}


type Status struct {
	Phase         string  `json:"phase"`
	Message       string  `json:"message"`
	TorrentName   string  `json:"torrentName"`
	OutputPath    string  `json:"outputPath"`
	PeerCount     int     `json:"peerCount"`
	ActivePeers   int     `json:"activePeers"`
	Pending       int     `json:"pending"`
	InProgress    int     `json:"inProgress"`
	Completed     int     `json:"completed"`
	TotalPieces   int     `json:"totalPieces"`
	Downloaded    int64   `json:"downloaded"`
	TotalBytes    int64   `json:"totalBytes"`
	Percent       float64 `json:"percent"`
	StartedAtUnix int64   `json:"startedAtUnix"`
	UpdatedAtUnix int64   `json:"updatedAtUnix"`
}

type Options struct {
	OnStatus  func(Status)
	OnStarted func(*piece.PieceManager, *torrent.TorrentFile, string)
}

func Bool(value bool) *bool {
	return &value
}

func enabled(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func resolveFeatures(req Request) features {
	return features{
		DHT:        enabled(req.EnableDHT, true),
		PEX:        enabled(req.EnablePEX, true),
		Streaming:  enabled(req.EnableStreaming, true),
		Choking:    enabled(req.EnableChoking, true),
		Metrics:    enabled(req.EnableMetrics, true),
		Encryption: enabled(req.EnableEncryption, true),
		UPnP:       enabled(req.EnableUPnP, true),
	}
}


func Download(ctx context.Context, req Request, opts Options) error {
	if req.TorrentPath == "" {
		return errors.New("missing torrent path")
	}
	// Apply dynamic configurations from Request
	if req.MaxActiveConnections > 0 {
		config.GlobalConfig.MaxActiveConnections = req.MaxActiveConnections
	}
	if req.IncomingPort > 0 {
		config.GlobalConfig.IncomingPort = req.IncomingPort
	}
	if req.MaxUploadKB > 0 {
		config.GlobalConfig.MaxUploadKB = req.MaxUploadKB
	}
	config.GlobalConfig.EnableEncryption = enabled(req.EnableEncryption, config.GlobalConfig.EnableEncryption)
	config.GlobalConfig.EnableUPnP = enabled(req.EnableUPnP, config.GlobalConfig.EnableUPnP)

	features := resolveFeatures(req)

	startedAt := time.Now()

	status := Status{
		Phase:         "loading",
		Message:       "Loading torrent metadata",
		StartedAtUnix: startedAt.Unix(),
	}
	publish := func(next Status) {
		next.StartedAtUnix = startedAt.Unix()
		next.UpdatedAtUnix = time.Now().Unix()
		status = next
		if opts.OnStatus != nil {
			opts.OnStatus(next)
		}
	}
	publish(status)

	tf, err := torrent.Open(req.TorrentPath)
	if err != nil {
		return fmt.Errorf("load torrent: %w", err)
	}

	if req.OutputPath == "" {
		req.OutputPath = filepath.Join("downloads", tf.Name)
	} else {
		fi, err := os.Stat(req.OutputPath)
		if err == nil && fi.IsDir() {
			req.OutputPath = filepath.Join(req.OutputPath, tf.Name)
		}
	}


	status.TorrentName = tf.Name
	status.OutputPath = req.OutputPath
	status.TotalBytes = tf.Length
	status.TotalPieces = tf.PieceCount()
	status.Message = "Torrent loaded"
	publish(status)

	incomingClients := make(chan *peer.Client, 100)
	var listener net.Listener
	port := config.GlobalConfig.IncomingPort
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err == nil {
		listener = l
		fmt.Printf("[STAGE] Session: Listening for incoming TCP connections on port %d\n", port)
		if config.GlobalConfig.EnableUPnP {
			go func() {
				setupPortForwarding(port)
			}()
		}

		go func() {
			defer l.Close()
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				go func(c net.Conn) {
					client, err := peer.AcceptIncoming(c, tf.InfoHash)
					if err != nil {
						c.Close()
						return
					}
					select {
					case incomingClients <- client:
					default:
						c.Close()
					}
				}(conn)
			}
		}()
	} else {
		fmt.Printf("[WARNING] Session: Failed to listen on port %d: %v\n", port, err)
	}

	defer func() {
		if listener != nil {
			listener.Close()
		}
		if config.GlobalConfig.EnableUPnP {
			go clearPortForwarding(port)
		}
	}()


	var dhtNode *dht.DHTNode
	if features.DHT {
		dhtNode = openDHTNode()
		if dhtNode != nil {
			defer dhtNode.Close()
			status.Phase = "dht"
			status.Message = "Bootstrapping DHT (Experimental)"
			publish(status)
			dhtNode.Bootstrap()
		}
	}

	status.Phase = "discovering"
	status.Message = "Discovering peers from trackers"
	if dhtNode != nil {
		status.Message = "Discovering peers from trackers and DHT"
	}
	publish(status)

	peers, trackerErr := discoverPeers(tf, dhtNode)

	status.PeerCount = len(peers)
	status.Message = fmt.Sprintf("Discovered %d unique peers", len(peers))
	publish(status)

	if len(peers) == 0 && listener == nil && !features.DHT {
		if trackerErr != nil {
			return fmt.Errorf("no usable peers found (tracker error: %w)", trackerErr)
		}
		return errors.New("no usable peers found")
	}


	pm := piece.NewPieceManager(tf)
	pm.SetStreaming(features.Streaming, config.GlobalConfig.PieceStreamingWindow)
	if opts.OnStarted != nil {
		opts.OnStarted(pm, tf, req.OutputPath)
	}
	outputDir := filepath.Dir(req.OutputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	writer, err := storage.NewWriter(tf, req.OutputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer writer.Close()

	if err := pm.LoadExistingState(writer, tf); err != nil {
		status.Message = fmt.Sprintf("Resume verification warning: %v", err)
		publish(status)
	}

	status.Phase = "connecting"
	status.Message = "Connecting to peers"
	pending, inProgress, completed := pm.Stats()
	status.Pending, status.InProgress, status.Completed = pending, inProgress, completed
	status.Downloaded, status.TotalBytes, status.Percent = pm.Progress()
	publish(status)

	if pm.IsComplete() {
		status.Phase = "complete"
		status.Message = "Torrent already downloaded and verified"
		status.Percent = 100
		publish(status)
		return nil
	}

	clients := connectPeers(ctx, tf, pm, peers, func(active int) {
		status.ActivePeers = active
		status.Message = fmt.Sprintf("Connected to %d usable peers", active)
		publish(status)
	})

	status.ActivePeers = len(clients)
	status.Message = fmt.Sprintf("Connection phase complete with %d active peers", len(clients))
	publish(status)
	if len(clients) == 0 && listener == nil && !features.DHT && !features.PEX {
		return errors.New("no usable peers found")
	}
	defer func() {
		for _, client := range clients {
			_ = client.Conn.Close()
		}
	}()


	var rl *util.RateLimiter
	if req.MaxDownloadKB > 0 {
		rl = util.NewRateLimiter(req.MaxDownloadKB * 1024)
		defer rl.Close()
	}

	status.Phase = "downloading"
	status.Message = "Downloading pieces"
	publish(status)

	tickerCtx, stopTicker := context.WithCancel(ctx)
	defer stopTicker()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-tickerCtx.Done():
				return
			case <-ticker.C:
				pending, inProgress, completed := pm.Stats()
				downloaded, total, percent := pm.Progress()
				status.Pending, status.InProgress, status.Completed = pending, inProgress, completed
				status.Downloaded, status.TotalBytes, status.Percent = downloaded, total, percent
				publish(status)
			}
		}
	}()

	if err := piece.DownloadLoop(ctx, tf, pm, writer, clients, rl, piece.SchedulerOptions{
		EnablePEX:       features.PEX,
		EnableChoking:   features.Choking,
		EnableMetrics:   features.Metrics,
		IncomingClients: incomingClients,
	}); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	stopTicker()

	pending, inProgress, completed = pm.Stats()
	downloaded, total, percent := pm.Progress()
	status.Phase = "complete"
	status.Message = "Download finished"
	status.Pending, status.InProgress, status.Completed = pending, inProgress, completed
	status.Downloaded, status.TotalBytes, status.Percent = downloaded, total, percent
	publish(status)
	return nil
}

func openDHTNode() *dht.DHTNode {
	node, err := dht.NewDHTNode(config.GlobalConfig.DHTPort)
	if err != nil {
		node, err = dht.NewDHTNode(0)
	}
	if err != nil {
		return nil
	}
	return node
}

func discoverPeers(tf *torrent.TorrentFile, dhtNode *dht.DHTNode) ([]tracker.Peer, error) {
	var peers []tracker.Peer
	seenPeers := make(map[string]bool)
	addPeer := func(p tracker.Peer) {
		key := fmt.Sprintf("%s:%d", p.IP.String(), p.Port)
		if seenPeers[key] {
			return
		}
		seenPeers[key] = true
		peers = append(peers, p)
	}

	trackerPeers, trackerErr := tracker.GetPeers(tf)
	if trackerErr == nil {
		for _, p := range trackerPeers {
			addPeer(p)
		}
	}
	if dhtNode != nil {
		for _, p := range dhtNode.SearchPeers(tf.InfoHash) {
			addPeer(p)
		}
	}
	return peers, trackerErr
}

func connectPeers(ctx context.Context, tf *torrent.TorrentFile, pm *piece.PieceManager, peers []tracker.Peer, onActive func(int)) []*peer.Client {
	var clients []*peer.Client
	var mu sync.Mutex
	var wg sync.WaitGroup
	peerChan := make(chan tracker.Peer, len(peers))
	for _, p := range peers {
		peerChan <- p
	}
	close(peerChan)

	maxWorkers := 20
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case p, ok := <-peerChan:
					if !ok {
						return
					}

					mu.Lock()
					if len(clients) >= config.GlobalConfig.MaxActiveConnections {
						mu.Unlock()
						return
					}
					mu.Unlock()

					address := net.JoinHostPort(p.IP.String(), fmt.Sprintf("%d", p.Port))
					if pm.IsBlacklisted(address) {
						continue
					}
					client, err := peer.ConnectTimeout(address, tf.InfoHash, config.GlobalConfig.TrackerDialTimeout)
					if err != nil {
						continue
					}
					if err := client.ReadBitField(tf.PieceCount()); err != nil {
						_ = client.Conn.Close()
						continue
					}
					if err := client.SendInterested(); err != nil {
						_ = client.Conn.Close()
						continue
					}

					mu.Lock()
					if len(clients) < config.GlobalConfig.MaxActiveConnections {
						clients = append(clients, client)
						if onActive != nil {
							onActive(len(clients))
						}
					} else {
						_ = client.Conn.Close()
					}
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	return clients
}

func setupPortForwarding(port int) {
	fmt.Printf("[UPnP] Attempting to map port %d...\n", port)
	d, err := upnp.Discover()
	if err != nil {
		fmt.Printf("[UPnP] Discovery failed: %v\n", err)
		return
	}
	ip, err := d.ExternalIP()
	if err != nil {
		fmt.Printf("[UPnP] Failed to get external IP: %v\n", err)
	} else {
		fmt.Printf("[UPnP] Router external IP: %s\n", ip)
	}

	err = d.Forward(uint16(port), "SiddTorrent")
	if err != nil {
		fmt.Printf("[UPnP] Port mapping failed: %v\n", err)
	} else {
		fmt.Printf("[UPnP] Port mapping succeeded! External port %d is now open.\n", port)
	}
}

func clearPortForwarding(port int) {
	d, err := upnp.Discover()
	if err != nil {
		return
	}
	_ = d.Clear(uint16(port))
	fmt.Printf("[UPnP] Port mapping for %d cleared.\n", port)
}

