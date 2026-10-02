package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/testaccess"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

type convExecute struct {
	ID               string
	AgentName        string
	HarnessID        string
	Model            string
	Prompt           string
	Attachments      []harness.Attachment
	AttachmentChips  []tuiAttachment
	StageName        string
	BudgetCycleID    int64
	BudgetGeneration int64
	WaitingReason    string
	Wave             int
	UsageGeneration  int64
	SessionID        string
	AgentMsgIndex    int
	Origin           string // telegram:<address> when the turn came from Telegram
	Freechat         bool
	OccupancyKey     string
	HeroSessionID    string
	relay            *conversationStreamRelay
	cancel           context.CancelFunc
	done             <-chan struct{}
}

// stageAgentReport is the deliberately small contract between a stage agent
// and the TUI scheduler. The raw output is retained for the orchestrator so a
// human can still inspect the complete report when a gate refuses to close.
type stageAgentReport struct {
	Agent            string
	Stage            string
	Raw              string
	Status           string
	TasksCompleted   []string
	TasksRemaining   []string
	TestsPassed      bool
	AcceptanceGates  bool
	AcceptanceValues map[string]bool
	Valid            bool
	ValidationError  string
}

const maxImplementationHandoffWaves = 8

// maxFindingRounds is how many times one finding may travel
// validation → Implementation → validation before Hero stops re-dispatching it.
// Without this ceiling a single finding that the implementation agent cannot
// actually fix keeps the cycle looping until the stage iteration budget runs
// out, which is both slow and expensive. On the cap the stage is Escalated so
// the user can grant iterations (/hero-continue) or defer it (/hero-add-todo).
const maxFindingRounds = 3

type implementationChecklist struct {
	Path    string
	Linked  bool
	Ready   bool
	Raw     string
	Pending []string
}

type stageHandoffDecision struct {
	Complete        bool
	PartialProgress bool
	Reports         []stageAgentReport
	Checklist       implementationChecklist
	Reason          string
	ChatCopy        string
	OmitAgentOutput bool
	// SchedulerHandledFailure is true when validation failed close + loop-back was persisted.
	SchedulerHandledFailure bool
	// SchedulerHandledBlocked is true when an operational validation blocker was
	// atomically persisted. It deliberately does not resume an orchestrator turn.
	SchedulerHandledBlocked  bool
	JudgeSDDAmbiguity        bool
	CloseImplementationEmpty bool
	// EscalateReason stops the loop and hands the decision to the user instead
	// of asking the orchestrator for yet another wave.
	EscalateReason string
	// ContractWarnings are deviations Hero tolerated while decoding the report
	// (ignored extra fields, normalized key names). They are shown to the user
	// so a drifting agent prompt is visible, and never block the stage.
	ContractWarnings      []reports.ReportWarning
	ScreenshotWarnings    []string
	NormalizedAgentOutput string
}

const (
	stagePlanning       = "planning"
	stageImplementation = "implementation"
	stageQA             = "qa"
	stageJudge          = "judge"
	stageBrowserUI      = "browser_ui_validation"
	stageQAEndToEnd     = "qa_end_to_end"
	agentPlanning       = "planning_agent"
	agentBackend        = "backend_agent"
	agentFrontend       = "frontend_agent"
	agentGeneric        = "generic_agent"
	agentQA             = "qa_agent"
	agentJudge          = "judge_agent"
	agentBrowserUI      = "browser_ui_agent"
	agentEnd2End        = "end2end_qa_agent"
)

func (m model) maybeHandoffAfterExecute() (model, tea.Cmd) {
	if m.svc == nil {
		return m, nil
	}
	agent := strings.TrimSpace(m.runtimeAgentName)
	if m.runtimeCommandName == "continue" && !m.researchLive && len(m.executes) == 0 && (agent == agentOrchestration || agent == "") {
		// /hero-continue used to set orchestrationLive=false, so this gate ran
		// after ORCH STOP and never launched the granted Running stage.
		m.orchestrationLive = true
		m.stageProgressHoldUntilStart = false
		if m.stageHandoffLive {
			m = m.clearStageHandoffState()
		}
		m.stageHandoffDoneKey = ""
		return m.ensureStageProgress()
	}
	if !m.orchestrationLive {
		return m, nil
	}
	// /hero-approve follow-up can run while researchLive is still set from discover.
	if m.researchLive && agent == agentOrchestration && m.researchStageClosedOrMovedOn() {
		m.researchLive = false
	}
	if agent == agentOrchestration && !m.researchLive && !m.stageHandoffLive && m.stageRunning(stageResearch) {
		return m.startDiscoverResearchSession()
	}
	if m.researchLive && (agent == agentDiscover || agent == "") && m.researchStageClosedOrMovedOn() {
		return m.resumeOrchestratorAfterResearch()
	}
	if m.researchLive && agent == agentOrchestration && m.stageRunning(stageResearch) {
		m.harnessSessionID = m.researchSessionID
		m = m.withRuntimeAgent(agentDiscover)
		m = m.applyAgentRuntimePair(agentDiscover, "")
		m = m.bindSessionToRuntimeHarness()
	}
	if m.stageHandoffLive && agent != agentOrchestration && len(m.executes) == 0 {
		return m.resumeOrchestratorAfterStageHandoff()
	}
	if !m.researchLive && !m.stageHandoffLive && (agent == agentOrchestration || agent == "") {
		return m.ensureStageProgress()
	}
	return m, nil
}

func (m model) stageRunning(name string) bool {
	if m.svc == nil {
		return false
	}
	st, err := m.svc.ActiveStage()
	return err == nil && st.Name == name && st.Status == store.StageRunning
}

func (m model) stageInteractive(name string) bool {
	if m.svc == nil {
		return false
	}
	st, err := m.svc.ActiveStage()
	if err != nil {
		return false
	}
	if st.Name != name {
		return false
	}
	switch st.Status {
	case store.StageRunning:
		return true
	default:
		return false
	}
}

func (m model) runningStageAgents() []string {
	if m.svc == nil {
		return nil
	}
	st, err := m.svc.ActiveStage()
	if err != nil {
		return nil
	}
	switch st.Status {
	case store.StageRunning:
		return m.namedStageAgents(st)
	default:
		return nil
	}
}

func (m model) waitingNamedStageAgents() []string {
	if m.svc == nil {
		return nil
	}
	st, err := m.svc.ActiveStage()
	if err != nil {
		return nil
	}
	if st.Status != store.StageWaiting {
		return nil
	}
	return m.namedStageAgents(st)
}

func (m model) startWaitingActiveStage() error {
	st, err := m.svc.ActiveStage()
	if err != nil {
		return err
	}
	if st.Status != store.StageWaiting {
		return nil
	}
	return m.svc.StartStage(st.Name)
}

func (m model) namedStageAgents(st store.Stage) []string {
	switch st.Name {
	case stageResearch:
		return nil
	case stagePlanning:
		return []string{agentPlanning}
	case stageImplementation:
		return m.implementationAgentsFromScope()
	case stageQA:
		return []string{agentQA}
	case stageJudge:
		return []string{agentJudge}
	case stageBrowserUI:
		return []string{agentBrowserUI}
	case stageQAEndToEnd:
		return []string{agentEnd2End}
	default:
		return nil
	}
}

func (m model) runningStageHandoffKey() string {
	if m.svc == nil {
		return ""
	}
	st, err := m.svc.ActiveStage()
	if err != nil {
		return ""
	}
	switch st.Status {
	case store.StageRunning:
		return fmt.Sprintf("%s:%d", st.Name, st.Iteration)
	default:
		return ""
	}
}

func (m model) implementationAgentsFromScope() []string {
	if m.svc == nil {
		return nil
	}
	doc, err := workflowconfig.LoadCurrentDocument(m.svc.ProjectDir)
	if err != nil || doc == nil {
		return []string{agentGeneric}
	}
	var agents []string
	if doc.Config.Scope.Backend {
		agents = append(agents, agentBackend)
	}
	if doc.Config.Scope.Frontend {
		agents = append(agents, agentFrontend)
	}
	if doc.Config.Scope.Native || doc.Config.Scope.Script || doc.Config.Scope.Infrastructure {
		agents = append(agents, agentGeneric)
	}
	if len(agents) == 0 {
		return []string{agentGeneric}
	}
	return agents
}

func (m model) startStageAgentSessions(agents []string) (model, tea.Cmd) {
	if len(agents) == 0 || m.svc == nil {
		return m, nil
	}
	st, err := m.svc.ActiveStage()
	if err != nil {
		// Without a resolved stage there is no safe prompt or gate to attach
		// the Execute to. Do not fall through with an empty stage and dispatch
		// an orphaned agent.
		m.convError = fmt.Sprintf("resolve active stage: %v", err)
		m.stageHandoffInterventionRequired = true
		return m.emitSchedulerCTA(schedulerCTAStartFailed, store.Stage{Name: "cycle"}, err.Error())
	}
	// Escalated is a terminal control state until /hero-continue grants
	// iterations and the engine moves the stage back to Waiting. Never let a
	// stale TUI handoff launch an agent directly from Escalated.
	if st.Status != store.StageRunning {
		m = m.discardStageBrowserPreparation(false)
		return m.progressCTAForStage(st)
	}
	stage := strings.TrimSpace(st.Name)
	if !isValidationProgressStage(stage) && m.stageBrowserPreparation != nil {
		m = m.discardStageBrowserPreparation(false)
	}
	if stage == stageImplementation {
		empty, emptyErr := m.svc.ImplementationWorkloadEmpty()
		if emptyErr != nil {
			slog.Error("implementation workload check failed", "error", redact.Error(emptyErr))
			return m.returnStageAgentPreparationFailure(stage, emptyErr.Error())
		}
		if empty {
			summary := "Implementation assignment empty after scheduler recheck"
			if err := m.svc.CloseImplementationWhenAssignmentEmpty(summary); err != nil {
				slog.Error("empty implementation close failed", "error", redact.Error(err))
				return m.returnStageAgentPreparationFailure(stage, err.Error())
			}
			m.stageHandoffLive = false
			m.stageHandoffStage = ""
			m.stageHandoffOutputs = nil
			m = m.restoreOrchestratorSession()
			m = m.withRuntimeAgent(agentOrchestration)
			m.runtimeCommandName = "start"
			m = m.applyAgentRuntimePair(agentOrchestration, "")
			m = m.bindSessionToRuntimeHarness()
			label := "✓ Implementation assignment empty after scheduler recheck"
			prompt := formatImplementationAssignmentEmptyCloseChat() + tuiHeroStartContinueAfterStagePreamble(stageImplementation)
			m = m.beginSystemConversationExecute(label, prompt)
			return m, m.conversationExecuteCmds()
		}
	}
	if stage == "" {
		m.convError = "resolve active stage: active stage has an empty name"
		m.stageHandoffInterventionRequired = true
		return m.emitSchedulerCTA(schedulerCTAStartFailed, st, "active stage has an empty name")
	}
	preparationJSON := ""
	if isValidationProgressStage(stage) {
		cycle, cycleErr := m.svc.SessionCycle()
		if cycleErr != nil || cycle == nil {
			return m.returnStageAgentPreparationFailure(stage, "active cycle is unavailable for browser preparation")
		}
		preparation := m.stageBrowserPreparation
		if preparation == nil || preparation.cycleID != cycle.ID || preparation.stage != stage || preparation.iteration != st.Iteration {
			return m.beginStageBrowserPreparation(st, agents)
		}
		if preparation.pending || preparation.result == nil {
			return m, nil
		}
		result := blockedPreparationResult(*preparation.result, testaccess.PreparationReasonCheckFailed)
		if result.Status != testaccess.PreparationReady {
			return m.reportStageBrowserPreparationBlocked(preparation, result)
		}
		encoded, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return m.returnStageAgentPreparationFailure(stage, "bounded preparation result could not be serialized safely")
		}
		preparationJSON = string(encoded)
	}
	if m.stageHandoffStage != stage {
		m.stageHandoffWave = 0
	}
	m.stageHandoffWave++
	if sid := strings.TrimSpace(m.harnessSessionID); sid != "" {
		if strings.TrimSpace(m.orchestrationSessionID) == "" {
			owner := strings.TrimSpace(strings.ToLower(m.harnessSessionHarnessID))
			if owner == "" {
				owner = strings.TrimSpace(strings.ToLower(m.agentHarnessForName(agentOrchestration)))
			}
			if owner != "" {
				m = m.persistOrchestrationSessionPair(sid, owner)
			}
		}
	}
	checklist := m.implementationChecklist()
	runAgents := append([]string(nil), agents...)
	assignments := make(map[string][]implementationTaskBlock)
	expectedAgents := append([]string(nil), agents...)
	if stage == stageImplementation {
		var reason string
		findings, findErr := m.actionableImplementationFindings()
		if findErr != nil {
			m.stageHandoffPendingBefore = append([]string(nil), checklist.Pending...)
			return m.returnStageAgentPreparationFailure(stage, findErr.Error())
		}
		if stalled := findingsOverRoundCap(findings); len(stalled) > 0 {
			return m.escalateStalledFindings(st, stalled)
		}
		occsByID, occErr := m.findingOccurrenceHistory(findings)
		if occErr != nil {
			m.stageHandoffPendingBefore = append([]string(nil), checklist.Pending...)
			return m.returnStageAgentPreparationFailure(stage, occErr.Error())
		}
		runAgents, assignments, expectedAgents, reason = implementationStageDispatch(checklist, agents, findings, occsByID, m.svc.ReproPolicy())
		if reason != "" {
			m.stageHandoffPendingBefore = append([]string(nil), checklist.Pending...)
			return m.returnStageAgentPreparationFailure(stage, reason)
		}
	}
	if len(runAgents) == 0 {
		return m.returnStageAgentPreparationFailure(stage, "no active implementation agent owns a pending task or actionable finding")
	}

	// Build every prompt before starting the first Execute. A later prompt
	// failure must never leave an earlier stage agent running without its
	// sibling report expected by the gate.
	type preparedStageAgentPrompt struct {
		agent  string
		slug   string
		prompt string
		label  string
	}
	prepared := make([]preparedStageAgentPrompt, 0, len(runAgents))
	// Reuse the stage row already resolved and checked above. A second
	// ActiveStage lookup could fail or observe a different stage between
	// validation and dispatch, while the prompts would still be launched with
	// the first stage's ownership plan.
	stageSummary := strings.TrimSpace(st.Summary)
	for _, agent := range runAgents {
		body, err := m.readStageAgentPrompt(agent)
		if err != nil {
			slog.Error("tui stage agent prompt read failed", "agent", agent, "error", redact.Error(err))
			return m.returnStageAgentPreparationFailure(stage, err.Error())
		}
		slug, _ := m.stageAgentModelSlug(agent)
		prompt := tuiStageAgentPreamble(stage, agent) + strings.TrimSpace(body) + "\n"
		if stage == stageImplementation {
			prompt += formatImplementationAssignment(checklist, assignments[agent], m.stageHandoffWave)
		}
		if stageSummary != "" {
			prompt += "\n## Orchestrator assignment (loop-back)\n\n" + stageSummary + "\n"
		}
		if preparationJSON != "" {
			prompt += "\n## Hero bounded preparation result (sanitized)\n\n" + preparationJSON + "\n"
		}
		if feedback := strings.TrimSpace(m.stageHandoffReportFeedback); feedback != "" {
			prompt += "\n## Previous report rejected — reissue it\n\n" +
				"Hero could not decode your last report and persisted nothing. " +
				"Do not redo the stage work; re-emit the same findings as one valid JSON object.\n\n" +
				"Diagnostic: " + feedback + "\n\n" +
				"Extra fields are ignored with a warning, so the report fails only on a missing or malformed field. " +
				"Emit the JSON object as your entire completion output and stop.\n"
		}
		if strings.TrimSpace(prompt) == "" {
			return m.returnStageAgentPreparationFailure(stage, fmt.Sprintf("empty prompt for %s", agent))
		}
		label := "→ " + strings.TrimSpace(stage)
		if len(runAgents) > 1 {
			label = fmt.Sprintf("→ %s (%s)", strings.TrimSpace(stage), agentShortLabel(agent))
		}
		prepared = append(prepared, preparedStageAgentPrompt{agent: agent, slug: slug, prompt: prompt, label: label})
	}

	m.stageHandoffLive = true
	m.stageHandoffInterventionRequired = false
	// The diagnostic is now baked into the prompts above; a later wave must not
	// inherit a stale "your report was rejected" block.
	m.stageHandoffReportFeedback = ""
	m.stageProgressHoldUntilStart = false
	m.stageProgressCTAKey = ""
	m.stageHandoffStage = stage
	m.stageBrowserPreparation = nil
	m = m.beginScreenshotAutoWatch(stage, st.Iteration)
	m.stageHandoffOutputs = nil
	m.stageHandoffPreparationError = ""
	m = m.clearReproGateState()
	m.stageHandoffExpectedAgents = append([]string(nil), expectedAgents...)
	m.stageHandoffDoneKey = m.runningStageHandoffKey()
	m.harnessSessionID = ""
	m.harnessSessionHarnessID = ""
	m.conversationStage = stage
	m = m.beginValidationProgress(stage)
	m.runtimeCommandName = ""
	m.stageHandoffPendingBefore = append([]string(nil), checklist.Pending...)
	if m.stageHandoffAssignments == nil {
		m.stageHandoffAssignments = make(map[int]map[string][]implementationTaskBlock)
	}
	m.stageHandoffAssignments[m.stageHandoffWave] = cloneImplementationAssignments(assignments)

	var cmd tea.Cmd
	for i, item := range prepared {
		agent := item.agent
		m = m.withRuntimeAgent(agent)
		m = m.applyAgentRuntimePair(agent, item.slug)
		if m.svc != nil {
			taskIDs := make([]string, 0, len(assignments[agent]))
			for _, task := range assignments[agent] {
				taskIDs = append(taskIDs, task.ID)
			}
			if err := m.svc.RecordStageAgentAssignmentWithTasks(stage, agent, m.stageHandoffWave, taskIDs, item.prompt); err != nil {
				slog.Warn("tui persist stage agent assignment failed", "stage", stage, "agent", agent, "wave", m.stageHandoffWave, "error", redact.Error(err))
			}
		}
		if i == 0 {
			m = m.beginSystemConversationExecute(item.label, item.prompt)
			cmd = m.conversationExecuteCmds()
		} else {
			m = m.appendSystemConversationExecute(item.label, item.prompt)
		}
	}
	return m, cmd
}

func implementationStageDispatch(checklist implementationChecklist, activeAgents []string, findings []store.Finding, occsByID map[string][]store.FindingOccurrence, policy findingrepro.Policy) ([]string, map[string][]implementationTaskBlock, []string, string) {
	return buildImplementationStageDispatch(checklist, activeAgents, findings, occsByID, policy)
}

func (m model) actionableImplementationFindings() ([]store.Finding, error) {
	if m.svc == nil {
		return nil, nil
	}
	cycle, err := m.svc.SessionCycle()
	if err != nil {
		return nil, fmt.Errorf("resolve active cycle: %w", err)
	}
	if cycle == nil {
		return nil, fmt.Errorf("active cycle is unavailable")
	}
	findings, err := m.svc.Store.ListActionableFindings(cycle.ID, "")
	if err != nil {
		slog.Error("implementation assignment findings query failed", "cycle_id", cycle.ID, "error", redact.Error(err))
		return nil, fmt.Errorf("list actionable findings: %w", err)
	}
	return findings, nil
}

func (m model) findingOccurrenceHistory(findings []store.Finding) (map[string][]store.FindingOccurrence, error) {
	if m.svc == nil || m.svc.Store == nil || len(findings) == 0 {
		return nil, nil
	}
	cycle, err := m.svc.SessionCycle()
	if err != nil {
		return nil, fmt.Errorf("resolve active cycle: %w", err)
	}
	if cycle == nil {
		return nil, nil
	}
	out := make(map[string][]store.FindingOccurrence, len(findings))
	for _, f := range findings {
		id := strings.TrimSpace(f.ID)
		if id == "" {
			continue
		}
		occs, err := m.svc.Store.ListFindingOccurrences(cycle.ID, id)
		if err != nil {
			slog.Error("implementation assignment occurrence query failed", "finding_id", id, "error", redact.Error(err))
			return nil, fmt.Errorf("list finding occurrences: %w", err)
		}
		if len(occs) > 0 {
			out[id] = occs
		}
	}
	return out, nil
}

func cloneImplementationAssignments(assignments map[string][]implementationTaskBlock) map[string][]implementationTaskBlock {
	clone := make(map[string][]implementationTaskBlock, len(assignments))
	for agent, tasks := range assignments {
		clone[agent] = append([]implementationTaskBlock(nil), tasks...)
	}
	return clone
}

func (m model) returnStageAgentPreparationFailure(stage, reason string) (model, tea.Cmd) {
	m.stageHandoffLive = true
	m.stageHandoffStage = stage
	m.stageHandoffOutputs = nil
	m.stageHandoffExpectedAgents = nil
	m.stageHandoffPreparationError = strings.TrimSpace(reason)
	m.stageHandoffInterventionRequired = true
	m.stageHandoffDoneKey = m.runningStageHandoffKey()
	m.runtimeCommandName = ""
	return m.resumeOrchestratorAfterStageHandoff()
}

func (m model) stageAgentModelSlug(agentName string) (slug string, warned bool) {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	slug, usedFallback, err := workflowconfig.AgentModelSlug(projectDir, agentName)
	if err != nil || strings.TrimSpace(slug) == "" {
		return m.defaultHarnessModelSlug(), true
	}
	if usedFallback {
		return slug, true
	}
	return slug, false
}

func (m model) readStageAgentPrompt(agentName string) (string, error) {
	if m.svc == nil {
		return "", fmt.Errorf("cycle service unavailable")
	}
	harnessID := m.agentHarnessForName(agentName)
	candidates := []string{
		filepath.Join(m.svc.ProjectDir, agentPromptRel(harnessID, agentName)),
		filepath.Join(m.svc.ProjectDir, cursoradapter.AgentsDir, agentName+".md"),
	}
	var lastErr error
	for _, path := range candidates {
		body, err := cursoradapter.ReadAgentPrompt(path)
		if err == nil && strings.TrimSpace(body) != "" {
			return body, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return "", fmt.Errorf("read %s: %w", agentName, lastErr)
}

func agentPromptRel(harnessID, agentName string) string {
	file := agentName + ".md"
	switch strings.ToLower(strings.TrimSpace(harnessID)) {
	case "claude":
		return filepath.Join(".claude", "agents", file)
	case "opencode":
		return filepath.Join(".opencode", "agents", file)
	case "codex":
		return filepath.Join(".codex", "agents", file)
	default:
		return filepath.Join(cursoradapter.AgentsDir, file)
	}
}

func (m model) implementationChecklist() implementationChecklist {
	checklist := implementationChecklist{}
	if m.svc == nil {
		return checklist
	}
	cycle, err := m.svc.SessionCycle()
	if err != nil || cycle == nil {
		return checklist
	}
	change := strings.TrimSpace(cycle.OpenspecChange)
	if change == "" || change == "." || change == ".." || strings.ContainsAny(change, `/\\`) {
		return checklist
	}
	checklist.Linked = true
	checklist.Path = filepath.Join(m.svc.ProjectDir, "openspec", "changes", change, "tasks.md")
	info, err := os.Lstat(checklist.Path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return checklist
	}
	raw, err := os.ReadFile(checklist.Path)
	if err != nil {
		return checklist
	}
	checklist.Ready = true
	checklist.Raw = string(raw)
	checklist.Pending = pendingOpenSpecTasks(checklist.Raw)
	return checklist
}

func pendingOpenSpecTasks(raw string) []string {
	var pending []string
	for _, task := range parseImplementationTaskBlocks(raw) {
		if !task.Pending {
			continue
		}
		header, ok := parseImplementationTaskHeader(task.Header)
		if !ok {
			continue
		}
		text := strings.TrimSpace(header.rest)
		if text != "" {
			pending = append(pending, text)
		}
	}
	return pending
}

func formatImplementationAssignment(checklist implementationChecklist, tasks []implementationTaskBlock, wave int) string {
	if wave < 1 {
		wave = 1
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Explicit implementation assignment (wave %d)\n\n", wave)
	if !checklist.Linked {
		b.WriteString("No OpenSpec tasks file is linked to the active cycle. Do not infer an assignment or edit files; report status blocked and ask the orchestrator to set openspec_change before retrying.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "OpenSpec tasks file: %s\n", checklist.Path)
	b.WriteString("ownership_validated:true\n")
	b.WriteString("Read the task dependency/parallel-group rules in that file before editing; the blocks below are the exact tasks assigned to you for this wave.\n")
	b.WriteString("Read verification guidance in docs/testing/TESTING.md. Applicable checks include: go test ./... and openspec validate <change> --strict.\n")
	if !checklist.Ready {
		b.WriteString("The linked tasks.md could not be read. Do not claim completion; report status blocked with the file error.\n")
		return b.String()
	}
	if len(tasks) == 0 {
		b.WriteString("→ Implementation verification · no assigned task or finding IDs\n")
		b.WriteString("Required report: tasks_completed=[] · tasks_remaining=[]\n")
		b.WriteString("No unchecked task checkboxes or actionable findings remain at wave start. Verify the acceptance gates and return a complete JSON report only if they pass.\n")
		return b.String()
	}
	var planned, findingIDs []string
	for _, task := range tasks {
		id := strings.TrimSpace(task.ID)
		if id == "" {
			continue
		}
		if strings.HasPrefix(id, "find-") {
			findingIDs = append(findingIDs, id)
		} else {
			planned = append(planned, id)
		}
	}
	b.WriteString("Tasks and findings that must be handled in this wave:\n\n")
	if len(planned) > 0 {
		fmt.Fprintf(&b, "  planned: %s\n", strings.Join(planned, ", "))
	}
	if len(findingIDs) > 0 {
		fmt.Fprintf(&b, "  findings: %s\n\n", strings.Join(findingIDs, ", "))
	} else if len(planned) > 0 {
		b.WriteString("\n")
	}
	for _, task := range tasks {
		if block := strings.TrimSpace(task.Block); block != "" {
			b.WriteString(strings.TrimRight(block, "\n"))
			b.WriteString("\n\n")
		}
	}
	b.WriteString("Do not edit task checkboxes. After a structurally valid report, the scheduler marks completed task-* IDs in tasks.md and finding-* IDs done in SQLite. Do not report complete while any assigned ID remains.\n")
	return b.String()
}

func parseStageAgentReport(raw, agent string) stageAgentReport {
	report := stageAgentReport{Agent: strings.TrimSpace(agent), Raw: raw}
	obj, ok := extractStageReportObject(raw)
	if !ok {
		report.ValidationError = "no JSON report object found"
		return report
	}
	data, err := json.Marshal(obj)
	if err != nil {
		report.ValidationError = "report object could not be encoded for typed decode"
		return report
	}
	// Bootstrap assignment from the report's own ID arrays so DecodeImplementation can
	// enforce structure/enums/gates; the scheduler still applies the real wave union.
	bootstrap := append([]string{}, softReportTaskIDs(obj, "tasks_completed")...)
	bootstrap = append(bootstrap, softReportTaskIDs(obj, "tasks_remaining")...)
	decoded, derr := reports.DecodeImplementation(data, report.Agent, bootstrap, reports.DecodeContext{})
	if derr != nil {
		report.ValidationError = derr.Error()
		return report
	}
	report.Stage = decoded.Stage
	report.Agent = decoded.Agent
	report.Status = decoded.Status
	report.TasksCompleted = append([]string{}, decoded.TasksCompleted...)
	report.TasksRemaining = append([]string{}, decoded.TasksRemaining...)
	report.TestsPassed = decoded.TestsPassed
	report.AcceptanceValues = decoded.AcceptanceGates
	allGates := true
	for _, v := range decoded.AcceptanceGates {
		if !v {
			allGates = false
			break
		}
	}
	report.AcceptanceGates = allGates
	report.Valid = true
	return report
}

func softReportTaskIDs(obj map[string]json.RawMessage, field string) []string {
	raw, ok := obj[field]
	if !ok {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if id := strings.TrimSpace(v); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func extractStageReportObject(raw string) (map[string]json.RawMessage, bool) {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '{' {
			continue
		}
		var obj map[string]json.RawMessage
		decoder := json.NewDecoder(strings.NewReader(raw[i:]))
		if err := decoder.Decode(&obj); err != nil || obj == nil {
			continue
		}
		if _, ok := obj["status"]; ok {
			return obj, true
		}
	}
	return nil, false
}

func requiredReportStringSlice(fields map[string]json.RawMessage, name string) ([]string, error) {
	raw, ok := fields[name]
	if !ok {
		return nil, fmt.Errorf("%s is required", name)
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("%s must be a string array", name)
	}
	if values == nil {
		values = []string{}
	}
	return values, nil
}

func normalizeStageAgentTaskID(raw string) (string, error) {
	return normalizeImplementationAssignmentID(raw)
}

func validateStageAgentTaskIDLists(completed, remaining []string) error {
	seen := make(map[string]string, len(completed)+len(remaining))
	for _, id := range completed {
		if previous, ok := seen[id]; ok {
			return fmt.Errorf("task ID %q appears in both %s and tasks_completed", id, previous)
		}
		seen[id] = "tasks_completed"
	}
	for _, id := range remaining {
		if previous, ok := seen[id]; ok {
			return fmt.Errorf("task ID %q appears in both %s and tasks_remaining", id, previous)
		}
		seen[id] = "tasks_remaining"
	}
	return nil
}

// validateStageAgentTaskIDs checks the report against the IDs assigned to the
// agent for this wave. The owner map is deliberately supplied by the scheduler
// so a report cannot claim a task belonging to another implementation agent.
func validateStageAgentTaskIDs(assigned map[string]string, report stageAgentReport) error {
	if assigned == nil {
		return fmt.Errorf("assigned task set is required")
	}
	assignment := make([]string, 0, len(assigned))
	for id, owner := range assigned {
		if owner == report.Agent {
			assignment = append(assignment, id)
		}
	}
	sort.Strings(assignment)
	if err := validateImplementationAssignmentUnion(report.TasksCompleted, report.TasksRemaining, assignment); err != nil {
		return err
	}
	if report.Status == "complete" && len(report.TasksRemaining) != 0 {
		return fmt.Errorf("complete reports must have an empty tasks_remaining array")
	}
	return nil
}

func reportNonEmptyString(fields map[string]json.RawMessage, name string) bool {
	raw, ok := fields[name]
	if !ok {
		return false
	}
	var value string
	return json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != ""
}

func reportAgentLabel(raw string) string {
	line := strings.TrimSpace(raw)
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	line = strings.TrimSpace(strings.TrimSuffix(line, ":"))
	if line == "" {
		return "agent"
	}
	return line
}

func pendingTaskProgress(before, after []string) bool {
	if len(before) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(after))
	for _, task := range after {
		set[strings.TrimSpace(task)] = struct{}{}
	}
	for _, task := range before {
		if _, ok := set[strings.TrimSpace(task)]; !ok {
			return true
		}
	}
	return false
}

func (m model) expectedImplementationAgents() []string {
	if len(m.stageHandoffExpectedAgents) > 0 {
		return append([]string(nil), m.stageHandoffExpectedAgents...)
	}
	if m.svc != nil {
		return m.implementationAgentsFromScope()
	}
	return nil
}

func (m model) assignedImplementationTasks(agent string) map[string]string {
	assigned := make(map[string]string)
	if m.stageHandoffAssignments == nil {
		return assigned
	}
	byAgent := m.stageHandoffAssignments[m.stageHandoffWave]
	for owner, tasks := range byAgent {
		if owner != agent {
			continue
		}
		for _, task := range tasks {
			if strings.TrimSpace(task.ID) != "" {
				assigned[task.ID] = owner
			}
		}
	}
	return assigned
}

func (m model) evaluateStageHandoff(stage, outputs string) stageHandoffDecision {
	decision := stageHandoffDecision{}
	if stage != stageImplementation {
		if strings.TrimSpace(outputs) == "" {
			decision.Reason = "stage agents returned no report"
			return decision
		}
		decision.Complete = true
		return decision
	}
	decision.Checklist = m.implementationChecklist()
	if reason := strings.TrimSpace(m.stageHandoffPreparationError); reason != "" {
		decision.Reason = reason
		return decision
	}
	chunks := m.stageHandoffOutputs
	if len(chunks) == 0 {
		// Implementation reports are always associated with the Execute that
		// produced them. Never infer an agent from an untagged fallback string.
		decision.Reason = "no stage-agent reports were captured"
		return decision
	}
	expected := m.expectedImplementationAgents()
	if len(expected) == 0 {
		decision.Reason = "no expected implementation agents were recorded"
		return decision
	}
	expectedSet := make(map[string]struct{}, len(expected))
	for _, agent := range expected {
		expectedSet[agent] = struct{}{}
	}
	seen := make(map[string]struct{}, len(chunks))
	allValid := true
	allComplete := true
	allPassed := true
	allAccepted := true
	allRemainingEmpty := true
	anyBlocked := false
	completedGateFailure := false
	completedIDs := make([]string, 0)
	completedSet := make(map[string]struct{})
	for _, chunk := range chunks {
		agent := reportAgentLabel(chunk)
		if _, ok := expectedSet[agent]; !ok {
			decision.Reports = append(decision.Reports, stageAgentReport{
				Agent:           agent,
				Raw:             chunk,
				ValidationError: "report agent is not expected for this wave",
			})
			allValid = false
			continue
		}
		if _, duplicate := seen[agent]; duplicate {
			decision.Reports = append(decision.Reports, stageAgentReport{
				Agent:           agent,
				Raw:             chunk,
				ValidationError: "duplicate report for agent",
			})
			allValid = false
			continue
		}
		seen[agent] = struct{}{}
		report := parseStageAgentReport(chunk, agent)
		if report.Valid {
			if err := validateStageAgentTaskIDs(m.assignedImplementationTasks(agent), report); err != nil {
				report.Valid = false
				report.ValidationError = "task assignment: " + err.Error()
			}
		}
		decision.Reports = append(decision.Reports, report)
		if !report.Valid {
			allValid = false
		}
		if report.Status != "complete" {
			allComplete = false
		}
		if report.Status == "blocked" {
			anyBlocked = true
		}
		if !report.TestsPassed {
			allPassed = false
		}
		if !report.AcceptanceGates {
			allAccepted = false
		}
		if len(report.TasksRemaining) > 0 {
			allRemainingEmpty = false
		}
		if len(report.TasksCompleted) > 0 {
			if !report.TestsPassed || !report.AcceptanceGates {
				completedGateFailure = true
			}
			for _, id := range report.TasksCompleted {
				if _, exists := completedSet[id]; exists {
					allValid = false
					continue
				}
				completedSet[id] = struct{}{}
				completedIDs = append(completedIDs, id)
			}
		}
	}
	if len(decision.Reports) != len(expected) {
		allValid = false
	}
	for _, agent := range expected {
		if _, ok := seen[agent]; !ok {
			allValid = false
		}
	}
	if len(decision.Reports) == 0 {
		decision.Reason = "no stage-agent reports were captured"
		return decision
	}
	if !decision.Checklist.Linked {
		decision.Reason = "active cycle has no linked OpenSpec tasks.md"
		return decision
	}
	if decision.Checklist.Linked && !decision.Checklist.Ready {
		decision.Reason = "linked OpenSpec tasks.md could not be read"
		return decision
	}
	if err := validateImplementationChecklistPlan(decision.Checklist, expected); err != nil {
		decision.Reason = err.Error()
		return decision
	}
	if completedGateFailure {
		decision.Reason = "reports with completed tasks must pass tests and all canonical acceptance gates"
		return decision
	}
	findingProgress := false
	if allValid && len(completedIDs) > 0 {
		taskIDs := implementationOpenSpecTaskIDs(completedIDs)
		findingIDs := implementationFindingIDs(completedIDs)
		if len(findingIDs) > 0 {
			if m.svc == nil {
				decision.Reason = "cycle service unavailable"
				return decision
			}
			if reason := m.reproGateFailure(findingIDs); reason != "" {
				decision.Reason = reason
				return decision
			}
		}
		if len(taskIDs) > 0 {
			if err := markImplementationTasksComplete(decision.Checklist.Path, taskIDs); err != nil {
				decision.Reason = "could not mark completed implementation tasks: " + err.Error()
				return decision
			}
		}
		if len(findingIDs) > 0 {
			if m.svc == nil {
				decision.Reason = "cycle service unavailable"
				return decision
			}
			cycleRow, err := m.svc.SessionCycle()
			if err != nil || cycleRow == nil {
				decision.Reason = "active cycle is unavailable"
				return decision
			}
			if err := markImplementationFindingsDone(m.svc.Store, cycleRow.ID, findingIDs); err != nil {
				decision.Reason = "could not mark completed implementation findings: " + err.Error()
				return decision
			}
			findingProgress = true
		}
		decision.Checklist = m.implementationChecklist()
		if !decision.Checklist.Linked || !decision.Checklist.Ready {
			decision.Reason = "could not reread linked OpenSpec tasks.md after marking completed tasks"
			return decision
		}
		if err := validateImplementationChecklistPlan(decision.Checklist, expected); err != nil {
			decision.Reason = strings.TrimSuffix(err.Error(), ".") + " after marking completed tasks"
			return decision
		}
	}
	zeroPending := len(decision.Checklist.Pending) == 0
	if allValid && allComplete && allPassed && allAccepted && allRemainingEmpty && zeroPending {
		decision.Complete = true
		return decision
	}
	checklistProgress := decision.Checklist.Linked && decision.Checklist.Ready && pendingTaskProgress(m.stageHandoffPendingBefore, decision.Checklist.Pending)
	progress := checklistProgress || findingProgress
	if allValid && !anyBlocked && progress {
		if m.stageHandoffWave < maxImplementationHandoffWaves {
			decision.PartialProgress = true
			decision.Reason = "implementation wave made checklist progress; starting a fresh wave"
			return decision
		}
		decision.Reason = fmt.Sprintf("implementation wave limit reached (%d)", maxImplementationHandoffWaves)
		decision.EscalateReason = "implementation_wave_limit"
		return decision
	}
	switch {
	case !allValid:
		decision.Reason = "one or more stage-agent reports are invalid or missing required gate fields"
	case anyBlocked:
		decision.Reason = "one or more stage agents reported blocked"
	case !allPassed:
		decision.Reason = "tests_passed is false in one or more reports"
	case !allAccepted:
		decision.Reason = "acceptance_gates is false in one or more reports"
	case !allRemainingEmpty:
		decision.Reason = "tasks_remaining is non-empty in one or more reports"
	case !zeroPending:
		decision.Reason = fmt.Sprintf("%d unchecked OpenSpec task(s) remain", len(decision.Checklist.Pending))
	default:
		decision.Reason = "stage-agent reports are incomplete"
	}
	if !decision.Complete && !decision.PartialProgress {
		decision.ChatCopy = formatImplementationHandoffDiagnostics(decision.Reports, m)
		if decision.ChatCopy != "" {
			decision.OmitAgentOutput = true
		}
	}
	return decision
}

func validateImplementationChecklistPlan(checklist implementationChecklist, activeAgents []string) error {
	plan := partitionImplementationTasks(checklist.Raw, activeAgents)
	if plan.Valid {
		return nil
	}
	if len(plan.Errors) == 0 {
		return fmt.Errorf("implementation task ownership plan is invalid")
	}
	return fmt.Errorf("implementation task ownership plan is invalid: %s", strings.Join(plan.Errors, "; "))
}

func (m model) resumeOrchestratorAfterStageHandoff() (model, tea.Cmd) {
	stage := strings.TrimSpace(m.stageHandoffStage)
	outputs := strings.TrimSpace(strings.Join(m.stageHandoffOutputs, "\n\n"))
	if stage == stageImplementation && !m.stageHandoffReproChecked && !m.stageHandoffReproRunning {
		// Re-running a locked repro can take minutes. Do it in a command so the
		// TUI stays responsive, then re-enter this function with the verdict.
		if ids := claimedFindingIDs(m.stageHandoffOutputs); len(ids) > 0 {
			return m.startReproGate(stage, ids)
		}
		m.stageHandoffReproChecked = true
	}
	var decision stageHandoffDecision
	if isValidationHandoffStage(stage) {
		decision = m.evaluateValidationStageHandoff(stage, outputs)
	} else {
		decision = m.evaluateStageHandoff(stage, outputs)
	}
	if stage == stageImplementation && decision.PartialProgress {
		// Keep the stage Running and launch a fresh wave in the same iteration.
		// The orchestrator is not resumed between productive waves, so a partial
		// report cannot accidentally trigger the Stage Close Sequence.
		if m.svc != nil {
			// A wave does not pass through StartStage, so this is the only place
			// where a long Implementation stage can be checked against its
			// timeout before burning another wave.
			if err := m.svc.EscalateIfStageTimedOut(stageImplementation); err != nil {
				slog.Error("implementation timeout check failed", "error", redact.Error(err))
			}
			if st, err := m.svc.ActiveStage(); err != nil || st.Status != store.StageRunning {
				decision.PartialProgress = false
				decision.Reason = "implementation stage is no longer Running; intervention required"
			} else {
				m.stageHandoffLive = false
				m.stageHandoffOutputs = nil
				m.stageHandoffDoneKey = ""
				m.stageHandoffPendingBefore = append([]string(nil), decision.Checklist.Pending...)
				m = m.clearReproGateState()
				m = m.restoreOrchestratorSession()
				m.runtimeCommandName = ""
				return m.startStageAgentSessions(m.namedStageAgents(st))
			}
		}
	}
	m.stageHandoffLive = false
	m.stageHandoffStage = ""
	m.stageHandoffOutputs = nil
	m.stageHandoffPendingBefore = nil
	m.stageHandoffWave = 0
	m.stageHandoffAssignments = nil
	m.stageHandoffExpectedAgents = nil
	m.stageHandoffPreparationError = ""
	m = m.clearReproGateState()
	if decision.SchedulerHandledBlocked {
		m = m.stopStageBudgetMonitor()
		progress := m.validationProgress
		progress.active = false
		progress.waiting = false
		progress.phase = "report"
		m.validationProgress = progress
	} else {
		m = m.stopValidationProgress()
	}
	m.stageHandoffInterventionRequired = !decision.Complete && !decision.SchedulerHandledFailure
	if decision.SchedulerHandledFailure {
		// Loop-back already moved Implementation to Waiting. This is a fresh
		// wave, not a completion-gate retry of the validation stage.
		m.stageHandoffDoneKey = ""
	}
	// A report Hero could not decode used to park the stage Running and wait for
	// a human to notice. The cycle is the priority: re-dispatch the stage agent
	// with the diagnostic, and only hand control back once the retries are spent.
	// Scoped to validation stages: Implementation already has its own wave loop
	// and completion gate, so a second retry mechanism there would double-handle
	// the same failure.
	if reportRejected := isValidationHandoffStage(stage) && decision.OmitAgentOutput &&
		strings.Contains(decision.ChatCopy, "report rejected"); reportRejected {
		if retry, ok := m.nextReportRetry(stage); ok {
			m.transcript = append(m.transcript, convMessage{
				role:    convRoleWarning,
				content: strings.TrimRight(decision.ChatCopy, "\n") + fmt.Sprintf("\n→ Re-running %s (attempt %d of %d)", validationStageTitle(stage), retry, maxReportRetries),
			})
			return m.retryStageAgentsAfterRejectedReport(stage, decision.Reason)
		}
		decision.EscalateReason = "report_contract"
	}
	m = m.restoreOrchestratorSession()
	m = m.withRuntimeAgent(agentOrchestration)
	m.runtimeCommandName = "start"
	m = m.applyAgentRuntimePair(agentOrchestration, "")
	m = m.bindSessionToRuntimeHarness()
	if m.svc != nil {
		if s, err := m.svc.ActiveRunStage(); err == nil {
			m.conversationStage = s
		}
	}
	if reason := strings.TrimSpace(decision.EscalateReason); reason != "" && m.svc != nil {
		if err := m.svc.EscalateStage(stage, reason); err != nil {
			slog.Error("stage escalation failed", "stage", stage, "reason", reason, "error", redact.Error(err))
		} else {
			decision.ChatCopy = strings.TrimSpace(decision.ChatCopy+"\n\n⚠ "+stageTitleForChat(stage)+" escalated · "+decision.Reason+
				"\n→ Run /hero-continue to grant more iterations, /hero-add-todo to defer a blocking finding, or /hero-cancel / /hero-finish.") + "\n"
		}
	}
	canClose := decision.Complete
	if stage != stageImplementation && outputs != "" && !isValidationHandoffStage(stage) {
		// Planning and other text handoffs remain compatible with legacy Output Formats.
		canClose = true
	}
	if decision.SchedulerHandledFailure || decision.JudgeSDDAmbiguity {
		canClose = false
	}
	if warnCopy := formatContractWarnings(stage, decision.ContractWarnings); warnCopy != "" {
		m.transcript = append(m.transcript, convMessage{role: convRoleWarning, content: warnCopy})
	}
	for _, warning := range decision.ScreenshotWarnings {
		m.transcript = append(m.transcript, convMessage{role: convRoleWarning, content: "⚠ " + warning})
	}
	if decision.NormalizedAgentOutput != "" {
		outputs = decision.NormalizedAgentOutput
	}
	if decision.SchedulerHandledBlocked {
		copy := strings.TrimSpace(decision.ChatCopy)
		if copy == "" {
			copy = validationBlockedFallbackCopy(stage, m.userPreferredLanguage())
		}
		m.transcript = append(m.transcript, convMessage{role: convRoleSystem, content: copy})
		m.bumpTranscriptLayout()
		m.runtimeCommandName = ""
		m = m.setStatusWarning("validation blocked", firstStatusLine(copy))
		if m.svc != nil {
			if status, err := m.svc.Status(); err == nil {
				m.status = status
			} else {
				slog.Error("tui refresh blocked validation status failed", "stage", stage, "error", redact.Error(err))
			}
		}
		// A blocked operational prerequisite is a scheduler stop. In particular,
		// do not resume the orchestration model or dispatch automatic
		// Implementation repair; only an explicit /hero-continue may recheck.
		return m, m.refreshCmd()
	}
	// The stage is ending (closing, or closed-failed with loop-back), so this
	// is the stage-close moment the metrics summary belongs to. The TUI prints
	// it because only the TUI sees real harness usage; the orchestration agent
	// no longer estimates or reports metrics.
	if canClose || decision.SchedulerHandledFailure {
		if summary := strings.TrimRight(m.stageMetricsSummary(stage), "\n"); summary != "" {
			m.transcript = append(m.transcript, convMessage{role: convRoleSystem, content: summary})
		}
	}
	var prompt string
	if copy := strings.TrimSpace(decision.ChatCopy); copy != "" {
		prompt = copy + "\n\n"
	}
	switch {
	case decision.SchedulerHandledFailure:
		prompt += tuiHeroStartContinueAfterSchedulerFailedValidationPreamble(stage)
	case decision.JudgeSDDAmbiguity:
		prompt += tuiHeroStartContinueAfterIncompleteStagePreamble(stage, "judge reported sdd_ambiguity; use /hero-back or /hero-approve")
	case canClose:
		prompt += tuiHeroStartContinueAfterStagePreamble(stage)
	case decision.OmitAgentOutput && strings.Contains(decision.ChatCopy, "report rejected"):
		prompt += tuiHeroStartContinueAfterValidationReportRejectedPreamble(stage)
	default:
		prompt += tuiHeroStartContinueAfterIncompleteStagePreamble(stage, decision.Reason)
	}
	if outputs != "" && !decision.OmitAgentOutput {
		prompt += "Stage agent output:\n\n" + outputs + "\n"
	}
	label := handoffResumeLabel(stage, canClose, decision)
	m = m.beginSystemConversationExecute(label, prompt)
	return m, m.conversationExecuteCmds()
}

func isValidationHandoffStage(stage string) bool {
	switch strings.TrimSpace(stage) {
	case stageQA, stageJudge, stageBrowserUI, stageQAEndToEnd:
		return true
	default:
		return false
	}
}

func handoffResumeLabel(stage string, canClose bool, decision stageHandoffDecision) string {
	if line := firstLine(decision.ChatCopy); line != "" {
		return line
	}
	if canClose {
		return "→ " + stage + " closed"
	}
	return "→ " + stage + " gate pending"
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}

func formatImplementationAssignmentEmptyCloseChat() string {
	return "✓ Implementation assignment empty after scheduler recheck\n→ Closing Implementation without another wave\n"
}

func (m model) evaluateValidationStageHandoff(stage, outputs string) stageHandoffDecision {
	decision := stageHandoffDecision{}
	if m.svc == nil {
		decision.Reason = "cycle service unavailable"
		return decision
	}
	rawBody := validationReportBody(outputs, m.stageHandoffOutputs)
	if rawBody == "" {
		decision.Reason = "no stage-agent reports were captured"
		return decision
	}
	ctx, err := m.svc.ValidationDecodeContextForStage(stage)
	if err != nil {
		decision.Reason = err.Error()
		return decision
	}
	reportJSON, derr := reports.ReportJSONFromText(rawBody)
	if derr != nil {
		decision.ChatCopy = formatValidationReportRejected(stage, derr)
		decision.OmitAgentOutput = true
		decision.Reason = derr.Error()
		return decision
	}
	ingested := m.ingestStageScreenshots(stage, reportJSON, ctx)
	decision.ScreenshotWarnings = ingested.Warnings
	if string(ingested.ReportJSON) != string(reportJSON) {
		reportJSON = ingested.ReportJSON
		decision.NormalizedAgentOutput = string(reportJSON)
		// The original decoder context snapshots the ready screenshot set before
		// ingestion. Refresh it so promoted immutable manifests are eligible in
		// the final typed-report validation.
		ctx, err = m.svc.ValidationDecodeContextForStage(stage)
		if err != nil {
			decision.Reason = err.Error()
			return decision
		}
	}
	status, judgeAmbiguity, contractWarnings, decodeErr := decodeValidationReportStatus(stage, reportJSON, ctx)
	if decodeErr != nil {
		decision.ChatCopy = formatValidationReportRejected(stage, decodeErr)
		decision.OmitAgentOutput = true
		decision.Reason = decodeErr.Error()
		return decision
	}
	decision.ContractWarnings = contractWarnings
	if judgeAmbiguity {
		decision.JudgeSDDAmbiguity = true
		if status == reports.ValidationStatusPassed {
			decision.Complete = true
		}
		decision.Reason = "judge reported sdd_ambiguity"
		return decision
	}
	if status == reports.ValidationStatusPassed {
		decision.Complete = true
		return decision
	}
	if status == reports.ValidationStatusBlocked {
		_, closeErr := m.svc.CloseStageBlockedWithReport(stage, reportJSON, "")
		if closeErr != nil {
			var rve *cycle.ReportValidationError
			if errors.As(closeErr, &rve) && rve != nil && rve.Diagnostic != nil {
				decision.ChatCopy = formatValidationReportRejected(stage, rve.Diagnostic)
				decision.OmitAgentOutput = true
				decision.Reason = rve.Diagnostic.Error()
				return decision
			}
			decision.Reason = redact.Error(closeErr)
			if errors.Is(closeErr, store.ErrStageBudgetExpired) {
				decision.ChatCopy = validationBudgetExpiredCopy(stage, m.userPreferredLanguage())
				decision.OmitAgentOutput = true
			}
			return decision
		}
		decision.SchedulerHandledBlocked = true
		decision.OmitAgentOutput = true
		decision.ChatCopy = m.formatBlockedValidationChat(stage, reportJSON, ctx)
		decision.Reason = "operational browser prerequisite requires explicit correction and /hero-continue"
		return decision
	}
	slog.Info("validation stage failed; invoking atomic close handoff", "stage", stage)
	out, closeErr := m.svc.CloseStageFailedWithFindings(stage, reportJSON, "")
	if closeErr != nil {
		var rve *cycle.ReportValidationError
		if errors.As(closeErr, &rve) && rve != nil && rve.Diagnostic != nil {
			decision.ChatCopy = formatValidationReportRejected(stage, rve.Diagnostic)
			decision.OmitAgentOutput = true
			decision.Reason = rve.Diagnostic.Error()
			return decision
		}
		decision.Reason = closeErr.Error()
		return decision
	}
	decision.SchedulerHandledFailure = true
	decision.ChatCopy = m.formatValidationFailedHandoffChat(stage, out.FindingIDs)
	decision.OmitAgentOutput = true
	decision.Reason = strings.TrimSpace(out.Summary)
	return decision
}

func validationReportBody(outputs string, chunks []string) string {
	raw := strings.TrimSpace(outputs)
	if raw == "" && len(chunks) > 0 {
		raw = strings.TrimSpace(chunks[0])
	}
	if raw == "" {
		return ""
	}
	if idx := strings.IndexByte(raw, '\n'); idx >= 0 {
		return strings.TrimSpace(raw[idx+1:])
	}
	return raw
}

func (m model) userPreferredLanguage() string {
	if m.svc == nil || strings.TrimSpace(m.svc.ProjectDir) == "" {
		return "EN"
	}
	doc, err := workflowconfig.LoadCurrentDocument(m.svc.ProjectDir)
	if err != nil || doc == nil {
		return "EN"
	}
	language := strings.TrimSpace(doc.Config.WorkflowConfig.UserPreferredLanguage)
	if language == "" {
		return "EN"
	}
	return language
}

func (m model) formatBlockedValidationChat(stage string, reportJSON []byte, ctx reports.DecodeContext) string {
	var preparation reports.Preparation
	var blockers []reports.Blocker
	var decodeErr error
	if stage == stageBrowserUI {
		var report *reports.BrowserUIReport
		report, diag := reports.DecodeBrowserUI(reportJSON, ctx)
		if diag != nil {
			decodeErr = diag
		} else {
			preparation = *report.Preparation
			blockers = report.Blockers
		}
	} else {
		var report *reports.QAEndToEndReport
		report, diag := reports.DecodeQAEndToEnd(reportJSON, ctx)
		if diag != nil {
			decodeErr = diag
		} else {
			preparation = *report.Preparation
			blockers = report.Blockers
		}
	}
	if decodeErr != nil || len(blockers) == 0 {
		return validationBlockedFallbackCopy(stage, m.userPreferredLanguage())
	}

	var coverageIDs, profileIDs []string
	for _, blocker := range blockers {
		for _, id := range blocker.AffectedCoverageIDs {
			if safeValidationID(id) {
				coverageIDs = appendUniqueValidationID(coverageIDs, id)
			}
		}
		for _, id := range blocker.AffectedProfileIDs {
			if safeValidationID(id) {
				profileIDs = appendUniqueValidationID(profileIDs, id)
			}
		}
	}
	sort.Strings(coverageIDs)
	sort.Strings(profileIDs)
	primaryReason := strings.TrimSpace(blockers[0].Reason)
	reason, action := safeBlockedReasonAction(primaryReason, preparation.Method, profileIDs, m.userPreferredLanguage())
	remaining := "unlimited"
	if m.svc != nil && m.svc.Engine != nil {
		if cycle, err := m.svc.SessionCycle(); err == nil && cycle != nil {
			if budget, err := m.svc.Engine.StageBudget(cycle.ID, stage); err == nil && budget.Limit > 0 {
				remaining = budget.RemainingAt(m.validationProgressNow()).Truncate(time.Second).String()
			}
		}
	}
	coverage := safeIDList(coverageIDs, ", ")
	profiles := safeIDList(profileIDs, ", ")
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(m.userPreferredLanguage())), "PT") {
		if coverage == "—" {
			coverage = "nenhuma ID de cobertura identificada"
		}
		if profiles == "—" {
			profiles = "nenhum perfil identificado"
		}
		return fmt.Sprintf("→ %s bloqueado: %s\n→ Não validadas: %s. Perfis afetados: %s.\n→ %s\n→ Depois execute /hero-continue. Restam %s de execução ativa.\n",
			validationStageTitle(stage), reason, coverage, profiles, action, remaining)
	}
	if coverage == "—" {
		coverage = "no coverage IDs identified"
	}
	if profiles == "—" {
		profiles = "no profiles identified"
	}
	return fmt.Sprintf("→ %s blocked: %s\n→ Not validated: %s. Affected profiles: %s.\n→ %s\n→ Run /hero-continue. Active budget remaining: %s.\n",
		validationStageTitle(stage), reason, coverage, profiles, action, remaining)
}

func safeBlockedReasonAction(reason, method string, profiles []string, language string) (string, string) {
	methodLabel := safeBrowserMethod(method)
	pt := strings.HasPrefix(strings.ToUpper(strings.TrimSpace(language)), "PT")
	profile := safeIDList(profiles, ", ")
	switch strings.TrimSpace(reason) {
	case "tool_unavailable", "tool_missing", "tool_version", "tool_version_incompatible", "method_unavailable":
		if pt {
			return "o método de navegador selecionado (" + methodLabel + ") está indisponível ou abaixo da versão exigida", "Instale ou configure o método selecionado conforme as instruções do projeto; o Hero não instala ferramentas."
		}
		return "the selected browser method (" + methodLabel + ") is unavailable or below the required version", "Install or configure the selected method using the project setup instructions; Hero does not install tools."
	case "credentials_unavailable", "selected_account_unavailable", "account_unusable", "invalid_account", "test_access_disabled":
		if pt {
			if strings.Contains(profile, "admin") {
				return "falta o usuário administrator com perfil admin", "Habilite Config → Test users e configure localmente a conta selecionada no .env.hero da raiz; valores de credenciais não são exibidos."
			}
			return "a conta de teste selecionada não está disponível para o perfil " + profile, "Habilite Config → Test users e configure localmente a conta selecionada no .env.hero da raiz; valores de credenciais não são exibidos."
		}
		return "the selected test account is unavailable for profile " + profile, "Enable Config → Test users and configure the selected account locally in the project-root .env.hero; credential values are not shown."
	case "unsupported_authentication":
		if pt {
			return "o fluxo de login exige autenticação interativa não compatível (MFA/CAPTCHA/SSO)", "Use uma conta de teste aprovada com fluxo de login compatível e execute /hero-continue."
		}
		return "the login flow requires unsupported interactive authentication (MFA/CAPTCHA/SSO)", "Use an approved test account with a supported login flow, then run /hero-continue."
	case "browser_permission_unavailable":
		if pt {
			return "a permissão necessária do navegador não foi concedida", "Conceda a permissão no TUI para a sessão do estágio e execute /hero-continue."
		}
		return "the required browser permission is unavailable", "Grant the permission in the stage TUI session, then run /hero-continue."
	case "fixtures_unavailable":
		if pt {
			return "os fixtures locais planejados não estão disponíveis", "Disponibilize os fixtures locais declarados durante Planning e execute /hero-continue."
		}
		return "the planned local fixtures are unavailable", "Provide the declared local fixtures from Planning, then run /hero-continue."
	default:
		if pt {
			return "um pré-requisito do navegador não pôde ser verificado", "Revise o browser plan, os usuários de teste e a configuração do método selecionado; depois execute /hero-continue."
		}
		return "a browser prerequisite could not be verified", "Review the browser plan, test users and selected-method configuration, then run /hero-continue."
	}
}

func safeBrowserMethod(method string) string {
	switch strings.TrimSpace(method) {
	case "playwright_test":
		return "Playwright Test"
	case "cli_skill":
		return "Playwright CLI with skill"
	case "cli":
		return "Playwright CLI"
	case "mcp":
		return "MCP"
	case "http":
		return "HTTP"
	default:
		return "planned browser method"
	}
}

func safeIDList(values []string, separator string) string {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		if safeValidationID(value) {
			ids = appendUniqueValidationID(ids, value)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return "—"
	}
	return strings.Join(ids, separator)
}

func appendUniqueValidationID(ids []string, value string) []string {
	for _, existing := range ids {
		if existing == value {
			return ids
		}
	}
	return append(ids, value)
}

func validationBlockedFallbackCopy(stage, language string) string {
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(language)), "PT") {
		return fmt.Sprintf("→ %s bloqueado: um pré-requisito do navegador não pôde ser verificado.\n→ Revise a configuração local e execute /hero-continue; a configuração atual será revalidada.\n", validationStageTitle(stage))
	}
	return fmt.Sprintf("→ %s blocked: a browser prerequisite could not be verified.\n→ Review local configuration and run /hero-continue; current configuration will be rechecked.\n", validationStageTitle(stage))
}

func validationBudgetExpiredCopy(stage, language string) string {
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(language)), "PT") {
		return fmt.Sprintf("→ %s: o orçamento de execução expirou; evidências parciais foram preservadas. Aumente o limite explicitamente antes de executar /hero-continue.\n", validationStageTitle(stage))
	}
	return fmt.Sprintf("→ %s: active execution budget expired; partial evidence was preserved. Explicitly increase the limit before /hero-continue.\n", validationStageTitle(stage))
}

func decodeValidationReportStatus(stage string, reportJSON []byte, ctx reports.DecodeContext) (string, bool, []reports.ReportWarning, *reports.DiagnosticError) {
	switch stage {
	case stageQA:
		r, err := reports.DecodeQA(reportJSON, ctx)
		if err != nil {
			return "", false, nil, err
		}
		return r.Status, false, r.ContractWarnings, nil
	case stageJudge:
		r, err := reports.DecodeJudge(reportJSON, ctx)
		if err != nil {
			return "", false, nil, err
		}
		return r.Status, r.SDDAmbiguity, r.ContractWarnings, nil
	case stageBrowserUI:
		r, err := reports.DecodeBrowserUI(reportJSON, ctx)
		if err != nil {
			return "", false, nil, err
		}
		return r.Status, false, r.ContractWarnings, nil
	case stageQAEndToEnd:
		r, err := reports.DecodeQAEndToEnd(reportJSON, ctx)
		if err != nil {
			return "", false, nil, err
		}
		return r.Status, false, r.ContractWarnings, nil
	default:
		return "", false, nil, reportsDiagInvalidStage(stage)
	}
}

func reportsDiagInvalidStage(stage string) *reports.DiagnosticError {
	return &reports.DiagnosticError{
		Code:  reports.CodeInvalidEnum,
		Field: "stage",
		Value: stage,
		Rule:  "stage cannot emit validation findings",
	}
}

func validationStageTitle(stage string) string {
	switch stage {
	case stageQA:
		return "QA"
	case stageJudge:
		return "Judge"
	case stageBrowserUI:
		return "Browser UI"
	case stageQAEndToEnd:
		return "QA End-to-End"
	default:
		return stage
	}
}

func formatValidationReportRejected(stage string, d *reports.DiagnosticError) string {
	if d == nil {
		return "✗ " + validationStageTitle(stage) + " report rejected · invalid_json\n  No finding, stage close, or loop-back was persisted.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "✗ %s report rejected · %s\n", validationStageTitle(stage), d.Code)
	if field := strings.TrimSpace(d.Field); field != "" {
		if value := strings.TrimSpace(d.Value); value != "" {
			fmt.Fprintf(&b, "  %s: %s\n", field, value)
		} else {
			fmt.Fprintf(&b, "  %s\n", field)
		}
		if strings.TrimSpace(d.Rule) != "" {
			fmt.Fprintf(&b, "  %s\n", strings.TrimSpace(d.Rule))
		}
	} else if strings.TrimSpace(d.Rule) != "" {
		fmt.Fprintf(&b, "  %s\n", strings.TrimSpace(d.Rule))
	}
	b.WriteString("  No finding, stage close, or loop-back was persisted.\n")
	return b.String()
}

func (m model) formatValidationFailedHandoffChat(stage string, findingIDs []string) string {
	title := validationStageTitle(stage)
	if len(findingIDs) == 0 {
		return fmt.Sprintf("✗ %s failed\n→ Loop-back %s → Implementation\n", title, title)
	}
	cycleRow, err := m.svc.SessionCycle()
	if err != nil || cycleRow == nil {
		slog.Error("validation handoff chat missing cycle", "error", redact.Error(err))
		return fmt.Sprintf("✗ %s failed · %d findings\n→ Loop-back %s → Implementation\n", title, len(findingIDs), title)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "✗ %s failed · %d findings\n", title, len(findingIDs))
	for _, id := range findingIDs {
		f, err := m.svc.Store.GetFinding(cycleRow.ID, id)
		if err != nil {
			slog.Error("validation handoff chat finding lookup failed", "finding_id", id, "error", redact.Error(err))
			fmt.Fprintf(&b, "  %s\n", id)
			continue
		}
		state := strings.TrimSpace(f.Status)
		round := ""
		if state == store.FindingStatusReopened {
			round = fmt.Sprintf(" (round %d)", f.Round)
		}
		fmt.Fprintf(&b, "  %s · %s · %s%s\n", f.ID, agentShortLabel(f.Owner), state, round)
		file := strings.TrimSpace(f.File)
		if file != "" {
			fmt.Fprintf(&b, "  %s · %s\n", file, strings.TrimSpace(f.Issue))
		} else {
			fmt.Fprintf(&b, "  %s\n", strings.TrimSpace(f.Issue))
		}
	}
	fmt.Fprintf(&b, "\n→ Loop-back %s → Implementation\n", title)
	fmt.Fprintf(&b, "→ Assignment will include %s\n", strings.Join(findingIDs, ", "))
	return b.String()
}

func formatImplementationHandoffDiagnostics(reports []stageAgentReport, m model) string {
	var b strings.Builder
	for _, report := range reports {
		if report.Valid {
			continue
		}
		code := implementationReportDiagnosticCode(report)
		fmt.Fprintf(&b, "✗ implementation report rejected · %s\n", code)
		if msg := strings.TrimSpace(report.ValidationError); msg != "" {
			fmt.Fprintf(&b, "  %s\n", msg)
		}
		assign := assignedIDsForAgent(m, report.Agent)
		if len(assign) > 0 {
			fmt.Fprintf(&b, "  assigned IDs: %s\n", strings.Join(assign, ", "))
		}
		b.WriteString("  No finding, stage close, or loop-back was persisted.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func implementationReportDiagnosticCode(report stageAgentReport) string {
	msg := strings.TrimSpace(report.ValidationError)
	if msg == "" {
		return "invalid_report"
	}
	if code, _, ok := strings.Cut(msg, ":"); ok {
		code = strings.ToLower(strings.TrimSpace(code))
		if isImplementationDiagnosticCode(code) {
			return code
		}
	}
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "no json report"):
		return string(reports.CodeInvalidJSON)
	case strings.Contains(lower, "assignment_union_mismatch"):
		return string(reports.CodeAssignmentUnionMismatch)
	case strings.Contains(lower, "not assigned") || strings.Contains(lower, "unassigned"):
		return string(reports.CodeUnassignedID)
	case strings.Contains(lower, "nonempty") || strings.Contains(lower, "verification wave assigned no"):
		return string(reports.CodeNonemptyEmptyAssignment)
	case strings.Contains(lower, "duplicate"):
		return string(reports.CodeDuplicateID)
	case strings.Contains(lower, "both"):
		return string(reports.CodeOverlappingArrays)
	default:
		return "invalid_report"
	}
}

func isImplementationDiagnosticCode(code string) bool {
	switch reports.Code(code) {
	case reports.CodeInvalidJSON,
		reports.CodeUnknownField,
		reports.CodeMissingField,
		reports.CodeInvalidEnum,
		reports.CodeInvalidOwner,
		reports.CodeUnknownReopenID,
		reports.CodeDuplicateID,
		reports.CodeOverlappingArrays,
		reports.CodeAssignmentUnionMismatch,
		reports.CodeUnassignedID,
		reports.CodeFalseAcceptanceGate,
		reports.CodeNonemptyEmptyAssignment,
		reports.CodeNoActionableFinding:
		return true
	default:
		return false
	}
}

func assignedIDsForAgent(m model, agent string) []string {
	ids := make([]string, 0)
	for id := range m.assignedImplementationTasks(agent) {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// stageTitleForChat renders a stage name for user-facing chat copy.
func stageTitleForChat(stage string) string {
	if title := validationStageTitle(strings.TrimSpace(stage)); title != strings.TrimSpace(stage) {
		return title
	}
	if stage == stageImplementation {
		return "Implementation"
	}
	return strings.TrimSpace(stage)
}

// findingsOverRoundCap lists findings that already travelled the validation
// loop more times than maxFindingRounds allows.
func findingsOverRoundCap(findings []store.Finding) []store.Finding {
	var out []store.Finding
	for _, f := range findings {
		if f.Round > maxFindingRounds {
			out = append(out, f)
		}
	}
	return out
}

// escalateStalledFindings stops the Implementation loop for findings that keep
// coming back, and hands the decision to the user instead of spending another
// wave on them.
func (m model) escalateStalledFindings(st store.Stage, stalled []store.Finding) (model, tea.Cmd) {
	ids := make([]string, 0, len(stalled))
	for _, f := range stalled {
		ids = append(ids, fmt.Sprintf("%s (round %d)", f.ID, f.Round))
	}
	slog.Info("implementation escalated on finding round cap", "findings", ids, "cap", maxFindingRounds)
	if m.svc != nil {
		if err := m.svc.EscalateStage(stageImplementation, "finding_round_limit"); err != nil {
			slog.Error("finding round cap escalation failed", "error", redact.Error(err))
			return m.returnStageAgentPreparationFailure(stageImplementation, err.Error())
		}
		if cur, err := m.svc.ActiveStage(); err == nil {
			st = cur
		}
	}
	m.stageHandoffLive = false
	m.stageHandoffInterventionRequired = true
	extra := fmt.Sprintf("%d finding(s) reached the %d-round limit: %s",
		len(stalled), maxFindingRounds, strings.Join(ids, ", "))
	return m.emitSchedulerCTA(schedulerCTAContinue, st, extra)
}

// formatContractWarnings renders what Hero tolerated while decoding a report.
// The stage already proceeded — this exists so a drifting agent prompt is
// visible instead of silently accumulating noise in every report.
func formatContractWarnings(stage string, warnings []reports.ReportWarning) string {
	if len(warnings) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "⚠ %s report accepted with %d contract warning(s)\n", validationStageTitle(stage), len(warnings))
	for _, w := range warnings {
		switch w.Code {
		case reports.CodeFieldRenamed:
			fmt.Fprintf(&b, "  %s → %s (normalized)\n", w.Field, w.Value)
		default:
			fmt.Fprintf(&b, "  %s ignored (not in the report contract)\n", w.Field)
		}
	}
	b.WriteString("  The cycle continued; fix the agent prompt so future reports stay on contract.\n")
	return strings.TrimRight(b.String(), "\n")
}

// maxReportRetries bounds automatic re-dispatch after a rejected report. Two
// attempts cover the common case (a model that drifts once and corrects when
// shown the diagnostic) without burning the stage budget on a model that
// cannot produce the contract at all.
const maxReportRetries = 2

// nextReportRetry claims one retry for this stage, returning the attempt number.
// The counter is scoped to a stage so moving on always starts fresh.
func (m *model) nextReportRetry(stage string) (int, bool) {
	stage = strings.TrimSpace(stage)
	if m.stageHandoffRetryStage != stage {
		m.stageHandoffRetryStage = stage
		m.stageHandoffReportRetries = 0
	}
	if m.stageHandoffReportRetries >= maxReportRetries {
		return 0, false
	}
	m.stageHandoffReportRetries++
	return m.stageHandoffReportRetries, true
}

// retryStageAgentsAfterRejectedReport re-runs the stage agent with the decoder
// diagnostic attached, without touching persisted state: a rejected report
// still applies nothing.
func (m model) retryStageAgentsAfterRejectedReport(stage, diagnostic string) (model, tea.Cmd) {
	if m.svc == nil {
		return m, nil
	}
	st, err := m.svc.ActiveStage()
	if err != nil {
		// Without a resolved stage there is nothing safe to re-dispatch into.
		slog.Error("report retry could not resolve active stage", "stage", stage, "error", redact.Error(err))
		m.stageHandoffInterventionRequired = true
		return m, nil
	}
	if st.Status != store.StageRunning {
		// The stage moved on underneath us; hand back to the orchestrator
		// rather than dispatching an agent into a stage that is not Running.
		slog.Warn("report retry skipped; stage no longer running", "stage", stage, "status", st.Status)
		return m.progressCTAForStage(st)
	}
	m.stageHandoffLive = false
	m.stageHandoffOutputs = nil
	m.stageHandoffDoneKey = ""
	m.stageHandoffReportFeedback = strings.TrimSpace(diagnostic)
	m = m.clearReproGateState()
	m = m.restoreOrchestratorSession()
	m.runtimeCommandName = ""
	return m.startStageAgentSessions(m.namedStageAgents(st))
}
