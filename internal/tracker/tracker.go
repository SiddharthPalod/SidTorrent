package tracker

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

type Peer struct {
	IP   net.IP
	Port uint16
}

func GetPeers(tf *torrent.TorrentFile) ([]Peer, error) {

	peerID := randomPeerID()

	base, err := url.Parse(tf.Announce)
	if err != nil {
		return nil, err
	}

	params := url.Values{
		"info_hash":  []string{string(tf.InfoHash[:])},
		"peer_id":    []string{peerID},
		"port":       []string{"6881"},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"left":       []string{strconv.FormatInt(tf.Length, 10)},
		"compact":    []string{"1"},
	}

	base.RawQuery = params.Encode()

	fmt.Println("tracker url:")
	fmt.Println(base.String())

	resp, err := http.Get(base.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	decoded, err := bencode.DecodeBytes(body)
	if err != nil {
		return nil, err
	}

	root := decoded.(map[string]interface{})

	peerBytes, ok := root["peers"].([]byte)
	if !ok {
		return nil, fmt.Errorf("tracker response missing peers")
	}

	return parseCompactPeers(peerBytes), nil
}

func parseCompactPeers(data []byte) []Peer {

	var peers []Peer

	for i := 0; i+6 <= len(data); i += 6 {

		ip := net.IP(data[i : i+4])

		port := binary.BigEndian.Uint16(data[i+4 : i+6])

		fmt.Printf(
			"peer => %s:%d\n",
			ip.String(),
			port,
		)

		peers = append(peers, Peer{
			IP:   ip,
			Port: port,
		})
	}

	return peers
}

func randomPeerID() string {
	return fmt.Sprintf("-SD0001-%012d", rand.Intn(999999999999))
}
