package codex_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/codex"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
)

func TestErrorCopy_AuthMissingGolden(t *testing.T) {
	got := (&codex.AuthError{Err: errors.New("account/read: no account")}).Error()
	assertGolden(t, "auth_missing.golden", got)
	if strings.Contains(strings.ToLower(got), "api key") {
		t.Fatal("must not prompt for API key")
	}
	if strings.Contains(got, "detail:") {
		t.Fatal("user-facing auth copy must not dump internal detail")
	}
}

func TestErrorCopy_CLIMissingGolden(t *testing.T) {
	attempts := []harnessmgr.FallbackAttempt{{
		AgentName: "agent",
		HarnessID: "codex",
		Model:     "gpt-5.4",
		Err:       errors.New("codex CLI not on PATH: exec: \"codex\": executable file not found in $PATH"),
	}}
	got := harnessmgr.FormatHardStop("planning_agent", "codex", "gpt-5.4", attempts)
	assertGolden(t, "cli_missing.golden", got)
}

func TestErrorCopy_AppServerStartFailureGolden(t *testing.T) {
	got := (&codex.AppServerError{
		Message: "Codex app-server failed to start (incompatible or not installed).",
		Err:     errors.New("initialize: EOF"),
	}).Error()
	assertGolden(t, "app_server_start_failure.golden", got)
	if strings.Contains(got, "detail:") {
		t.Fatal("user-facing app-server copy must not dump internal detail")
	}
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	want := strings.TrimSuffix(string(wantBytes), "\n")
	got = strings.TrimSuffix(got, "\n")
	if got != want {
		t.Fatalf("%s mismatch\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}
