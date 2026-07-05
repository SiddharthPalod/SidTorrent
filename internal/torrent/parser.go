package torrent

import (
	"crypto/sha1"
	"errors"
	"os"
	"strconv"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
)

type FileEntry struct {
	Length int64
	Path   []string
}

type TorrentFile struct {
	Announce    string
	Trackers    [][]string
	Name        string
	Length      int64
	PieceLength int64
	Pieces      []byte
	InfoHash    [20]byte
	RawInfo     []byte
	Files       []FileEntry
	IsMultiFile bool
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
	var filesList []FileEntry
	var isMultiFile bool

	// SINGLE FILE
	if length, ok := asInt(info["length"]); ok {
		totalLength = length
	}

	// MULTI FILE
	if files, ok := info["files"].([]interface{}); ok {
		isMultiFile = true
		for _, f := range files {
			fileMap, ok := f.(map[string]interface{})
			if !ok {
				continue
			}
			var fileLen int64
			if l, ok := asInt(fileMap["length"]); ok {
				fileLen = l
				totalLength += l
			}
			var pathList []string
			if pList, ok := fileMap["path"].([]interface{}); ok {
				for _, p := range pList {
					if pStr, ok := asString(p); ok {
						pathList = append(pathList, pStr)
					}
				}
			}
			filesList = append(filesList, FileEntry{
				Length: fileLen,
				Path:   pathList,
			})
		}
	}

	if totalLength == 0 {
		return nil, errors.New("could not determine torrent size")
	}

	name, ok := asString(info["name"])
	if !ok {
		return nil, errors.New("missing torrent name")
	}

	if !isMultiFile {
		filesList = append(filesList, FileEntry{
			Length: totalLength,
			Path:   []string{name},
		})
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

	var announce string
	if ann, ok := asString(root["announce"]); ok {
		announce = ann
	}

	trackers := collectTrackerTiers(announce, root["announce-list"])
	if len(trackers) == 0 {
		// Fallback: Inject popular public trackers if the torrent is trackerless/webseed-only
		trackers = [][]string{
			{"udp://tracker.opentrackr.org:1337/announce"},
			{"udp://open.demonii.com:1337/announce"},
			{"udp://exodus.desync.com:6969/announce"},
			{"http://tracker.opentrackr.org:1337/announce"},
		}
	}

	if announce == "" && len(trackers) > 0 && len(trackers[0]) > 0 {
		announce = trackers[0][0]
	}

	tf := &TorrentFile{
		Announce:    announce,
		Trackers:    trackers,
		Name:        name,
		Length:      totalLength,
		PieceLength: pieceLen,
		Pieces:      pieces,
		InfoHash:    hash,
		RawInfo:     rawInfo,
		Files:       filesList,
		IsMultiFile: isMultiFile,
	}

	return tf, nil
}

func collectTrackerTiers(primary string, announceList interface{}) [][]string {
	seen := make(map[string]bool)
	var tiers [][]string
	addTier := func(rawTrackers []string) {
		var tier []string
		for _, raw := range rawTrackers {
			if raw == "" || seen[raw] {
				continue
			}
			seen[raw] = true
			tier = append(tier, raw)
		}
		if len(tier) > 0 {
			tiers = append(tiers, tier)
		}
	}

	addTier([]string{primary})

	rawTiers, ok := announceList.([]interface{})
	if !ok {
		return tiers
	}
	for _, rawTier := range rawTiers {
		rawTrackers, ok := rawTier.([]interface{})
		if !ok {
			continue
		}
		var tier []string
		for _, tracker := range rawTrackers {
			if trackerURL, ok := asString(tracker); ok {
				tier = append(tier, trackerURL)
			}
		}
		addTier(tier)
	}

	return tiers
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
