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
	harnessHangConfirmMsg = "Harness appears stalled. [y] Cancel  [n] Wait  [r] Restart harness"
	emptyResponseWarning  = "WARNING: Harness returned an empty response. The agent produced no text or tool output."
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	m.harnessHangPending = false
	m.harnessHangMsg = ""
	m.harnessHangDismissed = false
	m.lastExecutePrompt = executePrompt
	return m
}

func (m model) handleHarnessHealthProbe() (model, tea.Cmd) {
	if !m.streaming {
		return m, nil
	}
	return m, tea.Batch(m.harnessHealthCheckCmd(), harnessHealthProbeCmd())
}

func (m model) handleHarnessHealthResult(msg harnessHealthResultMsg) (model, tea.Cmd) {
	if !m.streaming {
		return m, nil
	}
	prev := m.harnessHealthStatus
	m.harnessHealthStatus = msg.status

	switch msg.status {
	case harness.HealthFailed:
		warn := "Harness process is not running."
		if d := strings.TrimSpace(msg.health.Details); d != "" {
			warn = d
		}
		m.insertBeforeAgent(convMessage{role: convRoleWarning, content: "WARNING: " + warn})
		m = m.setStatusWarning("execute", warn)
		m.harnessHangPending = false
		return m, m.cancelStreamCmd()

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
		return m, harnessHealthProbeCmd()

	case harness.HealthSuspected:
		if m.harnessHangDismissed {
			return m, harnessHealthProbeCmd()
		}
		if !m.harnessHangPending {
			m.harnessHangPending = true
			m.harnessHangMsg = harnessHangConfirmMsg
			m.insertBeforeAgent(convMessage{role: convRoleWarning, content: "WARNING: Harness appears stalled (no activity). " + harnessHangConfirmMsg})
		}
		return m, harnessHealthProbeCmd()

	default:
		if prev == harness.HealthSuspected || prev == harness.HealthDegraded {
			m.harnessHangPending = false
			m.harnessHangMsg = ""
		}
		return m, harnessHealthProbeCmd()
	}
}

func (m model) handleHarnessHangKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.harnessHangPending = false
		m.harnessHangMsg = ""
		return m, m.cancelStreamCmd()
	case "r", "R":
		m.harnessHangPending = false
		m.harnessHangMsg = ""
		harnessID := m.conversationHarnessTool()
		next, resetCmd := m.applyHarnessReset(harnessID)
		return next, tea.Batch(m.cancelStreamCmd(), resetCmd)
	default:
		m.harnessHangPending = false
		m.harnessHangMsg = ""
		m.harnessHangDismissed = true
		return m, harnessHealthProbeCmd()
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
