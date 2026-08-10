# cursor-cli-harness-execution Specification

## Purpose
Execute AI work through Cursor Agent CLI via a full HarnessAdapter implementation (ADR-025; PRD-C03-001 §4.1–4.2).

## ADDED Requirements

### Requirement: Cursor adapter SHALL execute via Agent CLI
The Cursor adapter SHALL resolve `cursor-agent` or `cursor agent` on PATH, run prompts with `--print`, and support `--output-format json` and `stream-json` (design D3; `07_cursor_adapter.md`).

#### Scenario: Execute with print mode
- **WHEN** `Execute` is called with a prompt and CLI is available
- **THEN** the adapter runs the Cursor Agent CLI with `--print` and returns parsed output

#### Scenario: Resume session
- **WHEN** `Execute` is called with a non-empty session ID
- **THEN** the adapter passes `--resume=<session-id>` to the CLI

### Requirement: IsAvailable SHALL detect missing CLI and auth failures
`IsAvailable` SHALL verify the executable exists and report authentication failures with guidance to run `cursor agent login` (ADR-027; UI-C03-001 §2).

#### Scenario: Not logged in
- **WHEN** the CLI indicates authentication is required
- **THEN** `IsAvailable` returns an error whose message mentions `cursor agent login`

#### Scenario: CLI missing
- **WHEN** neither `cursor-agent` nor `cursor agent` is found
- **THEN** `IsAvailable` returns an error indicating the harness is unavailable

### Requirement: Workflow code SHALL NOT exec Cursor CLI directly
Only `internal/adapters/cursor` MAY invoke the Cursor Agent CLI process (ADR-025 layer separation).

#### Scenario: TUI calls harness interface
- **WHEN** the TUI runs an agent interação
- **THEN** it calls `HarnessAdapter.Execute` and does not shell out to `cursor` itself
