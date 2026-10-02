package workflowconfig_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
	"gopkg.in/yaml.v3"
)

const managedDocumentYAML = `# Cycle title stays a YAML comment.
title: Original title
objective: Original objective
workflow_config:
  # This comment must not become a TUI help text.
  user_preferred_language: EN
scope:
  backend: true
  frontend: false
  native: false
  script: false
  infrastructure: false
stages:
  research:
    enabled: true
    purpose: Gather requirements.
    max_iterations: 2
    timeout_minutes: 10
    require_human_approval: true
  implementation:
    enabled: true
    purpose: Implement.
    max_iterations: 3
    timeout_minutes: 30
    require_human_approval: false
  browser_ui_validation:
    enabled: false
    purpose: Browser checks.
    max_iterations: 2
    timeout_minutes: 10
    require_human_approval: false
    visual_validation:
      enabled: false
      reference_dir: docs/ui
  qa_end_to_end:
    enabled: false
    purpose: End to end.
    max_iterations: 2
    timeout_minutes: 10
    require_human_approval: false
    use_playwright: false
agents:
  orchestration_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: true
      model: composer-2.5
      reasoning_effort: na
      enable_fast_model: false
      thinking: na
  context_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: true
      model: composer-2.5
      reasoning_effort: na
      enable_fast_model: false
      thinking: na
  discover_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: false
      model: cursor-grok-4.6
      reasoning_effort: high
      enable_fast_model: false
      thinking: off
  backend_agent:
    harness: cursor
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: true
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
workflow_rules:
  - This is a runtime guardrail and must stay untouched.
future_setting:
  nested: retained
`

func TestDocumentWritePreservesRulesCommentsAndUnknownKeys(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := doc.Config
	draft.Title = "Updated title"
	if err := doc.Write(draft, workflowconfig.ValidationOptions{ValidateEnabledHarnesses: true, EnabledHarnesses: []string{"cursor"}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{
		"# Cycle title stays a YAML comment.",
		"# This comment must not become a TUI help text.",
		"workflow_rules:",
		"This is a runtime guardrail and must stay untouched.",
		"future_setting:",
		"nested: retained",
		"title: Updated title",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("saved document missing %q:\n%s", want, got)
		}
	}
}

func TestDocumentWriteMergesExternalChanges(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := doc.Config
	draft.Title = "TUI title"
	latest := strings.Replace(managedDocumentYAML, "nested: retained", "nested: edited outside\nexternal_only: keep", 1)
	if err := os.WriteFile(path, []byte(latest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := doc.Write(draft, workflowconfig.ValidationOptions{ValidateEnabledHarnesses: true, EnabledHarnesses: []string{"cursor"}}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "title: TUI title") || !strings.Contains(string(got), "external_only: keep") {
		t.Fatalf("managed merge did not retain external unmanaged edit:\n%s", got)
	}
}

func TestDocumentReapplyUsesLatestUnknownContent(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := doc.Config
	draft.Title = "TUI title"
	latest := strings.Replace(managedDocumentYAML, "nested: retained", "nested: edited outside\nexternal_only: keep", 1)
	if err := os.WriteFile(path, []byte(latest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := doc.Reapply(draft, workflowconfig.ValidationOptions{ValidateEnabledHarnesses: true, EnabledHarnesses: []string{"cursor"}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if !strings.Contains(got, "title: TUI title") || !strings.Contains(got, "nested: edited outside") || !strings.Contains(got, "external_only: keep") {
		t.Fatalf("reapply did not combine changes:\n%s", got)
	}
}

func TestManagedConfigValidateAppliesStageAndSubagentRules(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := doc.Config
	cfg.Agents["discover_agent"] = workflowconfig.AgentModelConfig{
		Harness: "cursor", Model: "composer-2.5", ReasoningEffort: "na", Thinking: "na",
		Subagent: workflowconfig.SubagentConfig{SameOfAgent: false, Model: "not-a-cursor-model", ReasoningEffort: "na", Thinking: "na"},
	}
	err = cfg.Validate(workflowconfig.ValidationOptions{
		ValidateEnabledHarnesses: true,
		EnabledHarnesses:         []string{"cursor"},
		ModelKnown: func(harnessID, modelID string) (bool, bool) {
			return true, modelID == "composer-2.5" || modelID == "cursor-grok-4.6"
		},
	})
	if err == nil || !strings.Contains(err.Error(), "subagent.model") {
		t.Fatalf("err=%v, want invalid subagent model", err)
	}

	cfg = doc.Config
	stage := cfg.Stages["implementation"]
	stage.MaxIterations = 0
	cfg.Stages["implementation"] = stage
	err = cfg.Validate(workflowconfig.ValidationOptions{ValidateEnabledHarnesses: true, EnabledHarnesses: []string{"cursor"}})
	if err == nil || !strings.Contains(err.Error(), "max_iterations") {
		t.Fatalf("err=%v, want stage budget error", err)
	}
}

func TestManagedConfigRequiredAgentsFollowEnabledStagesAndScope(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	names := doc.Config.RequiredAgentNames()
	if !contains(names, "orchestration_agent") || !contains(names, "context_agent") || !contains(names, "discover_agent") || !contains(names, "backend_agent") {
		t.Fatalf("required agents=%v", names)
	}
	if contains(names, "frontend_agent") || contains(names, "qa_agent") {
		t.Fatalf("disabled-stage/scope agents must not be required: %v", names)
	}
}

func TestLoadDocumentFailsClosedForMissingAndInvalidFiles(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "workflow-config.yml")
	if _, err := workflowconfig.LoadDocument(missing); err == nil {
		t.Fatal("missing document must fail")
	}

	path := writeDocument(t, "title: [invalid\n")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workflowconfig.LoadDocument(path); err == nil {
		t.Fatal("invalid YAML must fail")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("invalid YAML must remain untouched")
	}
}

func TestDocumentReportsExplicitStageFields(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.HasExplicitStageField("qa_end_to_end", "use_playwright") {
		t.Fatal("explicit use_playwright:false was not detected")
	}
	if doc.HasExplicitStageField("browser_ui_validation", "use_playwright") {
		t.Fatal("absent stage field was reported explicit")
	}

	implicit := strings.Replace(managedDocumentYAML, "    use_playwright: false\n", "", 1)
	if err := os.WriteFile(path, []byte(implicit), 0o644); err != nil {
		t.Fatal(err)
	}
	implicitDoc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if implicitDoc.HasExplicitStageField("qa_end_to_end", "use_playwright") {
		t.Fatal("implicit false zero value was reported explicit")
	}
}

func TestDocumentWriteFailsClosedWhenLatestYAMLBecomesInvalid(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := doc.Config
	draft.Title = "must not write"
	invalid := []byte("title: [broken\n")
	if err := os.WriteFile(path, invalid, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := doc.Write(draft, workflowconfig.ValidationOptions{ValidateEnabledHarnesses: true, EnabledHarnesses: []string{"cursor"}}); err == nil {
		t.Fatal("save over invalid latest YAML must fail")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(invalid) {
		t.Fatalf("invalid latest YAML was overwritten: %q", got)
	}
}

func TestDocumentWriteValidationFailureLeavesOriginalBytesUntouched(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := doc.Config
	draft.Title = ""
	if err := doc.Write(draft, workflowconfig.ValidationOptions{
		ValidateEnabledHarnesses: true,
		EnabledHarnesses:         []string{"cursor"},
	}); err == nil {
		t.Fatal("invalid draft must fail before writing")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("validation failure changed workflow-config.yml bytes")
	}
}

func TestManagedDiffReportsOnlyChangedManagedPath(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	after := doc.Config
	after.Title = "new"
	if paths := workflowconfig.ManagedDiff(doc.Config, after); contains(paths, "stages.qa.max_iterations") {
		t.Fatalf("title-only diff marked QA: %v", paths)
	}
	after.Stages = make(map[string]workflowconfig.ManagedStage, len(doc.Config.Stages))
	for name, stage := range doc.Config.Stages {
		after.Stages[name] = stage
	}
	stage := after.Stages["research"]
	stage.MaxIterations++
	after.Stages["research"] = stage
	if !contains(workflowconfig.ManagedDiff(doc.Config, after), "stages.research.max_iterations") {
		t.Fatal("stage budget diff missing")
	}
}

func TestManagedKeysScreenshotsToggleRoundTripsAndMergesLatestDocument(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := doc.Config
	draft.Stages = make(map[string]workflowconfig.ManagedStage, len(doc.Config.Stages))
	for name, stage := range doc.Config.Stages {
		draft.Stages[name] = stage
	}
	if draft.Stages["browser_ui_validation"].Screenshots.Enabled {
		t.Fatal("missing Browser UI screenshots key must default to false")
	}
	if draft.Stages["qa_end_to_end"].Screenshots.Enabled {
		t.Fatal("missing E2E screenshots key must default to false")
	}
	browser := draft.Stages["browser_ui_validation"]
	browser.Screenshots.Enabled = true
	draft.Stages["browser_ui_validation"] = browser
	if paths := workflowconfig.ManagedDiff(doc.Config, draft); !contains(paths, "stages.browser_ui_validation.screenshots.enabled") {
		t.Fatalf("managed diff missing Browser UI screenshot toggle: %v", paths)
	}
	latest := strings.Replace(managedDocumentYAML, "nested: retained", "nested: edited outside\nexternal_only: keep", 1)
	if err := os.WriteFile(path, []byte(latest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := doc.Write(draft, workflowconfig.ValidationOptions{ValidateEnabledHarnesses: true, EnabledHarnesses: []string{"cursor"}}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(got)
	for _, want := range []string{
		"stages:",
		"browser_ui_validation:",
		"screenshots:",
		"enabled: true",
		"qa_end_to_end:",
		"use_playwright: false",
		"external_only: keep",
		"# Cycle title stays a YAML comment.",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("saved workflow config missing %q:\n%s", want, content)
		}
	}
	var reloaded struct {
		Stages map[string]struct {
			Screenshots struct {
				Enabled bool `yaml:"enabled"`
			} `yaml:"screenshots"`
			UsePlaywright bool `yaml:"use_playwright"`
		} `yaml:"stages"`
	}
	if err := yaml.Unmarshal(got, &reloaded); err != nil {
		t.Fatal(err)
	}
	if !reloaded.Stages["browser_ui_validation"].Screenshots.Enabled {
		t.Fatal("Browser UI screenshot toggle was not saved")
	}
	if reloaded.Stages["qa_end_to_end"].Screenshots.Enabled {
		t.Fatal("HTTP-only E2E screenshot toggle should remain off")
	}
}

func TestTestAccessOptInDefaultsOffAndPersistsWithoutDroppingUnmanagedKeys(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Config.TestAccess.Enabled {
		t.Fatal("absent test_access.enabled must default to false")
	}

	draft := doc.Config
	draft.TestAccess.Enabled = true
	if paths := workflowconfig.ManagedDiff(doc.Config, draft); !contains(paths, "test_access.enabled") {
		t.Fatalf("managed diff omitted test_access.enabled: %v", paths)
	}
	latest := strings.Replace(managedDocumentYAML, "nested: retained", "nested: retained\nexternal_test_access_setting: preserve", 1)
	if err := os.WriteFile(path, []byte(latest), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := workflowconfig.ValidationOptions{ValidateEnabledHarnesses: true, EnabledHarnesses: []string{"cursor"}}
	if err := doc.Write(draft, opts); err != nil {
		t.Fatal(err)
	}
	reloaded, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.Config.TestAccess.Enabled {
		t.Fatal("test_access.enabled did not persist across reload")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "external_test_access_setting: preserve") {
		t.Fatalf("managed save dropped unrelated entry:\n%s", content)
	}
}

func TestTestAccessRejectsNonBooleanEnabledValue(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML+"\ntest_access:\n  enabled: 'enabled'\n")
	if _, err := workflowconfig.LoadDocument(path); err == nil || !strings.Contains(err.Error(), "cannot unmarshal") || !strings.Contains(err.Error(), "into bool") {
		t.Fatalf("LoadDocument err=%v, want non-boolean test_access.enabled rejection", err)
	}
}

func TestManagedKeysRejectHTTPScreenshotsForDisabledE2EStage(t *testing.T) {
	path := writeDocument(t, managedDocumentYAML)
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := doc.Config
	e2e := cfg.Stages["qa_end_to_end"]
	e2e.Enabled = false
	e2e.UsePlaywright = false
	e2e.Screenshots.Enabled = true
	cfg.Stages["qa_end_to_end"] = e2e
	err = cfg.Validate(workflowconfig.ValidationOptions{ValidateEnabledHarnesses: true, EnabledHarnesses: []string{"cursor"}})
	if err == nil || !strings.Contains(err.Error(), "screenshots.enabled requires stages.qa_end_to_end.use_playwright") {
		t.Fatalf("err=%v, want HTTP-only E2E screenshot rejection even when the stage is disabled", err)
	}
}

func TestManagedKeysTemplateDefaultsScreenshotsOff(t *testing.T) {
	templateBytes, err := fs.ReadFile(assets.FS, "templates/workflow-config.yml")
	if err != nil {
		t.Fatal(err)
	}
	path := writeDocument(t, string(templateBytes))
	doc, err := workflowconfig.LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stageName := range []string{"browser_ui_validation", "qa_end_to_end"} {
		if doc.Config.Stages[stageName].Screenshots.Enabled {
			t.Errorf("template stages.%s.screenshots.enabled = true, want false", stageName)
		}
	}
	if doc.Config.Stages["qa_end_to_end"].UsePlaywright {
		t.Fatal("template qa_end_to_end.use_playwright must default to false")
	}
	if doc.Config.TestAccess.Enabled {
		t.Fatal("template test_access.enabled must default to false")
	}
	if !strings.Contains(string(templateBytes), "cumulative active wall-time budget") {
		t.Fatal("template must describe timeout_minutes as a cumulative active budget")
	}
}

func writeDocument(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workflow-config.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
