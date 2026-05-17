package torrent

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
	"os"
)

type TorrentFile struct {
	Announce string
	Name     string
	Length   int
	PieceLen int
	Pieces   []byte
	InfoHash [20]byte
}

func Open(path string) (*TorrentFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	decoded, err := bencode.Decode(reader)
	if err != nil {
		return nil, err
	}

	root := decoded.(map[string]interface{})
	info := root["info"].(map[string]interface{})

	infoBytes := []byte(fmt.Sprintf("%v", info))
	hash := sha1.Sum(infoBytes)

	torrent := &TorrentFile{
		Announce: root["announce"].(string),
		Name:     info["name"].(string),
		Length:   info["length"].(int),
		PieceLen: info["piece length"].(int),
		Pieces:   []byte(info["pieces"].(string)),
		InfoHash: hash,
	}

	return torrent, nil
}
