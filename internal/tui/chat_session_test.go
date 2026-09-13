package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

func TestConversationBatchPersistKeepsStreamWaiter(t *testing.T) {
	m, _, _ := newConversationTestModel(t)
	m.streaming = true
	ch := make(chan tea.Msg, 4)
	m.convStreamCh = ch
	m.runtimeAgentName = agentOrchestration
	m.transcript = []convMessage{{role: convRoleAgent, content: ""}}
	m.agentMsgIndex = 0
	m.executes = map[string]convExecute{
		"ex-1": {ID: "ex-1", HarnessID: "cursor", AgentName: agentOrchestration, AgentMsgIndex: 0},
	}

	next, cmd := m.Update(conversationBatchMsg{messages: []tea.Msg{
		streamDeltaMsg{
			executeID: "ex-1",
			delta: harness.StreamDelta{
				Kind:      harness.StreamKindSession,
				SessionID: "native-sess",
			},
		},
	}})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("model type %T", next)
	}
	if !got.streaming {
		t.Fatal("streaming must remain true")
	}
	if cmd == nil {
		t.Fatal("session persist drain must keep a stream waiter")
	}

	ch <- executeDoneMsg{executeID: "ex-1"}
	msg := runConversationCmd(cmd)
	if msg == nil {
		t.Fatal("stream waiter was dropped after binding persist; CloseAndWait would deadlock")
	}
}

func TestFirstTurnAttachmentPersistIsAtomicAndKeepsLease(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	m := NewTestModel(svc)
	ex := convExecute{Attachments: []harness.Attachment{
		{Name: "ok.png", Path: "/tmp/ok.png", MIMEType: "image/png", ContentHash: "hash-ok"},
		{Name: "bad.png", Path: "", MIMEType: "image/png", ContentHash: "hash-bad"},
	}}
	_, err := m.persistUserTurnBeforeExecute(context.Background(), ex, "with images", convRoleUser, "cursor", "composer-2.5", nil)
	if err == nil {
		t.Fatal("expected attachment persist failure")
	}
	pe, ok := err.(*sessionTurnPersistError)
	if !ok {
		t.Fatalf("want sessionTurnPersistError, got %T %v", err, err)
	}
	if len(pe.events) == 0 || len(pe.assets) == 0 {
		t.Fatalf("retry payload missing attachment suffix events=%d assets=%d", len(pe.events), len(pe.assets))
	}
	sessions, err := svc.Store.ListSessions(store.ListSessionsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("partial session persisted: %+v", sessions)
	}
}

func TestPersistDrainFIFOPreservesVisibleOrder(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	res, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "t"}, conversation.FirstTurnContent{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m.heroChatSessionID = res.SessionID
	m.testPersistDelay = 60 * time.Millisecond
	m.queueSessionPersist(store.AppendSessionEventInput{
		EventType: store.SessionEventAssistant, Origin: store.SessionOriginLocal, PayloadJSON: `{"text":"first"}`,
	})
	m, firstCmd := m.drainSessionPersistCmd()
	m.testPersistDelay = 0
	m.queueSessionPersist(store.AppendSessionEventInput{
		EventType: store.SessionEventAssistant, Origin: store.SessionOriginLocal, PayloadJSON: `{"text":"second"}`,
	})
	m, secondCmd := m.drainSessionPersistCmd()
	if firstCmd == nil || secondCmd == nil {
		t.Fatal("expected drain cmds")
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = secondCmd()
	}()
	time.Sleep(10 * time.Millisecond)
	go func() {
		defer wg.Done()
		_ = firstCmd()
	}()
	wg.Wait()
	events, err := svc.Store.ListSessionEventsNewest(res.SessionID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, ev := range events {
		if ev.EventType != store.SessionEventAssistant {
			continue
		}
		texts = append(texts, ev.PayloadJSON)
	}
	if len(texts) != 2 || !strings.Contains(texts[0], "first") || !strings.Contains(texts[1], "second") {
		t.Fatalf("durable order=%v want first then second", texts)
	}
}

func TestSyncPersistExecuteResultAtomicWithBindFailure(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	taken, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "a"}, conversation.FirstTurnContent{Text: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sessionSvc.BindNativeSession(ctx, taken.SessionID, "cursor", "native-taken", "m", "{}"); err != nil {
		t.Fatal(err)
	}
	target, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "b"}, conversation.FirstTurnContent{Text: "b"})
	if err != nil {
		t.Fatal(err)
	}
	asset := harness.Asset{
		Attachment: harness.Attachment{Name: "a.png", Path: "/tmp/a.png", MIMEType: "image/png", ContentHash: "hash-bind-fail"},
		Source:     harness.AssetSourceModel,
	}
	err = syncPersistExecuteResult(ctx, sessionSvc, target.SessionID, convExecute{}, &harness.ExecutionResult{
		Output:    "done",
		SessionID: "native-taken",
		Assets:    []harness.Asset{asset},
	}, "cursor", "m", nil)
	if err == nil {
		t.Fatal("expected bind failure")
	}
	events, err := svc.Store.ListSessionEventsNewest(target.SessionID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if ev.EventType == store.SessionEventAssistant || ev.EventType == store.SessionEventAsset {
			t.Fatalf("suffix leaked after bind failure: %+v", events)
		}
	}
	assets, err := svc.Store.ListSessionAssets(target.SessionID)
	if err != nil || len(assets) != 0 {
		t.Fatalf("assets leaked: %+v err=%v", assets, err)
	}
	got, err := sessionSvc.GetSession(ctx, target.SessionID)
	if err != nil || got.NativeSessionID != "" {
		t.Fatalf("native bind leaked: %+v err=%v", got, err)
	}
}

func TestLateExecuteDoneAfterCancelDoesNotPersistNativeBind(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	res, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "t"}, conversation.FirstTurnContent{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m.heroChatSessionID = res.SessionID
	m.streaming = true
	m.executes = map[string]convExecute{
		"ex-late": {ID: "ex-late", AgentMsgIndex: 1, Freechat: true},
	}
	m.transcript = []convMessage{{role: convRoleUser, content: "hi"}, {role: convRoleAgent, content: "partial"}}
	m.agentMsgIndex = 1
	next, cancelCmd := CancelConversationStreamForTest(m)
	if cancelCmd == nil {
		t.Fatal("expected cancel cmd")
	}
	_ = cancelCmd()
	updated, persistCmd := next.Update(executeDoneMsg{
		executeID: "ex-late",
		harnessID: "cursor",
		result:    &harness.ExecutionResult{Output: "late", SessionID: "native-late"},
	})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("model type %T", updated)
	}
	if persistCmd != nil {
		if msg := persistCmd(); msg != nil {
			if _, isOK := msg.(sessionPersistOKMsg); !isOK {
				next2, _ := got.handleConversationMsg(msg)
				if nm, ok := next2.(model); ok {
					got = nm
				}
			}
		}
	}
	sess, err := sessionSvc.GetSession(ctx, res.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if sess.NativeSessionID != "" {
		t.Fatalf("late completion bound native session: %+v", sess)
	}
}

func TestLeaseReleaseFailureRetainsOwnershipAndRetries(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	res, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "t"}, conversation.FirstTurnContent{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m.tuiOwnerID = "owner-tui"
	if _, err := sessionSvc.AcquireLease(ctx, res.SessionID, m.tuiOwnerID); err != nil {
		t.Fatal(err)
	}
	m.heroChatSessionID = res.SessionID
	m.heroLeasedSessionID = res.SessionID
	var calls int
	m.leaseReleaseFn = func(context.Context, string, string) error {
		calls++
		if calls == 1 {
			return errors.New("lease release injected failure")
		}
		return sessionSvc.ReleaseLease(ctx, res.SessionID, m.tuiOwnerID)
	}
	next, _ := m.emptyChatAfterCurrentSessionMutation()
	if next.pendingLeaseReleaseID != res.SessionID {
		t.Fatalf("pending lease=%q", next.pendingLeaseReleaseID)
	}
	cmd := next.releaseHeroChatLeaseCmd(res.SessionID)
	if cmd == nil {
		t.Fatal("expected release cmd")
	}
	msg := cmd()
	fail, ok := msg.(sessionLeaseReleaseResultMsg)
	if !ok || fail.err == nil {
		t.Fatalf("msg=%T %+v", msg, msg)
	}
	_, held, err := sessionSvc.Store.GetSessionLease(res.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Fatal("database lease released despite failure")
	}
	updated, retry := next.handleConversationMsg(fail)
	got := updated.(model)
	if got.pendingLeaseReleaseID != res.SessionID {
		t.Fatalf("pending cleared on failure: %q", got.pendingLeaseReleaseID)
	}
	if retry == nil {
		t.Fatal("expected retry cmd")
	}
	retryMsg := sessionLeaseReleaseRetryMsg{sessionID: res.SessionID}
	updated, retryCmd := got.handleConversationMsg(retryMsg)
	got = updated.(model)
	if retryCmd == nil {
		t.Fatal("expected release retry cmd")
	}
	okMsg := retryCmd().(sessionLeaseReleaseResultMsg)
	if okMsg.err != nil {
		t.Fatalf("retry err=%v", okMsg.err)
	}
	updated, _ = got.handleConversationMsg(okMsg)
	got = updated.(model)
	if got.pendingLeaseReleaseID != "" {
		t.Fatalf("pending after success=%q", got.pendingLeaseReleaseID)
	}
	_, stillHeld, err := sessionSvc.Store.GetSessionLease(res.SessionID)
	if err != nil || stillHeld {
		t.Fatalf("lease should be released after retry held=%v err=%v", stillHeld, err)
	}
}

func TestDiagnosticLogsRedactRecoveryAttachImportExecute(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))

	secretID := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	opencodeID := "ses_opencodeleak1"
	pathLeak := "/home/user/.local/share/hero/sessions/" + secretID + "/assets/x.png"
	err := errors.New("provider failed session " + secretID + " native " + opencodeID + " path " + pathLeak)

	m := NewTestModel(nil)
	m.executes = map[string]convExecute{"ex-1": {ID: "ex-1"}}
	_, _ = m.handleConversationMsg(executeDoneMsg{executeID: "ex-1", err: err})
	_, _ = m.handleConversationMsg(sessionRecoverAttachDoneMsg{sessionID: secretID, err: err})
	m = SetScreen(m, ScreenHistory)
	_, _ = m.handleHistoryMsg(historyImportMsg{sessionID: secretID, err: err})
	_, _ = m.handleSessionRecoverStatus(sessionRecoverStatusMsg{sessionID: secretID, err: err, message: "recover failed"})

	out := buf.String()
	for _, leak := range []string{secretID, opencodeID, "/home/user", "assets/x.png"} {
		if strings.Contains(out, leak) {
			t.Fatalf("log leaked %q: %q", leak, out)
		}
	}
	for _, code := range []string{
		"tui conversation execute failed",
		"tui session recover live attach failed",
		"history remote import failed",
		"tui session recover status failed",
	} {
		if !strings.Contains(out, code) {
			t.Fatalf("missing log code %q in %q", code, out)
		}
	}
}

func TestLateStreamDeltaAfterCancelDoesNotMutateTranscript(t *testing.T) {
	m := NewTestModel(nil)
	m.streaming = true
	m.executeCancelled = false
	m.executes = map[string]convExecute{
		"ex-1": {ID: "ex-1", AgentMsgIndex: 1, HeroSessionID: "hero-1"},
	}
	m.transcript = []convMessage{{role: convRoleUser, content: "hi"}, {role: convRoleAgent, content: "partial"}}
	m.agentMsgIndex = 1
	next, cancelCmd := CancelConversationStreamForTest(m)
	if cancelCmd == nil {
		t.Fatal("expected cancel cmd")
	}
	_ = cancelCmd()
	updated, persistCmd := next.Update(streamDeltaMsg{
		executeID: "ex-1",
		delta:     harness.StreamDelta{Kind: harness.StreamKindText, Text: "late-token"},
	})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("model type %T", updated)
	}
	if persistCmd != nil {
		if msg := persistCmd(); msg != nil {
			if _, isOK := msg.(sessionPersistOKMsg); isOK {
				t.Fatal("late delta must not persist")
			}
		}
	}
	if strings.Contains(got.transcript[1].content, "late-token") {
		t.Fatalf("late delta applied after cancel: %q", got.transcript[1].content)
	}
}

func TestAssistantDeltaEnqueuePersistFIFO(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	res, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "t"}, conversation.FirstTurnContent{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m.heroChatSessionID = res.SessionID
	m.streaming = true
	m.executes = map[string]convExecute{
		"ex-1": {ID: "ex-1", AgentMsgIndex: 1, HeroSessionID: res.SessionID},
	}
	m.transcript = []convMessage{{role: convRoleUser, content: "hi"}, {role: convRoleAgent, content: ""}}
	m.agentMsgIndex = 1
	m.testPersistDelay = 40 * time.Millisecond
	updated, firstCmd := m.Update(streamDeltaMsg{
		executeID: "ex-1",
		delta:     harness.StreamDelta{Kind: harness.StreamKindText, Text: "Hel"},
	})
	got := updated.(model)
	got.testPersistDelay = 0
	updated, secondCmd := got.Update(streamDeltaMsg{
		executeID: "ex-1",
		delta:     harness.StreamDelta{Kind: harness.StreamKindText, Text: "lo"},
	})
	got = updated.(model)
	if firstCmd == nil || secondCmd == nil {
		t.Fatal("expected persist cmds for assistant deltas")
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = secondCmd()
	}()
	time.Sleep(10 * time.Millisecond)
	go func() {
		defer wg.Done()
		_ = firstCmd()
	}()
	wg.Wait()
	events, err := svc.Store.ListSessionEventsNewest(res.SessionID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	var assistant []string
	for _, ev := range events {
		if ev.EventType == store.SessionEventAssistant {
			assistant = append(assistant, ev.PayloadJSON)
		}
	}
	if len(assistant) != 1 || !strings.Contains(assistant[0], "Hello") {
		t.Fatalf("durable assistant snapshots=%v want one Hello", assistant)
	}
}

func drainPersistCmds(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	runAllTeaCmds(cmd)
}

func runAllTeaCmds(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	switch b := msg.(type) {
	case tea.BatchMsg:
		for _, nested := range b {
			runAllTeaCmds(nested)
		}
	}
}

func TestExecuteResultPersistUsesExecuteHeroSession(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	owned, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "owned"}, conversation.FirstTurnContent{Text: "a"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "other"}, conversation.FirstTurnContent{Text: "b"})
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m.heroChatSessionID = other.SessionID
	m.streaming = true
	m.executes = map[string]convExecute{
		"ex-1": {ID: "ex-1", AgentMsgIndex: 1, HeroSessionID: owned.SessionID, Freechat: true},
	}
	m.transcript = []convMessage{{role: convRoleUser, content: "a"}, {role: convRoleAgent, content: "done"}}
	m.agentMsgIndex = 1
	updated, persistCmd := m.Update(executeDoneMsg{
		executeID: "ex-1",
		harnessID: "cursor",
		modelSlug: "m",
		result:    &harness.ExecutionResult{Output: "done", SessionID: "native-owned"},
	})
	if _, ok := updated.(model); !ok {
		t.Fatalf("model type %T", updated)
	}
	drainPersistCmds(t, persistCmd)
	ownedSess, err := sessionSvc.GetSession(ctx, owned.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if ownedSess.NativeSessionID != "native-owned" {
		t.Fatalf("owned native=%q", ownedSess.NativeSessionID)
	}
	otherSess, err := sessionSvc.GetSession(ctx, other.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if otherSess.NativeSessionID != "" {
		t.Fatalf("global session mutated: %+v", otherSess)
	}
}

func TestCancelledFirstTurnPersistErrorIsRetained(t *testing.T) {
	m := NewTestModel(nil)
	m.executeCancelled = true
	events := []store.AppendSessionEventInput{{
		SessionID: "hero-1", BoundSessionID: "hero-1",
		EventType: store.SessionEventAttachment, Origin: store.SessionOriginLocal, PayloadJSON: `{"name":"a.png"}`,
	}}
	updated, _ := m.handleConversationMsg(sessionPersistErrMsg{err: errors.New("persist failed"), events: events})
	got := updated.(model)
	if !got.sessionPersistBlocked {
		t.Fatal("cancel must not drop persist retry payload")
	}
	if len(got.sessionPersistQueue) != 1 {
		t.Fatalf("retry events=%d", len(got.sessionPersistQueue))
	}
}
