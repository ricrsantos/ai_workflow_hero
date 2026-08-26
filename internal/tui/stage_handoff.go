package tui

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

type convExecute struct {
	ID            string
	AgentName     string
	HarnessID     string
	SessionID     string
	AgentMsgIndex int
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
	if agent == agentOrchestration && !m.researchLive && !m.stageHandoffLive && m.researchStageInteractive() {
		return m.startDiscoverResearchSession()
	}
	if m.researchLive && (agent == agentDiscover || agent == "") && m.researchStageClosedOrMovedOn() {
		return m.resumeOrchestratorAfterResearch()
	}
	if m.researchLive && agent == agentOrchestration && m.researchStageInteractive() {
		m.harnessSessionID = m.researchSessionID
		m = m.withRuntimeAgent(agentDiscover)
		m = m.applyAgentRuntimePair(agentDiscover, "")
		m = m.bindSessionToRuntimeHarness()
	}
	if agent == agentOrchestration && !m.researchLive && !m.stageHandoffLive {
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
	case store.StageRunning, store.StageEscalated:
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
	case store.StageRunning, store.StageEscalated:
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
	case store.StageRunning, store.StageEscalated:
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
	if len(agents) == 0 {
		return m, nil
	}
	stage := ""
	if st, err := m.svc.ActiveStage(); err == nil {
		stage = st.Name
	}
	if sid := strings.TrimSpace(m.harnessSessionID); sid != "" {
		m.orchestrationSessionID = sid
	}
	m.stageHandoffLive = true
	m.stageHandoffStage = stage
	m.stageHandoffOutputs = nil
	m.stageHandoffDoneKey = m.runningStageHandoffKey()
	m.harnessSessionID = ""
	m.harnessSessionHarnessID = ""
	m.conversationStage = stage
	m.runtimeCommandName = ""

	var cmd tea.Cmd
	for i, agent := range agents {
		body, err := m.readStageAgentPrompt(agent)
		if err != nil {
			slog.Error("tui stage agent prompt read failed", "agent", agent, "error", err)
			m.convError = err.Error()
			m.stageHandoffLive = false
			return m, nil
		}
		slug, _ := m.stageAgentModelSlug(agent)
		m = m.withRuntimeAgent(agent)
		m = m.applyAgentRuntimePair(agent, slug)
		prompt := tuiStageAgentPreamble(stage, agent) + strings.TrimSpace(body) + "\n"
		label := "→ " + strings.TrimSpace(stage)
		if len(agents) > 1 {
			label = fmt.Sprintf("→ %s (%s)", strings.TrimSpace(stage), agentShortLabel(agent))
		}
		if i == 0 {
			m = m.beginConversationExecute(label, prompt)
			cmd = m.conversationExecuteCmds()
		} else {
			m = m.appendConversationExecute(label, prompt)
		}
	}
	return m, cmd
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
	case "opencode":
		return filepath.Join(".opencode", "agents", file)
	case "codex":
		return filepath.Join(".codex", "agents", file)
	default:
		return filepath.Join(cursoradapter.AgentsDir, file)
	}
}

func (m model) resumeOrchestratorAfterStageHandoff() (model, tea.Cmd) {
	stage := strings.TrimSpace(m.stageHandoffStage)
	outputs := strings.TrimSpace(strings.Join(m.stageHandoffOutputs, "\n\n"))
	orchID := strings.TrimSpace(m.orchestrationSessionID)
	m.stageHandoffLive = false
	m.stageHandoffStage = ""
	m.stageHandoffOutputs = nil
	m.harnessSessionID = orchID
	m = m.withRuntimeAgent(agentOrchestration)
	m.runtimeCommandName = "start"
	m = m.applyAgentRuntimePair(agentOrchestration, "")
	m = m.bindSessionToRuntimeHarness()
	if m.svc != nil {
		if s, err := m.svc.ActiveRunStage(); err == nil {
			m.conversationStage = s
		}
	}
	prompt := tuiHeroStartContinueAfterStagePreamble(stage)
	if outputs != "" {
		prompt += "Stage agent output:\n\n" + outputs + "\n"
	} else {
		prompt += "Stage agents finished with empty output. Close the stage using artifacts on disk and continue.\n"
	}
	m = m.beginConversationExecute("→ "+stage+" closed", prompt)
	return m, m.conversationExecuteCmds()
}
