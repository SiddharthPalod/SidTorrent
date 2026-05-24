package peer

import "time"

type PeerState struct {
	// protocol state
	Choked     bool
	Interested bool

	// pieces this peer owns
	Bitfield []bool

	// stats
	Downloaded int64
	Uploaded   int64

	ChokedByUs     bool
	InterestedInUs bool
	IntervalBytes  int64
	DownloadRate   float64
	RemotePexID    int
	CorruptCount   int

	// lifecycle
	LastActive time.Time

	// RTT and reliability tracking for scoring (Issue #12)
	SuccessfulDownloads int64
	TotalDownloads      int64
	LatencySum          time.Duration
	LatencyCount        int64
}

func (ps *PeerState) CalculateScore() float64 {
	successRate := 1.0
	if ps.TotalDownloads > 0 {
		successRate = float64(ps.SuccessfulDownloads) / float64(ps.TotalDownloads)
	}

	avgLatency := 100 * time.Millisecond
	if ps.LatencyCount > 0 {
		avgLatency = ps.LatencySum / time.Duration(ps.LatencyCount)
	}

	// Normalize latency: 100ms or lower gets 1.0, 5s or higher gets 0.0
	latencyScore := 1.0 - float64(avgLatency)/float64(5*time.Second)
	if latencyScore < 0 {
		latencyScore = 0
	}
	if latencyScore > 1.0 {
		latencyScore = 1.0
	}

	// Average speed score: normalized against max speed of 1MB/s
	speedScore := ps.DownloadRate / (1024 * 1024)
	if speedScore > 1.0 {
		speedScore = 1.0
	}

	// Calculate weighted aggregate score
	return (successRate * 0.4) + (speedScore * 0.4) + (latencyScore * 0.1) + (1.0-float64(ps.CorruptCount)/2.0)*0.1
}
