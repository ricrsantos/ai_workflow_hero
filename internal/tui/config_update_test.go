package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

func writeConfigUpdateFixture(t *testing.T, dir, heroJSON, workflowYAML string) *cycle.Service {
	t.Helper()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), []byte(heroJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(workflowYAML), 0o644); err != nil {
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
	return svc
}

func TestHeroConfigUpdateReloadsWorkflowModel(t *testing.T) {
	dir := t.TempDir()
	hero := `{
  "harnesses": {"cursor": {"enabled": true, "model": "composer-2.5"}},
  "freechat_default": {"harness": "cursor", "model": "composer-2.5"}
}`
	yamlA := `title: Config Update
objective: test
agents:
  orchestration_agent:
    harness: cursor
    model: composer-2.5
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
    enabled: true
    max_iterations: 1
    require_human_approval: false
`
	svc := writeConfigUpdateFixture(t, dir, hero, yamlA)

	m := NewTestModel(svc)
	m = SetWidth(m, 100)
	m = SetHeight(m, 28)
	m = EnterConversationForTest(m)
	m = SetOrchestrationLiveForTest(m, true)
	m = SetRuntimeModelSlugForTest(m, "composer-2.5")
	m = SetRuntimeHarnessIDForTest(m, "cursor")
	m.runtimeAgentName = "orchestration_agent"

	yamlB := strings.Replace(yamlA, "model: composer-2.5", "model: cursor-grok-4.6-low", 1)
	if err := os.WriteFile(filepath.Join(dir, ".workflow-hero", "cycles", "current", "workflow-config.yml"), []byte(yamlB), 0o644); err != nil {
		t.Fatal(err)
	}

	next, _ := RunPaletteItemForTest(m, "/hero-config-update")
	if RuntimeModelSlugForTest(next) != "cursor-grok-4.6-low" {
		t.Fatalf("runtime model=%q want cursor-grok-4.6-low", RuntimeModelSlugForTest(next))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "cursor-grok-4.6-low") {
		t.Fatalf("input/view missing reloaded model: %q", view)
	}
	if !strings.Contains(StatusTextForTest(next), "config reloaded") {
		t.Fatalf("status=%q", StatusTextForTest(next))
	}
}

func TestHeroConfigUpdateReloadsFreechatModel(t *testing.T) {
	dir := t.TempDir()
	heroA := `{
  "harnesses": {"cursor": {"enabled": true, "model": "composer-2.5"}},
  "freechat_default": {"harness": "cursor", "model": "composer-2.5"}
}`
	yaml := `title: Config Update
objective: test
agents:
  orchestration_agent:
    harness: cursor
    model: composer-2.5
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
    enabled: true
    max_iterations: 1
    require_human_approval: false
`
	svc := writeConfigUpdateFixture(t, dir, heroA, yaml)

	m := NewTestModel(svc)
	m = SetWidth(m, 100)
	m = SetHeight(m, 28)
	m = EnterConversationForTest(m)
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m = SetChatHarnessIDForTest(m, "cursor")

	heroB := `{
  "harnesses": {"cursor": {"enabled": true, "model": "auto"}},
  "freechat_default": {"harness": "cursor", "model": "auto"}
}`
	if err := os.WriteFile(filepath.Join(dir, ".workflow-hero", "config", "hero.json"), []byte(heroB), 0o644); err != nil {
		t.Fatal(err)
	}

	next, _ := RunPaletteItemForTest(m, "/hero-config-update")
	if ChatModelSlugForTest(next) != "auto" {
		t.Fatalf("chat model=%q want auto", ChatModelSlugForTest(next))
	}
	view := ViewForTest(next)
	if !strings.Contains(view, "auto") {
		t.Fatalf("input/view missing reloaded freechat model: %q", view)
	}
}

func TestHeroConfigUpdateFromComposerSlash(t *testing.T) {
	dir := t.TempDir()
	hero := `{
  "harnesses": {"cursor": {"enabled": true, "model": "composer-2.5"}},
  "freechat_default": {"harness": "cursor", "model": "old-model"}
}`
	yaml := `title: t
objective: t
agents:
  orchestration_agent:
    harness: cursor
    model: composer-2.5
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
    enabled: true
    max_iterations: 1
    require_human_approval: false
`
	svc := writeConfigUpdateFixture(t, dir, hero, yaml)
	m := NewTestModel(svc)
	m = EnterConversationForTest(m)
	m = SetChatModelSlugForTest(m, "old-model")

	heroB := `{
  "harnesses": {"cursor": {"enabled": true, "model": "composer-2.5"}},
  "freechat_default": {"harness": "cursor", "model": "composer-2.5"}
}`
	if err := os.WriteFile(filepath.Join(dir, ".workflow-hero", "config", "hero.json"), []byte(heroB), 0o644); err != nil {
		t.Fatal(err)
	}

	m = SetConversationInput(m, "/hero-config-update")
	next, _ := HandleTestKey(m, "alt+enter")
	if ChatModelSlugForTest(next) != "composer-2.5" {
		t.Fatalf("chat model=%q want composer-2.5", ChatModelSlugForTest(next))
	}
}
