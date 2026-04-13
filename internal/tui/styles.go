package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	frequencyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Bold(true)

	confidenceHighStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82"))

	confidenceLowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Underline(true)

	// Graph visualization styles
	nodeBoxStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	nodeSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("170"))

	edgeColdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	edgeWarmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))

	edgeHotStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208"))

	edgeBurningStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("196"))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("236"))
)
