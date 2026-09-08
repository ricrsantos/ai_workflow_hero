# UI-C13-001 — TUI Claude Code Adapter

> Cycle C13 UX. Extends [UI-C04-001](UI-C04-001-tui-multi-harness.md), [UI-C05-001](UI-C05-001-tui-model-properties.md), and [UI-C06-001](UI-C06-001-tui-codex-adapter.md).

## 1. Selection and identity

Install and `/hero-harness` add PATH-independent Claude as a fourth checkbox. Enable reports `✓ Claude enabled (projected .claude/)`; disable reports `✓ Claude disabled (files kept)`. The last-harness guard remains.

`/hero-model` selects Claude then native aliases/IDs; it persists the free-chat pair only. C5 exposes supported `ef` values and renders `th`/`fs` unavailable. Turns name the harness:

```text
[DISC - sonnet · claude]
[TASK - opus[1m] · claude]
```

## 2. Safety and diagnostics

For `ask`, the existing `y`/`n` TUI permission gate (and Telegram forwarding) is used; pending permission pauses watchdog accounting. Enabling Claude/context management warns that headless execution can load project `CLAUDE.md`, hooks, plugins, and MCP. Existing `CLAUDE.md` offers managed-block diff/update or preservation; user text is never replaced.

Missing CLI, authentication, and incompatible version errors are actionable; version copy names the required 2.1.261 minimum and directs the user to `/hero-continue` after remediation. Doctor warns when enabled Claude is unavailable. Claude has no persistent server and is not a `/harness-reset` target.

## 3. Verification

Test the fourth selector row, projection/preservation, pair picker, `· claude` label, all-verbosity permission visibility, errors, `CLAUDE.md` choices, and constrained terminal rendering.
