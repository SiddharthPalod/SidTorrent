package disk

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

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

	closeChan chan struct{}
	wg        sync.WaitGroup
	closed    bool
}

func NewDiskWriter(tf *torrent.TorrentFile, file *os.File) *DiskWriter {
	dw := &DiskWriter{
		file:         file,
		tf:           tf,
		writeChan:    make(chan WriteRequest, DefaultQueueSize),
		cache:        make(map[int][]byte),
		maxCacheSize: DefaultMaxCacheSize,
		closeChan:    make(chan struct{}),
	}
	dw.startWriterLoop()
	return dw
}

func (dw *DiskWriter) WritePiece(pieceIndex int, data []byte) error {
	dw.errMu.RLock()
	defer dw.errMu.RUnlock()
	if dw.err != nil {
		return dw.err
	}
	dw.cacheMu.Lock()
	closed := dw.closed
	dw.cacheMu.Unlock()
	if closed {
		return errors.New("disk already closed")
	}

	select {
	case dw.writeChan <- WriteRequest{PieceIndex: pieceIndex, Data: data}:
		return nil
	case <-dw.closeChan:
		return errors.New("disk writer closed")
	}
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
			case <-dw.closeChan:
				for {
					select {
					case req := <-dw.writeChan:
						dw.addToCache(req.PieceIndex, req.Data)
					default:
						dw.flush()
						return
					}
				}
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
		_, err := dw.file.WriteAt(data, offset)
		if err != nil {
			dw.setError(fmt.Errorf("disk write failed at piece %d (offset %d): %w", idx, offset, err))
			return
		}
	}
}

func (dw *DiskWriter) SetMaxCacheSize(size int) {
	dw.cacheMu.Lock()
	defer dw.cacheMu.Unlock()
	dw.maxCacheSize = size
}

func (dw *DiskWriter) Close() error {
	dw.cacheMu.Lock()
	if dw.closed {
		dw.cacheMu.Unlock()
		return nil
	}
	dw.closed = true
	dw.cacheMu.Unlock()
	close(dw.closeChan)
	dw.wg.Wait()
	return dw.getError()
}
