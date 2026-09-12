# PRD-C15-001 — Deterministic Loop-back Findings and Deferred ToDos

> Cycle C15 product requirements. Replaces prose-only QA/Judge gap handoff with
> scheduler-owned findings, makes escalation triage durable, and lets later
> Research sessions adopt deferred work. Architecture: [ADR-C15-001](../architecture/ADR-C15-001-loopback-findings-handoff.md).
> Terminal UX: [UI-C15-001](UI-C15-001-loopback-findings-handoff.md).

## 1. Outcome

Hero carries every actionable failure from QA, Judge, Browser UI Validation,
or QA End-to-End into an exact Implementation assignment. Findings have stable
IDs and lifecycle state in `hero.db`; OpenSpec remains the planning checklist.
The Go scheduler, not an agent, validates reports, persists transitions, writes
OpenSpec checkboxes, and changes finding state.

When an exhausted loop escalates, the user can explicitly choose which findings
must still be resolved in the current cycle and which become durable project
ToDos. Deferring every blocker closes the cycle with a visible
`completed_with_deferred_todos` disposition. A later Research session offers
pending ToDos for adoption. Pending ToDos may also be completed manually when
they were resolved outside Hero.

The behavior is identical across Hero TUI and Cursor IDE Runtime. The TUI
automates the deterministic commands; Cursor Runtime invokes the same CLI API.

## 2. Problem

In C13 and C14, a post-validation Implementation restart could have an empty
OpenSpec assignment because `tasks.md` was already fully checked. Agents then
reported earlier task IDs as completed. ADR-075 correctly rejected those
unassigned IDs, but the user only saw the generic message `one or more
stage-agent reports are invalid or missing required gate fields`.

Validation agents also produced prose files such as `qa-gaps.md` and
`judge-gaps.md`. Those artifacts were not scheduler inputs. The loop-back reason
was free-form text, so Implementation had no live checklist representing the
actual regression. Judge instructions additionally conflicted with TUI handoff
ownership by telling the agent to write a gap artifact and invoke loop-back.

The result was a fragile information boundary:

- validation findings could be lost between agents;
- completed OpenSpec tasks were incorrectly reused as repair IDs;
- report failures did not identify the violated contract;
- stage, finding, and loop-back writes could diverge;
- exhausted cycles had no controlled way to defer selected blockers;
- project ToDos were read-only prose and could not be adopted or resolved with
  a durable lifecycle.

## 3. Goals

1. Give every validation finding a stable, assignable `find-*` ID.
2. Make failed-stage close, finding persistence, and loop-back one atomic
   SQLite operation.
3. Extend ADR-075 assignments from pending `task-*` IDs to the disjoint union
   of pending `task-*` and actionable `find-*` IDs.
4. Fail closed with field-specific diagnostics when any report is invalid.
5. Define explicit, example-backed JSON contracts in every affected agent.
6. Give the user deterministic escalation triage through `/hero-add-todo`.
7. Persist project ToDos, adoption attempts, manual resolution, and audit data.
8. Surface active-cycle ping-pong in Status, Events, Runtime, JSON, and
   Telegram without adding another navbar screen.
9. Migrate existing schema-v10 projects without data loss.

## 4. Personas and primary journeys

### 4.1 Cycle operator

A developer running Hero in the TUI or Cursor needs to understand why a stage
failed, which implementation agent owns each repair, and whether a repeated
failure reopened earlier work.

### 4.2 Escalation decision maker

When iteration limits are exhausted, the user classifies every open blocker as
either "resolve in this cycle" or "defer to project ToDos". The decision must
not be inferred from silence.

### 4.3 Later-cycle researcher

At Research startup, the user sees pending project ToDos and decides which ones
belong in the new cycle. Selected items remain visible as adopted until the
cycle validates them successfully.

### 4.4 External resolver

A maintainer may fix a pending ToDo outside a Hero cycle. They need a guarded
way to mark it complete with a durable explanation.

## 5. Domain requirements

### 5.1 Finding identity

- User-facing IDs are stable within a cycle and source namespace:
  `find-qa-1`, `find-judge-1`, `find-bui-1`, and `find-e2e-1`.
- Hero allocates an ID when a valid failure entry omits one.
- An agent may provide `reopen_id` only for an existing `done` finding in the
  same cycle and valid source/owner context.
- `reopen_id` takes precedence over fingerprint matching **only when** the
  report's canonical `file`, `requirement`, and `acceptance_criteria` match the
  stored finding contract. A mismatched contract is `unknown_reopen_id`; the
  agent must omit `reopen_id` so Hero allocates a new ID.
- Reopen, rediscovery, done, and deferral do **not** overwrite the stored
  issue, file, requirement, or acceptance criterion. New issue text is
  append-only on `finding_occurrences`. Implementation assignments always
  show the frozen contract plus occurrence history.
- Without `reopen_id`, equality is deterministic. The fingerprint uses cycle,
  source stage, owner, canonical file or requirement, and normalized acceptance
  criterion. Free-form issue wording is not semantically compared.
- An exact fingerprint match to `done` reopens that ID. A match to `open` or
  `reopened` appends an occurrence without cloning the finding. No exact match
  creates a new ID.
- Different source stages do not silently merge findings.

### 5.2 Finding record

The operational model must retain at least:

- ID and cycle ID;
- source stage: `qa`, `judge`, `browser_ui_validation`, or `qa_end_to_end`;
- owner: `backend_agent`, `frontend_agent`, or `generic_agent`;
- status: `open`, `done`, `reopened`, or `deferred_todo`;
- deterministic fingerprint;
- canonical file and/or requirement reference;
- issue summary and independently testable acceptance criterion;
- current round and timestamps;
- optional safe evidence paths;
- append-only occurrences for creation, rediscovery, reopens, implementation
  completion, and deferral;
- linked ToDo ID when deferred.

Issue text, acceptance criteria, notes, and evidence paths must never include
credentials, file contents, image bytes, or other secrets. Existing Hero
redaction and safe-path conventions apply before persistence or display.

### 5.3 Finding lifecycle

```text
new failure ──→ open ──Implementation complete──→ done
                  ↑                                  │
                  └──────── rediscovered ── reopened─┘

open/reopened ──user defers at Escalated──→ deferred_todo
```

- Only Go scheduler/service code changes finding status.
- `done` means the assigned Implementation agent reported and verified that
  exact ID; it is not a promise that later validation cannot reopen it.
- Reopening preserves the ID and appends the new round, issue context, source
  report, and evidence. The finding row's issue, file, requirement, and
  acceptance criterion stay frozen; Implementation is assigned that contract.
- A `deferred_todo` finding is no longer assignable in its source cycle.
- If the same fingerprint is reported later in that same cycle, Hero records a
  warning occurrence against the deferred finding, does not reopen it, and lets
  the validation stage complete if no other failures remain.

### 5.4 Owner resolution

- QA and QA End-to-End failure entries require a valid `owner`/`agent`.
- Browser UI maps `failure_class=frontend` to `frontend_agent` and
  `failure_class=backend` to `backend_agent`; Visual failures are frontend.
- Missing visual reference PNGs remain warnings and never create findings.
- Judge implementation gaps require an explicit owner. A missing owner may
  default only when exactly one Implementation agent is active; otherwise the
  report fails closed.
- An invalid or unavailable owner prevents the entire report from being
  persisted.

## 6. Structured report contracts

### 6.1 Common validation rules

QA, Judge, Browser UI, and QA End-to-End must emit one JSON object and stop.
They must not:

- call `hero stage close`, `hero stage loop-back`, or another transition;
- edit OpenSpec task checkboxes;
- write `qa-gaps.md`, `judge-gaps.md`, or another operational gap file;
- edit `context/current-state.md`;
- invent `find-*` IDs not provided in their current context.

Each failure/gap entry must contain enough structured data for deterministic
routing and verification. At minimum: `owner` when not derivable, `file` and/or
`requirement`, non-empty `issue`, non-empty `acceptance_criteria`, optional
`evidence`, and optional valid `reopen_id`.

The canonical embedded agent source for every supported harness must include:

- the complete field rules;
- one passing example and one reopening example;
- the prohibition on stage/file mutation;
- valid enum values and owner rules;
- the instruction to return empty failure arrays on success.

Installed Cursor, OpenCode, Codex, and Claude projections must remain generated
from those canonical assets so contracts do not drift by harness.

### 6.2 QA and QA End-to-End

```json
{
  "status": "failed",
  "failures": [
    {
      "owner": "generic_agent",
      "file": "internal/tui/stage_handoff.go",
      "requirement": "PRD-C15-001 §7.1",
      "issue": "Open finding IDs are absent from the Implementation assignment.",
      "acceptance_criteria": "The next assignment contains every open finding ID exactly once.",
      "evidence": ["go test ./internal/tui"],
      "reopen_id": null
    }
  ],
  "summary": "One deterministic handoff failure."
}
```

QA End-to-End uses the same failure shape and adds its existing journey fields.

### 6.3 Judge

Judge keeps implementation gaps separate from SDD ambiguity:

```json
{
  "status": "failed",
  "implementation_gaps": [
    {
      "owner": "generic_agent",
      "requirement": "PRD-C15-001 §9.4",
      "issue": "Manual completion accepts an adopted ToDo.",
      "acceptance_criteria": "Only pending ToDos can be manually completed.",
      "evidence": [],
      "reopen_id": null
    }
  ],
  "sdd_ambiguity": false,
  "summary": "One implementation gap."
}
```

`sdd_ambiguity=true` does not create a finding and continues to use the existing
`/hero-back` versus `/hero-approve` decision path.

### 6.4 Browser UI Validation

Browser failures use the common entry shape plus `failure_class`. Browser
Health still runs before Visual. Visual is skipped after Health failure.
Reports and screenshots remain evidence files; the finding stores only safe
paths to them.

### 6.5 Implementation

Implementation agents receive an explicit ordered assignment containing
`task-*`, `find-*`, or both. Their existing report is extended, not replaced:

```json
{
  "status": "complete",
  "tasks_completed": ["task-03.2", "find-qa-1"],
  "tasks_remaining": [],
  "tests_passed": true,
  "acceptance_gates": {
    "completed_tasks_verified": true,
    "task_ownership_respected": true,
    "required_tests_passed": true
  },
  "summary": "Assigned task and finding verified."
}
```

`tasks_completed` and `tasks_remaining` are disjoint and their union must equal
the exact assignment. IDs not assigned to that report are rejected. A truly
empty verification assignment accepts only empty arrays.

## 7. Deterministic stage handoff and scheduling

### 7.1 Atomic failed-stage handoff

Extend the deterministic CLI API so a failed validation close accepts the full
validated report, for example:

```text
hero stage close --name qa --failed --findings-json '<JSON>'
```

The service validates the whole report before opening a transaction. One
SQLite transaction must:

1. create, append, or reopen every finding;
2. close the source stage as failed;
3. reopen Implementation and reset affected downstream stages according to the
   existing loop-back rules;
4. store the finding IDs in the loop-back event and Implementation summary;
5. append assignment-relevant audit data.

If validation or any transactional write fails, none of those changes commit.
Closing a validation stage as failed without at least one actionable new/open/
reopened finding is rejected. The standalone `hero stage loop-back` remains for
backward compatibility but the C15 agent-driven flow must use the atomic API.

### 7.2 Assignment construction

For each active Implementation agent, the scheduler builds one ordered,
duplicate-free assignment from:

1. unchecked OpenSpec tasks owned by that agent; then
2. findings with status `open` or `reopened` owned by that agent.

The assignment and raw result remain append-only records in `conversation`.
`task_ids` may contain both prefixes. Existing OpenSpec owner validation still
applies. A finding owner that is not in the active Implementation scope fails
before Execute.

### 7.3 Applying Implementation results

- A verified `task-*` completion lets the scheduler check its OpenSpec box.
- A verified `find-*` completion transitions that finding to `done` and appends
  an occurrence.
- A partial result applies only valid completed IDs, retains remaining IDs, and
  may schedule another bounded productive wave under ADR-075.
- Invalid, missing, duplicate, foreign, or overlapping IDs apply nothing and
  return a specific diagnostic.
- The scheduler must distinguish contract failures such as unknown field,
  missing field, invalid enum, unassigned ID, duplicated ID, union mismatch,
  false gate, invalid owner, and omitted empty assignment.
- Starting/restarting Implementation with zero pending tasks and zero actionable
  findings may use the existing single verification wave. After work or user
  deferral empties the assignment, it closes directly and never redispatches an
  empty wave.

## 8. Escalation triage

### 8.1 Availability

`/hero-add-todo` is a mutating control command available only when the relevant
loop is `Escalated`. It accepts one or more open/reopened finding IDs. In the
TUI, invoking it without IDs opens the checklist specified by UI-C15-001. In
Cursor, the user supplies IDs in the slash command; Runtime calls a deterministic
`hero add-todo` CLI operation.

### 8.2 Partial deferral

- Selected findings become `deferred_todo` and linked pending ToDos.
- Unselected findings and unchecked OpenSpec tasks remain blockers.
- The stage stays `Escalated`.
- The user must separately run `/hero-continue [N]` to authorize more iterations
  for remaining work.

### 8.3 Deferring every blocker

If the confirmed selection leaves no unchecked OpenSpec tasks and no open/
reopened findings:

- no empty Implementation wave runs;
- the cycle closes immediately with normal cycle status `completed`;
- a structured disposition `completed_with_deferred_todos` is stored;
- disposition details contain IDs, count, origin stages, and a safe summary;
- enabled downstream stages are not executed;
- Chat, Status, Events, status JSON, and completion output state explicitly
  that the cycle closed with deferred work.

This is a controlled completion, distinct from `/hero-finish`.

### 8.4 Emergency finish

`/hero-finish` remains available. When open/reopened findings exist, it requires
a strong confirmation that those findings will not become ToDos. Existing
emergency semantics remain otherwise unchanged.

## 9. Durable ToDos

### 9.1 ToDo identity and state

Finding-derived ToDos retain the finding ID as their user-facing identifier.
A legacy markdown-only item receives a project-scoped `todo-*` ID when first
adopted or manually completed.

Structured ToDos store at least: ID, origin type, origin finding/cycle/stage,
safe summary, acceptance criterion when present, state (`pending`, `adopted`,
`resolved`), current adopted cycle when any, timestamps, and resolution note.
An append-only adoption history records every selected, released, and resolved
attempt.

### 9.2 `current-state.md` projection

`hero.db` is authoritative for structured ToDo lifecycle. The recognized
`## Pending ...` section in `context/current-state.md` is the durable,
human-readable project projection and remains consumable without Hero.

`hero add-todo` must be idempotent across the database and file boundary:

1. write/validate an atomic replacement candidate for the projection;
2. persist the structured transition and projection intent;
3. install/verify the projection;
4. only then permit stage/cycle advancement.

A crash or file-write failure must be safely retryable without duplicate lines.
Until reconciliation succeeds, the loop remains Escalated. The SDD may choose
the exact recoverable ordering, but may not claim cross-resource atomicity or
advance with an unprojected pending item.

### 9.3 Research adoption

At the start of every Research session, after reading the cycle objective and
active idea notes but before general grilling, `discover_agent` must:

1. read pending ToDos from the deterministic ToDo query/projection;
2. show concise IDs and summaries in the user's preferred chat language;
3. identify items already implied by the objective;
4. ask which remaining items, if any, enter this cycle;
5. recommend leaving unrelated items out;
6. persist selected items as `adopted` by the active cycle through a
   deterministic CLI operation;
7. include selected items in the requirements handed to Planning.

No new public slash command is added for adoption. A legacy item selected here
is promoted to structured state before adoption. It stays visible as adopted;
it is not removed at selection time.

### 9.4 Adoption completion and release

- An adopted ToDo becomes `resolved` only after its adopting cycle completes
  successfully through all enabled validation stages.
- A cycle completed with deferred ToDos does not validate unresolved adopted
  items merely by closing.
- Cancel, emergency finish without validation, rejection/rollback, or another
  non-validating terminal outcome releases unresolved adopted items back to
  `pending` while preserving adoption history.
- Pending items therefore reappear in the next Research session.

### 9.5 Manual completion

Add the mutating command `/hero-complete-todo <id> [<id>...]`, backed by a
deterministic CLI verb. It must:

- accept structured and legacy ToDos;
- promote a legacy item to `todo-*` before completion;
- accept only `pending` items, never an item adopted by an active cycle;
- require explicit confirmation in the TUI;
- require a non-empty, safe resolution note;
- set state to `resolved` with timestamp and note;
- remove the item from Pending projection and default `/hero-todos` output;
- retain ID, origin, note, and history in SQLite and Events;
- be idempotent for a retry of the same resolved ID and reject conflicting
  attempts clearly.

`/hero-todos` remains read-only. It lists pending and adopted work by default
and retains the existing `/hero-sync` notice.

## 10. Visibility and integrations

### 10.1 Status surfaces

The active cycle's Status screen, `hero status`, `/hero-status`, and `--json`
gain additive blocks containing:

- counts by finding status;
- finding rows: ID, source, owner, status, round, one-line issue;
- short loop-back history: source → Implementation, round, timestamp, IDs;
- deferred ToDo count and IDs;
- completion disposition when present;
- escalation CTAs.

No active cycle keeps the existing Status behavior. Archived findings remain in
SQLite for audit, but C15 does not expand `/hero-cycles`.

### 10.2 Events

Append readable events for finding created, done, reopened, deferred, deferred
recurrence, ToDo adopted, adoption released, manually completed, and cycle
completed with deferred ToDos. Event payloads use IDs and safe metadata rather
than only free-form reasons.

### 10.3 TUI, Runtime, and Telegram

- Add `/hero-add-todo` and `/hero-complete-todo` to canonical slash vocabulary,
  TUI palette/help, embedded Runtime projections, workflow help, and Telegram
  help/forwarding where project control commands are supported.
- `/hero-status` and Telegram status consume the additive JSON, not duplicated
  parsing logic.
- All surfaces use the same deterministic service/CLI rules.
- Do not add an eighth navbar item; enrich Status and Events.
- Images or other attachments are not accepted on these control commands.

### 10.4 Required component impact map

Planning must account for every row below. Exact new filenames may change to
preserve vertical-slice cohesion, but omitting a responsibility is not allowed.

| Component | Required C15 responsibility |
|---|---|
| `internal/store/migrate.go` | Transactional schema v11 migration, constraints, and indexes |
| `internal/store` finding slice | Finding/occurrence persistence, ID allocation, fingerprint uniqueness, status queries |
| `internal/store` ToDo slice | ToDo/adoption/disposition persistence and idempotency keys |
| `internal/engine` | Transaction-aware loop-back, Escalated guards, terminal disposition, adoption release/resolve hooks |
| `internal/cycle/service.go` | Shared orchestration façade for atomic failure, assignment inputs, ToDo mutations, Research adoption |
| `internal/cycle/command.go` | `--findings-json`, deterministic ToDo CLI verbs, JSON-safe errors |
| `internal/tui/implementation_assignment.go` | Merge ordered `task-*` and `find-*`; validate exact report union; apply scheduler results |
| `internal/tui/stage_handoff.go` | Parse validation reports, call atomic handoff, prevent agent-driven transitions |
| `internal/tui/research_session.go` | Query/present/adopt pending ToDos before general grilling |
| `internal/tui/herocmd.go` | Dispatch `/hero-add-todo` and `/hero-complete-todo`; enforce state gates |
| `internal/tui/palette.go`, `slash_overlay.go` | Discoverable command entries and argument help |
| `internal/tui/screens.go`, `output_view.go` | Status finding board, loop-back history, Events, scrolling and focus behavior |
| `internal/status` | Additive table/JSON query contract for findings, ToDos, actions, disposition |
| `internal/todos` | Parse legacy Pending sections; stable projection, promotion, reconcile, removal on resolution |
| `internal/conversation` | Preserve mixed assignment IDs and validated/raw result audit semantics |
| `internal/telegram` and plugin daemon | Help/forwarding/status rendering for both commands and additive JSON |
| `assets/{cursor,opencode,codex,claude}/agents` | Canonical-equivalent contracts for orchestration, discovery, Implementation, and all validators |
| `assets/{cursor,opencode,codex,claude}/commands` | New command assets plus updates to start/status/todos/continue/finish/help |
| installed harness projections | Upgrade-generated copies only; no projection may retain obsolete Judge/gap-file behavior |
| `.workflow-hero/docs/workflow-help.md` source asset | User-facing command, escalation, adoption, and manual-completion documentation |
| install/upgrade checksums | Track new/changed embedded assets under existing customization protection |
| tests and fixtures | Cover every data transition, failure boundary, projection, harness asset, and UI state in TESTING.md |

The SDD must map each OpenSpec task to exactly one implementation agent and
must include cross-component dependency order. In particular, UI/prompt work
cannot precede stable service contracts, and lifecycle rollout cannot be marked
complete before migration and recovery tests pass.

## 11. Compatibility and migration

- Current store schema is v10. C15 introduces a forward-only v11 migration.
- Opening an existing database applies v11 in the normal migration transaction.
- The migration never recreates `hero.db` and preserves cycles, stages, events,
  metrics, conversation records, artifacts, process registries, and model caches.
- Existing rows need no synthetic findings or ToDos.
- Absence of new rows produces empty additive JSON blocks and existing visible
  behavior.
- Runtime asset upgrade refreshes the affected agent/command contracts under the
  established checksum/conflict policy; customized assets are warned, not
  silently overwritten.
- Existing `hero stage loop-back` and `/hero-finish` remain compatible.

## 12. Quality requirements

- Apply `go-engineering` and `golang-tui` during implementation and review.
- Preserve feature-based vertical slices and existing CLI-versus-Runtime
  boundaries.
- No agent reasoning moves into Go; Go validates and transitions structured
  state only.
- Every mutation is tested with real temporary SQLite databases and
  `t.TempDir()` projections.
- Concurrency and retry tests must prove idempotence.
- No test invokes a real harness or external account.
- `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt`,
  `git diff --check`, and strict OpenSpec validation are final gates.

## 13. Out of scope

- Reopening completed OpenSpec checkboxes during loop-back.
- Using markdown gap files as operational truth.
- Allowing agents to mutate finding/ToDo state or stage transitions.
- `/hero-add-todo` before escalation.
- A general-purpose issue tracker or arbitrary ToDo editor.
- A dedicated findings/ToDos navbar screen.
- Semantic/fuzzy finding deduplication.
- Expanding `/hero-cycles` with historical finding drill-down.
- Windows support, CI/CD release automation, unrelated Claude extensions, or
  the general post-1.0 D2–D13 backlog.

## 14. Acceptance criteria

1. Each of the four validation stages can atomically fail with one or more valid
   findings, and Implementation receives the resulting IDs.
2. A failed close with malformed JSON, invalid owner, missing acceptance
   criterion, or no actionable finding commits no partial state.
3. A mixed Implementation wave receives disjoint ordered `task-*` and `find-*`
   assignments by owner.
4. Completing a `find-*` marks it `done`; exact rediscovery reopens the same ID
   and appends a round.
5. An unassigned old `task-*` in a findings-only wave is rejected with an
   `unassigned ID` diagnostic, not a generic gate message.
6. A truly empty assignment accepts only empty completion/remain arrays.
7. Deferred findings are not recreated in the same cycle; recurrences are
   warning occurrences.
8. Partial escalation triage keeps the loop Escalated until `/hero-continue`.
9. Deferring every blocker closes the cycle without another Execute and records
   `completed_with_deferred_todos` everywhere required.
10. Projection failure prevents advancement; retry creates neither duplicate
    database rows nor duplicate markdown lines.
11. Research lists pending ToDos, adopts selected items, and leaves unrelated
    items pending.
12. A validated adopting cycle resolves its ToDos; cancellation or non-validating
    termination returns them to pending with history retained.
13. `/hero-complete-todo` promotes legacy items, requires a note, resolves only
    pending items, rejects adopted items, and cleans the Pending projection.
14. Status JSON additions remain backward-compatible and power TUI/Runtime/
    Telegram views.
15. A v10 fixture migrates to v11 without losing or changing existing records.
16. Canonical agent prompts for every supported harness contain consistent,
    valid examples and forbid agent-side state mutation.
17. The complete test and quality gate in §12 passes.
