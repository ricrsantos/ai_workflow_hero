package tui

import (
	"fmt"
	"regexp"
	"strings"
)

const tuiHeroNewClosingLine = "→ After completing the configuration in .workflow-hero/cycles/current/workflow-config.yml, run /hero-start to start the new cycle."

const tuiRuntimePreamble = "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
	"You are running inside the Hero TUI. Follow the command instructions below with these overrides:\n\n" +
	"- Output plain text only: no markdown tables, no markdown links, no bold markdown syntax. Use the arrow status lines from \"Output Format\" (→, ✓).\n" +
	"- Do not include \"Clean Session Handoff\" or tell the user to open a new Cursor chat, select an IDE orchestrator model, or run /hero-start in another chat session.\n" +
	"- When configuration is ready, tell the user to run /hero-start from the Hero TUI.\n\n" +
	"---\n\n"

func tuiHeroNewPreamble() string {
	return "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-new inside the Hero TUI. Follow the command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓).\n" +
		"- Prepare and write workflow-config.yml, then the TUI will call hero cycle new automatically (active cycle, empty title/objective in SQLite).\n" +
		"- Do NOT ask the user to reply or confirm before the cycle is prepared. Do NOT run shell/CLI commands yourself.\n" +
		"- Do not include \"Clean Session Handoff\" or Cursor-chat instructions (new empty chat, select orchestrator model).\n" +
		"- Tell the user to edit title/objective/scope in the config file, then run /hero-start from the Hero TUI.\n" +
		"- End your response with exactly this line (copy verbatim):\n" +
		tuiHeroNewClosingLine + "\n\n" +
		"---\n\n"
}

var (
	mdLinkRE   = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	mdBoldRE   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdCodeRE   = regexp.MustCompile("`([^`]+)`")
	mdHeaderRE = regexp.MustCompile(`^#{1,6}\s+`)
)

// tuiRuntimeCommandPrompt prepends TUI-specific instructions to a Hero runtime command body.
func tuiHeroStartPreamble() string {
	return "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-start inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓).\n" +
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n" +
		"- Do NOT depend on prior chat history from /hero-new — bootstrap from disk and CLI state.\n" +
		"- Do NOT run `hero cycle new` — the cycle was prepared during /hero-new.\n" +
		"- The TUI runs hero cycle sync-config before this session; do not ask the user to run it manually.\n" +
		"- Run full orchestration: validate workflow-config, dispatch Task subagents, persist via hero CLI with metrics.\n" +
		"- Stay inside this project root (the directory that contains .workflow-hero/). Do not read, grep, glob, or search parent directories, sibling folders, or any Hero framework/source tree.\n" +
		"- Invoke `hero` from PATH via the Shell tool (e.g. `hero status`). Do not hunt the filesystem for the binary. If Shell fails, stop and tell the user — do not reverse-engineer Hero internals.\n" +
		"- Call `hero stage start --name <stage>` before dispatching that stage's agent.\n" +
		"- After Configuration, if Research is enabled: call `hero stage start --name research` then STOP. Do NOT grill in this orchestrator session. Do NOT dispatch Task discover_agent. The TUI continues Research as discover_agent.\n" +
		"- If research is disabled, skip Research and continue orchestration as usual.\n" +
		"- After every Task dispatch: set run_in_background to false, wait until the Task returns, then post that agent's Output Format summary in chat. Nested Task work does not stream here. Do not end your turn after launching Task. Do not start the next stage until the current Task has returned.\n" +
		"- Read require_human_approval for the stage that just finished (not the next one). If false: auto-close via CLI and dispatch the next stage in the same turn — never ask yes/no to proceed. If true: close as PendingApproval, list /hero-approve /hero-reject /hero-cancel /hero-finish, and STOP.\n" +
		"- Tell the user to use /hero-approve, /hero-reject, /hero-cancel, or /hero-finish in the Hero TUI (not Cursor chat handoff).\n\n" +
		"---\n\n"
}

func tuiDiscoverResearchPreamble() string {
	return "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running the Research stage inside the Hero TUI as discover_agent. Follow discover_agent.md with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓).\n" +
		"- Grill interactively with the user in this session.\n" +
		"- When Research deliverables are done, persist via `hero stage close --name research --metrics-json '<JSON>'` (Metrics Procedure) and STOP.\n" +
		"- Do NOT dispatch planning_agent or any later stage. Do NOT ask the user to start Planning.\n" +
		"- Tell the user to use /hero-approve, /hero-reject, /hero-cancel, or /hero-finish in the Hero TUI when human approval is required.\n\n" +
		"---\n\n"
}

func tuiHeroStartContinueAfterResearchPreamble() string {
	return "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are the orchestration agent resuming after Research closed in the Hero TUI. Follow these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓).\n" +
		"- Research grilling is done. Do NOT re-run Research or grill again.\n" +
		"- If Research is PendingApproval, list /hero-approve /hero-reject /hero-cancel /hero-finish and STOP.\n" +
		"- Otherwise dispatch the next enabled stage via Task with Model Resolution from workflow-config.yml.\n" +
		"- After every Task dispatch: set run_in_background to false, wait until the Task returns, then post that agent's Output Format summary.\n" +
		"- Tell the user to use /hero-approve, /hero-reject, /hero-cancel, or /hero-finish in the Hero TUI (not Cursor chat handoff).\n\n" +
		"---\n\n"
}

func tuiHeroApprovePreamble() string {
	return "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-approve inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓).\n" +
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n" +
		"- Run `hero status` (or `hero status --json`) to confirm the current stage is pending approval.\n" +
		"- Run `hero metrics` or `hero metrics --json` to gather existing stage context when estimating metrics.\n" +
		"- Apply the Metrics Procedure; never leave token/cost/duration unset for the approved stage.\n" +
		"- Persist approval via `hero approve --metrics-json '<JSON>'` (optional `--summary`). Do NOT write metrics.md or workflow.md.\n" +
		"- Show the metrics summary block from the command Output Format and point to `hero metrics` for full details.\n" +
		"- Tell the user to continue control commands in the Hero TUI (/hero-reject, /hero-finish, etc.) — not Cursor chat handoff.\n\n" +
		"---\n\n"
}

func tuiHeroCancelPreamble(reason string) string {
	preamble := "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-cancel inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓, ⚠).\n" +
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n" +
		"- Run `hero status` (or `hero status --json`) before and after cancel.\n" +
		"- Persist cancellation via `hero cancel` (optional `--reason`). Do NOT write workflow.md.\n" +
		"- Roll back uncommitted working-tree changes via `git checkout` / `git restore` when appropriate (CLI does not run git).\n" +
		"- Tell the user to run /hero-new or /hero-resume in the Hero TUI — not Cursor chat handoff.\n\n"
	if r := strings.TrimSpace(reason); r != "" {
		preamble += "## User cancellation reason\n\n" + r + "\n\n"
	}
	return preamble + "---\n\n"
}

func tuiHeroFinishPreamble() string {
	return "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-finish inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓).\n" +
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n" +
		"- Run `hero status` and validate required stages before finishing.\n" +
		"- Apply the Metrics Procedure; persist via `hero finish --metrics-json '<JSON>'`. Do NOT write workflow.md or metrics.md.\n" +
		"- Update `context-log.md` and `current-state.md` after cycle completion.\n" +
		"- Remind the user to run /hero-archive in the Hero TUI when ready.\n\n" +
		"---\n\n"
}

func tuiHeroContinuePreamble(extra int) string {
	if extra <= 0 {
		extra = 1
	}
	return fmt.Sprintf("## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n"+
		"You are running /hero-continue inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n"+
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓).\n"+
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n"+
		"- The user requested **+%d** extra iteration(s). Run `hero status` and confirm the current stage is Escalated.\n"+
		"- Grant iterations via `hero continue --extra %d`. Do NOT edit workflow-config.yml max_iterations.\n"+
		"- After granting, resume execution of the escalated stage via Task subagents.\n"+
		"- Apply Metrics Procedure on subsequent stage closes.\n\n"+
		"---\n\n", extra, extra)
}

func tuiHeroBackPreamble() string {
	return "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-back inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓, ⚠).\n" +
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n" +
		"- Run `hero status` to confirm Judge stage context.\n" +
		"- There is no `hero back` CLI verb — reopen Planning via Task `planning_agent` with Model Resolution from workflow-config.yml.\n" +
		"- After Planning completes, re-run Implementation → QA → Judge with fresh Task sessions.\n" +
		"- Persist each stage close via hero CLI with `--metrics-json` per Metrics Procedure.\n" +
		"- Record the back-step decision in `context-log.md`.\n\n" +
		"---\n\n"
}

func tuiHeroSyncPreamble() string {
	return "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-sync inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓).\n" +
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n" +
		"- Invoke Task `context_agent` (read-only) with Model Resolution from workflow-config.yml when available.\n" +
		"- Generate AGENTS.md, context/current-state.md, context/context-log.md; scan docs/product and docs/architecture for pending items (ADR-029).\n" +
		"- Update .workflow-hero/config/project.json; run `hero doctor` for harness warnings.\n" +
		"- Tell the user to run /hero-todos and /hero-new in the Hero TUI — not Cursor chat handoff.\n\n" +
		"---\n\n"
}

func tuiHeroStatusPreamble() string {
	return "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-status inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓).\n" +
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n" +
		"- Run `hero status` (or `hero status --json`) and relay the full CLI table: Stage, Status, Iteration, Human Approval.\n" +
		"- If no active cycle, tell the user to run /hero-new in the Hero TUI.\n" +
		"- Do NOT read workflow.md for operational status — SQLite via hero status is the source of truth.\n\n" +
		"---\n\n"
}

func tuiHeroArchivePreamble() string {
	return "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-archive inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓, ✗).\n" +
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n" +
		"- Run `hero status` (or `--json`) before archive; persist via `hero cycle archive` — do not hand-roll folder moves.\n" +
		"- Do NOT dispatch Task or stage agents (including end2end_qa_agent). Archive is orchestrator-only.\n" +
		"- On OpenSpec failure, offer retry /hero-archive or `hero cycle archive --force` only after explicit user consent.\n" +
		"- Optionally update metrics-summary.md from `hero metrics` for the archived cycle.\n" +
		"- Tell the user to run /hero-resume in the Hero TUI when ready — not Cursor chat handoff.\n\n" +
		"---\n\n"
}

func tuiHeroResumePreamble(cycleN int) string {
	preamble := "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-resume inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓).\n" +
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n" +
		"- If another cycle is active, warn and suggest archive or finish first when appropriate.\n" +
		"- Resume via `hero cycle resume` or `hero cycle resume --number N`. Do NOT edit workflow.md.\n" +
		"- Run `hero status` after resume to show paused/current stage.\n" +
		"- Tell the user to run /hero-start or /hero-approve / /hero-reject in the Hero TUI — not Cursor chat handoff.\n\n"
	if cycleN > 0 {
		preamble += fmt.Sprintf("## Target cycle\n\nResume cycle C%d (`hero cycle resume --number %d`).\n\n", cycleN, cycleN)
	}
	return preamble + "---\n\n"
}

func tuiHeroRejectPreamble(reason string) string {
	preamble := "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-reject inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓, ⚠).\n" +
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n" +
		"- The user already provided rejection feedback in the TUI (see below). Do NOT ask for feedback again.\n" +
		"- Run `hero status` (or `hero status --json`) to confirm the current stage is pending approval.\n" +
		"- Persist rejection via `hero reject --reason '<feedback>'` using the user feedback below. Do NOT write workflow.md.\n" +
		"- Re-run the current stage via Task, passing the rejection reason to the responsible agent.\n" +
		"- Respect max_iterations; if exhausted, escalate and tell the user to run /hero-continue in the Hero TUI.\n" +
		"- Use the command Output Format (⚠ <Stage> rejected. Re-running with feedback...).\n" +
		"- Tell the user to continue control commands in the Hero TUI (/hero-approve, /hero-finish, etc.) — not Cursor chat handoff.\n\n" +
		"## User rejection feedback\n\n" +
		strings.TrimSpace(reason) + "\n\n" +
		"---\n\n"
	return preamble
}

func tuiRuntimeCommandPrompt(cmdName, commandBody string, opts heroRuntimeOpts) string {
	preamble := tuiRuntimePreamble
	switch cmdName {
	case "new":
		preamble = tuiHeroNewPreamble()
	case "start":
		preamble = tuiHeroStartPreamble()
	case "approve":
		preamble = tuiHeroApprovePreamble()
	case "reject":
		preamble = tuiHeroRejectPreamble(opts.RejectReason)
	case "cancel":
		preamble = tuiHeroCancelPreamble(opts.CancelReason)
	case "finish":
		preamble = tuiHeroFinishPreamble()
	case "continue":
		preamble = tuiHeroContinuePreamble(opts.ContinueExtra)
	case "back":
		preamble = tuiHeroBackPreamble()
	case "sync":
		preamble = tuiHeroSyncPreamble()
	case "status":
		preamble = tuiHeroStatusPreamble()
	case "archive":
		preamble = tuiHeroArchivePreamble()
	case "resume":
		preamble = tuiHeroResumePreamble(opts.ResumeCycleNumber)
	}
	return preamble + strings.TrimSpace(commandBody) + "\n"
}

// formatChatAgentText converts model output into terminal-friendly plain text for the Chat pane.
func formatChatAgentText(runtimeCmd, raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	s = stripCursorChatHandoff(s)
	if runtimeCmd == "new" {
		s = normalizeTUINewOutput(s)
	}
	s = mdLinkRE.ReplaceAllString(s, "$1")
	s = mdBoldRE.ReplaceAllString(s, "$1")
	s = mdCodeRE.ReplaceAllString(s, "$1")
	s = flattenMarkdownTables(s)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = mdHeaderRE.ReplaceAllString(line, "")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func normalizeTUINewOutput(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	sawConfig := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		lower := strings.ToLower(t)
		if isTUINewCyclePromptLine(lower) {
			continue
		}
		if strings.Contains(lower, "workflow-config.yml") ||
			strings.Contains(lower, "preparing workflow-config") {
			sawConfig = true
		}
		out = append(out, line)
	}
	s = strings.TrimRight(strings.Join(out, "\n"), "\n\t ")
	if sawConfig && !strings.Contains(s, tuiHeroNewClosingLine) {
		if s != "" {
			s += "\n"
		}
		s += tuiHeroNewClosingLine
	}
	return s
}

func isTUINewCyclePromptLine(lower string) bool {
	switch {
	case strings.Contains(lower, "waiting for your confirmation"):
		return true
	case strings.Contains(lower, "reply to confirm"):
		return true
	case strings.Contains(lower, "hero cycle new"):
		return true
	case strings.Contains(lower, "cycle will be created"):
		return true
	case strings.Contains(lower, "creating cycle via"):
		return true
	case strings.Contains(lower, "cycle initialized"):
		return true
	case strings.HasPrefix(lower, "✓ cycle c") && strings.Contains(lower, "initialized"):
		return true
	}
	return false
}

func stripCursorChatHandoff(s string) string {
	markers := []string{
		"→ Next (clean session handoff)",
		"→ Next:",
		"## Clean Session Handoff",
		"Clean Session Handoff",
	}
	lower := strings.ToLower(s)
	for _, marker := range markers {
		if idx := strings.Index(lower, strings.ToLower(marker)); idx >= 0 {
			s = strings.TrimRight(s[:idx], " \n\t")
			lower = strings.ToLower(s)
		}
	}
	return dropHandoffTailLines(s)
}

func dropHandoffTailLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		t := strings.TrimSpace(line)
		lower := strings.ToLower(t)
		if isCursorHandoffLine(lower) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func isCursorHandoffLine(lower string) bool {
	if lower == "" {
		return false
	}
	switch {
	case strings.Contains(lower, "new empty chat"):
		return true
	case strings.Contains(lower, "clean session handoff"):
		return true
	case strings.Contains(lower, "do not continue") && strings.Contains(lower, "hero-start"):
		return true
	case strings.Contains(lower, "configuration session"):
		return true
	case strings.Contains(lower, "orchestrator") && strings.Contains(lower, "grill-me"):
		return true
	case strings.Contains(lower, "select the agent") && strings.Contains(lower, "hero"):
		return true
	case strings.HasPrefix(lower, "1. open a"):
		return true
	case strings.HasPrefix(lower, "2. in that chat"):
		return true
	case strings.HasPrefix(lower, "3. run `/hero-start`") || strings.HasPrefix(lower, "3. run /hero-start"):
		return true
	}
	return false
}

func flattenMarkdownTables(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if !strings.Contains(trim, "|") {
			out = append(out, line)
			continue
		}
		if isMarkdownTableSeparator(trim) {
			continue
		}
		cells := splitMarkdownTableCells(trim)
		if len(cells) >= 2 {
			out = append(out, strings.Join(cells, ": "))
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func isMarkdownTableSeparator(line string) bool {
	stripped := strings.ReplaceAll(line, "|", "")
	stripped = strings.ReplaceAll(stripped, "-", "")
	stripped = strings.ReplaceAll(stripped, ":", "")
	return strings.TrimSpace(stripped) == ""
}

func splitMarkdownTableCells(line string) []string {
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cells = append(cells, p)
	}
	return cells
}
