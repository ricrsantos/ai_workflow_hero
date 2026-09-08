# Design: Claude Code Adapter for Hero TUI

## Context

This change adds Claude Code as an explicit, opt-in fourth harness for the Hero
TUI. The approved C13 PRD, ADR, and UI specification are authoritative. The
existing runtime already has a common `HarnessAdapter` contract, persisted
harness/session identity, a Bubble Tea conversation loop, install projections,
model-property catalogs, watchdogs, and deterministic CLI services. The design
extends those seams instead of introducing a second workflow engine.

The Claude CLI is a supervised child process. Hero starts one headless
`claude -p` execution per turn, consumes its incremental `stream-json` output,
and terminates the process group when a turn is cancelled. Claude native
sessions are bound to the Hero stage/harness identity already stored by the
runtime. There is no Claude daemon, persistent process registry, orphan reaper,
Agent SDK integration, login flow, or credential storage in this change.

The C13 protocol spike is a hard prerequisite. It must establish the exact
wire shape and supported flags for the minimum supported Claude CLI version
(2.1.261) before the production adapter or permission bridge is treated as
implementable.

## Goals

- Execute Claude Code in the existing TUI conversation pipeline with the
  normalized stream, session, usage, permission, question, cancellation, and
  health semantics already expected by Hero.
- Keep Claude process ownership, parsing, permission bridging, and compatibility
  handling local to `internal/adapters/claude` and small shared seams where an
  existing contract requires them.
- Make Claude installation, enable/disable, upgrade, uninstall, detection,
  model selection, Doctor, Status, and Telegram behavior explicit and testable.
- Preserve user-owned `.claude` content and root `CLAUDE.md` content while
  managing only marked Hero fields.
- Support mixed Cursor, OpenCode, Codex, and Claude installations without
  cross-harness fallback or session reuse.

## Non-goals

- Supporting Windows, the Claude Agent SDK, persistent Claude servers, `hero
  serve`, Claude login/authentication, user MCP/plugin/hook management,
  attachments, forks, teams, or a third fallback provider.
- Replacing the existing harness manager, workflow engine, Bubble Tea model, or
  database schema.
- Copying `AGENTS.md` into the project or rewriting an unmarked `CLAUDE.md`.

## Design decisions

### 1. Use the existing adapter and stage-session contract

`internal/adapters/claude` implements `harness.HarnessAdapter` and the existing
health/model interfaces. The adapter receives injected process, clock, random
token, filesystem, and executable lookup dependencies so unit and integration
tests never require a Claude account or a live binary.

Claude's lifecycle is narrower than a daemon-backed harness, so the shared
methods have explicit semantics:

- `CreateSession` creates only a local Hero execution binding; it does not start
  a Claude child process.
- `ResumeSession` validates a persisted Claude session bound to the same Hero
  stage/harness and lets the next `Execute` issue `--resume <id>`.
- `Execute` is the only operation that starts `claude -p` and emits the native
  session as soon as the `system/init` or equivalent session event provides it.
- `Status` reports the current execution snapshot from adapter-owned state.
- `Dispatch` is a thin command-to-`Execute` path and never creates a daemon.
- `Cancel` is idempotent and targets only the current execution process group.

The existing store fields for harness ID and native session ID remain the
source of truth. No new process registry or database migration is introduced.
The TUI's existing session callback must receive the first native ID before
completion so an interrupted turn can be resumed safely.

### 2. Gate implementation on a fake-process protocol spike

The spike uses a deterministic fake Claude executable and injected launcher to
record arguments and emit representative NDJSON. It covers version/flag
probing, `system/init` session identity, partial text, thinking, tool calls,
subagents, retries, hooks/plugins, usage, final result, malformed/unknown
events, resume, auth failure, and the `permission-prompt-tool` transport.

The spike produces a checked-in fixture contract and a compatibility result.
Unsupported ask-mode wire behavior is an explicit failure, never a silent
downgrade to auto-approve. Production code may only depend on fields and
flags established by the fixture.

### 3. Normalize NDJSON in stages and repair the final result

The adapter separates process supervision, line decoding, event normalization,
and result assembly. Each complete JSON line is mapped to the existing
`StreamDelta`/permission/question/usage contract. Text and thinking partials
are emitted incrementally; tool, activity, session, retry, hook, plugin,
subagent, and warning events remain observable through the normalized metadata
and watchdog path. Unknown events produce a redacted, bounded warning.

The assembler keeps the last known text/session/usage state and performs a
final-result repair when the CLI exits after a partial stream or omits a final
message. A non-zero exit, malformed terminal payload, compatibility error, or
permission bridge failure remains distinguishable from a normal completed
turn. Output is never fabricated to hide an execution error.

### 4. Keep permission handling execution-scoped and fail closed

The adapter maps Hero permission profiles to Claude's supported behavior:
`ask`, `auto-project` (constrained edits), and `auto-all`. For ask mode, the
execution-scoped stdio bridge receives a random one-time token, validates the
token and request shape, forwards only permission decisions to the TUI/Telegram
callback, and exits when the turn ends. It cannot expose arbitrary MCP/plugin
operations or credentials.

The bridge implementation follows the protocol-spike fixture. If the installed
CLI cannot provide the required ask transport, the adapter returns an
actionable unsupported-ask error. It does not approve, downgrade, or keep a
long-lived bridge alive. Pending permission/question states pause progress
watchdog accounting while retaining cancellation and timeout control.

### 5. Supervise one child process per turn

The process runner creates a new process group for each execution, applies the
working directory and resolved model/permission flags, streams stdout, and
captures stderr with redaction. Cancellation sends SIGINT to the group, waits
for the bounded grace period, and then kills the group if needed. Cleanup is
idempotent and covers abnormal exits, bridge errors, and context cancellation.

Availability checks are deterministic: PATH lookup, version comparison, and
required-flag capability checks only. They do not start a session. Claude uses
the C13 five-minute stall timeout and thirty-second health probe cadence;
normalized activity and progress events update the existing watchdog even when
the TUI does not render every event.

### 6. Project native assets with a marked root instruction block

The Claude projection owns `.claude/agents`, `.claude/commands`, and
`.claude/skills/workflow-hero`/`grilling` through the existing checksum and
conflict mechanisms. Enabling Claude creates or updates the projection;
disabling it removes only Hero-owned projected files according to current
uninstall semantics. An upgrade with Claude disabled does not create `.claude`
or `CLAUDE.md`.

The root `CLAUDE.md` block imports `@AGENTS.md` and is delimited by a stable
Hero marker. The writer must present an explicit choice when a file exists:
insert/update the marked block or leave it unchanged. It preserves all
unmarked user text, updates only marked managed fields, handles duplicate or
malformed markers safely, and writes atomically with the existing permission
mode. No operation copies or owns `AGENTS.md`.

### 7. Give Claude a namespaced local model catalog

`assets/models/claude.yml` is the authoritative local catalog for Claude
aliases (`sonnet`, `opus`, `haiku`, `fable`), full IDs, dated suffixes, and
official metadata. Entries resolve under the `claude` provider namespace so a
same-named Anthropic API model cannot silently win. Unsupported pricing is
represented as unknown; fabricated prices are forbidden.

The property adapter maps effort to `--effort`; thinking and fast-mode
properties remain `na` unless the spike proves a supported Claude flag. System
prompt/init injection and effective verbosity are represented in the existing
property resolution contract. Catalog discovery is local and does not invoke a
Claude session.

### 8. Preserve TUI Elm-architecture boundaries

All process, filesystem, health, catalog, permission, question, and Telegram
operations run from `tea.Cmd`/existing asynchronous callbacks. `Update` does
not block on Claude or bridge I/O. Claude is a fourth explicit harness in
enable/disable, boot diagnostics, harness/model pickers, labels, fallback
resolution, Telegram formatting, and mixed-harness tests. It is not exposed as
a persistent-server reset target. Long labels and stream metadata use the
existing ANSI-aware layout helpers.

## Data flow

```text
TUI tea.Cmd
  -> harness manager resolves explicit Claude pair
  -> Claude adapter validates stage binding and builds per-turn command
  -> process group: claude -p --output-format stream-json ...
  -> NDJSON decoder -> normalizer -> TUI stream relay / session store
                         |             |-> permission/question callback
                         |             |-> watchdog activity/health state
                         |             `-> final result/usage/error repair
                         `-> execution-scoped permission bridge (ask only)
```

Projection and model resolution are separate deterministic flows:

```text
install/enable -> Claude asset projection + marked CLAUDE.md decision
model picker   -> claude catalog/provider -> native model ID + properties
doctor/status  -> PATH/version/flags + enabled/projection/health diagnostics
```

## Component boundaries

- `internal/adapters/claude`: command construction, protocol decoder,
  normalization, result assembly, process lifecycle, bridge, and adapter tests.
- `internal/harness` / `internal/harnessmgr`: only the smallest shared contract,
  identity, health, activity, supported-ID, and fallback extensions needed by
  all harnesses.
- `internal/install` and `internal/upgrade` / `internal/uninstall`: Claude
  selection, projection, checksums, and preservation semantics.
- `internal/modelprops` and embedded assets: namespaced catalog and property
  mapping.
- `internal/tui`, `internal/doctor`, `internal/status`, and Telegram adapters:
  presentation and orchestration only; no Claude protocol parsing.

## Testing strategy

Use fake process runners, fake clocks, deterministic token sources, temporary
projects, and real embedded assets. Tests cover the protocol fixtures and
golden normalized events, session-before-completion persistence, resume and
cross-harness rejection, malformed/unknown events, ask/auto profiles,
bridge-token replay, cancellation escalation, watchdog pause/activity,
projection conflict/marker preservation, catalog precedence, boot/doctor/
status diagnostics, Telegram permission callbacks, and a four-harness mixed
scenario. No test requires Claude installed, network access, credentials, or a
live account. Finish with `go test ./...` and the repository's documented
static/build checks.

## Migration and rollback

Existing projects keep their current harness state. Claude is disabled unless
explicitly selected. Upgrade does not project disabled Claude assets. Enabling
and disabling uses existing state/checksum migrations; no database migration is
needed. Uninstall removes only Hero-owned Claude projection and preserves
user-owned `.claude` files and unmarked `CLAUDE.md` text. If a projection or
marked-block update is rejected, the operation reports the conflict and leaves
the previous files unchanged. Removing the Claude selection and reverting the
adapter leaves Cursor, OpenCode, and Codex routing untouched.

## Traceability

- `docs/product/PRD-C13-001-claude-code-adapter.md`: execution, permissions,
  assets, catalog, acceptance criteria, and out-of-scope boundaries.
- `docs/architecture/ADR-C13-001-claude-code-adapter.md`: opt-in harness,
  supervised process, permission bridge, projection, and catalog decisions.
- `docs/product/UI-C13-001-tui-claude-code-adapter.md`: picker, labels,
  diagnostics, permission waits, preservation choices, and reset behavior.
- Existing `harness-adapter`, `asset-bootstrap-and-layout`,
  `runtime-workflow-execution`, `model-property-*`, and `cli-deterministic-*`
  specs: compatibility with current contracts.
