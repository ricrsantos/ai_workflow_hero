package opencode

import (
	"strings"
	"testing"
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
	partTexts := map[string]string{}
	assistantID := ""
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
	_, _, assistantID = parseEventDelta(evt, "sess-1", partTexts, assistantID)

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
	text, done, _ := parseEventDelta(evt, "sess-1", partTexts, assistantID)
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
	text, done, _ = parseEventDelta(evt, "sess-1", partTexts, assistantID)
	if done || text != "!" {
		t.Fatalf("delta=%q done=%v", text, done)
	}

	evt = map[string]any{
		"type":       "session.idle",
		"properties": map[string]any{"sessionID": "sess-1"},
	}
	_, done, _ = parseEventDelta(evt, "sess-1", partTexts, assistantID)
	if !done {
		t.Fatal("expected done on session.idle")
	}
}

func TestParseEventDeltaIgnoresUserParts(t *testing.T) {
	texts := make(map[string]string)
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
	text, done, _ := parseEventDelta(evt, "sess-1", texts, "msg-asst")
	if text != "" || done {
		t.Fatalf("user part leaked with assistant id: text=%q done=%v", text, done)
	}

	// Real OpenCode order: user part before assistant message.updated.
	text, done, _ = parseEventDelta(evt, "sess-1", texts, "")
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
