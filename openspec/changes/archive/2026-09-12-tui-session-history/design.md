## Context

See `proposal.md` for motivation. Today schema is v11. Free Chat `freechatSessionID` and the in-memory `transcript []convMessage` die with the TUI process. Orchestration stores `cycles.orchestration_session_id` + `orchestration_harness_id`; each stage stores one `harness_session_id` + `harness_id`. The `conversation` table is a cycle-scoped audit log (approvals, assignments, validation reports), not a TUI transcript. `internal/conversation.Service` classifies and dispatches turns but does not persist them. C14 media lives under `{XDG_DATA_HOME}/hero/sessions/{tui-run-uuid}/` and is deleted after seven days of directory mtime. No History screen, leases, remote-history capability, or native-delete capability exist. Adapters can resume a native ID on Execute (Cursor UUID `--resume`, OpenCode GET `/session/{id}`, Codex `thread/resume`, Claude `--resume`) but none expose a public native-delete API; OpenCode `fetchSessionMessages` is SSE recovery only.

Authoritative requirements: PRD-C16-001 §§1–7, UI-C16-001 §§1–10, ADR-091–098. Idea notes yield on conflict. Browser UI Validation and QA End-to-End are disabled in this cycle's workflow-config. Scope is native=true → `generic_agent`. Implementation must apply repository `go-engineering` and `golang-tui` skills.

## Goals / Non-Goals

**Goals:**

- Durable project-scoped Hero sessions for every TUI-observed Free Chat, orchestration, Research, and named stage-agent conversation.
- Complete locally visible transcript persistence and restoration, including Telegram origin and asset cards.
- History screen with active/archived views, name search, rename, archive/restore, and permanent delete.
- Exact native resume under a one-writer lease, with honest fork and interrupted-turn recovery.
- Idempotent offline migration of legacy bindings without invented messages.
- Bubble Tea remains a non-blocking view/controller.

**Non-Goals:**

- Global cross-project history; importing Cursor IDE / OpenCode / Codex / Claude UI catalogs.
- Full-text search; LLM-generated titles or summarization; application-layer encryption.
- Automatic expiry/quotas; syncing the same session across machines.
- Reopening workflow stages by opening their conversations.
- New public CLI verbs for session CRUD (TUI + conversation service are sufficient in C16).
- Windows; live harness, network, Telegram Bot API, or wall-clock tests.

## Decisions

### D1 — Schema v12 tables (ADR-091, ADR-092, ADR-094, ADR-096, ADR-098)

Forward-only migration from v11. Do not recreate `hero.db`. Existing operational rows stay unchanged. The audit `conversation` table is neither dropped nor reused.

Physical names may be normalized for cohesion; the invariants below are required.

```text
sessions
  id TEXT PRIMARY KEY                 -- opaque UUID, Hero session identity
  kind TEXT NOT NULL                  -- freechat | orchestration | research | stage_agent
  title TEXT NOT NULL
  lifecycle TEXT NOT NULL             -- active | archived | interrupted | deleting
  harness_id TEXT NOT NULL DEFAULT ''
  native_session_id TEXT NOT NULL DEFAULT ''
  model TEXT NOT NULL DEFAULT ''
  model_properties_json TEXT NOT NULL DEFAULT '{}'
  cycle_id INTEGER REFERENCES cycles(id) ON DELETE SET NULL
  stage_name TEXT NOT NULL DEFAULT ''
  agent_name TEXT NOT NULL DEFAULT ''
  transcript_state TEXT NOT NULL      -- available | unavailable_legacy
  last_origin TEXT NOT NULL DEFAULT 'local'  -- local | telegram
  created_at TEXT NOT NULL            -- RFC3339 UTC
  last_activity_at TEXT NOT NULL
  interrupted_at TEXT
  remote_import_confirmed INTEGER NOT NULL DEFAULT 0
  legacy_source_key TEXT UNIQUE
  CHECK (kind IN ('freechat','orchestration','research','stage_agent'))
  CHECK (lifecycle IN ('active','archived','interrupted','deleting'))
  CHECK (transcript_state IN ('available','unavailable_legacy'))
  CHECK (last_origin IN ('local','telegram'))

CREATE UNIQUE INDEX sessions_harness_native
  ON sessions(harness_id, native_session_id)
  WHERE native_session_id != '';
CREATE INDEX sessions_active_activity
  ON sessions(lifecycle, last_activity_at DESC, id);
CREATE INDEX sessions_title_nocase ON sessions(title COLLATE NOCASE);

session_events
  session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE
  seq INTEGER NOT NULL                -- monotonic per session, starts at 1
  event_type TEXT NOT NULL            -- user | assistant | thinking | tool |
                                      -- warning | permission | question |
                                      -- attachment | asset | interruption | note
  origin TEXT NOT NULL                -- local | telegram
  origin_address TEXT NOT NULL DEFAULT ''
  payload_json TEXT NOT NULL
  provider_event_id TEXT NOT NULL DEFAULT ''
  created_at TEXT NOT NULL
  PRIMARY KEY (session_id, seq)
  CHECK (seq >= 1)
  CHECK (origin IN ('local','telegram'))

CREATE UNIQUE INDEX session_events_provider
  ON session_events(session_id, provider_event_id)
  WHERE provider_event_id != '';

session_assets
  session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE
  asset_id TEXT NOT NULL
  ownership TEXT NOT NULL             -- managed_copy | external_source
  path TEXT NOT NULL
  mime TEXT NOT NULL DEFAULT ''
  original_name TEXT NOT NULL DEFAULT ''
  card_meta_json TEXT NOT NULL DEFAULT '{}'
  PRIMARY KEY (session_id, asset_id)
  CHECK (ownership IN ('managed_copy','external_source'))

session_leases
  session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE
  owner_id TEXT NOT NULL              -- unguessable TUI instance UUID
  acquired_at TEXT NOT NULL
  heartbeat_at TEXT NOT NULL
  expires_at TEXT NOT NULL

session_delete_ops
  id INTEGER PRIMARY KEY
  session_id TEXT NOT NULL
  native_session_id TEXT NOT NULL DEFAULT ''
  harness_id TEXT NOT NULL DEFAULT ''
  status TEXT NOT NULL                -- intent | local_purged | remote_attempted | completed
  remote_warning TEXT NOT NULL DEFAULT ''
  created_at TEXT NOT NULL
  updated_at TEXT NOT NULL
  CHECK (status IN ('intent','local_purged','remote_attempted','completed'))
```

Event append and `sessions.last_activity_at` / `lifecycle` updates share one `store.InTx` transaction. Cross-session routing is rejected: an append MUST use the Hero session ID bound to that Execute. Sequence numbers are allocated inside the same transaction (`MAX(seq)+1`). `MaxOpenConns(1)` remains.

Cycle/stage columns are nullable attribution. Archiving or deleting a cycle MUST NOT cascade-delete History (`ON DELETE SET NULL`).

### D2 — Hero identity vs native identity (ADR-091)

Every persisted conversation has an opaque Hero session ID generated by Go (`crypto/rand` UUID). Native session ID, harness, model, and effective C5 property snapshot are attributes. Native identity is unique only within its harness. A session NEVER resumes through another harness.

Going forward, one Hero session exists per named agent conversation. Parallel Implementation agents (BACK/FRNT/GEN) MUST NOT share one History row. Nested generic `TASK` chips remain in the parent agent's session (same harness stream).

`cycles.orchestration_session_id` and `stages.harness_session_id` become compatibility projections. Chat resume uses the `sessions` aggregate. After a successful native-id bind, the service MAY still write the existing cycle/stage columns so older tooling does not see empty bindings; those columns are not the History key. Historical stages that stored only one native ID migrate as one legacy row (D11).

Standalone `hero chat` continues to use its synthetic Free Chat store under `~/.workflow-hero/` and MUST NOT list sessions from unrelated project `hero.db` files.

### D3 — Conversation service owns lifecycle (ADR-093)

Extend `internal/conversation` with a focused repository/service. The service owns create-on-first-turn, append, rename, list/search, archive/restore, lease, resume/fork, import, and delete. It accepts `context.Context`, store, clock, and optional adapter capabilities. It MUST NOT import Bubble Tea, Lip Gloss, or Telegram protocol types. Origin is an already-normalized enum (`local` | `telegram` + optional address).

TUI History is a child model owning only presentation state (view, selection, query, focus, loading/error/dialog, bounded rows). Every store/filesystem/adapter call is a `tea.Cmd` returning a typed message. `Update` remains non-blocking; `View` is pure; key bindings are centralized `key.Binding` / `key.Matches`. Dimensions come from `tea.WindowSizeMsg`. Existing Chat and Telegram ingress call the same service.

Empty Chat does not create a row. The first accepted user message or stage-agent prompt creates the session and first event atomically. Persistence failure is explicit, does not send to the harness, and blocks further sends until retry succeeds or the user cancels.

`/new-chat` releases the current lease, leaves the session in History, and opens an unpersisted empty Chat surface (PRD-C16-001 FR-13).

### D4 — Titles (PRD-C16-001 §3.2)

No LLM call.

- Free Chat with first textual user message: Unicode NFC, trim, collapse unicode whitespace to a single ASCII space, take the first 48 graphemes. If the result is empty, use the media fallback.
- Attachment-only first turn: `Image conversation`.
- Orchestration: `C{number} · Orchestration · ORCH`.
- Research: `C{number} · Research · DISC`.
- Named stage agent: `C{number} · {Stage} · {LABEL}` using the existing four-letter codes (BACK, FRNT, GEN, QA, JUDG, BUI, E2E). Stage display names stay English (`Implementation`, `QA`, …).

Rename trims surrounding whitespace, rejects empty, allows duplicates, and is legal for active and archived rows including the currently open idle owned session.

### D5 — Leases (ADR-094)

Injected clock in tests; production uses UTC.

- Each TUI process generates one unguessable `owner_id` at start (memory only).
- Heartbeat interval: 5 seconds via `tea.Cmd` (cancellable).
- Lease TTL: 30 seconds after last heartbeat.
- History listing is read-only and requires no lease.
- Continuation (open in Chat, append user/assistant execution events, lifecycle mutations except listing) requires a held lease.
- Acquire: INSERT if none; if existing lease `expires_at <= now`, atomic replace after a read-only native-status check and recovery copy; if unexpired and `owner_id` differs, return busy (`UI-C16-001` busy copy).
- Graceful navigation away, `/new-chat`, archive of current idle session, and TUI exit release the lease.
- Archive, restore, delete, session switch, and lease takeover are blocked while any Execute or `/hero-start` preflight makes the action unsafe. Browsing History during Execute remains allowed.
- The current owner MAY rename its idle session.

### D6 — Exact resume, interrupt, fork (ADR-095, PRD-C16-001 §3.5)

Normal resume uses stored harness, model, `model_properties_json`, and native session ID. Availability is checked through the owning adapter (`Status` / existing health). Composer stays on that pair; silent harness/model substitution is forbidden.

Completed stage-agent conversations enter historical-continuation mode: Chat shows `Historical continuation · workflow stage remains completed.` Navigation MUST NOT call `hero stage start`, change stage status, or dispatch scheduler transitions (runtime-workflow-execution delta).

If the TUI exits during a response: persist received events, set `lifecycle=interrupted`. On reopen, check native execution status. When the adapter can attach to a live turn, reconnect and keep the composer disabled until completion/cancellation. Otherwise show the adapter limitation and offer cancellation/recovery. Received events remain visible.

If exact resume is impossible, show the UI-C16-001 fork dialog naming `<harness>/<model>`. Confirmed fork:

- Creates a new Hero session (new id, new native identity after first Execute).
- Leaves the original row unchanged.
- Seeds the new session with a single labeled note plus a bounded deterministic context blob: newest-first user and assistant text events until 32 KiB UTF-8, then chronological order. Prefix `[Hero context fork from session <title>]`. No LLM summarization. Thinking/tool payloads are omitted from the blob.
- Legacy `unavailable_legacy` rows may fork with an empty blob and an explicit note that no local transcript exists.

### D7 — Optional adapter capabilities (ADR-097)

Do not enlarge the mandatory `HarnessAdapter` interface. Add optional interfaces consumed by the conversation service:

```text
RemoteHistoryReader
  SupportsRemoteHistory() bool
  ReadRemoteHistory(ctx, nativeSessionID) ([]NormalizedEvent, error)

NativeSessionDeleter
  SupportsNativeDelete() bool
  DeleteNativeSession(ctx, nativeSessionID) error
```

Capability discovery is type-assert / explicit supports methods. Adapter calls honor context cancellation and bounded timeouts. Session IDs, prompts, transcript text, attachment paths, Telegram identifiers, and provider payloads MUST NOT appear in diagnostic logs.

C16 truthful baseline (do not invent):

- Cursor, Codex, Claude: no public remote-history read; no native delete → leave interfaces unimplemented.
- OpenCode: `fetchSessionMessages` MAY back `ReadRemoteHistory` only if events can be normalized without invention; native delete remains unimplemented. SSE recovery stays internal and is not an implicit import.

Remote import requires user confirmation (UI-C16-001 copy). Imported events use `provider_event_id` uniqueness. Failed or unsupported read leaves local state unchanged.

Permanent delete: complete local purge first (events, metadata, lease, managed copies via `session_delete_ops`). Only then best-effort native delete. Unsupported/failed remote delete warns and MUST NOT resurrect local data.

### D8 — Assets and retention (ADR-096)

Amend ADR-080 for sessions registered in `sessions`. Private directory `{XDG_DATA_HOME}/hero/sessions/{hero-session-id}/` and managed copies remain until permanent session deletion. Image bytes stay out of SQLite and out of the Bubble Tea model.

`session_assets.ownership`:

- `managed_copy`: Hero-written UUID file; deleted with the session.
- `external_source`: original user path; NEVER unlinked.

Startup cleanup may remove only:

1. Session directories with no matching `sessions` row (orphans).
2. Non-durable legacy temporary directories that have no `sessions` row and are older than seven days.

Age alone MUST NOT delete a registered durable session. Pending composer chips before first send remain process-local; first send creates the Hero session then materializes attachments into that id.

Deletion protocol (`session_delete_ops`):

1. `intent` — capture harness/native refs and managed paths.
2. Delete SQLite session row (cascades events/assets/lease) inside `InTx`.
3. `local_purged` — remove managed files; retry is idempotent; never delete `external_source` paths.
4. `remote_attempted` / `completed` — best-effort native delete.

Partial filesystem failure keeps the op retryable. If step 2 fails, keep the row and all local assets.

### D9 — History UI (UI-C16-001)

Navbar (visible order, Alt shortcuts match visible index):

- Project, no cycle: `Chat | History | Status | Artifacts | Costs | Events | Settings` (`alt+1`–`alt+7`).
- Project, active cycle: plus `Config` (`alt+1`–`alt+8`).
- Standalone `hero chat`: `Chat | History | Settings` (`alt+1`–`alt+3`).

History is available without an active cycle. C15 Status findings remain on Status; they do not get a navbar item. Config remains conditional.

History content focus: `/` is search, not the command palette (footer must show this). Esc first clears/exits search; second Esc focuses the navbar. Left/Right switches Active/Archived. Enter opens (archived: restore then open). `r` rename, `a` archive/restore, `d` delete confirmation defaulting to Cancel.

Active rows: `last_activity_at DESC`, then session id. Search: case-insensitive name match within the selected view. Detail panel updates from already-loaded row data; no I/O in `View`.

Responsive: wide list+detail; medium stacked; narrow list with a details state. ANSI/rune-aware truncation; hide harness/model/timestamps before the name. Copy and empty/busy/legacy/interrupted/fork/import states follow UI-C16-001 §4 exactly. Modals own keyboard focus.

Chat header shows saved name plus existing cycle/freechat and harness information. Restore loads newest page (default 200 events) and scrolls to newest content. Context occupancy continues to represent the selected session, not a cross-session total. Archiving or deleting the currently open idle session opens a new empty Chat surface after persistence succeeds.

### D10 — Event model

Persist only events that were visible in Chat. Never fabricate private reasoning the harness did not expose. Telegram origin is stored on the event (`origin`, `origin_address`) and restored with `←/→ [Telegram · addr]` labels.

Long transcripts page by `seq` descending. Chat may request older pages when the user scrolls near the top; History detail does not load full payloads for list rows.

### D11 — Legacy migration (ADR-098)

Runs inside schema v12 migration / first open. No network, no harness process.

For each unambiguous binding, create at most one session:

- Orchestration: non-empty `orchestration_session_id` + `orchestration_harness_id` → `legacy_source_key = legacy:cycle:{id}:orchestration`.
- Stage: non-empty `harness_session_id` + `harness_id` → `legacy_source_key = legacy:cycle:{id}:stage:{name}`.

Derive only metadata present on the cycle/stage/config snapshot (kind, cycle number, stage name, harness, native id, title via D4). `transcript_state=unavailable_legacy`. Do not insert `session_events`. Ambiguous or empty bindings are skipped and reported only in migration diagnostics. A second open is a no-op because of `legacy_source_key` uniqueness.

Migrated rows remain resumable when the harness accepts the native ID.

### D12 — Testing and packages

Vertical slices:

- `internal/store`: v12 persistence
- `internal/conversation`: session service (no TUI types)
- `internal/tui`: History child model, navbar, Chat persist/restore cmds
- `internal/harness` + adapters: optional capabilities
- `internal/media`: retention gate using registered session ids
- `internal/telegram`: origin already flows through conversation; persist via service
- `internal/cycle` / `internal/engine`: no stage mutation on conversation resume; optional compatibility projection of native ids

Tests use real temporary SQLite, `t.TempDir()` media roots, injected clocks, and fake adapter capabilities. No live harness, network, Telegram Bot API, user home, or wall clock. Race tests for concurrent appends. Final gates: `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt`, `git diff --check`, `openspec validate tui-session-history --strict`. Binaries only under `./temp/` with cleanup.

## Risks / Trade-offs

- [One native ID per stage historically] → migrate one legacy row per stage binding; new executions are per named agent (D2, D11).
- [SQLite + filesystem cannot be atomic] → `session_delete_ops` recoverable protocol; retry tests mandatory (D8).
- [OpenCode message fetch vs truthful history] → optional capability only when normalization is complete; otherwise unsupported (D7).
- [Live reconnect differs by adapter] → reconnect when Status reports running and the adapter can attach; otherwise explicit cancel/recovery (D6).
- [C15 Status said no eighth navbar item] → that forbade a findings screen; History is a distinct C16 screen and Config becomes the eighth visible item when a cycle is active (D9).
- [C14 7-day cleanup vs durable History] → registered sessions are exempt; orphans and unregistered temp dirs still clean (D8).

## Migration Plan

Opening any schema-v11 `hero.db` applies v12 in the normal migration transaction, then D11 legacy entries. Absence of new rows yields empty History copy (`No saved sessions yet.`). Runtime asset upgrade is not required for agent JSON contracts; workflow-help and architecture-overview are updated in the docs task. Existing cycle/stage session columns remain readable.

## Open Questions

None blocking Planning. Physical filenames inside a vertical slice may move for cohesion; omitting a PRD-C16-001 FR or UI-C16-001 acceptance scenario is not allowed.
