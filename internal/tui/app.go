package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/agent-jit/agentjit/internal/trace"
)

// AnnotatedPath is a HotPath enriched with display metadata.
type AnnotatedPath struct {
	Path       trace.HotPath
	Pattern    *trace.Pattern // parameterized version (nil if not yet parameterized)
	Labels     []string       // human-readable step labels
	Confidence float64
	Savings    int // estimated tokens saved per invocation
}

// view is the current TUI view mode.
type view int

const (
	viewList view = iota
	viewDetail
)

// Model is the bubbletea model for the aj trace TUI.
type Model struct {
	paths    []AnnotatedPath
	graph    *trace.TraceGraph
	cursor   int
	offset   int // scroll offset for list view
	view     view
	width    int
	height   int
	quitting bool
}

// NewModel creates a new TUI model.
func NewModel(paths []AnnotatedPath, graph *trace.TraceGraph) Model {
	return Model{
		paths: paths,
		graph: graph,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.offset = clampOffset(m.cursor, m.offset, m.visibleLines())

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.view == viewList && m.cursor > 0 {
				m.cursor--
				m.offset = clampOffset(m.cursor, m.offset, m.visibleLines())
			}

		case "down", "j":
			if m.view == viewList && m.cursor < len(m.paths)-1 {
				m.cursor++
				m.offset = clampOffset(m.cursor, m.offset, m.visibleLines())
			}

		case "enter":
			if m.view == viewList && len(m.paths) > 0 {
				m.view = viewDetail
			}

		case "esc":
			if m.view == viewDetail {
				m.view = viewList
			}
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	switch m.view {
	case viewDetail:
		return m.viewDetail()
	default:
		return m.viewList()
	}
}

// visibleLines returns how many list items fit in the terminal.
// Accounts for title (2 lines), help footer (2 lines).
func (m Model) visibleLines() int {
	if m.height <= 0 {
		return len(m.paths) // no size info yet, show all
	}
	avail := m.height - 4 // title + blank + blank + help
	if avail < 1 {
		avail = 1
	}
	return avail
}

// clampOffset adjusts the scroll offset so the cursor stays visible.
func clampOffset(cursor, offset, vis int) int {
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+vis {
		offset = cursor - vis + 1
	}
	return offset
}

func (m Model) viewList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("aj trace \u2014 Hot Paths"))
	b.WriteString("\n\n")

	if len(m.paths) == 0 {
		b.WriteString(dimStyle.Render("No hot paths detected. Need more session data."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("[q] quit"))
		return b.String()
	}

	vis := m.visibleLines()
	end := m.offset + vis
	if end > len(m.paths) {
		end = len(m.paths)
	}

	if m.offset > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  \u2191 %d more", m.offset)))
		b.WriteString("\n")
	}

	for i := m.offset; i < end; i++ {
		ap := m.paths[i]
		cursor := "  "
		style := dimStyle
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		label := strings.Join(ap.Labels, " \u2192 ")
		freq := frequencyStyle.Render(fmt.Sprintf("(%dx)", ap.Path.Frequency))

		b.WriteString(style.Render(cursor + label))
		b.WriteString(" ")
		b.WriteString(freq)
		b.WriteString("\n")
	}

	if end < len(m.paths) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  \u2193 %d more", len(m.paths)-end)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("[enter] expand  [q] quit"))

	return b.String()
}

func (m Model) viewDetail() string {
	if m.cursor >= len(m.paths) {
		return "No path selected"
	}

	ap := m.paths[m.cursor]
	var b strings.Builder

	b.WriteString(titleStyle.Render("Path Detail"))
	b.WriteString("\n\n")

	b.WriteString(headerStyle.Render("Steps:"))
	b.WriteString("\n")
	for i, label := range ap.Labels {
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, label))
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Frequency:    %s\n", frequencyStyle.Render(fmt.Sprintf("%d sessions", ap.Path.Frequency))))

	confStyle := confidenceHighStyle
	if ap.Confidence < 0.6 {
		confStyle = confidenceLowStyle
	}
	b.WriteString(fmt.Sprintf("Confidence:   %s\n", confStyle.Render(fmt.Sprintf("%.0f%%", ap.Confidence*100))))
	b.WriteString(fmt.Sprintf("Est. savings: %d tokens/invocation\n", ap.Savings))

	if ap.Pattern != nil {
		params := trace.CollectUniqueParams(ap.Pattern.Steps)
		if len(params) > 0 {
			b.WriteString("\n")
			b.WriteString(headerStyle.Render("Parameters:"))
			b.WriteString("\n")
			for _, p := range params {
				b.WriteString(fmt.Sprintf("  $%s \u2014 e.g. %s\n", p.Name, strings.Join(p.Values, ", ")))
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("[esc] back  [q] quit"))

	return b.String()
}

// Run starts the TUI program.
func Run(paths []AnnotatedPath, graph *trace.TraceGraph) error {
	p := tea.NewProgram(NewModel(paths, graph), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
