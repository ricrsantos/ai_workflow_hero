# ADR-C15-001 — Deterministic Findings, Escalation ToDos, and Research Adoption

> Cycle C15 architecture decisions. Product: [PRD-C15-001](../product/PRD-C15-001-loopback-findings-handoff.md).
> Terminal UX: [UI-C15-001](../product/UI-C15-001-loopback-findings-handoff.md).

| # | Decision | Status |
|---|---|---|
| [ADR-083](#adr-083-findings-are-sqlite-operational-state-owned-by-the-go-scheduler) | Findings are SQLite operational state owned by the Go scheduler | Accepted |
| [ADR-084](#adr-084-failed-stage-close-finding-persistence-and-loop-back-are-one-transaction) | Failed-stage close, finding persistence, and loop-back are one transaction | Accepted |
| [ADR-085](#adr-085-implementation-assignments-union-openspec-tasks-with-actionable-findings) | Implementation assignments union OpenSpec tasks with actionable findings | Accepted |
| [ADR-086](#adr-086-structured-agent-contracts-fail-closed-before-state-mutation) | Structured agent contracts fail closed before state mutation | Accepted |
| [ADR-087](#adr-087-deferred-findings-become-durable-project-todos-with-a-reconciled-file-projection) | Deferred findings become durable project ToDos with a reconciled file projection | Accepted |
| [ADR-088](#adr-088-escalation-triage-may-complete-a-cycle-with-a-deferred-work-disposition) | Escalation triage may complete a cycle with a deferred-work disposition | Accepted |
| [ADR-089](#adr-089-research-adopts-pending-todos-manual-completion-is-pending-only) | Research adopts pending ToDos; manual completion is pending-only | Accepted |
| [ADR-090](#adr-090-existing-status-and-event-surfaces-project-the-active-findings-flow) | Existing Status and Event surfaces project the active findings flow | Accepted |

**Amends:** [ADR-012](ADR-C01-001-hero-1-0.md#adr-012-go-owns-deterministic-ai-loop-state-machine),
[ADR-013](ADR-C01-001-hero-1-0.md#adr-013-sqlite-as-sole-hero-operational-store),
[ADR-015](ADR-C01-001-hero-1-0.md#adr-015-dual-entry-ui-chat-and-tui-parity),
[ADR-024](ADR-C03-001-cursor-harness-tui-autonomy.md#adr-024-hero-slash-vocabulary-uses-hyphen-not-colon),
[ADR-028](ADR-C03-001-cursor-harness-tui-autonomy.md#adr-028-hero-cycles-and-hero-todos-runtime-commands),
and [ADR-075](ADR-C08-001-tui-stage-execute.md#adr-075-implementation-completion-is-gated-by-structured-reports-and-the-openspec-checklist).

## Context

ADR-075 made the linked OpenSpec checklist and structured Implementation report
the completion authority. That prevents accidental closure, but a validation
loop currently carries only prose in a stage summary/event. A fully checked
OpenSpec file therefore yields an empty repair assignment even when QA or Judge
has identified real work. Reusing old task IDs correctly fails the assignment
gate, but the repair itself has no scheduler-owned identity.

Gap markdown files cannot solve this because they are agent-written project
artifacts, not state-machine input. Letting agents call close/loop-back also
creates multiple writers and partial-transition hazards. C15 introduces a
second checklist domain: findings discovered by validation. It also turns the
existing markdown ToDo view into a durable bridge for intentionally deferred
findings without turning Hero into a general issue tracker.

---

## ADR-083: Findings are SQLite operational state owned by the Go scheduler

### Context

Validation findings control whether a stage can close, which agent runs next,
and whether a failure has been resolved. They are Hero-exclusive operational
state under ADR-013. `tasks.md`, by contrast, is the OpenSpec planning artifact
under ADR-007 and must not be rewritten to represent every regression.

### Decision

Add a schema-v11 findings model under `internal/store`, exposed through focused
store/service APIs. The physical SDD may normalize columns further, but the
model must represent:

```text
findings
  id text                         -- find-qa-1 / find-judge-1 / ...
  cycle_id integer
  source_stage text
  owner text
  status text                     -- open | done | reopened | deferred_todo
  fingerprint text
  file text nullable
  requirement text nullable
  issue text
  acceptance_criteria text
  evidence_json text
  round integer
  todo_id text nullable
  created_at / updated_at

finding_occurrences
  finding_id + sequence
  kind                            -- created/done/reopened/rediscovered/deferred
  source_stage + round
  issue / acceptance criteria / evidence snapshot
  created_at
```

Use database constraints for valid enums, positive rounds, non-empty issue and
acceptance criteria, cycle foreign keys, stable ID uniqueness, and one
fingerprint identity per cycle/source/owner/canonical target/criterion.

The Go scheduler/service is the only status writer. Agents may suggest
`reopen_id`; they never allocate arbitrary IDs or mutate records. Optional
read-only markdown evidence may exist, but it cannot drive scheduling.

Deduplication is deterministic, not semantic. A valid `reopen_id` wins only
when the report's canonical file, requirement, and acceptance criteria match
the stored finding contract. Otherwise the decoder rejects `unknown_reopen_id`
and the agent must omit `reopen_id` so a new ID is allocated. Reopen,
rediscovery, done, and deferral never overwrite the stored issue, file,
requirement, or acceptance criterion; issue snapshots belong on
`finding_occurrences`. Implementation assignments always show the frozen
contract plus occurrence history.

Otherwise canonicalize path/requirement and normalize the acceptance criterion
with documented mechanical rules before hashing it with cycle, source, and
owner. Issue prose is retained but not used for fuzzy matching. Exact
rediscovery of `done` yields `reopened`; exact rediscovery of `open`/`reopened`
appends an occurrence; no match creates a new finding. A deferred finding in the
same cycle records later recurrence as a warning and is not reopened.

### Amendment (2026-09-12)

C16 loop-back showed that `reopen_id` without a contract match let QA rewrite
issue and acceptance under the same ID, so Implementation patched a moving
target. The freeze above is mandatory: same ID means same file + requirement +
acceptance. A new residual is a new `find-*` ID.

### Amendment (2026-09-12) — Residual and locked repro tests

Frozen Issue/Acceptance remain identity only. The current defect sentence is
the latest non-done occurrence issue (**Residual**), shown untruncated in the
Implementation assignment together with `repro.source`.

Schema **v14** adds `findings.repro_package`/`repro_test` and
`finding_occurrences.repro_package`/`repro_test`/`repro_source`. Every
validation failure entry must include `repro: {package, test, source}`.
Validators must not Write that test into the project. Implementation lands
the source first (it must fail) and may not claim the ID done until
`go test <package> -count=1 -run ^TestName$` passes. The scheduler re-runs
that command before `MarkFindingDone` (`repro_test_failed` otherwise; a
`go test` exit 0 with no named test run is not a pass).

`reopen_id` additionally requires the same repro package+test. A different
test is a new `find-*` ID. In-flight empty stored repro skips the done-gate
until the first incoming repro locks onto the row (and rewrites the
fingerprint). Fingerprints without repro stay the pre-v14 six-field hash so
legacy rows still match; fingerprints with repro include package+test.

### Consequences

- Repairs have stable IDs independent of OpenSpec task completion.
- Finding history remains queryable after cycle archive.
- Different validators may create distinct findings for the same apparent
  symptom; Hero does not make an unsafe semantic merge.
- Status transitions and fingerprints require migration, store, service, and
  invariant tests.

---

## ADR-084: Failed-stage close, finding persistence, and loop-back are one transaction

### Context

The current flow can close a stage and then separately call
`hero stage loop-back`. Adding a third finding write would permit states such as
"QA failed but no finding exists" or "finding exists but Implementation was not
reopened" after a crash or error.

### Decision

Extend the deterministic stage-close API to accept a structured finding report
when `--failed` is used. The CLI surface is an extension of the existing command:

```text
hero stage close --name <source> --failed --findings-json '<report>'
```

Validation occurs entirely before the transaction. The transaction then:

1. creates/appends/reopens every finding;
2. marks the source stage failed;
3. invokes the engine's loop-back transition semantics for Implementation and
   affected downstream stages;
4. stores the resulting IDs in the loop-back event and Implementation summary;
5. appends the validated raw-result audit.

The transaction commits all or none. A failed validation close without at
least one actionable finding is illegal. A successful report uses normal stage
close and an empty failure array. SDD must factor engine logic so the CLI does
not perform a nested transaction.

Keep `hero stage loop-back` for compatibility and explicit administrative use,
but generated TUI/Runtime agent flow uses the atomic close path. The standalone
command cannot create anonymous repair work in the new automated path.

### Consequences

- Store and engine boundaries require a transaction-aware operation.
- Loop-back events carry IDs instead of only a prose reason.
- Invalid reports cannot leave partial findings or transitions.
- CLI remains deterministic and contains no LLM reasoning, preserving ADR-003.

---

## ADR-085: Implementation assignments union OpenSpec tasks with actionable findings

### Context

ADR-075 treats unchecked OpenSpec tasks as the whole Implementation assignment.
That is correct for planned work but insufficient after completed work regresses.
Unchecking old task boxes would destroy planning history and make repeated
repairs indistinguishable.

### Decision

Amend ADR-075. The Implementation assignment is an ordered, duplicate-free
union per owner:

```text
unchecked owned task-* IDs
  + open/reopened owned find-* IDs
```

OpenSpec remains authoritative only for `task-*`; SQLite findings are
authoritative only for `find-*`. Both appear in `conversation.task_ids` so the
existing assignment/result audit remains the shared envelope.

The Implementation report contract continues to require disjoint
`tasks_completed` and `tasks_remaining`, despite the historical field name.
Their set union must equal the exact assignment, including both prefixes. Only
the scheduler checks task boxes or marks findings done after report, ownership,
acceptance-gate, and test verification.

Assignment order is stable: OpenSpec order first, then finding creation/order.
An owner must be active in cycle scope. Cross-agent ownership remains disjoint.
A partial valid report may advance only its completed IDs and trigger another
bounded productive wave. Invalid reports apply no subset.

Empty behavior is explicit:

- Implementation starting/restarting with no task and no finding may run the
  one verification wave already allowed by ADR-075;
- that wave must return empty completed/remaining arrays;
- after a productive wave or user deferral empties all work, scheduler rereads
  both authorities and closes directly without dispatching an empty wave.

### Consequences

- C13/C14's "checked tasks but real gap" state becomes representable.
- Old task IDs in a finding-only wave fail as unassigned.
- Existing column names can remain for compatibility, but code/docs must define
  them as generic assignment item IDs.
- Checklist and finding state must be reread together at every completion gate.

---

## ADR-086: Structured agent contracts fail closed before state mutation

### Context

A generic "missing gate fields" message hid the actual C14 error. Existing
agent projections also disagree: some Judge prompts still write gap files and
call loop-back. Distributed, prose-only contract knowledge invites recurrence.

### Decision

Define typed Go decoders/validators for QA, Judge, Browser UI, QA End-to-End,
and Implementation output. Reject unknown/invalid status and enum values,
missing required fields, invalid owners, invalid `reopen_id`, duplicate IDs,
overlapping completed/remaining arrays, assignment union mismatch, false gates,
and non-empty IDs on an empty assignment. Validation returns a stable diagnostic
code plus field path and concise message.

No finding, task, stage, event, or conversation-result mutation occurs until the
entire report passes the relevant validation phase. Raw invalid output may be
kept only in the existing safe audit channel and must not be interpreted as
state.

Update the single canonical embedded source for every affected agent and then
project it to Cursor, OpenCode, Codex, and Claude. Each prompt includes required
fields, enums, ownership rules, one valid creation example, one reopening
example where applicable, empty-success behavior, and explicit prohibitions on
checkbox/gap-file/current-state/stage mutation. Judge emits JSON and stops;
SDD ambiguity remains separate from implementation findings.

### Consequences

- Error copy can say `tasks_completed[0]: unassigned ID task-03.2` instead of
  blaming missing gates.
- Prompt goldens and projection parity tests become mandatory.
- New optional fields require an explicit backward-compatible contract change.

---

## ADR-087: Deferred findings become durable project ToDos with a reconciled file projection

### Context

ADR-028 defines `/hero-todos` as a read-only view of Pending sections in
`context/current-state.md`. This is useful outside Hero, but markdown alone
cannot safely track adoption, release, manual resolution, or source finding.
SQLite alone would violate the project's portable-context goal.

### Decision

Add structured ToDo and adoption-history state in schema v11:

```text
todos
  id text                         -- finding ID or project-scoped todo-N
  origin_type text                -- finding | legacy
  origin_finding_id nullable
  origin_cycle_id/source_stage nullable
  summary / acceptance_criteria
  status text                     -- pending | adopted | resolved
  adopted_cycle_id nullable
  resolution_note nullable
  created_at / updated_at / resolved_at nullable

todo_adoptions
  todo_id + sequence
  cycle_id
  status                          -- adopted | released | resolved
  note nullable
  created_at
```

A finding-derived ToDo retains its `find-*` ID. A markdown-only legacy entry is
promoted to a project-scoped `todo-*` ID when first adopted or manually
completed. Promotion uses exact selected line identity plus normalized content;
it does not silently import every Pending entry.

SQLite is authoritative for structured lifecycle. `context/current-state.md`
remains the durable project-facing projection for pending/adopted work. Use a
stable machine-recognizable ID in each projected line while keeping the line
readable in non-Hero tools.

SQLite and a file cannot share a real transaction. Therefore `hero add-todo`
uses a recoverable, idempotent reconciliation protocol: build and fsync an
atomic replacement candidate, persist transition/projection intent, install
and verify the projection, then authorize stage/cycle advancement. A retry
repairs either side without duplicate rows or lines. The SDD selects the exact
ordering and recovery marker, but a projection failure leaves the cycle
Escalated and visible as incomplete.

`/hero-todos` remains read-only and retains the `/hero-sync` notice. ADR-028 is
amended only to add structured pending/adopted rows to its source/view; it does
not become a tracker editor.

### Consequences

- Deferred work survives archive in portable project context.
- Operational lifecycle and audit remain deterministic.
- File reconciliation adds explicit crash/retry states and tests.
- Non-Hero edits remain possible; sync/promotion must detect ambiguity rather
  than overwrite user prose.

---

## ADR-088: Escalation triage may complete a cycle with a deferred-work disposition

### Context

At iteration exhaustion, `/hero-continue` is the only constructive option.
Users sometimes intentionally accept known work for a later cycle. Emergency
`/hero-finish` closes without creating durable ToDos and should not masquerade
as that controlled decision.

### Decision

Add `/hero-add-todo` to canonical slash vocabulary and back it with a
deterministic `hero add-todo` operation. It is legal only for open/reopened
findings in the currently Escalated loop.

For a partial selection, defer selected findings, create pending ToDos, and
keep the stage Escalated. `/hero-continue [N]` remains a separate decision for
the work retained in-cycle.

If the confirmed operation leaves zero unchecked OpenSpec tasks and zero open/
reopened findings, do not dispatch verification or later validation. Complete
the cycle immediately with its existing `completed` status plus a structured
`completed_with_deferred_todos` disposition and IDs. This terminal operation is
allowed only after ToDo projection reconciliation succeeds.

`/hero-finish` remains an emergency exit. When findings are open it requires a
strong warning/confirmation that they will not be converted to ToDos.

### Consequences

- Users explicitly divide "resolve now" from "defer".
- Existing completed-cycle queries remain compatible.
- Consumers that care about quality disposition can distinguish ordinary
  completion from deferred-work completion.
- Downstream skipped stages are explained, not shown as successful validation.

---

## ADR-089: Research adopts pending ToDos; manual completion is pending-only

### Context

Deferral is useful only if later cycles reliably surface it. Some work may also
be fixed outside Hero and must not remain pending forever. Removing an item as
soon as a cycle selects it risks losing work when that cycle is cancelled.

### Decision

Amend the Research startup protocol. After objective/configuration and active
idea-note loading, but before general grilling, `discover_agent` queries pending
ToDos, presents IDs/summaries in the user's preferred language, identifies any
already implied by the objective, and asks which other items enter scope.
Unrelated items are recommended out. Selection is conversational; a
deterministic internal CLI operation records adoption. No public
`/hero-adopt-todo` command is added.

Selected items become `adopted` and remain in the projection with the adopting
cycle ID. They resolve only when that cycle completes successfully through all
enabled validation stages. Cancellation, rollback/rejection, emergency finish,
or another non-validating terminal outcome releases unresolved items to
`pending` and retains the attempt history. A cycle that closes with newly
deferred findings does not automatically validate adopted items.

Add `/hero-complete-todo <id>...` plus deterministic CLI backing for work fixed
outside Hero. It accepts only `pending` items, requires a non-empty safe
resolution note and explicit TUI confirmation, promotes a selected legacy line
when necessary, marks it resolved, removes it from Pending/default
`/hero-todos`, and keeps its audit history. An item adopted by an active cycle
cannot be manually completed.

### Consequences

- Deferred work is proactively reconsidered at the correct stage.
- Adoption cannot make work disappear on cycle failure.
- External resolution has an auditable path without a synthetic cycle.
- Research prompts, cycle terminal transitions, Runtime commands, and context
  projection all require coordinated changes.

---

## ADR-090: Existing Status and Event surfaces project the active findings flow

### Context

Users need to understand validation ping-pong, but a new navbar screen would
fragment cycle state. Events already provide raw audit and Status already owns
the current stage machine. Telegram and Runtime need the same truth.

### Decision

Extend the active-cycle status query with additive structured blocks for finding
counts/rows, short loop-back history, deferred ToDos, escalation actions, and
completion disposition. `hero status --json` is the shared contract used by TUI,
Runtime `/hero-status`, and Telegram. Existing fields remain unchanged; no rows
produce empty additions.

Enrich the existing Status screen and Events screen. Add readable events for
finding and ToDo lifecycle changes. Do not add another navbar item and do not
expand `/hero-cycles` historical UI in C15. Archived rows remain queryable from
SQLite for audit.

Add `/hero-add-todo` and `/hero-complete-todo` to TUI palette, help, installed
Runtime assets, workflow help, and Telegram's supported project-control command
surface. These control commands reject multimodal attachments.

### Consequences

- All UI surfaces share one deterministic query contract.
- JSON clients gain fields additively.
- Historical drill-down remains future work.

---

## Migration and rollout constraints

Schema v11 is a forward-only transactional migration from current schema v10.
It adds constrained tables/indexes without recreating the database or rewriting
existing operational rows. Empty tables mean old projects retain their visible
behavior. Migration fixtures must prove cycles, stages, events, metrics,
conversation, artifacts, process registries, and model caches remain intact.

The feature must ship as one integrated product increment. Internal tasks may
stage store, service, scheduler, prompt, TUI, Runtime, and Telegram work, but no
release is complete while findings can be persisted without being assigned or
while ToDos can be deferred without recovery/projection/adoption support.

## Rejected alternatives

### Reopen OpenSpec checkboxes

Rejected because planned completion history and regression history are distinct
domains. It also cannot express repeated rounds cleanly.

### Keep `qa-gaps.md` / `judge-gaps.md` as the scheduler input

Rejected because agent-written prose is not transactional, typed, or safe as
the state-machine authority.

### Let validation agents call loop-back

Rejected because it creates multiple transition writers and conflicts with
TUI-direct handoff.

### Fuzzy semantic deduplication

Rejected because model-generated wording is unstable and unsafe for automated
identity. Explicit `reopen_id` plus deterministic fingerprints are testable.

### SQLite-only ToDos

Rejected because project backlog context must remain useful outside Hero.

### Markdown-only ToDos

Rejected because adoption, release, manual completion, and crash recovery need
structured lifecycle and audit.

### Automatically continue after partial deferral

Rejected because choosing what to defer and granting extra iterations are two
separate human decisions.

### Use a new navbar screen

Rejected because Status and Events already own current state and audit.
