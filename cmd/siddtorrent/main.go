package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/piece"
	"github.com/SiddharthPalod/SidTorrent/internal/storage"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/tracker"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var outputPath string
	flag.StringVar(&outputPath, "out", "", "output file path")
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
	tf, err := torrent.Open(
		torrentPath,
	)

	if err != nil {
		return fmt.Errorf("load torrent: %w", err)
	}

	fmt.Println(
		"torrent loaded:",
		tf.Name,
	)

	// DEBUG TRACKERS
	fmt.Println("trackers:")
	for i, tier := range tf.Trackers {
		fmt.Printf(
			"tier %d:\n",
			i,
		)

		for _, tr := range tier {

			fmt.Println("  ", tr)
		}
	}

	peers, err := tracker.GetPeers(tf)

	if err != nil {
		return fmt.Errorf("tracker failed: %w", err)
	}

	fmt.Println(
		"peer count:",
		len(peers),
	)

	if len(peers) == 0 {
		return errors.New("no usable peers found")
	}

	// piece manager
	pm := piece.NewPieceManager(tf)

	pending,
		inProgress,
		completed := pm.Stats()

	fmt.Printf(
		"piece manager => pending=%d inprogress=%d completed=%d\n",
		pending,
		inProgress,
		completed,
	)

	if outputPath == "" {
		outputPath = filepath.Join("downloads", tf.Name)
	}

	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		err = os.MkdirAll(
			outputDir,
			os.ModePerm,
		)
		if err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	writer, err := storage.NewWriter(
		tf,
		outputPath,
	)

	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer writer.Close()

	// connect multiple peers
	var clients []*peer.Client
	totalPieces := tf.PieceCount()

	for _, p := range peers {
		address := fmt.Sprintf(
			"%s:%d",
			p.IP.String(),
			p.Port,
		)
		fmt.Println(
			"trying peer:",
			address,
		)
		client, err := peer.Connect(
			address,
			tf.InfoHash,
		)
		if err != nil {
			fmt.Println(
				"connect failed:",
				err,
			)
			continue
		}
		fmt.Println(
			"connected:",
			address,
		)
		err = client.ReadBitField(
			totalPieces,
		)
		if err != nil {
			fmt.Println(
				"bitfield failed:",
				err,
			)
			_ = client.Conn.Close()
			continue
		}
		fmt.Println(
			"received bitfield",
		)

		err = client.SendInterested()
		if err != nil {
			fmt.Println(
				"interested failed:",
				err,
			)
			_ = client.Conn.Close()
			continue
		}
		fmt.Println(
			"sent interested",
		)
		clients = append(
			clients,
			client,
		)
		if len(clients) >= 10 {
			break
		}
	}

	fmt.Println("usable peers:", len(clients))
	if len(clients) == 0 {
		return errors.New("no usable peers found")
	}

	// START DOWNLOAD LOOP
	if err := piece.DownloadLoop(tf, pm, writer, clients); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	pending, inProgress, completed = pm.Stats()
	fmt.Printf(
		"FINAL => pending=%d inprogress=%d completed=%d\n",
		pending,
		inProgress,
		completed,
	)

	for _, client := range clients {
		_ = client.Conn.Close()
	}

	fmt.Println(
		"download finished:",
		outputPath,
	)
	return nil
}
