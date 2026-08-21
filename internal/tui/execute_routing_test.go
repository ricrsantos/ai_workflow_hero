package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

// recordingNamedHarness records Dispatch/Execute and reports a fixed Name().
type recordingNamedHarness struct {
	id         string
	dispatched bool
	executed   bool
	lastModel  string
}

func (h *recordingNamedHarness) Name() string { return h.id }
func (h *recordingNamedHarness) IsAvailable(context.Context) error {
	if h.id == "codex-unavailable" {
		return errors.New("codex CLI not on PATH")
	}
	return nil
}
func (h *recordingNamedHarness) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return &harness.Session{ID: "s"}, nil
}
func (h *recordingNamedHarness) ResumeSession(context.Context, string) error { return nil }
func (h *recordingNamedHarness) Execute(_ context.Context, req harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	h.executed = true
	h.lastModel = req.Model
	return &harness.ExecutionResult{SessionID: "s", Output: "ok"}, nil
}
func (h *recordingNamedHarness) Cancel(context.Context, string) error { return nil }
func (h *recordingNamedHarness) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return &harness.ExecutionStatus{State: harness.StatusIdle}, nil
}
func (h *recordingNamedHarness) Dispatch(_ context.Context, req harness.DispatchRequest) (harness.DispatchResult, error) {
	h.dispatched = true
	h.lastModel = req.Model
	return harness.DispatchResult{Dispatched: true, Message: "ok"}, nil
}

type routingRegistry struct {
	adapters map[string]harness.HarnessAdapter
}

func (r routingRegistry) Adapter(id string) (harness.HarnessAdapter, error) {
	a, ok := r.adapters[id]
	if !ok {
		return nil, errors.New("unknown harness")
	}
	return a, nil
}
func (r routingRegistry) EnabledIDs(hero install.HeroJSON) []string {
	return install.ListEnabledHarnesses(hero)
}
func (r routingRegistry) SupportedIDs() []string { return install.SupportedHarnessIDs }

func TestDispatchRoutesToCodexByChatHarness(t *testing.T) {
	dir := t.TempDir()
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "codex": true})
	svc := newTestServiceInDir(t, dir)
	codexH := &recordingNamedHarness{id: "codex"}
	svc.Harness = nil
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"codex":  codexH,
		"cursor": &recordingNamedHarness{id: "cursor"},
	}}

	m := NewTestModel(svc)
	m = SetChatHarnessIDForTest(m, "codex")
	m = SetChatModelSlugForTest(m, "gpt-5.4")

	msg := dispatchPromptMsg(svc, "/opsx-x", "hello", "gpt-5.4", harness.ModeBuild, "codex")
	result, ok := msg.(actionResultMsg)
	if !ok {
		t.Fatalf("msg type %T", msg)
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	if !codexH.dispatched {
		t.Fatal("expected Codex Dispatch")
	}
	_ = m
}

func TestResolveExecuteResolution_UnavailableCodexFallsBack(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceInDir(t, dir)
	writeHeroJSONHarnesses(t, dir, map[string]bool{"codex": true, "cursor": true})
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), []byte(`title: t
objective: t
agents:
  planning_agent:
    harness: codex
    model: gpt-5.4
fallback_model:
  harness: cursor
  model: composer-2.5
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cursorH := &recordingNamedHarness{id: "cursor"}
	codexH := &recordingNamedHarness{id: "codex-unavailable"}
	svc.Harness = nil
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"codex":  codexH,
		"cursor": cursorH,
	}}

	m := NewTestModel(svc)
	m.runtimeAgentName = "planning_agent"

	resolved, err := m.resolveExecuteResolution(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resolved.pair.HarnessID != "cursor" {
		t.Fatalf("harness=%q want cursor fallback; warning=%q err path", resolved.pair.HarnessID, resolved.warning)
	}
	if !strings.Contains(resolved.warning, "codex") || !strings.Contains(resolved.warning, "Fallback") {
		t.Fatalf("warning=%q", resolved.warning)
	}
}

func writeHeroJSONHarnesses(t *testing.T, dir string, enabled map[string]bool) {
	t.Helper()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString(`{"harnesses":{`)
	first := true
	for id, on := range enabled {
		if !first {
			b.WriteByte(',')
		}
		first = false
		model := "composer-2.5"
		if id == "codex" {
			model = "gpt-5.4"
		}
		b.WriteString(`"` + id + `":{"enabled":`)
		if on {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString(`,"model":"` + model + `"}`)
	}
	b.WriteString(`},"freechat_default":{"harness":"cursor","model":"composer-2.5"}}`)
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
