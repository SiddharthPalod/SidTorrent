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

	name, ok := asString(info["name"])
	if !ok {
		return nil, errors.New("missing torrent name")
	}

	pieceLen, ok := asInt(info["piece length"])
	if !ok {
		return nil, errors.New("missing piece length")
	}

	pieces, ok := asBytes(info["pieces"])
	if !ok {
		return nil, errors.New("missing pieces")
	}

	// TEMPORARY HASH
	// Later we’ll properly bencode info dict before hashing
	infoBytes := bencode.Encode(info)
	hash := sha1.Sum(infoBytes)

	announce, ok := asString(root["announce"])
	if !ok {
		return nil, errors.New("missing announce url")
	}

	tf := &TorrentFile{
		Announce: announce,
		Name:     name,
		Length:   totalLength,
		PieceLen: pieceLen,
		Pieces:   pieces,
		InfoHash: hash,
	}

	return tf, nil
}

func asString(v interface{}) (string, bool) {

	switch val := v.(type) {

	case string:
		return val, true

	case []byte:
		return string(val), true

	default:
		return "", false
	}
}

func asBytes(v interface{}) ([]byte, bool) {

	switch val := v.(type) {

	case []byte:
		return val, true

	case string:
		return []byte(val), true

	default:
		return nil, false
	}
}

func asInt(v interface{}) (int, bool) {

	switch val := v.(type) {

	case int:
		return val, true

	case int64:
		return int(val), true

	default:
		return 0, false
	}
}
