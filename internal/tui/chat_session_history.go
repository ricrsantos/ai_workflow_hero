package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const (
	historicalContinuationBanner  = "Historical continuation · workflow stage remains completed."
	sessionLeaseHeartbeatInterval = 5 * time.Second
	sessionRecoverPollInterval    = 2 * time.Second

	sessionRecoverUnsupportedCopy        = "Cannot reconnect to the harness session. Press Ctrl+C to dismiss and continue read-only."
	sessionRecoverRunningUnsupportedCopy = "Harness is still running a prior turn. Live reconnect is not supported for this harness. Press Ctrl+C to dismiss and continue read-only."
	sessionRecoverStatusErrCopy          = "Could not verify the harness execution. Press Ctrl+C to dismiss or reopen the session to retry."
	sessionRecoverAttachCopy             = "Reconnecting to the harness stream…"
)

type sessionRecoverStatusMsg struct {
	sessionID   string
	nativeID    string
	harnessID   string
	running     bool
	attachLive  bool
	unsupported bool
	resolved    bool
	message     string
	err         error
}

type historyForkDoneMsg struct {
	sessionID  string
	err        error
	releaseIDs []string
}

func (m model) priorHeroLeaseSessionID() string {
	if id := strings.TrimSpace(m.heroLeasedSessionID); id != "" {
		return id
	}
	return strings.TrimSpace(m.heroChatSessionID)
}

func (m model) adapterForHarnessID(harnessID string) harness.HarnessAdapter {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	if harnessID == "" {
		return m.harnessAdapter()
	}
	if m.svc != nil && m.svc.Harness != nil {
		if strings.EqualFold(m.conversationHarnessTool(), harnessID) {
			return m.svc.Harness
		}
	}
	if m.svc != nil && m.svc.Registry != nil {
		if a, err := m.svc.Registry.Adapter(harnessID); err == nil && a != nil {
			return a
		}
	}
	return nil
}

func tryExactHarnessResume(ctx context.Context, adapter harness.HarnessAdapter, nativeSessionID string) error {
	nativeSessionID = strings.TrimSpace(nativeSessionID)
	if nativeSessionID == "" {
		return nil
	}
	if adapter == nil {
		return harness.NewExactResumeUnavailable(nativeSessionID, fmt.Errorf("harness adapter unavailable"))
	}
	if err := adapter.ResumeSession(ctx, nativeSessionID); err != nil {
		if errors.Is(err, harness.ErrExactResumeUnavailable) {
			return err
		}
		return harness.NewExactResumeUnavailable(nativeSessionID, err)
	}
	return nil
}

func parseSessionModelProps(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return nil
	}
	var props map[string]string
	if err := json.Unmarshal([]byte(raw), &props); err != nil {
		return nil
	}
	return harness.NormalizeProperties(props)
}

func (m model) applyHeroSessionBinding(sess store.Session) model {
	hid := strings.TrimSpace(sess.HarnessID)
	modelSlug := strings.TrimSpace(sess.Model)
	if hid != "" {
		m.chatHarnessID = hid
	}
	if modelSlug != "" {
		m.chatModelSlug = modelSlug
	}
	if props := parseSessionModelProps(sess.ModelPropertiesJSON); len(props) > 0 {
		m.freechatProps = props
	}
	nativeID := strings.TrimSpace(sess.NativeSessionID)
	nativeHarness := strings.TrimSpace(strings.ToLower(hid))

	switch sess.Kind {
	case store.SessionKindOrchestration:
		m.orchestrationLive = true
		m.researchLive = false
		m = m.withRuntimeAgent(agentOrchestration)
		m.runtimeHarnessID = hid
		m.runtimeModelSlug = modelSlug
		if nativeID != "" {
			m = m.persistOrchestrationSessionPair(nativeID, nativeHarness)
			m.harnessSessionID = nativeID
			m.harnessSessionHarnessID = nativeHarness
		}
	case store.SessionKindResearch:
		m.researchLive = true
		m.orchestrationLive = false
		m.conversationStage = stageResearch
		m = m.withRuntimeAgent(agentDiscover)
		m.runtimeHarnessID = hid
		m.runtimeModelSlug = modelSlug
		if nativeID != "" {
			m.researchSessionID = nativeID
			m.harnessSessionID = nativeID
			m.harnessSessionHarnessID = nativeHarness
			m = m.persistStageSession(stageResearch, nativeID, nativeHarness)
		}
	case store.SessionKindStageAgent:
		m.orchestrationLive = false
		m.researchLive = false
		m.stageHandoffLive = false
		stage := strings.TrimSpace(sess.StageName)
		agent := strings.TrimSpace(sess.AgentName)
		m.conversationStage = stage
		m = m.withRuntimeAgent(agent)
		m.runtimeHarnessID = hid
		m.runtimeModelSlug = modelSlug
		if nativeID != "" {
			m.harnessSessionID = nativeID
			m.harnessSessionHarnessID = nativeHarness
			m = m.persistStageSession(stage, nativeID, nativeHarness)
		}
	default:
		m.orchestrationLive = false
		m.researchLive = false
		m = m.clearRuntimePair()
		if nativeID != "" {
			m = m.persistFreechatSession(nativeID, nativeHarness)
		}
	}

	m.heroChatSessionTitle = strings.TrimSpace(sess.Title)
	m.heroSessionHistorical = false
	return m
}

func (m model) composerBlockedBySessionRecover() bool {
	return m.heroSessionRecoverBusy
}

func (m model) sessionLeaseHeartbeatTickCmd(sessionID string) tea.Cmd {
	sessionID = strings.TrimSpace(sessionID)
	owner := strings.TrimSpace(m.tuiOwnerID)
	svc := m.sessionService
	if sessionID == "" || owner == "" || svc == nil {
		return nil
	}
	return func() tea.Msg {
		if _, err := svc.HeartbeatLease(context.Background(), sessionID, owner); err != nil {
			slog.Error("tui session lease heartbeat failed", "error", redact.Error(err))
			return sessionLeaseLostMsg{sessionID: sessionID}
		}
		return nil
	}
}

func (m model) startHeroLeaseHeartbeat(sessionID string) tea.Cmd {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	return m.sessionLeaseHeartbeatTickCmd(sessionID)
}

func (m model) syncFinalizeSessionInterruptCmd() tea.Cmd {
	eventBatch := append([]store.AppendSessionEventInput(nil), m.sessionPersistQueue...)
	assetBatch := append([]sessionAssetPersistItem(nil), m.sessionAssetUpsertQueue...)
	bindingBatch := append([]sessionBindingPersistItem(nil), m.sessionBindingQueue...)
	nativeBatch := append([]sessionNativeBindPersistItem(nil), m.sessionNativeBindQueue...)
	sessionID := strings.TrimSpace(m.heroChatSessionID)
	svc := m.sessionService
	cycleSvc := m.svc
	heroKey := persistBatchSessionID(eventBatch, assetBatch, nativeBatch)
	if heroKey == "" {
		heroKey = sessionID
	}
	pipe := sessionPersistPipelineFor(heroKey)
	return pipe.submit(func() tea.Msg {
		msg := persistSessionSuffix(context.Background(), svc, cycleSvc, eventBatch, assetBatch, bindingBatch, nativeBatch)
		if errMsg, ok := msg.(sessionPersistErrMsg); ok {
			return errMsg
		}
		if svc != nil && sessionID != "" {
			if _, err := svc.MarkInterrupted(context.Background(), sessionID); err != nil {
				slog.Error("tui sync mark hero session interrupted failed", "error", redact.Error(err))
				return sessionPersistErrMsg{
					err:         sessionPersistenceError(err),
					sessionID:   sessionID,
					events:      eventBatch,
					assets:      assetBatch,
					bindings:    bindingBatch,
					nativeBinds: nativeBatch,
				}
			}
		}
		return sessionInterruptFinalizeDoneMsg{}
	})
}

func (m model) markHeroSessionInterruptedCmd() tea.Cmd {
	sessionID := strings.TrimSpace(m.heroChatSessionID)
	svc := m.sessionService
	if sessionID == "" || svc == nil {
		return nil
	}
	return func() tea.Msg {
		if _, err := svc.MarkInterrupted(context.Background(), sessionID); err != nil {
			slog.Error("tui mark hero session interrupted failed", "error", redact.Error(err))
		}
		return nil
	}
}

func (m model) sessionRecoverCheckCmd(sessionID, nativeID, harnessID string) tea.Cmd {
	svc := m.sessionService
	adapter := m.adapterForHarnessID(harnessID)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		nativeID = strings.TrimSpace(nativeID)
		harnessID = strings.TrimSpace(harnessID)
		sessionID = strings.TrimSpace(sessionID)
		if nativeID == "" || adapter == nil {
			return sessionRecoverStatusMsg{
				sessionID:   sessionID,
				nativeID:    nativeID,
				harnessID:   harnessID,
				unsupported: true,
				message:     sessionRecoverUnsupportedCopy,
			}
		}
		st, err := adapter.Status(ctx, nativeID)
		if err != nil {
			return sessionRecoverStatusMsg{
				sessionID: sessionID,
				nativeID:  nativeID,
				harnessID: harnessID,
				err:       err,
				message:   sessionRecoverStatusErrCopy,
			}
		}
		if st != nil && st.State == harness.StatusRunning {
			if attacher, ok := adapter.(harness.LiveStreamAttacher); ok && attacher.SupportsLiveStreamAttach() {
				return sessionRecoverStatusMsg{
					sessionID:  sessionID,
					nativeID:   nativeID,
					harnessID:  harnessID,
					attachLive: true,
					message:    sessionRecoverAttachCopy,
				}
			}
			return sessionRecoverStatusMsg{
				sessionID:   sessionID,
				nativeID:    nativeID,
				harnessID:   harnessID,
				unsupported: true,
				message:     sessionRecoverRunningUnsupportedCopy,
			}
		}
		if svc != nil && svc.Store != nil {
			if _, err := svc.Store.UpdateSessionLifecycle(sessionID, store.SessionLifecycleActive, nil); err != nil {
				return sessionRecoverStatusMsg{
					sessionID: sessionID,
					nativeID:  nativeID,
					harnessID: harnessID,
					err:       err,
					message:   sessionRecoverStatusErrCopy,
				}
			}
		}
		return sessionRecoverStatusMsg{
			sessionID: sessionID,
			nativeID:  nativeID,
			harnessID: harnessID,
			resolved:  true,
		}
	}
}

func (m model) sessionRecoverPollCmd(sessionID, nativeID, harnessID string) tea.Cmd {
	return tea.Tick(sessionRecoverPollInterval, func(time.Time) tea.Msg {
		return sessionRecoverPollTickMsg{
			sessionID: strings.TrimSpace(sessionID),
			nativeID:  strings.TrimSpace(nativeID),
			harnessID: strings.TrimSpace(harnessID),
		}
	})
}

func (m model) sessionRecoverCancelCmd() tea.Cmd {
	sessionID := strings.TrimSpace(m.heroChatSessionID)
	nativeID := strings.TrimSpace(m.heroSessionRecoverNativeID)
	harnessID := strings.TrimSpace(m.heroSessionRecoverHarnessID)
	adapter := m.adapterForHarnessID(harnessID)
	svc := m.sessionService
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if adapter != nil && nativeID != "" {
			if err := adapter.Cancel(ctx, nativeID); err != nil {
				slog.Error("tui session recover cancel failed", "error", redact.Error(err))
				return sessionRecoverStatusMsg{
					sessionID: sessionID,
					nativeID:  nativeID,
					harnessID: harnessID,
					err:       err,
					message:   sessionRecoverStatusErrCopy,
				}
			}
		}
		if svc != nil && svc.Store != nil && sessionID != "" {
			if _, err := svc.Store.UpdateSessionLifecycle(sessionID, store.SessionLifecycleActive, nil); err != nil {
				slog.Error("tui session recover dismiss lifecycle failed", "error", redact.Error(err))
				return sessionRecoverStatusMsg{
					sessionID: sessionID,
					nativeID:  nativeID,
					harnessID: harnessID,
					err:       err,
					message:   sessionRecoverStatusErrCopy,
				}
			}
		}
		return sessionRecoverDismissedMsg{}
	}
}

func appendReleaseID(ids []string, id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ids
	}
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func releaseLeaseCollect(svc *conversation.SessionService, releaseFn func(context.Context, string, string) error, ctx context.Context, sessionID, owner string, leftover []string) []string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return leftover
	}
	var err error
	if releaseFn != nil {
		err = releaseFn(ctx, sessionID, owner)
	} else if svc != nil {
		err = svc.ReleaseLease(ctx, sessionID, owner)
	}
	if err != nil {
		slog.Error("tui history lease cleanup failed", "error", redact.Error(err))
		return appendReleaseID(leftover, sessionID)
	}
	return leftover
}

func (m model) historyOpenCmd(id string, restore bool, priorLeaseID string) tea.Cmd {
	svc := m.sessionService
	var registry harnessmgr.Registry
	if m.svc != nil {
		registry = m.svc.Registry
	}
	owner := strings.TrimSpace(m.tuiOwnerID)
	releaseFn := m.leaseReleaseFn
	defaultAdapter := m.harnessAdapter()
	lookupAdapter := func(harnessID string) harness.HarnessAdapter {
		harnessID = strings.TrimSpace(harnessID)
		if harnessID == "" {
			return defaultAdapter
		}
		return m.adapterForHarnessID(harnessID)
	}
	return func() tea.Msg {
		if svc == nil {
			return historyOpenMsg{err: fmt.Errorf("sessions unavailable")}
		}
		ctx := context.Background()
		if restore {
			if _, err := svc.RestoreSession(ctx, id); err != nil {
				return historyOpenMsg{err: err}
			}
		}
		if _, err := svc.AcquireLease(ctx, id, owner); err != nil {
			if errors.Is(err, store.ErrSessionBusy) {
				return historyOpenMsg{busy: true}
			}
			return historyOpenMsg{err: err}
		}
		if epoch := m.leaseEpoch; epoch != nil {
			epoch.bump(id)
		}
		// Validate the target fully before releasing the prior lease so a failed
		// open cannot leave the TUI without ownership or with dual leases (find-qa-29).
		binding, err := svc.ResumeBinding(ctx, id)
		if err != nil {
			leftover := releaseLeaseCollect(svc, releaseFn, ctx, id, owner, nil)
			return historyOpenMsg{err: err, releaseIDs: leftover}
		}
		nativeID := strings.TrimSpace(binding.NativeSessionID)
		if nativeID != "" {
			adapter := lookupAdapter(binding.HarnessID)
			if err := tryExactHarnessResume(ctx, adapter, nativeID); err != nil {
				leftover := releaseLeaseCollect(svc, releaseFn, ctx, id, owner, nil)
				if errors.Is(err, harness.ErrExactResumeUnavailable) {
					return historyOpenMsg{
						needFork:    true,
						forkHarness: binding.HarnessID,
						forkModel:   binding.Model,
						forkSource:  id,
						releaseIDs:  leftover,
					}
				}
				return historyOpenMsg{err: err, releaseIDs: leftover}
			}
		}
		sess, err := svc.GetSession(ctx, id)
		if err != nil {
			leftover := releaseLeaseCollect(svc, releaseFn, ctx, id, owner, nil)
			return historyOpenMsg{err: err, releaseIDs: leftover}
		}
		if prior := strings.TrimSpace(priorLeaseID); prior != "" && prior != id {
			var priorErr error
			if releaseFn != nil {
				priorErr = releaseFn(ctx, prior, owner)
			} else {
				priorErr = svc.ReleaseLease(ctx, prior, owner)
			}
			if priorErr != nil {
				slog.Error("tui history prior lease release failed", "error", redact.Error(priorErr))
				leftover := appendReleaseID(nil, prior)
				leftover = releaseLeaseCollect(svc, releaseFn, ctx, id, owner, leftover)
				return historyOpenMsg{err: fmt.Errorf("release prior session lease: %w", priorErr), releaseIDs: leftover}
			}
		}
		bindRemoteHistoryReader(svc, registry, sess.HarnessID)
		offerImport := svc.ShouldOfferRemoteImport(sess, sessionHasLocalEvents(svc, id))
		return historyOpenMsg{
			sessionID:     id,
			restored:      restore,
			offerImport:   offerImport,
			importHarness: sess.HarnessID,
		}
	}
}

func (m model) historyForkCmd(sourceID string) tea.Cmd {
	sourceID = strings.TrimSpace(sourceID)
	svc := m.sessionService
	owner := strings.TrimSpace(m.tuiOwnerID)
	releaseFn := m.leaseReleaseFn
	priorLease := m.priorHeroLeaseSessionID()
	return func() tea.Msg {
		if svc == nil {
			return historyForkDoneMsg{err: fmt.Errorf("sessions unavailable")}
		}
		ctx := context.Background()
		sess, err := svc.ForkSession(ctx, conversation.ForkSessionInput{SourceSessionID: sourceID})
		if err != nil {
			return historyForkDoneMsg{err: err}
		}
		if _, err := svc.AcquireLease(ctx, sess.ID, owner); err != nil {
			return historyForkDoneMsg{err: err}
		}
		if epoch := m.leaseEpoch; epoch != nil {
			epoch.bump(sess.ID)
		}
		if prior := strings.TrimSpace(priorLease); prior != "" && prior != sess.ID {
			var priorErr error
			if releaseFn != nil {
				priorErr = releaseFn(ctx, prior, owner)
			} else {
				priorErr = svc.ReleaseLease(ctx, prior, owner)
			}
			if priorErr != nil {
				slog.Error("tui history fork prior lease release failed", "error", redact.Error(priorErr))
				leftover := appendReleaseID(nil, prior)
				leftover = releaseLeaseCollect(svc, releaseFn, ctx, sess.ID, owner, leftover)
				return historyForkDoneMsg{err: fmt.Errorf("release prior session lease: %w", priorErr), releaseIDs: leftover}
			}
		}
		return historyForkDoneMsg{sessionID: sess.ID}
	}
}

type sessionDeleteRetryResultMsg struct {
	retried int
	err     error
}

func (m model) retryIncompleteSessionDeletesCmd() tea.Cmd {
	svc := m.sessionService
	if svc == nil || svc.Store == nil {
		return nil
	}
	storeDB := svc.Store
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ops, err := storeDB.ListIncompleteSessionDeleteOps()
		if err != nil {
			slog.Error("tui list incomplete session delete ops failed", "error", redact.Error(err))
			return sessionDeleteRetryResultMsg{err: err}
		}
		var firstErr error
		retried := 0
		for _, op := range ops {
			if err := svc.ResumeIncompleteDelete(ctx, op); err != nil {
				slog.Error("tui incomplete session delete retry failed", "error", redact.Error(err))
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			retried++
		}
		return sessionDeleteRetryResultMsg{retried: retried, err: firstErr}
	}
}

func (m model) historyDeleteCompleteCmd(id string) tea.Cmd {
	svc := m.sessionService
	return func() tea.Msg {
		if svc == nil {
			return historyDeleteMsg{err: fmt.Errorf("sessions unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		op, paths, err := svc.DeleteLocalFirst(ctx, id)
		if err != nil {
			return historyDeleteMsg{err: err}
		}
		purgeErr := conversation.PurgeManagedAssetFiles(paths)
		warn := strings.TrimSpace(op.RemoteWarning)
		if warn == "" && strings.TrimSpace(op.NativeSessionID) != "" {
			if svc.Deleter == nil || !svc.Deleter.SupportsNativeDelete() {
				warn = "Provider-side data may remain."
			}
		}
		if purgeErr != nil {
			slog.Error("tui session delete purge incomplete; will retry")
			// Leave delete op incomplete for idempotent retry; surface failure.
			return historyDeleteMsg{err: purgeErr}
		}
		if err := svc.CompleteDeleteAfterPurge(ctx, op, warn); err != nil {
			return historyDeleteMsg{err: err}
		}
		return historyDeleteMsg{sessionID: id, remoteWarning: warn}
	}
}

func (m model) openHeroSessionFromHistory(sessionID string) (model, tea.Cmd) {
	sessionID = strings.TrimSpace(sessionID)
	m, resetCmd := m.resetChatSession()
	m.heroLeasedSessionID = sessionID
	m.sessionLeaseLost = false
	m.bumpLeaseReleaseEpoch(sessionID)
	m.heroLeaseNextHeartbeat = time.Now().Add(sessionLeaseHeartbeatInterval)
	m, enterCmd := m.enterConversation()
	pm := &m
	openCmd := tea.Batch(
		resetCmd,
		enterCmd,
		m.loadChatTranscriptCmd(sessionID),
		m.startHeroLeaseHeartbeat(sessionID),
		m.historyLoadCmd(),
	)
	var loopCmd tea.Cmd
	if loop := pm.ensureTimerLoop(); loop != nil {
		loopCmd = loop
	}
	m = *pm
	return m, combineTimerCmds(openCmd, loopCmd)
}

func (m model) emptyChatAfterCurrentSessionMutation() (model, tea.Cmd) {
	cur := strings.TrimSpace(m.heroChatSessionID)
	if cur == "" {
		return m, nil
	}
	prior := cur
	m, resetCmd := m.resetChatSession()
	m.trackPendingLeaseRelease(prior)
	m, enterCmd := m.enterConversation()
	m.screen = screenConversation
	return m, tea.Batch(resetCmd, enterCmd, m.releaseHeroChatLeaseCmd(prior))
}

func (m model) handleSessionRecoverStatus(msg sessionRecoverStatusMsg) (model, tea.Cmd) {
	m.heroSessionRecoverNativeID = strings.TrimSpace(msg.nativeID)
	m.heroSessionRecoverHarnessID = strings.TrimSpace(msg.harnessID)

	if msg.resolved {
		m.heroSessionRecoverBusy = false
		m.heroSessionRecoverNativeID = ""
		m.heroSessionRecoverHarnessID = ""
		m.chatInputFocused = true
		m.convError = ""
		return m, nil
	}

	m.heroSessionRecoverBusy = true
	m.chatInputFocused = false

	switch {
	case msg.unsupported:
		m.convError = strings.TrimSpace(msg.message)
		if m.convError == "" {
			m.convError = sessionRecoverUnsupportedCopy
		}
		return m, nil
	case msg.err != nil:
		m.convError = strings.TrimSpace(msg.message)
		if m.convError == "" {
			m.convError = msg.err.Error()
		}
		slog.Error("tui session recover status failed", "error", redact.Error(msg.err))
		return m, nil
	case msg.attachLive:
		m.convError = strings.TrimSpace(msg.message)
		if m.convError == "" {
			m.convError = sessionRecoverAttachCopy
		}
		m = m.prepareSessionRecoverLiveAttach(msg.sessionID, msg.nativeID, msg.harnessID)
		var cmds []tea.Cmd
		cmds = append(cmds, m.sessionRecoverLiveAttachCmd(msg.sessionID, msg.nativeID, msg.harnessID))
		if m.convStreamCh != nil {
			cmds = append(cmds, waitConvBatchMsg(m.convStreamCh))
		}
		return m, combineTimerCmds(cmds...)
	default:
		m.heroSessionRecoverBusy = false
		m.chatInputFocused = true
		return m, nil
	}
}

const sessionRecoverLiveAttachExecuteID = "session-recover-live"

func (m model) prepareSessionRecoverLiveAttach(sessionID, nativeID, harnessID string) model {
	sessionID = strings.TrimSpace(sessionID)
	nativeID = strings.TrimSpace(nativeID)
	harnessID = strings.TrimSpace(harnessID)
	for i := len(m.transcript) - 1; i >= 0; i-- {
		if m.transcript[i].role == convRoleAgent {
			m.agentMsgIndex = i
			m.transcript[i].interrupted = false
			break
		}
	}
	m.streaming = true
	m.streamInterrupted = false
	if m.convStreamCh == nil {
		m.convStreamCh = make(chan tea.Msg, 512)
	}
	if m.executes == nil {
		m.executes = make(map[string]convExecute)
	}
	m.executes[sessionRecoverLiveAttachExecuteID] = convExecute{
		ID:            sessionRecoverLiveAttachExecuteID,
		SessionID:     nativeID,
		HarnessID:     harnessID,
		AgentMsgIndex: m.agentMsgIndex,
		Freechat:      m.freeChatMode,
	}
	m.heroSessionRecoverBusy = true
	m.chatInputFocused = false
	m.heroSessionRecoverNativeID = nativeID
	m.heroSessionRecoverHarnessID = harnessID
	if sessionID != "" {
		m.heroChatSessionID = sessionID
	}
	return m
}

func (m model) sessionRecoverLiveAttachCmd(sessionID, nativeID, harnessID string) tea.Cmd {
	adapter := m.adapterForHarnessID(harnessID)
	attacher, ok := adapter.(harness.LiveStreamAttacher)
	projectDir := m.executeDir()
	ch := m.convStreamCh
	if !ok || !attacher.SupportsLiveStreamAttach() || ch == nil {
		return func() tea.Msg {
			return sessionRecoverAttachDoneMsg{
				sessionID: sessionID,
				err:       fmt.Errorf("live stream attach unavailable"),
			}
		}
	}
	relay := newConversationStreamRelay(sessionRecoverLiveAttachExecuteID, ch)
	return func() tea.Msg {
		ctx := context.Background()
		result, err := attacher.AttachLiveStream(ctx, harness.LiveStreamAttachRequest{
			NativeSessionID: nativeID,
			ProjectDir:      projectDir,
			OnStreamDelta: func(delta harness.StreamDelta) {
				relay.Enqueue(delta)
			},
		})
		relay.CloseAndWait()
		return sessionRecoverAttachDoneMsg{
			sessionID: sessionID,
			result:    result,
			err:       err,
		}
	}
}
