package tui

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestFindQA19LateSessionBindingAfterCancelDoesNotMutateChat(t *testing.T) {
	m := NewTestModel(nil)
	m.executeCancelled = true
	m.executes = map[string]convExecute{}

	updated, _ := m.handleConversationMsg(heroSessionBoundMsg{
		executeID: "old-execute",
		sessionID: "late-session",
	})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("model type %T", updated)
	}
	if got.heroChatSessionID != "" {
		t.Fatalf("late binding repopulated cancelled chat: %q", got.heroChatSessionID)
	}
}

func TestFindQA20TUIDiagnosticLogsRedactRawErrors(t *testing.T) {
	for _, name := range []string{"app.go", "stage_handoff.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (method.Sel.Name != "Error" && method.Sel.Name != "Warn" && method.Sel.Name != "Info" && method.Sel.Name != "Debug") {
				return true
			}
			receiver, ok := method.X.(*ast.Ident)
			if !ok || receiver.Name != "slog" {
				return true
			}
			for i := 1; i+1 < len(call.Args); i += 2 {
				key, ok := call.Args[i].(*ast.BasicLit)
				if !ok || key.Kind != token.STRING {
					continue
				}
				attr, _ := strconv.Unquote(key.Value)
				if attr == "error" && !qa20RedactedDiagnosticExpr(call.Args[i+1]) {
					t.Errorf("%s passes an unredacted error to slog", name)
				}
			}
			return true
		})
	}
}

func qa20RedactedDiagnosticExpr(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Error" {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && receiver.Name == "redact"
}

func TestFindQA25FirstTurnLeaseFailureRetainsWholeRetryPayload(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	m := NewTestModel(svc)
	m.tuiOwnerID = ""
	ex := convExecute{Attachments: []harness.Attachment{{
		Name: "shot.png", Path: "/tmp/shot.png", MIMEType: "image/png", ContentHash: "hash-shot",
	}}}

	_, err := m.persistUserTurnBeforeExecute(context.Background(), ex, "with image", convRoleUser, "cursor", "model", nil)
	if err == nil {
		t.Fatal("expected lease acquisition failure")
	}
	var pe *sessionTurnPersistError
	if !errors.As(err, &pe) {
		t.Fatalf("lease failure lost retry envelope: %T %v", err, err)
	}
	if strings.TrimSpace(pe.sessionID) == "" || len(pe.events) < 2 || len(pe.assets) != 1 {
		t.Fatalf("incomplete retry payload: session=%q events=%d assets=%d", pe.sessionID, len(pe.events), len(pe.assets))
	}
	for _, event := range pe.events {
		if event.SessionID != pe.sessionID || event.BoundSessionID != pe.sessionID {
			t.Fatalf("event is not retry-routable: %+v", event)
		}
	}
	if pe.assets[0].sessionID != pe.sessionID {
		t.Fatalf("asset is not retry-routable: %+v", pe.assets[0])
	}
}

func TestFindQA27FinalSuffixFailureIsAtomicPerExecute(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	first, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "first"}, conversation.FirstTurnContent{Text: "first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "second"}, conversation.FirstTurnContent{Text: "second"})
	if err != nil {
		t.Fatal(err)
	}

	msg := persistSessionSuffix(ctx, sessionSvc, nil, []store.AppendSessionEventInput{
		{SessionID: first.SessionID, BoundSessionID: first.SessionID, EventType: store.SessionEventAssistant, Origin: store.SessionOriginLocal, PayloadJSON: `{"text":"first result"}`, ProviderEventID: "result-first"},
		{SessionID: second.SessionID, BoundSessionID: second.SessionID, EventType: store.SessionEventAssistant, Origin: store.SessionOriginLocal, PayloadJSON: `{"text":"second result"}`, ProviderEventID: "result-second"},
	}, nil, nil, []sessionNativeBindPersistItem{
		{heroID: first.SessionID, harnessID: "cursor", nativeID: "native-shared", modelSlug: "model"},
		{heroID: second.SessionID, harnessID: "cursor", nativeID: "native-shared", modelSlug: "model"},
	})
	failed, ok := msg.(sessionPersistErrMsg)
	if !ok || failed.err == nil {
		t.Fatalf("expected second native bind failure, got %T %+v", msg, msg)
	}

	events, err := sessionSvc.ListSessionEventsNewest(ctx, second.SessionID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.EventType == store.SessionEventAssistant {
			t.Fatalf("second execution suffix committed without its binding: %+v", event)
		}
	}
	got, err := sessionSvc.GetSession(ctx, second.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if got.NativeSessionID != "" {
		t.Fatalf("second execution binding committed after failure: %+v", got)
	}
}

func TestFindQA28AttributedStreamIsQueuedForExecutingSession(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	res, err := sessionSvc.EnsureFirstTurn(context.Background(), "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "chat"}, conversation.FirstTurnContent{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m.heroChatSessionID = ""
	m.streaming = true
	m.executes = map[string]convExecute{"ex-1": {ID: "ex-1", AgentMsgIndex: 1, HeroSessionID: res.SessionID}}
	m.transcript = []convMessage{{role: convRoleUser, content: "hi"}, {role: convRoleAgent, content: "parent"}}
	m.agentMsgIndex = 1

	m.applyStreamDelta(streamDeltaMsg{executeID: "ex-1", delta: harness.StreamDelta{
		Kind: harness.StreamKindText, Text: "child-token", CallID: "child-call", AgentName: "context_agent", Model: "model",
	}})
	for _, event := range m.sessionPersistQueue {
		if event.SessionID == res.SessionID && strings.Contains(event.PayloadJSON, "child-token") {
			return
		}
	}
	t.Fatal("visible attributed stream row was not queued for the executing Hero session")
}

func TestFindQA29RetryDoesNotReleaseReacquiredLease(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	sessionSvc := conversation.NewSessionService(svc.Store, store.DefaultClock())
	ctx := context.Background()
	res, err := sessionSvc.EnsureFirstTurn(ctx, "", conversation.CreateSessionParams{Kind: store.SessionKindFreechat, Title: "chat"}, conversation.FirstTurnContent{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}

	m := NewTestModel(svc)
	m.sessionService = sessionSvc
	m.tuiOwnerID = "owner"
	m.heroChatSessionID = res.SessionID
	m.heroLeasedSessionID = res.SessionID
	if _, err := sessionSvc.AcquireLease(ctx, res.SessionID, m.tuiOwnerID); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	allow := make(chan struct{})
	var once sync.Once
	m.leaseReleaseFn = func(_ context.Context, sessionID, owner string) error {
		once.Do(func() { close(started) })
		<-allow
		return sessionSvc.ReleaseLease(context.Background(), sessionID, owner)
	}
	allowRelease := func() {
		select {
		case <-allow:
		default:
			close(allow)
		}
	}
	defer allowRelease()

	next, _ := m.emptyChatAfterCurrentSessionMutation()
	releaseCmd := next.releaseHeroChatLeaseCmd(res.SessionID)
	if releaseCmd == nil {
		t.Fatal("expected delayed release command")
	}
	releaseResult := make(chan tea.Msg, 1)
	go func() { releaseResult <- releaseCmd() }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("release command did not start")
	}

	openMsg := next.historyOpenCmd(res.SessionID, false, "")()
	openedModel, _ := next.handleHistoryMsg(openMsg)
	opened, ok := openedModel.(model)
	if !ok || opened.heroLeasedSessionID != res.SessionID {
		t.Fatalf("session was not reopened: %T %+v", openedModel, openedModel)
	}
	allowRelease()

	var releaseMsg tea.Msg
	select {
	case releaseMsg = <-releaseResult:
	case <-time.After(time.Second):
		t.Fatal("release command did not finish")
	}
	_, _ = opened.handleConversationMsg(releaseMsg)
	_, held, err := sessionSvc.Store.GetSessionLease(res.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Fatal("stale cleanup released the lease reacquired by the reopened chat")
	}
}

func TestFirstTurnLeaseFailureRetryRestoresSessionLease(t *testing.T) {
	svc := newTestServiceInstalledNoCycle(t, t.TempDir())
	m := NewTestModel(svc)
	m.tuiOwnerID = ""
	ex := convExecute{Attachments: []harness.Attachment{{
		Name: "shot.png", Path: "/tmp/shot.png", MIMEType: "image/png", ContentHash: "hash-shot",
	}}}

	_, err := m.persistUserTurnBeforeExecute(context.Background(), ex, "with image", convRoleUser, "cursor", "model", nil)
	if err == nil {
		t.Fatal("expected lease acquisition failure")
	}
	var pe *sessionTurnPersistError
	if !errors.As(err, &pe) {
		t.Fatalf("lease failure lost retry envelope: %T %v", err, err)
	}
	if strings.TrimSpace(pe.sessionID) == "" {
		t.Fatal("retry envelope lost session id")
	}

	updated, _ := m.handleConversationMsg(sessionPersistErrMsg{
		err: pe.err, events: pe.events, assets: pe.assets,
		bindings: pe.bindings, nativeBinds: pe.nativeBinds, serialization: pe.serialization,
	})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("model type %T", updated)
	}
	got.tuiOwnerID = "owner"
	got, retryCmd := got.drainSessionPersistCmd()
	if retryCmd == nil {
		t.Fatal("expected retry command")
	}
	updated, _ = got.handleConversationMsg(retryCmd())
	got, ok = updated.(model)
	if !ok {
		t.Fatalf("retry model type %T", updated)
	}
	if got.heroChatSessionID != pe.sessionID {
		t.Fatalf("retry lost Hero session id: got %q want %q", got.heroChatSessionID, pe.sessionID)
	}
	_, held, err := got.sessionService.Store.GetSessionLease(pe.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Fatal("retry did not reacquire the first-turn lease")
	}
}
