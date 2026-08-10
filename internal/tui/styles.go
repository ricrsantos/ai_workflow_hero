package tui

import "github.com/charmbracelet/lipgloss"

// Hero semantic colors (UI.md §2.1).
var (
	colorGreen  = lipgloss.Color("2")
	colorYellow = lipgloss.Color("3")
	colorRed    = lipgloss.Color("1")
	colorBlue   = lipgloss.Color("4")
	colorMuted  = lipgloss.Color("8")
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBlue)
	headerStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	footerStyle = lipgloss.NewStyle().Foreground(colorMuted)
	successStyle = lipgloss.NewStyle().Foreground(colorGreen)
	warnStyle = lipgloss.NewStyle().Foreground(colorYellow)
	errorStyle = lipgloss.NewStyle().Foreground(colorRed)
	infoStyle = lipgloss.NewStyle().Foreground(colorBlue)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBlue).Background(lipgloss.Color("236"))
	mutedStyle = lipgloss.NewStyle().Foreground(colorMuted)
	// Reverse block caret for the conversation input (UI-C03-001 §3).
	caretStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(colorBlue).
			Bold(true)
	promptStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBlue)
	// Output panel body: soft readable text (not raw default black/white).
	outputBodyStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "236", Dark: "252"})
	outputCycleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBlue)
	outputTotalStyle = lipgloss.NewStyle().Foreground(colorGreen)
	outputPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBlue).
			Padding(0, 1)
)

func stageStatusStyle(status string) lipgloss.Style {
	switch status {
	case "Completed":
		return successStyle
	case "Running", "PendingApproval":
		return infoStyle
	case "Escalated", "Failed":
		return errorStyle
	case "Waiting":
		return mutedStyle
	default:
		return lipgloss.NewStyle()
	}
}

func stageStatusIcon(status string) string {
	switch status {
	case "Completed":
		return "✓"
	case "Running":
		return "→"
	case "PendingApproval":
		return "⚠"
	case "Escalated", "Failed":
		return "✗"
	case "Waiting":
		return "·"
	default:
		return "·"
	}
}
