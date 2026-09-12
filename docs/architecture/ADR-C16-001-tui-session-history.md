# ADR-C16-001 — Durable TUI Session History

> Cycle C16 architecture decisions. Product: [PRD-C16-001](../product/PRD-C16-001-tui-session-history.md). UI: [UI-C16-001](../product/UI-C16-001-tui-session-history.md).

| # | Decision | Status |
|---|---|---|
| ADR-091 | Hero session identity is durable and distinct from native harness identity | Proposed |
| ADR-092 | A session aggregate and append-only normalized events become the conversation source of truth | Proposed |
| ADR-093 | Conversation lifecycle belongs in a focused service; Bubble Tea remains an asynchronous view/controller | Proposed |
| ADR-094 | Database leases enforce one continuation owner per session | Proposed |
| ADR-095 | Native resume is exact; incompatibility creates an explicit context fork | Proposed |
| ADR-096 | Persistent sessions retain managed assets until explicit deletion | Proposed |
| ADR-097 | Adapter session history and deletion are optional narrow capabilities | Proposed |
| ADR-098 | Legacy bindings migrate idempotently without invented transcript data | Proposed |

## ADR-091: Hero session identity is durable and distinct from native harness identity

Context: Existing freechat state is process-local, while orchestration and stage rows each carry a harness-native ID. A parallel stage can have multiple agents, and provider IDs are meaningful only inside their owning harness. Reusing a provider ID as the History key would couple UI identity to adapter lifecycle and would not represent a session before a native ID is returned.

Decision: Assign every persisted conversation an opaque Hero session ID. Store native session ID, harness, model, effective property snapshot, conversation kind, optional cycle/stage/agent attribution, timestamps, lifecycle state, title, and transcript-availability state as attributes of that aggregate. Native identity is unique only within its harness. A session never resumes through another harness.

The cycle/stage foreign references are nullable attribution, not ownership: archiving or removing workflow rows must not silently delete History. Existing `cycles.orchestration_session_id` and `stages.harness_session_id` become compatibility inputs/projections during migration and are not the long-term aggregate key.

Consequences: Free Chat and parallel agents fit the same model. History survives process and cycle lifecycle boundaries. Store and service APIs route by Hero session ID and validate the harness/native pair before execution.

## ADR-092: A session aggregate and append-only normalized events become the conversation source of truth

Context: The current `conversation` table is an audit record for cycle approvals/assignments, not a complete TUI transcript. Reconstructing UI history from provider payloads would be inconsistent across harnesses and would omit local/Telegram presentation events.

Decision: Schema v12 adds cohesive session-history tables owned by `internal/store`:

- `sessions`: the aggregate and lifecycle metadata;
- `session_events`: append-only, monotonically sequenced normalized visible events with typed payloads and origin;
- `session_assets`: references that distinguish Hero-managed copies from immutable user-source paths;
- `session_leases`: current continuation owner and expiry/heartbeat data;
- an import marker on the session aggregate or a focused import table so confirmed remote imports are idempotent.

Exact columns, checks, and indexes belong in Planning, but the schema must support `last_activity DESC`, case-insensitive name lookup, bounded event paging, idempotent provider-event import, and a single transaction for event append plus session activity update. Transcript text and structured payloads remain local plaintext in `hero.db`; image bytes remain outside SQLite.

The old `conversation` audit table is not repurposed or destroyed. Audit records and user-visible session events have different retention and semantics.

Consequences: Locally visible history is deterministic and adapter-neutral. Long transcripts can page without reading the entire history. Schema growth and privacy implications are explicit.

## ADR-093: Conversation lifecycle belongs in a focused service; Bubble Tea remains an asynchronous view/controller

Context: ADR-061 established a transport-neutral `internal/conversation` service, but TUI memory still owns most transcript/session state. History lifecycle, Telegram ingress, execution routing, and persistence would diverge if implemented separately in the screen.

Decision: Extend the conversation vertical slice with a focused session repository/service boundary. The service owns create-on-first-turn, append, rename, list/search, archive/restore, lease, resume/fork, import, and delete rules. It accepts explicit dependencies and context; it does not import Bubble Tea, Lip Gloss, or Telegram types.

The History child model owns only presentation state (view, selection, query, focus, loading/error/dialog state, and bounded rows). Every store/filesystem/adapter operation is a `tea.Cmd` returning a typed message. `Update` remains non-blocking, `View` is pure, key bindings are centralized, and dimensions/styles are computed from terminal/theme messages. Existing Chat and Telegram paths call the same service rather than writing transcript state independently.

Implementation must apply the repository-local `golang-tui` and `go-engineering` skills. These are implementation constraints, not substitutes for the approved PRD/ADR/UI requirements.

Consequences: Session rules are testable without terminal rendering, and all ingress paths remain consistent. The TUI remains responsive during paging, remote import, deletion, and native health checks.

## ADR-094: Database leases enforce one continuation owner per session

Context: Multiple TUI processes can use the same project. Allowing simultaneous prompts against one native session can reorder provider turns and corrupt transcript attribution. A process-only mutex cannot coordinate independent TUIs, while a permanent lock would strand sessions after a crash.

Decision: Acquire a project-database lease before opening a session for continuation. The lease contains an unguessable process-instance owner ID, acquisition/heartbeat timestamps, and expiry. Only the owner may append user/assistant execution events or mutate that session's lifecycle. Heartbeats are bounded, cancellable work owned by the TUI; graceful navigation/exit releases the lease. A stale lease can be atomically replaced after a read-only native-status check and clear recovery messaging.

History listing remains read-only and requires no lease. The current owner may rename its idle session. Archive, restore, delete, session switch, and lease takeover are blocked during an unsafe active Execute/preflight.

Consequences: One-writer ordering holds across processes without a daemon. Crash recovery is possible. Planning must define expiry and heartbeat intervals, injected-clock tests, and transaction compare-and-swap semantics.

## ADR-095: Native resume is exact; incompatibility creates an explicit context fork

Context: Provider sessions are harness-bound and may also depend on their original model/properties. Silently sending an old provider ID to another adapter or model breaks continuity. Conversely, permanently blocking on an expired provider session discards valuable local context.

Decision: Normal resume uses the stored harness, model, effective properties, and native session ID. Availability and execution status are checked through the owning adapter. A completed stage session enters historical-continuation mode; this navigation does not alter stage state or dispatch scheduler transitions.

If exact resume is impossible, offer a user-confirmed fork. The new session uses a new native identity and receives a bounded, explicitly labeled context derived from the available normalized transcript. The original row remains unchanged. No harness/model substitution or remote transcript fetch occurs silently.

Consequences: Continuity failures are honest and recoverable. Planning must specify deterministic context serialization and size handling without introducing LLM summarization in C16.

## ADR-096: Persistent sessions retain managed assets until explicit deletion

Context: ADR-080 made C14 assets temporary, deleted session directories after seven days, and did not reconstruct cards after restart. Durable History requires the opposite retention behavior, while the user explicitly prohibited deletion of original source files.

Decision: Amend ADR-080 for sessions registered in the durable store. Their private session directory and managed asset copies remain until permanent session deletion. Startup cleanup may remove only provable orphans and non-durable legacy temporary sessions; age alone is not sufficient. Asset rows record ownership (`managed_copy` versus `external_source`) and stable card metadata. Only managed copies are deleted with the session. Original external paths are never unlinked.

Local database deletion and managed-file cleanup use a recoverable operation record or equivalent idempotent sequence defined in Planning so partial filesystem failure can be retried without deleting external files.

Consequences: Image cards survive restart and archive. Disk use becomes user-controlled through History deletion rather than a seven-day timer. Existing private permissions remain mandatory.

## ADR-097: Adapter session history and deletion are optional narrow capabilities

Context: Resume/status exist in the common harness contract, but providers differ in support for reading or deleting native session history. Expanding the mandatory adapter interface would force fake implementations and could break otherwise valid adapters.

Decision: Define narrow optional capability interfaces consumed by the conversation service, such as remote history reading and native deletion. Capability discovery is explicit. Remote history import requires user confirmation, normalizes provider events, and is idempotent. Permanent deletion first completes the authoritative local purge; only then does Hero best-effort native deletion using the captured harness/native reference. Unsupported/failed remote deletion produces a warning and never resurrects local data.

Session/native identifiers and provider payloads are redacted from logs. Adapter calls honor context cancellation and bounded timeouts.

Consequences: Each adapter exposes only truthful capabilities. Local privacy control does not depend on provider availability, while provider-side persistence limitations remain visible to the user.

## ADR-098: Legacy bindings migrate idempotently without invented transcript data

Context: Schema v11 may contain cycle orchestration and stage harness bindings but lacks normalized transcripts. Some rows omit model/property information or represent a stage shared by older execution logic.

Decision: Schema v12 migration preserves every existing row and creates at most one deterministic legacy History entry for each valid harness/native binding. It derives only metadata that is unambiguous from cycle/stage/config snapshots, marks missing transcript explicitly, and records a stable legacy source key so a second open cannot duplicate it. Ambiguous or empty bindings are left untouched and reported only through migration diagnostics, never guessed.

No network or harness process is started during database migration. Optional remote transcript import happens later through the confirmed UI flow.

Consequences: Existing resumable sessions become discoverable without pretending local messages exist. Migration remains deterministic, offline, and testable from copied v11 fixtures.

