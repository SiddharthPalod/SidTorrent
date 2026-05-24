package torrent

import "fmt"

func (t *TorrentFile) PieceCount() int {
	count := t.Length / t.PieceLength
	if t.Length%t.PieceLength != 0 {
		count++
	}
	return int(count)
}

func (t *TorrentFile) PieceLengthAt(index int) int64 {
	length, err := PieceLengthAt(
		t.Length,
		t.PieceLength,
		index,
	)
	if err != nil {
		return 0
	}
	return length
}

func PieceLengthAt(totalLength, standardPieceLength int64, pieceIndex int) (int64, error) {
	if totalLength <= 0 {
		return 0, fmt.Errorf("invalid total length: %d", totalLength)
	}
	if standardPieceLength <= 0 {
		return 0, fmt.Errorf("invalid piece length: %d", standardPieceLength)
	}
	if pieceIndex < 0 {
		return 0, fmt.Errorf("invalid piece index: %d", pieceIndex)
	}

	fullPieces := totalLength / standardPieceLength
	remainder := totalLength % standardPieceLength
	pieceCount := fullPieces
	if remainder > 0 {
		pieceCount++
	}
	if int64(pieceIndex) >= pieceCount {
		return 0, fmt.Errorf("piece index %d out of range", pieceIndex)
	}
	if remainder > 0 && int64(pieceIndex) == pieceCount-1 {
		return remainder, nil
	}
	return standardPieceLength, nil
}
