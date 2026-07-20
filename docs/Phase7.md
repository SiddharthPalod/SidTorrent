# SiddTorrent — Phase 7 Evolution

## Initial Phase 7 Plan

* Override rarest-first with priority-based sequential downloading
* Implement sequential downloading for the first N pieces
* Maintain standard Rarest-First for remaining pieces to protect swarm health
* Deliverable: Start playing media/video files before the download is 100% complete

---

## Problems Encountered

### 1. Swarm Health Degradation vs. Instant Streaming Balance
* **Problem**: Downloading a torrent 100% sequentially is highly detrimental to swarm health because rare pieces in the swarm are ignored, leading to download stalls and low bandwidth throughput.
* **Fix**: Designed a highly optimal **Hybrid Selector** where only the first $N$ pieces (the streaming window, default = 15) are prioritized sequentially, after which the client automatically reverts to standard Rarest-First, achieving the perfect balance between instant playback and swarm sustainability.

### 2. Gap Repairs in the Playback Buffer
* **Problem**: If a worker fails to download a prioritized sequential piece (due to connection loss or verification failure), that piece must be repaired immediately. If subsequent workers skip it and continue downloading higher indices, a permanent gap is left in the streaming buffer, causing video playback to hang.
* **Fix**: Programmed `pm.MarkFailed()` to immediately return the failed piece to the `Pending` map. The hybrid piece selector automatically captures this missing piece and assigns it sequentially to the very next available worker, repairing playback buffer gaps in real-time.

---

## Resulting Phase 7 Implementation

### Configurable Hybrid Piece Selector (`internal/piece/` [MODIFY])
* **Streaming Engine State (`manager.go`)**: Added `StreamingMode` and `StreamingWindowSize` configurations to the core `PieceManager` struct, enabling streaming mode by default.
* **Sequential Priority Loop (`NextStreamingPiece`)**: Scans the first $N$ pieces sequentially, immediately selecting the lowest index pending and available piece.
* **Rarest-First Fallback Loop**: Once the sequential window pieces are in-progress or completed, the selector falls back to the Kademlia-inspired Rarest-First search among the remaining pieces.
* **Worker Integration (`worker.go`)**: Swapped out `pm.NextRandomPiece` for `pm.NextPiece` inside the background worker loops, seamlessly running the hybrid selector across all concurrent downloading threads.

---

## Tests Added (`phase7_test.go` [NEW])

### Hybrid Streaming Piece Selector Test (`TestHybridStreamingPieceSelection`)
* Verifies that when streaming mode is active, the first $N$ pieces are strictly requested sequentially, even when extremely rare pieces exist elsewhere in the swarm.
* Confirms that if a piece in the sequential window is currently unavailable from a peer, the selector dynamically skips to the next available sequential index.
* Validates that once the sequential window is complete, the selector seamlessly falls back to Rarest-First selection for the remaining pending pieces.

---

## Final Outcome of Phase 7

Phase 7 introduces real-time media playback capabilities to SiddTorrent:
* **Immediate Playback**: Users can stream high-definition video and audio files instantly after beginning a download, without waiting for the full file to download.
* **Self-Healing Stream**: The buffer dynamically repairs itself from subsequent peer disconnects or packet drops.
* **Optimal Swarm Preservation**: The hybrid rarest-first fallback keeps the client highly cooperative and optimal within public swarms.
