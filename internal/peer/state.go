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

	// lifecycle
	LastActive time.Time
}
