package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

// reproGateResultMsg carries the outcome of the asynchronous repro gate. The
// gate re-runs the locked repro of every finding an Implementation report
// claims as done. It must never run inside Update: a single finding can hold a
// test command for minutes, which would freeze the whole TUI.
type reproGateResultMsg struct {
	stage   string
	wave    int
	results map[string]*cycle.ReproGateError
	err     error
}

// claimedFindingIDs collects the find-* IDs the captured reports claim as
// completed. It is deliberately a soft parse: the authoritative assignment
// union is validated later, and verifying a superset costs only time.
func claimedFindingIDs(chunks []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, chunk := range chunks {
		obj, ok := extractStageReportObject(chunk)
		if !ok {
			continue
		}
		for _, id := range softReportTaskIDs(obj, "tasks_completed") {
			id = strings.TrimSpace(id)
			if !strings.HasPrefix(id, "find-") {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// startReproGate launches the gate off the Update loop and reports progress in
// the transcript so a long verification never looks like a hang.
func (m model) startReproGate(stage string, ids []string) (model, tea.Cmd) {
	svc := m.svc
	if svc == nil || svc.Store == nil {
		m.stageHandoffReproChecked = true
		return m.resumeOrchestratorAfterStageHandoff()
	}
	cycleRow, err := svc.SessionCycle()
	if err != nil || cycleRow == nil {
		m.stageHandoffReproChecked = true
		m.stageHandoffReproError = "active cycle is unavailable"
		return m.resumeOrchestratorAfterStageHandoff()
	}
	cycleID := cycleRow.ID
	projectDir := svc.ProjectDir
	stageName := stage
	wave := m.stageHandoffWave
	verify := append([]string(nil), ids...)

	m.stageHandoffReproRunning = true
	m.transcript = append(m.transcript, convMessage{
		role: convRoleSystem,
		content: fmt.Sprintf("⏳ Verifying %d finding repro(s) before closing the wave: %s",
			len(verify), strings.Join(verify, ", ")),
	})
	m.transcriptFollowBottom = true
	m = m.maybeFollowTranscriptBottom()
	slog.Info("repro gate started", "stage", stageName, "wave", wave, "finding_ids", verify)

	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cycle.ReproGateBudget)
		defer cancel()
		results, err := cycle.VerifyFindingRepros(ctx, svc.Store, cycleID, projectDir, verify)
		return reproGateResultMsg{stage: stageName, wave: wave, results: results, err: err}
	}
}

// handleReproGateResult stores the gate outcome and resumes the handoff the
// gate interrupted.
func (m model) handleReproGateResult(msg reproGateResultMsg) (model, tea.Cmd) {
	m.stageHandoffReproRunning = false
	if !m.stageHandoffLive || m.stageHandoffStage != msg.stage || m.stageHandoffWave != msg.wave {
		// The wave this gate belonged to is gone (cancel, /hero-reset, restart).
		// Applying its verdict to a different wave would be wrong.
		slog.Info("repro gate result discarded as stale", "stage", msg.stage, "wave", msg.wave)
		return m, nil
	}
	m.stageHandoffReproChecked = true
	m.stageHandoffReproResults = msg.results
	m.stageHandoffReproError = ""
	if msg.err != nil {
		slog.Error("repro gate failed", "stage", msg.stage, "error", redact.Error(msg.err))
		m.stageHandoffReproError = msg.err.Error()
	}
	return m.resumeOrchestratorAfterStageHandoff()
}

// clearReproGateState resets the gate before a new wave collects reports.
func (m model) clearReproGateState() model {
	m.stageHandoffReproChecked = false
	m.stageHandoffReproRunning = false
	m.stageHandoffReproResults = nil
	m.stageHandoffReproError = ""
	return m
}

// reproGateFailure returns the reason the wave must not close, or "" when every
// claimed finding's repro passed. A finding the gate did not cover is treated
// as unverified: the scheduler never marks a find-* done on trust.
func (m model) reproGateFailure(findingIDs []string) string {
	if strings.TrimSpace(m.stageHandoffReproError) != "" {
		return "repro gate could not run: " + m.stageHandoffReproError
	}
	if !m.stageHandoffReproChecked {
		return "repro gate has not verified the completed findings yet"
	}
	for _, id := range findingIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		gate, ok := m.stageHandoffReproResults[id]
		if !ok {
			return fmt.Sprintf("repro gate did not verify %s; the report claimed it after the gate ran", id)
		}
		if gate != nil {
			return gate.Error()
		}
	}
	return ""
}
