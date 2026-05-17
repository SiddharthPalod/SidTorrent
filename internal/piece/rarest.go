package piece

import "sort"

type PieceAvailability struct {
	Index int
	Count int
}

func ComputeAvailability(
	peerBitfields [][]bool, totalPieces int) []PieceAvailability {
	counts := make([]PieceAvailability, totalPieces)

	for i := 0; i < totalPieces; i++ {
		counts[i] = PieceAvailability{Index: i, Count: 0}
	}

	for _, bf := range peerBitfields {
		for i, has := range bf {
			if has {
				counts[i].Count++
			}
		}
	}

	sort.Slice(counts, func(i, j int) bool {
		return counts[i].Count < counts[j].Count
	})
	return counts
}
