package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var cycleWelcomeKeys = struct {
	Next     key.Binding
	Previous key.Binding
	Confirm  key.Binding
	Close    key.Binding
}{
	Next:     key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab/→", "next")),
	Previous: key.NewBinding(key.WithKeys("shift+tab", "left"), key.WithHelp("shift+tab/←", "previous")),
	Confirm:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Close:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
}

var (
	// Inner text styles share the dialog surface background. Nested
	// foreground-only styles emit a reset that punches default/black holes
	// through the box fill, which is what produced the post-/hero-new gaps.
	cycleWelcomeBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorAccentAI).
				Background(colorBgSurface).
				Padding(1, 2)
	cycleWelcomeTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorTextPri).
				Background(colorBgSurface)
	cycleWelcomeLeadStyle = lipgloss.NewStyle().
				Foreground(colorAccentAI).
				Background(colorBgSurface)
	cycleWelcomeBodyStyle = lipgloss.NewStyle().
				Foreground(colorTextPri).
				Background(colorBgSurface)
	cycleWelcomeDetailStyle = lipgloss.NewStyle().
				Foreground(colorTextDim).
				Background(colorBgSurface)
	cycleWelcomeTipStyle = lipgloss.NewStyle().
				Foreground(colorAccentFast).
				Bold(true).
				Background(colorBgSurface)
	cycleWelcomeFillStyle   = lipgloss.NewStyle().Background(colorBgSurface)
	cycleWelcomeButtonStyle = lipgloss.NewStyle().
				Foreground(colorTextDim).
				Background(colorBgSurface).
				Padding(0, 1)
	cycleWelcomeSelectedStyle = lipgloss.NewStyle().
					Foreground(colorBgBase).
					Background(colorAccentAI).
					Bold(true).
					Padding(0, 1)
)

// handleCycleWelcomeKey owns input while the post-/hero-new guidance dialog is
// visible. It deliberately has no persistence: the prompt is for the newly
// created cycle in this TUI session only.
func (m model) handleCycleWelcomeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, cycleWelcomeKeys.Next), key.Matches(msg, cycleWelcomeKeys.Previous):
		m.cycleWelcomeFocus = 1 - m.cycleWelcomeFocus
		return m, nil
	case key.Matches(msg, cycleWelcomeKeys.Close):
		m.cycleWelcomeDialog = false
		m.cycleWelcomeFocus = 0
		return m, nil
	case key.Matches(msg, cycleWelcomeKeys.Confirm):
		if m.cycleWelcomeFocus == 1 {
			m.cycleWelcomeDialog = false
			m.cycleWelcomeFocus = 0
			return m, nil
		}
		m.cycleWelcomeDialog = false
		m.cycleWelcomeFocus = 0
		return m.openConfig()
	}
	return m, nil
}

// renderCycleWelcomeDialog renders a centered, modal-style panel. Terminal
// cells cannot be translucent, so the full-screen surface becomes the backdrop
// and the bordered panel is placed above it.
func (m model) renderCycleWelcomeDialog() string {
	if m.width < 62 || m.height < 26 {
		return m.renderCompactCycleWelcomeDialog()
	}

	innerWidth := cycleWelcomeInnerWidth(minInt(94, m.width-8))
	rows := []string{
		cycleWelcomeLine(cycleWelcomeTitleStyle, "New development cycle created", innerWidth),
		cycleWelcomeBlank(innerWidth),
		cycleWelcomeLine(cycleWelcomeLeadStyle, "Thank you for starting a new development cycle with Hero!", innerWidth),
		cycleWelcomeBlank(innerWidth),
		cycleWelcomeWrapped(cycleWelcomeBodyStyle, "Before you begin, review these recommendations:", innerWidth),
		cycleWelcomeBlank(innerWidth),
		cycleWelcomeBullet("Make sure you are authenticated with every Harness you intend to use.", []string{
			"See each Harness documentation for instructions on signing in or verifying authentication.",
		}, innerWidth),
		cycleWelcomeBlank(innerWidth),
		cycleWelcomeBullet("Check that your favorite Skills are available in every Harness you intend to use.", []string{
			"You can ask Hero's AI directly in Free Chat. For example:",
			"\"Equalize my Skills across the configured Harnesses and install any that are missing.\"",
		}, innerWidth),
		cycleWelcomeBlank(innerWidth),
		cycleWelcomeTip("You can save time by creating a file that describes your idea in:", "docs/idea/", innerWidth),
		cycleWelcomeIndented("You can also develop and refine that idea beforehand with an agent in Hero Free Chat.", innerWidth),
		cycleWelcomeBlank(innerWidth),
		cycleWelcomeTip("You can configure the cycle you just created by editing:", ".workflow-hero/cycles/current/workflow-config.yml", innerWidth),
		cycleWelcomeIndented("Or use Hero's TUI Config section.", innerWidth),
		cycleWelcomeBlank(innerWidth),
		m.renderCycleWelcomeButtons(innerWidth),
	}
	return m.placeCycleWelcomeBox(strings.Join(rows, "\n"), innerWidth)
}

func (m model) renderCompactCycleWelcomeDialog() string {
	innerWidth := cycleWelcomeInnerWidth(max(1, m.width-4))
	rows := []string{
		cycleWelcomeLine(cycleWelcomeTitleStyle, "New cycle created", innerWidth),
		cycleWelcomeBlank(innerWidth),
		cycleWelcomeWrapped(cycleWelcomeBodyStyle, "Resize the terminal to review the setup guide.", innerWidth),
		cycleWelcomeLine(cycleWelcomeDetailStyle, "You can also open Config now.", innerWidth),
		cycleWelcomeBlank(innerWidth),
		m.renderCycleWelcomeButtons(innerWidth),
	}
	return m.placeCycleWelcomeBox(strings.Join(rows, "\n"), innerWidth)
}

func (m model) placeCycleWelcomeBox(content string, innerWidth int) string {
	// Width is the content box including padding, not the border. Matching
	// that here keeps lipgloss from synthesizing a second, unstyled pad.
	box := cycleWelcomeBoxStyle.Width(innerWidth + cycleWelcomeBoxStyle.GetHorizontalPadding()).Render(content)
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		box,
		lipgloss.WithWhitespaceBackground(colorBgSurface),
	)
}

func (m model) renderCycleWelcomeButtons(width int) string {
	goConfig := "[ Go to Config ]"
	close := "[ Close ]"
	if m.cycleWelcomeFocus == 0 {
		goConfig = cycleWelcomeSelectedStyle.Render(goConfig)
		close = cycleWelcomeButtonStyle.Render(close)
	} else {
		goConfig = cycleWelcomeButtonStyle.Render(goConfig)
		close = cycleWelcomeSelectedStyle.Render(close)
	}
	row := lipgloss.PlaceHorizontal(
		width,
		lipgloss.Center,
		goConfig+"   "+close,
		lipgloss.WithWhitespaceBackground(colorBgSurface),
	)
	return cycleWelcomeFillStyle.Width(width).Render(row)
}

func cycleWelcomeBullet(head string, details []string, width int) string {
	var b strings.Builder
	b.WriteString(cycleWelcomeWrapped(cycleWelcomeBodyStyle, "• "+head, width))
	for _, detail := range details {
		b.WriteByte('\n')
		b.WriteString(cycleWelcomeIndented(detail, width))
	}
	return b.String()
}

func cycleWelcomeTip(head, path string, width int) string {
	return cycleWelcomeWrapped(cycleWelcomeTipStyle, "• Tip: "+head, width) + "\n" + cycleWelcomeIndented(path, width)
}

func cycleWelcomeIndented(text string, width int) string {
	contentWidth := max(12, width-4)
	lines := wrapOutputLine(text, contentWidth)
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = cycleWelcomeLine(cycleWelcomeDetailStyle, "    "+line, width)
	}
	return strings.Join(out, "\n")
}

func cycleWelcomeWrapped(style lipgloss.Style, text string, width int) string {
	lines := wrapOutputLine(text, width)
	if len(lines) == 0 {
		return cycleWelcomeLine(style, "", width)
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = cycleWelcomeLine(style, line, width)
	}
	return strings.Join(out, "\n")
}

func cycleWelcomeLine(style lipgloss.Style, text string, width int) string {
	if width < 1 {
		width = 1
	}
	return style.Width(width).MaxWidth(width).Render(text)
}

func cycleWelcomeBlank(width int) string {
	return cycleWelcomeLine(cycleWelcomeFillStyle, "", width)
}

func cycleWelcomeInnerWidth(boxWidth int) int {
	inner := boxWidth - cycleWelcomeBoxStyle.GetHorizontalFrameSize()
	if inner < 12 {
		return 12
	}
	return inner
}
