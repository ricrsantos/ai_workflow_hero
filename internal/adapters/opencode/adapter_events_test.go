package opencode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestModelPayload(t *testing.T) {
	got := modelPayload("opencode-go/grok-4.5")
	if got["providerID"] != "opencode-go" || got["modelID"] != "grok-4.5" {
		t.Fatalf("payload=%v", got)
	}
	if modelPayload("") != nil {
		t.Fatal("empty slug should be nil")
	}
}

func TestParseEventDeltaAssistantText(t *testing.T) {
	state := newStreamState()
	evt := map[string]any{
		"type": "message.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"info": map[string]any{
				"id":   "msg-asst",
				"role": "assistant",
			},
		},
	}
	parseEventDelta(evt, "sess-1", state)

	evt = map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"part": map[string]any{
				"id":        "prt-1",
				"type":      "text",
				"text":      "Hi",
				"messageID": "msg-asst",
			},
		},
	}
	text, done := parseEventDelta(evt, "sess-1", state)
	if done || text != "Hi" {
		t.Fatalf("text=%q done=%v", text, done)
	}

	evt = map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"part": map[string]any{
				"id":        "prt-1",
				"type":      "text",
				"text":      "Hi!",
				"messageID": "msg-asst",
			},
		},
	}
	text, done = parseEventDelta(evt, "sess-1", state)
	if done || text != "!" {
		t.Fatalf("delta=%q done=%v", text, done)
	}

	evt = map[string]any{
		"type":       "session.idle",
		"properties": map[string]any{"sessionID": "sess-1"},
	}
	_, done = parseEventDelta(evt, "sess-1", state)
	if !done {
		t.Fatal("expected done on session.idle")
	}
}

func TestParseEventDeltaIgnoresUserParts(t *testing.T) {
	state := newStreamState()
	state.assistantMsgID = "msg-asst"
	evt := map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"part": map[string]any{
				"id":        "prt-user",
				"type":      "text",
				"text":      "user prompt",
				"messageID": "msg-user",
			},
		},
	}
	text, done := parseEventDelta(evt, "sess-1", state)
	if text != "" || done {
		t.Fatalf("user part leaked with assistant id: text=%q done=%v", text, done)
	}

	state.assistantMsgID = ""
	text, done = parseEventDelta(evt, "sess-1", state)
	if text != "" || done {
		t.Fatalf("user part leaked before assistant id: text=%q done=%v", text, done)
	}
}

func TestExecHandleScanServeOutput(t *testing.T) {
	h := &execHandle{urlReady: make(chan struct{})}
	go h.scanServeOutput(strings.NewReader("Warning: foo\nopencode server listening on http://127.0.0.1:45123\n"))
	<-h.urlReady
	if h.baseURL != "http://127.0.0.1:45123" || h.port != 45123 {
		t.Fatalf("url=%q port=%d err=%v", h.baseURL, h.port, h.urlErr)
	}
}

func TestProcessSSEEventToolAndWarning(t *testing.T) {
	state := newStreamState()
	var kinds []string
	a := &Adapter{ProjectDir: t.TempDir()}
	emit := func(d harness.StreamDelta) {
		kinds = append(kinds, string(d.Kind))
	}
	req := harness.ExecuteRequest{OnStreamDelta: emit}

	toolCalled := map[string]any{
		"type": "session.next.tool.called",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"callID":    "c1",
			"tool":      "read",
		},
	}
	out := a.processSSEEvent(context.Background(), toolCalled, "sess-1", state, req, nil)
	if out.done || out.err != nil {
		t.Fatalf("tool.called: done=%v err=%v", out.done, out.err)
	}

	unknown := map[string]any{"type": "future.event", "properties": map[string]any{"sessionID": "sess-1"}}
	out = a.processSSEEvent(context.Background(), unknown, "sess-1", state, req, nil)
	if out.err != nil {
		t.Fatal(out.err)
	}

	if len(kinds) < 2 || kinds[0] != string(harness.StreamKindTool) || kinds[1] != string(harness.StreamKindWarning) {
		t.Fatalf("kinds=%v", kinds)
	}
}

func TestProcessSSEEventSessionNextTextDelta(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	delta := map[string]any{
		"type": "session.next.text.delta",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"textID":    "txt-1",
			"delta":     "Hel",
		},
	}
	if out := a.processSSEEvent(context.Background(), delta, "sess-1", state, req, nil); out.err != nil {
		t.Fatalf("delta: %v", out.err)
	}
	if text != "" {
		t.Fatalf("delta alone should not emit: %q", text)
	}
	ended := map[string]any{
		"type": "session.next.text.ended",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"textID":    "txt-1",
			"text":      "Hello world",
		},
	}
	if out := a.processSSEEvent(context.Background(), ended, "sess-1", state, req, nil); out.err != nil {
		t.Fatalf("ended: %v", out.err)
	}
	if text != "Hello world" {
		t.Fatalf("text=%q", text)
	}
}

func TestProcessSSEEventSyncUnwrap(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	evt := map[string]any{
		"type": "sync",
		"properties": map[string]any{
			"syncEvent": map[string]any{
				"type": "session.next.text.ended",
				"data": map[string]any{
					"sessionID": "sess-1",
					"textID":    "txt-1",
					"text":      "via sync",
				},
			},
		},
	}
	out := a.processSSEEvent(context.Background(), evt, "sess-1", state, req, nil)
	if out.done || out.err != nil {
		t.Fatalf("done=%v err=%v", out.done, out.err)
	}
	if text != "via sync" {
		t.Fatalf("text=%q", text)
	}
}

func TestProcessSSEEventPermission(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/permission/perm-1/reply" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.baseURL = srv.URL
	state := newStreamState()
	asked := false
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {},
		OnPermissionRequest: func(_ context.Context, pr harness.PermissionRequest) (harness.PermissionResponse, error) {
			asked = true
			if pr.ID != "perm-1" {
				t.Fatalf("id=%q", pr.ID)
			}
			return harness.PermissionResponse{Approved: true}, nil
		},
	}
	evt := map[string]any{
		"type": "permission.asked",
		"properties": map[string]any{
			"sessionID":  "sess-1",
			"id":         "perm-1",
			"permission": "bash",
			"patterns":   []any{"npm test"},
		},
	}
	if out := a.processSSEEvent(context.Background(), evt, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	if !asked {
		t.Fatal("expected permission callback")
	}
}

func TestProcessSSEEventSessionError(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	evt := map[string]any{
		"type": "session.error",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"error":     "boom",
		},
	}
	out := a.processSSEEvent(context.Background(), evt, "sess-1", state, harness.ExecuteRequest{}, nil)
	if out.err == nil {
		t.Fatal("expected error")
	}
}

func TestDebugOnlyActivitySuppressed(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	var kinds []string
	emit := func(d harness.StreamDelta) {
		kinds = append(kinds, string(d.Kind))
	}
	req := harness.ExecuteRequest{OnStreamDelta: emit, Debug: false}
	evt := map[string]any{
		"type": "session.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"title":     "renamed",
		},
	}
	out := a.processSSEEvent(context.Background(), evt, "sess-1", state, req, nil)
	if out.err != nil {
		t.Fatal(out.err)
	}
	if len(kinds) != 0 {
		t.Fatalf("expected suppression, got kinds=%v", kinds)
	}

	req.Debug = true
	out = a.processSSEEvent(context.Background(), evt, "sess-1", state, req, nil)
	if out.err != nil {
		t.Fatal(out.err)
	}
	if len(kinds) != 1 || kinds[0] != string(harness.StreamKindActivity) {
		t.Fatalf("expected activity in debug, kinds=%v", kinds)
	}
}

func TestCatalogUpdatedUserSummaryWithoutDebug(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	var text string
	req := harness.ExecuteRequest{
		Debug: false,
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindActivity {
				text = d.Text
			}
		},
	}
	evt := map[string]any{
		"type": "catalog.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"title":     "Models",
			"items":     []any{"gpt-5", "claude-4"},
		},
	}
	out := a.processSSEEvent(context.Background(), evt, "sess-1", state, req, nil)
	if out.err != nil {
		t.Fatal(out.err)
	}
	if !strings.Contains(text, "Models") || !strings.Contains(text, "gpt-5") {
		t.Fatalf("catalog summary=%q", text)
	}
}

func TestStepBoundaryFilteringWithoutDebug(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	state.assistantMsgID = "msg-asst"
	var texts []string
	req := harness.ExecuteRequest{
		Debug: false,
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				texts = append(texts, d.Text)
			}
		},
	}

	start := map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"part": map[string]any{
				"type":      "step-start",
				"messageID": "msg-asst",
			},
		},
	}
	if out := a.processSSEEvent(context.Background(), start, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}

	inside := map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"part": map[string]any{
				"id":        "prt-1",
				"type":      "text",
				"text":      "inside step",
				"messageID": "msg-asst",
			},
		},
	}
	if out := a.processSSEEvent(context.Background(), inside, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}

	finish := map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"part": map[string]any{
				"type":      "step-finish",
				"messageID": "msg-asst",
			},
		},
	}
	if out := a.processSSEEvent(context.Background(), finish, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}

	outside := map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"part": map[string]any{
				"id":        "prt-2",
				"type":      "text",
				"text":      "after step",
				"messageID": "msg-asst",
			},
		},
	}
	if out := a.processSSEEvent(context.Background(), outside, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}

	got := strings.Join(texts, "")
	if got != "inside step after step" {
		t.Fatalf("text=%q", got)
	}
}

func TestEmitTextDeltaInsertsPartGap(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	emitPart := func(partID, content string) {
		evt := map[string]any{
			"type": "message.part.updated",
			"properties": map[string]any{
				"sessionID": "sess-1",
				"part": map[string]any{
					"id":        partID,
					"type":      "text",
					"text":      content,
					"messageID": "msg-asst",
				},
			},
		}
		state.assistantMsgID = "msg-asst"
		if out := a.processSSEEvent(context.Background(), evt, "sess-1", state, req, nil); out.err != nil {
			t.Fatal(out.err)
		}
	}
	emitPart("prt-1", "The")
	emitPart("prt-2", "user")
	emitPart("prt-3", "is")
	emitPart("prt-4", "asking")
	if text != "The user is asking" {
		t.Fatalf("text=%q", text)
	}
}

func TestEmitTextDeltaNoGapWithinStreamingPart(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	state.assistantMsgID = "msg-asst"
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	for _, chunk := range []string{"c", "co", "con", "conc", "concisely"} {
		evt := map[string]any{
			"type": "message.part.updated",
			"properties": map[string]any{
				"sessionID": "sess-1",
				"part": map[string]any{
					"id":        "prt-1",
					"type":      "text",
					"text":      chunk,
					"messageID": "msg-asst",
				},
			},
		}
		if out := a.processSSEEvent(context.Background(), evt, "sess-1", state, req, nil); out.err != nil {
			t.Fatal(out.err)
		}
	}
	if text != "concisely" {
		t.Fatalf("text=%q", text)
	}
}

func TestPartBoundarySpaceAfterPunctuation(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	state.assistantMsgID = "msg-asst"
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	emit := func(partID, content string) {
		evt := map[string]any{
			"type": "message.part.updated",
			"properties": map[string]any{
				"sessionID": "sess-1",
				"part": map[string]any{
					"id":        partID,
					"type":      "text",
					"text":      content,
					"messageID": "msg-asst",
				},
			},
		}
		if out := a.processSSEEvent(context.Background(), evt, "sess-1", state, req, nil); out.err != nil {
			t.Fatal(out.err)
		}
	}
	emit("prt-1", "What model are you?")
	emit("prt-2", "What harness")
	if text != "What model are you? What harness" {
		t.Fatalf("text=%q", text)
	}
}

func TestReasoningEndedAuthoritative(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	var thinking string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindThinking {
				thinking += d.Text
			}
		},
	}
	delta := map[string]any{
		"type": "session.next.reasoning.delta",
		"properties": map[string]any{
			"sessionID":   "sess-1",
			"reasoningID": "rsn-1",
			"delta":       "Theuserisasking",
		},
	}
	if out := a.processSSEEvent(context.Background(), delta, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	if thinking != "" {
		t.Fatalf("delta should not emit: %q", thinking)
	}
	ended := map[string]any{
		"type": "session.next.reasoning.ended",
		"properties": map[string]any{
			"sessionID":   "sess-1",
			"reasoningID": "rsn-1",
			"text":        "The user is asking in Portuguese.",
		},
	}
	if out := a.processSSEEvent(context.Background(), ended, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	if thinking != "The user is asking in Portuguese." {
		t.Fatalf("thinking=%q", thinking)
	}
}

func TestMessagePartTextStillStreamsWithSessionNext(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	updated := map[string]any{
		"type": "message.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"info": map[string]any{
				"id":   "msg-asst",
				"role": "assistant",
			},
		},
	}
	if out := a.processSSEEvent(context.Background(), updated, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	part := map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"part": map[string]any{
				"id":        "prt-1",
				"type":      "text",
				"text":      "Resposta completa.",
				"messageID": "msg-asst",
			},
		},
	}
	if out := a.processSSEEvent(context.Background(), part, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	if text != "Resposta completa." {
		t.Fatalf("text=%q", text)
	}
	if state.assistantMsgID != "msg-asst" {
		t.Fatalf("assistantMsgID=%q", state.assistantMsgID)
	}
}

func TestSessionNextSetsAssistantIDBeforeMessagePart(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	started := map[string]any{
		"type": "session.next.text.started",
		"properties": map[string]any{
			"sessionID":          "sess-1",
			"textID":             "prt-1",
			"assistantMessageID": "msg-asst",
		},
	}
	if out := a.processSSEEvent(context.Background(), started, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	part := map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"part": map[string]any{
				"id":        "prt-1",
				"type":      "text",
				"text":      "Resposta via part.",
				"messageID": "msg-asst",
			},
		},
	}
	if out := a.processSSEEvent(context.Background(), part, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	if text != "Resposta via part." {
		t.Fatalf("text=%q", text)
	}
}

func TestSessionNextAndMessagePartDedup(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	updated := map[string]any{
		"type": "message.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"info": map[string]any{
				"id":   "msg-asst",
				"role": "assistant",
			},
		},
	}
	if out := a.processSSEEvent(context.Background(), updated, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	_ = a.processSSEEvent(context.Background(), map[string]any{
		"type": "session.next.text.delta",
		"properties": map[string]any{
			"sessionID":          "sess-1",
			"textID":             "prt-1",
			"assistantMessageID": "msg-asst",
			"delta":              "Resposta",
		},
	}, "sess-1", state, req, nil)
	part := map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"part": map[string]any{
				"id":        "prt-1",
				"type":      "text",
				"text":      "Resposta completa.",
				"messageID": "msg-asst",
			},
		},
	}
	if out := a.processSSEEvent(context.Background(), part, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	if text != "Resposta completa." {
		t.Fatalf("text=%q", text)
	}
}

func TestTextEndedRecoversAfterDivergentDeltaAccumulation(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	state.assistantMsgID = "msg-asst"
	state.partTexts["text:txt-1"] = strings.Repeat("x", 80)
	state.emittedText["emitted:text:txt-1"] = strings.Repeat("a", 60)
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	ended := map[string]any{
		"type": "session.next.text.ended",
		"properties": map[string]any{
			"sessionID":          "sess-1",
			"textID":             "txt-1",
			"assistantMessageID": "msg-asst",
			"text":               "Você está no projeto AI Workflow Hero.",
		},
	}
	if out := a.processSSEEvent(context.Background(), ended, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	if text != "Você está no projeto AI Workflow Hero." {
		t.Fatalf("text=%q", text)
	}
}

func TestFlushPendingTextOnSessionIdle(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	state.assistantMsgID = "msg-asst"
	var text string
	req := harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				text += d.Text
			}
		},
	}
	state.partTexts["text:txt-1"] = "partial full answer"
	idle := map[string]any{
		"type":       "session.idle",
		"properties": map[string]any{"sessionID": "sess-1"},
	}
	out := a.processSSEEvent(context.Background(), idle, "sess-1", state, req, nil)
	if !out.done {
		t.Fatal("expected done")
	}
	if text != "partial full answer" {
		t.Fatalf("flush text=%q", text)
	}
}

func TestSessionDiffSuppressedWithoutDebug(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	var kinds []string
	req := harness.ExecuteRequest{
		Debug: false,
		OnStreamDelta: func(d harness.StreamDelta) {
			kinds = append(kinds, string(d.Kind))
		},
	}
	evt := map[string]any{
		"type": "session.diff",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"diff":      "file.go +1",
		},
	}
	if out := a.processSSEEvent(context.Background(), evt, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	if len(kinds) != 0 {
		t.Fatalf("expected suppression, kinds=%v", kinds)
	}
}

func TestToolPartSuppressedWithoutDebug(t *testing.T) {
	a := &Adapter{}
	state := newStreamState()
	state.assistantMsgID = "msg-asst"
	var kinds []string
	req := harness.ExecuteRequest{
		Debug: false,
		OnStreamDelta: func(d harness.StreamDelta) {
			kinds = append(kinds, string(d.Kind))
		},
	}
	evt := map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"sessionID": "sess-1",
			"part": map[string]any{
				"type":      "tool",
				"tool":      "read",
				"messageID": "msg-asst",
			},
		},
	}
	if out := a.processSSEEvent(context.Background(), evt, "sess-1", state, req, nil); out.err != nil {
		t.Fatal(out.err)
	}
	if len(kinds) != 0 {
		t.Fatalf("expected suppression, kinds=%v", kinds)
	}
}
