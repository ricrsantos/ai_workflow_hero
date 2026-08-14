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
	got := tuiRuntimeCommandPrompt("new", "# /hero-new\n\nbody", heroRuntimeOpts{})
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
	got := tuiRuntimeCommandPrompt("start", "# /hero-start\n\nbody", heroRuntimeOpts{})
	if !strings.Contains(got, "orchestration agent") {
		t.Fatalf("missing start orchestrator context: %q", got)
	}
	if !strings.Contains(got, "hero cycle sync-config") {
		t.Fatalf("missing sync-config guard: %q", got)
	}
	if !strings.Contains(got, "Stay inside this project root") {
		t.Fatalf("missing workspace isolation: %q", got)
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
	for _, kw := range []string{
		"run_in_background",
		"hero stage start",
		"require_human_approval",
		"/hero-approve",
	} {
		if !strings.Contains(got, kw) {
			t.Fatalf("start preamble missing %q: %q", kw, got)
		}
	}
}

func TestTUIRuntimeCommandPrompt_HeroApproveOverrides(t *testing.T) {
	got := tuiRuntimeCommandPrompt("approve", "# /hero-approve\n\nbody", heroRuntimeOpts{})
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

func TestTUIRuntimeCommandPrompt_HeroRejectOverrides(t *testing.T) {
	reason := "fix the failing tests"
	got := tuiRuntimeCommandPrompt("reject", "# /hero-reject\n\nbody", heroRuntimeOpts{RejectReason: reason})
	if !strings.Contains(got, "/hero-reject") {
		t.Fatalf("missing reject context: %q", got)
	}
	if !strings.Contains(got, "hero reject --reason") {
		t.Fatalf("missing reject CLI requirement: %q", got)
	}
	if !strings.Contains(got, reason) {
		t.Fatalf("missing user rejection feedback: %q", got)
	}
	if !strings.Contains(got, "body") {
		t.Fatalf("missing command body: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_HeroSyncOverrides(t *testing.T) {
	got := tuiRuntimeCommandPrompt("sync", "# /hero-sync\n\nbody", heroRuntimeOpts{})
	if !strings.Contains(got, "/hero-sync") {
		t.Fatalf("missing sync context: %q", got)
	}
	if !strings.Contains(got, "context_agent") {
		t.Fatalf("missing context_agent: %q", got)
	}
	if !strings.Contains(got, "hero doctor") {
		t.Fatalf("missing hero doctor: %q", got)
	}
	if strings.Contains(got, "Do NOT run `hero cycle new`") {
		t.Fatalf("hero-new override should not apply to sync: %q", got)
	}
	if !strings.Contains(got, "body") {
		t.Fatalf("missing command body: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_HeroStatusOverrides(t *testing.T) {
	got := tuiRuntimeCommandPrompt("status", "# /hero-status\n\nbody", heroRuntimeOpts{})
	if !strings.Contains(got, "hero status") {
		t.Fatalf("missing hero status: %q", got)
	}
	if !strings.Contains(got, "Human Approval") {
		t.Fatalf("missing table columns: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_HeroArchiveOverrides(t *testing.T) {
	got := tuiRuntimeCommandPrompt("archive", "# /hero-archive\n\nbody", heroRuntimeOpts{})
	if !strings.Contains(got, "hero cycle archive") {
		t.Fatalf("missing archive CLI: %q", got)
	}
	if !strings.Contains(got, "--force") {
		t.Fatalf("missing force path: %q", got)
	}
	if !strings.Contains(got, "end2end_qa_agent") {
		t.Fatalf("must forbid stage-agent dispatch: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_HeroResumeOverrides(t *testing.T) {
	got := tuiRuntimeCommandPrompt("resume", "# /hero-resume\n\nbody", heroRuntimeOpts{ResumeCycleNumber: 4})
	if !strings.Contains(got, "hero cycle resume") {
		t.Fatalf("missing resume CLI: %q", got)
	}
	if !strings.Contains(got, "Resume cycle C4") {
		t.Fatalf("missing target cycle: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_HeroCancelOverrides(t *testing.T) {
	reason := "scope changed"
	got := tuiRuntimeCommandPrompt("cancel", "# /hero-cancel\n\nbody", heroRuntimeOpts{CancelReason: reason})
	if !strings.Contains(got, "hero cancel") {
		t.Fatalf("missing cancel CLI: %q", got)
	}
	if !strings.Contains(got, "git checkout") {
		t.Fatalf("missing git rollback: %q", got)
	}
	if !strings.Contains(got, reason) {
		t.Fatalf("missing cancel reason: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_HeroFinishOverrides(t *testing.T) {
	got := tuiRuntimeCommandPrompt("finish", "# /hero-finish\n\nbody", heroRuntimeOpts{})
	if !strings.Contains(got, "hero finish --metrics-json") {
		t.Fatalf("missing finish CLI: %q", got)
	}
	if !strings.Contains(got, "context-log.md") {
		t.Fatalf("missing context update: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_HeroContinueOverrides(t *testing.T) {
	got := tuiRuntimeCommandPrompt("continue", "# /hero-continue\n\nbody", heroRuntimeOpts{ContinueExtra: 3})
	if !strings.Contains(got, "hero continue --extra 3") {
		t.Fatalf("missing continue extra: %q", got)
	}
	if !strings.Contains(got, "+3") {
		t.Fatalf("missing +3 in preamble: %q", got)
	}
}

func TestTUIRuntimeCommandPrompt_HeroBackOverrides(t *testing.T) {
	got := tuiRuntimeCommandPrompt("back", "# /hero-back\n\nbody", heroRuntimeOpts{})
	if !strings.Contains(got, "planning_agent") {
		t.Fatalf("missing planning_agent: %q", got)
	}
	if !strings.Contains(got, "no `hero back` CLI") {
		t.Fatalf("missing no CLI verb note: %q", got)
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
