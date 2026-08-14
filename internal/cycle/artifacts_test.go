package cycle_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestServiceArtifacts(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	res, err := svc.NewCycle("", "")
	if err != nil {
		t.Fatal(err)
	}

	view, err := svc.Artifacts()
	if err != nil {
		t.Fatal(err)
	}
	if view.CycleNumber != res.Cycle.Number {
		t.Fatalf("cycle number = %d", view.CycleNumber)
	}
	foundConfig := false
	for _, a := range view.Artifacts {
		if a.Path == ".workflow-hero/cycles/current/workflow-config.yml" {
			foundConfig = true
			break
		}
	}
	if !foundConfig {
		t.Fatalf("expected cycle config artifact, got %+v", view.Artifacts)
	}

	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Store.AddArtifact(store.Artifact{
		CycleID: c.ID, Path: "docs/product/PRD.md", Kind: "prd", Label: "PRD",
	}); err != nil {
		t.Fatal(err)
	}

	view, err = svc.Artifacts()
	if err != nil {
		t.Fatal(err)
	}
	foundPRD := false
	for _, a := range view.Artifacts {
		if a.Path == "docs/product/PRD.md" && a.Label == "PRD" {
			foundPRD = true
			break
		}
	}
	if !foundPRD {
		t.Fatalf("expected registered PRD, got %+v", view.Artifacts)
	}

	// No active cycle returns empty list.
	_ = svc.Finish("")
	empty, err := svc.Artifacts()
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Artifacts) != 0 {
		t.Fatalf("expected empty without active cycle, got %+v", empty.Artifacts)
	}
}

func TestServiceArtifactsNoProject(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".workflow-hero", "config"), 0o755)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	view, err := svc.Artifacts()
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Artifacts) != 0 {
		t.Fatal("expected empty artifacts without cycle")
	}
}
