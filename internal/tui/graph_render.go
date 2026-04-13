package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/agent-jit/agentjit/internal/trace"
)

// Canvas is a 2D character buffer with per-cell styling.
type Canvas struct {
	Cells  [][]rune
	Styles [][]lipgloss.Style
	Width  int
	Height int
}

// NewCanvas creates an empty canvas filled with spaces.
func NewCanvas(width, height int) *Canvas {
	cells := make([][]rune, height)
	styles := make([][]lipgloss.Style, height)
	defaultStyle := lipgloss.NewStyle()
	for y := 0; y < height; y++ {
		cells[y] = make([]rune, width)
		styles[y] = make([]lipgloss.Style, width)
		for x := 0; x < width; x++ {
			cells[y][x] = ' '
			styles[y][x] = defaultStyle
		}
	}
	return &Canvas{Cells: cells, Styles: styles, Width: width, Height: height}
}

// Set writes a rune at (x,y) with a style, if within bounds.
func (c *Canvas) Set(x, y int, ch rune, style lipgloss.Style) {
	if x >= 0 && x < c.Width && y >= 0 && y < c.Height {
		c.Cells[y][x] = ch
		c.Styles[y][x] = style
	}
}

// SetString writes a string starting at (x,y).
func (c *Canvas) SetString(x, y int, s string, style lipgloss.Style) {
	for i, ch := range s {
		c.Set(x+i, y, ch, style)
	}
}

// RenderCanvas creates a fully rendered Canvas from the layout and graph.
func RenderCanvas(layout *LayoutResult, g *trace.TraceGraph) *Canvas {
	// Add padding for edge labels and arrows beyond the layout bounds.
	w := layout.TotalWidth + hGap
	h := layout.TotalHeight + vGap
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	c := NewCanvas(w, h)

	// Compute max edge weight for heat coloring.
	maxWeight := 1
	for _, adj := range g.Edges {
		for _, e := range adj {
			if e.Weight > maxWeight {
				maxWeight = e.Weight
			}
		}
	}

	// Draw edges first (behind nodes).
	for fromID, adj := range g.Edges {
		for toID, edge := range adj {
			if fromID == toID {
				continue // skip self-loops for now
			}
			fromNode := layout.Nodes[fromID]
			toNode := layout.Nodes[toID]
			if fromNode == nil || toNode == nil {
				continue
			}
			drawEdge(c, fromNode, toNode, edge, maxWeight)
		}
	}

	// Draw nodes on top.
	for _, ln := range layout.Nodes {
		drawNode(c, ln, nodeBoxStyle)
	}

	return c
}

// drawNode renders a box at the node's position.
func drawNode(c *Canvas, ln *LayoutNode, style lipgloss.Style) {
	x, y := ln.X, ln.Y
	w := ln.Width

	// Top border: ┌───┐
	c.Set(x, y, '\u250c', style)
	for i := 1; i < w-1; i++ {
		c.Set(x+i, y, '\u2500', style)
	}
	c.Set(x+w-1, y, '\u2510', style)

	// Content line: │ label │
	c.Set(x, y+1, '\u2502', style)
	label := ln.Label
	contentWidth := w - 4 // 2 border + 2 padding
	if len(label) > contentWidth {
		label = label[:contentWidth-3] + "..."
	}
	padded := fmt.Sprintf(" %-*s ", contentWidth, label)
	c.SetString(x+1, y+1, padded, style)
	c.Set(x+w-1, y+1, '\u2502', style)

	// Bottom border: └───┘
	c.Set(x, y+2, '\u2514', style)
	for i := 1; i < w-1; i++ {
		c.Set(x+i, y+2, '\u2500', style)
	}
	c.Set(x+w-1, y+2, '\u2518', style)
}

// drawEdge draws an orthogonal edge between two nodes with heat coloring.
func drawEdge(c *Canvas, from, to *LayoutNode, edge *trace.Edge, maxWeight int) {
	style := edgeHeatStyle(edge.Weight, maxWeight)

	// Exit point: bottom-center of from node.
	exitX := from.X + from.Width/2
	exitY := from.Y + from.Height // row just below from node

	// Entry point: top-center of to node.
	entryX := to.X + to.Width/2
	entryY := to.Y - 1 // row just above to node

	if entryY < exitY {
		// Back-edge (to is above from). Draw a simple dashed route to the right.
		drawBackEdge(c, from, to, edge, style)
		return
	}

	// Weight label.
	weightLabel := fmt.Sprintf("%dx", edge.Weight)

	if exitX == entryX {
		// Straight vertical edge.
		for y := exitY; y <= entryY; y++ {
			c.Set(exitX, y, '\u2502', style)
		}
		// Arrow at entry.
		c.Set(entryX, entryY, '\u25bc', style)
		// Weight label to the right of midpoint.
		midY := (exitY + entryY) / 2
		c.SetString(exitX+2, midY, weightLabel, style)
	} else {
		// Orthogonal: down, across, down.
		midY := (exitY + entryY) / 2

		// Vertical segment from exit down to midY.
		for y := exitY; y < midY; y++ {
			c.Set(exitX, y, '\u2502', style)
		}

		// Horizontal segment at midY.
		startX, endX := exitX, entryX
		if startX > endX {
			startX, endX = endX, startX
		}

		// Corner at (exitX, midY).
		if entryX > exitX {
			c.Set(exitX, midY, '\u2514', style) // └
		} else {
			c.Set(exitX, midY, '\u2518', style) // ┘
		}

		for x := startX + 1; x < endX; x++ {
			c.Set(x, midY, '\u2500', style)
		}

		// Corner at (entryX, midY).
		if entryX > exitX {
			c.Set(entryX, midY, '\u2510', style) // ┐
		} else {
			c.Set(entryX, midY, '\u250c', style) // ┌
		}

		// Vertical segment from midY down to entry.
		for y := midY + 1; y <= entryY; y++ {
			c.Set(entryX, y, '\u2502', style)
		}

		// Arrow at entry.
		c.Set(entryX, entryY, '\u25bc', style)

		// Weight label near horizontal segment.
		labelX := (startX + endX) / 2
		if labelX+len(weightLabel) >= endX {
			labelX = startX + 1
		}
		c.SetString(labelX, midY-1, weightLabel, style)
	}
}

// drawBackEdge draws a back-edge (to a node in a higher/same layer) using a route to the right.
func drawBackEdge(c *Canvas, from, to *LayoutNode, edge *trace.Edge, style lipgloss.Style) {
	// Route: right side of from → up → left to top of to.
	rightX := from.X + from.Width + 1
	if toRight := to.X + to.Width + 1; toRight > rightX {
		rightX = toRight
	}
	// Keep within canvas bounds.
	if rightX >= c.Width-2 {
		rightX = c.Width - 3
	}

	fromY := from.Y + from.Height/2
	toY := to.Y + to.Height/2

	weightLabel := fmt.Sprintf("%dx", edge.Weight)

	// Horizontal from "from" right side to rightX.
	for x := from.X + from.Width; x <= rightX; x++ {
		c.Set(x, fromY, '\u2500', style)
	}
	// Corner up.
	c.Set(rightX, fromY, '\u2510', style)

	// Vertical up.
	startY, endY := toY, fromY
	if startY > endY {
		startY, endY = endY, startY
	}
	for y := startY + 1; y < endY; y++ {
		c.Set(rightX, y, '\u2502', style)
	}
	// Corner left.
	c.Set(rightX, toY, '\u2518', style)

	// Horizontal from to right side to rightX.
	for x := to.X + to.Width; x < rightX; x++ {
		c.Set(x, toY, '\u2500', style)
	}
	// Arrow at to node.
	c.Set(to.X+to.Width, toY, '\u25c0', style)

	// Weight label.
	midY := (fromY + toY) / 2
	c.SetString(rightX+1, midY, weightLabel, style)
}

// edgeHeatStyle returns the lipgloss style for an edge based on its weight.
func edgeHeatStyle(weight, maxWeight int) lipgloss.Style {
	ratio := float64(weight) / float64(maxWeight)
	switch {
	case ratio >= 0.75:
		return edgeBurningStyle
	case ratio >= 0.50:
		return edgeHotStyle
	case ratio >= 0.25:
		return edgeWarmStyle
	default:
		return edgeColdStyle
	}
}

// RenderViewport extracts a visible rectangle from the canvas as a styled string.
func RenderViewport(c *Canvas, vpX, vpY, width, height int) string {
	var b strings.Builder
	for y := vpY; y < vpY+height && y < c.Height; y++ {
		for x := vpX; x < vpX+width && x < c.Width; x++ {
			ch := c.Cells[y][x]
			style := c.Styles[y][x]
			b.WriteString(style.Render(string(ch)))
		}
		if y < vpY+height-1 {
			b.WriteRune('\n')
		}
	}
	return b.String()
}

// HighlightNode temporarily re-renders a node with selection style on the canvas.
func HighlightNode(c *Canvas, ln *LayoutNode) {
	drawNode(c, ln, nodeSelectedStyle)
}

// UnhighlightNode restores a node to its default style on the canvas.
func UnhighlightNode(c *Canvas, ln *LayoutNode) {
	drawNode(c, ln, nodeBoxStyle)
}
