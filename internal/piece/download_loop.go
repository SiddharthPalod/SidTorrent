package piece

import (
	"context"
	"fmt"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/util"
)

func DownloadLoop(
	ctx context.Context,
	tf *torrent.TorrentFile,
	pm *PieceManager,
	writer *storage.Writer,
	clients []*peer.Client,
	rl *util.RateLimiter,
	opts SchedulerOptions,
) error {
	if len(clients) == 0 && opts.IncomingClients == nil && !opts.EnablePEX {
		return fmt.Errorf("no connected peers")
	}

	if err := StartScheduler(ctx, tf, pm, writer, clients, rl, opts); err != nil {
		return err
	}
	fmt.Println("torrent download complete")
	return nil
}
