package tui

import (
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const (
	schedulerCTARetryStart  = "retry_start"
	schedulerCTAContinue    = "continue"
	schedulerCTAApprove     = "approve"
	schedulerCTAJudgeBack   = "judge_back"
	schedulerCTAStartFailed = "start_failed"
	schedulerCTAIdle        = "idle"
)

// ensureStageProgress is the TUI scheduler's "never idle" gate. When no
// Execute is in flight it either launches the named stage agent(s) or posts a
// deterministic Hero CTA. It must not wait for the orchestrator LLM to STOP
// or to remember the next slash command.
func (m model) ensureStageProgress() (model, tea.Cmd) {
	if m.svc == nil || m.freeChatMode || !m.orchestrationLive {
		return m, nil
	}
	if m.heroStartBootstrapping || m.heroStartPreparing {
		return m, nil
	}
	if len(m.executes) > 0 || m.stageHandoffLive {
		return m, nil
	}
	if m.stageProgressHoldUntilStart {
		st, err := m.svc.ActiveStage()
		if err != nil {
			return m, nil
		}
		return m.emitSchedulerCTA(schedulerCTARetryStart, st, "cancelled")
	}

	st, err := m.svc.ActiveStage()
	if err != nil {
		slog.Debug("tui scheduler has no actionable stage", "error", err)
		return m, nil
	}

	switch st.Status {
	case store.StageEscalated:
		return m.emitSchedulerCTA(schedulerCTAContinue, st, "")
	case store.StagePendingApproval:
		return m.emitSchedulerCTA(schedulerCTAApprove, st, "")
	case store.StageWaiting:
		return m.dispatchWaitingStage(st)
	case store.StageRunning:
		return m.dispatchRunningStage(st)
	default:
		return m, nil
	}
}

func (m model) dispatchWaitingStage(st store.Stage) (model, tea.Cmd) {
	if strings.TrimSpace(st.Name) == stageResearch {
		if err := m.startWaitingActiveStage(); err != nil {
			return m.handleStartStageFailure(st, err)
		}
		if m.researchLive {
			return m, nil
		}
		return m.startDiscoverResearchSession()
	}
	agents := m.namedStageAgents(st)
	if len(agents) == 0 {
		return m.emitSchedulerCTA(schedulerCTAIdle, st, "no named stage agent is configured")
	}
	if err := m.startWaitingActiveStage(); err != nil {
		return m.handleStartStageFailure(st, err)
	}
	return m.startStageAgentSessions(agents)
}

func (m model) dispatchRunningStage(st store.Stage) (model, tea.Cmd) {
	if strings.TrimSpace(st.Name) == stageResearch {
		if m.researchLive {
			return m, nil
		}
		return m.startDiscoverResearchSession()
	}
	agents := m.namedStageAgents(st)
	if len(agents) == 0 {
		return m.emitSchedulerCTA(schedulerCTAIdle, st, "no named stage agent is configured")
	}
	key := m.runningStageHandoffKey()
	if key != "" && key == m.stageHandoffDoneKey {
		if m.stageHandoffInterventionRequired {
			if strings.TrimSpace(st.Name) == stageJudge {
				return m.emitSchedulerCTA(schedulerCTAJudgeBack, st, "")
			}
			return m.emitSchedulerCTA(schedulerCTARetryStart, st, "")
		}
		// This iteration already ran a TUI wave. The orchestrator owns close
		// or advance; do not redispatch or the same Running stage loops.
		return m, nil
	}
	return m.startStageAgentSessions(agents)
}

func (m model) handleStartStageFailure(st store.Stage, err error) (model, tea.Cmd) {
	if err != nil {
		slog.Error("tui start waiting stage failed", "stage", st.Name, "error", err)
		m.convError = err.Error()
	}
	if m.svc != nil {
		if cur, aerr := m.svc.ActiveStage(); aerr == nil && cur.Status == store.StageEscalated {
			extra := ""
			if err != nil {
				extra = err.Error()
			}
			return m.emitSchedulerCTA(schedulerCTAContinue, cur, extra)
		}
	}
	extra := "could not start stage"
	if err != nil {
		extra = err.Error()
	}
	return m.emitSchedulerCTA(schedulerCTAStartFailed, st, extra)
}

func (m model) progressCTAForStage(st store.Stage) (model, tea.Cmd) {
	switch st.Status {
	case store.StageEscalated:
		return m.emitSchedulerCTA(schedulerCTAContinue, st, "")
	case store.StagePendingApproval:
		return m.emitSchedulerCTA(schedulerCTAApprove, st, "")
	case store.StageWaiting:
		return m.dispatchWaitingStage(st)
	default:
		return m.emitSchedulerCTA(schedulerCTAIdle, st, "")
	}
}

func (m model) emitSchedulerCTA(kind string, st store.Stage, extra string) (model, tea.Cmd) {
	key := fmt.Sprintf("%s:%s:%d:%s", kind, strings.TrimSpace(st.Name), st.Iteration, st.Status)
	if extra != "" {
		key += ":" + extra
	}
	if m.stageProgressCTAKey == key {
		return m, nil
	}
	m.stageProgressCTAKey = key
	body := formatSchedulerProgressCTA(kind, st, extra)
	m.transcript = append(m.transcript, convMessage{role: convRoleSystem, content: body})
	m.transcriptFollowBottom = true
	m = m.maybeFollowTranscriptBottom()
	slog.Info("tui scheduler idle CTA", "kind", kind, "stage", st.Name, "status", st.Status)
	return m, nil
}

func formatSchedulerProgressCTA(kind string, st store.Stage, extra string) string {
	title := validationStageTitle(strings.TrimSpace(st.Name))
	if title == strings.TrimSpace(st.Name) && title != "" {
		title = strings.TrimSpace(st.Name)
	}
	if title == "" {
		title = "stage"
	}
	extra = strings.TrimSpace(extra)
	switch kind {
	case schedulerCTAContinue:
		body := "⚠ " + title + " is Escalated.\n→ Run /hero-continue to grant more iterations, or /hero-add-todo / /hero-cancel / /hero-finish."
		if extra != "" {
			body += "\n  " + extra
		}
		return body
	case schedulerCTAApprove:
		return "⚠ " + title + " is waiting for approval.\n→ Run /hero-approve, /hero-reject, /hero-cancel, or /hero-finish."
	case schedulerCTAJudgeBack:
		return "⚠ Judge reported SDD ambiguity.\n→ Run /hero-back to reopen Planning, or /hero-approve to accept the SDD."
	case schedulerCTARetryStart:
		body := "⚠ " + title + " is Running and waiting for an explicit retry.\n→ Run /hero-start to launch the next wave."
		if extra == "cancelled" {
			body = "⚠ " + title + " is still active after cancel.\n→ Run /hero-start to retry, or /hero-cancel / /hero-finish."
		}
		return body
	case schedulerCTAStartFailed:
		body := "⚠ Could not start " + title + "."
		if extra != "" {
			body += "\n  " + extra
		}
		body += "\n→ Run /hero-start or /hero-continue in the Hero TUI."
		return body
	default:
		body := "⚠ " + title + " is " + string(st.Status) + " with no agent in flight."
		if extra != "" {
			body += "\n  " + extra
		}
		body += "\n→ Run /hero-start, /hero-approve, or /hero-continue."
		return body
	}
}

func (m model) shouldWatchStageProgress() bool {
	if m.testMode || m.freeChatMode || !m.orchestrationLive || m.svc == nil {
		return false
	}
	if len(m.executes) > 0 || m.streaming || m.stageHandoffLive {
		return false
	}
	if m.heroStartBootstrapping || m.heroStartPreparing {
		return false
	}
	if m.confirmPending || m.harnessPermissionPending || m.todoControlActive() || m.cycleWelcomeDialog {
		return false
	}
	return true
}
