# UI Spec — Hero CLI Terminal UX

> "UI" for Hero means the terminal user experience of the `hero` CLI, plus the shared visual conventions also used by agents in the Runtime (chat). Baseline (0.9.x): grilling 2026-07-20. **Hero 1.0 TUI + CLI query UX:** [UI-C01-001-hero-tui.md](UI-C01-001-hero-tui.md). **C2 slash/TUI command parity:** [UI-C02-001-tui-slash-command-parity.md](UI-C02-001-tui-slash-command-parity.md).

## Cycle UI specs

| Document | Cycle | Summary |
|---|---|---|
| [UI-C01-001-hero-tui.md](UI-C01-001-hero-tui.md) | C1 | TUI screens, dual entry parity, CLI status/metrics/events replacing cycle markdown |
| [UI-C02-001-tui-slash-command-parity.md](UI-C02-001-tui-slash-command-parity.md) | C2 | `/hero:*` TUI labels, imported Cursor commands, archive force UX, harness doctor warnings |

## 1. Scope

This document defines:

- Visual style (colors, icons, formatting) for CLI output.
- Structured vs. human-readable output for read commands.
- Interactive prompt behavior and non-interactive (scriptable) overrides.
- Error message format.
- Shared conventions reused by Runtime agents inside the chat.

## 2. Visual Style

### 2.1 Semantic Icons and Colors

| Meaning | Icon | Color |
|---|---|---|
| Success | ✓ | Green |
| Warning | ⚠ | Yellow |
| Error | ✗ | Red |
| Progress / info | → | Blue |

- Color and icon support is auto-detected: if stdout is not a TTY, or the `NO_COLOR` environment variable is set, the CLI degrades to plain text (icons and color codes omitted, but the semantic wording stays the same, e.g. `[OK]`, `[WARN]`, `[ERROR]`).
- This exact same icon/semantic convention is reused by agents in the **Runtime** (chat), for example in stage-closing summaries and model-fallback warnings, to keep the Hero experience visually consistent across CLI and chat.

### 2.2 Example

```text
✓ Hero installed successfully.
⚠ .cursor/agents/backend_agent.md was customized locally and was not overwritten.
✗ workflow-config.yml is invalid: stage "qa" is missing "max_iterations".
→ Scanning existing codebase for AGENTS.md generation...
```

## 3. Output Formats

### 3.1 Read Commands (`hero status`, `hero variables`, `hero doctor`)

- **Default**: human-readable table, rendered with an ASCII table library.
- **`--json` flag**: available on every read command, for scripting/CI consumption. Emits a single JSON object/array to stdout; all human-readable decoration (colors, icons) is suppressed when `--json` is passed.

Example:

```bash
hero status
# ┌───────────────┬───────────┬───────────┬────────────────┐
# │ Stage         │ Status    │ Iteration │ Human Approval │
# ├───────────────┼───────────┼───────────┼────────────────┤
# │ Configuration │ Completed │ 1         │ N/A            │
# │ Research      │ Completed │ 1/1       │ Approved       │
# │ Planning      │ In Progress │ 1/3     │ Pending        │
# └───────────────┴───────────┴───────────┴────────────────┘

hero status --json
# {"stages":[{"name":"configuration","status":"Completed","iteration":"1","humanApproval":"N/A"}, ...]}
```

## 4. Interactive Prompts

- Use a Go survey-style prompt library (e.g. `AlecAivazis/survey` or `charmbracelet/huh`) for multi-step interactive commands (`hero install`, confirmations like "run `git init`?").
- Prompts support arrow-key navigation for multiple-choice questions and inline validation for required fields (e.g. project name cannot be empty).
- **Non-interactive override**: every interactive prompt has an equivalent flag (e.g. `--name`, `--summary`, `--yes`). If all required flags for a command are supplied, the command runs with zero prompts (usable in CI/scripts). If some are missing, only the missing fields fall back to interactive prompts.
- **`hero install` ceremony**: print a setup header, then minimal prompts (`Title` + `> ` input, no bordered huh chrome). Project summary is optional. Success uses the standard ✓ icon (UI §2.1). The 🚀 emoji is only used in this install header when the terminal supports color (TTY / no `NO_COLOR`).

Example (interactive):

```text
hero install --tools cursor

🚀 Hero Project Setup

Project name:
> Indoor Location

Project summary (Opcional):
> Indoor positioning platform using BLE gateways.

✓ Hero installed successfully.
→ Full user guide: .workflow-hero/docs/workflow-help.md
```

```bash
# Fully scripted (CI-friendly), no prompts at all
hero install --tools cursor --name "Indoor Location" --summary "BLE indoor positioning platform" --yes
```

## 5. Error Messages

Every CLI error follows the same structure:

```text
✗ <clear description of what went wrong>

  Suggestion: <how to fix it, when applicable>

(exit code: 1)
```

- Exit code is always non-zero on failure.
- Stack traces for unexpected panics are hidden by default; they only print when `--verbose` (or `--debug`) is passed.

Example:

```text
✗ This directory is not a git repository.

  Suggestion: run `hero install --tools cursor --git-init` to let Hero initialize
  git automatically, or run `git init` manually and retry.

(exit code: 1)
```

## 6. Reference Table

| Concern | Decision |
|---|---|
| Color/icon degradation | Auto-detected via TTY / `NO_COLOR` |
| Read command output | Table by default, `--json` flag for scripting |
| Prompt library | Survey-style (arrow navigation, inline validation) |
| Non-interactive mode | Flag overrides per prompt; missing flags fall back to prompts |
| Error structure | Icon + description + suggestion + non-zero exit code |
| Verbose/debug | `--verbose`/`--debug` flag reveals stack traces |
| Runtime consistency | Same icon/semantic convention reused in chat by all agents |

## 7. Testing Requirement

Every CLI command's output formatting (table rendering, `--json` payloads, error structure) must be covered by golden-file or unit tests before being considered done, per the testing strategy in [AGENTS.md — Testing](../../AGENTS.md#testing) and [ADR-009](../architecture/ADR.md#adr-009-test-real-dependencies-over-mocks). Run `go test ./...` and fix any failure before finishing — never leave the repository in a failing state.
