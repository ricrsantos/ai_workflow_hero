package tui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestChatSessionFirstTurnPersistAndRestore(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	h := &streamingHarness{deltas: []string{"Hello"}, sessionID: "native-1"}
	svc.Harness = h
	m := withDefaultChatModel(NewTestModel(svc))
	m = SetConversationInput(m, "hello history")
	next, cmd := m.submitConversation()
	next = drainConversationStream(t, next, cmd)

	if strings.TrimSpace(next.heroChatSessionID) == "" {
		t.Fatal("expected hero chat session id after first turn")
	}
	sessions, err := svc.Store.ListSessions(store.ListSessionsFilter{})
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions=%d err=%v", len(sessions), err)
	}
	events, err := svc.Store.ListSessionEventsNewest(next.heroChatSessionID, 0, 50)
	if err != nil || len(events) < 2 {
		t.Fatalf("events=%d err=%v", len(events), err)
	}

	restore := next.loadChatTranscriptCmd(next.heroChatSessionID)
	msg := restore()
	rest, ok := msg.(chatTranscriptRestoreMsg)
	if !ok || rest.err != nil {
		t.Fatalf("restore msg=%T err=%v", msg, rest.err)
	}
	if len(rest.transcript) < 2 {
		t.Fatalf("restored transcript=%d", len(rest.transcript))
	}
	foundUser := false
	for _, row := range rest.transcript {
		if row.role == convRoleUser && strings.Contains(row.content, "hello history") {
			foundUser = true
		}
	}
	if !foundUser {
		t.Fatalf("restored transcript missing user turn: %+v", rest.transcript)
	}
}

func TestNewChatKeepsHistoryRow(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	h := &streamingHarness{deltas: []string{"pong"}, sessionID: "native-1"}
	svc.Harness = h
	m := withDefaultChatModel(NewTestModel(svc))
	m = SetConversationInput(m, "persist me")
	next, cmd := m.submitConversation()
	next = drainConversationStream(t, next, cmd)
	heroID := strings.TrimSpace(next.heroChatSessionID)
	if heroID == "" {
		t.Fatal("missing hero session")
	}

	next, cmd = RunPaletteItemForTest(next, "/new-chat")
	if cmd != nil {
		_ = cmd()
	}
	if strings.TrimSpace(next.heroChatSessionID) != "" {
		t.Fatalf("hero session should be cleared after new-chat, got %q", next.heroChatSessionID)
	}
	sessions, err := svc.Store.ListSessions(store.ListSessionsFilter{})
	if err != nil || len(sessions) != 1 || sessions[0].ID != heroID {
		t.Fatalf("history row missing: %+v err=%v", sessions, err)
	}
}

func TestEventsToTranscriptTelegramOrigin(t *testing.T) {
	events := []store.SessionEvent{
		{EventType: store.SessionEventUser, Origin: store.SessionOriginTelegram, OriginAddress: "aiwk", PayloadJSON: `{"text":"from telegram"}`},
		{EventType: store.SessionEventAssistant, Origin: store.SessionOriginTelegram, OriginAddress: "aiwk", PayloadJSON: `{"text":"reply","agent_name":"orchestration_agent","model":"composer-2.5","harness_id":"cursor","occupancy_key":"freechat"}`},
	}
	transcript, _, occ := eventsToTranscript(events, nil)
	if len(transcript) != 2 {
		t.Fatalf("transcript=%d", len(transcript))
	}
	if transcript[0].origin != "telegram:aiwk" {
		t.Fatalf("user origin=%q", transcript[0].origin)
	}
	if transcript[1].origin != "telegram:aiwk" {
		t.Fatalf("assistant origin=%q", transcript[1].origin)
	}
	if got, ok := telegramOriginLabel(transcript[1]); !ok || got != "→ [Telegram · aiwk]" {
		t.Fatalf("assistant label=%q ok=%v", got, ok)
	}
	if transcript[1].agentName != "orchestration_agent" {
		t.Fatalf("agent=%q", transcript[1].agentName)
	}
	if occ["freechat"] == 0 {
		t.Fatal("expected occupancy for restored session")
	}
}

func TestAssistantPersistOriginTelegram(t *testing.T) {
	origin, addr := assistantPersistOrigin(convExecute{Origin: "telegram:addr"})
	if origin != store.SessionOriginTelegram || addr != "addr" {
		t.Fatalf("origin=%q addr=%q", origin, addr)
	}
}

func TestPersistUserTurnBlocksWithoutStore(t *testing.T) {
	m := NewTestModel(nil)
	ex := convExecute{Origin: "telegram:addr"}
	_, err := m.persistUserTurnBeforeExecute(context.Background(), ex, "hi", convRoleUser, "cursor", "composer-2.5", nil)
	if err != nil {
		t.Fatalf("nil store should not error: %v", err)
	}
}

func TestEmptyChatCreatesNoHeroSessionRow(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	m := NewTestModel(svc)
	if strings.TrimSpace(m.heroChatSessionID) != "" {
		t.Fatal("empty chat should not have hero session id")
	}
	sessSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	res, err := sessSvc.EnsureFirstTurn(context.Background(), "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat}, conversation.FirstTurnContent{})
	if err != nil || res.SessionID != "" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestSessionInterruptionRestore(t *testing.T) {
	events := []store.SessionEvent{
		{EventType: store.SessionEventUser, PayloadJSON: `{"text":"hi"}`},
		{EventType: store.SessionEventInterruption, PayloadJSON: `{"text":"interrupted"}`},
	}
	transcript, _, _ := eventsToTranscript(events, nil)
	if len(transcript) != 2 || !transcript[1].interrupted {
		t.Fatalf("transcript=%+v", transcript)
	}
}

func TestAssetEventRestore(t *testing.T) {
	asset := harness.Asset{
		Attachment: harness.Attachment{Name: "shot.png", Path: "/tmp/shot.png", ContentHash: "abc"},
		Source:     harness.AssetSourceModel,
	}
	raw, err := json.Marshal(asset)
	if err != nil {
		t.Fatal(err)
	}
	events := []store.SessionEvent{
		{EventType: store.SessionEventAsset, PayloadJSON: string(raw)},
	}
	transcript, assets, _ := eventsToTranscript(events, nil)
	if len(assets) != 1 || assets[0].ContentHash != "abc" {
		t.Fatalf("assets=%+v", assets)
	}
	if len(transcript) != 1 || len(transcript[0].assets) != 1 {
		t.Fatalf("transcript=%+v", transcript)
	}
}

func TestAttachmentEventRestoreAsAssetCard(t *testing.T) {
	payload := `{"type":"image","filename":"remote.png","mime":"image/png","url":"https://example.com/remote.png"}`
	events := []store.SessionEvent{
		{EventType: store.SessionEventAttachment, PayloadJSON: payload},
	}
	transcript, assets, _ := eventsToTranscript(events, nil)
	if len(assets) != 1 || assets[0].Name != "remote.png" {
		t.Fatalf("assets=%+v", assets)
	}
	if len(transcript) != 1 || len(transcript[0].assets) != 1 {
		t.Fatalf("transcript=%+v", transcript)
	}
}

func TestImportedAssetEventRestore(t *testing.T) {
	asset := harness.Asset{
		Attachment: harness.Attachment{Name: "import.png", Path: "https://example.com/import.png", ContentHash: "import-hash"},
		Source:     harness.AssetSourceModel,
	}
	raw, err := json.Marshal(asset)
	if err != nil {
		t.Fatal(err)
	}
	events := []store.SessionEvent{
		{EventType: store.SessionEventAsset, PayloadJSON: string(raw)},
	}
	transcript, assets, _ := eventsToTranscript(events, nil)
	if len(assets) != 1 || assets[0].ContentHash != "import-hash" {
		t.Fatalf("assets=%+v", assets)
	}
	if len(transcript) != 1 || transcript[0].assets[0].Name != "import.png" {
		t.Fatalf("transcript=%+v", transcript)
	}
}

func TestDrainSessionPersistSurfacesAppendFailure(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m.heroChatSessionID = "missing-session-id"
	m.queueSessionPersist(store.AppendSessionEventInput{
		EventType:   store.SessionEventUser,
		Origin:      store.SessionOriginLocal,
		PayloadJSON: `{"text":"x"}`,
	})
	_, cmd := m.drainSessionPersistCmd()
	if cmd == nil {
		t.Fatal("expected persist cmd")
	}
	msg := cmd()
	if _, ok := msg.(sessionPersistErrMsg); !ok {
		t.Fatalf("msg=%T want sessionPersistErrMsg", msg)
	}
	next, _ := m.handleConversationMsg(msg)
	nm, ok := next.(model)
	if !ok {
		t.Fatalf("model type %T", next)
	}
	if !nm.sessionPersistBlocked {
		t.Fatal("expected session persist blocked")
	}
}

func TestSyncPersistExecuteResultPropagatesBindError(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	meta := conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "t"}
	turn := conversation.FirstTurnContent{Text: "hi"}
	res, err := sessionSvc.EnsureFirstTurn(ctx, "", meta, turn)
	if err != nil {
		t.Fatal(err)
	}
	heroID := res.SessionID
	err = syncPersistExecuteResult(ctx, sessionSvc, heroID, convExecute{}, &harness.ExecutionResult{
		Output:    "done",
		SessionID: "native-bind",
	}, "cursor", "composer-2.5", nil)
	if err != nil {
		t.Fatalf("unexpected persist error: %v", err)
	}
}

func TestSessionRecoverUnsupportedStaysBlocked(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	m := NewTestModel(svc)
	m.heroChatSessionID = "sess-1"
	cmd := m.sessionRecoverCheckCmd("sess-1", "", "cursor")
	msg := cmd().(sessionRecoverStatusMsg)
	if !msg.unsupported {
		t.Fatalf("expected unsupported, got %+v", msg)
	}
	next, follow := m.handleSessionRecoverStatus(msg)
	if follow != nil {
		t.Fatal("unexpected follow-up cmd for unsupported")
	}
	if !next.heroSessionRecoverBusy {
		t.Fatal("expected composer blocked")
	}
	if next.convError == "" {
		t.Fatal("expected actionable error copy")
	}
}

type statusHarness struct {
	streamingHarness
	state harness.ExecutionStatus
}

func (h *statusHarness) Status(_ context.Context, _ string) (*harness.ExecutionStatus, error) {
	st := h.state
	return &st, nil
}

func TestSessionRecoverRunningUnsupportedForCursor(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	h := &statusHarness{streamingHarness: streamingHarness{}, state: harness.ExecutionStatus{State: harness.StatusRunning}}
	svc.Harness = h
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	meta := conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "t", HarnessID: "cursor"}
	turn := conversation.FirstTurnContent{Text: "hi"}
	res, err := sessionSvc.EnsureFirstTurn(ctx, "", meta, turn)
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m.heroChatSessionID = res.SessionID
	statusMsg := m.sessionRecoverCheckCmd(res.SessionID, "native-run", "cursor")().(sessionRecoverStatusMsg)
	if !statusMsg.unsupported {
		t.Fatalf("expected unsupported for running cursor, got %+v", statusMsg)
	}
	next, follow := m.handleSessionRecoverStatus(statusMsg)
	if follow != nil {
		t.Fatal("unexpected follow-up for unsupported running harness")
	}
	if !next.heroSessionRecoverBusy {
		t.Fatal("expected composer blocked")
	}
	if next.convError == "" {
		t.Fatal("expected actionable error copy")
	}
}

type liveAttachHarness struct {
	streamingHarness
	state  harness.ExecutionStatus
	attach bool
}

func (h *liveAttachHarness) Status(_ context.Context, _ string) (*harness.ExecutionStatus, error) {
	st := h.state
	return &st, nil
}

func (h *liveAttachHarness) SupportsLiveStreamAttach() bool { return h.attach }

func (h *liveAttachHarness) AttachLiveStream(_ context.Context, req harness.LiveStreamAttachRequest) (*harness.ExecutionResult, error) {
	if req.OnStreamDelta != nil {
		req.OnStreamDelta(harness.StreamDelta{Kind: harness.StreamKindText, Text: "partial", SessionID: "native-run"})
	}
	return &harness.ExecutionResult{SessionID: "native-run", Output: "done"}, nil
}

func TestSessionRecoverRunningLiveAttach(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	h := &liveAttachHarness{
		streamingHarness: streamingHarness{},
		state:            harness.ExecutionStatus{State: harness.StatusRunning},
		attach:           true,
	}
	svc.Harness = h
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	meta := conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "t", HarnessID: "cursor"}
	turn := conversation.FirstTurnContent{Text: "hi"}
	res, err := sessionSvc.EnsureFirstTurn(ctx, "", meta, turn)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sessionSvc.Store.UpdateSessionLifecycle(res.SessionID, store.SessionLifecycleInterrupted, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m.heroChatSessionID = res.SessionID
	m.transcript = []convMessage{{role: convRoleUser, content: "hi"}, {role: convRoleAgent, content: "", interrupted: true}}
	statusMsg := m.sessionRecoverCheckCmd(res.SessionID, "native-run", "cursor")().(sessionRecoverStatusMsg)
	if !statusMsg.attachLive {
		t.Fatalf("expected attachLive, got %+v", statusMsg)
	}
	next, attachCmd := m.handleSessionRecoverStatus(statusMsg)
	if attachCmd == nil {
		t.Fatal("expected attach cmd")
	}
	if !next.heroSessionRecoverBusy || !next.streaming {
		t.Fatal("expected recover busy and streaming during attach")
	}
	doneMsg, ok := next.sessionRecoverLiveAttachCmd(res.SessionID, "native-run", "cursor")().(sessionRecoverAttachDoneMsg)
	if !ok {
		t.Fatalf("attach cmd msg=%T", attachCmd())
	}
	updated, _ := next.handleConversationMsg(doneMsg)
	final, ok := updated.(model)
	if !ok {
		t.Fatalf("model type %T", updated)
	}
	if final.heroSessionRecoverBusy {
		t.Fatal("expected composer unblocked after attach")
	}
	if final.convError != "" {
		t.Fatalf("convError=%q", final.convError)
	}
}

func TestQuitWhileStreamingMarksInterrupted(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	meta := conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "t"}
	turn := conversation.FirstTurnContent{Text: "hi"}
	res, err := sessionSvc.EnsureFirstTurn(ctx, "", meta, turn)
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m.heroChatSessionID = res.SessionID
	m.streaming = true
	m.agentMsgIndex = 1
	m.transcript = []convMessage{{role: convRoleUser, content: "hi"}, {role: convRoleAgent, content: "partial"}}
	m.pendingQuitAfterInterrupt = true

	updated, cmd := m.handleConversationMsg(streamCancelDoneMsg{})
	next, ok := updated.(model)
	if !ok {
		t.Fatalf("model type %T", updated)
	}
	if next.pendingQuitAfterInterrupt {
		t.Fatal("pending quit flag should clear")
	}
	if cmd == nil {
		t.Fatal("expected finalize cmd")
	}
	if _, ok := cmd().(sessionInterruptFinalizeDoneMsg); !ok {
		t.Fatalf("finalize msg=%T", cmd())
	}
	sess, err := sessionSvc.GetSession(ctx, res.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Lifecycle != store.SessionLifecycleInterrupted {
		t.Fatalf("lifecycle=%q want interrupted", sess.Lifecycle)
	}
}

func TestSyncPersistExecuteResultPersistsAssets(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	res, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat}, conversation.FirstTurnContent{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	asset := harness.Asset{
		Attachment: harness.Attachment{
			Name:        "a.png",
			Path:        "/tmp/a.png",
			MIMEType:    "image/png",
			ContentHash: "hash-asset-1",
		},
		Source: harness.AssetSourceModel,
	}
	err = syncPersistExecuteResult(ctx, sessionSvc, res.SessionID, convExecute{}, &harness.ExecutionResult{
		Output: "ok",
		Assets: []harness.Asset{asset},
	}, "cursor", "model", nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	assets, err := svc.Store.ListSessionAssets(res.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].AssetID != "hash-asset-1" {
		t.Fatalf("assets=%+v", assets)
	}
}

func TestSubmitBlockedBySessionLeaseLost(t *testing.T) {
	m := model{sessionLeaseLost: true}
	if !m.submitBlockedBySessionPersist() {
		t.Fatal("lease lost must block submits")
	}
}
