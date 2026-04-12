package trace

import "sort"

// HotPath represents a frequently-traversed sequence of nodes in the trace graph.
type HotPath struct {
	NodeIDs    []uint64 // ordered sequence of node IDs
	Frequency  int      // number of sessions where this exact path appears
	SessionIDs []string // contributing sessions
}

// FindHotPaths finds frequently-traversed subgraphs using DFS with session intersection.
func FindHotPaths(g *TraceGraph, minFrequency, minLength, maxLength int) []HotPath {
	var candidates []HotPath

	for nodeID := range g.Nodes {
		// Only start DFS from nodes that have outgoing edges
		if g.Edges[nodeID] == nil {
			continue
		}

		dfs(g, []uint64{nodeID}, nil, minFrequency, minLength, maxLength, &candidates)
	}

	return pruneSubPaths(candidates)
}

func dfs(g *TraceGraph, path []uint64, sessionSet []string, minFrequency, minLength, maxLength int, results *[]HotPath) {
	tail := path[len(path)-1]
	adj := g.Edges[tail]
	if adj == nil {
		return
	}

	for nextID, edge := range adj {
		if containsNode(path, nextID) {
			continue
		}
		var commonSessions []string
		if sessionSet == nil {
			// First edge: use edge's session IDs directly
			commonSessions = edge.SessionIDs
		} else {
			commonSessions = intersect(sessionSet, edge.SessionIDs)
		}

		if len(commonSessions) < minFrequency {
			continue
		}

		newPath := make([]uint64, len(path)+1)
		copy(newPath, path)
		newPath[len(path)] = nextID

		if len(newPath) >= minLength {
			sessions := make([]string, len(commonSessions))
			copy(sessions, commonSessions)
			*results = append(*results, HotPath{
				NodeIDs:    newPath,
				Frequency:  len(commonSessions),
				SessionIDs: sessions,
			})
		}

		if len(newPath) < maxLength {
			dfs(g, newPath, commonSessions, minFrequency, minLength, maxLength, results)
		}
	}
}

// containsNode checks if a node ID already exists in the current path (cycle detection).
func containsNode(path []uint64, id uint64) bool {
	for _, n := range path {
		if n == id {
			return true
		}
	}
	return false
}

// intersect returns the intersection of two string slices.
func intersect(a, b []string) []string {
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	var result []string
	for _, s := range b {
		if set[s] {
			result = append(result, s)
		}
	}
	return result
}

// pruneSubPaths removes paths that are strict subpaths of a longer path
// with the same or higher frequency.
func pruneSubPaths(paths []HotPath) []HotPath {
	if len(paths) == 0 {
		return paths
	}

	// Sort by length descending, then frequency descending
	sort.Slice(paths, func(i, j int) bool {
		if len(paths[i].NodeIDs) != len(paths[j].NodeIDs) {
			return len(paths[i].NodeIDs) > len(paths[j].NodeIDs)
		}
		return paths[i].Frequency > paths[j].Frequency
	})

	var kept []HotPath
	for _, candidate := range paths {
		subsumed := false
		for _, longer := range kept {
			if longer.Frequency >= candidate.Frequency && isSubPath(candidate.NodeIDs, longer.NodeIDs) {
				subsumed = true
				break
			}
		}
		if !subsumed {
			kept = append(kept, candidate)
		}
	}

	return kept
}

// isSubPath checks if short is a contiguous subsequence of long.
func isSubPath(short, long []uint64) bool {
	if len(short) >= len(long) {
		return false
	}
	for i := 0; i <= len(long)-len(short); i++ {
		match := true
		for j := 0; j < len(short); j++ {
			if long[i+j] != short[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
