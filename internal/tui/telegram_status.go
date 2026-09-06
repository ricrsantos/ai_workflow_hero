package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

const telegramStatusCommand = "/status"

// telegramAutoReportEnabled reports whether this project has periodic Telegram
// status reports configured. The interval itself is persisted in hero.json.
func (m model) telegramAutoReportEnabled() bool {
	return m.telegram != nil && m.telegram.autoReportMinutes > 0
}

// maybeTelegramAutoReport sends one report per configured interval. It runs
// from the existing Bubble Tea timer tick, so it never blocks Update.
func (m model) maybeTelegramAutoReport(at time.Time) tea.Cmd {
	if !m.telegramAutoReportEnabled() || m.telegram == nil || !m.telegram.connected || !m.telegram.paired {
		return nil
	}
	if at.Before(m.telegram.nextAutoReportAt) {
		return nil
	}
	interval := time.Duration(m.telegram.autoReportMinutes) * time.Minute
	m.telegram.nextAutoReportAt = at.Add(interval)
	return m.telegramOutboundCmd(m.telegramStatusText(at))
}

// telegramStatusText returns the compact remote status representation shared
// by /status, automatic reports, and the initial response to a remote turn.
func (m model) telegramStatusText(at time.Time) string {
	if at.IsZero() {
		at = time.Now()
	}
	if m.hasActiveCycle() {
		return telegramCycleStatusText(m.status, m.telegramTimerAndContextText(at))
	}
	if m.streaming {
		return "Waiting for harness\n" + m.telegramTimerAndContextText(at)
	}
	return "idle"
}

func telegramCycleStatusText(status cycle.StatusView, timing string) string {
	var b strings.Builder
	title := strings.TrimSpace(status.Title)
	if title == "" {
		title = fmt.Sprintf("C%d", status.CycleNumber)
	}
	fmt.Fprintf(&b, "Cycle C%d: %s\n", status.CycleNumber, title)
	if objective := strings.TrimSpace(status.Objective); objective != "" {
		b.WriteString("Objective: ")
		b.WriteString(objective)
		b.WriteByte('\n')
	}
	if state := strings.TrimSpace(status.Status); state != "" {
		b.WriteString("Status: ")
		b.WriteString(state)
		b.WriteByte('\n')
	}
	if stage, ok := telegramCurrentStage(status.Stages); ok {
		fmt.Fprintf(&b, "Current stage: %s (%s, iteration %s)\n", stage.Name, stage.Status, stage.Iteration)
	}
	b.WriteString(timing)
	return strings.TrimSpace(b.String())
}

func telegramCurrentStage(stages []cycle.StatusStage) (cycle.StatusStage, bool) {
	for _, stage := range stages {
		state := strings.ToLower(strings.TrimSpace(stage.Status))
		if state == "running" || state == "active" || state == "in_progress" || state == "in progress" {
			return stage, true
		}
	}
	return cycle.StatusStage{}, false
}

func (m model) telegramTimerAndContextText(at time.Time) string {
	session := m.sessionTimer.elapsedAt(at)
	aiWork := m.aiTimer.elapsedAt(at)
	aiResponse := m.aiResponseTimer.elapsedAt(at)
	return fmt.Sprintf(
		"Session: %s\nAI wk: %s\nAI rp: %s\nContext: %s",
		formatElapsed(session),
		formatElapsed(aiWork),
		formatElapsed(aiResponse),
		telegramContextWindowText(m.contextUsedTokens, m.contextWindowMax()),
	)
}

func telegramContextWindowText(used, max int64) string {
	if max <= 0 {
		return "n/a"
	}
	return telegramTokenCount(used) + "/" + telegramTokenCount(max)
}

func telegramTokenCount(tokens int64) string {
	if tokens >= 1000 {
		return fmt.Sprintf("%dk", tokens/1000)
	}
	return fmt.Sprintf("%d", tokens)
}
