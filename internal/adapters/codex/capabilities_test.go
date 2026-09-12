package codex_test

import (
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/codex"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestCodexDoesNotImplementOptionalC16Capabilities(t *testing.T) {
	var a harness.HarnessAdapter = codex.NewAdapter(t.TempDir(), nil)
	if _, ok := a.(harness.RemoteHistoryReader); ok {
		t.Fatal("codex must not implement RemoteHistoryReader")
	}
	if _, ok := a.(harness.NativeSessionDeleter); ok {
		t.Fatal("codex must not implement NativeSessionDeleter")
	}
}
