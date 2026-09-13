package tui

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

type conversationContextSyncPurpose int

const (
	syncContextGeneric conversationContextSyncPurpose = iota
	syncContextEnter
	syncContextAfterReset
	syncContextFollowUp
	syncContextBootstrap
)

type conversationContextSnapshot struct {
	ConversationContextErr error
	Stage                  string
	SessionID              string
	OrchestrationSID       string
	OrchestrationHID       string
	StageBindingHID        string
	StageBindingSID        string
	StageBindingErr        error
	StageHarnessID         string
}

type conversationContextSyncDoneMsg struct {
	seq      uint64
	purpose  conversationContextSyncPurpose
	snapshot conversationContextSnapshot
}

type pendingChatFollowUpState struct {
	text     string
	telegram bool
}

type heroNewPrepareDoneMsg struct {
	err error
}

func fetchConversationContextSnapshot(svc *cycle.Service) conversationContextSnapshot {
	var snap conversationContextSnapshot
	if svc == nil {
		return snap
	}
	snap.Stage, snap.SessionID, snap.ConversationContextErr = svc.ConversationContext()
	snap.OrchestrationSID, snap.OrchestrationHID, _ = svc.OrchestrationSession()
	stage := strings.TrimSpace(snap.Stage)
	if stage != "" {
		snap.StageBindingHID, snap.StageBindingSID, snap.StageBindingErr = svc.StageSessionBinding(stage)
		if snap.StageBindingErr != nil {
			snap.StageHarnessID, _ = svc.StageHarnessID(stage)
		}
	}
	return snap
}

func clearChatSessionStore(svc *cycle.Service) {
	if svc == nil {
		return
	}
	if err := svc.ClearOrchestrationSession(); err != nil {
		slog.Debug("tui clear orchestration session failed", "error", redact.Error(err))
	}
	stage, _, err := svc.ConversationContext()
	if err == nil && strings.TrimSpace(stage) != "" {
		if err := svc.SetStageSessionBinding(stage, "", ""); err != nil {
			slog.Debug("tui clear harness session failed", "error", redact.Error(err))
		}
	}
}

func (m model) applyConversationContextSnapshot(snap conversationContextSnapshot) model {
	if m.svc == nil {
		return m
	}
	if snap.ConversationContextErr != nil {
		slog.Debug("tui conversation context unavailable", "error", redact.Error(snap.ConversationContextErr))
		m.conversationStage = ""
		return m
	}
	m.conversationStage = snap.Stage
	if m.researchLive {
		return m
	}
	if m.orchestrationLive || strings.EqualFold(strings.TrimSpace(m.runtimeAgentName), agentOrchestration) {
		m = m.applyOrchestrationSnapshot(snap.OrchestrationSID, snap.OrchestrationHID)
		if strings.TrimSpace(m.harnessSessionID) == "" {
			if sid := strings.TrimSpace(m.orchestrationSessionID); sid != "" {
				m.harnessSessionID = sid
				m.harnessSessionHarnessID = strings.TrimSpace(strings.ToLower(m.orchestrationSessionHarnessID))
			}
		}
		return m
	}
	live := strings.TrimSpace(m.harnessSessionID)
	if live != "" {
		return m
	}
	hid := strings.TrimSpace(strings.ToLower(snap.StageBindingHID))
	sid := strings.TrimSpace(snap.StageBindingSID)
	if snap.StageBindingErr != nil {
		sid = strings.TrimSpace(snap.SessionID)
		hid = ""
		hid = strings.TrimSpace(strings.ToLower(snap.StageHarnessID))
	}
	if sid == "" || hid == "" {
		return m
	}
	m.harnessSessionID = sid
	m.harnessSessionHarnessID = hid
	return m
}

func (m model) applyOrchestrationSnapshot(sessionID, harnessID string) model {
	sessionID = strings.TrimSpace(sessionID)
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if sessionID == "" || harnessID == "" {
		return m
	}
	m.orchestrationSessionID = sessionID
	m.orchestrationSessionHarnessID = harnessID
	return m
}

func (m model) conversationContextIOAsync() bool {
	return !m.testMode || m.testContextIODelay > 0
}

func (m model) syncConversationContextCmd(seq uint64, purpose conversationContextSyncPurpose) tea.Cmd {
	svc := m.svc
	delay := m.testContextIODelay
	return func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay)
		}
		return conversationContextSyncDoneMsg{
			seq:      seq,
			purpose:  purpose,
			snapshot: fetchConversationContextSnapshot(svc),
		}
	}
}

func (m model) resetChatSessionStoreCmd(seq uint64) tea.Cmd {
	svc := m.svc
	delay := m.testContextIODelay
	return func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay)
		}
		clearChatSessionStore(svc)
		return conversationContextSyncDoneMsg{
			seq:      seq,
			purpose:  syncContextAfterReset,
			snapshot: fetchConversationContextSnapshot(svc),
		}
	}
}

func (m model) nextConversationContextSync() (model, uint64) {
	m.conversationContextSyncSeq++
	return m, m.conversationContextSyncSeq
}

func (m model) handleConversationContextSyncDone(msg conversationContextSyncDoneMsg) (model, tea.Cmd) {
	if msg.seq != 0 && msg.seq < m.conversationContextSyncSeq {
		return m, nil
	}
	m = m.applyConversationContextSnapshot(msg.snapshot)

	if msg.purpose != syncContextFollowUp || m.pendingChatFollowUp == nil {
		return m, nil
	}
	pending := m.pendingChatFollowUp
	m.pendingChatFollowUp = nil

	if pending.text != "" && m.chatFollowUpControlSlash(pending.text) && m.researchLive {
		m = m.prepareOrchestratorFollowUp()
	} else if m.researchLive {
		m = m.prepareDiscoverFollowUp()
	} else if m.orchestrationLive && !m.researchLive && !m.stageHandoffLive {
		m = m.prepareOrchestratorFollowUp()
	}

	m = m.clearChatInput()
	m = m.beginConversationExecute(pending.text, controlSlashFollowUpPrompt(pending.text))
	cmds := m.conversationExecuteCmds()
	if pending.telegram {
		cmds = combineTimerCmds(cmds, m.telegramOutboundCmd(m.telegramAutoReportText(time.Now())))
	}
	return m, cmds
}

func (m model) heroNewPrepareCmd() tea.Cmd {
	svc := m.svc
	delay := m.testContextIODelay
	return func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay)
		}
		if svc == nil {
			return heroNewPrepareDoneMsg{err: errCycleServiceUnavailable()}
		}
		_, err := svc.PrepareWorkflowConfig()
		return heroNewPrepareDoneMsg{err: err}
	}
}

func errCycleServiceUnavailable() error {
	return fmt.Errorf("cycle service unavailable")
}
