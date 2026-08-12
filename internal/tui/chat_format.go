package tui

import (
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
		"- Prepare and write workflow-config.yml only. Do NOT run `hero cycle new` or any shell/CLI command to create or start the cycle.\n" +
		"- Do NOT ask the user to reply or confirm to create the cycle. Do NOT claim the cycle is initialized or created.\n" +
		"- Do not include \"Clean Session Handoff\" or Cursor-chat instructions (new empty chat, select orchestrator model).\n" +
		"- In the TUI, cycles start only via /hero-start after the user edits the config file.\n" +
		"- End your response with exactly this line (copy verbatim):\n" +
		tuiHeroNewClosingLine + "\n\n" +
		"---\n\n"
}

var (
	mdLinkRE  = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	mdBoldRE  = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdCodeRE  = regexp.MustCompile("`([^`]+)`")
	mdHeaderRE = regexp.MustCompile(`^#{1,6}\s+`)
)

// tuiRuntimeCommandPrompt prepends TUI-specific instructions to a Hero runtime command body.
func tuiHeroStartPreamble() string {
	return "## TUI execution context (Hero terminal UI — not Cursor IDE chat)\n\n" +
		"You are running /hero-start inside the Hero TUI as the orchestration agent. Follow the agent instructions and command instructions below with these overrides:\n\n" +
		"- Output plain text only: no markdown tables, links, or bold syntax. Use arrow status lines (→, ✓).\n" +
		"- Do NOT ask the user to open a new Cursor chat or select an IDE orchestrator model.\n" +
		"- Do NOT depend on prior chat history from /hero-new — bootstrap from disk and CLI state.\n" +
		"- Do NOT run `hero cycle new` — the cycle already exists in SQLite.\n" +
		"- Run full orchestration: validate workflow-config, dispatch Task subagents, persist via hero CLI with metrics.\n" +
		"- Tell the user to use /hero-approve, /hero-reject, /hero-cancel, or /hero-finish in the Hero TUI (not Cursor chat handoff).\n\n" +
		"---\n\n"
}

func tuiRuntimeCommandPrompt(cmdName, commandBody string) string {
	preamble := tuiRuntimePreamble
	switch cmdName {
	case "new":
		preamble = tuiHeroNewPreamble()
	case "start":
		preamble = tuiHeroStartPreamble()
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
