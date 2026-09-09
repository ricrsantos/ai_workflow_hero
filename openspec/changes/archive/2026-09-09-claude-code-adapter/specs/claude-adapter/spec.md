## Purpose

Provide a supervised, resumable Claude Code CLI execution boundary that exposes Claude activity through Hero's existing normalized harness contract without introducing a Claude daemon or provider-specific orchestration.

## ADDED Requirements

### Requirement: Claude execution SHALL use a compatible supervised CLI boundary

The Claude harness SHALL require `claude` on PATH, enforce the minimum supported version 2.1.261 and required headless flags, and execute each turn in the requested project directory with the native model id. Availability checks SHALL not start a session or perform authentication. A turn SHALL use `claude -p --output-format stream-json --verbose --include-partial-messages --model <model>` and SHALL add `--resume <id>` only for a Claude session bound to the same Hero execution context (PRD-C13-001 §§2–3; ADR-071).

#### Scenario: Missing Claude CLI
- **WHEN** Claude is selected but `claude` is not on PATH
- **THEN** availability and execution fail with an actionable message naming Claude Code and remediation, without starting a child process

#### Scenario: Incompatible Claude CLI
- **WHEN** the installed CLI is older than 2.1.261 or lacks a required headless flag
- **THEN** execution fails explicitly with the required minimum version or missing capability and does not fall back to a permissive invocation

#### Scenario: Headless turn invocation
- **WHEN** a Claude turn is executed with a native model and project directory
- **THEN** the child receives the required print/stream arguments, runs with that directory as its working directory, and receives the original prompt exactly once for a new session

#### Scenario: Authentication is diagnosed during execution
- **WHEN** the compatible child reports that Claude authentication is missing or expired
- **THEN** Hero surfaces actionable login guidance, does not open a login flow, and does not request or persist an API key

### Requirement: Claude NDJSON events SHALL normalize without silent loss

The adapter SHALL decode stream-json incrementally and map text, thinking, tool calls/results, subagent lifecycle, retry/hook/plugin activity, session metadata, errors, usage, and the final result to Hero's normalized stream and execution result. Unknown or malformed events SHALL produce a redacted/truncated Debug warning and SHALL not panic or disappear silently. The final result SHALL repair incomplete partial text and provide authoritative completion, usage, and effective-session data (PRD-C13-001 §2; existing harness-adapter contract).

#### Scenario: Text and thinking deltas
- **WHEN** the child emits text and reasoning deltas in separate NDJSON records
- **THEN** Hero emits corresponding text and thinking stream events in arrival order and includes the completed text in the execution result

#### Scenario: Tool and subagent activity
- **WHEN** the child emits tool calls/results, retries, hooks, plugins, or subagent lifecycle records
- **THEN** Hero emits normalized tool/activity events, attributes a subagent when a parent tool id is present, and counts the event as progress

#### Scenario: Unknown event
- **WHEN** the child emits an event type not recognized by the current parser
- **THEN** Hero emits a warning containing the harness and event type plus a redacted/truncated payload in Debug output, then continues reading subsequent records

#### Scenario: Final result repairs a partial stream
- **WHEN** a partial stream loses a text span but the final result contains the completed output and usage
- **THEN** the execution result uses the final authoritative output and usage while retaining already-visible stream activity

### Requirement: Claude sessions SHALL be persisted and harness-bound

The first Claude session id emitted by a turn SHALL be persisted before turn completion. A later turn MAY resume only a stored session whose harness identity is `claude`; a resume failure SHALL return an explicit error and SHALL NOT silently resend the original prompt. Session ids from Cursor, OpenCode, or Codex SHALL never be accepted as Claude resume ids (PRD-C13-001 §2; ADR-070–071).

#### Scenario: Session id arrives before completion
- **WHEN** Claude emits `session_id` before the final result
- **THEN** Hero stores the id immediately and returns it in the normalized execution result even if the turn later fails or is cancelled

#### Scenario: Same-harness resume
- **WHEN** a later Claude turn is bound to a previously stored Claude session
- **THEN** the child receives `--resume` with that id and receives only the new turn prompt

#### Scenario: Cross-harness resume is rejected
- **WHEN** a stored session belongs to Cursor, OpenCode, or Codex and a Claude turn attempts to use it
- **THEN** Hero rejects the binding before invocation and does not pass the foreign id to Claude

#### Scenario: Resume failure does not re-prompt
- **WHEN** Claude rejects a requested resume id
- **THEN** Hero reports the resume failure, preserves the native session id for diagnosis, and does not invoke a new turn with the original prompt automatically

### Requirement: Claude process lifetime SHALL be execution-scoped and cancellable

Each Claude child and permission bridge SHALL belong to one Execute call. Hero SHALL not create a Claude daemon, persistent process registry, or orphan reaper. Cancellation SHALL send SIGINT to the child process group first, wait briefly, and kill only if the group remains alive; cleanup SHALL preserve the native session and leave no child or bridge process behind (PRD-C13-001 §2; ADR-071).

#### Scenario: Normal turn cleanup
- **WHEN** a Claude turn completes with success or error
- **THEN** the child and any bridge terminate, resources are released, and no persistent Claude process state is written

#### Scenario: SIGINT-first cancellation
- **WHEN** the TUI cancels an in-flight Claude turn
- **THEN** Hero interrupts the process group with SIGINT, allows a bounded grace period, and uses kill only if necessary

#### Scenario: Cancellation preserves session
- **WHEN** a turn is cancelled after Claude emitted a session id
- **THEN** Hero returns the session binding for future explicit resume and does not delete or recreate the native session

### Requirement: Claude health SHALL observe the current turn without creating work

Health checks SHALL observe only the supervised child, NDJSON reader, bridge when present, known session, last event, and redacted stderr. They SHALL not start a second Claude session or consume tokens. Claude SHALL use a five-minute inactivity threshold with 30-second probes; text, thinking, tools, retries, hooks, plugins, and subagent progress count as activity, while a pending permission or question pauses inactivity accounting (PRD-C13-001 §2; UI-C13-001 §2).

#### Scenario: Health during active stream
- **WHEN** a Claude child is alive and recent normalized activity exists
- **THEN** health reports the supervised turn as healthy without launching another child

#### Scenario: Permission wait pauses watchdog
- **WHEN** the TUI is waiting for a permission or question response for longer than the normal inactivity interval
- **THEN** the watchdog does not report a false stall while the pending decision remains active

#### Scenario: Stalled live child
- **WHEN** the child, reader, and bridge remain alive but no qualifying activity arrives for five minutes outside a pending decision
- **THEN** health reports a suspected stall and the existing cancellation/reset path can act without starting a replacement session

