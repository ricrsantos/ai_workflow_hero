package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const (
	validationProgressHarnessType = "hero.validation.progress"
	validationProgressNoProgress  = 60 * time.Second
	validationProgressRepeatLimit = 30
)

type validationCoverageProgress struct {
	profileID string
	result    string
}

type validationProgressState struct {
	cycleID       int64
	stage         string
	active        bool
	phase         string
	coverage      map[string]validationCoverageProgress
	currentID     string
	completedIDs  map[string]struct{}
	startedAt     time.Time
	lastProgress  time.Time
	pausedAt      time.Time
	waiting       bool
	warningIssued bool
	repeatKey     string
	repeatCount   int
	toolLifecycle string
	interrupted   bool
	expired       bool
	limit         time.Duration
	elapsed       time.Duration
	remaining     time.Duration
}

func isValidationProgressStage(stage string) bool {
	switch strings.TrimSpace(stage) {
	case stageBrowserUI, stageQAEndToEnd:
		return true
	default:
		return false
	}
}

func (m model) validationProgressNow() time.Time {
	if m.svc != nil && m.svc.Engine != nil && m.svc.Engine.Now != nil {
		return m.svc.Engine.Now()
	}
	return time.Now()
}

func (m model) beginValidationProgress(stage string) model {
	if !isValidationProgressStage(stage) || m.svc == nil || m.svc.Store == nil {
		m.validationProgress = validationProgressState{}
		return m
	}
	cycle, err := m.svc.SessionCycle()
	if err != nil || cycle == nil {
		m.validationProgress = validationProgressState{}
		return m
	}
	rows, err := m.svc.Store.ListStageCoverage(cycle.ID, stage)
	if err != nil {
		m.validationProgress = validationProgressState{}
		return m
	}
	now := m.validationProgressNow()
	state := validationProgressState{
		cycleID: cycle.ID, stage: stage, active: true, phase: "preparation",
		coverage:     make(map[string]validationCoverageProgress, len(rows)),
		completedIDs: make(map[string]struct{}, len(rows)),
		startedAt:    now, lastProgress: now,
	}
	for _, row := range rows {
		if !safeValidationID(row.ID) {
			continue
		}
		state.coverage[row.ID] = validationCoverageProgress{profileID: row.ProfileID, result: row.Result}
		if row.Result != store.StageCoveragePlanned {
			state.completedIDs[row.ID] = struct{}{}
		}
		if state.currentID == "" && row.Result == store.StageCoveragePlanned {
			state.currentID = row.ID
		}
	}
	if m.svc.Engine != nil {
		if budget, budgetErr := m.svc.Engine.StageBudget(cycle.ID, stage); budgetErr == nil {
			state = updateValidationProgressBudget(state, budget, now)
		}
	}
	m.validationProgress = state
	return m
}

func updateValidationProgressBudget(state validationProgressState, budget store.StageBudget, at time.Time) validationProgressState {
	if budget.Limit <= 0 {
		return state
	}
	remaining := budget.RemainingAt(at)
	state.limit = budget.Limit
	state.remaining = remaining
	state.elapsed = budget.Limit - remaining
	if state.elapsed < 0 {
		state.elapsed = 0
	}
	return state
}

func (m model) refreshValidationProgressBudget(at time.Time) model {
	state := m.validationProgress
	if !state.active || m.svc == nil || m.svc.Engine == nil {
		return m
	}
	budget, err := m.svc.Engine.StageBudget(state.cycleID, state.stage)
	if err == nil {
		m.validationProgress = updateValidationProgressBudget(state, budget, at)
	}
	return m
}

func safeValidationID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for i, r := range value {
		if !safeValidationIDRune(r) {
			return false
		}
		if i == 0 && !validationASCIIAlphaNumeric(r) {
			return false
		}
	}
	return true
}

func safeValidationIDRune(r rune) bool {
	return validationASCIIAlphaNumeric(r) || r == '_' || r == '.' || r == '-'
}

func validationASCIIAlphaNumeric(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

func safeValidationPhase(value string) string {
	switch value {
	case "preparation", "browser-permission", "selected-user", "fixture", "health", "visual", "e2e", "report":
		return value
	default:
		return ""
	}
}

// recordValidationProgressDelta accepts only Hero's structured marker and
// store-backed coverage/profile IDs. The raw stream text is never used here.
func (m model) recordValidationProgressDelta(delta harness.StreamDelta, at time.Time) (model, string, bool) {
	state := m.validationProgress
	if !state.active || state.waiting {
		return m, "", false
	}
	if delta.Kind == harness.StreamKindActivity && delta.HarnessType == validationProgressHarnessType {
		metadata := delta.Metadata
		phase := safeValidationPhase(metadata["phase"])
		phaseChanged := phase != "" && phase != state.phase
		if phaseChanged {
			state.phase = phase
			state.warningIssued = false
			state.repeatKey = ""
			state.repeatCount = 0
		}
		coverageID := strings.TrimSpace(metadata["coverage_id"])
		progress := false
		if row, ok := state.coverage[coverageID]; ok && safeValidationID(coverageID) {
			profileID := strings.TrimSpace(metadata["profile_id"])
			if profileID == "" || profileID == row.profileID {
				if state.currentID != coverageID {
					state.currentID = coverageID
					state.lastProgress = at
					state.warningIssued = false
					state.repeatKey = ""
					state.repeatCount = 0
					progress = true
				}
				if metadata["coverage_complete"] == "true" {
					if _, exists := state.completedIDs[coverageID]; !exists {
						state.completedIDs[coverageID] = struct{}{}
						state.lastProgress = at
						state.warningIssued = false
						state.repeatKey = ""
						state.repeatCount = 0
						progress = true
					}
				}
			}
		}
		if !progress && !phaseChanged {
			return m.recordValidationRepetitiveEvent(state, "coverage-progress", at)
		}
		m.validationProgress = state
		return m, "", false
	}

	signature := ""
	switch delta.Kind {
	case harness.StreamKindTool:
		switch delta.Phase {
		case harness.StreamPhaseStarted:
			state.toolLifecycle = "browser tool running"
		case harness.StreamPhaseCompleted:
			state.toolLifecycle = "browser tool completed"
		}
		signature = "tool:" + safeStreamPhase(delta.Phase)
	case harness.StreamKindActivity:
		state.toolLifecycle = "browser activity"
		signature = "activity:" + safeStreamPhase(delta.Phase)
	case harness.StreamKindWarning:
		state.toolLifecycle = "browser warning"
		signature = "warning:" + safeStreamPhase(delta.Phase)
	case harness.StreamKindSession:
		state.toolLifecycle = "browser session activity"
		signature = "session:" + safeStreamPhase(delta.Phase)
	default:
		return m, "", false
	}
	return m.recordValidationRepetitiveEvent(state, signature, at)
}

func safeStreamPhase(value string) string {
	switch value {
	case harness.StreamPhaseStarted:
		return "started"
	case harness.StreamPhaseCompleted:
		return "completed"
	default:
		return "event"
	}
}

func (m model) recordValidationRepetitiveEvent(state validationProgressState, signature string, at time.Time) (model, string, bool) {
	if signature == state.repeatKey {
		state.repeatCount++
	} else {
		state.repeatKey = signature
		state.repeatCount = 1
	}
	m.validationProgress = state
	if state.repeatCount >= validationProgressRepeatLimit && !state.warningIssued {
		return m.issueValidationProgressWarning(at, true)
	}
	return m, "", false
}

func (m model) setValidationProgressWaiting(waiting bool, at time.Time) model {
	state := m.validationProgress
	if !state.active || state.waiting == waiting {
		return m
	}
	if waiting {
		state.waiting = true
		state.pausedAt = at
	} else {
		if !state.pausedAt.IsZero() && at.After(state.pausedAt) && !state.lastProgress.IsZero() {
			state.lastProgress = state.lastProgress.Add(at.Sub(state.pausedAt))
		}
		state.waiting = false
		state.pausedAt = time.Time{}
	}
	m.validationProgress = state
	return m
}

func (m model) checkValidationProgressWarning(at time.Time) (model, string, bool) {
	state := m.validationProgress
	if !state.active || state.waiting || state.warningIssued || state.lastProgress.IsZero() || at.Before(state.lastProgress) {
		return m, "", false
	}
	if at.Sub(state.lastProgress) < validationProgressNoProgress {
		return m, "", false
	}
	return m.issueValidationProgressWarning(at, false)
}

func (m model) issueValidationProgressWarning(at time.Time, repetitive bool) (model, string, bool) {
	state := m.validationProgress
	if !state.active || state.waiting || state.warningIssued {
		return m, "", false
	}
	state.warningIssued = true
	m.validationProgress = state
	phase := safeValidationPhase(state.phase)
	if phase == "" {
		phase = "preparation"
	}
	remaining := "unlimited"
	if state.limit > 0 {
		remaining = state.remaining.Truncate(time.Second).String()
	}
	current := "coverage plan unavailable"
	if state.currentID != "" {
		current = state.currentID
		if row := state.coverage[state.currentID]; row.profileID != "" {
			current += " / " + row.profileID
		}
	}
	why := "no coverage progress for 60 seconds"
	if repetitive {
		why = "30 repetitive events without coverage progress"
	}
	message := fmt.Sprintf("%s is in %s (%s); %s. Active budget remaining: %s. The TUI continues monitoring; no corrective action was taken.",
		validationStageTitle(state.stage), phase, current, why, remaining)
	return m, message, true
}

func validationProgressPhaseLabel(phase string) string {
	switch safeValidationPhase(phase) {
	case "preparation":
		return "preparation"
	case "browser-permission":
		return "browser permission"
	case "selected-user":
		return "selected user"
	case "fixture":
		return "fixture check"
	case "health":
		return "health"
	case "visual":
		return "visual validation"
	case "e2e":
		return "E2E validation"
	case "report":
		return "report"
	default:
		return "preparation"
	}
}

func (m model) validationProgressStatusLines(at time.Time) []string {
	state := m.validationProgress
	if state.stage == "" || (!state.active && !state.interrupted && !state.expired) {
		return nil
	}
	stage := validationStageTitle(state.stage)
	phase := validationProgressPhaseLabel(state.phase)
	coverage := state.currentID
	profile := ""
	if coverage != "" {
		profile = state.coverage[coverage].profileID
	} else {
		coverage = "coverage plan unavailable"
	}
	if state.interrupted {
		return []string{fmt.Sprintf("→ %s interrupted · evidence preserved · /hero-continue", stage)}
	}
	if state.expired {
		return []string{fmt.Sprintf("→ %s expired · evidence preserved · /hero-continue with explicit budget increase", stage)}
	}
	completed := len(state.completedIDs)
	line1 := fmt.Sprintf("→ %s · %s · %s", stage, phase, coverage)
	if profile != "" {
		line1 += " / " + profile
	}
	line2 := "Coverage plan unavailable"
	if len(state.coverage) > 0 {
		line2 = fmt.Sprintf("Coverage %d/%d complete", completed, len(state.coverage))
	}
	if state.limit > 0 {
		line2 += fmt.Sprintf(" · elapsed %s · remaining %s", state.elapsed.Truncate(time.Second), state.remaining.Truncate(time.Second))
	}
	if state.toolLifecycle != "" {
		line2 += " · " + state.toolLifecycle
	}
	return []string{line1, line2}
}

func (m model) validationProgressWarningTick(at time.Time) model {
	updated, message, ok := m.checkValidationProgressWarning(at)
	if !ok {
		return updated
	}
	updated.transcript = append(updated.transcript, convMessage{role: convRoleWarning, content: message})
	updated.bumpTranscriptLayout()
	return updated
}

func (m model) stopValidationProgress() model {
	if m.validationProgress.stage != "" {
		m.validationProgress.active = false
		m.validationProgress.waiting = false
		m.validationProgress.pausedAt = time.Time{}
	}
	return m
}

func (m model) markValidationProgressInterrupted(expired bool) model {
	state := m.validationProgress
	if !isValidationProgressStage(state.stage) {
		return m
	}
	state.active = false
	state.waiting = false
	state.pausedAt = time.Time{}
	state.phase = "report"
	state.interrupted = !expired
	state.expired = expired
	m.validationProgress = state
	return m
}
