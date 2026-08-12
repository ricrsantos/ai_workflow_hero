package tui

import (
	"strings"
	"testing"
)

func TestFormatChatAgentText_StripsMarkdownAndHandoff(t *testing.T) {
	raw := `→ Preparing workflow-config.yml...
✓ Cycle C4 initialized.

| Field | Value |
|---|---|
| Languages | Go |

Review [.workflow-hero/cycles/current/workflow-config.yml](.workflow-hero/cycles/current/workflow-config.yml)

→ Next (clean session handoff):
  1. Open a new empty chat (do not continue /hero-start in this configuration session).
  2. In that chat, select the agent you want as the Hero orchestrator / grill-me.
  3. Run /hero-start.`

	got := formatChatAgentText("new", raw)
	if strings.Contains(got, "|") {
		t.Fatalf("table pipes should be flattened: %q", got)
	}
	if strings.Contains(got, "**") || strings.Contains(got, "](") {
		t.Fatalf("markdown should be stripped: %q", got)
	}
	if strings.Contains(got, "clean session handoff") || strings.Contains(got, "new empty chat") {
		t.Fatalf("cursor handoff should be removed: %q", got)
	}
	if strings.Contains(got, "Cycle C4 initialized") {
		t.Fatalf("cycle initialized line should be removed for TUI /hero-new: %q", got)
	}
	if !strings.Contains(got, tuiHeroNewClosingLine) {
		t.Fatalf("expected TUI closing line: %q", got)
	}
}

func TestFormatChatAgentText_HeroNewStripsConfirmationPrompt(t *testing.T) {
	raw := `→ Preparing workflow-config.yml...
→ Waiting for your confirmation after editing the config.
Reply to confirm when ready, and the cycle will be created via hero cycle new.`

	got := formatChatAgentText("new", raw)
	if strings.Contains(strings.ToLower(got), "confirmation") {
		t.Fatalf("confirmation prompt should be removed: %q", got)
	}
	if strings.Contains(got, "hero cycle new") {
		t.Fatalf("hero cycle new mention should be removed: %q", got)
	}
	if !strings.Contains(got, tuiHeroNewClosingLine) {
		t.Fatalf("expected closing line: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_HeroNewOverrides(t *testing.T) {
	got := tuiRuntimeCommandPrompt("new", "# /hero-new\n\nbody")
	if !strings.Contains(got, "TUI will call hero cycle new automatically") {
		t.Fatalf("missing hero-new override: %q", got)
	}
	if !strings.Contains(got, tuiHeroNewClosingLine) {
		t.Fatalf("missing closing line in preamble: %q", got)
	}
	if !strings.Contains(got, "body") {
		t.Fatalf("missing command body: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_HeroStartOverrides(t *testing.T) {
	got := tuiRuntimeCommandPrompt("start", "# /hero-start\n\nbody")
	if !strings.Contains(got, "orchestration agent") {
		t.Fatalf("missing start orchestrator context: %q", got)
	}
	if !strings.Contains(got, "hero cycle sync-config") {
		t.Fatalf("missing sync-config guard: %q", got)
	}
	if strings.Contains(got, "Clean Session Handoff") {
		t.Fatalf("start preamble should not mention handoff: %q", got)
	}
	if strings.Contains(got, "Do NOT run `hero cycle new` or any shell") {
		t.Fatalf("hero-new override leaked into start: %q", got)
	}
	if !strings.Contains(got, "body") {
		t.Fatalf("missing command body: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_HeroApproveOverrides(t *testing.T) {
	got := tuiRuntimeCommandPrompt("approve", "# /hero-approve\n\nbody")
	if !strings.Contains(got, "/hero-approve") {
		t.Fatalf("missing approve context: %q", got)
	}
	if !strings.Contains(got, "hero approve --metrics-json") {
		t.Fatalf("missing metrics-json requirement: %q", got)
	}
	if !strings.Contains(got, "hero status") {
		t.Fatalf("missing hero status guard: %q", got)
	}
	if strings.Contains(got, "hero cycle sync-config") {
		t.Fatalf("start override leaked into approve: %q", got)
	}
	if !strings.Contains(got, "body") {
		t.Fatalf("missing command body: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_GenericPreamble(t *testing.T) {
	got := tuiRuntimeCommandPrompt("sync", "# /hero-sync\n\nbody")
	if !strings.Contains(got, "Hero TUI") {
		t.Fatalf("missing TUI preamble: %q", got)
	}
	if strings.Contains(got, "Do NOT run `hero cycle new`") {
		t.Fatalf("hero-new override should not apply to sync: %q", got)
	}
}

func TestWrapOutputLine_BreaksOnSpaces(t *testing.T) {
	got := wrapOutputLine("one two three four", 10)
	if len(got) != 2 {
		t.Fatalf("lines=%v", got)
	}
	if got[0] != "one two" || got[1] != "three four" {
		t.Fatalf("unexpected wrap: %v", got)
	}
}

func TestSplitOutputLines_PreservesParagraphs(t *testing.T) {
	got := splitOutputLines("line one\nline two", 40)
	if len(got) != 2 || got[0] != "line one" || got[1] != "line two" {
		t.Fatalf("lines=%v", got)
	}
}
