package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHealthIncludesCORSForLocalhost(t *testing.T) {
	server := NewServer(filepath.Join("..", "..", "frontend", "dist"))
	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want localhost origin", got)
	}
}

func TestInspectTorrent(t *testing.T) {
	server := NewServer(filepath.Join("..", "..", "frontend", "dist"))
	body := bytes.NewBufferString(`{"path":"../../testdata/ubuntu.torrent"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/torrents/inspect", body)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Name       string `json:"name"`
		PieceCount int    `json:"pieceCount"`
		InfoHash   string `json:"infoHash"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if payload.Name != "ubuntu-26.04-desktop-amd64.iso" {
		t.Fatalf("Name = %q, want ubuntu torrent", payload.Name)
	}
	if payload.PieceCount == 0 {
		t.Fatal("PieceCount is zero")
	}
	if len(payload.InfoHash) != 40 {
		t.Fatalf("InfoHash length = %d, want 40", len(payload.InfoHash))
	}
}

func TestUploadTorrent(t *testing.T) {
	server := NewServer(filepath.Join("..", "..", "frontend", "dist"))
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("torrent", "ubuntu.torrent")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	source, err := os.Open(filepath.Join("..", "..", "testdata", "ubuntu.torrent"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer source.Close()
	if _, err := io.Copy(part, source); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/torrents/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if payload.Path == "" {
		t.Fatal("Path is empty")
	}
	if payload.Name != "ubuntu-26.04-desktop-amd64.iso" {
		t.Fatalf("Name = %q, want ubuntu torrent", payload.Name)
	}
}
