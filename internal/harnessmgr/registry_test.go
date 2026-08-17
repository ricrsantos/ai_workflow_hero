package harnessmgr_test

import (
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
)

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
}
