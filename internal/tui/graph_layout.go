package tui

import (
	"math"
	"sort"

	"github.com/agent-jit/agentjit/internal/trace"
)

const (
	nodeWidth  = 24
	nodeHeight = 3
	hGap       = 6
	vGap       = 4
)

// LayoutNode holds a positioned node for rendering.
type LayoutNode struct {
	ID     uint64
	Label  string
	Layer  int
	Order  int
	X, Y   int
	Width  int
	Height int
}

// LayoutResult holds the complete graph layout.
type LayoutResult struct {
	Nodes       map[uint64]*LayoutNode
	TotalWidth  int
	TotalHeight int
}

// ComputeLayout runs a simplified Sugiyama-style layered layout on the trace graph.
func ComputeLayout(g *trace.TraceGraph) *LayoutResult {
	if len(g.Nodes) == 0 {
		return &LayoutResult{Nodes: make(map[uint64]*LayoutNode)}
	}

	labels := buildLabels(g)
	layers := assignLayers(g)
	orderInLayer := orderNodes(g, layers)
	return positionNodes(g, labels, layers, orderInLayer)
}

// buildLabels creates a display label for each node.
func buildLabels(g *trace.TraceGraph) map[uint64]string {
	labels := make(map[uint64]string, len(g.Nodes))

	// Count how many nodes share each ToolName to decide if we need disambiguation.
	toolCounts := make(map[string]int)
	for _, n := range g.Nodes {
		toolCounts[n.ToolName]++
	}

	for id, n := range g.Nodes {
		if n.ToolName == "Bash" {
			if cmd, ok := n.InputShape["command"]; ok {
				maxLen := nodeWidth - 4 // border + padding
				if len(cmd) > maxLen {
					cmd = cmd[:maxLen-3] + "..."
				}
				labels[id] = cmd
			} else {
				labels[id] = "Bash"
			}
		} else if toolCounts[n.ToolName] > 1 {
			// Disambiguate non-Bash nodes that share a ToolName.
			suffix := ""
			for k, v := range n.InputShape {
				suffix = k + ":" + v
				break
			}
			maxLen := nodeWidth - 4
			label := n.ToolName
			if suffix != "" {
				label = n.ToolName + " " + suffix
			}
			if len(label) > maxLen {
				label = label[:maxLen-3] + "..."
			}
			labels[id] = label
		} else {
			labels[id] = n.ToolName
		}
	}
	return labels
}

// assignLayers assigns each node to a layer using longest-path BFS from root nodes.
func assignLayers(g *trace.TraceGraph) map[uint64]int {
	inDegree := make(map[uint64]int)
	for id := range g.Nodes {
		inDegree[id] = 0
	}
	for _, adj := range g.Edges {
		for toID := range adj {
			inDegree[toID]++
		}
	}

	// Find roots: nodes with in-degree 0.
	var roots []uint64
	for id, deg := range inDegree {
		if deg == 0 {
			roots = append(roots, id)
		}
	}

	// If no roots (pure cycle), pick the node with highest total outgoing weight.
	if len(roots) == 0 {
		var bestID uint64
		bestWeight := -1
		for id := range g.Nodes {
			w := 0
			for _, e := range g.Edges[id] {
				w += e.Weight
			}
			if w > bestWeight {
				bestWeight = w
				bestID = id
			}
		}
		roots = []uint64{bestID}
	}

	layers := make(map[uint64]int)
	for id := range g.Nodes {
		layers[id] = 0
	}

	// BFS longest-path. We iterate until stable since cycles may require multiple passes.
	changed := true
	for iter := 0; changed && iter < len(g.Nodes)+1; iter++ {
		changed = false
		for fromID, adj := range g.Edges {
			for toID := range adj {
				if fromID == toID {
					continue // skip self-loops
				}
				newLayer := layers[fromID] + 1
				if newLayer > layers[toID] {
					layers[toID] = newLayer
					changed = true
				}
			}
		}
	}

	// Ensure roots are at layer 0 and shift everything so min layer is 0.
	for _, r := range roots {
		layers[r] = 0
	}

	minLayer := math.MaxInt
	for _, l := range layers {
		if l < minLayer {
			minLayer = l
		}
	}
	if minLayer != 0 {
		for id := range layers {
			layers[id] -= minLayer
		}
	}

	return layers
}

// orderNodes determines the left-to-right order of nodes within each layer
// using a barycenter heuristic.
func orderNodes(g *trace.TraceGraph, layers map[uint64]int) map[uint64]int {
	// Group nodes by layer.
	layerNodes := make(map[int][]uint64)
	for id, l := range layers {
		layerNodes[l] = append(layerNodes[l], id)
	}

	maxLayer := 0
	for l := range layerNodes {
		if l > maxLayer {
			maxLayer = l
		}
	}

	// Initial order: sort by total edge weight descending (hottest first within layer).
	for l := range layerNodes {
		nodes := layerNodes[l]
		sort.Slice(nodes, func(i, j int) bool {
			wi := totalWeight(g, nodes[i])
			wj := totalWeight(g, nodes[j])
			if wi != wj {
				return wi > wj
			}
			return nodes[i] < nodes[j]
		})
		layerNodes[l] = nodes
	}

	// Build position lookup.
	pos := make(map[uint64]int)
	for _, nodes := range layerNodes {
		for i, id := range nodes {
			pos[id] = i
		}
	}

	// Barycenter sweep: top-down then bottom-up (2 passes).
	for pass := 0; pass < 2; pass++ {
		startLayer, endLayer, step := 1, maxLayer+1, 1
		if pass == 1 {
			startLayer, endLayer, step = maxLayer-1, -1, -1
		}

		for l := startLayer; l != endLayer; l += step {
			nodes := layerNodes[l]
			bary := make(map[uint64]float64)
			for _, id := range nodes {
				bary[id] = barycenter(g, id, layers, pos, l-step)
			}
			sort.SliceStable(nodes, func(i, j int) bool {
				return bary[nodes[i]] < bary[nodes[j]]
			})
			for i, id := range nodes {
				pos[id] = i
			}
			layerNodes[l] = nodes
		}
	}

	return pos
}

// barycenter computes the average position of a node's neighbors in the target layer.
func barycenter(g *trace.TraceGraph, id uint64, layers, pos map[uint64]int, targetLayer int) float64 {
	sum := 0.0
	count := 0

	// Check outgoing edges.
	for toID := range g.Edges[id] {
		if layers[toID] == targetLayer {
			sum += float64(pos[toID])
			count++
		}
	}

	// Check incoming edges.
	for fromID, adj := range g.Edges {
		if _, ok := adj[id]; ok && layers[fromID] == targetLayer {
			sum += float64(pos[fromID])
			count++
		}
	}

	if count == 0 {
		return float64(pos[id])
	}
	return sum / float64(count)
}

func totalWeight(g *trace.TraceGraph, id uint64) int {
	w := 0
	for _, e := range g.Edges[id] {
		w += e.Weight
	}
	for _, adj := range g.Edges {
		if e, ok := adj[id]; ok {
			w += e.Weight
		}
	}
	return w
}

// positionNodes converts layer/order assignments into pixel coordinates.
func positionNodes(g *trace.TraceGraph, labels map[uint64]string, layers, order map[uint64]int) *LayoutResult {
	result := &LayoutResult{
		Nodes: make(map[uint64]*LayoutNode, len(g.Nodes)),
	}

	maxLayer := 0
	maxOrder := make(map[int]int) // max order per layer
	for id := range g.Nodes {
		l := layers[id]
		o := order[id]
		if l > maxLayer {
			maxLayer = l
		}
		if o > maxOrder[l] {
			maxOrder[l] = o
		}
	}

	for id, n := range g.Nodes {
		_ = n
		l := layers[id]
		o := order[id]
		ln := &LayoutNode{
			ID:     id,
			Label:  labels[id],
			Layer:  l,
			Order:  o,
			X:      o * (nodeWidth + hGap),
			Y:      l * (nodeHeight + vGap),
			Width:  nodeWidth,
			Height: nodeHeight,
		}
		result.Nodes[id] = ln
	}

	// Compute total canvas size.
	for l := 0; l <= maxLayer; l++ {
		layerWidth := (maxOrder[l] + 1) * (nodeWidth + hGap)
		if layerWidth > result.TotalWidth {
			result.TotalWidth = layerWidth
		}
	}
	result.TotalHeight = (maxLayer + 1) * (nodeHeight + vGap)

	// Ensure minimum size.
	if result.TotalWidth < nodeWidth {
		result.TotalWidth = nodeWidth
	}
	if result.TotalHeight < nodeHeight {
		result.TotalHeight = nodeHeight
	}

	return result
}

// NodeLabel returns the display label for a node (exported for use in rendering).
func NodeLabel(n *trace.Node) string {
	if n.ToolName == "Bash" {
		if cmd, ok := n.InputShape["command"]; ok {
			maxLen := nodeWidth - 4
			if len(cmd) > maxLen {
				cmd = cmd[:maxLen-3] + "..."
			}
			return cmd
		}
	}
	return n.ToolName
}

// SortedNodeIDs returns node IDs sorted by layer then order for consistent Tab cycling.
func SortedNodeIDs(layout *LayoutResult) []uint64 {
	ids := make([]uint64, 0, len(layout.Nodes))
	for id := range layout.Nodes {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		ni := layout.Nodes[ids[i]]
		nj := layout.Nodes[ids[j]]
		if ni.Layer != nj.Layer {
			return ni.Layer < nj.Layer
		}
		return ni.Order < nj.Order
	})
	return ids
}
