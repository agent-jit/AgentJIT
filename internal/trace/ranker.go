package trace

import "sort"

// RankedHotPath is a HotPath with computed compilation value metrics.
type RankedHotPath struct {
	HotPath
	EstimatedTokenCost int // len(NodeIDs) * 200
	CompilationValue   int // Frequency * EstimatedTokenCost
}

// RankHotPaths computes compilation value for each hot path and returns them
// sorted by CompilationValue descending. The caller applies a top-N cut.
func RankHotPaths(paths []HotPath) []RankedHotPath {
	ranked := make([]RankedHotPath, len(paths))
	for i, hp := range paths {
		cost := len(hp.NodeIDs) * 200
		ranked[i] = RankedHotPath{
			HotPath:            hp,
			EstimatedTokenCost: cost,
			CompilationValue:   hp.Frequency * cost,
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].CompilationValue > ranked[j].CompilationValue
	})

	return ranked
}
