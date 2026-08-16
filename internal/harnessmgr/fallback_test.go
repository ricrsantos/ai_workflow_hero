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
