package tui

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/upgrade"
)

// telegramHeroUpgradeCommand is the Telegram-only command that upgrades the
// Hero assets of the selected project instance in place (equivalent to
// running `hero upgrade` in the project directory) and reports the result
// back to the Telegram chat. It never starts a harness turn.
const telegramHeroUpgradeCommand = "/hero-upgrade"

// upgradeRunFunc is the injectable seam for the upgrade service. Production
// calls upgrade.Run against the embedded assets; tests replace it so no
// filesystem project is required.
var upgradeRunFunc = func(opts upgrade.Options, stdout, stderr io.Writer) (upgrade.Result, error) {
	return upgrade.Run(opts, stdout, stderr)
}

// telegramHeroUpgradeResultMsg carries the finished `hero upgrade` outcome
// from the background worker back to the Bubble Tea Update loop.
type telegramHeroUpgradeResultMsg struct {
	version string
	result  upgrade.Result
	output  string
	err     error
}

// parseTelegramHeroUpgrade classifies Telegram input for /hero-upgrade.
// matched reports whether the text targets /hero-upgrade at all; valid
// reports whether it carries no extra arguments (the command takes none,
// mirroring the other exact-match Telegram commands like /auto-update).
func parseTelegramHeroUpgrade(text string) (matched, valid bool) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 || !strings.EqualFold(fields[0], telegramHeroUpgradeCommand) {
		return false, false
	}
	return true, len(fields) == 1
}

// handleTelegramHeroUpgrade starts the project upgrade in a background
// tea.Cmd so the Bubble Tea Update loop never blocks on file I/O. The
// captured CLI-style output is returned to the Telegram chat when done.
func (m model) handleTelegramHeroUpgrade() (model, tea.Cmd) {
	if m.heroUpgradeBusy {
		return m, m.telegramOutboundCmd("Hero upgrade is already running.")
	}
	if m.svc == nil || strings.TrimSpace(m.svc.ProjectDir) == "" {
		return m, m.telegramOutboundCmd("Hero upgrade is unavailable: Hero project directory is not set.")
	}
	if strings.TrimSpace(m.version) == "" {
		return m, m.telegramOutboundCmd("Hero upgrade is unavailable: Hero version is not set.")
	}
	if m.streaming || m.heroStartPreparing || m.heroStartBootstrapping {
		return m, m.telegramOutboundCmd("Stop the active Hero execution with /interrupt before requesting a hero upgrade.")
	}

	projectDir := m.svc.ProjectDir
	version := strings.TrimSpace(m.version)
	m.heroUpgradeBusy = true
	start := m.telegramOutboundCmd(fmt.Sprintf("Hero upgrade started (hero upgrade to version %s).", version))
	work := func() tea.Msg {
		var stdout, stderr bytes.Buffer
		result, err := upgradeRunFunc(upgrade.Options{
			ProjectDir: projectDir,
			Version:    version,
			AssetsFS:   assets.FS,
		}, &stdout, &stderr)
		return telegramHeroUpgradeResultMsg{
			version: version,
			result:  result,
			output:  strings.TrimSpace(stdout.String() + "\n" + stderr.String()),
			err:     err,
		}
	}
	return m, combineTimerCmds(start, work)
}

// handleTelegramHeroUpgradeResult clears the busy flag and reports the
// `hero upgrade` outcome (CLI-style output included) to the Telegram chat.
func (m model) handleTelegramHeroUpgradeResult(msg telegramHeroUpgradeResultMsg) (model, tea.Cmd) {
	m.heroUpgradeBusy = false
	if msg.err != nil {
		text := fmt.Sprintf("Hero upgrade failed: %v", msg.err)
		if msg.output != "" {
			text += "\n" + msg.output
		}
		return m, m.telegramOutboundCmd(text)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Hero upgraded to version %s.", msg.version)
	fmt.Fprintf(&b, "\nUpdated: %d file(s).", len(msg.result.Updated))
	if len(msg.result.Migrated) > 0 {
		fmt.Fprintf(&b, "\nMigrated workflow-config: %d file(s).", len(msg.result.Migrated))
	}
	if len(msg.result.Replaced) > 0 {
		fmt.Fprintf(&b, "\n%d file(s) replaced due to conflicts (backups saved with .conflict suffix).", len(msg.result.Replaced))
	}
	if msg.output != "" {
		b.WriteString("\n" + msg.output)
	}
	return m, m.telegramOutboundCmd(b.String())
}
