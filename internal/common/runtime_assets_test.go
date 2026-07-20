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
		"QA", "Judge", "QA End-to-End",
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
	// The word "fallback" or "generic_model" must appear.
	allContent := loadAllAssetContent(t, "cursor")
	if !strings.Contains(strings.ToLower(allContent), "fallback") &&
		!strings.Contains(allContent, "generic_model") {
		t.Error("model fallback semantics not found in any Runtime asset")
	}
}

// TestRuntimeAssets_Metrics verifies metrics-related keywords appear.
func TestRuntimeAssets_Metrics(t *testing.T) {
	keywords := []string{"metrics.md", "metrics-summary.md"}
	allContent := loadAllAssetContent(t, "cursor")
	for _, kw := range keywords {
		if !strings.Contains(allContent, kw) {
			t.Errorf("metrics keyword %q not found in any Runtime asset", kw)
		}
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
