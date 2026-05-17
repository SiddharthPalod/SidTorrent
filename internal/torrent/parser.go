package torrent

import (
	"crypto/sha1"
	"errors"
	"os"
	"strconv"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
)

type TorrentFile struct {
	Announce    string
	Name        string
	Length      int64
	PieceLength int64
	Pieces      []byte
	InfoHash    [20]byte
	RawInfo     []byte
}

func Open(path string) (*TorrentFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	rootNode, err := bencode.DecodeWithRaw(data)
	if err != nil {
		return nil, err
	}

	root, ok := rootNode.Value.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid torrent root")
	}

	infoNode, ok := rootNode.Dict["info"]
	if !ok {
		return nil, errors.New("missing info dictionary")
	}

	info, ok := root["info"].(map[string]interface{})
	if !ok {
		return nil, errors.New("missing info dictionary")
	}

	var totalLength int64

	// SINGLE FILE
	if length, ok := asInt(info["length"]); ok {
		totalLength = length
	}

	// MULTI FILE
	if files, ok := info["files"].([]interface{}); ok {

		for _, f := range files {

			fileMap := f.(map[string]interface{})

			if l, ok := asInt(fileMap["length"]); ok {
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

	rawInfo := append([]byte(nil), data[infoNode.Start:infoNode.End]...)
	hash := sha1.Sum(rawInfo)

	announce, ok := asString(root["announce"])
	if !ok {
		return nil, errors.New("missing announce url")
	}

	tf := &TorrentFile{
		Announce:    announce,
		Name:        name,
		Length:      totalLength,
		PieceLength: pieceLen,
		Pieces:      pieces,
		InfoHash:    hash,
		RawInfo:     rawInfo,
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

func asInt(v interface{}) (int64, bool) {

	switch val := v.(type) {

	case int:
		return int64(val), true

	case int64:
		return val, true

	case string:
		i, err := strconv.ParseInt(val, 10, 64)
		return i, err == nil

	default:
		return 0, false
	}
}
