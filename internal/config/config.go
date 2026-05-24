package config

import "time"

type Config struct {
	TrackerPort          int
	TrackerNumWant       int
	TrackerDialTimeout   time.Duration
	DHTPort              int
	DHTK                 int
	DHTBootstrapNodes    []string
	PieceMaxRetries      int
	PieceStreamingWindow int
	DiskMaxCacheSize    int
	DiskFlushInterval    time.Duration
	MaxActiveConnections int
}

var GlobalConfig = Config{
	TrackerPort:          6881,
	TrackerNumWant:       50,
	TrackerDialTimeout:   15 * time.Second,
	DHTPort:              6881,
	DHTK:                 8,
	DHTBootstrapNodes: []string{
		"router.bittorrent.com:6881",
		"dht.transmissionbt.com:6881",
		"router.utorrent.com:6881",
	},
	PieceMaxRetries:      3,
	PieceStreamingWindow: 15,
	DiskMaxCacheSize:    4 * 1024 * 1024, // 4MB
	DiskFlushInterval:    2 * time.Second,
	MaxActiveConnections: 10,
}
