package cycle

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reprotest"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func reproModeProject(t *testing.T, cfg string) (*store.Store, int64, string) {
	t.Helper()
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(dir, "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cycleID, err := s.CreateCycle(store.Cycle{Number: 1, Title: "t", Status: store.CycleStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	return s, cycleID, dir
}

func TestVerifyFindingReprosRunsConfiguredCommand(t *testing.T) {
	cfg := "title: t\nverification:\n  repro:\n    mode: command\n    command: [\"npm\", \"test\", \"--\", \"-t\", \"{{test}}\", \"{{package}}\"]\n"
	s, cycleID, dir := reproModeProject(t, cfg)
	res, err := s.PersistFinding(store.FindingInput{
		CycleID: cycleID, SourceStage: store.FindingSourceQA, Owner: store.FindingOwnerFrontend,
		File: "src/checkout.ts", Issue: "total is wrong", AcceptanceCriteria: "total matches cart",
		ReproMode: findingrepro.ModeCommand, ReproPackage: "src/checkout.test.ts", ReproTest: "renders total",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got reprotest.Spec
	restore := SetFindingReproRunForTest(func(_ context.Context, root string, spec reprotest.Spec) error {
		got = spec
		return nil
	})
	t.Cleanup(restore)
	if err := VerifyCompletedFindingRepros(context.Background(), s, cycleID, dir, []string{res.Finding.ID}); err != nil {
		t.Fatalf("gate err=%v", err)
	}
	if got.Mode != findingrepro.ModeCommand {
		t.Fatalf("spec=%+v", got)
	}
	want := "npm test -- -t renders total src/checkout.test.ts"
	if strings.Join(got.Argv, " ") != want {
		t.Fatalf("argv=%v want %q", got.Argv, want)
	}
}

func TestVerifyFindingReprosSkipsEvidenceFindings(t *testing.T) {
	cfg := "title: t\nverification:\n  repro:\n    mode: evidence\n    allow_evidence: true\n"
	s, cycleID, dir := reproModeProject(t, cfg)
	res, err := s.PersistFinding(store.FindingInput{
		CycleID: cycleID, SourceStage: store.FindingSourceBrowserUI, Owner: store.FindingOwnerFrontend,
		File: "src/app.css", Issue: "header overlaps", AcceptanceCriteria: "header does not overlap",
		Evidence:  []string{".workflow-hero/cycles/current/browser-ui/home.png"},
		ReproMode: findingrepro.ModeEvidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	restore := SetFindingReproRunForTest(func(context.Context, string, reprotest.Spec) error {
		called = true
		return nil
	})
	t.Cleanup(restore)
	if err := VerifyCompletedFindingRepros(context.Background(), s, cycleID, dir, []string{res.Finding.ID}); err != nil {
		t.Fatalf("gate err=%v", err)
	}
	if called {
		t.Fatal("an evidence-only finding has no command to re-run")
	}
}

func TestFindingAssignmentMarkdownPerMode(t *testing.T) {
	policy := findingrepro.Policy{
		DefaultMode:    findingrepro.ModeCommand,
		Modes:          map[string]bool{findingrepro.ModeCommand: true, findingrepro.ModeEvidence: true},
		Command:        []string{"npm", "test", "--", "-t", "{{test}}"},
		EvidenceStages: []string{"browser_ui_validation"},
	}
	command := store.Finding{
		ID: "find-qa-1", SourceStage: "qa", Issue: "total is wrong", AcceptanceCriteria: "total matches cart",
		File: "src/checkout.ts", ReproMode: findingrepro.ModeCommand,
		ReproPackage: "src/checkout.test.ts", ReproTest: "renders total",
	}
	md := FindingAssignmentMarkdown(command, nil, policy)
	if !strings.Contains(md, "Mode: command") || !strings.Contains(md, "npm test -- -t renders total") {
		t.Fatalf("command markdown=%q", md)
	}
	if strings.Contains(md, "go test") {
		t.Fatalf("command markdown must not tell the agent to run go test:\n%s", md)
	}

	evidence := store.Finding{
		ID: "find-bui-1", SourceStage: "browser_ui_validation", Issue: "header overlaps",
		AcceptanceCriteria: "header does not overlap", File: "src/app.css",
		ReproMode:    findingrepro.ModeEvidence,
		EvidenceJSON: `[".workflow-hero/cycles/current/browser-ui/home.png"]`,
	}
	md = FindingAssignmentMarkdown(evidence, nil, policy)
	if !strings.Contains(md, "evidence-only") || !strings.Contains(md, "home.png") {
		t.Fatalf("evidence markdown=%q", md)
	}

	goTest := store.Finding{
		ID: "find-qa-2", SourceStage: "qa", Issue: "handoff", AcceptanceCriteria: "gate holds",
		File: "internal/tui/stage_handoff.go", ReproMode: findingrepro.ModeGoTest,
		ReproPackage: "./internal/tui", ReproTest: "TestX",
	}
	md = FindingAssignmentMarkdown(goTest, nil, findingrepro.DefaultGoPolicy())
	if !strings.Contains(md, "go test ./internal/tui -count=1 -run ^TestX$") {
		t.Fatalf("go markdown=%q", md)
	}
}
