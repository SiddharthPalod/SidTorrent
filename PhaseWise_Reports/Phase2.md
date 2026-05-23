# SiddTorrent — Phase 2 Evolution

## Initial Phase 2 Plan

* Piece hashing (SHA-1 validation)
* Bitfield tracking
* Retry failed pieces
* Parallel downloads (multiple peers)
* Random piece selection
* Tracking completed, in-progress, and pending pieces

---

## Problems Encountered

### 1. Integrity Verification
* Pieces are composed of multiple block transfers. Validating before receiving all blocks causes panics.
* Hashing must happen exactly at the piece boundary (combining all blocks) and compare against the corresponding 20-byte SHA-1 hash slice inside the `.torrent` file.

### 2. Swarm Bitfield Synchronization
* Connected peers announce their initial pieces via `bitfield` and subsequent pieces via `HAVE` messages.
* Out-of-bounds piece index tracking in peer bitfields can corrupt client scheduling state and cause buffer index overflow.

### 3. Error Handling and Piece Recovery
* Remote peers often disconnect, throttle, or send corrupt bytes mid-piece.
* Failing a piece should not block the download. The client must safely recycle failed pieces back to the pending queue and retry them, up to a configurable maximum retry count, before blacklisting the peer or halting.

### 4. Concurrent I/O Coordination
* Spawning concurrent peer connections means multiple threads can attempt to modify the same piece scheduling maps (data race).
* Writing randomly received pieces to disk concurrently must be coordinated thread-safely to prevent corrupted writes or race conditions in the file writer.

---

## Resulting Phase 2 Implementation

### Integrity Verifier (`verify.go`)
* Thread-safe SHA-1 verification of fully assembled pieces.
* Precise boundary mapping for variable-sized last pieces.

### Piece Assembler (`assembler.go`)
* Segmented piece buffer structure tracking received block indices.
* Concurrent block writing into the piece payload buffer.

### Piece Manager (`manager.go`)
* Thread-safe scheduling state utilizing mutexes to coordinate `Pending`, `InProgress`, and `Completed` piece sets.
* Robust retry recovery recycling failed pieces up to `DefaultMaxRetries = 3`.

### Parallel Downloader (`scheduler.go` & `worker.go`)
* Multi-peer worker pool running concurrent downloader loops.
* Thread-safe concurrent writing to disk via `storage.Writer` lock.

---

## Tests Added

### Assembler Tests
* Short single-block piece assembly validation.
* Precise block requested/received slice state initialization.

### Storage Tests
* Truncation size mapping against torrent metadata length.
* Offsets and file segment alignment writes.
* Handlers rejecting out-of-range piece sizes.

### Parser Pieces Tests
* Bounds checking on standard piece indexing.
* piece length calculation for intermediate and trailing short pieces.

---

## Final Outcome of Phase 2

Phase 2 transformed SiddTorrent into:
* A resilient multi-peer concurrent downloader.
* An integrity-safe data storage manager capable of validating and persisting blocks.
* A robust error-recovery engine designed to survive peer drops.
