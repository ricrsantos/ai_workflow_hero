package tui

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func closeOnce(ch chan struct{}) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			select {
			case <-ch:
			default:
				close(ch)
			}
		})
	}
}

func writeAgentFile(t *testing.T, dir, name, marker string) {
	t.Helper()
	agentDir := filepath.Join(dir, ".cursor", "agents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\n---\n\n" + marker
	if err := os.WriteFile(filepath.Join(agentDir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeHeroStartCommand(t *testing.T, dir string) {
	t.Helper()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# /hero-start\n\nHERO_START_RUNTIME_MARKER"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestServiceWithRunningStage(t *testing.T, dir, stage, yamlBody string) *cycle.Service {
	t.Helper()
	heroDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(heroDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(heroDir, "workflow-config.yml"), []byte(yamlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".workflow-hero", "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartStage(stage); err != nil {
		t.Fatal(err)
	}
	return svc
}

const planningHandoffYAML = `title: TUI Planning Handoff
objective: test
agents:
  orchestration_agent:
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
  planning_agent:
    harness: codex
    model: gpt-5.4
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
  enable_fast_model: false
  thinking: na
stages:
  research:
    enabled: false
    max_iterations: 1
    require_human_approval: false
  planning:
    enabled: true
    max_iterations: 1
    require_human_approval: false
`

func TestHeroStartHandsOffToPlanningAgentPair(t *testing.T) {
	dir := t.TempDir()
	writeHeroStartCommand(t, dir)
	writeAgentFile(t, dir, "orchestration_agent", "ORCHESTRATION_AGENT_MARKER")
	writeAgentFile(t, dir, "planning_agent", "PLANNING_AGENT_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "planning", planningHandoffYAML)
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "codex": true})

	orchH := &streamingHarness{deltas: []string{"started planning"}, sessionIDs: []string{"orch-sess", "orch-resume"}}
	planH := &streamingHarness{deltas: []string{"writing SDD"}, sessionIDs: []string{"plan-sess"}}
	svc.Harness = nil
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor": orchH,
		"codex":  planH,
	}}

	m := withDefaultChatModel(NewTestModel(svc))
	m = SetWidth(m, 100)
	m = SetHeight(m, 32)
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	next = drainConversationStream(t, next, cmd)

	calls := planH.Calls()
	if len(calls) != 1 {
		t.Fatalf("planning executes=%d want 1 (%+v)", len(calls), calls)
	}
	if calls[0].Agent != "planning_agent" {
		t.Fatalf("planning agent=%q want planning_agent", calls[0].Agent)
	}
	if calls[0].Model != "gpt-5.4" {
		t.Fatalf("planning model=%q want gpt-5.4", calls[0].Model)
	}
	if !strings.Contains(calls[0].Prompt, "PLANNING_AGENT_MARKER") {
		t.Fatalf("planning prompt missing agent body: %q", calls[0].Prompt)
	}
	if !strings.Contains(calls[0].Prompt, "planning_agent") {
		t.Fatalf("planning prompt missing TUI preamble: %q", calls[0].Prompt)
	}
	orchCalls := orchH.Calls()
	if len(orchCalls) < 1 || orchCalls[0].Agent != "orchestration_agent" {
		t.Fatalf("orchestrator first call=%+v", orchCalls)
	}
	if len(orchCalls) != 2 {
		t.Fatalf("orchestrator executes=%d want 2 (start + resume after planning)", len(orchCalls))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "[PLAN - gpt-5.4 · codex]") {
		t.Fatalf("expected planning speaker header: %q", view)
	}
}

const implementationHandoffYAML = `title: TUI Implementation Handoff
objective: test
scope:
  backend: true
  frontend: true
  native: false
  script: false
  infrastructure: false
agents:
  orchestration_agent:
    harness: cursor
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
  backend_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
  frontend_agent:
    harness: codex
    model: gpt-5.4
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
fallback_model:
  harness: cursor
  model: composer-2.5
  reasoning_effort: na
  enable_fast_model: false
  thinking: na
stages:
  research:
    enabled: false
    max_iterations: 1
    require_human_approval: false
  planning:
    enabled: false
    max_iterations: 1
    require_human_approval: false
  implementation:
    enabled: true
    max_iterations: 1
    require_human_approval: false
`

func TestImplementationScopeStartsParallelExecutes(t *testing.T) {
	dir := t.TempDir()
	writeHeroStartCommand(t, dir)
	writeAgentFile(t, dir, "orchestration_agent", "ORCHESTRATION_AGENT_MARKER")
	writeAgentFile(t, dir, "backend_agent", "BACKEND_AGENT_MARKER")
	writeAgentFile(t, dir, "frontend_agent", "FRONTEND_AGENT_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "codex": true})

	backRel := make(chan struct{})
	frntRel := make(chan struct{})
	closeBack := closeOnce(backRel)
	closeFrnt := closeOnce(frntRel)
	t.Cleanup(func() { closeBack(); closeFrnt() })
	cursorH := &streamingHarness{
		deltas:      []string{"ok"},
		sessionIDs:  []string{"orch-sess", "back-sess"},
		release:     backRel,
		skipRelease: 1,
	}
	frntH := &streamingHarness{
		deltas:     []string{"frnt work"},
		sessionIDs: []string{"frnt-sess"},
		release:    frntRel,
	}
	svc.Harness = nil
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor": cursorH,
		"codex":  frntH,
	}}

	m := withDefaultChatModel(NewTestModel(svc))
	m = SetWidth(m, 100)
	m = SetHeight(m, 36)
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	next, cmd = pumpConversationUntil(t, next, cmd, 5*time.Second, func(m model) bool {
		if cursorH.ExecuteCount() < 2 || frntH.ExecuteCount() < 1 {
			return false
		}
		var hasBack, hasFrnt bool
		for _, a := range LiveAgentsForTest(m) {
			if a.Label == "BACK" {
				hasBack = true
			}
			if a.Label == "FRNT" {
				hasFrnt = true
			}
		}
		return hasBack && hasFrnt && IsConversationStreaming(m)
	})

	if len(LiveAgentsForTest(next)) < 2 {
		t.Fatalf("navbar count=%d want >= 2: %+v", len(LiveAgentsForTest(next)), LiveAgentsForTest(next))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "[BACK - composer-2.5 · cursor]") {
		t.Fatalf("missing BACK header: %q", view)
	}
	if !strings.Contains(view, "[FRNT - gpt-5.4 · codex]") {
		t.Fatalf("missing FRNT header: %q", view)
	}
	if !strings.Contains(view, "Waiting for harness") {
		t.Fatalf("spinner should remain while children are live: %q", view)
	}

	closeBack()
	next, cmd = pumpConversationUntil(t, next, cmd, 5*time.Second, func(m model) bool {
		if !IsConversationStreaming(m) {
			return false
		}
		var hasBack, hasFrnt bool
		for _, a := range LiveAgentsForTest(m) {
			if a.Label == "BACK" {
				hasBack = true
			}
			if a.Label == "FRNT" {
				hasFrnt = true
			}
		}
		return !hasBack && hasFrnt
	})
	if !IsConversationStreaming(next) {
		t.Fatal("first child done must not end the sibling stream")
	}

	closeFrnt()
	next = drainConversationStream(t, next, cmd)
	backCalls := cursorH.Calls()
	if len(backCalls) < 2 || backCalls[1].Agent != "backend_agent" {
		t.Fatalf("cursor calls=%+v want orch then backend_agent", backCalls)
	}
	frntCalls := frntH.Calls()
	if len(frntCalls) != 1 || frntCalls[0].Agent != "frontend_agent" {
		t.Fatalf("frontend calls=%+v", frntCalls)
	}
}

func TestImplementationCancelCancelsAllExecutes(t *testing.T) {
	dir := t.TempDir()
	writeHeroStartCommand(t, dir)
	writeAgentFile(t, dir, "orchestration_agent", "ORCHESTRATION_AGENT_MARKER")
	writeAgentFile(t, dir, "backend_agent", "BACKEND_AGENT_MARKER")
	writeAgentFile(t, dir, "frontend_agent", "FRONTEND_AGENT_MARKER")
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	writeHeroJSONHarnesses(t, dir, map[string]bool{"cursor": true, "codex": true})

	backRel := make(chan struct{})
	frntRel := make(chan struct{})
	closeBack := closeOnce(backRel)
	closeFrnt := closeOnce(frntRel)
	t.Cleanup(func() { closeBack(); closeFrnt() })
	cursorH := &streamingHarness{
		deltas:      []string{"ok"},
		sessionIDs:  []string{"orch-sess", "back-sess"},
		release:     backRel,
		skipRelease: 1,
	}
	frntH := &streamingHarness{
		deltas:     []string{"frnt work"},
		sessionIDs: []string{"frnt-sess"},
		release:    frntRel,
	}
	svc.Harness = nil
	svc.Registry = routingRegistry{adapters: map[string]harness.HarnessAdapter{
		"cursor": cursorH,
		"codex":  frntH,
	}}

	m := withDefaultChatModel(NewTestModel(svc))
	next, cmd := RunPaletteItemForTest(m, "/hero-start")
	next, _ = pumpConversationUntil(t, next, cmd, 5*time.Second, func(m model) bool {
		return cursorH.ExecuteCount() >= 2 && frntH.ExecuteCount() >= 1 && IsConversationStreaming(m)
	})

	next, cancelCmd := CancelConversationStreamForTest(next)
	if cancelCmd != nil {
		msg := cancelCmd()
		next2, _ := next.Update(msg)
		next = next2.(model)
	}
	if IsConversationStreaming(next) {
		t.Fatal("expected streaming stopped after cancel-all")
	}
	if !cursorH.CancelCalled() || !frntH.CancelCalled() {
		t.Fatal("Ctrl+C must cancel every in-flight Execute")
	}
}

func TestImplementationAgentsFromScope(t *testing.T) {
	dir := t.TempDir()
	svc := newTestServiceWithRunningStage(t, dir, "implementation", implementationHandoffYAML)
	m := NewTestModel(svc)
	got := m.implementationAgentsFromScope()
	if len(got) != 2 || got[0] != agentBackend || got[1] != agentFrontend {
		t.Fatalf("agents=%v want backend+frontend", got)
	}
}
