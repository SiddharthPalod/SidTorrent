# SiddTorrent — Phase 1 Evolution

## Initial Phase 1 Plan

* Parse `.torrent` file
* Contact tracker
* Get peer list
* Connect to peer
* Perform handshake
* Request/download one piece
* Save piece to disk

---

# Problems Encountered

## Torrent Parsing Problems

* `.torrent` files contained binary data (`pieces`)
* Using Go `string` corrupted binary fields
* Large torrents overflowed normal `int`
* Torrents had different formats:

  * `announce`
  * `announce-list`
  * `url-list`
  * private torrents
  * multi-file torrents

---

## Info Hash Problems

* `SHA1(Encode(decodedMap))` was not valid
* Trackers rejected requests
* Real BitTorrent requires:

  ```text id="pb1"
  SHA1(exact original raw info dictionary bytes)
  ```

---

## Tracker Problems

* Tracker response is bencoded
* Compact peers are binary bytes, not strings
* Peer parsing initially corrupted IPs/ports

---

## Peer Protocol Problems

* Peers disconnected on bad handshakes
* Requests sent before unchoke
* Incorrect block offsets possible
* Last piece size handling incorrect
* Dead peers caused hangs/timeouts

---

# Resulting New Phase 1 Implementation

## Bencode Layer

* Binary-safe decoder using `[]byte`
* `int64` support
* Validation for malformed bencode
* Binary-safe encoder
* `DecodeWithRaw()` support
* Raw byte offset tracking

---

## Torrent Parser

* Exact raw `info` dictionary extraction
* Correct `info_hash = SHA1(rawInfoBytes)`
* Multi-file torrent support
* Trackerless torrent support
* Webseed torrent detection
* `PieceLengthAt()` helper

---

## Tracker Layer

* Proper bencoded tracker response parsing
* Compact peer decoding fixed
* Correct IPv4 + port extraction

---

## Peer Layer

* Handshake deadlines
* Peer disconnect handling
* Interested → unchoke flow
* Bitfield validation
* Request validation
* Offset validation

---

# Tests Added
| Issue                   | Cause                    |
| ----------------------- | ------------------------ |
| timeout                 | peer dead                |
| EOF                     | peer disconnected        |
| choked forever          | missing interested logic |
| malformed bitfield      | parser bug               |
| piece never arrives     | bad request format       |
| incorrect block offsets | endian issue             |
| invalid piece length    | last-piece handling      |

## Bencode Tests

* Integer parsing
* Binary string preservation
* Encode/decode roundtrip
* Malformed input validation
* Raw byte offset validation

---

## Torrent Tests

* Ubuntu torrent parsing
* Raw info extraction validation
* `InfoHash == SHA1(RawInfo)`
* Last piece sizing tests

---

## Peer Tests

* Peer disconnect during handshake
* Timeout handling
* Request message serialization
* Bitfield validation

---

## Piece Tests

* Incorrect offset rejection
* Invalid block handling
* Piece length validation

---

# Final Outcome of Phase 1

Phase 1 became:

* a protocol-correct binary torrent parser
* canonical info hash generator
* tracker-compatible announce client
* peer wire protocol foundation
* tested/reliable piece transfer base

instead of just:

```text id="pb2"
simple torrent downloader
```