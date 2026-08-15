package tui

import (
	"log/slog"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

const (
	agentOrchestration = "orchestration_agent"
	agentDiscover      = "discover_agent"
	stageResearch      = "research"
)

func (m model) researchStageInteractive() bool {
	if m.svc == nil {
		return false
	}
	st, err := m.svc.ActiveStage()
	if err != nil {
		return false
	}
	if st.Name != stageResearch {
		return false
	}
	switch st.Status {
	case store.StageRunning, store.StageEscalated:
		return true
	default:
		return false
	}
}

func (m model) researchStageClosedOrMovedOn() bool {
	if m.svc == nil {
		return true
	}
	st, err := m.svc.ActiveStage()
	if err != nil {
		return true
	}
	if st.Name != stageResearch {
		return true
	}
	switch st.Status {
	case store.StageCompleted, store.StagePendingApproval, store.StageSkipped, store.StageFailed:
		return true
	default:
		return false
	}
}

func (m model) discoverModelSlug() (slug string, warned bool) {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	slug, usedFallback, err := workflowconfig.AgentModelSlug(projectDir, agentDiscover)
	if err != nil || strings.TrimSpace(slug) == "" {
		return m.defaultHarnessModelSlug(), true
	}
	if usedFallback {
		return slug, true
	}
	return slug, false
}

func (m model) orchestratorModelSlug() (slug string, warned bool) {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	slug, usedFallback, err := workflowconfig.AgentModelSlug(projectDir, agentOrchestration)
	if err != nil || strings.TrimSpace(slug) == "" {
		return m.defaultHarnessModelSlug(), true
	}
	if usedFallback {
		return slug, true
	}
	return slug, false
}

func (m model) maybeHandoffAfterExecute() (model, tea.Cmd) {
	if !m.orchestrationLive || m.svc == nil {
		return m, nil
	}
	agent := strings.TrimSpace(m.runtimeAgentName)
	if agent == agentOrchestration && !m.researchLive && m.researchStageInteractive() {
		return m.startDiscoverResearchSession()
	}
	if m.researchLive && (agent == agentDiscover || agent == "") && m.researchStageClosedOrMovedOn() {
		return m.resumeOrchestratorAfterResearch()
	}
	if m.researchLive && agent == agentOrchestration && m.researchStageInteractive() {
		m.harnessSessionID = m.researchSessionID
		m.runtimeAgentName = agentDiscover
		if slug, _ := m.discoverModelSlug(); slug != "" {
			m.runtimeModelSlug = slug
		}
	}
	return m, nil
}

func (m model) startDiscoverResearchSession() (model, tea.Cmd) {
	if m.svc == nil {
		return m, nil
	}
	agentPath := filepath.Join(m.svc.ProjectDir, cursoradapter.AgentsDir, "discover_agent.md")
	agentBody, err := cursoradapter.ReadAgentPrompt(agentPath)
	if err != nil {
		slog.Error("tui discover_agent prompt read failed; keeping orchestrator session", "error", err)
		return m, nil
	}
	if sid := strings.TrimSpace(m.harnessSessionID); sid != "" {
		m.orchestrationSessionID = sid
	}
	if err := m.svc.SetStageHarnessSessionID(stageResearch, ""); err != nil {
		slog.Debug("tui clear research harness session failed", "error", err)
	}
	slug, warned := m.discoverModelSlug()
	m.researchLive = true
	m.researchSessionID = ""
	m.harnessSessionID = ""
	m.conversationStage = stageResearch
	m.runtimeCommandName = ""
	m.runtimeAgentName = agentDiscover
	m.runtimeModelSlug = slug
	prompt := tuiDiscoverResearchPreamble() + strings.TrimSpace(agentBody) + "\n"
	label := "→ Research"
	if warned {
		label = "→ Research (model from fallback /hero-model; set agents.discover_agent in workflow-config.yml)"
	}
	m = m.beginConversationExecute(label, prompt)
	return m, waitConvMsg(m.convStreamCh)
}

func (m model) resumeOrchestratorAfterResearch() (model, tea.Cmd) {
	orchID := strings.TrimSpace(m.orchestrationSessionID)
	m.researchLive = false
	m.harnessSessionID = orchID
	m.runtimeAgentName = agentOrchestration
	m.runtimeCommandName = "start"
	if slug, _ := m.orchestratorModelSlug(); slug != "" {
		m.runtimeModelSlug = slug
	}
	if s, err := m.svc.ActiveRunStage(); err == nil {
		m.conversationStage = s
	}
	prompt := tuiHeroStartContinueAfterResearchPreamble() +
		"Research closed. Continue the cycle: dispatch the next enabled stage via Task with Model Resolution from workflow-config.yml. Do not grill or re-run Research.\n"
	m = m.beginConversationExecute("→ Research closed", prompt)
	return m, waitConvMsg(m.convStreamCh)
}

func (m model) prepareDiscoverFollowUp() model {
	m.runtimeAgentName = agentDiscover
	m.runtimeCommandName = ""
	if sid := strings.TrimSpace(m.researchSessionID); sid != "" {
		m.harnessSessionID = sid
	}
	if slug, _ := m.discoverModelSlug(); slug != "" {
		m.runtimeModelSlug = slug
	}
	m.conversationStage = stageResearch
	return m
}

func (m model) prepareOrchestratorFollowUp() model {
	m.runtimeAgentName = agentOrchestration
	if sid := strings.TrimSpace(m.orchestrationSessionID); sid != "" {
		m.harnessSessionID = sid
	}
	if slug, _ := m.orchestratorModelSlug(); slug != "" {
		m.runtimeModelSlug = slug
	}
	return m
}
