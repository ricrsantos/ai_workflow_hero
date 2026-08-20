package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/atotto/clipboard"
	osc52 "github.com/aymanbagabas/go-osc52/v2"
)

// copyToClipboardCmd writes text via OSC 52 (terminal) and native clipboard fallback.
func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		osc52.New(text).WriteTo(os.Stderr)
		_ = clipboard.WriteAll(text)
		return nil
	}
}
