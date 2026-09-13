# durable-session-history Specification

## Purpose
TBD - created by archiving change tui-session-history. Update Purpose after archive.

## Requirements

### Requirement: Hero SHALL persist a durable session aggregate distinct from native harness identity
Every conversation executed or observed by the Hero TUI SHALL receive an opaque Hero session ID. Native session ID, harness, model, effective model-property snapshot, kind, optional cycle/stage/agent attribution, timestamps, lifecycle, title, and transcript availability SHALL be attributes of that aggregate. Native identity SHALL be unique only within its harness. A session MUST NOT resume through another harness. Cycle and stage columns SHALL be nullable attribution with `ON DELETE SET NULL`; deleting a cycle MUST NOT delete History rows (PRD-C16-001 §3.1, §4 FR-01; ADR-091).

#### Scenario: First accepted turn creates a Hero session
- **WHEN** the user sends the first Free Chat message on an empty Chat surface
- **THEN** a `sessions` row is created with a new Hero ID, kind `freechat`, and `transcript_state=available` in the same transaction as the first event

#### Scenario: Empty Chat creates no row
- **WHEN** the user opens Chat and does not send a message or stage-agent prompt
- **THEN** no `sessions` row is inserted

#### Scenario: Parallel stage agents are distinct rows
- **WHEN** Implementation runs BACK and GEN as concurrent named agents
- **THEN** History contains one session per agent rather than one shared stage row

#### Scenario: Nested TASK stays on the parent session
- **WHEN** a named stage agent launches a nested generic Task
- **THEN** Task stream events append to the parent agent's Hero session

#### Scenario: Cycle delete keeps History
- **WHEN** a cycle row is removed after archive
- **THEN** sessions that referenced that cycle remain queryable with null cycle attribution

### Requirement: Visible transcript events SHALL append in stable session order
Hero SHALL persist every Chat-visible event (user, assistant, harness-exposed thinking, tool activity/results, warnings, permission and question prompts/outcomes, attachment and generated-asset cards, interruptions, and Telegram origin) as append-only `session_events` with monotonic `seq` starting at 1. Private reasoning the harness did not expose MUST NOT be stored. Event append and `last_activity_at` SHALL share one transaction. Cross-session routing MUST be rejected. Persistence failure SHALL be explicit and SHALL block further sends until recovery or cancellation (PRD-C16-001 §3.3, §4 FR-02, FR-14; ADR-092).

#### Scenario: Stream events are durable before the next send
- **WHEN** assistant text and a tool result become visible during a turn
- **THEN** both events exist in `session_events` with increasing `seq` and the session `last_activity_at` matches the latest event

#### Scenario: Cross-session append is rejected
- **WHEN** a stream callback attempts to append to a Hero session ID other than the Execute binding
- **THEN** the write is rejected and the other session is unchanged

#### Scenario: Persistence failure blocks send
- **WHEN** appending the accepted user event fails
- **THEN** Chat shows an actionable error and does not dispatch the harness Execute until retry succeeds or the user cancels

#### Scenario: Unexposed reasoning is omitted
- **WHEN** an adapter does not emit thinking deltas
- **THEN** no thinking `session_events` are invented

### Requirement: History SHALL list, search, rename, archive, restore, and delete project sessions
Hero SHALL list project-scoped sessions on History with Active sorted by `last_activity_at DESC` then id, a separate Archived view, and case-insensitive name search within the selected view. Rename SHALL trim whitespace, reject empty results, and allow duplicates. Archive SHALL be reversible and MUST NOT end the native harness session. Permanent delete SHALL require confirmation, remove local transcript/metadata/ownership/managed copies, never unlink original user files, and remain successful when remote native deletion fails with a warning (PRD-C16-001 §3.4, §3.7, §4 FR-03, FR-04, FR-10; UI-C16-001 §§2,5–7).

#### Scenario: Active list orders by recent activity
- **WHEN** two active sessions exist
- **THEN** the more recently active row appears first

#### Scenario: Search matches name only
- **WHEN** the user searches `deploy` and a session title contains `Deployment` but message bodies contain other text
- **THEN** the name match is listed and body text is not searched

#### Scenario: Duplicate rename is accepted
- **WHEN** the user renames a session to a title that already exists
- **THEN** the write succeeds

#### Scenario: Archive is reversible
- **WHEN** the user archives an idle session and later restores it
- **THEN** the native session ID is unchanged and the row returns to Active

#### Scenario: Local delete succeeds when remote delete fails
- **WHEN** local purge completes and native deletion is unsupported or fails
- **THEN** the History row is gone, managed copies are removed, original files remain, and an amber warning states that provider-side data may remain

#### Scenario: Failed local delete keeps data
- **WHEN** local SQLite or managed-file purge fails
- **THEN** the row and local assets remain, an actionable error is shown, and remote deletion is not requested

### Requirement: Long transcripts SHALL load by bounded pages
History list rows SHALL NOT load full payloads. Chat restore SHALL load the newest page of events (default 200) and MAY request older pages by `seq` when the user scrolls near the top so History and Chat remain responsive (PRD-C16-001 §5).

#### Scenario: Newest page restores first
- **WHEN** a session has more than 200 events and the user opens it
- **THEN** Chat shows the newest page scrolled to the latest content without reading the entire table in one query

### Requirement: Legacy bindings SHALL migrate idempotently without invented messages
Opening a schema-v11 database SHALL create at most one deterministic History entry per valid orchestration or stage harness/native binding, mark `transcript_state=unavailable_legacy`, and record a stable `legacy_source_key`. Ambiguous or empty bindings SHALL be skipped. Migration MUST NOT start a network or harness process. A second open MUST NOT duplicate rows (PRD-C16-001 §3.6, §4 FR-12; ADR-098).

#### Scenario: Orchestration binding becomes a labeled row
- **WHEN** a v11 cycle has a non-empty orchestration session/harness pair
- **THEN** History shows one orchestration entry titled with the D4 cycle-aware pattern and `Local transcript unavailable`

#### Scenario: Empty binding is skipped
- **WHEN** a stage row has an empty `harness_session_id`
- **THEN** no History row is created for that stage

#### Scenario: Second open is a no-op
- **WHEN** the same migrated database is opened again
- **THEN** no additional legacy sessions are inserted

#### Scenario: Live native identity skips legacy insert
- **WHEN** a History row already binds `(harness_id, native_session_id)` that a cycle or stage compatibility column also stores
- **THEN** opening the store SHALL skip that legacy insert, SHALL NOT fail, and SHALL NOT create a second row for the same native identity

### Requirement: Standalone hero chat SHALL NOT aggregate other projects
Standalone `hero chat` SHALL list only sessions from its synthetic Free Chat store and MUST NOT read unrelated project `hero.db` files (PRD-C16-001 §3.1).

#### Scenario: Project sessions stay out of standalone History
- **WHEN** a user opens `hero chat` while a different directory contains a project store with sessions
- **THEN** those project sessions are absent from standalone History
