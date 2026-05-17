package piece

import "fmt"

func PrintProgress(pm *PieceManager) {
	pending, inProgress, completed := pm.Stats()
	downloaded, total, percent := pm.Progress()

	fmt.Printf(
		"progress %.2f%% | downloaded=%d/%d bytes | pending=%d inprogress=%d completed=%d\n",
		percent,
		downloaded,
		total,
		pending,
		inProgress,
		completed,
	)
}
