package engine

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// ReportValidationError wraps a typed report diagnostic from internal/cycle/reports.
type ReportValidationError struct {
	Diagnostic *reports.DiagnosticError
}

func (e *ReportValidationError) Error() string {
	if e == nil || e.Diagnostic == nil {
		return "invalid report"
	}
	return e.Diagnostic.Error()
}

// FailedCloseHandoffResult is returned after a successful atomic failed validation close.
type FailedCloseHandoffResult struct {
	FindingIDs []string
	Summary    string
}

// handoffTestHook runs inside the atomic handoff transaction after findings persist (tests only).
var handoffTestHook func(*sql.Tx) error

// CloseStageFailedWithFindings validates reportJSON, then atomically persists findings,
// closes the source stage as Failed, applies loop-back, and records audit data.
// Public entry points MUST NOT nest store.InTx around this method (ADR-084).
func (e *Engine) CloseStageFailedWithFindings(cycleID int64, stageName string, reportJSON []byte, metrics []MetricInput) (FailedCloseHandoffResult, error) {
	var empty FailedCloseHandoffResult
	stageName = strings.TrimSpace(stageName)
	if _, ok := loopBackSources[stageName]; !ok {
		return empty, fmt.Errorf("atomic failed close is not supported for stage %q", stageName)
	}
	if err := e.assertHandoffPreconditions(cycleID, stageName); err != nil {
		return empty, err
	}

	ctx, err := e.decodeContextForCycle(cycleID)
	if err != nil {
		return empty, err
	}
	summary, entries, derr := decodeFailedValidationReport(stageName, reportJSON, ctx)
	if derr != nil {
		return empty, &ReportValidationError{Diagnostic: derr}
	}
	if len(entries) == 0 {
		return empty, &ReportValidationError{
			Diagnostic: reportsDiagNoActionable("failures", "failed close requires at least one actionable finding entry"),
		}
	}

	var out FailedCloseHandoffResult
	err = e.Store.InTx(func(tx *sql.Tx) error {
		findingIDs, err := e.atomicFailedCloseHandoffTx(tx, cycleID, stageName, summary, reportJSON, entries, metrics)
		if err != nil {
			return err
		}
		out.FindingIDs = findingIDs
		out.Summary = summary
		return nil
	})
	if err != nil {
		e.Logger.Error("atomic failed close handoff failed",
			"cycle_id", cycleID, "stage", stageName, "error", err)
		return empty, err
	}
	e.Logger.Info("atomic failed close handoff committed",
		"cycle_id", cycleID, "stage", stageName, "finding_ids", out.FindingIDs)
	events, listErr := e.Store.ListEvents(cycleID, store.EventStageCompleted, 10)
	if listErr == nil {
		for i := len(events) - 1; i >= 0; i-- {
			if strings.Contains(events[i].PayloadJSON, `"status":"Failed"`) {
				e.publishFailedClose(events[i].ID, cycleID, stageName, out.Summary)
				break
			}
		}
	}
	return out, nil
}

func (e *Engine) assertHandoffPreconditions(cycleID int64, stageName string) error {
	st, err := e.Store.GetStage(cycleID, stageName)
	if err != nil {
		return err
	}
	if st.Status != store.StageRunning {
		return fmt.Errorf("stage %s is %s, expected Running", stageName, st.Status)
	}
	stages, err := e.Store.ListStages(cycleID)
	if err != nil {
		return err
	}
	for _, s := range stages {
		if s.Status == store.StageRunning && s.Name != stageName {
			return fmt.Errorf("stage %s is Running; close it before failed handoff", s.Name)
		}
	}
	return nil
}

func (e *Engine) atomicFailedCloseHandoffTx(
	tx *sql.Tx,
	cycleID int64,
	stageName, summary string,
	reportJSON []byte,
	entries []reports.FailureEntry,
	metrics []MetricInput,
) ([]string, error) {
	e.Logger.Debug("atomic handoff transaction begin",
		"cycle_id", cycleID, "stage", stageName, "entries", len(entries))

	if err := e.persistMetricsTx(tx, cycleID, stageName, metrics); err != nil {
		return nil, err
	}

	sourceStage := stageName
	var findingIDs []string
	var actionable int
	for _, ent := range entries {
		in := findingInputFromEntry(cycleID, sourceStage, ent)
		res, err := e.Store.PersistFindingTx(tx, in)
		if err != nil {
			e.Logger.Error("finding persist in handoff failed",
				"cycle_id", cycleID, "stage", stageName, "error", err)
			return nil, err
		}
		findingIDs = appendUniqueString(findingIDs, res.Finding.ID)
		if res.Actionable {
			actionable++
		}
	}
	if actionable == 0 {
		return nil, &ReportValidationError{
			Diagnostic: reportsDiagNoActionable("failures", "failed close requires at least one actionable finding entry"),
		}
	}

	if handoffTestHook != nil {
		if err := handoffTestHook(tx); err != nil {
			return nil, err
		}
	}

	st, err := e.Store.GetStageTx(tx, cycleID, stageName)
	if err != nil {
		return nil, err
	}
	st.Summary = summary
	st.Status = store.StageFailed
	st.CompletedAt = e.now()
	if err := e.Store.UpdateStageTx(tx, st); err != nil {
		return nil, err
	}

	if err := e.applyLoopBackInTx(tx, cycleID, stageName, summary, findingIDs); err != nil {
		return nil, err
	}

	if _, err := e.Store.AppendEventTx(tx, store.Event{
		CycleID:     cycleID,
		Type:        store.EventStageCompleted,
		PayloadJSON: fmt.Sprintf(`{"stage":%q,"status":"Failed"}`, stageName),
	}); err != nil {
		return nil, err
	}

	if _, err := e.Store.AddConversationTx(tx, store.ConversationEntry{
		CycleID: cycleID,
		Role:    "system",
		Kind:    "validation_report",
		Body:    string(reportJSON),
	}); err != nil {
		return nil, err
	}

	e.Logger.Debug("atomic handoff transaction steps complete",
		"cycle_id", cycleID, "stage", stageName, "finding_ids", findingIDs)
	return findingIDs, nil
}

func (e *Engine) persistMetricsTx(tx *sql.Tx, cycleID int64, defaultStage string, metrics []MetricInput) error {
	for _, m := range metrics {
		stage := m.StageName
		if stage == "" {
			stage = defaultStage
		}
		agent := m.Agent
		existing, err := e.Store.GetMetricTx(tx, cycleID, stage, agent)
		if err != nil {
			return err
		}
		merged := mergeMetricRow(existing, MetricInput{
			StageName:    stage,
			Model:        m.Model,
			Agent:        agent,
			InputTokens:  m.InputTokens,
			OutputTokens: m.OutputTokens,
			CostUSD:      m.CostUSD,
			DurationMS:   m.DurationMS,
		})
		merged.CycleID = cycleID
		if err := e.Store.UpsertMetricTx(tx, merged); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) applyLoopBackInTx(tx *sql.Tx, cycleID int64, fromStage, reason string, findingIDs []string) error {
	stages, err := e.Store.ListStagesTx(tx, cycleID)
	if err != nil {
		return err
	}
	var impl *store.Stage
	for i := range stages {
		st := &stages[i]
		if st.Name == "implementation" {
			impl = st
			break
		}
	}
	if impl == nil {
		return fmt.Errorf("implementation stage is not in this cycle")
	}
	switch impl.Status {
	case store.StageCompleted, store.StageFailed, store.StageWaiting:
	default:
		return fmt.Errorf("implementation is %s, cannot loop back", impl.Status)
	}

	if impl.Status == store.StageWaiting {
		e.Logger.Debug("implementation already waiting; updating summary in handoff tx",
			"cycle_id", cycleID, "from_stage", fromStage)
		impl.Summary = reason
		if err := e.Store.UpdateStageTx(tx, *impl); err != nil {
			return err
		}
		return e.appendLoopBackEventTx(tx, cycleID, fromStage, reason, findingIDs)
	}

	for i := range stages {
		st := stages[i]
		if st.SortOrder < impl.SortOrder {
			continue
		}
		if st.Status == store.StageSkipped {
			continue
		}
		e.Logger.Debug("resetting downstream stage in handoff tx",
			"cycle_id", cycleID, "stage", st.Name, "from", st.Status)
		st.Status = store.StageWaiting
		st.StartedAt = ""
		st.CompletedAt = ""
		if st.Name == "implementation" {
			st.Summary = reason
		}
		if err := e.Store.UpdateStageTx(tx, st); err != nil {
			return err
		}
	}
	return e.appendLoopBackEventTx(tx, cycleID, fromStage, reason, findingIDs)
}

func (e *Engine) appendLoopBackEventTx(tx *sql.Tx, cycleID int64, fromStage, reason string, findingIDs []string) error {
	payload := map[string]any{
		"from":   fromStage,
		"to":     "implementation",
		"reason": reason,
	}
	if len(findingIDs) > 0 {
		payload["finding_ids"] = findingIDs
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("loop-back event: %w", err)
	}
	_, err = e.Store.AppendEventTx(tx, store.Event{
		CycleID: cycleID, Type: store.EventLoopBack, PayloadJSON: string(b),
	})
	return err
}

func decodeFailedValidationReport(stageName string, data []byte, ctx reports.DecodeContext) (string, []reports.FailureEntry, *reports.DiagnosticError) {
	switch stageName {
	case "qa":
		r, err := reports.DecodeQA(data, ctx)
		if err != nil {
			return "", nil, err
		}
		if r.Status != reports.ValidationStatusFailed {
			return "", nil, reportsDiagInvalidEnum("status", r.Status, "atomic failed close requires status failed")
		}
		return r.Summary, r.Failures, nil
	case "judge":
		r, err := reports.DecodeJudge(data, ctx)
		if err != nil {
			return "", nil, err
		}
		if r.Status != reports.ValidationStatusFailed {
			return "", nil, reportsDiagInvalidEnum("status", r.Status, "atomic failed close requires status failed")
		}
		if r.SDDAmbiguity {
			return "", nil, reportsDiagInvalidEnum("sdd_ambiguity", "true", "SDD ambiguity reports must use the approval/back path, not atomic failed close")
		}
		return r.Summary, r.ImplementationGaps, nil
	case "browser_ui_validation":
		r, err := reports.DecodeBrowserUI(data, ctx)
		if err != nil {
			return "", nil, err
		}
		if r.Status != reports.ValidationStatusFailed {
			return "", nil, reportsDiagInvalidEnum("status", r.Status, "atomic failed close requires status failed")
		}
		return r.Summary, r.Failures, nil
	case "qa_end_to_end":
		r, err := reports.DecodeQAEndToEnd(data, ctx)
		if err != nil {
			return "", nil, err
		}
		if r.Status != reports.ValidationStatusFailed {
			return "", nil, reportsDiagInvalidEnum("status", r.Status, "atomic failed close requires status failed")
		}
		return r.Summary, r.Failures, nil
	default:
		return "", nil, reportsDiagInvalidEnum("stage", stageName, "stage cannot emit validation findings")
	}
}

// ValidationDecodeContext builds report decoders' scope and store-backed validators for a cycle.
func (e *Engine) ValidationDecodeContext(cycleID int64) (reports.DecodeContext, error) {
	return e.decodeContextForCycle(cycleID)
}

func (e *Engine) decodeContextForCycle(cycleID int64) (reports.DecodeContext, error) {
	c, err := e.Store.GetCycle(cycleID)
	if err != nil {
		return reports.DecodeContext{}, err
	}
	active, implAgents := activeOwnersFromConfigSnapshot(c.ConfigSnapshotJSON)
	ctx := reports.DecodeContext{
		ActiveOwners:               active,
		ActiveImplementationAgents: implAgents,
		ReopenIDs: reports.ReopenIDValidateFunc(func(req reports.ReopenRequest) *reports.DiagnosticError {
			if err := e.Store.ValidateReopenID(cycleID, req.SourceStage, req.Owner, req.ReopenID, req.File, req.Requirement, req.AcceptanceCriteria, req.ReproPackage, req.ReproTest); err != nil {
				if errors.Is(err, store.ErrInvalidReopenID) {
					return reportsUnknownReopenID(req.ReopenID)
				}
				return reportsDiagInternal(err.Error())
			}
			return nil
		}),
		Actionable: reports.ActionableFindingCheckFunc(func(sourceStage string, entries []reports.FailureEntry) *reports.DiagnosticError {
			return e.hasActionableFindings(cycleID, sourceStage, entries)
		}),
	}
	return ctx, nil
}

func (e *Engine) hasActionableFindings(cycleID int64, sourceStage string, entries []reports.FailureEntry) *reports.DiagnosticError {
	if len(entries) == 0 {
		return reportsDiagNoActionable("failures", "failed close requires at least one actionable finding entry")
	}
	for _, ent := range entries {
		in := findingInputFromEntry(cycleID, sourceStage, ent)
		ok, err := e.Store.PredictFindingActionable(cycleID, in)
		if err != nil {
			if errors.Is(err, store.ErrInvalidFindingContent) {
				return reportsDiagInvalidEnum("failures", "", err.Error())
			}
			return reportsDiagInternal(err.Error())
		}
		if ok {
			return nil
		}
	}
	return reportsDiagNoActionable("failures", "failed close requires at least one actionable finding entry")
}

type scopeConfigYAML struct {
	Backend        bool `yaml:"backend"`
	Frontend       bool `yaml:"frontend"`
	Native         bool `yaml:"native"`
	Script         bool `yaml:"script"`
	Infrastructure bool `yaml:"infrastructure"`
}

type workflowScopeYAML struct {
	Scope scopeConfigYAML `yaml:"scope"`
}

func activeOwnersFromConfigSnapshot(snapshot string) (reports.ActiveOwners, []string) {
	active := reports.ActiveOwners{
		reports.OwnerBackend:  {},
		reports.OwnerFrontend: {},
		reports.OwnerGeneric:  {},
	}
	var impl []string
	if strings.TrimSpace(snapshot) == "" || snapshot == "{}" {
		return active, []string{reports.OwnerBackend, reports.OwnerFrontend, reports.OwnerGeneric}
	}
	var cfg workflowScopeYAML
	if err := yaml.Unmarshal([]byte(snapshot), &cfg); err != nil {
		return active, []string{reports.OwnerBackend, reports.OwnerFrontend, reports.OwnerGeneric}
	}
	active = reports.ActiveOwners{}
	impl = nil
	if cfg.Scope.Backend {
		active[reports.OwnerBackend] = struct{}{}
		impl = append(impl, reports.OwnerBackend)
	}
	if cfg.Scope.Frontend {
		active[reports.OwnerFrontend] = struct{}{}
		impl = append(impl, reports.OwnerFrontend)
	}
	if cfg.Scope.Native || cfg.Scope.Script || cfg.Scope.Infrastructure {
		active[reports.OwnerGeneric] = struct{}{}
		impl = append(impl, reports.OwnerGeneric)
	}
	if len(active) == 0 {
		active[reports.OwnerBackend] = struct{}{}
		active[reports.OwnerFrontend] = struct{}{}
		active[reports.OwnerGeneric] = struct{}{}
		impl = []string{reports.OwnerBackend, reports.OwnerFrontend, reports.OwnerGeneric}
	}
	return active, impl
}

func findingInputFromEntry(cycleID int64, sourceStage string, ent reports.FailureEntry) store.FindingInput {
	in := store.FindingInput{
		CycleID:            cycleID,
		SourceStage:        sourceStage,
		Owner:              ent.Owner,
		File:               ent.File,
		Requirement:        ent.Requirement,
		Issue:              ent.Issue,
		AcceptanceCriteria: ent.AcceptanceCriteria,
		Evidence:           ent.Evidence,
		ReproPackage:       ent.Repro.Package,
		ReproTest:          ent.Repro.Test,
		ReproSource:        ent.Repro.Source,
	}
	if ent.ReopenID != nil {
		in.ReopenID = *ent.ReopenID
	}
	return in
}

func appendUniqueString(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}

func reportsDiagNoActionable(field, rule string) *reports.DiagnosticError {
	return &reports.DiagnosticError{
		Code:  reports.CodeNoActionableFinding,
		Field: field,
		Rule:  rule,
	}
}

func reportsDiagInvalidEnum(field, value, rule string) *reports.DiagnosticError {
	return &reports.DiagnosticError{
		Code:  reports.CodeInvalidEnum,
		Field: field,
		Value: value,
		Rule:  rule,
	}
}

func reportsUnknownReopenID(reopenID string) *reports.DiagnosticError {
	return &reports.DiagnosticError{
		Code:  reports.CodeUnknownReopenID,
		Field: "reopen_id",
		Value: reopenID,
		Rule:  "finding is not a done match in this cycle with matching source stage, owner, file, requirement, and acceptance criteria",
	}
}

func reportsDiagInternal(msg string) *reports.DiagnosticError {
	return &reports.DiagnosticError{
		Code:  reports.CodeInvalidJSON,
		Field: "",
		Rule:  msg,
	}
}

// publishFailedClose notifies downstream transports after a successful atomic handoff.
func (e *Engine) publishFailedClose(eventID int64, cycleID int64, stageName, summary string) {
	e.publish(eventID, conversation.EventError, cycleID, "", stageName, summary)
}
