# SiddTorrent — Phase 6 Evolution

## Initial Phase 6 Plan

* Share peer lists between connected peers (PEX - BEP 11)
* Maintain a thread-safe dynamic peer pool in the swarm scheduler
* Deliverable: Rapid swarm expansion and active peer discovery from connected clients

---

## Problems Encountered

### 1. Mock Test Signature Mismatch
* **Problem**: Injecting the dynamic PEX peer channel directly into the function signatures of `DownloadPiece` and `StartWorker` would have broken several dozen direct mock unit tests in `tests/piece_test.go` and increased maintenance complexity.
* **Fix**: Integrated a thread-safe `PeerChan chan<- string` channel directly into the `peer.Client` struct so it can be dynamically injected during scheduling, avoiding any signature modifications or mock test breakages.

### 2. Double Worker Spawning & Redeclarations
* **Problem**: Redundant copy-pasted blocks from older phases in `scheduler.go` redeclared `wg` and `errCh` inside `StartScheduler()`, causing Go compiler redeclaration failures and double client worker starts.
* **Fix**: Cleaned up the scoping, removed the duplicate blocks, merged client setup sequences into a single clean PEX-supporting `startPeerWorker` closure, and eliminated unused scoping variables.

### 3. Handshake Extension Bit Serialization Error
* **Problem**: The `Serialize()` method in `internal/peer/handshake.go` was copying a blank 8-byte slice `make([]byte, 8)` rather than the `reserved` byte slice, resulting in the Extension Protocol advertisement bit (`0x10` at index 25) always being serialized as zero.
* **Fix**: Corrected the buffer copy operation to copy the populated `reserved` slice, enabling advertising of BEP 10/11 Extension Protocol support.

### 4. Post-Handshake `got=20` or `got=1` Disconnect Crashes
* **Problem**: Once extension support bit was set, peers immediately sent their bencoded Extended Handshake (ID 20) right after handshaking. Since `ReadBitField()` expected ONLY `MsgBitfield` (ID 5) as the first message, it immediately errored with `expected bitfield message got=20` and aborted active connections.
* **Fix**: Refactored `ReadBitField()` into a post-handshake message processing loop that parses keepalives and `MsgExtended` (ID 20) handshakes dynamically, maps initial `MsgHave`/`MsgChoke`/`MsgUnchoke` events, and initializes a default empty bitfield if the peer chooses to start standard communication without sending a bitfield.

### 5. Tracker Dictionary List (Non-Compact Format) Ignored
* **Problem**: The HTTP/HTTPS tracker parser only expected `peers` as a compact byte slice (`root["peers"].([]byte)`), so when contacting standard trackers like Ubuntu's HTTPS announce tracker (which returns `peers` as a list of dictionaries), it ignored the key and threw a `tracker response missing both peers and peers6 fields` error.
* **Fix**: Added a robust fallback parser supporting bencoded lists of dictionary items containing ASCII string IP addresses and numeric ports.

---

## Resulting Phase 6 Implementation

### Core Handshake & Extensions Setup (`internal/peer/` [MODIFY])
* **Handshake Bit Correction (`handshake.go`)**: Serializes the `reserved` byte array, advertising BEP 10 support.
* **Peer Extensions Framework (`pex.go`)**: Safely processes incoming extended handshakes, registers remote extension IDs, and processes IPv4 and IPv6 compact PEX lists securely.
* **Startup Handshake Loop (`peer.go`)**: Restructured `ReadBitField` into a message loop to parse keepalives, extended handshakes (ID 20), have, choke, and unchoke messages during connection startup.

### Swarm PEX Scheduler & Dialer (`internal/piece/` [MODIFY])
* **Dynamic PEX Dialer (`scheduler.go`)**: Spawns a background thread listening on `peerChan` to dial, handshake, and register new PEX-discovered peers in real-time.
* **Active Clients Tracker**: Tracks all active workers dynamically in a thread-safe slice (`activeClients` protected by a `sync.Mutex`).
* **Periodic PEX Broadcast Loop**: Periodically broadcasts active peer addresses (filtering out each target's own IP) to connected peers that support PEX using bencoded `SendPexMessage` calls.

### Robust HTTP Tracker Resolver (`internal/tracker/` [MODIFY])
* **Hybrid Compact / List Decoder (`tracker.go`)**: Decodes both compact byte slices and non-compact lists of dictionaries for both `peers` and `peers6` responses.

---

## Tests Added (`pex_test.go` [NEW])

### PEX Handshake & Extensions Test
* Verifies that handshakes correctly indicate extension support by setting the 20th reserved bit (`0x10` at index 25), parses back accurately, and sets the `SupportsExtensions` flag.

---

## Final Outcome of Phase 6

Phase 6 provides complete swarming and peer exchange capabilities to SiddTorrent:
* **Enhanced Swarm Expansion**: Discovers and dials new peers on-the-fly dynamically.
* **Real-World Spec Compatibility**: Connects and stays connected to popular torrent clients (Transmission, qBittorrent) without startup protocol crashes.
