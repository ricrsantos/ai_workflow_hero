## ADDED Requirements

### Requirement: Continuation SHALL require an exclusive database lease
Hero SHALL acquire a project-database lease before opening a session for continuation. The lease SHALL contain an unguessable process-instance owner ID, acquisition/heartbeat timestamps, and expiry. Only the owner MAY append user/assistant execution events or mutate that session's lifecycle. Heartbeats SHALL be bounded cancellable TUI commands (5s interval, 30s TTL). History listing SHALL remain read-only without a lease. A stale lease MAY be atomically replaced after a read-only native-status check and recovery messaging. A second live owner SHALL receive the busy error (PRD-C16-001 §3.5, §4 FR-07; ADR-094; UI-C16-001 §4).

#### Scenario: Second TUI is busy
- **WHEN** TUI A holds an unexpired lease and TUI B opens the same session for continuation
- **THEN** TUI B shows `⚠ Session is open in another Hero TUI.` and does not append

#### Scenario: Stale lease is recoverable
- **WHEN** the recorded owner has `expires_at` in the past and the user opens the session
- **THEN** Hero replaces the lease after a native-status check and continuation proceeds

#### Scenario: Listing needs no lease
- **WHEN** a TUI opens History during another instance's continuation
- **THEN** the list loads and no lease is taken

#### Scenario: Unsafe execute blocks ownership changes
- **WHEN** an Execute or `/hero-start` preflight is active
- **THEN** archive, restore, delete, session switch, and lease takeover are disabled while browsing History remains allowed

### Requirement: Native resume SHALL be exact
Opening a session in Chat SHALL resume the stored harness, model, effective properties, and native session ID. Saved model-property values needed for semantic continuity SHALL be restored with the session. Hero MUST NOT silently change harness or model (PRD-C16-001 §3.5, §4 FR-05; ADR-095).

#### Scenario: Resume uses the stored pair
- **WHEN** the user opens a Free Chat session saved as `codex` / `gpt-5.6-sol` with stored properties
- **THEN** the next Execute uses that harness, model, property snapshot, and native session ID

#### Scenario: Cross-harness resume is forbidden
- **WHEN** the stored harness is `cursor` and the current freechat default is `opencode`
- **THEN** Hero still targets `cursor` for that session and does not send the native ID to OpenCode

### Requirement: Completed stage conversations SHALL continue without mutating workflow state
Continuing a completed stage-agent conversation SHALL enter historical-continuation mode. Chat SHALL show `Historical continuation · workflow stage remains completed.` The action MUST NOT reopen the stage, change stage status, or dispatch scheduler transitions (PRD-C16-001 §3.5, §4 FR-06).

#### Scenario: Completed QA conversation stays completed
- **WHEN** the user opens a completed QA agent session from History
- **THEN** the QA stage status remains completed and no Execute of the stage scheduler starts merely because Chat resumed

### Requirement: Interrupted sessions SHALL preserve events and recover or cancel explicitly
If a TUI exits during a response, received events SHALL remain visible and the session SHALL be marked interrupted. On reopen, Hero SHALL check native execution status, reconnect to a live stream when the adapter supports attaching, and keep the composer disabled until completion or cancellation; otherwise it SHALL explain the adapter limitation and offer cancellation/recovery (PRD-C16-001 §3.5, §4 FR-08; UI-C16-001 §§4,8).

#### Scenario: Partial stream survives restart
- **WHEN** the TUI is killed after three assistant events arrived
- **THEN** those three events are present after reopen and the session is `interrupted`

#### Scenario: Live turn gates the composer
- **WHEN** native status reports a still-running turn and the adapter can attach
- **THEN** Chat reconnects and the composer stays disabled until the turn completes or is cancelled

### Requirement: Native incompatibility SHALL offer an explicit context fork
If exact resume is impossible, Hero SHALL explain why and offer a user-confirmed fork that creates a new native session using a bounded, explicitly labeled deterministic serialization of the available local transcript. The original row SHALL remain unchanged. No harness/model substitution or remote transcript fetch SHALL occur silently. C16 MUST NOT call an LLM to summarize (PRD-C16-001 §3.5, §4 FR-09; ADR-095; UI-C16-001 §4).

#### Scenario: Fork dialog names the pair
- **WHEN** the original `cursor/composer-2.5` native session cannot be resumed
- **THEN** Chat asks `✗ The original cursor/composer-2.5 session cannot be resumed.` and offers to create a new session from local context

#### Scenario: Confirmed fork leaves the original unchanged
- **WHEN** the user confirms the fork
- **THEN** a new History row exists, the original row is unmodified, and the seeded context is prefixed `[Hero context fork from session <title>]` and bounded to 32 KiB of user/assistant text
