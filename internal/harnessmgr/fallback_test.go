package harnessmgr_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

type stubAdapter struct {
	id        string
	available bool
}

func (s stubAdapter) Name() string { return s.id }
func (s stubAdapter) IsAvailable(context.Context) error {
	if s.available {
		return nil
	}
	return errors.New("unavailable")
}
func (s stubAdapter) CreateSession(context.Context, harness.SessionRequest) (*harness.Session, error) {
	return &harness.Session{ID: "sess"}, nil
}
func (s stubAdapter) ResumeSession(context.Context, string) error { return nil }
func (s stubAdapter) Execute(context.Context, harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	return &harness.ExecutionResult{}, nil
}
func (s stubAdapter) Cancel(context.Context, string) error { return nil }
func (s stubAdapter) Status(context.Context, string) (*harness.ExecutionStatus, error) {
	return &harness.ExecutionStatus{State: harness.StatusIdle}, nil
}
func (s stubAdapter) Dispatch(context.Context, harness.DispatchRequest) (harness.DispatchResult, error) {
	return harness.DispatchResult{}, nil
}

type stubRegistry struct {
	adapters map[string]harness.HarnessAdapter
}

func (r stubRegistry) Adapter(id string) (harness.HarnessAdapter, error) {
	a, ok := r.adapters[id]
	if !ok {
		return nil, errors.New("unknown harness")
	}
	return a, nil
}

func (r stubRegistry) EnabledIDs(hero install.HeroJSON) []string {
	return install.ListEnabledHarnesses(hero)
}

func (r stubRegistry) SupportedIDs() []string {
	return install.SupportedHarnessIDs
}

func TestResolveExecutePair_FallbackToFallbackModel(t *testing.T) {
	reg := stubRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor":   stubAdapter{id: "cursor", available: false},
		"opencode": stubAdapter{id: "opencode", available: true},
	}}
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"cursor", "opencode"}),
	}
	pair, attempts, err := harnessmgr.ResolveExecutePair(context.Background(), reg, hero,
		"cursor", "composer-2.5",
		"opencode", "anthropic/claude-sonnet-4")
	if err != nil {
		t.Fatal(err)
	}
	if pair.HarnessID != "opencode" {
		t.Fatalf("harness=%q", pair.HarnessID)
	}
	if len(attempts) != 1 {
		t.Fatalf("attempts=%v", attempts)
	}
}

func TestResolveExecutePair_CodexTriedFirst(t *testing.T) {
	reg := stubRegistry{adapters: map[string]harness.HarnessAdapter{
		"codex":  stubAdapter{id: "codex", available: true},
		"cursor": stubAdapter{id: "cursor", available: true},
	}}
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"codex", "cursor"}),
	}
	pair, attempts, err := harnessmgr.ResolveExecutePair(context.Background(), reg, hero,
		"codex", "gpt-5.4",
		"cursor", "composer-2.5")
	if err != nil {
		t.Fatal(err)
	}
	if pair.HarnessID != "codex" || pair.Model != "gpt-5.4" {
		t.Fatalf("pair=%+v", pair)
	}
	if len(attempts) != 0 {
		t.Fatalf("attempts=%v want none when agent pair available", attempts)
	}
}

func TestResolveExecutePair_UnavailableCodexFallsBackWithWarning(t *testing.T) {
	reg := stubRegistry{adapters: map[string]harness.HarnessAdapter{
		"codex":  stubAdapter{id: "codex", available: false},
		"cursor": stubAdapter{id: "cursor", available: true},
	}}
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"codex", "cursor"}),
	}
	pair, attempts, err := harnessmgr.ResolveExecutePair(context.Background(), reg, hero,
		"codex", "gpt-5.4",
		"cursor", "composer-2.5")
	if err != nil {
		t.Fatal(err)
	}
	if pair.HarnessID != "cursor" || pair.Model != "composer-2.5" {
		t.Fatalf("pair=%+v", pair)
	}
	if len(attempts) != 1 || attempts[0].HarnessID != "codex" {
		t.Fatalf("attempts=%v", attempts)
	}
	warn := harnessmgr.FormatFallbackWarning("planning_agent", "codex", "gpt-5.4", pair.HarnessID, pair.Model)
	for _, part := range []string{"planning_agent", "codex", "gpt-5.4", "cursor", "composer-2.5", "Fallback"} {
		if !strings.Contains(warn, part) {
			t.Fatalf("fallback warning=%q missing %q", warn, part)
		}
	}
}

func TestResolveExecutePair_CodexHardStopNoThirdHarness(t *testing.T) {
	reg := stubRegistry{adapters: map[string]harness.HarnessAdapter{
		"codex":    stubAdapter{id: "codex", available: false},
		"cursor":   stubAdapter{id: "cursor", available: false},
		"opencode": stubAdapter{id: "opencode", available: true},
	}}
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"codex", "cursor", "opencode"}),
	}
	_, attempts, err := harnessmgr.ResolveExecutePair(context.Background(), reg, hero,
		"codex", "gpt-5.4",
		"cursor", "composer-2.5")
	if err == nil {
		t.Fatal("expected hard stop")
	}
	if len(attempts) != 2 {
		t.Fatalf("attempts=%v want agent+fallback only (no invented third harness)", attempts)
	}
	msg := harnessmgr.FormatHardStop("planning_agent", "codex", "gpt-5.4", attempts)
	if !strings.Contains(msg, "codex") || !strings.Contains(msg, "/hero-continue") {
		t.Fatalf("hard stop=%q", msg)
	}
	if strings.Contains(msg, "opencode") {
		t.Fatalf("must not invent opencode beyond configured fallback: %q", msg)
	}
}

func TestResolveExecutePair_HardStopWhenAllFail(t *testing.T) {
	reg := stubRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor":   stubAdapter{id: "cursor", available: false},
		"opencode": stubAdapter{id: "opencode", available: false},
	}}
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"cursor", "opencode"}),
	}
	_, attempts, err := harnessmgr.ResolveExecutePair(context.Background(), reg, hero,
		"opencode", "anthropic/claude-sonnet-4",
		"cursor", "composer-2.5")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(attempts) < 2 {
		t.Fatalf("attempts=%v", attempts)
	}
}

func TestResolveExecutePair_NoFreechatThirdFallback(t *testing.T) {
	reg := stubRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor":   stubAdapter{id: "cursor", available: false},
		"opencode": stubAdapter{id: "opencode", available: false},
	}}
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"cursor", "opencode"}),
	}
	_, attempts, err := harnessmgr.ResolveExecutePair(context.Background(), reg, hero,
		"cursor", "composer-2.5",
		"opencode", "anthropic/claude-sonnet-4")
	if err == nil {
		t.Fatal("expected hard stop when agent and fallback both fail")
	}
	if len(attempts) != 2 {
		t.Fatalf("attempts=%v want 2 (agent + fallback only)", attempts)
	}
	for _, a := range attempts {
		if a.AgentName == "freechat_default" {
			t.Fatalf("freechat must not be third Execute fallback: %+v", attempts)
		}
	}
}

func TestFormatFallbackWarning(t *testing.T) {
	msg := harnessmgr.FormatFallbackWarning("qa_agent", "cursor", "composer-2.5", "opencode", "anthropic/claude-sonnet-4")
	for _, part := range []string{"qa_agent", "cursor", "composer-2.5", "opencode", "anthropic/claude-sonnet-4"} {
		if !strings.Contains(msg, part) {
			t.Fatalf("msg=%q missing %q", msg, part)
		}
	}
}

func TestFormatHardStop_CodexCLIMissing(t *testing.T) {
	attempts := []harnessmgr.FallbackAttempt{{
		HarnessID: "codex",
		Model:     "gpt-5.4",
		Err:       errors.New("codex CLI not on PATH"),
	}}
	msg := harnessmgr.FormatHardStop("planning_agent", "codex", "gpt-5.4", attempts)
	for _, part := range []string{
		"Cannot run planning_agent",
		"harness codex is not available",
		"codex CLI not found on PATH",
		"/hero-harness",
		"/hero-continue",
	} {
		if !strings.Contains(msg, part) {
			t.Fatalf("msg=%q missing %q", msg, part)
		}
	}
	if strings.Contains(msg, "workflow-config.yml") {
		t.Fatalf("Codex CLI missing must use UI-C06 suggestion, got %q", msg)
	}
}

type listingAdapter struct {
	stubAdapter
	listed bool
	models []string
}

func (a *listingAdapter) ListModels(context.Context) ([]string, error) {
	a.listed = true
	return a.models, nil
}

func TestListModels_SkipsOpenCodeAndCodexAtBoot(t *testing.T) {
	cursor := &listingAdapter{stubAdapter: stubAdapter{id: "cursor", available: true}, models: []string{"composer-2.5"}}
	opencode := &listingAdapter{stubAdapter: stubAdapter{id: "opencode", available: true}, models: []string{"opencode/gpt"}}
	codex := &listingAdapter{stubAdapter: stubAdapter{id: "codex", available: true}, models: []string{"gpt-5.4"}}
	reg := stubRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor":   cursor,
		"opencode": opencode,
		"codex":    codex,
	}}
	hero := install.HeroJSON{
		Harnesses: install.HarnessesFromSelection([]string{"cursor", "opencode", "codex"}),
	}
	opts, err := harnessmgr.ListModels(context.Background(), reg, hero)
	if err != nil {
		t.Fatal(err)
	}
	if !cursor.listed {
		t.Fatal("cursor ListModels should run at boot")
	}
	if opencode.listed || codex.listed {
		t.Fatal("boot ListModels must not start OpenCode serve or Codex app-server")
	}
	if len(opts) != 1 || opts[0].Model != "composer-2.5" || opts[0].Harness != "cursor" {
		t.Fatalf("opts=%v", opts)
	}
}
