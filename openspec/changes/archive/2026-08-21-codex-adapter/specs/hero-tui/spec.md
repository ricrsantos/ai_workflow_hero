# hero-tui Specification

## MODIFIED Requirements

### Requirement: Install harness picker SHALL list Codex as third supported harness
`hero install` SHALL present Cursor, OpenCode, and Codex checkboxes. Selection is PATH-independent and requires at least one harness (UI-C06-001 §2; PRD-C06-001 §4.6).

#### Scenario: Third checkbox present
- **WHEN** the user runs interactive install
- **THEN** the harness list includes an unchecked Codex option alongside Cursor and OpenCode

#### Scenario: Codex-only install valid
- **WHEN** the user selects only Codex and continues
- **THEN** install succeeds with Codex enabled and `.codex/` projected

### Requirement: /hero-harness SHALL manage Codex enable and disable
The harness picker SHALL show Codex with available/unavailable suffix. Enabling provisions `.codex/`; disabling keeps files; last enabled harness cannot be disabled (UI-C06-001 §3).

#### Scenario: Enable success copy
- **WHEN** the user enables Codex in `/hero-harness`
- **THEN** Chat shows `✓ Codex enabled (projected .codex/)`

#### Scenario: Disable success copy
- **WHEN** the user disables Codex
- **THEN** Chat shows `✓ Codex disabled (files kept)`

### Requirement: /hero-model SHALL include Codex when enabled
Step 1 of the two-step picker SHALL list Codex when `harnesses.codex.enabled` is true. Step 2 SHALL list native Codex model ids from the adapter (UI-C06-001 §4).

#### Scenario: Harness step lists Codex
- **WHEN** Codex is enabled and the user opens `/hero-model`
- **THEN** step 1 includes a Codex row

#### Scenario: Model step uses native ids
- **WHEN** the user selects Codex in step 1
- **THEN** step 2 lists native Codex ids without cross-harness translation

### Requirement: Chat speaker SHALL show codex harness id
Transcript and green-pane status lines SHALL use lowercase harness id `codex` in the `[LABEL - model · harness]` format (UI-C06-001 §5).

#### Scenario: Orchestrator on Codex
- **WHEN** orchestration_agent runs on Codex with model `gpt-5.4`
- **THEN** the speaker line includes `[ORCH - gpt-5.4 · codex]`

### Requirement: Codex errors SHALL use established red/yellow copy
Auth, missing CLI, app-server failure, and Prepare probe failure SHALL render the UI-C06-001 §6 templates without API key prompts (ADR-047).

#### Scenario: Unauthenticated template
- **WHEN** Codex Execute fails for missing login
- **THEN** Chat shows `✗ Codex is not authenticated.` with `codex login` suggestion

#### Scenario: App-server failure template
- **WHEN** Codex app-server fails to start or handshake fails
- **THEN** Chat shows incompatible/not installed guidance and notes Hero does not pin Codex CLI version

### Requirement: /harness-reset SHALL include Codex when enabled
The reset picker SHALL offer Codex when enabled, stop the Hero-managed app-server, and yellow-warn if not started (UI-C06-001 §7).

#### Scenario: Reset stops Codex child
- **WHEN** the user selects Codex in `/harness-reset` and a Hero-managed app-server is running
- **THEN** the child is stopped using the same graceful-then-kill pattern as OpenCode reset
