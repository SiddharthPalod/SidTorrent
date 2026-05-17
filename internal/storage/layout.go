package storage

import "github.com/SiddharthPalod/SidTorrent/internal/torrent"

func PieceOffset(
	tf *torrent.TorrentFile,
	pieceIndex int) int64 {
	return int64(pieceIndex) * tf.PieceLength
}
