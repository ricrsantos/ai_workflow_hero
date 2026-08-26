package tui

import "github.com/charmbracelet/lipgloss"

// Bonito-inspired dark palette (blue/violet accents). Hex tokens degrade to
// ANSI approximations on terminals without truecolor.
var (
	colorBgBase     = lipgloss.Color("#0B0E1A")
	colorBgSurface  = lipgloss.Color("#131726")
	colorBgSurface2 = lipgloss.Color("#1B2036")
	colorBorder     = lipgloss.Color("#2A3050")
	colorTextPri    = lipgloss.Color("#E8EAF6")
	colorTextDim    = lipgloss.Color("#8A90B0")
	colorAccentUser = lipgloss.Color("#7C6CFF")
	colorAccentAI   = lipgloss.Color("#4CC2FF")
	colorAccentFast = lipgloss.Color("#E3B341")
	colorOK         = lipgloss.Color("#56D364")
	colorError      = lipgloss.Color("#F85149")

	// Aliases kept for call sites that still use the older names.
	colorGreen  = colorOK
	colorYellow = colorAccentFast
	colorRed    = colorError
	colorBlue   = colorAccentAI
	colorUser   = colorAccentUser
	colorMuted  = colorTextDim
	chatBg      = colorBgSurface
)

// Nav sidebar layout (ported from the Bonito reference TUI).
const (
	navSidebarWidth     = 24
	navSidebarMinWidth  = 80 // show sidebar when terminal width >= this
	navSidebarMinMain   = 40 // need at least this many cols for the main pane
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorTextPri)
	headerStyle   = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(colorTextPri)
	footerStyle   = lipgloss.NewStyle().Foreground(colorTextDim)
	successStyle  = lipgloss.NewStyle().Foreground(colorOK)
	// Config form field label/value separation. Labels reuse the blue accent bar
	// from the Chat composer, while inactive controls remain intentionally muted.
	configLabelStyle         = lipgloss.NewStyle().Foreground(colorAccentAI)
	configDisabledLabelStyle = lipgloss.NewStyle().Foreground(colorTextDim)
	configSelectedLabelStyle = lipgloss.NewStyle().
					Foreground(colorAccentAI).
					Background(colorBgSurface2).
					Bold(true)
	configDisabledSelectedLabelStyle = lipgloss.NewStyle().
						Foreground(colorTextDim).
						Background(colorBgSurface2).
						Bold(true)
	configValueStyle         = lipgloss.NewStyle().Foreground(colorTextPri)
	configDisabledValueStyle = lipgloss.NewStyle().Foreground(colorTextDim)
	configEditingValueStyle  = lipgloss.NewStyle().Foreground(colorTextPri).Background(colorBgSurface2)
	configEditingCaretStyle  = lipgloss.NewStyle().Foreground(colorBgSurface2).Background(colorAccentAI).Bold(true)
	warnStyle     = lipgloss.NewStyle().Foreground(colorAccentFast)
	errorStyle    = lipgloss.NewStyle().Foreground(colorError)
	infoStyle     = lipgloss.NewStyle().Foreground(colorAccentAI)
	thinkingStyle = lipgloss.NewStyle().Foreground(colorTextDim).Italic(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccentUser).Background(colorBgSurface2)
	mutedStyle    = lipgloss.NewStyle().Foreground(colorTextDim)
	// Filled caret when chat input has focus (light on surface — distinct from accent bar).
	caretFilledStyle = lipgloss.NewStyle().
				Foreground(colorBgSurface).
				Background(colorTextPri).
				Bold(true)
	// Outline caret when chat input lost focus.
	caretHollowStyle = lipgloss.NewStyle().
				Foreground(colorTextPri).
				Background(colorBgSurface).
				Bold(true)
	promptStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccentAI)
	// OpenCode-style chat panes: solid fill comes from the box Background + Width.
	// In-box text must NOT set Background (nested bg causes black gutter artifacts).
	chatBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Background(colorBgSurface)
	chatInText  = lipgloss.NewStyle().Foreground(colorTextPri)
	chatInMuted = lipgloss.NewStyle().Foreground(colorTextDim)
	chatInModel = lipgloss.NewStyle().Foreground(colorTextPri)
	chatInBuild = lipgloss.NewStyle().Bold(true).Foreground(colorAccentAI)
	chatInUser  = lipgloss.NewStyle().Bold(true).Foreground(colorAccentUser)
	chatInPlan  = lipgloss.NewStyle().Bold(true).Foreground(colorAccentFast)
	chatInAgent = lipgloss.NewStyle().Bold(true).Foreground(colorOK)
	chatInThink = lipgloss.NewStyle().Foreground(colorTextDim).Italic(true)
	chatInOK    = lipgloss.NewStyle().Foreground(colorOK)
	chatInWarn  = lipgloss.NewStyle().Foreground(colorAccentFast)
	chatInErr   = lipgloss.NewStyle().Foreground(colorError)
	// Solid 1-cell accent bar (composer / bordered panes).
	chatAccentBuild    = lipgloss.NewStyle().Background(colorAccentAI).Foreground(colorAccentAI)
	chatAccentPlan     = lipgloss.NewStyle().Background(colorAccentFast).Foreground(colorAccentFast)
	chatAccentUser     = lipgloss.NewStyle().Background(colorAccentUser).Foreground(colorAccentUser)
	chatAccentResponse = lipgloss.NewStyle().Background(colorOK).Foreground(colorOK)
	// Thin vertical accent bar for the linear transcript (Bonito-style │).
	chatBarUser  = lipgloss.NewStyle().Foreground(colorAccentUser)
	chatBarAgent = lipgloss.NewStyle().Foreground(colorOK)
	chatBarMuted = lipgloss.NewStyle().Foreground(colorTextDim)
	chatBarWarn  = lipgloss.NewStyle().Foreground(colorAccentFast)
	chatBarErr   = lipgloss.NewStyle().Foreground(colorError)
	// Output panel body: soft readable text (not raw default black/white).
	outputBodyStyle  = lipgloss.NewStyle().Foreground(colorTextPri)
	outputCycleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccentAI)
	outputTotalStyle = lipgloss.NewStyle().Foreground(colorOK)
	outputPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Background(colorBgSurface).
				Padding(0, 1)

	// Nav sidebar styles (SESSIONS-like frame).
	navSidebarTitleStyle  = lipgloss.NewStyle().Foreground(colorTextPri).Bold(true)
	navSidebarSepStyle    = lipgloss.NewStyle().Foreground(colorBorder)
	navSidebarItemStyle   = lipgloss.NewStyle().Foreground(colorTextPri)
	navSidebarActiveStyle = lipgloss.NewStyle().Foreground(colorAccentUser).Bold(true)
	navSidebarFooterStyle = lipgloss.NewStyle().Foreground(colorTextDim)
	navSidebarBoxStyle    = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Background(colorBgSurface)
)

// chatBarGlyph is the per-line transcript accent (thin vertical bar).
func chatBarGlyph() string { return "│" }

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
