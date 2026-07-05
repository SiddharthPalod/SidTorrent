package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/piece"
	"github.com/SiddharthPalod/SidTorrent/internal/session"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

type Server struct {
	staticDir string
	uploadDir string
	jobs      *JobStore
}

type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

type Job struct {
	ID        string          `json:"id"`
	State     string          `json:"state"`
	Error     string          `json:"error,omitempty"`
	Request   session.Request `json:"request"`
	Status    session.Status  `json:"status"`
	CreatedAt int64           `json:"createdAt"`
	IsVideo   bool            `json:"isVideo"`
	VideoName string          `json:"videoName"`

	cancel     context.CancelFunc
	PM         *piece.PieceManager  `json:"-"`
	TF         *torrent.TorrentFile `json:"-"`
	OutputPath string               `json:"-"`
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

func NewServer(staticDir string) http.Handler {
	server := &Server{
		staticDir: staticDir,
		uploadDir: filepath.Join("uploads", "torrents"),
		jobs: &JobStore{
			jobs: make(map[string]*Job),
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", server.health)
	mux.HandleFunc("/api/torrents/upload", server.uploadTorrent)
	mux.HandleFunc("/api/torrents/inspect", server.inspectTorrent)
	mux.HandleFunc("/api/downloads", server.downloads)
	mux.HandleFunc("/api/downloads/stream/", server.streamDownloadByID)
	mux.HandleFunc("/api/downloads/", server.downloadByID)
	mux.Handle("/", noCache(http.FileServer(http.Dir(staticDir))))

	return cors(mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "online"})
}

func (s *Server) uploadTorrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(8 * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart upload")
		return
	}

	file, header, err := r.FormFile("torrent")
	if err != nil {
		writeError(w, http.StatusBadRequest, "torrent file is required")
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".torrent") {
		writeError(w, http.StatusBadRequest, "only .torrent files are supported")
		return
	}
	if header.Size > 8*1024*1024 {
		writeError(w, http.StatusBadRequest, "torrent file is too large")
		return
	}

	id, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.MkdirAll(s.uploadDir, os.ModePerm); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	filename := fmt.Sprintf("%s-%s", id, safeFilename(header.Filename))
	path := filepath.Join(s.uploadDir, filename)
	out, err := os.Create(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	tf, err := torrent.Open(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("uploaded file is not a valid torrent: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"path":        path,
		"filename":    header.Filename,
		"name":        tf.Name,
		"length":      tf.Length,
		"pieceLength": tf.PieceLength,
		"pieceCount":  tf.PieceCount(),
		"announce":    tf.Announce,
		"trackers":    tf.Trackers,
		"infoHash":    hex.EncodeToString(tf.InfoHash[:]),
		"defaultOut":  filepath.Join("downloads", tf.Name),
	})
}

func (s *Server) inspectTorrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	tf, err := torrent.Open(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":        tf.Name,
		"length":      tf.Length,
		"pieceLength": tf.PieceLength,
		"pieceCount":  tf.PieceCount(),
		"announce":    tf.Announce,
		"trackers":    tf.Trackers,
		"infoHash":    hex.EncodeToString(tf.InfoHash[:]),
		"defaultOut":  filepath.Join("downloads", tf.Name),
	})
}

func (s *Server) downloads(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.jobs.List())
	case http.MethodPost:
		s.startDownload(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) startDownload(w http.ResponseWriter, r *http.Request) {
	var req session.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.TorrentPath) == "" {
		writeError(w, http.StatusBadRequest, "torrentPath is required")
		return
	}

	id, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &Job{
		ID:        id,
		State:     "queued",
		Request:   req,
		CreatedAt: time.Now().Unix(),
		cancel:    cancel,
	}
	s.jobs.Put(job)

	go func() {
		s.jobs.Update(id, func(job *Job) {
			job.State = "running"
			job.Status.Phase = "queued"
			job.Status.Message = "Download queued"
		})
		err := session.Download(ctx, req, session.Options{
			OnStatus: func(status session.Status) {
				s.jobs.Update(id, func(job *Job) {
					job.Status = status
				})
			},
			OnStarted: func(pm *piece.PieceManager, tf *torrent.TorrentFile, outputPath string) {
				s.jobs.Update(id, func(job *Job) {
					job.PM = pm
					job.TF = tf
					job.OutputPath = outputPath

					vf, err := findVideoFile(tf, outputPath)
					if err == nil {
						ext := strings.ToLower(filepath.Ext(vf.Path))
						videoExtensions := map[string]bool{
							".mp4":  true,
							".mkv":  true,
							".webm": true,
							".avi":  true,
							".mov":  true,
							".mp3":  true,
						}
						if videoExtensions[ext] {
							job.IsVideo = true
							job.VideoName = filepath.Base(vf.Path)
						}
					}
				})
			},
		})
		s.jobs.Update(id, func(job *Job) {
			if err != nil {
				if ctx.Err() != nil {
					job.State = "cancelled"
					job.Status.Phase = "cancelled"
					job.Status.Message = "Download cancelled"
				} else {
					job.State = "failed"
					job.Error = err.Error()
					job.Status.Phase = "failed"
					job.Status.Message = err.Error()
				}
				return
			}
			job.State = "complete"
		})
	}()

	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) downloadByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/downloads/")
	if id == "" {
		writeError(w, http.StatusNotFound, "download not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		job, ok := s.jobs.Get(id)
		if !ok {
			writeError(w, http.StatusNotFound, "download not found")
			return
		}
		writeJSON(w, http.StatusOK, job)
	case http.MethodDelete:
		job, ok := s.jobs.Cancel(id)
		if !ok {
			writeError(w, http.StatusNotFound, "download not found")
			return
		}
		writeJSON(w, http.StatusOK, job)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *JobStore) Put(job *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}

func (s *JobStore) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, false
	}
	return cloneJob(job), true
}

func (s *JobStore) List() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, cloneJob(job))
	}
	return jobs
}

func (s *JobStore) Update(id string, update func(*Job)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[id]; ok {
		update(job)
	}
}

func (s *JobStore) Cancel(id string) (*Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, false
	}
	if job.cancel != nil {
		job.cancel()
	}
	job.State = "cancelled"
	job.Status.Phase = "cancelled"
	job.Status.Message = "Download cancelled"
	return cloneJob(job), true
}

func cloneJob(job *Job) *Job {
	next := *job
	next.cancel = nil
	return &next
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:") {
			if origin == "" {
				origin = "*"
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func newID() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

func safeFilename(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "upload.torrent"
	}
	return b.String()
}

type VideoFile struct {
	Path        string
	StartOffset int64
	Length      int64
}

func findVideoFile(tf *torrent.TorrentFile, outputPath string) (VideoFile, error) {
	files := tf.Files
	if len(files) == 0 {
		return VideoFile{
			Path:        outputPath,
			StartOffset: 0,
			Length:      tf.Length,
		}, nil
	}

	var bestEntry torrent.FileEntry
	var bestStart int64
	var currentOffset int64
	foundVideo := false

	videoExtensions := map[string]bool{
		".mp4":  true,
		".mkv":  true,
		".webm": true,
		".avi":  true,
		".mov":  true,
		".mp3":  true,
	}

	for _, fe := range files {
		ext := strings.ToLower(filepath.Ext(filepath.Join(fe.Path...)))
		if videoExtensions[ext] {
			if !foundVideo || fe.Length > bestEntry.Length {
				bestEntry = fe
				bestStart = currentOffset
				foundVideo = true
			}
		} else if !foundVideo && fe.Length > bestEntry.Length {
			bestEntry = fe
			bestStart = currentOffset
		}
		currentOffset += fe.Length
	}

	if bestEntry.Length == 0 {
		return VideoFile{}, fmt.Errorf("no files found in torrent")
	}

	return VideoFile{
		Path:        filepath.Join(outputPath, filepath.Join(bestEntry.Path...)),
		StartOffset: bestStart,
		Length:      bestEntry.Length,
	}, nil
}

type StreamingReader struct {
	pm          *piece.PieceManager
	tf          *torrent.TorrentFile
	file        *os.File
	startOffset int64
	fileLength  int64
	offset      int64
	mu          sync.Mutex
}

func NewStreamingReader(pm *piece.PieceManager, tf *torrent.TorrentFile, vf VideoFile) (*StreamingReader, error) {
	file, err := os.Open(vf.Path)
	if err != nil {
		return nil, err
	}
	return &StreamingReader{
		pm:          pm,
		tf:          tf,
		file:        file,
		startOffset: vf.StartOffset,
		fileLength:  vf.Length,
	}, nil
}

func (sr *StreamingReader) Seek(offset int64, whence int) (int64, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = sr.offset + offset
	case io.SeekEnd:
		newOffset = sr.fileLength + offset
	default:
		return 0, fmt.Errorf("invalid whence")
	}

	if newOffset < 0 || newOffset > sr.fileLength {
		return 0, fmt.Errorf("seek out of bounds")
	}
	sr.offset = newOffset
	return sr.offset, nil
}

func (sr *StreamingReader) Read(p []byte) (n int, err error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.offset >= sr.fileLength {
		return 0, io.EOF
	}

	limit := int64(len(p))
	if sr.offset+limit > sr.fileLength {
		limit = sr.fileLength - sr.offset
	}

	if limit == 0 {
		return 0, nil
	}

	torrentStartOffset := sr.startOffset + sr.offset
	torrentEndOffset := torrentStartOffset + limit - 1

	pieceLength := sr.tf.PieceLength
	startPiece := int(torrentStartOffset / pieceLength)
	endPiece := int(torrentEndOffset / pieceLength)

	var priorityIndices []int
	for i := startPiece; i <= endPiece; i++ {
		priorityIndices = append(priorityIndices, i)
	}
	sr.pm.SetPriorityPieces(priorityIndices)

	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var timedOut bool
	for !sr.pm.IsPieceComplete(startPiece) {
		select {
		case <-ticker.C:
			// check again
		case <-timeout:
			timedOut = true
			break
		}
		if timedOut {
			break
		}
	}

	if timedOut && !sr.pm.IsPieceComplete(startPiece) {
		return 0, fmt.Errorf("timeout waiting for piece %d to download", startPiece)
	}

	n, err = sr.file.ReadAt(p[:limit], sr.offset)
	sr.offset += int64(n)
	return n, err
}

func (sr *StreamingReader) Close() error {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.file.Close()
}

func (s *Server) streamDownloadByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/downloads/stream/")
	if id == "" {
		writeError(w, http.StatusNotFound, "download not found")
		return
	}

	s.jobs.mu.RLock()
	job, ok := s.jobs.jobs[id]
	s.jobs.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "download not found")
		return
	}

	if job.PM == nil || job.TF == nil {
		writeError(w, http.StatusServiceUnavailable, "session not initialized yet")
		return
	}

	outputPath := job.OutputPath
	if outputPath == "" {
		outputPath = job.Status.OutputPath
	}

	vf, err := findVideoFile(job.TF, outputPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	reader, err := NewStreamingReader(job.PM, job.TF, vf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("open video stream: %v", err))
		return
	}
	defer reader.Close()

	ext := strings.ToLower(filepath.Ext(vf.Path))
	switch ext {
	case ".mp4":
		w.Header().Set("Content-Type", "video/mp4")
	case ".mkv":
		w.Header().Set("Content-Type", "video/x-matroska")
	case ".webm":
		w.Header().Set("Content-Type", "video/webm")
	case ".avi":
		w.Header().Set("Content-Type", "video/x-msvideo")
	case ".mov":
		w.Header().Set("Content-Type", "video/quicktime")
	case ".mp3":
		w.Header().Set("Content-Type", "audio/mpeg")
	}

	http.ServeContent(w, r, filepath.Base(vf.Path), time.Unix(job.CreatedAt, 0), reader)
}
