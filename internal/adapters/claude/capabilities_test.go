package claude_test

import (
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/claude"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestClaudeDoesNotImplementOptionalC16Capabilities(t *testing.T) {
	var a harness.HarnessAdapter = claude.NewAdapter(t.TempDir())
	if _, ok := a.(harness.RemoteHistoryReader); ok {
		t.Fatal("claude must not implement RemoteHistoryReader")
	}
	if _, ok := a.(harness.NativeSessionDeleter); ok {
		t.Fatal("claude must not implement NativeSessionDeleter")
	}
}
