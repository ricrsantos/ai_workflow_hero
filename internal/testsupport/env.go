// Package testsupport provides test-only helpers shared across packages. It is
// imported exclusively by *_test.go files so it never enters a production
// binary.
package testsupport

import (
	"os"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/lifecycle"
)

// Run executes the suite with the lifecycle relay endpoint removed from the
// environment. A live Hero TUI exports HERO_LIFECYCLE_EVENT_SOCKET to its
// harness descendants, so a test process inherits it and any cycle service it
// opens would publish synthetic lifecycle events to that live relay (and on to
// Telegram). Use from TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(testsupport.Run(m)) }
//
// Tests that exercise the notifier on purpose re-set the variable with
// t.Setenv, which overrides the unset for that test only.
func Run(m *testing.M) int {
	_ = os.Unsetenv(lifecycle.EventSocketEnv)
	return m.Run()
}
