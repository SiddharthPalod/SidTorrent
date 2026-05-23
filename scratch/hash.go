package main

import (
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
)

func main() {
	data, err := os.ReadFile("testdata/alpine.torrent")
	if err != nil {
		panic(err)
	}
	rootNode, err := bencode.DecodeWithRaw(data)
	if err != nil {
		panic(err)
	}
	infoNode := rootNode.Dict["info"]
	rawInfo := data[infoNode.Start:infoNode.End]
	hash := sha1.Sum(rawInfo)
	fmt.Printf("InfoHash: %x\n", hash)
}
