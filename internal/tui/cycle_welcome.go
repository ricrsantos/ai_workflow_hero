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
	cycleWelcomeBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorAccentAI).
				Background(colorBgSurface).
				Padding(1, 2)
	cycleWelcomeTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorTextPri)
	cycleWelcomeLeadStyle     = lipgloss.NewStyle().Foreground(colorAccentAI)
	cycleWelcomeBodyStyle     = lipgloss.NewStyle().Foreground(colorTextPri)
	cycleWelcomeDetailStyle   = lipgloss.NewStyle().Foreground(colorTextDim)
	cycleWelcomeTipStyle      = lipgloss.NewStyle().Foreground(colorAccentFast).Bold(true)
	cycleWelcomeButtonStyle   = lipgloss.NewStyle().Foreground(colorTextDim).Padding(0, 1)
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

	boxWidth := minInt(94, m.width-8)
	innerWidth := boxWidth - cycleWelcomeBoxStyle.GetHorizontalFrameSize()
	var b strings.Builder
	b.WriteString(cycleWelcomeTitleStyle.Render("New development cycle created"))
	b.WriteString("\n\n")
	b.WriteString(cycleWelcomeLeadStyle.Render("Thank you for starting a new development cycle with Hero!"))
	b.WriteString("\n\n")
	b.WriteString(cycleWelcomeBodyStyle.Render("Before you begin, review these recommendations:"))
	b.WriteString("\n\n")
	b.WriteString(cycleWelcomeBullet("Make sure you are authenticated with every Harness you intend to use.", []string{
		"See each Harness documentation for instructions on signing in or verifying authentication.",
	}, innerWidth))
	b.WriteString("\n\n")
	b.WriteString(cycleWelcomeBullet("Check that your favorite Skills are available in every Harness you intend to use.", []string{
		"You can ask Hero's AI directly in Free Chat. For example:",
		"\"Equalize my Skills across the configured Harnesses and install any that are missing.\"",
	}, innerWidth))
	b.WriteString("\n\n")
	b.WriteString(cycleWelcomeTip("You can save time by creating a file that describes your idea in:", "docs/idea/", innerWidth))
	b.WriteString("\n")
	b.WriteString(cycleWelcomeIndented("You can also develop and refine that idea beforehand with an agent in Hero Free Chat.", innerWidth))
	b.WriteString("\n\n")
	b.WriteString(cycleWelcomeTip("You can configure the cycle you just created by editing:", ".workflow-hero/cycles/current/workflow-config.yml", innerWidth))
	b.WriteString("\n")
	b.WriteString(cycleWelcomeIndented("Or use Hero's TUI Config section.", innerWidth))
	b.WriteString("\n\n")
	b.WriteString(m.renderCycleWelcomeButtons(innerWidth))

	box := cycleWelcomeBoxStyle.Width(boxWidth).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderCompactCycleWelcomeDialog() string {
	boxWidth := max(1, m.width-4)
	innerWidth := boxWidth - cycleWelcomeBoxStyle.GetHorizontalFrameSize()
	var b strings.Builder
	b.WriteString(cycleWelcomeTitleStyle.Render("New cycle created"))
	b.WriteString("\n\n")
	b.WriteString(cycleWelcomeBodyStyle.Render("Resize the terminal to review the setup guide."))
	b.WriteString("\n")
	b.WriteString(cycleWelcomeDetailStyle.Render("You can also open Config now."))
	b.WriteString("\n\n")
	b.WriteString(m.renderCycleWelcomeButtons(innerWidth))
	box := cycleWelcomeBoxStyle.Width(boxWidth).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
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
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, goConfig+"   "+close)
}

func cycleWelcomeBullet(head string, details []string, width int) string {
	var b strings.Builder
	b.WriteString(cycleWelcomeBodyStyle.Render("• " + head))
	for _, detail := range details {
		b.WriteByte('\n')
		b.WriteString(cycleWelcomeIndented(detail, width))
	}
	return b.String()
}

func cycleWelcomeTip(head, path string, width int) string {
	return cycleWelcomeTipStyle.Render("• Tip: "+head) + "\n" + cycleWelcomeIndented(path, width)
}

func cycleWelcomeIndented(text string, width int) string {
	contentWidth := max(12, width-4)
	lines := wrapOutputLine(text, contentWidth)
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return cycleWelcomeDetailStyle.Render(strings.Join(lines, "\n"))
}
