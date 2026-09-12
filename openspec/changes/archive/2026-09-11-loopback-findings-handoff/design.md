## Context

See `proposal.md` for motivation. Today `internal/engine.LoopBackToImplementation` resets downstream stages from a free-form reason; `internal/tui/implementation_assignment.go` assigns only unchecked OpenSpec `task-*` IDs; `internal/tui/stage_handoff.go` validates Implementation reports against that assignment; Judge/QA assets still mention gap files and `hero stage loop-back`. Schema is v10. `internal/todos` parses markdown-only Pending lines. `cycle.StatusView` has no finding/ToDo blocks.

Authoritative requirements: PRD-C15-001 §§1–14, UI-C15-001 §§1–16, ADR-083–090. Idea notes yield on conflict. Browser UI Validation and QA End-to-End are disabled in this cycle's workflow-config, but their contracts still ship so enabling those stages later uses the same path.

## Goals / Non-Goals

**Goals:**
- Scheduler-owned findings with stable IDs and deterministic fingerprints.
- One transaction for failed validation close + finding writes + loop-back.
- Mixed `task-*`/`find-*` Implementation assignments with exact diagnostics.
- Durable ToDos with recoverable file projection, Research adoption, and pending-only manual completion.
- Additive Status/Events/JSON/Telegram visibility without a new screen.
- Four-harness prompt parity that forbids agent-side state mutation.

**Non-Goals:**
- Reopening OpenSpec checkboxes; markdown gap files as scheduler input; agent-driven stage transitions.
- `/hero-add-todo` before Escalated; a general issue tracker; a findings navbar item.
- Semantic/fuzzy finding merge; `/hero-cycles` historical drill-down; Windows; live harness accounts.

## Decisions

### D1 — Schema v11 tables (ADR-083, ADR-087, ADR-088)

Forward-only migration from v10. Do not recreate `hero.db`. Existing operational rows stay unchanged; new tables start empty.

```text
findings
  id TEXT NOT NULL                  -- find-qa-1 / find-judge-1 / find-bui-1 / find-e2e-1
  cycle_id INTEGER NOT NULL REFERENCES cycles(id) ON DELETE CASCADE
  source_stage TEXT NOT NULL        -- qa | judge | browser_ui_validation | qa_end_to_end
  owner TEXT NOT NULL               -- backend_agent | frontend_agent | generic_agent
  status TEXT NOT NULL              -- open | done | reopened | deferred_todo
  fingerprint TEXT NOT NULL
  file TEXT
  requirement TEXT
  issue TEXT NOT NULL
  acceptance_criteria TEXT NOT NULL
  evidence_json TEXT NOT NULL DEFAULT '[]'
  round INTEGER NOT NULL DEFAULT 1
  todo_id TEXT
  created_at TEXT NOT NULL
  updated_at TEXT NOT NULL
  PRIMARY KEY (cycle_id, id)
  UNIQUE (cycle_id, fingerprint)
  CHECK (round >= 1)
  CHECK (length(issue) > 0 AND length(acceptance_criteria) > 0)
  CHECK (source_stage IN (...))
  CHECK (owner IN (...))
  CHECK (status IN (...))

finding_occurrences
  cycle_id INTEGER NOT NULL
  finding_id TEXT NOT NULL
  sequence INTEGER NOT NULL
  kind TEXT NOT NULL                -- created | done | reopened | rediscovered | deferred | deferred_recurrence
  source_stage TEXT NOT NULL
  round INTEGER NOT NULL
  issue TEXT NOT NULL
  acceptance_criteria TEXT NOT NULL
  evidence_json TEXT NOT NULL DEFAULT '[]'
  created_at TEXT NOT NULL
  PRIMARY KEY (cycle_id, finding_id, sequence)
  FOREIGN KEY (cycle_id, finding_id) REFERENCES findings(cycle_id, id)

todos
  id TEXT PRIMARY KEY               -- finding ID or todo-N
  origin_type TEXT NOT NULL         -- finding | legacy
  origin_finding_id TEXT
  origin_cycle_id INTEGER
  origin_source_stage TEXT
  summary TEXT NOT NULL
  acceptance_criteria TEXT
  status TEXT NOT NULL              -- pending | adopted | resolved
  adopted_cycle_id INTEGER
  resolution_note TEXT
  created_at TEXT NOT NULL
  updated_at TEXT NOT NULL
  resolved_at TEXT
  CHECK (status IN ('pending','adopted','resolved'))

todo_adoptions
  todo_id TEXT NOT NULL REFERENCES todos(id)
  sequence INTEGER NOT NULL
  cycle_id INTEGER NOT NULL
  status TEXT NOT NULL              -- adopted | released | resolved
  note TEXT
  created_at TEXT NOT NULL
  PRIMARY KEY (todo_id, sequence)

todo_projection_ops
  id INTEGER PRIMARY KEY
  cycle_id INTEGER
  op_kind TEXT NOT NULL             -- defer | complete | adopt | release
  idempotency_key TEXT NOT NULL UNIQUE
  status TEXT NOT NULL              -- intent_persisted | candidate_ready | installed | verified
  todo_ids_json TEXT NOT NULL
  candidate_sha256 TEXT
  created_at TEXT NOT NULL
  updated_at TEXT NOT NULL

cycles.completion_disposition TEXT NOT NULL DEFAULT ''
cycles.completion_disposition_json TEXT NOT NULL DEFAULT ''
```

Physical column names may be normalized further if cohesion requires it; the invariants above are required.

### D2 — Fingerprint and ID allocation (ADR-083)

IDs are allocated by Go, never invented by agents except a valid `reopen_id`.

- Namespaces: `find-qa-N`, `find-judge-N`, `find-bui-N`, `find-e2e-N` with N = max existing suffix in that cycle+namespace + 1.
- `reopen_id` is accepted only for an existing `done` finding in the same cycle with the same `source_stage` and `owner`. It wins over fingerprint matching.
- Otherwise canonicalize:
  - file: slash-normalize, trim, strip leading `./`; empty if absent
  - requirement and acceptance_criteria: Unicode NFC, trim, collapse any unicode whitespace to a single ASCII space; preserve case
  - issue text is stored and displayed, never hashed
- Fingerprint material is UTF-8 fields joined with U+001F: `cycle_id, source_stage, owner, file, requirement, acceptance_criteria`. Hash is lowercase SHA-256 hex.
- Exact match to `done` → reopen same ID and increment round.
- Exact match to `open` or `reopened` → append `rediscovered` occurrence, do not clone.
- Exact match to `deferred_todo` in the same cycle → append `deferred_recurrence` warning occurrence; do not reopen; do not count as an actionable finding.
- No match → new ID with status `open`.
- Different source stages never merge.

Redact secrets and reject file contents/image bytes in issue, acceptance, notes, and evidence paths using existing Hero redaction/safe-path rules before persist or display.

### D3 — One transaction, no nested CLI tx (ADR-084)

Introduce `store.InTx` (or equivalent) so service/engine share one `*sql.Tx`.

Validation of the typed report happens entirely before the transaction. The transaction then:

1. create/append/reopen findings and occurrences
2. close the source stage as Failed
3. apply existing loop-back stage resets (Implementation + downstream enabled stages)
4. write loop-back event payload with finding IDs (keep `from`/`to`; add `finding_ids`)
5. append assignment-relevant conversation/raw-result audit

CLI `hero stage close` must not begin a second transaction around `Service`/`Engine`. Standalone `hero stage loop-back` remains for administrative compatibility and MUST NOT create findings or anonymous repair work.

A failed validation close with zero actionable new/open/reopened findings is illegal and commits nothing. Judge `sdd_ambiguity=true` creates no findings and keeps the existing `/hero-back` versus `/hero-approve` path.

### D4 — Report validators and diagnostics (ADR-086)

Typed decoders in a focused package used by TUI handoff and CLI (for example `internal/cycle/reports` or colocated under `internal/cycle`). Unknown JSON fields fail closed.

Required diagnostic codes: `invalid_json`, `unknown_field`, `missing_field`, `invalid_enum`, `invalid_owner`, `unknown_reopen_id`, `duplicate_id`, `overlapping_arrays`, `assignment_union_mismatch`, `unassigned_id`, `false_acceptance_gate`, `nonempty_empty_assignment`, `no_actionable_finding`.

Each error exposes `code`, JSON field path, offending value, and a concise rule. Generic copy is only for unexpected internal errors. Invalid reports may be stored only in the existing safe audit channel and never as applied state.

Owner rules:

- QA and QA End-to-End entries require `owner` in `{backend_agent,frontend_agent,generic_agent}` and that owner must be active in cycle scope.
- Browser UI maps `failure_class=frontend` → `frontend_agent`, `failure_class=backend` → `backend_agent`; Visual failures are frontend. Missing visual reference PNGs are warnings, never findings.
- Judge implementation gaps require owner; default only when exactly one Implementation agent is active; otherwise fail closed.
- Invalid/unavailable owner rejects the entire report.

### D5 — Assignment union and empty waves (ADR-085)

Per active Implementation agent, ordered duplicate-free assignment is:

1. unchecked OpenSpec tasks owned by that agent (existing file order)
2. then findings with status `open` or `reopened` owned by that agent (stable creation/id order)

`cycle.StageAgentAuditBody.TaskIDs` stores both prefixes. Historical field names `tasks_completed` / `tasks_remaining` mean assignment item IDs. Their sets must be disjoint and their union must equal the exact assignment. IDs not assigned to that report are `unassigned_id`.

Scheduler applies a verified `task-*` by checking the OpenSpec box and a verified `find-*` by transitioning to `done` plus a `done` occurrence. Partial valid reports apply only completed IDs and may start another bounded productive wave. Invalid reports apply nothing.

Empty behavior:

- Implementation starting/restarting with zero pending tasks and zero actionable findings may run the existing one verification wave; that wave must return empty arrays.
- After a productive wave or user deferral, scheduler rereads both authorities and closes directly if empty; it never redispatches an empty wave.

A finding owner that is not in active Implementation scope fails before Execute.

### D6 — Projection reconciliation (ADR-087)

SQLite is authoritative for structured lifecycle. `context/current-state.md` `## Pending Features` remains the portable projection for pending and adopted items. Resolved items are removed from the file.

Projected line form (machine ID first, human-readable rest):

```text
- `find-qa-2` · pending · C15/QA · migration loses event rows
- `todo-4` · pending · legacy · Windows CLI support
- `find-qa-1` · adopted · C16 · deterministic loop-back assignment
```

Finding-derived ToDos keep the finding ID. Legacy markdown-only lines are promoted to project-scoped `todo-N` (N = max existing `todo-*` + 1) only when first adopted or manually completed. Exact selected line identity plus normalized content; never silently import every Pending line.

Recoverable protocol for `hero add-todo` / complete / adopt / release:

1. Build replacement candidate bytes from SQLite truth plus unmatched non-Hero prose lines.
2. Write and fsync candidate under `.workflow-hero/tmp/`.
3. Persist structured transition + `todo_projection_ops` with `idempotency_key` in SQLite (`intent_persisted`).
4. Atomically install the candidate onto `current-state.md`.
5. Re-read and verify IDs/hashes (`verified`).
6. Only then allow stage/cycle advancement.

Retry with the same idempotency key repairs either side and never duplicates rows or markdown lines. Projection failure leaves the loop Escalated. Non-Hero prose edits to unmatched lines are preserved. Structured-ID lines whose summary diverges from SQLite are not overwritten by read-only `/hero-sync`; explicit mutating commands rewrite from SQLite truth.

This is not claimed as a cross-resource transaction.

### D7 — Escalation triage and disposition (ADR-088)

`/hero-add-todo` is legal only for open/reopened findings in the currently Escalated loop. TUI without IDs opens the checklist; Cursor Runtime requires IDs then explicit confirmation. Unknown/done/deferred/foreign IDs mutate nothing.

Partial selection: selected findings become `deferred_todo` with linked pending ToDos; unselected findings and unchecked OpenSpec tasks remain blockers; stage stays Escalated; `/hero-continue [N]` remains a separate decision.

If the confirmed selection leaves zero unchecked OpenSpec tasks and zero open/reopened findings: do not dispatch Implementation or later validation; after projection verification, complete the cycle with existing status `completed` plus `completion_disposition=completed_with_deferred_todos` and JSON details (IDs, count, origin stages, safe summary). Downstream skipped stages keep skipped/not-run representation with reason `cycle completed with deferred ToDos`.

`/hero-finish` remains emergency. When open/reopened findings exist, require typed `FINISH` (or the existing stricter terminal convention) and warn that findings will not become ToDos. Emergency finish must not write `completed_with_deferred_todos`.

Terminal confirmation for defer-all uses typed `DEFER` or the existing stricter convention; not a single accidental keypress.

### D8 — Research adoption and manual completion (ADR-089)

At Research startup, after objective/config and active idea notes and before general grilling, query pending ToDos, present IDs/summaries in `user_preferred_language`, identify items already implied by the objective, recommend leaving unrelated items out, and persist selected items as `adopted` via deterministic CLI/service. No public `/hero-adopt-todo`.

Adopted items remain visible with the adopting cycle ID. They become `resolved` only after that cycle completes successfully through all enabled validation stages. Cancel, rollback/rejection, emergency finish, or another non-validating terminal outcome releases unresolved adopted items to `pending` and appends adoption history. A cycle that closes with newly deferred findings does not automatically resolve adopted items.

`/hero-complete-todo <id>...` accepts only `pending` items, requires a non-empty safe resolution note, promotes legacy lines when selected, rejects items adopted by an active cycle, is retry-idempotent for the same resolved ID, and rejects a conflicting new note. `/hero-todos` stays read-only and keeps the `/hero-sync` notice; default list is pending + adopted.

### D9 — Status JSON and UI surfaces (ADR-090)

Extend `cycle.StatusView` additively. Do not rename or remove existing fields. Follow existing mixed casing: keep `openspec_change`; new nested objects use camelCase (`loopBacks`, `sourceStage`, `findingIds`, `occurredAt`, `completionDisposition`, `availableActions`).

Empty data is empty arrays/zero counts or `completionDisposition: null`. No active cycle keeps current Status behavior. TUI Status adds compact sections below the stage table: loop-backs, findings, deferred ToDos, disposition/CTAs. `deferred_todo` renders as `ToDo` in the compact table and stays `deferred_todo` in JSON. No eighth navbar item. `/hero-cycles` is unchanged.

Events: `finding_created`, `finding_done`, `finding_reopened`, `finding_deferred`, `finding_deferred_recurrence`, `todo_adopted`, `todo_released`, `todo_completed_manual`, `cycle_completed_with_deferred_todos`. Payloads use IDs and safe metadata.

TUI Chat copy follows UI-C15-001 §§3–5. Commands reject multimodal attachments.

### D10 — Package placement and assets

Keep vertical slices:

- store: persistence
- engine: stage machine transitions (tx-aware helpers)
- cycle: orchestration façade + CLI + report validators
- tui: assignment, handoff, screens, dialogs, research startup
- todos: markdown parse/promote/project
- status: table/JSON query
- telegram: help/forward/compact status from the same JSON

Canonical prompts live in `assets/{cursor,opencode,codex,claude}/agents` and `commands`. Installed projections are upgrade-generated copies under existing checksum/conflict policy. Canonical Judge/QA/BUI/E2E/Implementation/discover/orchestration contracts include field rules, enums, one passing example, one reopening example where applicable, empty-success arrays, and prohibitions on checkbox/gap-file/`current-state.md`/stage mutation.

### D11 — Testing

Real temporary SQLite databases and `t.TempDir()` projections. Inject failures at every transaction and projection boundary. Race tests where workers share boundaries. No live harness or Telegram/network. Final gates: `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt`, `git diff --check`, `openspec validate loopback-findings-handoff --strict`. Binaries only under `./temp/` with cleanup. Apply `go-engineering` and `golang-tui`.

## Risks / Trade-offs

- [SQLite + file cannot be atomic] → recoverable `todo_projection_ops` and Escalated-until-verified; retry tests are mandatory.
- [Judge SDD ambiguity vs implementation gaps] → `sdd_ambiguity=true` creates no findings; keep `/hero-back`.
- [Disabled BUI/E2E this cycle] → still implement contracts/tests with fixtures so enabling those stages does not need another schema change.
- [Prompt drift across four harnesses] → one canonical field set plus projection parity goldens.
- [Standalone loop-back leftover] → keep for admin; automated path must use atomic close.

## Migration Plan

Opening any schema-v10 `hero.db` applies v11 in the normal migration transaction. No synthetic findings or ToDos. Absence of new rows yields empty additive JSON and existing visible behavior. Runtime asset upgrade refreshes affected agent/command files under checksum protection; customized files are warned, not overwritten. Existing `hero stage loop-back` and `/hero-finish` remain callable.

## Open Questions

None blocking Planning. Physical filenames inside a vertical slice may move for cohesion; omitting a PRD-C15-001 §10.4 responsibility is not allowed.
