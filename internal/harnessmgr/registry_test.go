package harnessmgr_test

import (
	"slices"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func TestDefaultRegistry_SupportedIDsIncludesCodex(t *testing.T) {
	reg := harnessmgr.NewRegistry(t.TempDir(), nil)
	got := reg.SupportedIDs()
	want := install.SupportedHarnessIDs
	if !slices.Equal(got, want) {
		t.Fatalf("SupportedIDs() = %v, want %v", got, want)
	}
	if !slices.Contains(got, "codex") || !slices.Contains(got, "cursor") || !slices.Contains(got, "opencode") {
		t.Fatalf("SupportedIDs missing expected harnesses: %v", got)
	}
}

func TestDefaultRegistry_CachesAdapters(t *testing.T) {
	reg := harnessmgr.NewRegistry(t.TempDir(), nil)

	op1, err := reg.Adapter("opencode")
	if err != nil {
		t.Fatal(err)
	}
	op2, err := reg.Adapter("opencode")
	if err != nil {
		t.Fatal(err)
	}
	if op1 != op2 {
		t.Fatal("expected same opencode adapter instance")
	}

	cur1, err := reg.Adapter("cursor")
	if err != nil {
		t.Fatal(err)
	}
	cur2, err := reg.Adapter("cursor")
	if err != nil {
		t.Fatal(err)
	}
	if cur1 != cur2 {
		t.Fatal("expected same cursor adapter instance")
	}
	if op1 == cur1 {
		t.Fatal("opencode and cursor adapters must be distinct instances")
	}

	cx1, err := reg.Adapter("codex")
	if err != nil {
		t.Fatal(err)
	}
	cx2, err := reg.Adapter("codex")
	if err != nil {
		t.Fatal(err)
	}
	if cx1 != cx2 {
		t.Fatal("expected same codex adapter instance")
	}
	if cx1.Name() != "codex" {
		t.Fatalf("Name() = %q, want codex", cx1.Name())
	}
	if cx1 == op1 || cx1 == cur1 {
		t.Fatal("codex adapter must be distinct from cursor/opencode")
	}
}
