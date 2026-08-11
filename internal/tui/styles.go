package tui

import "github.com/charmbracelet/lipgloss"

// Hero semantic colors (UI.md §2.1).
var (
	colorGreen  = lipgloss.Color("2")
	colorYellow = lipgloss.Color("3")
	colorRed    = lipgloss.Color("1")
	colorBlue   = lipgloss.Color("4")
	colorMuted  = lipgloss.Color("8")
	chatBg      = lipgloss.Color("236")
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorBlue)
	headerStyle  = lipgloss.NewStyle().Bold(true).Underline(true)
	footerStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	successStyle = lipgloss.NewStyle().Foreground(colorGreen)
	warnStyle    = lipgloss.NewStyle().Foreground(colorYellow)
	errorStyle   = lipgloss.NewStyle().Foreground(colorRed)
	infoStyle    = lipgloss.NewStyle().Foreground(colorBlue)
	thinkingStyle = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBlue).Background(lipgloss.Color("236"))
	mutedStyle   = lipgloss.NewStyle().Foreground(colorMuted)
	// Filled caret when chat input has focus (white — distinct from accent bar).
	caretFilledStyle = lipgloss.NewStyle().
				Foreground(chatBg).
				Background(lipgloss.Color("15")).
				Bold(true)
	// Outline caret when chat input lost focus.
	caretHollowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(chatBg).
				Bold(true)
	promptStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBlue)
	// OpenCode-style chat panes: solid fill comes from the box Background + Width.
	// In-box text must NOT set Background (nested bg causes black gutter artifacts).
	chatBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Background(chatBg)
	chatInText  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	chatInMuted = lipgloss.NewStyle().Foreground(colorMuted)
	chatInModel = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	chatInBuild = lipgloss.NewStyle().Bold(true).Foreground(colorBlue)
	chatInPlan  = lipgloss.NewStyle().Bold(true).Foreground(colorYellow)
	chatInAgent = lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
	chatInThink = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	chatInOK    = lipgloss.NewStyle().Foreground(colorGreen)
	chatInWarn  = lipgloss.NewStyle().Foreground(colorYellow)
	// Solid 1-cell accent bar.
	chatAccentBuild    = lipgloss.NewStyle().Background(colorBlue).Foreground(colorBlue)
	chatAccentPlan     = lipgloss.NewStyle().Background(colorYellow).Foreground(colorYellow)
	chatAccentResponse = lipgloss.NewStyle().Background(colorGreen).Foreground(colorGreen)
	// Output panel body: soft readable text (not raw default black/white).
	outputBodyStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "236", Dark: "252"})
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
