package piece

import (
	"fmt"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

func DownloadLoop(
	tf *torrent.TorrentFile,
	pm *PieceManager,
	writer *storage.Writer,
	clients []*peer.Client,
) error {
	if len(clients) == 0 {
		return fmt.Errorf("no connected peers")
	}
	if err := StartScheduler(tf, pm, writer, clients); err != nil {
		return err
	}
	fmt.Println("torrent download complete")
	return nil
}
