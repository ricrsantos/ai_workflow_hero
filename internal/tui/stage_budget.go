package tui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

type stageBudgetTickMsg struct {
	cycleID    int64
	stageName  string
	generation int64
	key        string
}

type stageBudgetCancelDoneMsg struct {
	cycleID       int64
	stageName     string
	workersActive int
	err           error
}

type stageBudgetExpiredMsg struct {
	cycleID    int64
	stageName  string
	generation int64
}

func stageBudgetMonitorKey(cycleID int64, stageName string, generation int64) string {
	return fmt.Sprintf("%d:%s:%d", cycleID, stageName, generation)
}

func (m model) startStageBudgetMonitor(cycleID int64, stageName string, generation int64) model {
	if cycleID <= 0 || strings.TrimSpace(stageName) == "" || generation <= 0 || m.convSink == nil {
		return m
	}
	key := stageBudgetMonitorKey(cycleID, stageName, generation)
	if m.budgetMonitorKey == key && m.budgetMonitorCancel != nil {
		return m
	}
	m = m.stopStageBudgetMonitor()
	ctx, cancel := context.WithCancel(context.Background())
	m.budgetMonitorCancel = cancel
	m.budgetMonitorKey = key
	sink := m.convSink
	remaining := time.Duration(0)
	if m.svc != nil && m.svc.Engine != nil {
		if budget, err := m.svc.Engine.StageBudget(cycleID, stageName); err == nil && budget.Generation == generation && budget.State == store.StageBudgetActive {
			remaining = budget.RemainingAt(m.svc.Engine.Now())
		}
	}
	go func() {
		ticker := time.NewTicker(workflowconfig.BudgetCheckpointInterval)
		defer ticker.Stop()
		var deadline <-chan time.Time
		var deadlineTimer *time.Timer
		if remaining <= 0 {
			deadline = time.After(0)
		} else {
			deadlineTimer = time.NewTimer(remaining)
			deadline = deadlineTimer.C
		}
		if deadlineTimer != nil {
			defer deadlineTimer.Stop()
		}
		tick := stageBudgetTickMsg{
			cycleID: cycleID, stageName: stageName,
			generation: generation, key: key,
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sink.Send(tick)
			case <-deadline:
				// Checkpoints remain periodic, but expiry also has a one-shot
				// deadline so short remaining budgets do not wait for the next
				// 5-second checkpoint tick before revoking acceptance.
				sink.Send(tick)
				return
			}
		}
	}()
	return m
}

func (m model) stopStageBudgetMonitor() model {
	if m.budgetMonitorCancel != nil {
		m.budgetMonitorCancel()
	}
	m.budgetMonitorCancel = nil
	m.budgetMonitorKey = ""
	return m
}

func (m model) handleStageBudgetTick(msg stageBudgetTickMsg) (model, tea.Cmd) {
	if m.svc == nil || m.svc.Engine == nil || msg.key == "" || msg.key != m.budgetMonitorKey {
		return m, nil
	}
	b, err := m.svc.Engine.CheckpointStageBudget(msg.cycleID, msg.stageName, msg.generation)
	if err != nil {
		m = m.stopStageBudgetMonitor()
		if errors.Is(err, store.ErrStageBudgetGeneration) || errors.Is(err, store.ErrNotFound) {
			return m, nil
		}
		slog.Error("tui stage budget checkpoint failed", "cycle_id", msg.cycleID, "stage", msg.stageName, "error", redact.Error(err))
		return m, nil
	}
	switch b.State {
	case store.StageBudgetActive:
		return m, nil
	case store.StageBudgetWaiting:
		if strings.HasPrefix(b.PauseReason, "human_") {
			return m.stopStageBudgetMonitor(), nil
		}
		return m.stopStageBudgetMonitor(), nil
	case store.StageBudgetExpired:
		m = m.stopStageBudgetMonitor()
		return m.cancelStageBudgetExecutions(msg.cycleID, msg.stageName, msg.generation, true)
	default:
		return m.stopStageBudgetMonitor(), nil
	}
}

// refreshStageBudgetWait pauses only when every still-running execution for a
// stage is blocked on a person. One runnable sibling keeps the shared clock on.
func (m model) refreshStageBudgetWait(cycleID int64, stageName string) model {
	if m.svc == nil || m.svc.Engine == nil || cycleID <= 0 || strings.TrimSpace(stageName) == "" {
		return m
	}
	var current *convExecute
	runnable := false
	waiting := 0
	for _, ex := range m.executes {
		if ex.Freechat || ex.BudgetCycleID != cycleID || ex.StageName != stageName || ex.BudgetGeneration <= 0 {
			continue
		}
		copy := ex
		current = &copy
		if ex.WaitingReason == "" {
			runnable = true
		} else {
			waiting++
		}
	}
	if current == nil {
		return m
	}
	m = m.setValidationProgressWaiting(!runnable && waiting > 0, m.validationProgressNow())
	b, err := m.svc.Engine.StageBudget(cycleID, stageName)
	if err != nil {
		return m
	}
	if !runnable && waiting > 0 && b.State == store.StageBudgetActive {
		reason := "human_wait"
		selectedID := ""
		for id, ex := range m.executes {
			if ex.Freechat || ex.BudgetCycleID != cycleID || ex.StageName != stageName || ex.WaitingReason == "" {
				continue
			}
			// Map iteration is unordered. Choose a stable execution so the
			// durable reason does not change between identical sibling waits.
			if selectedID == "" || id < selectedID {
				selectedID = id
				reason = ex.WaitingReason
			}
		}
		slog.Debug("tui stage budget awaiting human input", "cycle_id", cycleID, "stage", stageName, "reason", reason)
		paused, pauseErr := m.svc.Engine.PauseStageBudget(cycleID, stageName, b.Generation, reason)
		if pauseErr != nil {
			slog.Debug("tui stage budget human pause failed", "cycle_id", cycleID, "stage", stageName, "error", redact.Error(pauseErr))
			return m
		}
		if paused.State == store.StageBudgetExpired {
			if m.convSink != nil {
				m.convSink.Send(stageBudgetExpiredMsg{cycleID: cycleID, stageName: stageName, generation: b.Generation})
			}
			return m.stopStageBudgetMonitor()
		}
		return m.stopStageBudgetMonitor()
	}
	if runnable && b.State == store.StageBudgetWaiting && strings.HasPrefix(b.PauseReason, "human_") {
		resumed, resumeErr := m.svc.Engine.ResumeStageBudget(cycleID, stageName, b.Generation, false)
		if resumeErr != nil {
			slog.Debug("tui stage budget human resume failed", "cycle_id", cycleID, "stage", stageName, "error", redact.Error(resumeErr))
			return m
		}
		if resumed.State == store.StageBudgetActive {
			m = m.startStageBudgetMonitor(cycleID, stageName, resumed.Generation)
		}
	}
	return m
}

func (m model) setExecuteWaiting(executeID, reason string) model {
	ex, ok := m.executes[executeID]
	if !ok || ex.Freechat || ex.BudgetCycleID <= 0 || ex.BudgetGeneration <= 0 {
		return m
	}
	ex.WaitingReason = reason
	m.executes[executeID] = ex
	return m.refreshStageBudgetWait(ex.BudgetCycleID, ex.StageName)
}

func (m model) refreshExecuteWaitReason(executeID string) model {
	reason := ""
	for _, pending := range m.harnessPermissionRequests {
		if pending.executeID == executeID {
			reason = "human_permission"
			break
		}
	}
	if m.harnessQuestionPending && m.harnessQuestionExecuteID == executeID {
		reason = "human_question"
	}
	return m.setExecuteWaiting(executeID, reason)
}

func (m model) acceptStageExecution(ex convExecute) (bool, store.StageBudget, error) {
	if ex.Freechat || ex.BudgetCycleID <= 0 || strings.TrimSpace(ex.StageName) == "" || ex.BudgetGeneration <= 0 {
		return true, store.StageBudget{}, nil
	}
	if m.svc == nil || m.svc.Engine == nil {
		return false, store.StageBudget{}, fmt.Errorf("stage budget engine unavailable")
	}
	return m.svc.Engine.AcceptStageBudget(ex.BudgetCycleID, ex.StageName, ex.BudgetGeneration)
}

func (m model) rejectExpiredStageResult(ex convExecute) (model, tea.Cmd) {
	if ex.BudgetCycleID <= 0 || strings.TrimSpace(ex.StageName) == "" || ex.BudgetGeneration <= 0 || m.svc == nil || m.svc.Engine == nil {
		return m, nil
	}
	b, err := m.svc.Engine.StageBudget(ex.BudgetCycleID, ex.StageName)
	if err != nil {
		return m, nil
	}
	if b.State == store.StageBudgetExpired || b.RemainingAt(m.svc.Engine.Now()) <= 0 {
		return m.cancelStageBudgetExecutions(ex.BudgetCycleID, ex.StageName, ex.BudgetGeneration, true)
	}
	if b.State == store.StageBudgetInterrupted || b.State == store.StageBudgetBlocked || b.State == store.StageBudgetWaiting {
		delete(m.executes, ex.ID)
		m = m.removeLiveAgent(ex.ID)
		return m, nil
	}
	return m, nil
}

// cancelStageBudgetExecutions removes the acceptance set synchronously before
// asking only the stage's own harness sessions to stop.
func (m model) cancelStageBudgetExecutions(cycleID int64, stageName string, generation int64, timeout bool) (model, tea.Cmd) {
	if timeout && m.svc != nil && m.svc.Engine != nil {
		if b, err := m.svc.Engine.StageBudget(cycleID, stageName); err == nil && b.State != store.StageBudgetExpired {
			if _, expired, expireErr := m.svc.Engine.ExpireStageBudget(cycleID, stageName, generation); expireErr != nil || !expired {
				slog.Debug("tui stage budget expiry confirmation failed", "cycle_id", cycleID, "stage", stageName, "error", redact.Error(expireErr))
			}
		}
	}
	if preparation := m.stageBrowserPreparation; preparation != nil && preparation.cycleID == cycleID && preparation.stage == stageName && preparation.generation == generation {
		m = m.discardStageBrowserPreparation(false)
	}
	var executes []convExecute
	for id, ex := range m.executes {
		if ex.BudgetCycleID != cycleID || ex.StageName != stageName || ex.BudgetGeneration != generation {
			continue
		}
		delete(m.executes, id)
		executes = append(executes, ex)
		if ex.cancel != nil {
			ex.cancel()
		}
		if ex.relay != nil {
			ex.relay.Stop()
		}
		m = m.removeLiveAgent(id)
	}
	m = m.stopStageBudgetMonitor()
	if len(executes) == 0 {
		if timeout {
			m = m.markValidationProgressInterrupted(true)
			m.convError = "Stage timed out. Partial evidence is preserved; run /hero-continue --budget-increase MINUTES to add time."
			m = m.setStatusResult(false, "stage timeout", m.convError)
		}
		return m, nil
	}
	for key, pending := range m.harnessPermissionRequests {
		for _, ex := range executes {
			if pending.executeID == ex.ID {
				sendHarnessPermissionResponse(pending.respCh, harness.PermissionResponse{Approved: false, Reason: "cancelled"})
				m = m.removePendingHarnessPermissionNotice(key)
				m = m.removeHarnessPermissionKey(key)
				break
			}
		}
	}
	if m.harnessQuestionExecuteID != "" {
		for _, ex := range executes {
			if m.harnessQuestionExecuteID == ex.ID {
				m = m.clearHarnessQuestion()
				break
			}
		}
	}
	m.streaming = len(m.executes) > 0
	if len(m.executes) == 0 {
		m = m.stopAITimer(time.Now())
		m.chatInputFocused = true
		m.stageHandoffLive = false
	}
	if timeout {
		m = m.markValidationProgressInterrupted(true)
		m.convError = "Stage timed out. Partial evidence is preserved; run /hero-continue --budget-increase MINUTES to add time."
		m = m.setStatusResult(false, "stage timeout", m.convError)
	}
	fallbackAdapter := m.harnessAdapter()
	var injected harness.HarnessAdapter
	var registry harnessmgr.Registry
	if m.svc != nil {
		injected = m.svc.Harness
		registry = m.svc.Registry
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), workflowconfig.TerminationGracePeriod)
		defer cancel()
		cancelResults := make(chan error, len(executes))
		workers := 0
		for _, ex := range executes {
			if ex.done != nil {
				workers++
			}
			adapter := adapterForCancel(ex.HarnessID, injected, fallbackAdapter, registry)
			if adapter == nil {
				cancelResults <- fmt.Errorf("harness adapter unavailable")
				continue
			}
			sessionID := strings.TrimSpace(ex.SessionID)
			go func(adapter harness.HarnessAdapter, sessionID string) {
				cancelResults <- adapter.Cancel(ctx, sessionID)
			}(adapter, sessionID)
		}
		pendingCancels := len(executes)
		workerDone := make(chan struct{}, workers)
		for _, ex := range executes {
			if ex.done != nil {
				go func(done <-chan struct{}) {
					select {
					case <-done:
					case <-ctx.Done():
					}
					workerDone <- struct{}{}
				}(ex.done)
			}
		}
		remainingWorkers := workers
		var cancelErr error
		for pendingCancels > 0 || remainingWorkers > 0 {
			select {
			case err := <-cancelResults:
				pendingCancels--
				if err != nil && cancelErr == nil {
					cancelErr = err
				}
			case <-workerDone:
				remainingWorkers--
			case <-ctx.Done():
				stillActive := remainingWorkers
				if pendingCancels > stillActive {
					stillActive = pendingCancels
				}
				if cancelErr == nil {
					cancelErr = ctx.Err()
				}
				return stageBudgetCancelDoneMsg{cycleID: cycleID, stageName: stageName, workersActive: stillActive, err: cancelErr}
			}
		}
		return stageBudgetCancelDoneMsg{cycleID: cycleID, stageName: stageName, err: cancelErr}
	}
}

func (m model) captureActiveStageBudget(ex convExecute) convExecute {
	if m.svc == nil || m.svc.Engine == nil || ex.Freechat || strings.TrimSpace(ex.StageName) == "" {
		return ex
	}
	cycle, err := m.svc.Store.GetActiveCycle()
	if err != nil {
		return ex
	}
	b, err := m.svc.Engine.StageBudget(cycle.ID, ex.StageName)
	if err != nil || b.State != store.StageBudgetActive {
		return ex
	}
	ex.BudgetCycleID = cycle.ID
	ex.BudgetGeneration = b.Generation
	return ex
}

func (m model) maybeLogBudgetCancel(msg stageBudgetCancelDoneMsg) {
	if msg.err != nil {
		slog.Error("tui stage budget cancellation incomplete", "cycle_id", msg.cycleID, "stage", msg.stageName, "workers_active", msg.workersActive, "error", redact.Error(msg.err))
		return
	}
	if msg.workersActive > 0 {
		slog.Error("tui stage budget cancellation grace elapsed", "cycle_id", msg.cycleID, "stage", msg.stageName, "workers_active", msg.workersActive)
	}
}

func (m model) prepareContinueBudget(increase time.Duration) (string, error) {
	if m.svc == nil || m.svc.Store == nil || m.svc.Engine == nil {
		return "", nil
	}
	cycle, err := m.svc.Store.GetActiveCycle()
	if err != nil {
		return "", err
	}
	stages, err := m.svc.Store.ListStages(cycle.ID)
	if err != nil {
		return "", err
	}
	var escalated *store.Stage
	for i := range stages {
		if stages[i].Status == store.StageEscalated {
			escalated = &stages[i]
			break
		}
	}
	if escalated == nil {
		for i := range stages {
			if stages[i].Status != store.StageRunning {
				continue
			}
			budget, budgetErr := m.svc.Engine.StageBudget(cycle.ID, stages[i].Name)
			if budgetErr != nil || budget.RemainingAt(m.svc.Engine.Now()) > 0 {
				continue
			}
			if _, _, err := m.svc.Engine.ExpireStageBudget(cycle.ID, stages[i].Name, budget.Generation); err != nil {
				return "", err
			}
			stages, err = m.svc.Store.ListStages(cycle.ID)
			if err != nil {
				return "", err
			}
			for j := range stages {
				if stages[j].Status == store.StageEscalated {
					escalated = &stages[j]
					break
				}
			}
			break
		}
	}
	if escalated == nil {
		if increase > 0 {
			return "", fmt.Errorf("no escalated timed stage has a budget to increase")
		}
		return "", nil
	}
	budget, budgetErr := m.svc.Engine.StageBudget(cycle.ID, escalated.Name)
	if errors.Is(budgetErr, store.ErrNotFound) {
		if increase > 0 {
			return "", fmt.Errorf("stage %s has no configured time budget to increase", escalated.Name)
		}
		return escalated.Name, nil
	}
	if budgetErr != nil {
		return "", budgetErr
	}
	if increase > 0 {
		if _, err := m.svc.Engine.IncreaseStageBudget(cycle.ID, escalated.Name, increase); err != nil {
			return "", err
		}
		return escalated.Name, nil
	}
	if budget.State == store.StageBudgetExpired || budget.RemainingAt(m.svc.Engine.Now()) <= 0 {
		return "", fmt.Errorf("stage %s has no time remaining; explicitly add time with /hero-continue --budget-increase MINUTES", escalated.Name)
	}
	return escalated.Name, nil
}
