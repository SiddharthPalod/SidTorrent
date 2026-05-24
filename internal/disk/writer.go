package disk

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/SiddharthPalod/SidTorrent/internal/metrics"
	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

const DefaultMaxCacheSize = 4 * 1024 * 1024
const DefaultQueueSize = 32

type WriteRequest struct {
	PieceIndex int
	Data       []byte
}

type DiskWriter struct {
	file *os.File
	tf   *torrent.TorrentFile

	writeChan chan WriteRequest
	errMu     sync.RWMutex
	err       error

	cacheMu      sync.Mutex
	cache        map[int][]byte
	cacheSize    int
	maxCacheSize int

	closeMu   sync.Mutex
	closed    bool
	wg        sync.WaitGroup
}

func NewDiskWriter(tf *torrent.TorrentFile, file *os.File) *DiskWriter {
	dw := &DiskWriter{
		file:         file,
		tf:           tf,
		writeChan:    make(chan WriteRequest, DefaultQueueSize),
		cache:        make(map[int][]byte),
		maxCacheSize: DefaultMaxCacheSize,
	}
	dw.startWriterLoop()
	return dw
}

func (dw *DiskWriter) WritePiece(pieceIndex int, data []byte) error {
	dw.errMu.RLock()
	err := dw.err
	dw.errMu.RUnlock()
	if err != nil {
		return err
	}

	dw.closeMu.Lock()
	if dw.closed {
		dw.closeMu.Unlock()
		return errors.New("disk already closed")
	}
	// Hold lock while sending to guarantee writeChan isn't closed concurrently
	dw.writeChan <- WriteRequest{PieceIndex: pieceIndex, Data: data}
	dw.closeMu.Unlock()
	return nil
}

func (dw *DiskWriter) getError() error {
	dw.errMu.RLock()
	defer dw.errMu.RUnlock()
	return dw.err
}

func (dw *DiskWriter) setError(err error) {
	dw.errMu.Lock()
	dw.err = err
	dw.errMu.Unlock()
}

func (dw *DiskWriter) startWriterLoop() {
	dw.wg.Add(1)
	go func() {
		defer dw.wg.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case req, ok := <-dw.writeChan:
				if !ok {
					dw.flush()
					return
				}
				dw.addToCache(req.PieceIndex, req.Data)
				if dw.getCacheSize() >= dw.maxCacheSize {
					dw.flush()
				}
			case <-ticker.C:
				dw.flush()
			}
		}
	}()
}

func (dw *DiskWriter) addToCache(pieceIndex int, data []byte) {
	dw.cacheMu.Lock()
	defer dw.cacheMu.Unlock()
	// If the piece already exists in the cache, overwrite it
	if old, exists := dw.cache[pieceIndex]; exists {
		dw.cacheSize -= len(old)
	}
	dw.cache[pieceIndex] = data
	dw.cacheSize += len(data)
}

func (dw *DiskWriter) getCacheSize() int {
	dw.cacheMu.Lock()
	defer dw.cacheMu.Unlock()
	return dw.cacheSize
}

func (dw *DiskWriter) Flush() error {
	dw.flush()
	return dw.getError()
}

func (dw *DiskWriter) flush() {
	dw.cacheMu.Lock()
	if len(dw.cache) == 0 {
		dw.cacheMu.Unlock()
		return
	}
	pending := dw.cache
	dw.cache = make(map[int][]byte)
	dw.cacheSize = 0
	dw.cacheMu.Unlock()
	// Sort indices to guarantee sequential offset writes on disk!
	indices := make([]int, 0, len(pending))
	for idx := range pending {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	// Perform sequential writes
	for _, idx := range indices {
		if err := dw.getError(); err != nil {
			return
		}
		data := pending[idx]
		offset := int64(idx) * dw.tf.PieceLength
		
		startTime := time.Now()
		_, err := dw.file.WriteAt(data, offset)
		duration := time.Since(startTime)
		
		if err != nil {
			dw.setError(fmt.Errorf("disk write failed at piece %d (offset %d): %w", idx, offset, err))
			return
		}
		
		// Record disk Write Latency
		metrics.GlobalMetrics.RecordDiskWrite(duration)
	}
}

func (dw *DiskWriter) SetMaxCacheSize(size int) {
	dw.cacheMu.Lock()
	defer dw.cacheMu.Unlock()
	dw.maxCacheSize = size
}

func (dw *DiskWriter) Close() error {
	dw.closeMu.Lock()
	if dw.closed {
		dw.closeMu.Unlock()
		return nil
	}
	dw.closed = true
	close(dw.writeChan) // Causes the startWriterLoop to read remaining then terminate
	dw.closeMu.Unlock()

	dw.wg.Wait()
	return dw.getError()
}
