package trace

import "testing"

func TestRankHotPaths_SortsByCompilationValue(t *testing.T) {
	paths := []HotPath{
		{NodeIDs: []uint64{1, 2}, Frequency: 3},           // 2 steps * 200 * 3 = 1200
		{NodeIDs: []uint64{1, 2, 3, 4, 5}, Frequency: 10}, // 5 steps * 200 * 10 = 10000
		{NodeIDs: []uint64{1, 2, 3}, Frequency: 5},         // 3 steps * 200 * 5 = 3000
	}

	ranked := RankHotPaths(paths)
	if len(ranked) != 3 {
		t.Fatalf("got %d ranked paths, want 3", len(ranked))
	}
	if ranked[0].CompilationValue != 10000 {
		t.Errorf("ranked[0].CompilationValue = %d, want 10000", ranked[0].CompilationValue)
	}
	if ranked[1].CompilationValue != 3000 {
		t.Errorf("ranked[1].CompilationValue = %d, want 3000", ranked[1].CompilationValue)
	}
	if ranked[2].CompilationValue != 1200 {
		t.Errorf("ranked[2].CompilationValue = %d, want 1200", ranked[2].CompilationValue)
	}
}

func TestRankHotPaths_EstimatedTokenCost(t *testing.T) {
	paths := []HotPath{
		{NodeIDs: []uint64{1, 2, 3}, Frequency: 4},
	}
	ranked := RankHotPaths(paths)
	if ranked[0].EstimatedTokenCost != 600 { // 3 * 200
		t.Errorf("EstimatedTokenCost = %d, want 600", ranked[0].EstimatedTokenCost)
	}
}

func TestRankHotPaths_Empty(t *testing.T) {
	ranked := RankHotPaths(nil)
	if len(ranked) != 0 {
		t.Errorf("got %d, want 0", len(ranked))
	}
}
