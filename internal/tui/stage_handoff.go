package tui

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

type convExecute struct {
	ID              string
	AgentName       string
	HarnessID       string
	Model           string
	Prompt          string
	Attachments     []harness.Attachment
	AttachmentChips []tuiAttachment
	StageName       string
	Wave            int
	UsageGeneration int64
	SessionID       string
	AgentMsgIndex   int
	Origin          string // telegram:<address> when the turn came from Telegram
	Freechat        bool
	OccupancyKey    string
	relay           *conversationStreamRelay
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
	if !m.orchestrationLive || m.svc == nil {
		return m, nil
	}
	agent := strings.TrimSpace(m.runtimeAgentName)
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
	if agent == agentOrchestration && !m.researchLive && !m.stageHandoffLive {
		if m.stageHandoffInterventionRequired && len(m.waitingNamedStageAgents()) == 0 {
			return m, nil
		}
		if agents := m.runningStageAgents(); len(agents) > 0 {
			if key := m.runningStageHandoffKey(); key != "" && key == m.stageHandoffDoneKey {
				return m, nil
			}
			return m.startStageAgentSessions(agents)
		}
		if agents := m.waitingNamedStageAgents(); len(agents) > 0 {
			if err := m.startWaitingActiveStage(); err != nil {
				slog.Error("tui start waiting stage after orchestrator failed", "error", err)
				m.convError = err.Error()
				return m, nil
			}
			return m.startStageAgentSessions(agents)
		}
	}
	if m.stageHandoffLive && agent != agentOrchestration && len(m.executes) == 0 {
		return m.resumeOrchestratorAfterStageHandoff()
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
		return m, nil
	}
	// Escalated is a terminal control state until /hero-continue grants
	// iterations and the engine moves the stage back to Waiting. Never let a
	// stale TUI handoff launch an agent directly from Escalated.
	if st.Status != store.StageRunning {
		return m, nil
	}
	stage := strings.TrimSpace(st.Name)
	if stage == "" {
		m.convError = "resolve active stage: active stage has an empty name"
		m.stageHandoffInterventionRequired = true
		return m, nil
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
		runAgents, assignments, expectedAgents, reason = implementationStageDispatch(checklist, agents)
		if reason != "" {
			m.stageHandoffPendingBefore = append([]string(nil), checklist.Pending...)
			return m.returnStageAgentPreparationFailure(stage, reason)
		}
	}
	if len(runAgents) == 0 {
		return m.returnStageAgentPreparationFailure(stage, "no active implementation agent owns a pending task")
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
			slog.Error("tui stage agent prompt read failed", "agent", agent, "error", err)
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
	m.stageHandoffStage = stage
	m.stageHandoffOutputs = nil
	m.stageHandoffPreparationError = ""
	m.stageHandoffExpectedAgents = append([]string(nil), expectedAgents...)
	m.stageHandoffDoneKey = m.runningStageHandoffKey()
	m.harnessSessionID = ""
	m.harnessSessionHarnessID = ""
	m.conversationStage = stage
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
				slog.Warn("tui persist stage agent assignment failed", "stage", stage, "agent", agent, "wave", m.stageHandoffWave, "error", err)
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

func implementationStageDispatch(checklist implementationChecklist, activeAgents []string) ([]string, map[string][]implementationTaskBlock, []string, string) {
	if !checklist.Linked {
		return nil, nil, nil, "active cycle has no linked OpenSpec tasks.md"
	}
	if !checklist.Ready {
		return nil, nil, nil, "linked OpenSpec tasks.md could not be read"
	}
	plan := partitionImplementationTasks(checklist.Raw, activeAgents)
	if !plan.Valid {
		if len(plan.Errors) == 0 {
			return nil, nil, nil, "implementation task ownership plan is invalid"
		}
		return nil, nil, nil, "implementation task ownership plan is invalid: " + strings.Join(plan.Errors, "; ")
	}
	assignments := make(map[string][]implementationTaskBlock, len(activeAgents))
	if len(plan.Tasks) == 0 {
		for _, agent := range activeAgents {
			assignments[agent] = []implementationTaskBlock{}
		}
		return append([]string(nil), activeAgents...), assignments, append([]string(nil), activeAgents...), ""
	}
	runAgents := make([]string, 0, len(activeAgents))
	for _, agent := range activeAgents {
		tasks := append([]implementationTaskBlock(nil), plan.ByAgent[agent]...)
		if len(tasks) == 0 {
			continue
		}
		assignments[agent] = tasks
		runAgents = append(runAgents, agent)
	}
	if len(runAgents) == 0 {
		return nil, nil, nil, "implementation task ownership plan has pending tasks but no active owner"
	}
	return runAgents, assignments, append([]string(nil), runAgents...), ""
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
		b.WriteString("No unchecked task checkboxes remain at wave start. Verify the acceptance gates and return a complete JSON report only if they pass.\n")
		return b.String()
	}
	b.WriteString("Tasks that must be handled in this wave:\n\n")
	for _, task := range tasks {
		b.WriteString(strings.TrimRight(task.Block, "\n"))
		b.WriteString("\n\n")
	}
	b.WriteString("Do not edit task checkboxes. After a structurally valid report, the scheduler marks the completed task IDs in tasks.md in one write. Do not report complete while any assigned task remains.\n")
	return b.String()
}

func parseStageAgentReport(raw, agent string) stageAgentReport {
	report := stageAgentReport{Agent: strings.TrimSpace(agent), Raw: raw}
	obj, ok := extractStageReportObject(raw)
	if !ok {
		report.ValidationError = "no JSON report object found"
		return report
	}
	report.Stage, ok = reportStringField(obj, "stage")
	if !ok || report.Stage != stageImplementation {
		report.ValidationError = "stage must be implementation"
		return report
	}
	reportedAgent, ok := reportStringField(obj, "agent")
	if !ok || strings.TrimSpace(reportedAgent) == "" {
		report.ValidationError = "agent is required"
		return report
	}
	reportedAgent = strings.TrimSpace(reportedAgent)
	if reportedAgent != report.Agent {
		report.ValidationError = fmt.Sprintf("agent %q does not match expected agent %q", reportedAgent, report.Agent)
		return report
	}
	report.Agent = reportedAgent
	statusRaw, ok := obj["status"]
	if !ok {
		report.ValidationError = "status is required"
		return report
	}
	if err := json.Unmarshal(statusRaw, &report.Status); err != nil {
		report.ValidationError = "status must be a string"
		return report
	}
	report.Status = strings.ToLower(strings.TrimSpace(report.Status))
	if report.Status != "complete" && report.Status != "partial" && report.Status != "blocked" {
		report.ValidationError = "status must be complete, partial, or blocked"
		return report
	}
	report.TestsPassed, ok = reportBoolField(obj, "tests_passed")
	if !ok {
		report.ValidationError = "tests_passed must be a boolean"
		return report
	}
	report.AcceptanceGates, report.AcceptanceValues, ok = reportAcceptanceField(obj)
	if !ok {
		report.ValidationError = "acceptance_gates must contain the canonical boolean gates"
		return report
	}
	var err error
	report.TasksCompleted, err = requiredReportTaskIDs(obj, "tasks_completed")
	if err != nil {
		report.ValidationError = err.Error()
		return report
	}
	report.TasksRemaining, err = requiredReportTaskIDs(obj, "tasks_remaining")
	if err != nil {
		report.ValidationError = err.Error()
		return report
	}
	if err := validateStageAgentTaskIDLists(report.TasksCompleted, report.TasksRemaining); err != nil {
		report.ValidationError = err.Error()
		return report
	}
	if report.Status == "complete" && len(report.TasksRemaining) != 0 {
		report.ValidationError = "complete reports must have an empty tasks_remaining array"
		return report
	}
	if report.Status == "complete" && !report.AcceptanceGates {
		report.ValidationError = "complete reports require all canonical acceptance gates to be true"
		return report
	}
	if report.Status != "complete" {
		if !reportNonEmptyString(obj, "blocker") || !reportNonEmptyString(obj, "next_action") {
			report.ValidationError = "partial and blocked reports require non-empty blocker and next_action"
			return report
		}
	}
	report.Valid = true
	return report
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

func reportBoolField(fields map[string]json.RawMessage, names ...string) (bool, bool) {
	for _, name := range names {
		raw, ok := fields[name]
		if !ok {
			continue
		}
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return false, false
		}
		return value, true
	}
	return false, false
}

func reportStringField(fields map[string]json.RawMessage, name string) (string, bool) {
	raw, ok := fields[name]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func reportAcceptanceField(fields map[string]json.RawMessage) (bool, map[string]bool, bool) {
	const (
		completedTasksVerified = "completed_tasks_verified"
		taskOwnershipRespected = "task_ownership_respected"
		requiredTestsPassed    = "required_tests_passed"
	)
	raw, ok := fields["acceptance_gates"]
	if !ok {
		return false, nil, false
	}
	var rawValues map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawValues); err != nil || rawValues == nil {
		return false, nil, false
	}
	values := make(map[string]bool, len(rawValues))
	for key, rawValue := range rawValues {
		if strings.TrimSpace(string(rawValue)) == "null" {
			return false, nil, false
		}
		var value bool
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return false, nil, false
		}
		values[key] = value
	}
	for _, key := range []string{completedTasksVerified, taskOwnershipRespected, requiredTestsPassed} {
		if _, exists := values[key]; !exists {
			return false, nil, false
		}
	}
	allPassed := true
	for _, value := range values {
		if !value {
			allPassed = false
			break
		}
	}
	return allPassed, values, true
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

func requiredReportTaskIDs(fields map[string]json.RawMessage, name string) ([]string, error) {
	values, err := requiredReportStringSlice(fields, name)
	if err != nil {
		return nil, err
	}
	for i, value := range values {
		id, err := normalizeStageAgentTaskID(value)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		values[i] = id
	}
	return values, nil
}

func normalizeStageAgentTaskID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if strings.HasPrefix(id, "[") && strings.HasSuffix(id, "]") {
		id = strings.TrimSuffix(strings.TrimPrefix(id, "["), "]")
	}
	if !strings.HasPrefix(id, "task-") || strings.TrimSpace(strings.TrimPrefix(id, "task-")) == "" || strings.ContainsAny(id, "[] \t\r\n") {
		return "", fmt.Errorf("task ID %q is invalid", raw)
	}
	return id, nil
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
	if err := validateStageAgentTaskIDLists(report.TasksCompleted, report.TasksRemaining); err != nil {
		return err
	}
	reported := make(map[string]struct{}, len(report.TasksCompleted)+len(report.TasksRemaining))
	for _, id := range append(append([]string(nil), report.TasksCompleted...), report.TasksRemaining...) {
		owner, ok := assigned[id]
		if !ok {
			return fmt.Errorf("task ID %q is not assigned to %q", id, report.Agent)
		}
		if owner != report.Agent {
			return fmt.Errorf("task ID %q is assigned to %q, not %q", id, owner, report.Agent)
		}
		reported[id] = struct{}{}
	}
	for id, owner := range assigned {
		if owner != report.Agent {
			continue
		}
		if _, ok := reported[id]; !ok {
			return fmt.Errorf("assigned task ID %q is omitted from the report", id)
		}
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
	if allValid && len(completedIDs) > 0 {
		if err := markImplementationTasksComplete(decision.Checklist.Path, completedIDs); err != nil {
			decision.Reason = "could not mark completed implementation tasks: " + err.Error()
			return decision
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
	progress := decision.Checklist.Linked && decision.Checklist.Ready && pendingTaskProgress(m.stageHandoffPendingBefore, decision.Checklist.Pending)
	if allValid && !anyBlocked && progress {
		if m.stageHandoffWave < maxImplementationHandoffWaves {
			decision.PartialProgress = true
			decision.Reason = "implementation wave made checklist progress; starting a fresh wave"
			return decision
		}
		decision.Reason = fmt.Sprintf("implementation wave limit reached (%d)", maxImplementationHandoffWaves)
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
	decision := m.evaluateStageHandoff(stage, outputs)
	if stage == stageImplementation && decision.PartialProgress {
		// Keep the stage Running and launch a fresh wave in the same iteration.
		// The orchestrator is not resumed between productive waves, so a partial
		// report cannot accidentally trigger the Stage Close Sequence.
		if m.svc != nil {
			if st, err := m.svc.ActiveStage(); err != nil || st.Status != store.StageRunning {
				decision.PartialProgress = false
				decision.Reason = "implementation stage is no longer Running; intervention required"
			} else {
				m.stageHandoffLive = false
				m.stageHandoffOutputs = nil
				m.stageHandoffDoneKey = ""
				m.stageHandoffPendingBefore = append([]string(nil), decision.Checklist.Pending...)
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
	m.stageHandoffInterventionRequired = !decision.Complete
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
	canClose := decision.Complete
	if stage != stageImplementation && outputs != "" {
		// Existing non-Implementation stage handoffs remain compatible with
		// their text Output Formats. Empty output is never a close signal.
		canClose = true
	}
	var prompt string
	if canClose {
		prompt = tuiHeroStartContinueAfterStagePreamble(stage)
	} else {
		prompt = tuiHeroStartContinueAfterIncompleteStagePreamble(stage, decision.Reason)
	}
	if outputs != "" {
		prompt += "Stage agent output:\n\n" + outputs + "\n"
	}
	label := "→ " + stage + " closed"
	if !canClose {
		label = "→ " + stage + " gate pending"
	}
	m = m.beginSystemConversationExecute(label, prompt)
	return m, m.conversationExecuteCmds()
}
