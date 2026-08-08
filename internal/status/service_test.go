package status

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
)

func TestStatusFromStore(t *testing.T) {
	dir := t.TempDir()
	cycleDir := filepath.Join(dir, ".workflow-hero", "cycles", "current")
	_ = os.MkdirAll(cycleDir, 0o755)
	cfg := `title: S
objective: O
stages:
  research:
    enabled: true
    max_iterations: 1
    timeout_minutes: 5
    require_human_approval: false
`
	_ = os.WriteFile(filepath.Join(cycleDir, "workflow-config.yml"), []byte(cfg), 0o644)

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	svc.Close()

	ws, err := Run(Options{ProjectDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Stages) != 1 || !strings.Contains(ws.Stages[0].Name, "Research") {
		t.Fatalf("%+v", ws)
	}

	var b strings.Builder
	PrintTable(&b, ws)
	if !strings.Contains(b.String(), "Cycle C1") {
		t.Fatalf("table=%s", b.String())
	}
}
