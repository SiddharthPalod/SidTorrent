package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/SiddharthPalod/SidTorrent/internal/session"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutdown initiated...")
		cancel()
	}()

	if err := run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("Shutdown complete")
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	var outputPath string
	flag.StringVar(&outputPath, "out", "", "output file path")
	var maxDownloadKB int64
	flag.Int64Var(&maxDownloadKB, "max-download", 0, "maximum download rate in KB/s (0 for unlimited)")
	var maxUploadKB int64
	flag.Int64Var(&maxUploadKB, "max-upload", 0, "maximum upload rate in KB/s (0 for unlimited)")
	var maxConns int
	flag.IntVar(&maxConns, "max-conns", 100, "maximum active connections")
	var port int
	flag.IntVar(&port, "port", 6881, "incoming port to listen on")
	enableDHT := flag.Bool("dht", true, "enable DHT peer discovery")
	enablePEX := flag.Bool("pex", true, "enable peer exchange")
	enableStreaming := flag.Bool("streaming", true, "prioritize initial pieces for streaming")
	enableChoking := flag.Bool("choking", true, "enable choke manager")
	enableMetrics := flag.Bool("metrics", true, "enable periodic metrics reporting")
	enableEncryption := flag.Bool("encryption", true, "enable protocol encryption (stream obfuscation)")
	enableUPnP := flag.Bool("upnp", true, "enable UPnP/NAT-PMP port forwarding")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: siddtorrent [-out output-file] <torrent-file>\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		return errors.New("missing torrent file")
	}

	torrentPath := flag.Arg(0)
	return session.Download(ctx, session.Request{
		TorrentPath:          torrentPath,
		OutputPath:           outputPath,
		MaxDownloadKB:        maxDownloadKB,
		MaxUploadKB:          maxUploadKB,
		MaxActiveConnections: maxConns,
		IncomingPort:         port,
		EnableDHT:            session.Bool(*enableDHT),
		EnablePEX:            session.Bool(*enablePEX),
		EnableStreaming:      session.Bool(*enableStreaming),
		EnableChoking:        session.Bool(*enableChoking),
		EnableMetrics:        session.Bool(*enableMetrics),
		EnableEncryption:     session.Bool(*enableEncryption),
		EnableUPnP:           session.Bool(*enableUPnP),
	}, session.Options{
		OnStatus: func(status session.Status) {
			fmt.Printf("[%s] %s %.2f%% peers=%d active=%d pieces=%d/%d\n",
				status.Phase,
				status.Message,
				status.Percent,
				status.PeerCount,
				status.ActivePeers,
				status.Completed,
				status.TotalPieces,
			)
		},
	})
}
