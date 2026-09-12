package cursor_test

import (
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestCursorDoesNotImplementOptionalC16Capabilities(t *testing.T) {
	var a harness.HarnessAdapter = cursoradapter.NewAdapter(t.TempDir())
	if _, ok := a.(harness.RemoteHistoryReader); ok {
		t.Fatal("cursor must not implement RemoteHistoryReader")
	}
	if _, ok := a.(harness.NativeSessionDeleter); ok {
		t.Fatal("cursor must not implement NativeSessionDeleter")
	}
}
