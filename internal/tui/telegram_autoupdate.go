package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/autoupdate"
)

const telegramAutoUpdateCommand = "/auto-update"

type telegramAutoUpdateResultMsg struct {
	result autoupdate.Result
	err    error
}

func isTelegramAutoUpdateCommand(text string) bool {
	return strings.EqualFold(strings.TrimSpace(text), telegramAutoUpdateCommand)
}

func (m model) handleTelegramAutoUpdate() (model, tea.Cmd) {
	if m.autoUpdateBusy {
		return m, m.telegramOutboundCmd("Auto-update is already running.")
	}
	if m.svc == nil || strings.TrimSpace(m.svc.ProjectDir) == "" {
		return m, m.telegramOutboundCmd("Auto-update is unavailable: Hero source directory is not set.")
	}
	if m.streaming || m.heroStartPreparing || m.heroStartBootstrapping {
		return m, m.telegramOutboundCmd("Stop the active Hero execution with /interrupt before requesting an auto-update.")
	}

	projectDir := m.svc.ProjectDir
	m.autoUpdateBusy = true
	start := m.telegramOutboundCmd("Auto-update started: committing source changes and arming the systemd updater.")
	work := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		result, err := autoupdate.Request(ctx, autoupdate.ConfigForProject(projectDir))
		return telegramAutoUpdateResultMsg{result: result, err: err}
	}
	return m, combineTimerCmds(start, work)
}

func (m model) handleTelegramAutoUpdateResult(msg telegramAutoUpdateResultMsg) (model, tea.Cmd) {
	m.autoUpdateBusy = false
	if msg.err != nil {
		if errors.Is(msg.err, autoupdate.ErrNoChanges) {
			return m, m.telegramOutboundCmd("Auto-update skipped: there are no changes to commit.")
		}
		return m, m.telegramOutboundCmd(fmt.Sprintf("Auto-update failed: %v", msg.err))
	}
	return m, m.telegramOutboundCmd(fmt.Sprintf(
		"Auto-update queued at commit %s. The systemd timer will build and install it within 5 minutes.",
		msg.result.Commit,
	))
}
