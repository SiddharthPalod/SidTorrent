package metrics

import (
	"fmt"
	"sync"
	"time"
)

type MetricsTracker struct {
	mu             sync.RWMutex
	Downloaded     int64
	Uploaded       int64
	ActivePeers    int
	SuccessPieces  int
	FailedPieces   int
	DiskLatencyMs  int64
	DiskWrites     int64
	StartTime      time.Time
}

var GlobalMetrics = &MetricsTracker{
	StartTime: time.Now(),
}

func (m *MetricsTracker) AddDownloaded(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Downloaded += bytes
}

func (m *MetricsTracker) AddUploaded(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Uploaded += bytes
}

func (m *MetricsTracker) SetActivePeers(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActivePeers = count
}

func (m *MetricsTracker) IncSuccessPieces() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SuccessPieces++
}

func (m *MetricsTracker) IncFailedPieces() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailedPieces++
}

func (m *MetricsTracker) RecordDiskWrite(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DiskWrites++
	m.DiskLatencyMs += duration.Milliseconds()
}

func (m *MetricsTracker) Report() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	elapsed := time.Since(m.StartTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	downSpeed := float64(m.Downloaded) / 1024 / elapsed
	upSpeed := float64(m.Uploaded) / 1024 / elapsed
	
	avgDiskLatency := 0.0
	if m.DiskWrites > 0 {
		avgDiskLatency = float64(m.DiskLatencyMs) / float64(m.DiskWrites)
	}

	fmt.Printf("[METRICS] Speed: Down=%.2f KB/s, Up=%.2f KB/s | Active Peers: %d | Pieces: Succeeded=%d, Failed=%d | Disk Write Avg Latency: %.2f ms\n",
		downSpeed, upSpeed, m.ActivePeers, m.SuccessPieces, m.FailedPieces, avgDiskLatency)
}
