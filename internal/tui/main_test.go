package tui

import (
	"os"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/testsupport"
)

func TestMain(m *testing.M) {
	os.Exit(testsupport.Run(m))
}
