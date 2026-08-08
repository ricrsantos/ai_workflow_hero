package cursor_test

import (
	"strings"
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
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
	res, err := cursoradapter.ParseStreamJSON(strings.NewReader(ndjson), func(d string) {
		deltas = append(deltas, d)
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

func TestIsAuthFailure(t *testing.T) {
	if !cursoradapter.IsAuthFailure("", "Please log in with cursor agent login") {
		t.Fatal("expected auth failure")
	}
	if cursoradapter.IsAuthFailure("ok", "") {
		t.Fatal("unexpected auth failure")
	}
}
