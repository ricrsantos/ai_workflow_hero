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
	if at.IsZero() {
		at = time.Now()
	}
	if at.Before(m.telegram.nextAutoReportAt) {
		return nil
	}
	interval := time.Duration(m.telegram.autoReportMinutes) * time.Minute
	m.telegram.nextAutoReportAt = at.Add(interval)
	return m.telegramOutboundCmd(m.telegramAutoReportText(at))
}

// telegramStatusText returns the compact response for the Telegram /status
// command. Unlike automatic reports, a manual status request may legitimately
// return idle.
func (m model) telegramStatusText(at time.Time) string {
	return m.telegramStatusTextAt(at, true)
}

// telegramAutoReportText returns the compact periodic status representation.
// An idle TUI has no useful automatic update, so the empty result suppresses
// the outbound message while the scheduler still advances to the next slot.
func (m model) telegramAutoReportText(at time.Time) string {
	return m.telegramStatusTextAt(at, false)
}

func (m model) telegramStatusTextAt(at time.Time, includeIdle bool) string {
	if at.IsZero() {
		at = time.Now()
	}
	agents := m.telegramStatusAgentsText()
	if m.hasActiveCycle() {
		return telegramCycleStatusText(m.status, agents, m.telegramTimerAndContextText(at))
	}
	if m.streaming {
		return telegramStatusWithAgents("Waiting for harness", agents, m.telegramTimerAndContextText(at))
	}
	if !includeIdle {
		return ""
	}
	return m.telegramIdleStatusText(at)
}

// telegramIdleStatusText keeps the manual idle response useful without
// exposing workflow-cycle details. The model is the free-chat selection, not
// the last workflow agent that happened to run.
func (m model) telegramIdleStatusText(at time.Time) string {
	modelSlug := strings.TrimSpace(m.chatModelSlug)
	if modelSlug == "" {
		modelSlug = "not set"
	}
	timing := m.telegramTimerAndContextTextForMax(at, m.contextWindows.lookup(m.chatModelSlug))
	return telegramStatusWithModel("idle", modelSlug, timing)
}

func telegramStatusWithModel(state, modelSlug, timing string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(state))
	fmt.Fprintf(&b, "\nModel: %s", strings.TrimSpace(modelSlug))
	if timing != "" {
		b.WriteByte('\n')
		b.WriteString(timing)
	}
	return strings.TrimSpace(b.String())
}

func telegramCycleStatusText(status cycle.StatusView, agents, timing string) string {
	var b strings.Builder
	title := strings.TrimSpace(status.Title)
	if title == "" {
		title = fmt.Sprintf("C%d", status.CycleNumber)
	}
	fmt.Fprintf(&b, "Cycle C%d: %s\n", status.CycleNumber, title)
	if state := strings.TrimSpace(status.Status); state != "" {
		b.WriteString("Status: ")
		b.WriteString(state)
		b.WriteByte('\n')
	}
	if stage, ok := telegramCurrentStage(status.Stages); ok {
		fmt.Fprintf(&b, "Current stage: %s (%s, iteration %s)\n", stage.Name, stage.Status, stage.Iteration)
	}
	if agents != "" {
		b.WriteString(agents)
		b.WriteByte('\n')
	}
	b.WriteString(timing)
	return strings.TrimSpace(b.String())
}

// telegramStatusWithAgents composes a non-cycle status while keeping the
// agent list between the state line and the timing counters.
func telegramStatusWithAgents(state, agents, timing string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(state))
	if agents != "" {
		b.WriteByte('\n')
		b.WriteString(agents)
	}
	if timing != "" {
		b.WriteByte('\n')
		b.WriteString(timing)
	}
	return strings.TrimSpace(b.String())
}

// telegramStatusAgentsText reports only agents currently operating in the
// TUI. The parent execute has an empty name for ordinary Free Chat, so it is
// exposed as the stable agent name "harness" for remote status consumers.
func (m model) telegramStatusAgentsText() string {
	agents := m.liveAgents
	if len(agents) == 0 && m.streaming {
		agents = []liveAgent{{
			Name:    m.runtimeAgentName,
			Model:   m.conversationModelSlug(),
			Harness: m.conversationHarnessTool(),
		}}
	}
	if len(agents) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Agents:")
	for _, agent := range agents {
		name := strings.TrimSpace(agent.Name)
		if name == "" {
			name = "harness"
		}
		model := strings.TrimSpace(agent.Model)
		if model == "" {
			model = "not set"
		}
		fmt.Fprintf(&b, "\n- %s: %s", name, model)
	}
	return b.String()
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
	return m.telegramTimerAndContextTextForMax(at, m.contextWindowMax())
}

func (m model) telegramTimerAndContextTextForMax(at time.Time, contextMax int64) string {
	session := m.sessionTimer.elapsedAt(at)
	aiWork := m.aiTimer.elapsedAt(at)
	aiResponse := m.aiResponseTimer.elapsedAt(at)
	return fmt.Sprintf(
		"Session: %s\nAI wk: %s\nAI rp: %s\nContext: %s",
		formatElapsed(session),
		formatElapsed(aiWork),
		formatElapsed(aiResponse),
		telegramContextWindowText(m.contextUsedTokens, contextMax),
	)
}

func telegramContextWindowText(used, max int64) string {
	if used < 0 {
		used = 0
	}
	if max <= 0 {
		return telegramTokenCount(used) + "/n/a"
	}
	return telegramTokenCount(used) + "/" + telegramTokenCount(max)
}

func telegramTokenCount(tokens int64) string {
	if tokens >= 1000 {
		return fmt.Sprintf("%dk", tokens/1000)
	}
	return fmt.Sprintf("%d", tokens)
}
