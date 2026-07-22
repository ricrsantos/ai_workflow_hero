package install

import (
	"fmt"
	"io"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
)

// heroInstallTheme returns a minimal huh theme matching the install ceremony mock:
// no left border, plain titles, clean text input.
func heroInstallTheme() *huh.Theme {
	t := huh.ThemeBase()
	clean := lipgloss.NewStyle()
	t.Focused.Base = clean
	t.Focused.Card = clean
	t.Blurred.Base = clean
	t.Blurred.Card = clean
	t.Focused.Title = lipgloss.NewStyle()
	t.Blurred.Title = lipgloss.NewStyle()
	t.Focused.TextInput.Prompt = lipgloss.NewStyle()
	t.Blurred.TextInput.Prompt = lipgloss.NewStyle()
	return t
}

// printSetupHeader prints the install ceremony header.
// Uses rocket emoji only when stdout is a color-capable TTY.
func printSetupHeader(w io.Writer) {
	fmt.Fprintln(w)
	if output.IsTerminal(w) {
		fmt.Fprintln(w, "🚀 Hero Project Setup")
	} else {
		fmt.Fprintln(w, "Hero Project Setup")
	}
	fmt.Fprintln(w)
}
