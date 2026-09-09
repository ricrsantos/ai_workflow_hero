# C13 Judge loop-back — implementation gaps (iteration 5)

Source: Judge (`judge_agent`, `cursor-grok-4.6-high`) on `openspec/changes/claude-code-adapter`.
`all_requirements_met: false`. No SDD ambiguity. Re-run `generic_agent` (native scope).

SDD: `openspec/changes/claude-code-adapter/` (proposal, design, 12 specs, 42 tasks).
Authoritative product docs: `docs/product/PRD-C13-001-claude-code-adapter.md`,
`docs/architecture/ADR-C13-001-claude-code-adapter.md`,
`docs/product/UI-C13-001-tui-claude-code-adapter.md`.

Do not reimplement landed work. Keep and extend it.

## What already landed (keep)

All prior remainder items from Judge iteration 4 are now present:

- Live ask transport: production Execute starts a localhost `socketPermissionBridge`, writes a 0600 one-server `--mcp-config` (`hero_permissions` stdio helper `hero internal claude-permission-bridge`), and passes `--strict-mcp-config`. Token stays in helper env only. Ask fails closed without TokenSource, OnPermissionRequest, or proven `--permission-prompt-tool`/`--mcp-config`/`--strict-mcp-config` flags.
- auto-project → `--permission-mode acceptEdits`; auto-all → `--dangerously-skip-permissions`; never silent approve.
- TUI/Telegram `OnPermissionRequest` / `OnQuestionRequest` with watchdog Pause/Resume, persist `HarnessPermissionPaused` for Claude, Telegram allow/deny/interrupt.
- `testdata/stream.ndjson` includes permission/question; normalizer and `resultAssembler` emit those kinds; unknown events remain redacted warnings.
- Adapter rejects identifiable Cursor/OpenCode/Codex resume ids (`cursor:`, `opencode:`, `thr_`/`thread_`) before `--resume`.
- SIGINT-first Cancel with kill fallback; bridge Close on Execute defer and interrupted ask turns.
- TUI completed-turn identity replaces the configured alias with `ExecutionResult.NativeModel`.
- Doctor unconfigured `.claude/` warns that Claude is not enabled / user-managed; it does not say "unsupported in this Hero version". Status reads `permissionPaused` from the active Claude stage row.
- Four-harness registry plus execution/cancel/fallback acceptance (`TestIntegration_FourHarnessExecutionCancelAndFallbackAcceptance`): no adapter receives another harness session; unavailable Claude falls back once.
- Protocol fixtures, live MCP helper round-trip, projection/`CLAUDE.md`, catalog/C5, Doctor/Status, Telegram, watchdog, and cancel coverage remain in package tests. No live Claude account.

## Required work

1. task-06.3 / claude-projection remainder — `/hero-start` Prepare for Claude. When any workflow agent uses `harness: claude`, asynchronously sync only Hero-managed model, effort, and applicable skill fields in `.claude/agents/<agent>.md`. Preserve user body text and unmarked frontmatter. Do not copy `AGENTS.md` or rewrite unmarked `CLAUDE.md`.
2. runtime-workflow-execution remainder — Compatibility probe on that same async Prepare path (PATH / 2.1.261+ / required flags), without starting a Claude turn, daemon, or persistent server. If CLI or marked-agent preparation is invalid, `/hero-start` must fail with actionable copy and must not partially rewrite user-owned files.
3. Wire the Claude Prepare into the existing TUI `heroStartPrepareCmd` next to OpenCode/Codex (no-op when no Claude agents). Keep Bubble Tea `Update` non-blocking.

## Unmet spec requirements

- claude-projection: Prepare SHALL update only marked Claude agent fields in `.claude/agents/<agent>.md` from `workflow-config.yml`.
- runtime-workflow-execution: Claude preparation SHALL be non-blocking, scoped to marked assets, and fail `/hero-start` on invalid CLI or agent preparation.

TUI already *reads* `.claude/agents/<agent>.md` for stage prompts (`agentPromptRel`). Execute already passes `--model` / `--effort` on argv. That is not the Prepare contract: OpenCode/Codex sync native agent files before start; Claude has no `PrepareHeroStart` / `SyncAgentDefinition` and `opencode_prepare.go` never calls a Claude path.

Tests must use fake processes, NDJSON fixtures, temporary directories, and injected clocks/launchers. No live Claude account, login flow, daemon, process registry, or credential storage.
