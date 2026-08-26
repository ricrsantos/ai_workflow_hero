package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const (
	harnessStallWarnMsg   = "Harness appears stalled (no activity). Cancel the run or use /harness-reset if needed."
	harnessFailedWarnHint = "Cancel the run or use /harness-reset if needed."
	emptyResponseWarning  = "WARNING: Harness returned an empty response. The agent produced no text or tool output."
	healthProbeTimeout    = 5 * time.Second
)

type harnessHealthProbeMsg struct{}

type harnessHealthResultMsg struct {
	health harness.HarnessHealth
	status harness.HealthStatus
	err    error
}

func harnessHealthProbeCmd() tea.Cmd {
	return tea.Tick(harness.HealthProbeInterval, func(time.Time) tea.Msg {
		return harnessHealthProbeMsg{}
	})
}

func (m model) harnessHealthCheckCmd() tea.Cmd {
	harnessID := m.conversationHarnessTool()
	sessionID := m.harnessSessionID
	stall := harness.StallTimeoutForHarness(harnessID)
	watchdog := m.harnessWatchdog
	adapter := m.harnessAdapter()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), healthProbeTimeout)
		defer cancel()
		probe := harness.HarnessHealth{
			ProcessAlive: true,
			ServerAlive:  true,
			SessionAlive: true,
		}
		var err error
		if adapter != nil {
			if hc, ok := adapter.(harness.HealthChecker); ok {
				probe, err = hc.CheckHealth(ctx, sessionID)
			}
		}
		status := watchdog.Evaluate(time.Now(), probe, stall)
		if err != nil && status == harness.HealthHealthy {
			status = harness.HealthDegraded
		}
		return harnessHealthResultMsg{health: probe, status: status, err: err}
	}
}

func (m model) resetHarnessWatchdog(executePrompt string) model {
	m.harnessWatchdog.Reset(time.Now())
	m.harnessHealthStatus = harness.HealthHealthy
	m.harnessHealthInFlight = false
	m.lastExecutePrompt = executePrompt
	return m
}

func (m model) clearHarnessHealthWarnings() model {
	m.harnessHealthStatus = harness.HealthHealthy
	return m
}

func (m model) handleHarnessHealthProbe() (model, tea.Cmd) {
	if !m.streaming {
		m.harnessHealthInFlight = false
		return m, nil
	}
	// Stream delivering substantive activity: healthy — skip HTTP probe this interval.
	if m.harnessWatchdog.HasRecentActivity(time.Now(), harness.HealthProbeInterval) {
		m = m.clearHarnessHealthWarnings()
		return m, harnessHealthProbeCmd()
	}
	if m.harnessHealthInFlight {
		return m, harnessHealthProbeCmd()
	}
	m.harnessHealthInFlight = true
	return m, tea.Batch(m.harnessHealthCheckCmd(), harnessHealthProbeCmd())
}

func (m model) handleHarnessHealthResult(msg harnessHealthResultMsg) (model, tea.Cmd) {
	m.harnessHealthInFlight = false
	if !m.streaming {
		return m, nil
	}
	prev := m.harnessHealthStatus
	m.harnessHealthStatus = msg.status

	switch msg.status {
	case harness.HealthFailed:
		if prev != harness.HealthFailed {
			warn := "Harness process is not running."
			if d := strings.TrimSpace(msg.health.Details); d != "" {
				warn = d
			}
			warn = warn + " " + harnessFailedWarnHint
			m.insertBeforeAgent(convMessage{role: convRoleWarning, content: "WARNING: " + warn})
			m = m.setStatusWarning("execute", warn)
		}
		if m.streaming {
			return m, m.cancelStreamCmd()
		}
		return m, nil

	case harness.HealthDegraded:
		if prev != harness.HealthDegraded {
			warn := "Harness health degraded."
			if d := strings.TrimSpace(msg.health.Details); d != "" {
				warn = d
			}
			if msg.err != nil {
				warn = msg.err.Error()
			}
			m.insertBeforeAgent(convMessage{role: convRoleWarning, content: "WARNING: " + warn})
			m = m.setStatusWarning("execute", warn)
		}
		return m, nil

	case harness.HealthSuspected:
		if prev != harness.HealthSuspected {
			m.insertBeforeAgent(convMessage{role: convRoleWarning, content: "WARNING: " + harnessStallWarnMsg})
			m = m.setStatusWarning("execute", harnessStallWarnMsg)
		}
		return m, nil

	default:
		return m, nil
	}
}

func agentResponseEmpty(m model, msg executeDoneMsg) bool {
	if msg.err != nil || m.streamInterrupted {
		return false
	}
	if msg.result != nil && strings.TrimSpace(msg.result.Output) != "" {
		return false
	}
	if m.agentMsgIndex >= 0 && m.agentMsgIndex < len(m.transcript) {
		if strings.TrimSpace(m.transcript[m.agentMsgIndex].content) != "" {
			return false
		}
	}
	for _, row := range m.latestAgentTurn() {
		switch row.role {
		case convRoleAgent, convRoleThinking, convRoleTool:
			if strings.TrimSpace(row.content) != "" {
				return false
			}
		}
	}
	return true
}

func (m model) warnEmptyAgentResponse() model {
	m.insertBeforeAgent(convMessage{role: convRoleWarning, content: emptyResponseWarning})
	return m.setStatusWarning("execute", "Harness returned an empty response")
}

func formatHarnessHealthDetails(h harness.HarnessHealth) string {
	return fmt.Sprintf("process=%v server=%v session=%v %s", h.ProcessAlive, h.ServerAlive, h.SessionAlive, h.Details)
}
