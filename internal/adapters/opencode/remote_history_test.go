package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestReadRemoteHistoryNormalizesUserAndAssistant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/sess-import/message":
			_ = json.NewEncoder(w).Encode([]storedMessage{
				{
					Info: map[string]any{
						"role": "user",
						"id":   "msg-user",
						"time": map[string]any{"created": 100.0},
					},
					Parts: []map[string]any{{"type": "text", "text": "hello"}},
				},
				{
					Info: map[string]any{
						"role": "assistant",
						"id":   "msg-asst",
						"time": map[string]any{"created": 101.0},
					},
					Parts: []map[string]any{
						{"type": "reasoning", "id": "r1", "text": "think"},
						{"type": "text", "text": "world"},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	a := NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "opencode", nil }
	a.Runner = &stubRunner{}
	a.HTTP = srv.Client()
	a.ResolveServeURL = func(ProcessHandle) (string, int, error) { return srv.URL, 1, nil }

	events, err := a.ReadRemoteHistory(context.Background(), "sess-import")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("events=%d", len(events))
	}
	if events[0].EventType != harness.NormalizedEventUser || events[0].ProviderEventID != "msg-user" {
		t.Fatalf("user=%+v", events[0])
	}
	if events[1].EventType != harness.NormalizedEventThinking {
		t.Fatalf("thinking=%+v", events[1])
	}
	if events[2].EventType != harness.NormalizedEventAssistant {
		t.Fatalf("assistant=%+v", events[2])
	}
	if !events[0].CreatedAt.Equal(time.Unix(100, 0).UTC()) {
		t.Fatalf("created=%v", events[0].CreatedAt)
	}
}

func TestRemoteHistoryPreservesMixedPartOrder(t *testing.T) {
	events, err := normalizeOpenCodeSessionMessages("sess", []storedMessage{{
		Info: map[string]any{"role": "assistant", "id": "msg-mix"},
		Parts: []map[string]any{
			{"type": "text", "text": "before"},
			{"type": "tool", "id": "tool-1", "name": "read"},
			{"type": "text", "text": "after"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("events=%d", len(events))
	}
	if events[0].EventType != harness.NormalizedEventAssistant {
		t.Fatalf("first=%+v", events[0])
	}
	if events[1].EventType != harness.NormalizedEventTool {
		t.Fatalf("second=%+v", events[1])
	}
	if events[2].EventType != harness.NormalizedEventAssistant {
		t.Fatalf("third=%+v", events[2])
	}
	var first, third map[string]string
	if err := json.Unmarshal(events[0].PayloadJSON, &first); err != nil || first["text"] != "before" {
		t.Fatalf("first payload=%s err=%v", events[0].PayloadJSON, err)
	}
	if err := json.Unmarshal(events[2].PayloadJSON, &third); err != nil || third["text"] != "after" {
		t.Fatalf("third payload=%s err=%v", events[2].PayloadJSON, err)
	}
}

func TestRemoteHistoryMissingPartIDIsStable(t *testing.T) {
	parts := []map[string]any{{"type": "tool", "name": "grep", "input": map[string]any{"pattern": "x"}}}
	first, err := normalizeOpenCodeSessionMessages("sess", []storedMessage{{
		Info: map[string]any{"role": "assistant", "id": "msg-a"}, Parts: parts,
	}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := normalizeOpenCodeSessionMessages("sess", []storedMessage{{
		Info: map[string]any{"role": "assistant", "id": "msg-a"}, Parts: parts,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("events first=%d second=%d", len(first), len(second))
	}
	if first[0].ProviderEventID == "" || first[0].ProviderEventID != second[0].ProviderEventID {
		t.Fatalf("provider ids=%q %q", first[0].ProviderEventID, second[0].ProviderEventID)
	}
	if providerIDUsesNumericIndexSuffix(first[0].ProviderEventID) {
		t.Fatalf("provider id must not use index suffix: %q", first[0].ProviderEventID)
	}
}

func TestRemoteHistoryRejectsMalformedText(t *testing.T) {
	_, err := normalizeOpenCodeSessionMessages("sess", []storedMessage{{
		Info:  map[string]any{"role": "user", "id": "m1"},
		Parts: []map[string]any{{"type": "text", "text": 42}},
	}})
	if err == nil {
		t.Fatal("expected malformed text error")
	}
	_, err = normalizeOpenCodeSessionMessages("sess", []storedMessage{{
		Info:  map[string]any{"role": "assistant", "id": "m1"},
		Parts: []map[string]any{{"type": "text", "text": nil}},
	}})
	if err != nil {
		t.Fatalf("nil text should be empty, got %v", err)
	}
}

func TestRemoteHistoryFilePartNormalizesAssetEvent(t *testing.T) {
	events, err := normalizeOpenCodeSessionMessages("sess", []storedMessage{{
		Info: map[string]any{"role": "assistant", "id": "msg-img"},
		Parts: []map[string]any{{
			"type": "image", "id": "img-1", "filename": "shot.png", "mime": "image/png", "url": "https://example.com/shot.png",
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventType != harness.NormalizedEventAsset {
		t.Fatalf("events=%+v", events)
	}
	var asset harness.Asset
	if err := json.Unmarshal(events[0].PayloadJSON, &asset); err != nil {
		t.Fatal(err)
	}
	if asset.Name != "shot.png" || asset.ContentHash == "" {
		t.Fatalf("asset=%+v", asset)
	}
}

func TestRemoteHistoryEmptyMessageIDReorderStable(t *testing.T) {
	userMsg := storedMessage{
		Info:  map[string]any{"role": "user", "time": map[string]any{"created": 10.0}},
		Parts: []map[string]any{{"type": "text", "text": "hello"}},
	}
	asstMsg := storedMessage{
		Info:  map[string]any{"role": "assistant", "time": map[string]any{"created": 11.0}},
		Parts: []map[string]any{{"type": "text", "text": "world"}},
	}
	first, err := normalizeOpenCodeSessionMessages("sess", []storedMessage{userMsg, asstMsg})
	if err != nil {
		t.Fatal(err)
	}
	second, err := normalizeOpenCodeSessionMessages("sess", []storedMessage{asstMsg, userMsg})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("events first=%d second=%d", len(first), len(second))
	}
	if first[0].ProviderEventID != second[1].ProviderEventID {
		t.Fatalf("user provider id drift: %q vs %q", first[0].ProviderEventID, second[1].ProviderEventID)
	}
	if first[1].ProviderEventID != second[0].ProviderEventID {
		t.Fatalf("assistant provider id drift: %q vs %q", first[1].ProviderEventID, second[0].ProviderEventID)
	}
	if strings.Contains(first[0].ProviderEventID, ":msg:0") || strings.Contains(first[0].ProviderEventID, ":msg:1") {
		t.Fatalf("provider id must not use index suffix: %q", first[0].ProviderEventID)
	}
}

func TestRemoteHistoryIdempotentReimport(t *testing.T) {
	msg := storedMessage{
		Info: map[string]any{"role": "assistant", "id": "msg-idem"},
		Parts: []map[string]any{
			{"type": "text", "text": "same"},
			{"type": "tool", "id": "t1", "name": "read"},
		},
	}
	first, err := normalizeOpenCodeSessionMessages("sess", []storedMessage{msg})
	if err != nil {
		t.Fatal(err)
	}
	second, err := normalizeOpenCodeSessionMessages("sess", []storedMessage{msg})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != len(second) {
		t.Fatalf("len first=%d second=%d", len(first), len(second))
	}
	for i := range first {
		if first[i].ProviderEventID != second[i].ProviderEventID {
			t.Fatalf("event %d provider id drift: %q vs %q", i, first[i].ProviderEventID, second[i].ProviderEventID)
		}
		if first[i].EventType != second[i].EventType {
			t.Fatalf("event %d type drift", i)
		}
	}
}

func providerIDUsesNumericIndexSuffix(id string) bool {
	i := strings.LastIndex(id, ":part:")
	if i < 0 {
		return false
	}
	suffix := id[i+len(":part:"):]
	if suffix == "" {
		return false
	}
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func TestReadRemoteHistoryRejectsUnknownPartType(t *testing.T) {
	_, err := normalizeOpenCodeSessionMessages("sess", []storedMessage{{
		Info:  map[string]any{"role": "assistant", "id": "m1"},
		Parts: []map[string]any{{"type": "mystery"}},
	}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAdapterSupportsRemoteHistory(t *testing.T) {
	a := NewAdapter(t.TempDir(), nil)
	if !a.SupportsRemoteHistory() {
		t.Fatal("expected remote history support")
	}
}

func TestRemoteHistoryNullMessageIDsDistinctAndStable(t *testing.T) {
	msgMissingID := storedMessage{
		Info:  map[string]any{"role": "user", "time": map[string]any{"created": 1.0}},
		Parts: []map[string]any{{"type": "text", "text": "alpha"}},
	}
	msgExplicitNullID := storedMessage{
		Info: map[string]any{
			"role": "user",
			"id":   nil,
			"time": map[string]any{"created": 2.0},
		},
		Parts: []map[string]any{{"type": "text", "text": "beta"}},
	}

	first, err := normalizeOpenCodeSessionMessages("sess-null", []storedMessage{msgMissingID, msgExplicitNullID})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("events=%d", len(first))
	}
	if first[0].ProviderEventID == "" || first[1].ProviderEventID == "" {
		t.Fatalf("empty provider ids: %q %q", first[0].ProviderEventID, first[1].ProviderEventID)
	}
	if first[0].ProviderEventID == first[1].ProviderEventID {
		t.Fatalf("distinct messages must not collide: %q", first[0].ProviderEventID)
	}
	for _, id := range []string{first[0].ProviderEventID, first[1].ProviderEventID} {
		if strings.Contains(id, "<nil>") || strings.Contains(id, "\u003cnil\u003e") {
			t.Fatalf("provider id must not contain nil literal: %q", id)
		}
	}

	second, err := normalizeOpenCodeSessionMessages("sess-null", []storedMessage{msgMissingID, msgExplicitNullID})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 {
		t.Fatalf("events=%d", len(second))
	}
	if first[0].ProviderEventID != second[0].ProviderEventID || first[1].ProviderEventID != second[1].ProviderEventID {
		t.Fatalf("idempotent reimport drift: first=%q,%q second=%q,%q",
			first[0].ProviderEventID, first[1].ProviderEventID,
			second[0].ProviderEventID, second[1].ProviderEventID)
	}
}

func TestStringFromAnyNilNotNilLiteral(t *testing.T) {
	if got := stringFromAny(nil); got != "" {
		t.Fatalf("nil => %q", got)
	}
	var nilIface any
	if got := stringFromAny(nilIface); got != "" {
		t.Fatalf("nil interface => %q", got)
	}
}
