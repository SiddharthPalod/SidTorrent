package tests

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
)

func TestReadMessageReturnsEOFWhenPeerDisconnects(t *testing.T) {
	if _, err := peer.ReadMessage(bytes.NewReader([]byte{0, 0})); err == nil {
		t.Fatal("ReadMessage() succeeded after short read, want EOF")
	}
}

func TestValidateBitfieldRejectsMalformedPayloads(t *testing.T) {
	tests := []struct {
		name       string
		payload    []byte
		pieceCount int
	}{
		{name: "wrong length", payload: []byte{0xff}, pieceCount: 9},
		{name: "spare bits set", payload: []byte{0xff}, pieceCount: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := peer.ValidateBitfield(tt.payload, tt.pieceCount)
			if !errors.Is(err, peer.ErrMalformedBitfield) {
				t.Fatalf("ValidateBitfield() error = %v, want ErrMalformedBitfield", err)
			}
		})
	}
}

func TestValidateBitfieldAcceptsValidPayload(t *testing.T) {
	if err := peer.ValidateBitfield([]byte{0xfe}, 7); err != nil {
		t.Fatalf("ValidateBitfield() error = %v", err)
	}
}

func TestConnectTimeoutReturnsWhenPeerGoesSilent(t *testing.T) {
	ln := listenLocal(t)
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.ReadFull(conn, make([]byte, 68))
		time.Sleep(200 * time.Millisecond)
	}()

	var infoHash [20]byte
	_, err := peer.ConnectTimeout(ln.Addr().String(), infoHash, 20*time.Millisecond)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
			t.Fatalf("ConnectTimeout() error = %v, want timeout", err)
		}
	}
	<-done
}

func TestConnectTimeoutReturnsEOFWhenPeerDisconnects(t *testing.T) {
	ln := listenLocal(t)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_, _ = io.ReadFull(conn, make([]byte, 68))
		_ = conn.Close()
	}()

	var infoHash [20]byte
	_, err := peer.ConnectTimeout(ln.Addr().String(), infoHash, time.Second)
	if !isDisconnect(err) {
		t.Fatalf("ConnectTimeout() error = %v, want disconnect", err)
	}
}

func listenLocal(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	return ln
}

func isDisconnect(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "forcibly closed")
}
