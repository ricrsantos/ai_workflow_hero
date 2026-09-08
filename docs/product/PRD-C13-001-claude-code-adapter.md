# PRD-C13-001 — Claude Code Adapter for Hero TUI

> Cycle C13 requirements. Adds an opt-in fourth TUI harness while preserving Hero architecture. Design input: [active Claude adapter idea](../idea/v3.1_claude_adapter/claude_adapter.md) (non-normative).

## 1. Outcome

Hero supports `claude` as an explicit `harness + model` pair in the terminal UI. It uses the installed Claude Code CLI (minimum supported version: **2.1.261**) as a supervised headless subprocess for each turn, streams normalized events, persists and resumes Claude sessions, supports existing permission profiles, and participates in the same install, configuration, diagnostics, cost, and projection paths as Cursor, OpenCode, and Codex.

Target platforms remain Linux and macOS on amd64/arm64. Windows and Cursor IDE Runtime dispatch are out of scope.

## 2. Functional requirements

### Adapter execution, streaming, sessions, and health

- Add `internal/adapters/claude`, implementing `harness.HarnessAdapter` and applicable optional model, property-discovery, health, and start-preparation contracts.
- `Execute` validates `claude` on PATH, runs in `ProjectDir`, and invokes `claude -p --output-format stream-json --verbose --include-partial-messages --model <native-model>`, adding `--resume <id>` only for a Hero session bound to `claude`.
- Persist the first emitted Claude session id before turn completion. Never resend the original prompt when resume fails. Session binding must prevent cross-harness reuse.
- Decode NDJSON incrementally; normalize text, thinking, tools/results, subagent lifecycle, retry/hook/plugin activity, errors, session metadata, usage, and final result. The final result repairs partial loss. Unknown events never panic or vanish silently: emit a redacted/truncated Debug warning.
- The child and any bridge are execution-scoped: no Claude daemon, process registry, or orphan reaper. Cancel sends SIGINT to its process group, waits briefly, then kills only if necessary; it preserves the native session and does not prefer SIGTERM.
- Availability checks CLI/version/required flags only. Authentication is diagnosed during execution; Hero does not launch login or store credentials. Health observes the child, NDJSON reader, bridge, session, and last event without starting a second session.
- Stall timeout is five minutes; probes are every 30 seconds. Text, thinking, tools, retry, hooks, and subagent progress count as activity. Permission/question waits pause inactivity accounting.

### Permissions

- Reuse `harnesses.claude.permission_profile`: `ask`, `auto-project`, `auto-all`.
- `ask` uses Claude's permission-prompt mechanism plus a per-execution local/stdio MCP bridge. A one-time random token authenticates the bridge; it accepts only permission requests, converts them to `harness.PermissionRequest`, waits for the TUI decision, returns it, stores no credential, and exits with its child.
- A fixture/fake-process protocol spike is the first implementation task. It verifies the installed-version wire schema and semantics of `--permission-prompt-tool`. Unsupported `ask` fails explicitly and never downgrades to a permissive profile.
- `auto-project` maps to `acceptEdits` without silently granting shell, network, MCP, or external path permissions. `auto-all` maps to the native bypass equivalent and retains explicit risk confirmation.

### Native assets, project memory, models, and UI

- Add `assets/claude/` projection for Hero commands, `workflow-hero`/`grilling` skills, and native Claude agent templates. Enable provisions `.claude/`; disable retains files. Upgrade adds disabled Claude state and creates neither `.claude/` nor `CLAUDE.md` before explicit enable.
- Manage only a marker-delimited root `CLAUDE.md` block importing `@AGENTS.md` and identifying Hero context paths. Never invent/copy `AGENTS.md`. For existing `CLAUDE.md`, explicitly offer marked-block insert/update with diff or leave unchanged. Upgrade/uninstall touch only that block. Start preparation changes only marked agent fields.
- Add Claude to supported IDs, install, `/hero-harness`, `/hero-model`, Config, Doctor, Status, Telegram selector, fallback, help, and projections. Show agent, native model, and `claude` in every TUI turn.
- Add `assets/models/claude.yml` with native aliases `sonnet`, `opus`, `haiku`, `fable`, permitted full IDs/suffixes, official dated metadata, and no invented prices. Update model/checksum, overlay, cost, and context-bar flows.
- Map C5 `ef` to `--effort` only where catalog-supported. `th` and `fs` are unavailable (`na`). `system/init` is authoritative for effective model/capability. Chat verbosity follows current Compact/Standard/Detailed/Debug behavior; security gates/errors/warnings always show.

## 3. Constraints and quality

- Preserve the feature-based vertical-slice architecture: Claude protocol, parser, subprocess supervision, and bridge stay in `internal/adapters/claude`; do not add an engine rewrite, execution manager, generic daemon, or model translation layer.
- Apply idiomatic, focused Go with explicit errors/cancellation and deterministic real-file/embedded-asset tests. TUI work preserves non-blocking Bubble Tea Elm flow and responsive existing chrome.
- No real Claude account, request, or secret is required by tests.

## 4. Out of scope

Claude Agent SDK/Node bridge, interactive Claude TUI automation, `hero serve`, login/API-key handling, management of user hooks/plugins/MCP, `--bare`, attachments, session forks, agent teams, Windows, and Cursor IDE Runtime changes. No automatic enablement or third fallback.

## 5. Acceptance criteria

1. Claude 2.1.261+ runs opted-in TUI agents with streaming, usage, warnings, sessions, cancellation, and final output.
2. Mixed four-harness cycles show pair identity and never cross-resume sessions.
3. Permission profiles are explicit; `ask` has an end-to-end bridge fixture and fails safely if unsupported.
4. `.claude/` and managed `CLAUDE.md` discover Hero assets while preserving user content.
5. Catalog, C5, context bar, metrics, fallback, Doctor, Status, and Telegram use native ids without fabricated pricing.
6. Watchdog pauses for permission and cancellation leaves no child/bridge process.
7. `go test ./...` passes.
