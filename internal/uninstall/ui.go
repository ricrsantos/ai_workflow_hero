package uninstall

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

const (
	uninstallConfirmTitle = "Proceed with uninstalling Hero?"
	uninstallConfirmBody  = "Files in .workflow-hero/ and other Hero-owned directories will be removed.\n" +
		"AGENTS.md, context/, docs/, and openspec/ are preserved."
)

func promptUninstallConfirm() (bool, error) {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(uninstallConfirmTitle).
				Description(uninstallConfirmBody).
				Value(&confirmed),
		),
	).WithTheme(uninstallTheme())
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

func uninstallTheme() *huh.Theme {
	t := huh.ThemeBase()
	clean := lipgloss.NewStyle()
	t.Focused.Base = clean
	t.Focused.Card = clean
	t.Blurred.Base = clean
	t.Blurred.Card = clean
	t.Focused.Title = lipgloss.NewStyle()
	t.Blurred.Title = lipgloss.NewStyle()
	return t
}
