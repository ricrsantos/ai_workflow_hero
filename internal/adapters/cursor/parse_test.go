package cursor_test

import (
	"strings"
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestParseJSONResult(t *testing.T) {
	raw := `{"type":"result","subtype":"success","is_error":false,"duration_ms":100,"result":"Done.\nMore","session_id":"abc","usage":{"input_tokens":7,"output_tokens":2}}`
	res, err := cursoradapter.ParseJSONResult([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "abc" || res.Output != "Done.\nMore" || res.Summary != "Done." {
		t.Fatalf("%+v", res)
	}
	if res.Usage.InputTokens != 7 || res.Usage.OutputTokens != 2 {
		t.Fatalf("usage=%+v", res.Usage)
	}
}

func TestParseStreamJSONPartialSkipsDuplicates(t *testing.T) {
	ndjson := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"s"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hel"}]},"session_id":"s","timestamp_ms":1}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"lo"}]},"session_id":"s","timestamp_ms":2}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello"}]},"session_id":"s","timestamp_ms":3,"model_call_id":"m1"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello"}]},"session_id":"s"}`,
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":5,"result":"Hello","session_id":"s"}`,
	}, "\n") + "\n"

	var deltas []string
	res, err := cursoradapter.ParseStreamJSON(strings.NewReader(ndjson), func(d harness.StreamDelta) {
		if d.Kind == harness.StreamKindText || d.Kind == "" {
			deltas = append(deltas, d.Text)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "Hello" {
		t.Fatalf("output=%q", res.Output)
	}
	if strings.Join(deltas, "") != "Hello" {
		t.Fatalf("deltas=%v", deltas)
	}
}

func TestParseStreamJSONThinkingAndToolCalls(t *testing.T) {
	ndjson := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"s"}`,
		`{"type":"thinking","subtype":"delta","text":"The user ","session_id":"s"}`,
		`{"type":"thinking","subtype":"delta","text":"wants a summary.","session_id":"s"}`,
		`{"type":"thinking","subtype":"completed","session_id":"s"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"I will read "}]},"session_id":"s","timestamp_ms":1}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"the file."}]},"session_id":"s","timestamp_ms":2}`,
		`{"type":"tool_call","subtype":"started","call_id":"c1","tool_call":{"readToolCall":{"args":{"path":"README.md"}}},"session_id":"s"}`,
		`{"type":"tool_call","subtype":"completed","call_id":"c1","tool_call":{"readToolCall":{"args":{"path":"README.md"},"result":{"success":{"content":"# Hi"}}}},"session_id":"s"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","text":"Looks good. "},{"type":"text","text":"Done."}]},"session_id":"s","timestamp_ms":3}`,
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":5,"result":"I will read the file.Done.","session_id":"s"}`,
	}, "\n") + "\n"

	var kinds []string
	var texts []string
	res, err := cursoradapter.ParseStreamJSON(strings.NewReader(ndjson), func(d harness.StreamDelta) {
		kinds = append(kinds, string(d.Kind))
		texts = append(texts, d.Text)
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "I will read the file.Done." {
		t.Fatalf("output=%q", res.Output)
	}
	wantKinds := []string{"thinking", "thinking", "text", "text", "tool", "thinking", "text"}
	wantTexts := []string{"The user ", "wants a summary.", "I will read ", "the file.", "Read README.md", "Looks good. ", "Done."}
	if strings.Join(kinds, "|") != strings.Join(wantKinds, "|") {
		t.Fatalf("kinds=%v want %v", kinds, wantKinds)
	}
	if strings.Join(texts, "|") != strings.Join(wantTexts, "|") {
		t.Fatalf("texts=%v want %v", texts, wantTexts)
	}
}

func TestParseStreamJSONTaskToolCall(t *testing.T) {
	ndjson := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"s"}`,
		`{"type":"tool_call","subtype":"started","call_id":"t1","tool_call":{"taskToolCall":{"args":{"description":"frontend_agent"}}},"session_id":"s"}`,
		`{"type":"tool_call","subtype":"completed","call_id":"t1","tool_call":{"taskToolCall":{"args":{"description":"frontend_agent"},"result":{"success":{"content":"{\"stage\":\"implementation\"}"}}}},"session_id":"s"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Implementation complete."}]},"session_id":"s","timestamp_ms":1}`,
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":5,"result":"Implementation complete.","session_id":"s"}`,
	}, "\n") + "\n"

	var texts []string
	_, err := cursoradapter.ParseStreamJSON(strings.NewReader(ndjson), func(d harness.StreamDelta) {
		if d.Kind == harness.StreamKindTool || d.Kind == harness.StreamKindText {
			texts = append(texts, d.Text)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(texts, "|")
	if !strings.Contains(joined, "Task frontend_agent") {
		t.Fatalf("missing Task start: %q", joined)
	}
	if !strings.Contains(joined, "Task frontend_agent (completed)") {
		t.Fatalf("missing Task completed: %q", joined)
	}
}

func TestIsAuthFailure(t *testing.T) {
	if !cursoradapter.IsAuthFailure("", "Please log in with cursor agent login") {
		t.Fatal("expected auth failure")
	}
	if cursoradapter.IsAuthFailure("ok", "") {
		t.Fatal("unexpected auth failure")
	}
}

func TestIsTrustFailure(t *testing.T) {
	if !cursoradapter.IsTrustFailure("", "⚠ Workspace Trust Required") {
		t.Fatal("expected trust failure")
	}
	if cursoradapter.IsTrustFailure("ok", "") {
		t.Fatal("unexpected trust failure")
	}
}
