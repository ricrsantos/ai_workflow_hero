package cursor

import (
	"context"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestParseStreamJSONPermissionRequest(t *testing.T) {
	ndjson := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"s"}`,
		`{"type":"permission_request","id":"call-1","name":"bash","reason":"run npm test","sideEffect":"shell","session_id":"s"}`,
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"result":"ok","session_id":"s"}`,
	}, "\n") + "\n"

	asked := false
	_, err := ParseStreamJSONWithOptions(context.Background(), strings.NewReader(ndjson), StreamParseOptions{
		OnDelta: func(d harness.StreamDelta) {},
		OnPermissionRequest: func(_ context.Context, pr harness.PermissionRequest) (harness.PermissionResponse, error) {
			asked = true
			if pr.ID != "call-1" || pr.Title != "bash" {
				t.Fatalf("req=%+v", pr)
			}
			if !strings.Contains(pr.Description, "npm test") {
				t.Fatalf("description=%q", pr.Description)
			}
			return harness.PermissionResponse{Approved: true}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !asked {
		t.Fatal("expected permission callback")
	}
}

func TestParseStreamJSONPermissionWithoutHandler(t *testing.T) {
	ndjson := `{"type":"permission_request","id":"call-1","name":"bash","reason":"run npm test","session_id":"s"}` + "\n"
	_, err := ParseStreamJSON(strings.NewReader(ndjson), nil)
	if err == nil {
		t.Fatal("expected error without OnPermissionRequest")
	}
}

func TestParseStreamJSONToolPermissionDenied(t *testing.T) {
	ndjson := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"s"}`,
		`{"type":"tool_call","subtype":"completed","call_id":"c1","tool_call":{"shellToolCall":{"args":{"command":"npm test"},"result":{"error":{"message":"User rejected the MCP call"}}}},"session_id":"s"}`,
		`{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"result":"ok","session_id":"s"}`,
	}, "\n") + "\n"

	var warnings []string
	_, err := ParseStreamJSON(strings.NewReader(ndjson), func(d harness.StreamDelta) {
		if d.Kind == harness.StreamKindWarning {
			warnings = append(warnings, d.Text)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected permission denied warning")
	}
}
