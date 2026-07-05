package tests

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/bencode"
	"github.com/SiddharthPalod/SidTorrent/internal/session"
)

func runMockPeer(t *testing.T, payload []byte, pieceLen int64) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for mock peer: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// 1. Read handshake (68 bytes)
		hsBuf := make([]byte, 68)
		_, err = io.ReadFull(conn, hsBuf)
		if err != nil {
			return
		}

		// 2. Write handshake
		var infoHash [20]byte
		copy(infoHash[:], hsBuf[28:48])
		var peerID [20]byte
		copy(peerID[:], []byte("-SD001-mockpeer12345"))
		respHs := make([]byte, 68)
		respHs[0] = 19
		copy(respHs[1:20], []byte("BitTorrent protocol"))
		copy(respHs[28:48], infoHash[:])
		copy(respHs[48:68], peerID[:])
		_, _ = conn.Write(respHs)

		// 3. Send Bitfield
		totalPieces := len(payload) / int(pieceLen)
		if len(payload)%int(pieceLen) != 0 {
			totalPieces++
		}
		bfBytesCount := totalPieces / 8
		if totalPieces%8 != 0 {
			bfBytesCount++
		}
		bfBytes := make([]byte, bfBytesCount)
		for i := 0; i < totalPieces; i++ {
			bfBytes[i/8] |= 1 << (7 - (i % 8))
		}

		bfMsgPayload := make([]byte, 5+len(bfBytes))
		binary.BigEndian.PutUint32(bfMsgPayload[0:4], uint32(1+len(bfBytes)))
		bfMsgPayload[4] = 5 // MsgBitfield
		copy(bfMsgPayload[5:], bfBytes)
		_, _ = conn.Write(bfMsgPayload)

		// 4. Read interested message
		interestBuf := make([]byte, 5)
		_, err = io.ReadFull(conn, interestBuf)
		if err != nil {
			return
		}

		// 5. Send unchoke
		unchokeMsg := []byte{0, 0, 0, 1, 1}
		_, _ = conn.Write(unchokeMsg)

		// 6. Handle block requests
		reqBuf := make([]byte, 17)
		for {
			_, err = io.ReadFull(conn, reqBuf[0:4])
			if err != nil {
				return // EOF
			}
			msgLen := binary.BigEndian.Uint32(reqBuf[0:4])
			if msgLen == 0 {
				continue
			}
			_, err = io.ReadFull(conn, reqBuf[4:4+msgLen])
			if err != nil {
				return
			}

			msgID := reqBuf[4]
			if msgID == 6 { // Request
				pieceIndex := int(binary.BigEndian.Uint32(reqBuf[5:9]))
				begin := int(binary.BigEndian.Uint32(reqBuf[9:13]))
				reqLength := int(binary.BigEndian.Uint32(reqBuf[13:17]))

				globalOffset := int64(pieceIndex)*pieceLen + int64(begin)
				if globalOffset+int64(reqLength) > int64(len(payload)) {
					return
				}

				block := payload[globalOffset : globalOffset+int64(reqLength)]
				respMsg := make([]byte, 4+1+4+4+len(block))
				binary.BigEndian.PutUint32(respMsg[0:4], uint32(9+len(block)))
				respMsg[4] = 7 // MsgPiece
				binary.BigEndian.PutUint32(respMsg[5:9], uint32(pieceIndex))
				binary.BigEndian.PutUint32(respMsg[9:13], uint32(begin))
				copy(respMsg[13:], block)
				_, _ = conn.Write(respMsg)
			}
		}
	}()

	return listener.Addr().String(), func() {
		_ = listener.Close()
		<-done
	}
}

func runMockTracker(t *testing.T, peerAddr string) *httptest.Server {
	t.Helper()
	host, portStr, err := net.SplitHostPort(peerAddr)
	if err != nil {
		t.Fatalf("invalid peer address: %v", err)
	}
	var port uint16
	_, _ = fmt.Sscanf(portStr, "%d", &port)
	peerIP := net.ParseIP(host)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peerBytes := make([]byte, 6)
		copy(peerBytes[0:4], peerIP.To4())
		binary.BigEndian.PutUint16(peerBytes[4:6], port)

		respDict := map[string]interface{}{
			"interval": int64(1800),
			"peers":    peerBytes,
		}
		_, _ = w.Write(bencode.Encode(respDict))
	}))

	return server
}

func calculatePiecesHash(payload []byte, pieceLen int64) []byte {
	var buf bytes.Buffer
	for i := 0; int64(i)*pieceLen < int64(len(payload)); i++ {
		start := int64(i) * pieceLen
		end := start + pieceLen
		if end > int64(len(payload)) {
			end = int64(len(payload))
		}
		hash := sha1.Sum(payload[start:end])
		buf.Write(hash[:])
	}
	return buf.Bytes()
}

func TestE2EDownloadSingleFile(t *testing.T) {
	payload := []byte("hello integration testing single file payload 1234567890abcdefghijklmnopqrstuvwxyz")
	pieceLen := int64(16)

	peerAddr, cleanupPeer := runMockPeer(t, payload, pieceLen)
	defer cleanupPeer()

	trackerServer := runMockTracker(t, peerAddr)
	defer trackerServer.Close()

	// Build metainfo bencode
	piecesHash := calculatePiecesHash(payload, pieceLen)
	infoDict := map[string]interface{}{
		"name":         "test_single.txt",
		"piece length": pieceLen,
		"pieces":       piecesHash,
		"length":       int64(len(payload)),
	}
	root := map[string]interface{}{
		"announce": trackerServer.URL,
		"info":     infoDict,
	}

	tempDir := t.TempDir()
	torrentPath := filepath.Join(tempDir, "test.torrent")
	err := os.WriteFile(torrentPath, bencode.Encode(root), 0644)
	if err != nil {
		t.Fatalf("failed to write torrent file: %v", err)
	}

	outputPath := filepath.Join(tempDir, "downloaded_single.txt")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Disable DHT metrics choking options to keep it isolated to this peer
	req := session.Request{
		TorrentPath:     torrentPath,
		OutputPath:      outputPath,
		EnableDHT:       session.Bool(false),
		EnablePEX:       session.Bool(false),
		EnableStreaming: session.Bool(false),
		EnableChoking:   session.Bool(false),
		EnableMetrics:   session.Bool(false),
	}

	err = session.Download(ctx, req, session.Options{})
	if err != nil {
		t.Fatalf("Download() failed: %v", err)
	}

	// Verify file content
	downloadedData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if !bytes.Equal(downloadedData, payload) {
		t.Fatalf("downloaded data mismatch: got %q, want %q", string(downloadedData), string(payload))
	}
}

func TestE2EDownloadMultiFile(t *testing.T) {
	// 50 bytes total payload divided into 3 files:
	// file1.txt (10 bytes), file2.txt (25 bytes), sub/file3.txt (15 bytes)
	payload := []byte("this is a multi-file integration test payload data")
	pieceLen := int64(16)

	peerAddr, cleanupPeer := runMockPeer(t, payload, pieceLen)
	defer cleanupPeer()

	trackerServer := runMockTracker(t, peerAddr)
	defer trackerServer.Close()

	// Build metainfo bencode
	piecesHash := calculatePiecesHash(payload, pieceLen)
	infoDict := map[string]interface{}{
		"name":         "multi_dir",
		"piece length": pieceLen,
		"pieces":       piecesHash,
		"files": []interface{}{
			map[string]interface{}{"length": int64(10), "path": []interface{}{"file1.txt"}},
			map[string]interface{}{"length": int64(25), "path": []interface{}{"file2.txt"}},
			map[string]interface{}{"length": int64(15), "path": []interface{}{"sub", "file3.txt"}},
		},
	}
	root := map[string]interface{}{
		"announce": trackerServer.URL,
		"info":     infoDict,
	}

	tempDir := t.TempDir()
	torrentPath := filepath.Join(tempDir, "test_multi.torrent")
	err := os.WriteFile(torrentPath, bencode.Encode(root), 0644)
	if err != nil {
		t.Fatalf("failed to write torrent file: %v", err)
	}

	outputPath := filepath.Join(tempDir, "downloaded_multi_dir")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := session.Request{
		TorrentPath:     torrentPath,
		OutputPath:      outputPath,
		EnableDHT:       session.Bool(false),
		EnablePEX:       session.Bool(false),
		EnableStreaming: session.Bool(false),
		EnableChoking:   session.Bool(false),
		EnableMetrics:   session.Bool(false),
	}

	err = session.Download(ctx, req, session.Options{})
	if err != nil {
		t.Fatalf("Download() failed: %v", err)
	}

	// Verify multi-file layout and contents
	f1Data, err := os.ReadFile(filepath.Join(outputPath, "file1.txt"))
	if err != nil {
		t.Fatalf("failed to read file1.txt: %v", err)
	}
	if !bytes.Equal(f1Data, payload[0:10]) {
		t.Fatalf("file1 data mismatch: got %q, want %q", string(f1Data), string(payload[0:10]))
	}

	f2Data, err := os.ReadFile(filepath.Join(outputPath, "file2.txt"))
	if err != nil {
		t.Fatalf("failed to read file2.txt: %v", err)
	}
	if !bytes.Equal(f2Data, payload[10:35]) {
		t.Fatalf("file2 data mismatch: got %q, want %q", string(f2Data), string(payload[10:35]))
	}

	f3Data, err := os.ReadFile(filepath.Join(outputPath, "sub", "file3.txt"))
	if err != nil {
		t.Fatalf("failed to read sub/file3.txt: %v", err)
	}
	if !bytes.Equal(f3Data, payload[35:50]) {
		t.Fatalf("file3 data mismatch: got %q, want %q", string(f3Data), string(payload[35:50]))
	}
}
