package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/agent-jit/agentjit/internal/trace"
)

const panStep = 4

// GraphModel is the bubbletea model for the 2D graph view.
type GraphModel struct {
	graph      *trace.TraceGraph
	layout     *LayoutResult
	canvas     *Canvas
	vpX, vpY   int
	width      int
	height     int
	selected   int // index into nodeOrder (-1 = none)
	nodeOrder  []uint64
	showDetail bool
	quitting   bool
}

// NewGraphModel creates a new graph model.
func NewGraphModel(graph *trace.TraceGraph, layout *LayoutResult, canvas *Canvas) GraphModel {
	return GraphModel{
		graph:     graph,
		layout:    layout,
		canvas:    canvas,
		selected:  -1,
		nodeOrder: SortedNodeIDs(layout),
	}
}

func (m GraphModel) Init() tea.Cmd {
	return nil
}

func (m GraphModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "h", "left":
			m.vpX -= panStep
			m.clampViewport()
		case "l", "right":
			m.vpX += panStep
			m.clampViewport()
		case "k", "up":
			m.vpY -= panStep
			m.clampViewport()
		case "j", "down":
			m.vpY += panStep
			m.clampViewport()

		case "0", "home":
			m.vpX = 0
			m.vpY = 0

		case "tab":
			m.cycleNode(1)
		case "shift+tab":
			m.cycleNode(-1)

		case "enter":
			if m.selected >= 0 {
				m.showDetail = !m.showDetail
			}
		case "esc":
			if m.showDetail {
				m.showDetail = false
			} else if m.selected >= 0 {
				m.unhighlight()
				m.selected = -1
			}
		}
	}
	return m, nil
}

func (m *GraphModel) cycleNode(dir int) {
	if len(m.nodeOrder) == 0 {
		return
	}
	m.unhighlight()
	m.selected += dir
	if m.selected >= len(m.nodeOrder) {
		m.selected = 0
	} else if m.selected < 0 {
		m.selected = len(m.nodeOrder) - 1
	}
	m.highlight()
	// Pan to show selected node.
	ln := m.layout.Nodes[m.nodeOrder[m.selected]]
	m.panTo(ln)
}

func (m *GraphModel) highlight() {
	if m.selected >= 0 && m.selected < len(m.nodeOrder) {
		ln := m.layout.Nodes[m.nodeOrder[m.selected]]
		if ln != nil {
			HighlightNode(m.canvas, ln)
		}
	}
}

func (m *GraphModel) unhighlight() {
	if m.selected >= 0 && m.selected < len(m.nodeOrder) {
		ln := m.layout.Nodes[m.nodeOrder[m.selected]]
		if ln != nil {
			UnhighlightNode(m.canvas, ln)
		}
	}
}

func (m *GraphModel) panTo(ln *LayoutNode) {
	viewH := m.graphViewHeight()
	// Center the node in the viewport.
	targetX := ln.X - m.width/2 + ln.Width/2
	targetY := ln.Y - viewH/2 + ln.Height/2
	m.vpX = targetX
	m.vpY = targetY
	m.clampViewport()
}

func (m *GraphModel) clampViewport() {
	viewH := m.graphViewHeight()
	maxX := m.canvas.Width - m.width
	maxY := m.canvas.Height - viewH
	if maxX < 0 {
		maxX = 0
	}
	if maxY < 0 {
		maxY = 0
	}
	if m.vpX < 0 {
		m.vpX = 0
	}
	if m.vpX > maxX {
		m.vpX = maxX
	}
	if m.vpY < 0 {
		m.vpY = 0
	}
	if m.vpY > maxY {
		m.vpY = maxY
	}
}

func (m GraphModel) graphViewHeight() int {
	h := m.height - 2 // reserve for status bar + help
	if h < 1 {
		h = 1
	}
	return h
}

func (m GraphModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	viewH := m.graphViewHeight()
	graphView := RenderViewport(m.canvas, m.vpX, m.vpY, m.width, viewH)

	if m.showDetail && m.selected >= 0 {
		// Overlay detail panel on the right side.
		detail := m.renderDetail()
		b.WriteString(overlayRight(graphView, detail, m.width, viewH))
	} else {
		b.WriteString(graphView)
	}
	b.WriteRune('\n')

	// Status bar.
	b.WriteString(m.renderStatusBar())

	return b.String()
}

func (m GraphModel) renderStatusBar() string {
	edgeCount := 0
	for _, adj := range m.graph.Edges {
		edgeCount += len(adj)
	}

	left := fmt.Sprintf(" %d nodes, %d edges", len(m.graph.Nodes), edgeCount)

	center := ""
	if m.selected >= 0 && m.selected < len(m.nodeOrder) {
		id := m.nodeOrder[m.selected]
		ln := m.layout.Nodes[id]
		if ln != nil {
			center = fmt.Sprintf("  [%s]", ln.Label)
		}
	}

	right := " [hjkl] pan  [Tab] select  [Enter] detail  [q] quit "

	// Pad to fill width.
	used := len(left) + len(center) + len(right)
	padding := m.width - used
	if padding < 0 {
		padding = 0
	}

	bar := left + center + strings.Repeat(" ", padding) + right
	if len(bar) > m.width && m.width > 0 {
		bar = bar[:m.width]
	}

	return statusBarStyle.Render(bar)
}

func (m GraphModel) renderDetail() string {
	if m.selected < 0 || m.selected >= len(m.nodeOrder) {
		return ""
	}

	id := m.nodeOrder[m.selected]
	node := m.graph.Nodes[id]
	ln := m.layout.Nodes[id]
	if node == nil || ln == nil {
		return ""
	}

	panelWidth := 32
	var b strings.Builder

	b.WriteString(headerStyle.Render(ln.Label))
	b.WriteRune('\n')
	b.WriteString(dimStyle.Render(node.ToolName))
	b.WriteRune('\n')
	b.WriteRune('\n')

	// Outgoing edges.
	b.WriteString(headerStyle.Render("Outgoing:"))
	b.WriteRune('\n')
	outEdges := sortedEdges(m.graph.Edges[id])
	if len(outEdges) == 0 {
		b.WriteString(dimStyle.Render("  (none)"))
		b.WriteRune('\n')
	}
	for _, e := range outEdges {
		toNode := m.layout.Nodes[e.To]
		label := "?"
		if toNode != nil {
			label = toNode.Label
		}
		if len(label) > panelWidth-10 {
			label = label[:panelWidth-13] + "..."
		}
		bar := heatBar(e.Weight, maxEdgeWeight(m.graph))
		b.WriteString(fmt.Sprintf("  \u2192 %-*s %s\n", panelWidth-10, label, frequencyStyle.Render(fmt.Sprintf("%dx", e.Weight))))
		_ = bar
	}

	b.WriteRune('\n')

	// Incoming edges.
	b.WriteString(headerStyle.Render("Incoming:"))
	b.WriteRune('\n')
	inEdges := incomingEdges(m.graph, id)
	if len(inEdges) == 0 {
		b.WriteString(dimStyle.Render("  (none)"))
		b.WriteRune('\n')
	}
	for _, e := range inEdges {
		fromNode := m.layout.Nodes[e.From]
		label := "?"
		if fromNode != nil {
			label = fromNode.Label
		}
		if len(label) > panelWidth-10 {
			label = label[:panelWidth-13] + "..."
		}
		b.WriteString(fmt.Sprintf("  \u2190 %-*s %s\n", panelWidth-10, label, frequencyStyle.Render(fmt.Sprintf("%dx", e.Weight))))
	}

	return b.String()
}

func maxEdgeWeight(g *trace.TraceGraph) int {
	max := 1
	for _, adj := range g.Edges {
		for _, e := range adj {
			if e.Weight > max {
				max = e.Weight
			}
		}
	}
	return max
}

func heatBar(weight, maxWeight int) string {
	maxBars := 8
	n := (weight * maxBars) / maxWeight
	if n < 1 {
		n = 1
	}
	return strings.Repeat("\u2588", n)
}

func sortedEdges(adj map[uint64]*trace.Edge) []*trace.Edge {
	edges := make([]*trace.Edge, 0, len(adj))
	for _, e := range adj {
		edges = append(edges, e)
	}
	// Sort by weight descending.
	for i := 0; i < len(edges); i++ {
		for j := i + 1; j < len(edges); j++ {
			if edges[j].Weight > edges[i].Weight {
				edges[i], edges[j] = edges[j], edges[i]
			}
		}
	}
	return edges
}

func incomingEdges(g *trace.TraceGraph, id uint64) []*trace.Edge {
	var edges []*trace.Edge
	for _, adj := range g.Edges {
		if e, ok := adj[id]; ok {
			edges = append(edges, e)
		}
	}
	// Sort by weight descending.
	for i := 0; i < len(edges); i++ {
		for j := i + 1; j < len(edges); j++ {
			if edges[j].Weight > edges[i].Weight {
				edges[i], edges[j] = edges[j], edges[i]
			}
		}
	}
	return edges
}

// overlayRight overlays a detail panel on the right side of the graph view.
func overlayRight(graphView, detail string, width, height int) string {
	graphLines := strings.Split(graphView, "\n")
	detailLines := strings.Split(detail, "\n")

	// Ensure we have enough graph lines.
	for len(graphLines) < height {
		graphLines = append(graphLines, "")
	}

	panelWidth := 34
	panelStart := width - panelWidth
	if panelStart < 0 {
		panelStart = 0
	}

	var b strings.Builder
	for i := 0; i < height; i++ {
		line := graphLines[i]
		// Pad line to full width.
		for len(line) < width {
			line += " "
		}

		if i < len(detailLines) && panelStart > 0 {
			// Replace right portion with detail content.
			detailLine := " \u2502 " + detailLines[i]
			if len(detailLine) > panelWidth {
				detailLine = detailLine[:panelWidth]
			}
			// Pad detail line.
			for len(detailLine) < panelWidth {
				detailLine += " "
			}

			runes := []rune(line)
			detailRunes := []rune(detailLine)
			if panelStart < len(runes) {
				result := make([]rune, panelStart)
				copy(result, runes[:panelStart])
				result = append(result, detailRunes...)
				line = string(result)
			}
		}

		b.WriteString(line)
		if i < height-1 {
			b.WriteRune('\n')
		}
	}
	return b.String()
}

// RunGraph starts the 2D graph TUI.
func RunGraph(graph *trace.TraceGraph) error {
	layout := ComputeLayout(graph)
	canvas := RenderCanvas(layout, graph)
	model := NewGraphModel(graph, layout, canvas)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
