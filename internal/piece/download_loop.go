package piece

import (
	"fmt"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/util"
)

func DownloadLoop(
	tf *torrent.TorrentFile,
	pm *PieceManager,
	writer *storage.Writer,
	clients []*peer.Client,
	rl *util.RateLimiter,
) error {
	if len(clients) == 0 {
		return fmt.Errorf("no connected peers")
	}
	if err := StartScheduler(tf, pm, writer, clients, rl); err != nil {
		return err
	}
	fmt.Println("torrent download complete")
	return nil
}
