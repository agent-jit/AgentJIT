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
)
