package common_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
)

// TestRuntimeAssets_StageOrder verifies the stage order keywords appear in at least
// one command/agent asset. This validates ADR-011 semantics are present in stubs.
func TestRuntimeAssets_StageOrder(t *testing.T) {
	// The canonical stage order must appear in at least one asset.
	keywords := []string{
		"Configuration", "Research", "Planning", "Implementation",
		"QA", "Judge", "Browser UI Validation", "QA End-to-End",
	}

	allContent := loadAllAssetContent(t, "cursor")
	for _, kw := range keywords {
		if !strings.Contains(allContent, kw) {
			t.Errorf("stage keyword %q not found in any Runtime asset", kw)
		}
	}
}

// TestRuntimeAssets_ApproveRejectCancelFinish verifies control loop commands are documented.
func TestRuntimeAssets_ApproveRejectCancelFinish(t *testing.T) {
	keywords := []string{
		"/hero:approve", "/hero:reject", "/hero:cancel", "/hero:finish",
	}
	allContent := loadAllAssetContent(t, "cursor")
	for _, kw := range keywords {
		if !strings.Contains(allContent, kw) {
			t.Errorf("control loop command %q not found in any Runtime asset", kw)
		}
	}
}

// TestRuntimeAssets_ContinueAndBack verifies iteration/escalation commands are present.
func TestRuntimeAssets_ContinueAndBack(t *testing.T) {
	keywords := []string{"/hero:continue", "/hero:back"}
	allContent := loadAllAssetContent(t, "cursor")
	for _, kw := range keywords {
		if !strings.Contains(allContent, kw) {
			t.Errorf("command %q not found in any Runtime asset", kw)
		}
	}
}

// TestRuntimeAssets_ScopeRouting verifies scope routing semantics are documented.
func TestRuntimeAssets_ScopeRouting(t *testing.T) {
	keywords := []string{"backend_agent", "frontend_agent", "generic_agent", "scope"}
	allContent := loadAllAssetContent(t, "cursor")
	for _, kw := range keywords {
		if !strings.Contains(allContent, kw) {
			t.Errorf("scope routing keyword %q not found in any Runtime asset", kw)
		}
	}
}

// TestRuntimeAssets_Fallback verifies model fallback semantics appear in assets.
func TestRuntimeAssets_Fallback(t *testing.T) {
	// The word "fallback" or "fallback_model" must appear.
	allContent := loadAllAssetContent(t, "cursor")
	if !strings.Contains(strings.ToLower(allContent), "fallback") &&
		!strings.Contains(allContent, "fallback_model") {
		t.Error("model fallback semantics not found in any Runtime asset")
	}
}

// TestRuntimeAssets_AgentFrontmatter verifies every Cursor agent file has YAML frontmatter
// with name, description, and model: inherit (effective model comes from workflow-config via Task).
func TestRuntimeAssets_AgentFrontmatter(t *testing.T) {
	agents := []string{
		"orchestration_agent", "discover_agent", "planning_agent", "context_agent",
		"backend_agent", "frontend_agent", "generic_agent",
		"qa_agent", "judge_agent", "browser_ui_agent", "end2end_qa_agent",
	}
	for _, agent := range agents {
		path := "cursor/agents/" + agent + ".md"
		data, err := fs.ReadFile(assets.FS, path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		content := string(data)
		if !strings.HasPrefix(content, "---\n") {
			t.Errorf("%s must start with YAML frontmatter (---)", path)
			continue
		}
		end := strings.Index(content[4:], "\n---")
		if end < 0 {
			t.Errorf("%s missing closing frontmatter ---", path)
			continue
		}
		fm := content[4 : 4+end]
		if !strings.Contains(fm, "name: "+agent) {
			t.Errorf("%s frontmatter missing name: %s", path, agent)
		}
		if !strings.Contains(fm, "description:") {
			t.Errorf("%s frontmatter missing description:", path)
		}
		if !strings.Contains(fm, "model: inherit") {
			t.Errorf("%s frontmatter missing model: inherit", path)
		}
	}
}

// TestRuntimeAssets_ModelResolution verifies orchestrator and hero-start encode mandatory
// Task tool model parameter resolution from workflow-config.yml as kebab slugs (not brackets).
func TestRuntimeAssets_ModelResolution(t *testing.T) {
	orch, err := fs.ReadFile(assets.FS, "cursor/agents/orchestration_agent.md")
	if err != nil {
		t.Fatalf("read orchestration_agent: %v", err)
	}
	orchStr := string(orch)
	for _, kw := range []string{
		"Model Resolution",
		"workflow-config.yml",
		"enable_fast_model",
		"fallback_model",
		"kebab",
		"-fast",
		"never omit",
		"never use",
		"brackets",
		"model",
	} {
		if !strings.Contains(orchStr, kw) {
			t.Errorf("orchestration_agent.md missing Model Resolution keyword %q", kw)
		}
	}
	if strings.Contains(orchStr, "<id>[fast=") {
		t.Error("orchestration_agent.md must not instruct bracket Task model syntax")
	}
	if !strings.Contains(orchStr, "Task tool") {
		t.Error("orchestration_agent.md must require Task tool model parameter")
	}

	start, err := fs.ReadFile(assets.FS, "cursor/commands/hero-start.md")
	if err != nil {
		t.Fatalf("read hero-start: %v", err)
	}
	startStr := string(start)
	for _, kw := range []string{"Model Resolution", "workflow-config.yml", "enable_fast_model", "kebab", "never omit", "never pass"} {
		if !strings.Contains(startStr, kw) {
			t.Errorf("hero-start.md missing Model Resolution keyword %q", kw)
		}
	}
	if strings.Contains(startStr, "[fast=true]") || strings.Contains(startStr, "[fast=false]") {
		t.Error("hero-start.md must not instruct bracket Task model syntax")
	}
}

// TestRuntimeAssets_Metrics verifies metrics-related keywords and executable procedure appear.
func TestRuntimeAssets_Metrics(t *testing.T) {
	keywords := []string{"metrics.md", "metrics-summary.md"}
	allContent := loadAllAssetContent(t, "cursor")
	for _, kw := range keywords {
		if !strings.Contains(allContent, kw) {
			t.Errorf("metrics keyword %q not found in any Runtime asset", kw)
		}
	}

	orch, err := fs.ReadFile(assets.FS, "cursor/agents/orchestration_agent.md")
	if err != nil {
		t.Fatalf("read orchestration_agent: %v", err)
	}
	orchStr := string(orch)
	for _, kw := range []string{"Metrics Procedure", "input_chars", "1_000_000", "per_1m_tokens", "Duration:", "Total:"} {
		if !strings.Contains(orchStr, kw) {
			t.Errorf("orchestration_agent.md missing Metrics Procedure keyword %q", kw)
		}
	}
	if !strings.Contains(orchStr, "show the stage metrics summary in the chat") &&
		!strings.Contains(orchStr, "required in chat every stage close") {
		t.Error("orchestration_agent.md must require showing metrics (tokens + duration) in chat")
	}
	if !strings.Contains(orchStr, "[.workflow-hero/cycles/current/metrics.md](.workflow-hero/cycles/current/metrics.md)") {
		t.Error("orchestration_agent.md must include a clickable markdown link to metrics.md")
	}

	newCmd, err := fs.ReadFile(assets.FS, "cursor/commands/hero-new.md")
	if err != nil {
		t.Fatalf("read hero-new: %v", err)
	}
	if !strings.Contains(string(newCmd), "[.workflow-hero/cycles/current/workflow-config.yml](.workflow-hero/cycles/current/workflow-config.yml)") {
		t.Error("hero-new.md must include a clickable markdown link to workflow-config.yml")
	}

	approve, err := fs.ReadFile(assets.FS, "cursor/commands/hero-approve.md")
	if err != nil {
		t.Fatalf("read hero-approve: %v", err)
	}
	approveStr := string(approve)
	if strings.Contains(approveStr, "final iteration count for the stage") {
		t.Error("hero-approve.md still limits metrics update to iteration count only")
	}
	if !strings.Contains(approveStr, "Metrics Procedure") {
		t.Error("hero-approve.md missing Metrics Procedure reference")
	}
	if !strings.Contains(approveStr, "[.workflow-hero/cycles/current/metrics.md](.workflow-hero/cycles/current/metrics.md)") {
		t.Error("hero-approve.md must include a clickable markdown link to metrics.md")
	}

	taskAgents := []string{
		"discover_agent", "planning_agent", "backend_agent", "frontend_agent",
		"generic_agent", "qa_agent", "judge_agent", "browser_ui_agent", "end2end_qa_agent", "context_agent",
	}
	for _, agent := range taskAgents {
		path := "cursor/agents/" + agent + ".md"
		data, err := fs.ReadFile(assets.FS, path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		content := string(data)
		if !strings.Contains(content, `"metrics"`) && !strings.Contains(content, `"input_chars"`) {
			t.Errorf("%s missing metrics/input_chars in output schema", path)
		}
		if !strings.Contains(content, "input_chars") {
			t.Errorf("%s missing input_chars", path)
		}
	}

	tmpl, err := fs.ReadFile(assets.FS, "templates/metrics.md")
	if err != nil {
		t.Fatalf("read metrics template: %v", err)
	}
	tmplStr := string(tmpl)
	if !strings.Contains(tmplStr, "1_000_000") {
		t.Error("templates/metrics.md missing explicit cost formula (1_000_000)")
	}
}

// TestRuntimeAssets_TaskIsolation verifies Task tool isolation semantics appear.
func TestRuntimeAssets_TaskIsolation(t *testing.T) {
	// ADR-005: subagents invoked via Task tool in fresh sessions.
	allContent := loadAllAssetContent(t, "cursor")
	hasTaskTool := strings.Contains(allContent, "Task tool") || strings.Contains(allContent, "isolated session")
	if !hasTaskTool {
		t.Error("Task tool isolation semantics not found in any Runtime asset")
	}
}

// TestRuntimeAssets_ImplementationParallelism verifies planning/orchestration/impl agents
// encode parallel Task dispatch and nested fan-out guidance.
func TestRuntimeAssets_ImplementationParallelism(t *testing.T) {
	planning, err := fs.ReadFile(assets.FS, "cursor/agents/planning_agent.md")
	if err != nil {
		t.Fatalf("read planning_agent: %v", err)
	}
	planStr := string(planning)
	for _, kw := range []string{"parallel", "series", "parallel_groups", "subagent"} {
		if !strings.Contains(strings.ToLower(planStr), strings.ToLower(kw)) && !strings.Contains(planStr, kw) {
			t.Errorf("planning_agent.md missing parallelism keyword %q", kw)
		}
	}
	if !strings.Contains(planStr, "parallel_groups") {
		t.Error("planning_agent.md missing parallel_groups in output schema")
	}

	orch, err := fs.ReadFile(assets.FS, "cursor/agents/orchestration_agent.md")
	if err != nil {
		t.Fatalf("read orchestration_agent: %v", err)
	}
	orchStr := string(orch)
	for _, kw := range []string{"Implementation Parallelism", "Task tool", "parallel"} {
		if !strings.Contains(orchStr, kw) {
			t.Errorf("orchestration_agent.md missing keyword %q", kw)
		}
	}

	implAgents := []string{"backend_agent", "frontend_agent", "generic_agent"}
	for _, agent := range implAgents {
		path := "cursor/agents/" + agent + ".md"
		data, err := fs.ReadFile(assets.FS, path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		content := string(data)
		for _, kw := range []string{"Parallelism / nested Task", "Task tool", "context_agent", "independent"} {
			if !strings.Contains(content, kw) {
				t.Errorf("%s missing parallelism keyword %q", path, kw)
			}
		}
	}
}

// TestRuntimeAssets_LoggingStandard verifies implementation agents require leveled
// logging (error/info/debug, default info) and qa_agent checks for it.
func TestRuntimeAssets_LoggingStandard(t *testing.T) {
	implAgents := []string{"backend_agent", "frontend_agent", "generic_agent"}
	for _, agent := range implAgents {
		path := "cursor/agents/" + agent + ".md"
		data, err := fs.ReadFile(assets.FS, path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		content := string(data)
		for _, kw := range []string{
			"## Logging",
			"**Levels** (only these): `error`, `info`, `debug`",
			"**Default level**: `info`",
			"NEVER skip the Logging standard",
			"NEVER log secrets",
		} {
			if !strings.Contains(content, kw) {
				t.Errorf("%s missing logging keyword %q", path, kw)
			}
		}
	}

	qa, err := fs.ReadFile(assets.FS, "cursor/agents/qa_agent.md")
	if err != nil {
		t.Fatalf("read qa_agent: %v", err)
	}
	qaStr := string(qa)
	for _, kw := range []string{
		"Logging implementation",
		"error",
		"info",
		"debug",
		"default level `info`",
		`"logging"`,
	} {
		if !strings.Contains(qaStr, kw) {
			t.Errorf("qa_agent.md missing logging check keyword %q", kw)
		}
	}
}

// TestRuntimeAssets_ResearchPreDocumentGate verifies discover_agent and grilling skill
// require asking for extra info before document generation.
func TestRuntimeAssets_ResearchPreDocumentGate(t *testing.T) {
	discover, err := fs.ReadFile(assets.FS, "cursor/agents/discover_agent.md")
	if err != nil {
		t.Fatalf("read discover_agent: %v", err)
	}
	discStr := string(discover)
	for _, kw := range []string{"Pre-document gate", "before generating", "pre_document_additions", "additions_summary"} {
		if !strings.Contains(discStr, kw) {
			t.Errorf("discover_agent.md missing Pre-document gate keyword %q", kw)
		}
	}

	skill, err := fs.ReadFile(assets.FS, "cursor/skills/grilling/SKILL.md")
	if err != nil {
		t.Fatalf("read grilling skill: %v", err)
	}
	skillStr := string(skill)
	for _, kw := range []string{"Pre-document gate", "add any more information", "evaluates the additions"} {
		if !strings.Contains(skillStr, kw) {
			t.Errorf("grilling/SKILL.md missing Pre-document gate keyword %q", kw)
		}
	}
}

// TestRuntimeAssets_CleanSessionHandoff verifies new/start encode the soft clean-chat
// handoff (new empty chat + select orchestrator/grill-me) and disk-only bootstrap.
func TestRuntimeAssets_CleanSessionHandoff(t *testing.T) {
	newCmd, err := fs.ReadFile(assets.FS, "cursor/commands/hero-new.md")
	if err != nil {
		t.Fatalf("read hero-new: %v", err)
	}
	newStr := string(newCmd)
	for _, kw := range []string{
		"Clean Session Handoff",
		"new empty chat",
		"orchestrator / grill-me",
		"/hero:start",
	} {
		if !strings.Contains(newStr, kw) {
			t.Errorf("hero-new.md missing Clean Session Handoff keyword %q", kw)
		}
	}

	start, err := fs.ReadFile(assets.FS, "cursor/commands/hero-start.md")
	if err != nil {
		t.Fatalf("read hero-start: %v", err)
	}
	startStr := string(start)
	for _, kw := range []string{
		"Session Bootstrap",
		"Do **not** rely on prior chat history",
		"workflow-config.yml",
		"orchestrator / grill-me",
		"new empty chat",
	} {
		if !strings.Contains(startStr, kw) {
			t.Errorf("hero-start.md missing Session Bootstrap keyword %q", kw)
		}
	}
}

// TestRuntimeAssets_PreviousCycleConfigImport verifies /hero:new mandates importing
// fallback_model + stages + agents from the prior cycle while resetting cycle-specific fields.
func TestRuntimeAssets_PreviousCycleConfigImport(t *testing.T) {
	newCmd, err := fs.ReadFile(assets.FS, "cursor/commands/hero-new.md")
	if err != nil {
		t.Fatalf("read hero-new: %v", err)
	}
	newStr := string(newCmd)
	for _, kw := range []string{
		"Previous Cycle Config Import",
		"fallback_model",
		"stages",
		"agents",
		"title",
		"objective",
		"scope",
		"always",
		"deep-merge",
		".workflow-hero/templates/workflow-config.yml",
	} {
		if !strings.Contains(newStr, kw) {
			t.Errorf("hero-new.md missing Previous Cycle Config Import keyword %q", kw)
		}
	}
	if !strings.Contains(newStr, "never copy those three") && !strings.Contains(newStr, "Never copy those three") {
		t.Error("hero-new.md must forbid copying title/objective/scope from the previous cycle")
	}

	orch, err := fs.ReadFile(assets.FS, "cursor/agents/orchestration_agent.md")
	if err != nil {
		t.Fatalf("read orchestration_agent: %v", err)
	}
	if !strings.Contains(string(orch), "Previous Cycle Config Import") {
		t.Error("orchestration_agent.md must reference Previous Cycle Config Import")
	}

	help, err := fs.ReadFile(assets.FS, "docs/workflow-help.md")
	if err != nil {
		t.Fatalf("read workflow-help: %v", err)
	}
	helpStr := string(help)
	if !strings.Contains(helpStr, "always imports") {
		t.Error("workflow-help.md (EN) must document always imports of prior cycle config")
	}
	if !strings.Contains(helpStr, "sempre importa") {
		t.Error("workflow-help.md (PT) must document sempre importa of prior cycle config")
	}
}

// TestRuntimeAssets_BrowserUIValidation verifies browser_ui_agent and orchestration encode
// Health/Visual semantics, gates, artifacts path, and stage order.
func TestRuntimeAssets_BrowserUIValidation(t *testing.T) {
	agent, err := fs.ReadFile(assets.FS, "cursor/agents/browser_ui_agent.md")
	if err != nil {
		t.Fatalf("read browser_ui_agent: %v", err)
	}
	agentStr := string(agent)
	for _, kw := range []string{
		"Browser Health",
		"Visual Validation",
		"Playwright",
		"1280",
		"768",
		"375",
		".workflow-hero/cycles/current/browser-ui/",
		"failure_class",
		"health-report.md",
		"model: inherit",
		`"input_chars"`,
	} {
		if !strings.Contains(agentStr, kw) {
			t.Errorf("browser_ui_agent.md missing keyword %q", kw)
		}
	}

	orch, err := fs.ReadFile(assets.FS, "cursor/agents/orchestration_agent.md")
	if err != nil {
		t.Fatalf("read orchestration_agent: %v", err)
	}
	orchStr := string(orch)
	for _, kw := range []string{
		"Browser UI Validation",
		"browser_ui_agent",
		"scope.frontend",
		"Health failure skips Visual",
		"frontend_agent",
		"backend_agent",
		".workflow-hero/cycles/current/browser-ui/",
	} {
		if !strings.Contains(orchStr, kw) {
			t.Errorf("orchestration_agent.md missing Browser UI keyword %q", kw)
		}
	}

	start, err := fs.ReadFile(assets.FS, "cursor/commands/hero-start.md")
	if err != nil {
		t.Fatalf("read hero-start: %v", err)
	}
	if !strings.Contains(string(start), "stages.browser_ui_validation.enabled") {
		t.Error("hero-start.md must validate browser_ui_validation.enabled against scope.frontend")
	}

	canonical := "QA → Judge → Browser UI Validation → QA End-to-End"
	if !strings.Contains(orchStr, canonical) {
		t.Errorf("orchestration_agent.md missing canonical stage order fragment %q", canonical)
	}

	e2e, err := fs.ReadFile(assets.FS, "cursor/agents/end2end_qa_agent.md")
	if err != nil {
		t.Fatalf("read end2end_qa_agent: %v", err)
	}
	e2eStr := string(e2e)
	if !strings.Contains(e2eStr, "business journeys") && !strings.Contains(e2eStr, "Browser UI Validation") {
		t.Error("end2end_qa_agent.md should keep journey semantics distinct from Browser UI Validation")
	}

	help, err := fs.ReadFile(assets.FS, "docs/workflow-help.md")
	if err != nil {
		t.Fatalf("read workflow-help: %v", err)
	}
	helpStr := string(help)
	for _, kw := range []string{
		"Browser UI Validation",
		"visual_validation",
		"docs/ui/visual_reference",
		".workflow-hero/cycles/current/browser-ui/",
		"browser_ui_agent",
	} {
		if !strings.Contains(helpStr, kw) {
			t.Errorf("workflow-help.md missing Browser UI keyword %q", kw)
		}
	}

	metrics, err := fs.ReadFile(assets.FS, "templates/metrics.md")
	if err != nil {
		t.Fatalf("read metrics template: %v", err)
	}
	if !strings.Contains(string(metrics), "Browser UI Validation") || !strings.Contains(string(metrics), "browser_ui_agent") {
		t.Error("templates/metrics.md must include Browser UI Validation row")
	}

	workflow, err := fs.ReadFile(assets.FS, "templates/workflow.md")
	if err != nil {
		t.Fatalf("read workflow template: %v", err)
	}
	if !strings.Contains(string(workflow), "Browser UI Validation") {
		t.Error("templates/workflow.md must include Browser UI Validation stage row")
	}
}

// TestRuntimeAssets_ArchiveUsesCompletedDate verifies archive naming uses workflow.md
// Completed date (cycle completion), not a hallucinated "today".
func TestRuntimeAssets_ArchiveUsesCompletedDate(t *testing.T) {
	archive, err := fs.ReadFile(assets.FS, "cursor/commands/hero-archive.md")
	if err != nil {
		t.Fatalf("read hero-archive: %v", err)
	}
	archStr := string(archive)
	for _, kw := range []string{
		"cycle completion date",
		"Completed",
		"date +%Y-%m-%d",
		"Never",
		"hallucinate",
	} {
		if !strings.Contains(archStr, kw) {
			t.Errorf("hero-archive.md missing archive-date keyword %q", kw)
		}
	}

	finish, err := fs.ReadFile(assets.FS, "cursor/commands/hero-finish.md")
	if err != nil {
		t.Fatalf("read hero-finish: %v", err)
	}
	finStr := string(finish)
	for _, kw := range []string{"Completed", "date +%Y-%m-%d", "archive folder name"} {
		if !strings.Contains(finStr, kw) {
			t.Errorf("hero-finish.md missing Completed-date keyword %q", kw)
		}
	}

	tmpl, err := fs.ReadFile(assets.FS, "templates/workflow.md")
	if err != nil {
		t.Fatalf("read workflow template: %v", err)
	}
	tmplStr := string(tmpl)
	for _, kw := range []string{"**Started**", "**Completed**", "**Status**"} {
		if !strings.Contains(tmplStr, kw) {
			t.Errorf("templates/workflow.md missing header field %q", kw)
		}
	}
}

// loadAllAssetContent concatenates the content of all files under dirPrefix in the embedded FS.
func loadAllAssetContent(t *testing.T, dirPrefix string) string {
	t.Helper()
	var sb strings.Builder
	err := fs.WalkDir(assets.FS, dirPrefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(assets.FS, path)
		if err != nil {
			return err
		}
		sb.Write(data)
		sb.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatalf("walk assets: %v", err)
	}
	return sb.String()
}
