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

	targetPeer := peers[0]
	address := fmt.Sprintf("%s:%d", targetPeer.IP.String(), targetPeer.Port)
	client, err := peer.Connect(address, tf.InfoHash)
	if err != nil {
		panic(err)
	}
	fmt.Println("connected to peer")

	err = client.SendInterested()
	if err != nil {
		panic(err)
	}
	fmt.Println("sent interested message")

	data, err := piece.DownloadPiece(client, 0, tf.PieceLen)
	if err != nil {
		panic(err)
	}
	os.MkdirAll("downloads", os.ModePerm)
	err = os.WriteFile("downloads/piece0.bin", data, 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println("piece downloaded successfully")
}
