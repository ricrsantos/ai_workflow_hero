package tui

import (
	"log/slog"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/ideadocs"
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

// bindSessionToRuntimeHarness records which harness owns harnessSessionID so
// mixed orchestrator/discover pairs (e.g. cursor + codex) can resume.
func (m model) bindSessionToRuntimeHarness() model {
	if h := strings.TrimSpace(strings.ToLower(m.runtimeHarnessID)); h != "" {
		m.harnessSessionHarnessID = h
		return m
	}
	if h := strings.TrimSpace(strings.ToLower(m.agentHarnessForName(m.runtimeAgentName))); h != "" {
		m.harnessSessionHarnessID = h
	}
	return m
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
	m.harnessSessionHarnessID = ""
	m.conversationStage = stageResearch
	m.runtimeCommandName = ""
	m = m.withRuntimeAgent(agentDiscover)
	m = m.applyAgentRuntimePair(agentDiscover, slug)
	prompt := tuiDiscoverResearchPreamble()
	if paths, err := ideadocs.ListActive(m.svc.ProjectDir); err != nil {
		slog.Debug("tui discover idea files scan failed", "error", err)
	} else if section := ideadocs.PromptSection(paths); section != "" {
		prompt += section + "\n\n"
	}
	prompt += strings.TrimSpace(agentBody) + "\n"
	label := "→ Research"
	if warned {
		label = "→ Research (model from fallback /model; set agents.discover_agent in workflow-config.yml)"
	}
	m = m.beginConversationExecute(label, prompt)
	return m, m.conversationExecuteCmds()
}

func (m model) resumeOrchestratorAfterResearch() (model, tea.Cmd) {
	orchID := strings.TrimSpace(m.orchestrationSessionID)
	m.researchLive = false
	m.harnessSessionID = orchID
	m = m.withRuntimeAgent(agentOrchestration)
	m.runtimeCommandName = "start"
	m = m.applyAgentRuntimePair(agentOrchestration, "")
	m = m.bindSessionToRuntimeHarness()
	if s, err := m.svc.ActiveRunStage(); err == nil {
		m.conversationStage = s
	}
	// Research may still be the last stored stage after close; do not use its
	// discover harness id to gate the orchestrator Cursor/OpenCode session.
	if strings.EqualFold(m.conversationStage, stageResearch) && m.researchStageClosedOrMovedOn() {
		m.conversationStage = ""
	}
	prompt := tuiHeroStartContinueAfterResearchPreamble() +
		"Research closed. Continue the cycle: start the next enabled stage then STOP so the TUI Executes named stage agents. Do not grill or re-run Research.\n"
	m = m.beginConversationExecute("→ Research closed", prompt)
	return m, m.conversationExecuteCmds()
}

func (m model) prepareDiscoverFollowUp() model {
	m = m.withRuntimeAgent(agentDiscover)
	m.runtimeCommandName = ""
	if sid := strings.TrimSpace(m.researchSessionID); sid != "" {
		m.harnessSessionID = sid
	}
	m = m.applyAgentRuntimePair(agentDiscover, "")
	m = m.bindSessionToRuntimeHarness()
	m.conversationStage = stageResearch
	return m
}

func (m model) prepareOrchestratorFollowUp() model {
	m.runtimeAgentName = agentOrchestration
	if sid := strings.TrimSpace(m.orchestrationSessionID); sid != "" {
		m.harnessSessionID = sid
	}
	m = m.applyAgentRuntimePair(agentOrchestration, "")
	m = m.bindSessionToRuntimeHarness()
	return m
}
