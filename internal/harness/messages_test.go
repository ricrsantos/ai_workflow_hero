package harness

import (
	"strings"
	"testing"
)

func TestUnsupportedMarkerMessage(t *testing.T) {
	m := MarkerDir{Dir: ".claude", ToolID: "claude", Supported: false}
	msg := UnsupportedMarkerMessage(m, false)
	if !strings.Contains(msg, "⚠ Detected .claude/ but cli.tools does not include it") {
		t.Fatalf("message = %q", msg)
	}
	if !strings.Contains(msg, "→ Supported today: cursor, opencode, codex, claude.") {
		t.Fatalf("message = %q", msg)
	}
}

func TestUnsupportedMarkerMessage_InCLITools(t *testing.T) {
	m := MarkerDir{Dir: ".windsurf", ToolID: "windsurf", Supported: false}
	msg := UnsupportedMarkerMessage(m, true)
	if strings.Contains(msg, "cli.tools does not include it") {
		t.Fatalf("expected in-cli-tools variant, got %q", msg)
	}
	if !strings.Contains(msg, "⚠ Detected .windsurf/") {
		t.Fatalf("message = %q", msg)
	}
}

func TestSupportedToolIDs(t *testing.T) {
	ids := SupportedToolIDs()
	if len(ids) != 4 || ids[0] != "cursor" || ids[1] != "opencode" || ids[2] != "codex" || ids[3] != "claude" {
		t.Fatalf("SupportedToolIDs() = %v", ids)
	}
}
