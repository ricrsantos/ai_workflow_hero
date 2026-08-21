package codex

import (
	"context"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestNativePropertyOptions(t *testing.T) {
	got := nativePropertyOptions(map[string]string{
		harness.PropertyEffort: "high",
		harness.PropertyThink:  "max",
		harness.PropertyFast:   "true",
		"na":                   "x",
	})
	if got["effort"] != "high" {
		t.Fatalf("effort=%v", got["effort"])
	}
	if got["summary"] != "detailed" {
		t.Fatalf("summary=%v", got["summary"])
	}
	if _, ok := got["fast"]; ok {
		t.Fatal("fs must be omitted (na)")
	}
}

func TestNativePropertyOptionsDropsNA(t *testing.T) {
	got := nativePropertyOptions(map[string]string{
		harness.PropertyEffort: "na",
		harness.PropertyThink:  "na",
	})
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestMapPropertyRejection(t *testing.T) {
	err := mapPropertyRejection(
		&rpcError{Message: "invalid effort for model"},
		"gpt-5.4",
		map[string]string{harness.PropertyEffort: "ultra"},
	)
	if !harness.IsPropertyRejection(err) {
		t.Fatalf("want PropertyRejectionError, got %T %v", err, err)
	}
}

func TestWarningDeltaUnknownEvent(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	var saw harness.StreamDelta
	out := a.handleNotification(context.Background(), "totally/unknown", []byte(`{"x":1,"secret":"raw-json"}`), "thr", harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) { saw = d },
	}, nil)
	if out.done || out.err != nil {
		t.Fatalf("unexpected outcome %+v", out)
	}
	if saw.Kind != harness.StreamKindWarning {
		t.Fatalf("kind=%s", saw.Kind)
	}
	if !strings.Contains(saw.Text, "totally/unknown") {
		t.Fatalf("warning must name event type: %q", saw.Text)
	}
	if strings.Contains(saw.Text, "{") || strings.Contains(saw.Text, "raw-json") || strings.Contains(saw.Text, "payload:") {
		t.Fatalf("unknown event must not dump raw JSON-RPC: %q", saw.Text)
	}
}
