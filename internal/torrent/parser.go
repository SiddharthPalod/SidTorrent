package torrent

import (
	"bufio"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
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

	root, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid torrent root")
	}

	info, ok := root["info"].(map[string]interface{})
	if !ok {
		return nil, errors.New("missing info dictionary")
	}

	// DEBUG
	fmt.Println("INFO DICT:")
	for k, v := range info {
		switch val := v.(type) {
		case string:
			if k == "pieces" {
				fmt.Printf("%s => binary data (%d bytes)\n", k, len(val))
			} else {
				fmt.Printf("%s => %s\n", k, val)
			}
		default:
			fmt.Printf("%s => %T => %v\n", k, v, v)
		}
	}

	var totalLength int

	// SINGLE FILE
	if length, ok := info["length"].(int); ok {
		totalLength = length
	}

	// MULTI FILE
	if files, ok := info["files"].([]interface{}); ok {

		for _, f := range files {

			fileMap := f.(map[string]interface{})

			if l, ok := fileMap["length"].(int); ok {
				totalLength += l
			}
		}
	}

	if totalLength == 0 {
		return nil, errors.New("could not determine torrent size")
	}

	name, ok := info["name"].(string)
	if !ok {
		return nil, errors.New("missing torrent name")
	}

	pieceLen, ok := info["piece length"].(int)
	if !ok {
		return nil, errors.New("missing piece length")
	}

	pieces, ok := info["pieces"].(string)
	if !ok {
		return nil, errors.New("missing pieces")
	}

	// TEMPORARY HASH
	// Later we’ll properly bencode info dict before hashing
	infoBytes := bencode.Encode(info)
	hash := sha1.Sum(infoBytes)

	tf := &TorrentFile{
		Announce: root["announce"].(string),
		Name:     name,
		Length:   totalLength,
		PieceLen: pieceLen,
		Pieces:   []byte(pieces),
		InfoHash: hash,
	}

	return tf, nil
}
