# SiddTorrent — Phase 3 Evolution

## Initial Phase 3 Plan

* Rarest-First piece selection algorithm
* Choking decision engine:
  * Optimistic unchoking
  * Tit-for-tat strategy
* Concurrency optimizations
* Rate limiting (Token Bucket)
* Peer scoring (EWMA download speed)
* Deliverable: Swarm-like download performance

---

## Problems Encountered

### 1. Swarm-Wide Starvation (Rarest-First)
* Using purely random selection caused "piece starvation," where rare pieces in the swarm were not prioritized, causing slower download completion times.
* The client needed to maintain a global view of piece frequencies by registering and unregistering all active peer bitfields dynamically.

### 2. Multi-threaded Capping Deadlocks (Rate Limiter)
* In our initial Token Bucket implementation, a rate limit cap strictly lower than a single block size led to an infinite blocking lock in the `Wait` function.
* Capping logic had to be dynamically adapted to `max(capacity, requested_tokens)` to allow large blocks to drain tokens safely without hanging.

### 3. Connection Deadlocks (Choke Manager Mocking)
* Unchoking messages wrote directly to peer socket connections. In test swarms, if connection mock buffers were not drained in background goroutines, the choke engine evaluations block-locked, stalling the tests.
* Fully buffered and drained net pipes were required to test the choking scheduler.

### 4. IPv6 & VPN Peer Mismatches
* Under VPN and IPv6-enabled networks, trackers return compact IPv6 lists (18 bytes per peer) rather than standard compact IPv4 formats (6 bytes per peer).
* Hardcoded 6-byte parsing interpreted 18-byte segments as garbage IPv4 subnets and random/zero ports, causing dial failures.
* Go's TCP dialer requires enclosing raw IPv6 addresses in `[...]` brackets (e.g. `[2001:db8::1]:port`). Failure to do so causes immediate connection refuels.

---

## Resulting Phase 3 Implementation

### Swarm Availability & Rarest-First (`manager.go` & `tracker.go`)
* **Global Frequency Array:** `Availability []int` inside `PieceManager` dynamically tracks the number of active peers owning each piece.
* **Dynamic Registration:** Workers register peer bitfields on connection (`RegisterPeerBitfield`) and safely decrement them on disconnect (`UnregisterPeerBitfield`) to prevent offline peers from skews.
* **Randomized Tie-Breaker:** Rarest-first prioritizes pieces with the lowest frequency count, randomly selecting among matching ties to prevent concurrent workers from redundant requests.

### Bandwidth Controller (`rate_limiter.go` [NEW])
* **Token Bucket Engine:** Implement thread-safe token generation proportional to time delta, supporting dynamic `-max-download` CLI limits.
* **Deadlock Protection:** Automatically handles large block requests that exceed baseline burst capacities.

### Choking Decision Engine (`choke_manager.go` [NEW])
* **EWMA Throughput Tracking:** Measures byte samples over time intervals (`IntervalBytes` / $\Delta t$) and applies an 80% weight filter to smooth network noise.
* **Tit-for-Tat Scheduler:** Automatically ranks interested peers descending by speed, unchoking top slots to maximize swarm reciprocity.
* **Optimistic Unchoke Scheduler:** Keeps a dedicated slot active and rotates it to a random interested peer every 30 seconds to facilitate discovery of faster connections.

### Network Adaptability Layer (`tracker.go` & `main.go`)
* **IPv6 Compact Parser:** Dynamically detects if the tracker UDP socket is IPv4 or IPv6 and switches between 6-byte or 18-byte parser buffers automatically.
* **IPv6 Dialer Compatibility:** Wraps parsed IPs via `net.JoinHostPort` to enforce `[...]` bracket formatting on outbound IPv6 dials.

---

## Tests Added (`phase3_test.go` [NEW])

### Rarest-First Piece Selector Test
* Verifies that `NextRarestPiece` isolates and returns the swarm's rarest available piece first.

### Token Bucket Throttling Test
* Asserts that token requests correctly throttle and delay program execution according to configured rate caps.

### Choke Manager TFT Ranking Test
* Emulates active client connections and verifies that the choking engine correctly calculates EWMA speeds and unchokes the fastest active peer.

---

## Final Outcome of Phase 3

Phase 3 turned SiddTorrent into:
* An active swarm participant prioritizing rare pieces.
* A robust, network-adaptable client capable of downloading over IPv4 and IPv6 VPN tunnels.
* A bandwidth-capped and throttled client matching standard torrent client behavior.
* A performance-optimized multi-core client utilizing the Tit-for-Tat protocol.
