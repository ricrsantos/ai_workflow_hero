package cycle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestServiceApprovalsHistory(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}

	view, err := svc.Approvals()
	if err != nil {
		t.Fatal(err)
	}
	if view.CycleNumber != 1 {
		t.Fatalf("cycle = %d", view.CycleNumber)
	}
	if view.Pending != "" || len(view.Entries) != 0 {
		t.Fatalf("expected no approval activity, got %+v", view)
	}

	if err := svc.StartStage("research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("research", "done", `{"agent":"discover_agent","input_tokens":4}`, false); err != nil {
		t.Fatal(err)
	}
	view, err = svc.Approvals()
	if err != nil {
		t.Fatal(err)
	}
	if view.Pending != "" || len(view.Entries) != 0 {
		t.Fatalf("auto-complete research must not appear: %+v", view)
	}

	if err := svc.StartStage("qa"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("qa", "ready", "", false); err != nil {
		t.Fatal(err)
	}
	view, err = svc.Approvals()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(view.Pending, "Qa") {
		t.Fatalf("pending = %q", view.Pending)
	}
	if len(view.Entries) != 1 || view.Entries[0].Event != "requested" || !strings.EqualFold(view.Entries[0].Stage, "Qa") {
		t.Fatalf("requested entry: %+v", view.Entries)
	}

	if err := svc.Approve("lgtm", `{"agent":"qa_agent","input_tokens":2}`); err != nil {
		t.Fatal(err)
	}
	view, err = svc.Approvals()
	if err != nil {
		t.Fatal(err)
	}
	if view.Pending != "" {
		t.Fatalf("pending after approve = %q", view.Pending)
	}
	if len(view.Entries) != 2 {
		t.Fatalf("entries: %+v", view.Entries)
	}
	if view.Entries[1].Event != "approved" {
		t.Fatalf("second event = %+v", view.Entries[1])
	}
}

func TestServiceApprovalsNoCycle(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".workflow-hero", "config"), 0o755)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	view, err := svc.Approvals()
	if err != nil {
		t.Fatal(err)
	}
	if view.CycleNumber != 0 || len(view.Entries) != 0 {
		t.Fatalf("expected empty: %+v", view)
	}
}

func TestServiceArtifactsDiscoversCycleFiles(t *testing.T) {
	dir := setupProject(t)
	prdDir := filepath.Join(dir, "docs", "product")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prdPath := filepath.Join(prdDir, "PRD-C01-001-test.md")
	if err := os.WriteFile(prdPath, []byte("# PRD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	specDir := filepath.Join(dir, "openspec", "changes", "slash-parity-tui-harness")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "proposal.md"), []byte("# proposal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	docsJSON := `{
  "documents": [
    {"category":"PRD","cycle":"C01","path":"docs/product/PRD-C01-001-test.md","title":"Hero 1.0 PRD"},
    {"category":"ADR","cycle":"C02","path":"docs/architecture/ADR-C02-001.md","title":"Other cycle"}
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, ".workflow-hero", "config", "documents.json"), []byte(docsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetOpenspecChange("slash-parity-tui-harness"); err != nil {
		t.Fatal(err)
	}

	view, err := svc.Artifacts()
	if err != nil {
		t.Fatal(err)
	}
	if view.CycleNumber != 1 {
		t.Fatalf("cycle = %d", view.CycleNumber)
	}

	paths := map[string]store.Artifact{}
	for _, a := range view.Artifacts {
		paths[a.Path] = a
	}
	if _, ok := paths[".workflow-hero/cycles/current/workflow-config.yml"]; !ok {
		t.Fatalf("missing cycle config: %+v", view.Artifacts)
	}
	prd, ok := paths["docs/product/PRD-C01-001-test.md"]
	if !ok {
		t.Fatalf("missing PRD: %+v", view.Artifacts)
	}
	if prd.Label != "Hero 1.0 PRD" {
		t.Fatalf("PRD label = %q", prd.Label)
	}
	if _, ok := paths["openspec/changes/slash-parity-tui-harness/proposal.md"]; !ok {
		t.Fatalf("missing OpenSpec proposal: %+v", view.Artifacts)
	}
	for _, a := range view.Artifacts {
		if a.Path == "docs/architecture/ADR-C02-001.md" {
			t.Fatal("must not include other-cycle documents.json entries")
		}
	}
}

func TestServiceArtifactsMergesStoreWithoutDuplicate(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Store.AddArtifact(store.Artifact{
		CycleID: c.ID, Path: ".workflow-hero/cycles/current/workflow-config.yml", Kind: "config", Label: "Workflow config",
	}); err != nil {
		t.Fatal(err)
	}
	view, err := svc.Artifacts()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, a := range view.Artifacts {
		if a.Path == ".workflow-hero/cycles/current/workflow-config.yml" {
			n++
			if a.Label != "Workflow config" {
				t.Fatalf("store label should win: %+v", a)
			}
		}
	}
	if n != 1 {
		t.Fatalf("expected one config row, got %d in %+v", n, view.Artifacts)
	}
}
