package workflowconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
)

func TestBuildReproPolicyDefaultsToGoTestForGoProjects(t *testing.T) {
	p := BuildReproPolicy(Verification{}, true)
	if p.DefaultMode != findingrepro.ModeGoTest {
		t.Fatalf("default mode=%q", p.DefaultMode)
	}
	if !p.ModeAllowed(findingrepro.ModeGoTest, "qa") {
		t.Fatal("go_test must be allowed")
	}
	if p.ModeAllowed(findingrepro.ModeEvidence, "qa") {
		t.Fatal("evidence must stay closed for QA while an automated gate exists")
	}
	if !p.ModeAllowed(findingrepro.ModeEvidence, "browser_ui_validation") {
		t.Fatal("browser UI must keep the evidence escape hatch")
	}
}

func TestBuildReproPolicyFallsBackToEvidenceWithoutAutomatedMode(t *testing.T) {
	p := BuildReproPolicy(Verification{}, false)
	if p.DefaultMode != findingrepro.ModeEvidence {
		t.Fatalf("default mode=%q", p.DefaultMode)
	}
	if p.ModeAllowed(findingrepro.ModeGoTest, "qa") {
		t.Fatal("go_test must not be offered without a go module")
	}
	if _, err := p.ResolveMode("go_test", "qa"); err == nil {
		t.Fatal("go_test must be rejected with a diagnostic")
	}
}

func TestBuildReproPolicyCommandMode(t *testing.T) {
	v, err := ParseVerification([]byte(`
verification:
  repro:
    mode: command
    command: ["npm", "test", "--", "-t", "{{test}}", "{{package}}"]
    allow_evidence: false
`))
	if err != nil {
		t.Fatal(err)
	}
	p := BuildReproPolicy(v, false)
	if p.DefaultMode != findingrepro.ModeCommand {
		t.Fatalf("default mode=%q", p.DefaultMode)
	}
	if p.ModeAllowed(findingrepro.ModeEvidence, "browser_ui_validation") {
		t.Fatal("allow_evidence:false must also close the per-stage allowance")
	}
	argv, err := p.CommandArgv("src/checkout", "renders total")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"npm", "test", "--", "-t", "renders total", "src/checkout"}
	if len(argv) != len(want) {
		t.Fatalf("argv=%v", argv)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv=%v want %v", argv, want)
		}
	}
	// A suite-wide repro drops the placeholder-only argument instead of passing
	// an empty filter that would match every test.
	argv, err = p.CommandArgv("src/checkout", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range argv {
		if a == "" {
			t.Fatalf("argv keeps an empty token: %v", argv)
		}
	}
}

func TestReproPolicyForProjectPrefersExplicitConfig(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := "title: t\nverification:\n  repro:\n    mode: command\n    command: [\"pytest\", \"-k\", \"{{test}}\"]\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "workflow-config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	p := ReproPolicyForProject(dir, nil)
	if p.DefaultMode != findingrepro.ModeCommand {
		t.Fatalf("default mode=%q", p.DefaultMode)
	}
	if !p.ModeAllowed(findingrepro.ModeGoTest, "qa") {
		t.Fatal("a go project keeps go_test available alongside a configured command")
	}
}

func TestReproPolicyForProjectUnknownDirKeepsGoContract(t *testing.T) {
	p := ReproPolicyForProject("", nil)
	if p.DefaultMode != findingrepro.ModeGoTest {
		t.Fatalf("default mode=%q", p.DefaultMode)
	}
}
