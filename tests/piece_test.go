package tests

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/peer"
	"github.com/SiddharthPalod/SidTorrent/internal/piece"
)

func TestRequestMessageUsesBigEndianOffsets(
	t *testing.T,
) {

	msg := piece.RequestMessage(
		1,
		16384,
		4096,
	)

	got := msg.Serialize()

	want := []byte{
		0, 0, 0, 13,
		peer.MsgRequest,

		0, 0, 0, 1,

		0, 0, 64, 0,

		0, 0, 16, 0,
	}

	if !bytes.Equal(got, want) {

		t.Fatalf(
			"request bytes = %v, want %v",
			got,
			want,
		)
	}
}

func TestPieceAssemblerAcceptsShortSingleBlockPiece(
	t *testing.T,
) {

	assembler := piece.NewPieceAssembler(
		0,
		5,
	)

	if err := assembler.AddBlock(
		0,
		[]byte("hello"),
	); err != nil {

		t.Fatalf(
			"AddBlock() error = %v",
			err,
		)
	}

	if !assembler.IsComplete() {

		t.Fatal(
			"assembler is incomplete after receiving full short piece",
		)
	}

	if !bytes.Equal(
		assembler.Data,
		[]byte("hello"),
	) {

		t.Fatalf(
			"assembler data = %q, want hello",
			assembler.Data,
		)
	}
}

func TestDownloadPieceSendsInterestedBeforeRequestWhenChoked(
	t *testing.T,
) {

	clientConn, serverConn := net.Pipe()

	defer clientConn.Close()
	defer serverConn.Close()

	client := &peer.Client{
		Conn: clientConn,
		State: peer.PeerState{
			Choked:     true,
			LastActive: time.Now(),
		},
	}

	wantBlock := []byte("hello")

	errc := make(chan error, 1)

	go func() {

		// interested
		msg, err := peer.ReadMessage(
			serverConn,
		)

		if err != nil {
			errc <- err
			return
		}

		if msg.ID != peer.MsgInterested {

			errc <- errors.New(
				"first message was not interested",
			)

			return
		}

		// unchoke
		_, err = serverConn.Write(
			(&peer.Message{
				ID: peer.MsgUnchoke,
			}).Serialize(),
		)

		if err != nil {
			errc <- err
			return
		}

		// request
		req, err := peer.ReadMessage(
			serverConn,
		)

		if err != nil {
			errc <- err
			return
		}

		if req.ID != peer.MsgRequest {

			errc <- errors.New(
				"second message was not request",
			)

			return
		}

		piecePayload := make(
			[]byte,
			8+len(wantBlock),
		)

		binary.BigEndian.PutUint32(
			piecePayload[0:4],
			0,
		)

		binary.BigEndian.PutUint32(
			piecePayload[4:8],
			0,
		)

		copy(
			piecePayload[8:],
			wantBlock,
		)

		_, err = serverConn.Write(
			(&peer.Message{
				ID:      peer.MsgPiece,
				Payload: piecePayload,
			}).Serialize(),
		)

		errc <- err
	}()
	got, err := piece.DownloadPiece(
		client,
		0,
		len(wantBlock),
		nil,
	)

	if err != nil {

		t.Fatalf(
			"DownloadPiece() error = %v",
			err,
		)
	}

	if !bytes.Equal(got, wantBlock) {

		t.Fatalf(
			"DownloadPiece() = %q, want %q",
			got,
			wantBlock,
		)
	}

	if err := <-errc; err != nil {

		t.Fatalf(
			"server error = %v",
			err,
		)
	}
}

func TestDownloadPieceReturnsEOFWhenPeerDisconnects(
	t *testing.T,
) {

	clientConn, serverConn := net.Pipe()

	client := &peer.Client{
		Conn: clientConn,
		State: peer.PeerState{
			Choked:     false,
			LastActive: time.Now(),
		},
	}

	go func() {

		_, _ = peer.ReadMessage(
			serverConn,
		)

		_ = serverConn.Close()
	}()

	_, err := piece.DownloadPiece(
		client,
		0,
		1,
		nil,
	)

	if !errors.Is(err, io.ErrClosedPipe) &&
		!errors.Is(err, io.EOF) {

		t.Fatalf(
			"DownloadPiece() error = %v, want EOF/closed pipe",
			err,
		)
	}

	_ = clientConn.Close()
}

func TestDownloadPieceRejectsIncorrectBlockOffset(
	t *testing.T,
) {

	clientConn, serverConn := net.Pipe()

	defer clientConn.Close()
	defer serverConn.Close()

	client := &peer.Client{
		Conn: clientConn,
		State: peer.PeerState{
			Choked:     false,
			LastActive: time.Now(),
		},
	}

	go func() {

		_, _ = peer.ReadMessage(
			serverConn,
		)

		piecePayload := make([]byte, 9)

		binary.BigEndian.PutUint32(
			piecePayload[0:4],
			0,
		)

		// WRONG OFFSET
		binary.BigEndian.PutUint32(
			piecePayload[4:8],
			1,
		)

		piecePayload[8] = 'x'

		_, _ = serverConn.Write(
			(&peer.Message{
				ID:      peer.MsgPiece,
				Payload: piecePayload,
			}).Serialize(),
		)
	}()

	_, err := piece.DownloadPiece(
		client,
		0,
		1,
		nil,
	)

	if !errors.Is(
		err,
		piece.ErrInvalidPieceBlock,
	) {

		t.Fatalf(
			"DownloadPiece() error = %v, want ErrInvalidPieceBlock",
			err,
		)
	}
}

func TestDownloadPieceReturnsTimeoutIfChokedForever(
	t *testing.T,
) {

	clientConn, serverConn := net.Pipe()

	defer clientConn.Close()
	defer serverConn.Close()

	client := &peer.Client{
		Conn: clientConn,
		State: peer.PeerState{
			Choked:     true,
			LastActive: time.Now(),
		},
	}

	_ = clientConn.SetDeadline(
		time.Now().Add(
			20 * time.Millisecond,
		),
	)

	go func() {

		_, _ = peer.ReadMessage(
			serverConn,
		)
	}()

	_, err := piece.DownloadPiece(
		client,
		0,
		1,
		nil,
	)

	if netErr, ok := err.(net.Error); !ok ||
		!netErr.Timeout() {

		t.Fatalf(
			"DownloadPiece() error = %v, want timeout",
			err,
		)
	}
}

func TestPieceAssemblerInitializesRequestedSlice(t *testing.T) {
	assembler := piece.NewPieceAssembler(0, 32768) // 2 blocks
	if len(assembler.Requested) != 2 {
		t.Fatalf("expected Requested length of 2, got %d", len(assembler.Requested))
	}

	offset, length, ok := assembler.NextMissingBlock()
	if !ok || offset != 0 || length != piece.BlockSize {
		t.Fatalf("expected first missing block, got offset=%d, length=%d, ok=%t", offset, length, ok)
	}
}
