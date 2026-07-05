package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SiddharthPalod/SidTorrent/internal/torrent"
)

type Storage interface {
	io.ReaderAt
	io.WriterAt
	io.Closer
}

type physicalFile struct {
	file        *os.File
	length      int64
	startOffset int64
	endOffset   int64
}

type TorrentStorage struct {
	files []physicalFile
}

func NewTorrentStorage(tf *torrent.TorrentFile, outputPath string) (*TorrentStorage, error) {
	var offset int64
	var physFiles []physicalFile

	files := tf.Files
	if len(files) == 0 {
		files = []torrent.FileEntry{
			{Length: tf.Length, Path: []string{tf.Name}},
		}
	}

	for _, fe := range files {
		var fullPath string
		if tf.IsMultiFile {
			fullPath = filepath.Join(outputPath, filepath.Join(fe.Path...))
		} else {
			fullPath = outputPath
		}

		// Ensure parent directory exists
		parentDir := filepath.Dir(fullPath)
		if parentDir != "." && parentDir != "" {
			if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
				// Close any already opened files on error
				for _, pf := range physFiles {
					_ = pf.file.Close()
				}
				return nil, fmt.Errorf("create parent directory %s: %w", parentDir, err)
			}
		}

		// Open/create file
		file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			for _, pf := range physFiles {
				_ = pf.file.Close()
			}
			return nil, fmt.Errorf("open physical file %s: %w", fullPath, err)
		}

		// Truncate to size
		if err := file.Truncate(fe.Length); err != nil {
			_ = file.Close()
			for _, pf := range physFiles {
				_ = pf.file.Close()
			}
			return nil, fmt.Errorf("truncate physical file %s to %d: %w", fullPath, fe.Length, err)
		}

		physFiles = append(physFiles, physicalFile{
			file:        file,
			length:      fe.Length,
			startOffset: offset,
			endOffset:   offset + fe.Length,
		})
		offset += fe.Length
	}

	return &TorrentStorage{files: physFiles}, nil
}

func (ts *TorrentStorage) ReadAt(p []byte, off int64) (n int, err error) {
	toRead := len(p)
	if toRead == 0 {
		return 0, nil
	}

	var totalRead int
	for _, pf := range ts.files {
		if off >= pf.endOffset || off+int64(toRead) <= pf.startOffset {
			continue
		}

		// Overlap exists
		overlapStart := maxVal(off, pf.startOffset)
		overlapEnd := minVal(off+int64(toRead), pf.endOffset)
		overlapLen := overlapEnd - overlapStart

		physicalOffset := overlapStart - pf.startOffset
		bufferOffset := overlapStart - off

		rn, rerr := pf.file.ReadAt(p[bufferOffset:bufferOffset+overlapLen], physicalOffset)
		totalRead += rn
		if rerr != nil && rerr != io.EOF {
			return totalRead, rerr
		}
	}

	if totalRead < toRead {
		return totalRead, io.ErrUnexpectedEOF
	}
	return totalRead, nil
}

func (ts *TorrentStorage) WriteAt(p []byte, off int64) (n int, err error) {
	toWrite := len(p)
	if toWrite == 0 {
		return 0, nil
	}

	var totalWritten int
	for _, pf := range ts.files {
		if off >= pf.endOffset || off+int64(toWrite) <= pf.startOffset {
			continue
		}

		// Overlap exists
		overlapStart := maxVal(off, pf.startOffset)
		overlapEnd := minVal(off+int64(toWrite), pf.endOffset)
		overlapLen := overlapEnd - overlapStart

		physicalOffset := overlapStart - pf.startOffset
		bufferOffset := overlapStart - off

		wn, werr := pf.file.WriteAt(p[bufferOffset:bufferOffset+overlapLen], physicalOffset)
		totalWritten += wn
		if werr != nil {
			return totalWritten, werr
		}
	}

	return totalWritten, nil
}

func (ts *TorrentStorage) Close() error {
	var firstErr error
	for _, pf := range ts.files {
		if err := pf.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func maxVal(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minVal(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
