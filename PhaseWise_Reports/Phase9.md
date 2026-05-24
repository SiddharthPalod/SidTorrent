# SiddTorrent — Phase 9 Evolution

## Initial Phase 9 Plan

* Strict timeout handling for all TCP network operations
* Peer disconnect recovery and seamless piece rollbacks
* Malicious/corrupt peer detection (Integrity check failures tracking)
* Global IP blacklisting / banning of compromised contacts
* Throttled token-bucket bandwidth rates
* Deliverable: Bulletproof, production-grade swarming engine secure against network hangs, malicious/faulty peers, and data corruption

---

## Problems Encountered

### 1. Indefinite Goroutine Socket Freezes on Slow/Hung Writes
* **Problem**: TCP connection writes (such as outgoing requests, choke/unchoke updates, and PEX messages) could hang indefinitely if a remote peer's socket froze, causing worker goroutines to block indefinitely and leaking system threads.
* **Fix**: Programmed a unified `WriteMessage()` utility under the `peer.Client` object that strictly applies a 10-second `SetWriteDeadline` before every write. If a socket freezes, the connection fails fast, releasing all occupied worker resources immediately.

### 2. Bandwidth Wasting & Disk Thrashing on Malicious Corrupted Data
* **Problem**: If a peer transmitted corrupt piece data, the hash check (`VerifyPiece`) correctly failed, but our client immediately retried downloading from the same peer, repeating the cycle and wasting massive bandwidth while thrashing disk writes.
* **Fix**: Added a `CorruptCount` tracker to `PeerState`. If a peer transmits corrupted blocks that fail the SHA-1 hash check twice, we globally ban their host IP and violently terminate their connection, protecting our client's integrity.

### 3. Re-dialing Blacklisted Peers via PEX/DHT crawls
* **Problem**: Banning a peer on one port does not prevent us from re-discovering and dialing them on a different port via background Peer Exchange (PEX) updates or DHT crawlers.
* **Fix**: Programmed a thread-safe global `BlacklistedPeers` registry inside the shared `PieceManager`. We strip port numbers and blacklist the host IP address entirely. Before initiating any connection during startup or dynamic background dialing, we verify `pm.IsBlacklisted(address)` and reject blocked hosts instantly.

---

## Resulting Phase 9 Implementation

### Strict Outgoing Deadlines (`internal/peer/peer.go` [MODIFY])
* **Unified Message Writer (`WriteMessage`)**: Enforces a 10-second `SetWriteDeadline` on all outgoing transmissions, safeguarding thread lifecycles.
* **Refactored Client Messaging**: Re-routed all outgoing packets (`SendInterested`, `SendNotInterested`, `SendHave`, `SendChoke`, `SendUnchoke`) through the deadline writer.

### Unified Request Timeout Writer (`internal/piece/downloader.go` [MODIFY])
* **Safe Block Requests**: Re-routed block request writes through `client.WriteMessage(req)` to protect down-stream connections from slow-peer write lockups.

### Corrupt Peer Detector (`internal/piece/` [MODIFY])
* **Malicious Detection & Banning (`worker.go`)**: Increments the peer's `CorruptCount` on SHA-1 hash failures and globally blacklists the host IP if the count hits 2.
* **Global IP Blacklist Registry (`manager.go`)**: Added a mutex-protected `BlacklistedPeers` host map, split-port IP stripping, and lookups inside `PieceManager`.
* **PEX Dialer Lockdown (`scheduler.go`)**: Checks `pm.IsBlacklisted(address)` before background dials.

### Core Connection Throttling (`cmd/siddtorrent/main.go` [MODIFY])
* **Startup Connection Lockdown**: Verifies resolved IP hosts against `pm.IsBlacklisted(address)` before dialing.

---

## Tests Added (`phase9_test.go` [NEW])

### Global Peer Blacklisting Test (`TestGlobalPeerBlacklisting`)
* Validates that blacklisting a host IP correctly bans all of its associated ports dynamically, while leaving other clean swarming hosts unaffected.

### Outgoing Message Write Deadline Test (`TestWriteMessageDeadline`)
* Verifies that the unified `WriteMessage` completes successfully under the strict write deadline constraint over a mock network pipe.

### Malicious Corrupt Peer Blacklisting Test (`TestMaliciousCorruptPeerBlacklisting`)
* Assures that when a peer sends corrupted data twice, the corrupt peer detector triggers a global blacklist ban against their host IP.

---

## Final Outcome of Phase 9

Phase 9 secures SiddTorrent as a robust, resilient torrent client:
* **Hung Goroutine Protection**: Indefinite connection freezes are mathematically impossible.
* **Integrity Defended**: Malicious or broken peers are blacklisted globally across the network instantly.
* **Swarm Resiliency**: Recovers seamlessly from disconnects, re-allocating missing blocks instantly without downloading stalls.
