package main

import (
	"fmt"
	"os"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/piece"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
	"github.com/SiddharthPalod/SidTorrent/internal/tracker"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("usage: siddtorrent <torrent-file>")
		return
	}

	torrentFile := os.Args[1]

	tf, err := torrent.Open(torrentFile)
	if err != nil {
		panic(err)
	}

	fmt.Println("torrent loaded:", tf.Name)

	peers, err := tracker.GetPeers(tf)
	if err != nil {
		panic(err)
	}

	fmt.Println("peer count:", len(peers))

	if len(peers) == 0 {
		panic("no peers found")
	}

	var client *peer.Client

	for _, p := range peers {

		address := fmt.Sprintf(
			"%s:%d",
			p.IP.String(),
			p.Port,
		)

		fmt.Println("trying peer:", address)

		client, err = peer.Connect(address, tf.InfoHash)

		if err != nil {

			fmt.Println("connect failed:", err)

			continue
		}

		fmt.Println("connected:", address)

		totalPieces := len(tf.Pieces) / 20

		err = client.ReadBitField(totalPieces)

		if err != nil {

			fmt.Println("bitfield failed:", err)

			_ = client.Conn.Close()

			client = nil

			continue
		}

		fmt.Println("received bitfield")

		break
	}

	if client == nil {
		panic("could not connect to any peer")
	}

	defer client.Conn.Close()

	err = client.SendInterested()
	if err != nil {
		panic(err)
	}

	fmt.Println("sent interested message")

	pieceIndex := 0

	pieceLength := tf.PieceLengthAt(pieceIndex)

	fmt.Printf(
		"downloading piece %d (%d bytes)\n",
		pieceIndex,
		pieceLength,
	)

	data, err := piece.DownloadPiece(
		client,
		pieceIndex,
		int(pieceLength),
	)

	if err != nil {
		panic(err)
	}

	fmt.Println("piece downloaded")

	os.MkdirAll("downloads", os.ModePerm)

	outputPath := fmt.Sprintf(
		"downloads/piece_%d.bin",
		pieceIndex,
	)

	err = os.WriteFile(outputPath, data, 0644)

	if err != nil {
		panic(err)
	}

	fmt.Println("saved:", outputPath)
}
