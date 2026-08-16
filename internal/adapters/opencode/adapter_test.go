package opencode_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/opencode"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestAdapter_Name(t *testing.T) {
	a := opencode.NewAdapter(t.TempDir(), nil)
	if a.Name() != "opencode" {
		t.Fatalf("name=%q", a.Name())
	}
}

func TestAdapter_IsAvailableWithoutCLI(t *testing.T) {
	a := opencode.NewAdapter(t.TempDir(), nil)
	a.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	if err := a.IsAvailable(context.Background()); err == nil {
		t.Fatal("expected error when CLI missing")
	}
}

func TestAdapter_ImplementsContract(t *testing.T) {
	var _ harness.HarnessAdapter = (*opencode.Adapter)(nil)
	var _ harness.ModelLister = (*opencode.Adapter)(nil)
}
