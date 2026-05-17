# SiddTorrent

SiddTorrent is a Go implementation of a BitTorrent-style peer-to-peer file distribution engine.

Phase 1 is intentionally narrow: parse a `.torrent`, announce to an HTTP tracker, connect to one peer, complete the peer handshake, request one piece in blocks, and verify that piece with SHA-1.

## Run

```powershell
go run ./cmd/siddtorrent -torrent path\to\file.torrent
```

## Current Packages

- `internal/torrent`: bencode decoder and metainfo parser
- `internal/tracker`: HTTP tracker announce and peer response decoding
- `internal/peer`: handshake, peer state, bitfield, and wire messages
- `internal/piece`: block assembly, simple picker, and SHA-1 verification
- `internal/session`: Phase 1 orchestration flow

## Near-Term Roadmap

1. Add unit tests for bencode, metainfo, tracker compact peers, and peer messages.
2. Persist verified pieces through a disk writer.
3. Replace the single-piece happy path with a retryable piece manager.
4. Add multi-peer scheduling and rarest-first selection.
