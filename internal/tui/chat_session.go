package tui

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// sessionPersistGates serialize durable writes per Hero session so stream
// queue drains, execute-result sync writes, and interrupt finalization cannot
// reorder or interleave failed suffixes (find-qa-28).
var sessionPersistGates sync.Map // map[string]*sync.Mutex

func sessionPersistGate(sessionID string) *sync.Mutex {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = "_"
	}
	v, _ := sessionPersistGates.LoadOrStore(sessionID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

type heroSessionBoundMsg struct {
	executeID string
	sessionID string
}

type sessionPersistErrMsg struct {
	err           error
	events        []store.AppendSessionEventInput
	assets        []sessionAssetPersistItem
	bindings      []sessionBindingPersistItem
	serialization bool
}

// sessionTurnPersistError carries retryable first-turn / attachment failure state
// so partial durable sessions can be resumed without dropping the payload (find-qa-25).
type sessionTurnPersistError struct {
	err           error
	sessionID     string
	events        []store.AppendSessionEventInput
	assets        []sessionAssetPersistItem
	bindings      []sessionBindingPersistItem
	serialization bool
}

func (e *sessionTurnPersistError) Error() string {
	if e == nil || e.err == nil {
		return "session turn persistence failed"
	}
	return e.err.Error()
}

func (e *sessionTurnPersistError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

type sessionBindingPersistItem struct {
	orchestration bool
	stage         string
	sessionID     string
	harnessID     string
}

type sessionAssetPersistItem struct {
	sessionID string
	asset     harness.Asset
}

type sessionRecoverPollTickMsg struct {
	sessionID string
	nativeID  string
	harnessID string
}

type sessionRecoverDismissedMsg struct{}

type sessionLeaseLostMsg struct {
	sessionID string
}

type sessionInterruptFinalizeDoneMsg struct{}

type sessionRecoverAttachDoneMsg struct {
	sessionID string
	err       error
	result    *harness.ExecutionResult
}

type chatTranscriptRestoreMsg struct {
	sessionID              string
	transcript             []convMessage
	assets                 []harness.Asset
	occupancy              map[string]int64
	nativeSessionID        string
	harnessID              string
	model                  string
	title                  string
	lifecycle              string
	session                store.Session
	historicalContinuation bool
	err                    error
}

func newTuiOwnerID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "tui-owner-fallback"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (m model) heroChatSessionActive() bool {
	return m.sessionService != nil && strings.TrimSpace(m.heroChatSessionID) != ""
}

func (m *model) queueSessionPersist(in store.AppendSessionEventInput) {
	if m.sessionService == nil {
		return
	}
	heroID := strings.TrimSpace(m.heroChatSessionID)
	if heroID == "" {
		return
	}
	in.BoundSessionID = heroID
	in.SessionID = heroID
	m.sessionPersistQueue = append(m.sessionPersistQueue, in)
}

func (m *model) queueSessionBindingPersist(orchestration bool, stage, sessionID, harnessID string) {
	if m.svc == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	stage = strings.TrimSpace(stage)
	if sessionID == "" || harnessID == "" {
		return
	}
	if orchestration {
		m.sessionBindingQueue = append(m.sessionBindingQueue, sessionBindingPersistItem{
			orchestration: true,
			sessionID:     sessionID,
			harnessID:     harnessID,
		})
		return
	}
	if stage == "" {
		return
	}
	m.sessionBindingQueue = append(m.sessionBindingQueue, sessionBindingPersistItem{
		stage:     stage,
		sessionID: sessionID,
		harnessID: harnessID,
	})
}

func (m model) drainSessionPersistCmd() (model, tea.Cmd) {
	hasEvents := len(m.sessionPersistQueue) > 0
	hasAssets := len(m.sessionAssetUpsertQueue) > 0
	hasBindings := len(m.sessionBindingQueue) > 0
	if !hasEvents && !hasAssets && !hasBindings {
		return m, nil
	}
	if (hasEvents || hasAssets) && m.sessionService == nil {
		return m, nil
	}
	if hasBindings && m.svc == nil && !hasEvents && !hasAssets {
		return m, nil
	}
	eventBatch := append([]store.AppendSessionEventInput(nil), m.sessionPersistQueue...)
	assetBatch := append([]sessionAssetPersistItem(nil), m.sessionAssetUpsertQueue...)
	bindingBatch := append([]sessionBindingPersistItem(nil), m.sessionBindingQueue...)
	m.sessionPersistQueue = nil
	m.sessionAssetUpsertQueue = nil
	m.sessionBindingQueue = nil
	svc := m.sessionService
	cycleSvc := m.svc
	return m, func() tea.Msg {
		heroKey := ""
		if len(eventBatch) > 0 {
			heroKey = strings.TrimSpace(eventBatch[0].SessionID)
			if heroKey == "" {
				heroKey = strings.TrimSpace(eventBatch[0].BoundSessionID)
			}
		}
		if heroKey == "" && len(assetBatch) > 0 {
			heroKey = strings.TrimSpace(assetBatch[0].sessionID)
		}
		gate := sessionPersistGate(heroKey)
		gate.Lock()
		defer gate.Unlock()
		fail := func(err error, eventIdx, assetIdx, bindingIdx int, serialization bool) tea.Msg {
			return sessionPersistErrMsg{
				err:           err,
				events:        append([]store.AppendSessionEventInput(nil), eventBatch[eventIdx:]...),
				assets:        append([]sessionAssetPersistItem(nil), assetBatch[assetIdx:]...),
				bindings:      append([]sessionBindingPersistItem(nil), bindingBatch[bindingIdx:]...),
				serialization: serialization,
			}
		}
		for i, in := range eventBatch {
			if svc == nil {
				return fail(fmt.Errorf("session service unavailable"), i, 0, 0, false)
			}
			if _, err := svc.AppendEvent(context.Background(), in); err != nil {
				slog.Error("tui session event persist failed", "event_type", in.EventType)
				return fail(err, i, 0, 0, false)
			}
		}
		for i, item := range assetBatch {
			if svc == nil || svc.Store == nil {
				return fail(fmt.Errorf("session store unavailable"), len(eventBatch), i, 0, false)
			}
			heroID := strings.TrimSpace(item.sessionID)
			if heroID == "" {
				continue
			}
			asset := item.asset
			cardMeta, err := json.Marshal(asset)
			if err != nil {
				slog.Error("tui session asset card meta marshal failed")
				return fail(fmt.Errorf("asset card serialization failed: %w", err), len(eventBatch), i, 0, true)
			}
			if err := svc.Store.UpsertSessionAsset(store.SessionAsset{
				SessionID:    heroID,
				AssetID:      asset.ContentHash,
				Ownership:    store.AssetOwnershipManagedCopy,
				Path:         asset.Path,
				Mime:         asset.MIMEType,
				OriginalName: asset.Name,
				CardMetaJSON: string(cardMeta),
			}); err != nil {
				slog.Error("tui session asset persist failed")
				return fail(err, len(eventBatch), i, 0, false)
			}
		}
		for i, item := range bindingBatch {
			if cycleSvc == nil {
				continue
			}
			if item.orchestration {
				if err := cycleSvc.SetOrchestrationSession(item.sessionID, item.harnessID); err != nil {
					slog.Error("tui persist orchestration session failed")
					return fail(err, len(eventBatch), len(assetBatch), i, false)
				}
				continue
			}
			if err := cycleSvc.SetStageSessionBinding(item.stage, item.harnessID, item.sessionID); err != nil {
				slog.Error("tui persist stage session binding failed")
				return fail(err, len(eventBatch), len(assetBatch), i, false)
			}
		}
		return nil
	}
}

func (m model) releaseHeroChatLeaseCmd(sessionID string) tea.Cmd {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || m.sessionService == nil {
		return nil
	}
	owner := strings.TrimSpace(m.tuiOwnerID)
	svc := m.sessionService
	return func() tea.Msg {
		if err := svc.ReleaseLease(context.Background(), sessionID, owner); err != nil {
			slog.Debug("tui session lease release failed")
		}
		return nil
	}
}

func marshalChatSessionProps(props map[string]string) string {
	if len(props) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(props)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func (m model) chatSessionCreateMeta(ex convExecute, pairHarness, pairModel string, props map[string]string) conversation.CreateSessionParams {
	kind := store.SessionKindFreechat
	stage := strings.TrimSpace(ex.StageName)
	agent := strings.TrimSpace(ex.AgentName)
	var cycleID *int64
	cycleNumber := 0
	// Refresh-snapshotted cycle metadata only (no store I/O on Update/execute paths).
	if m.status.CycleNumber > 0 {
		cycleNumber = m.status.CycleNumber
	}
	if m.sessionTimer.cycleID > 0 {
		id := m.sessionTimer.cycleID
		cycleID = &id
		if cycleNumber == 0 && m.sessionTimer.cycleNumber > 0 {
			cycleNumber = m.sessionTimer.cycleNumber
		}
	}
	title := ""
	switch {
	case m.orchestrationLive || agent == agentOrchestration:
		kind = store.SessionKindOrchestration
		title = conversation.TitleOrchestration(cycleNumber)
	case m.researchLive || stage == stageResearch:
		kind = store.SessionKindResearch
		title = conversation.TitleResearch(cycleNumber)
	case stage != "" && agent != "" && agent != agentOrchestration:
		kind = store.SessionKindStageAgent
		title = conversation.TitleStageAgent(cycleNumber, stage, agent)
	default:
		// Leave empty so EnsureFirstTurn derives TitleFreeChat from the first turn text.
		title = ""
	}
	origin := store.SessionOriginLocal
	if addr, ok := telegramAddressOf(ex.Origin); ok {
		origin = store.SessionOriginTelegram
		_ = addr
	}
	return conversation.CreateSessionParams{
		Kind:                kind,
		Title:               title,
		HarnessID:           strings.TrimSpace(pairHarness),
		Model:               strings.TrimSpace(pairModel),
		ModelPropertiesJSON: marshalChatSessionProps(props),
		CycleID:             cycleID,
		StageName:           stage,
		AgentName:           agent,
		TranscriptState:     store.TranscriptAvailable,
		LastOrigin:          origin,
	}
}

func (m model) chatFirstTurnContent(userLabel string, labelRole convRole, ex convExecute) conversation.FirstTurnContent {
	text := strings.TrimSpace(userLabel)
	attachmentCount := len(ex.Attachments)
	turn := conversation.FirstTurnContent{
		Text:            text,
		AttachmentCount: attachmentCount,
		Origin:          conversation.OriginLocal,
	}
	if addr, ok := telegramAddressOf(ex.Origin); ok {
		turn.Origin = conversation.OriginTelegram
		turn.OriginAddress = addr
	}
	if labelRole == convRoleSystem {
		turn.StageAgentPrompt = text
		turn.Text = ""
	}
	if text == "" && attachmentCount > 0 {
		turn.Text = ""
	}
	return turn
}

func sessionPersistenceError(err error) error {
	if err == nil {
		return fmt.Errorf("session persistence failed")
	}
	return fmt.Errorf("session persistence failed: %w", err)
}

func attachmentAsset(att harness.Attachment) harness.Asset {
	asset := harness.Asset{
		Source:     harness.AssetSourceUser,
		Attachment: att,
	}
	if strings.TrimSpace(asset.ContentHash) == "" {
		asset.ContentHash = harness.ContentHash(asset)
	}
	return asset
}

func attachmentPersistItems(heroID string, attachments []harness.Attachment) ([]sessionAssetPersistItem, error) {
	heroID = strings.TrimSpace(heroID)
	out := make([]sessionAssetPersistItem, 0, len(attachments))
	for _, att := range attachments {
		asset := attachmentAsset(att)
		if _, err := json.Marshal(asset); err != nil {
			return nil, err
		}
		out = append(out, sessionAssetPersistItem{sessionID: heroID, asset: asset})
	}
	return out, nil
}

func (m model) releaseLeaseQuietly(ctx context.Context, heroID string) {
	heroID = strings.TrimSpace(heroID)
	if m.sessionService == nil || heroID == "" {
		return
	}
	owner := strings.TrimSpace(m.tuiOwnerID)
	if err := m.sessionService.ReleaseLease(ctx, heroID, owner); err != nil {
		slog.Error("tui session lease release after persist failure failed")
	}
}
func (m model) persistAcceptedAttachments(ctx context.Context, heroID string, attachments []harness.Attachment) error {
	heroID = strings.TrimSpace(heroID)
	if m.sessionService == nil || heroID == "" || len(attachments) == 0 {
		return nil
	}
	gate := sessionPersistGate(heroID)
	gate.Lock()
	defer gate.Unlock()
	for _, att := range attachments {
		asset := attachmentAsset(att)
		raw, err := json.Marshal(asset)
		if err != nil {
			return sessionPersistenceError(err)
		}
		if _, err := m.sessionService.AppendEvent(ctx, store.AppendSessionEventInput{
			BoundSessionID: heroID,
			SessionID:      heroID,
			EventType:      store.SessionEventAttachment,
			Origin:         store.SessionOriginLocal,
			PayloadJSON:    string(raw),
		}); err != nil {
			return sessionPersistenceError(err)
		}
		cardMeta, err := json.Marshal(asset)
		if err != nil {
			cardMeta = []byte("{}")
		}
		assetID := strings.TrimSpace(asset.ContentHash)
		if assetID == "" {
			assetID = strings.TrimSpace(asset.ID)
		}
		if assetID == "" {
			continue
		}
		if err := m.sessionService.Store.UpsertSessionAsset(store.SessionAsset{
			SessionID:    heroID,
			AssetID:      assetID,
			Ownership:    store.AssetOwnershipManagedCopy,
			Path:         asset.Path,
			Mime:         asset.MIMEType,
			OriginalName: asset.Name,
			CardMetaJSON: string(cardMeta),
		}); err != nil {
			return sessionPersistenceError(err)
		}
	}
	return nil
}

func (m model) persistUserTurnBeforeExecute(
	ctx context.Context,
	ex convExecute,
	userLabel string,
	labelRole convRole,
	pairHarness, pairModel string,
	props map[string]string,
) (string, error) {
	if m.sessionService == nil {
		return strings.TrimSpace(m.heroChatSessionID), nil
	}
	turn := m.chatFirstTurnContent(userLabel, labelRole, ex)
	if !conversation.HasAcceptedTurn(turn) {
		return strings.TrimSpace(m.heroChatSessionID), nil
	}
	heroID := strings.TrimSpace(m.heroChatSessionID)
	meta := m.chatSessionCreateMeta(ex, pairHarness, pairModel, props)
	if heroID == "" {
		if provisional := strings.TrimSpace(m.mediaSessionID); provisional != "" {
			meta.ID = provisional
		}
		res, err := m.sessionService.EnsureFirstTurn(ctx, "", meta, turn)
		if err != nil {
			return "", sessionPersistenceError(err)
		}
		heroID = strings.TrimSpace(res.SessionID)
		if heroID == "" {
			return "", nil
		}
		owner := strings.TrimSpace(m.tuiOwnerID)
		if _, err := m.sessionService.AcquireLease(ctx, heroID, owner); err != nil {
			return "", sessionPersistenceError(err)
		}
		slog.Info("tui hero chat session created")
		if err := m.persistAcceptedAttachments(ctx, heroID, ex.Attachments); err != nil {
			assets, serErr := attachmentPersistItems(heroID, ex.Attachments)
			m.releaseLeaseQuietly(ctx, heroID)
			if serErr != nil {
				return "", &sessionTurnPersistError{err: sessionPersistenceError(serErr), sessionID: heroID, assets: assets, serialization: true}
			}
			return "", &sessionTurnPersistError{err: err, sessionID: heroID, assets: assets}
		}
		return heroID, nil
	}
	payload, err := buildUserPersistPayload(userLabel, labelRole, turn.AttachmentCount)
	if err != nil {
		return "", sessionPersistenceError(err)
	}
	origin := store.SessionOriginLocal
	addr := ""
	if turn.Origin == conversation.OriginTelegram {
		origin = store.SessionOriginTelegram
		addr = turn.OriginAddress
	}
	_, err = m.sessionService.AppendEvent(ctx, store.AppendSessionEventInput{
		BoundSessionID: heroID,
		SessionID:      heroID,
		EventType:      store.SessionEventUser,
		Origin:         origin,
		OriginAddress:  addr,
		PayloadJSON:    payload,
	})
	if err != nil {
		return "", sessionPersistenceError(err)
	}
	if err := m.persistAcceptedAttachments(ctx, heroID, ex.Attachments); err != nil {
		assets, serErr := attachmentPersistItems(heroID, ex.Attachments)
		if serErr != nil {
			return "", &sessionTurnPersistError{err: sessionPersistenceError(serErr), sessionID: heroID, assets: assets, serialization: true}
		}
		return "", &sessionTurnPersistError{err: err, sessionID: heroID, assets: assets}
	}
	return heroID, nil
}

func buildUserPersistPayload(userLabel string, labelRole convRole, attachmentCount int) (string, error) {
	body := map[string]any{
		"text": strings.TrimSpace(userLabel),
	}
	if labelRole == convRoleSystem {
		body["label_role"] = "system"
	}
	if attachmentCount > 0 {
		body["attachment_count"] = attachmentCount
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (m *model) queuePersistForMessage(msg convMessage) error {
	eventType, ok := sessionEventTypeForRole(msg.role)
	if !ok {
		return nil
	}
	payload, err := marshalTranscriptPayload(msg)
	if err != nil {
		return err
	}
	origin, addr := sessionOriginFromConv(msg.origin)
	m.queueSessionPersist(store.AppendSessionEventInput{
		EventType:     eventType,
		Origin:        origin,
		OriginAddress: addr,
		PayloadJSON:   payload,
	})
	return nil
}

func sessionEventTypeForRole(role convRole) (string, bool) {
	switch role {
	case convRoleUser, convRoleSystem:
		return store.SessionEventUser, true
	case convRoleAgent:
		return store.SessionEventAssistant, true
	case convRoleThinking:
		return store.SessionEventThinking, true
	case convRoleTool:
		return store.SessionEventTool, true
	case convRoleWarning:
		return store.SessionEventWarning, true
	case convRoleActivity:
		return store.SessionEventTool, true
	default:
		return "", false
	}
}

func marshalTranscriptPayload(msg convMessage) (string, error) {
	body := map[string]any{
		"text": msg.content,
	}
	if msg.agentName != "" {
		body["agent_name"] = msg.agentName
	}
	if msg.modelSlug != "" {
		body["model"] = msg.modelSlug
	}
	if msg.harnessID != "" {
		body["harness_id"] = msg.harnessID
	}
	if msg.callID != "" {
		body["call_id"] = msg.callID
	}
	if msg.interrupted {
		body["interrupted"] = true
	}
	if msg.failed {
		body["failed"] = true
	}
	if msg.role == convRoleSystem {
		body["label_role"] = "system"
	}
	if msg.occupancyKey != "" {
		body["occupancy_key"] = msg.occupancyKey
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func sessionOriginFromConv(origin string) (string, string) {
	if addr, ok := telegramAddressOf(origin); ok {
		return store.SessionOriginTelegram, addr
	}
	return store.SessionOriginLocal, ""
}

func assistantPersistOrigin(ex convExecute) (string, string) {
	if addr, ok := telegramAddressOf(ex.Origin); ok {
		return store.SessionOriginTelegram, addr
	}
	return store.SessionOriginLocal, ""
}

func (m *model) queueSessionInterruption() {
	m.queueSessionPersist(store.AppendSessionEventInput{
		EventType:   store.SessionEventInterruption,
		Origin:      store.SessionOriginLocal,
		PayloadJSON: `{"text":"interrupted"}`,
	})
}

func (m *model) queueSessionAsset(asset harness.Asset) error {
	payload, err := json.Marshal(asset)
	if err != nil {
		return err
	}
	m.queueSessionPersist(store.AppendSessionEventInput{
		EventType:   store.SessionEventAsset,
		Origin:      store.SessionOriginLocal,
		PayloadJSON: string(payload),
	})
	if m.sessionService == nil || m.sessionService.Store == nil {
		return nil
	}
	heroID := strings.TrimSpace(m.heroChatSessionID)
	if heroID == "" {
		return nil
	}
	m.sessionAssetUpsertQueue = append(m.sessionAssetUpsertQueue, sessionAssetPersistItem{
		sessionID: heroID,
		asset:     asset,
	})
	return nil
}

func (m model) bindNativeHeroSessionCmd(harnessID, nativeID, modelSlug string, props map[string]string) tea.Cmd {
	heroID := strings.TrimSpace(m.heroChatSessionID)
	if heroID == "" || m.sessionService == nil || strings.TrimSpace(nativeID) == "" {
		return nil
	}
	svc := m.sessionService
	return func() tea.Msg {
		_, err := svc.BindNativeSession(context.Background(), heroID, harnessID, nativeID, modelSlug, marshalChatSessionProps(props))
		if err != nil {
			slog.Error("tui bind native hero session failed")
			return sessionPersistErrMsg{err: err}
		}
		return nil
	}
}

func (m model) loadChatTranscriptCmd(sessionID string) tea.Cmd {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || m.sessionService == nil {
		return nil
	}
	svc := m.sessionService
	storeDB := svc.Store
	return func() tea.Msg {
		events, err := svc.ListEventsNewest(context.Background(), sessionID, 0, store.DefaultSessionEventPage)
		if err != nil {
			return chatTranscriptRestoreMsg{sessionID: sessionID, err: err}
		}
		assetRows, err := storeDB.ListSessionAssets(sessionID)
		if err != nil {
			return chatTranscriptRestoreMsg{sessionID: sessionID, err: err}
		}
		transcript, assets, occupancy := eventsToTranscript(events, assetRows)
		sess, err := svc.GetSession(context.Background(), sessionID)
		if err != nil {
			return chatTranscriptRestoreMsg{sessionID: sessionID, err: err}
		}
		historical := false
		if sess.Kind == store.SessionKindStageAgent && sess.CycleID.Valid && svc.Store != nil {
			if stage, stageErr := svc.Store.GetStage(sess.CycleID.Int64, sess.StageName); stageErr == nil {
				historical = stage.Status == store.StageCompleted
			}
		}
		return chatTranscriptRestoreMsg{
			sessionID:              sessionID,
			transcript:             transcript,
			assets:                 assets,
			occupancy:              occupancy,
			nativeSessionID:        sess.NativeSessionID,
			harnessID:              sess.HarnessID,
			model:                  sess.Model,
			title:                  sess.Title,
			lifecycle:              sess.Lifecycle,
			session:                sess,
			historicalContinuation: historical,
		}
	}
}

func eventsToTranscript(events []store.SessionEvent, assetRows []store.SessionAsset) ([]convMessage, []harness.Asset, map[string]int64) {
	assetByHash := map[string]harness.Asset{}
	for _, row := range assetRows {
		var asset harness.Asset
		if err := json.Unmarshal([]byte(row.CardMetaJSON), &asset); err != nil || asset.ContentHash == "" {
			asset = harness.Asset{
				Attachment: harness.Attachment{
					Path:        row.Path,
					MIMEType:    row.Mime,
					Name:        row.OriginalName,
					ContentHash: row.AssetID,
				},
				SessionID: row.SessionID,
			}
		}
		if asset.Path == "" {
			asset.Path = row.Path
		}
		if asset.Name == "" {
			asset.Name = row.OriginalName
		}
		if asset.ContentHash == "" {
			asset.ContentHash = row.AssetID
		}
		assetByHash[asset.ContentHash] = asset
	}
	var transcript []convMessage
	occupancy := make(map[string]int64)
	var cardAssets []harness.Asset
	for _, ev := range events {
		msg, asset, ok := eventToConvMessage(ev, assetByHash)
		if !ok {
			continue
		}
		transcript = append(transcript, msg)
		if asset != nil {
			cardAssets = append(cardAssets, *asset)
		}
		if key := strings.TrimSpace(msg.occupancyKey); key != "" && msg.role == convRoleAgent {
			occ := harness.EstimateUsage(msg.content, "").Occupancy()
			if occ > 0 {
				occupancy[key] = occ
			}
		}
	}
	return transcript, cardAssets, occupancy
}

func eventToConvMessage(ev store.SessionEvent, assetByHash map[string]harness.Asset) (convMessage, *harness.Asset, bool) {
	switch ev.EventType {
	case store.SessionEventInterruption:
		return convMessage{role: convRoleAgent, content: "", interrupted: true}, nil, true
	case store.SessionEventAttachment:
		asset, ok := restoreImportedAttachmentAsset(ev.PayloadJSON)
		if !ok {
			return convMessage{}, nil, false
		}
		if asset.ContentHash != "" {
			if merged, ok := assetByHash[asset.ContentHash]; ok {
				asset = merged
			}
		}
		return convMessage{role: convRoleAgent, content: "", assets: []harness.Asset{asset}}, &asset, true
	case store.SessionEventAsset:
		var asset harness.Asset
		if err := json.Unmarshal([]byte(ev.PayloadJSON), &asset); err != nil {
			return convMessage{}, nil, false
		}
		if asset.ContentHash != "" {
			if merged, ok := assetByHash[asset.ContentHash]; ok {
				asset = merged
			}
		}
		return convMessage{role: convRoleAgent, content: "", assets: []harness.Asset{asset}}, &asset, true
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err != nil {
		return convMessage{}, nil, false
	}
	text := jsonStringField(payload, "text")
	role := convRoleAgent
	switch ev.EventType {
	case store.SessionEventUser:
		if jsonStringField(payload, "label_role") == "system" {
			role = convRoleSystem
		} else {
			role = convRoleUser
		}
	case store.SessionEventAssistant:
		role = convRoleAgent
	case store.SessionEventThinking:
		role = convRoleThinking
	case store.SessionEventTool:
		role = convRoleTool
	case store.SessionEventWarning, store.SessionEventPermission, store.SessionEventQuestion:
		role = convRoleWarning
	case store.SessionEventNote:
		role = convRoleSystem
	default:
		role = convRoleActivity
	}
	origin := ""
	if ev.Origin == store.SessionOriginTelegram {
		addr := strings.TrimSpace(ev.OriginAddress)
		if addr != "" {
			origin = "telegram:" + addr
		}
	}
	msg := convMessage{
		role:         role,
		content:      text,
		agentName:    jsonStringField(payload, "agent_name"),
		modelSlug:    jsonStringField(payload, "model"),
		harnessID:    jsonStringField(payload, "harness_id"),
		callID:       jsonStringField(payload, "call_id"),
		origin:       origin,
		occupancyKey: jsonStringField(payload, "occupancy_key"),
		interrupted:  jsonBoolField(payload, "interrupted"),
		failed:       jsonBoolField(payload, "failed"),
	}
	return msg, nil, true
}

func restoreImportedAttachmentAsset(payloadJSON string) (harness.Asset, bool) {
	raw := strings.TrimSpace(payloadJSON)
	if raw == "" {
		return harness.Asset{}, false
	}
	var asset harness.Asset
	if err := json.Unmarshal([]byte(raw), &asset); err == nil {
		if harness.ContentHash(asset) != "" || strings.TrimSpace(asset.Name) != "" || strings.TrimSpace(asset.Path) != "" {
			if asset.ContentHash == "" {
				asset.ContentHash = harness.ContentHash(asset)
			}
			return asset, true
		}
	}
	var part map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &part); err != nil {
		return harness.Asset{}, false
	}
	name := jsonStringField(part, "name", "filename", "fileName", "file_name")
	path := jsonStringField(part, "path", "filepath", "filePath", "file_path", "savedPath", "saved_path", "file")
	if path == "" {
		path = jsonStringField(part, "url", "uri", "href", "src")
	}
	mime := jsonStringField(part, "mime", "mimeType", "mime_type", "mediaType", "contentType", "content_type")
	id := jsonStringField(part, "id", "assetID", "assetId")
	if name == "" && path != "" {
		name = filepath.Base(path)
	}
	if name == "" && path == "" {
		return harness.Asset{}, false
	}
	contentHash := harness.HashBytes([]byte(raw))
	return harness.Asset{
		Source: harness.AssetSourceModel,
		Attachment: harness.Attachment{
			ID:          id,
			Name:        name,
			Path:        path,
			MIMEType:    mime,
			ContentHash: contentHash,
		},
	}, true
}

func jsonStringField(payload map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func jsonBoolField(payload map[string]json.RawMessage, key string) bool {
	raw, ok := payload[key]
	if !ok {
		return false
	}
	var b bool
	return json.Unmarshal(raw, &b) == nil && b
}

func (m model) applyChatTranscriptRestore(msg chatTranscriptRestoreMsg) model {
	if msg.err != nil {
		m.convError = msg.err.Error()
		return m
	}
	m = m.applyHeroSessionBinding(msg.session)
	m.heroSessionHistorical = msg.historicalContinuation
	m.heroChatSessionID = msg.sessionID
	m.heroLeasedSessionID = msg.sessionID
	m.sessionLeaseLost = false
	pm := &m
	pm.syncMediaSessionFromHero()
	m = *pm
	m.heroLeaseNextHeartbeat = time.Now().Add(sessionLeaseHeartbeatInterval)
	m.transcript = append([]convMessage(nil), msg.transcript...)
	m.bumpTranscriptLayout()
	m.assets = harness.MergeAssetsByContentHash(nil, msg.assets)
	m.contextOccupancy = msg.occupancy
	if len(msg.occupancy) > 0 {
		if occ, ok := msg.occupancy[occupancyKeyFreechat]; ok {
			m.contextUsedTokens = occ
			m.contextDisplayKey = occupancyKeyFreechat
		}
	}
	if slug := strings.TrimSpace(msg.model); slug != "" {
		m.chatModelSlug = slug
	}
	if sid := strings.TrimSpace(msg.nativeSessionID); sid != "" {
		switch msg.session.Kind {
		case store.SessionKindFreechat:
			m = m.persistFreechatSession(sid, msg.harnessID)
		default:
			m = m.persistHarnessSession(sid, msg.harnessID)
		}
	}
	m.transcriptScrollOffset = 0
	m.transcriptFollowBottom = true
	return m
}

func (m model) submitBlockedBySessionPersist() bool {
	return m.sessionPersistBlocked || m.sessionLeaseLost
}

func syncPersistExecuteResult(
	ctx context.Context,
	svc *conversation.SessionService,
	heroID string,
	ex convExecute,
	result *harness.ExecutionResult,
	harnessID, modelSlug string,
	props map[string]string,
) error {
	heroID = strings.TrimSpace(heroID)
	if svc == nil || heroID == "" {
		return nil
	}
	gate := sessionPersistGate(heroID)
	gate.Lock()
	defer gate.Unlock()
	if result == nil {
		return nil
	}
	text := strings.TrimSpace(result.Output)
	if text != "" {
		payload, err := marshalTranscriptPayload(convMessage{
			role:         convRoleAgent,
			content:      text,
			agentName:    ex.AgentName,
			modelSlug:    modelSlug,
			harnessID:    harnessID,
			occupancyKey: ex.OccupancyKey,
		})
		if err != nil {
			return sessionPersistenceError(err)
		}
		origin, addr := assistantPersistOrigin(ex)
		if _, err := svc.AppendEvent(ctx, store.AppendSessionEventInput{
			BoundSessionID: heroID,
			SessionID:      heroID,
			EventType:      store.SessionEventAssistant,
			Origin:         origin,
			OriginAddress:  addr,
			PayloadJSON:    payload,
		}); err != nil {
			return sessionPersistenceError(err)
		}
	}
	for _, asset := range result.Assets {
		raw, err := json.Marshal(asset)
		if err != nil {
			return sessionPersistenceError(fmt.Errorf("asset serialization failed: %w", err))
		}
		if _, err := svc.AppendEvent(ctx, store.AppendSessionEventInput{
			BoundSessionID: heroID,
			SessionID:      heroID,
			EventType:      store.SessionEventAsset,
			Origin:         store.SessionOriginLocal,
			PayloadJSON:    string(raw),
		}); err != nil {
			return sessionPersistenceError(err)
		}
		cardMeta, err := json.Marshal(asset)
		if err != nil {
			cardMeta = []byte("{}")
		}
		if err := svc.Store.UpsertSessionAsset(store.SessionAsset{
			SessionID:    heroID,
			AssetID:      asset.ContentHash,
			Ownership:    store.AssetOwnershipManagedCopy,
			Path:         asset.Path,
			Mime:         asset.MIMEType,
			OriginalName: asset.Name,
			CardMetaJSON: string(cardMeta),
		}); err != nil {
			return sessionPersistenceError(err)
		}
	}
	if sid := strings.TrimSpace(result.SessionID); sid != "" {
		if _, err := svc.BindNativeSession(ctx, heroID, harnessID, sid, modelSlug, marshalChatSessionProps(props)); err != nil {
			return sessionPersistenceError(err)
		}
	}
	return nil
}
