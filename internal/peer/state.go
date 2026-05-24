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
}
