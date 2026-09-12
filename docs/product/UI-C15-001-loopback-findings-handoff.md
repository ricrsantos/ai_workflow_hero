# UI-C15-001 — Loop-back Findings and Deferred ToDos UX

> Cycle C15 terminal UX. Product: [PRD-C15-001](PRD-C15-001-loopback-findings-handoff.md).
> Architecture: [ADR-C15-001](../architecture/ADR-C15-001-loopback-findings-handoff.md).

## 1. Scope

This specification extends existing Chat, Status, Events, palette, Runtime, and
Telegram surfaces. It covers:

- visible finding handoff from validation to Implementation;
- exact completion-gate errors;
- Escalated triage through `/hero-add-todo`;
- controlled completion with deferred ToDos;
- Research startup ToDo adoption;
- manual pending-ToDo completion;
- additive status JSON consumed by non-terminal surfaces.

It does not add a navbar item, a general ToDo editor, or historical drill-down
to `/hero-cycles`.

## 2. Language, style, and accessibility

- Chat and prompts use `workflow_config.user_preferred_language`; examples here
  are English because cycle artifacts are English.
- IDs, enum values, command names, and file paths are never translated.
- Status colors supplement, never replace, explicit text labels.
- Long issue text truncates with an ellipsis in tables and is available in the
  focused detail/Chat output.
- Every dialog is fully keyboard operable. Focus is visible.
- Destructive or terminal choices require explicit confirmation; Escape cancels
  without mutation.
- Existing Hero arrow conventions apply: `→` in progress, `✓` completed,
  `⚠` warning/escalation, `✗` rejected/failed.

## 3. Validation failure handoff in Chat

After a valid failed report is atomically persisted, Chat shows the source,
created/reopened IDs, and next routing. It does not paste the entire report by
default.

```text
✗ QA failed · 2 findings
  find-qa-1 · GEN · reopened (round 2)
  internal/tui/stage_handoff.go · assignment omitted open findings
  find-qa-2 · GEN · open
  internal/store/migrate.go · v10 migration fixture loses event rows

→ Loop-back QA → Implementation
→ Assignment will include find-qa-1, find-qa-2
```

For Browser UI, the row also identifies Health or Visual evidence where safe:

```text
find-bui-1 · FRNT · open · browser-ui/health-report.md
```

Missing visual reference PNGs render as warnings and do not create IDs:

```text
⚠ Visual reference missing; no finding created
```

## 4. Implementation assignment display

Each TUI-direct Implementation Execute receives and displays only its owned
ordered IDs. Mixed assignments visibly distinguish the two namespaces:

```text
→ Implementation wave 3 · GEN
  planned: task-04.1, task-04.2
  findings: find-qa-1, find-judge-2
```

When the assignment was empty at stage start/restart, the one permitted
verification wave says:

```text
→ Implementation verification · no assigned task or finding IDs
  Required report: tasks_completed=[] · tasks_remaining=[]
```

After a productive wave or user deferral removes the last work item, Chat shows
direct closure rather than launching another empty Execute:

```text
✓ Implementation assignment empty after scheduler recheck
→ Closing Implementation without another wave
```

## 5. Exact completion-gate diagnostics

The TUI and Cursor Runtime must display the stable diagnostic code, JSON field
path when applicable, offending ID/value, and expected rule. Generic fallback
copy is used only for an unexpected internal error.

Examples:

```text
✗ implementation report rejected · unassigned_id
  tasks_completed[0]: task-03.2 was not assigned in wave 4
  assigned IDs: find-judge-1
```

```text
✗ QA report rejected · missing_field
  failures[0].acceptance_criteria is required
  No finding, stage close, or loop-back was persisted.
```

```text
✗ implementation report rejected · assignment_union_mismatch
  missing from completed/remaining: find-qa-2
  duplicated across arrays: none
```

```text
✗ implementation report rejected · nonempty_empty_assignment
  This verification wave assigned no IDs.
  Return tasks_completed=[] and tasks_remaining=[].
```

Other required diagnostics include `invalid_json`, `invalid_enum`,
`invalid_owner`, `unknown_reopen_id`, `duplicate_id`, `overlapping_arrays`,
`false_acceptance_gate`, and `no_actionable_finding`.

On rejection:

- status remains unchanged;
- no partial result appears as applied;
- the message states that nothing was persisted;
- the available retry/control action follows current stage state.

## 6. Status screen

### 6.1 Active cycle layout

Keep the existing stage table first. Add compact sections below it in this
order: loop-backs, findings, deferred ToDos, disposition/CTAs.

```text
Stages
Research        Completed   1/3
Planning        Completed   1/3
Implementation  Escalated   4/4
QA              Failed      2/2
Judge           Waiting     1/3

Loop-backs
QA → Implementation · round 2 · 14:32 · find-qa-1, find-qa-2
Judge → Implementation · round 1 · 13:58 · find-judge-1

Findings  open 1 · reopened 1 · done 2 · ToDo 1
ID             Source  Owner  State      R  Issue
find-qa-1      QA      GEN    reopened   2  assignment omitted open findings
find-qa-2      QA      GEN    open       1  migration loses event rows
find-judge-1   Judge   GEN    done       1  status JSON lacked finding counts
find-qa-3      QA      GEN    ToDo       1  manual fixture cleanup

Escalated
/hero-continue [N]   grant more iterations
/hero-add-todo       choose findings to defer
/hero-cancel         cancel and roll back
/hero-finish         emergency finish without creating ToDos
```

- `R` is current finding round.
- Owner uses existing TUI labels (`BACK`, `FRNT`, `GEN`).
- `deferred_todo` renders as `ToDo` in the compact table and remains explicit
  as `deferred_todo` in JSON/detail output.
- The finding table scrolls inside the existing content viewport.
- Focused rows may reveal full issue, acceptance criterion, occurrences, and
  evidence beneath the table without creating a new screen.

### 6.2 Completion disposition

For the active cycle while the terminal completion is being rendered, or in
the final completion response:

```text
✓ Cycle C15 completed with deferred ToDos
  disposition: completed_with_deferred_todos
  deferred: 2 · find-qa-1, find-judge-2
  skipped after escalation: QA, Judge
```

Downstream stages skipped by this terminal action must not appear as successful
validation. Use their existing skipped/not-run representation and the explicit
reason `cycle completed with deferred ToDos`.

### 6.3 No rows

Do not render empty tables noisily. A cycle with no findings may show:

```text
Findings  none
```

No active cycle retains the existing Status screen behavior.

## 7. `/hero-add-todo` TUI flow

### 7.1 Entry conditions

The command is enabled only for an Escalated loop with at least one open or
reopened finding. Otherwise Chat shows one precise reason:

```text
✗ /hero-add-todo is available only for open findings in an Escalated loop.
```

Arguments supplied in Chat may preselect valid IDs, but the confirmation view
still appears. Unknown, done, already deferred, or foreign-cycle IDs block the
operation and mutate nothing.

### 7.2 Selection dialog

No item is selected by default when the user invokes the command without IDs.

```text
Defer findings to project ToDos

[ ] find-qa-1 · GEN · assignment omitted open findings
[ ] find-qa-2 · GEN · migration loses event rows
[ ] find-judge-1 · GEN · status JSON lacks finding counts

space toggle   a select all   n select none   enter review   esc cancel
```

- `a` selects all; it is never applied until review and confirmation.
- Rows include ID, owner, and one-line issue.
- Selection preserves visible order from the scheduler.
- Enter with nothing selected leaves the dialog open and shows
  `Select at least one finding or press Esc to cancel.`

### 7.3 Review for partial selection

```text
Review escalation decision

Move to ToDos (1)
  find-qa-2 · migration loses event rows

Resolve in this cycle (2)
  find-qa-1, find-judge-1

The stage will remain Escalated.
Run /hero-continue after this operation to grant more iterations.

enter confirm   esc back
```

After success:

```text
✓ Added 1 finding to project ToDos: find-qa-2
⚠ Implementation remains Escalated with 2 open findings.
  Use /hero-continue [N], /hero-add-todo, /hero-cancel, or /hero-finish.
```

### 7.4 Review when every blocker is selected

This is a terminal confirmation and must state skipped work:

```text
Complete cycle with deferred ToDos?

Move to ToDos (3)
  find-qa-1, find-qa-2, find-judge-1

No OpenSpec tasks or other findings will remain.
The cycle will close now. No empty Implementation wave will run.
Remaining enabled validation stages will not run.

type DEFER to confirm   esc cancel
```

Use the existing terminal-confirmation convention if it is stricter than typed
`DEFER`; the confirmation must not be a single accidental keypress.

While reconciling SQLite and `current-state.md`, show a non-blocking progress
line. A failure keeps the cycle Escalated:

```text
✗ ToDo projection was not completed; the cycle remains Escalated.
  No duplicate was created. Run /hero-add-todo again to retry reconciliation.
```

After success, show the completion disposition from §6.2 and normal stage/cycle
metrics. Do not suggest starting another stage.

## 8. `/hero-finish` with open findings

Retain emergency finish but add a strong warning:

```text
Emergency finish with 2 open findings?

find-qa-1 and find-judge-1 will NOT be added to project ToDos.
Use /hero-add-todo if you want to preserve them as deferred work.

type FINISH to confirm   esc cancel
```

This flow must not use the `completed_with_deferred_todos` disposition.

## 9. Research startup ToDo adoption

### 9.1 Ordering

After the cycle objective/config and active idea notes are loaded, but before
ordinary requirements grilling, Research emits:

```text
→ Checking pending project ToDos
```

If none exist:

```text
✓ No pending project ToDos
```

Research then continues without an unnecessary question.

### 9.2 Candidate presentation

When items exist, `discover_agent` presents a concise list in the configured
chat language. It identifies objective matches separately:

```text
Pending project ToDos

Already implied by this cycle objective
  find-qa-1 · deterministic loop-back assignment

Other candidates
  todo-4 · Windows CLI support
  todo-5 · CI/CD release automation

Do you want to add any other pending ToDo to this cycle?
Recommendation: leave unrelated items out to keep the cycle focused.
```

The grilling protocol remains one focused question at a time. Selection is
conversational, not a new slash command.

### 9.3 Adoption result

After the user confirms selection and deterministic persistence succeeds:

```text
✓ Adopted by C16: find-qa-1
→ The item remains visible until this cycle validates it.
```

Legacy items are shown with their assigned `todo-*` ID after promotion.
Persistence failure blocks requirement finalization and provides a retry;
Research must not claim adoption that is absent from state.

## 10. `/hero-todos` output

The default remains read-only and shows pending plus adopted items:

```text
Project ToDos

Pending
  find-qa-2 · C15/QA · migration loses event rows
  todo-4 · legacy · Windows CLI support

Adopted
  find-qa-1 · adopted by C16 · deterministic loop-back assignment

2 pending · 1 adopted

⚠ If product or architecture docs changed, run /hero-sync then /hero-todos.
```

Resolved items are omitted from the default list. `/hero-todos` never changes
state and never silently adopts an item.

## 11. `/hero-complete-todo` flow

### 11.1 Eligibility

The command accepts one or more explicit pending IDs. Unknown or already
resolved IDs produce idempotent/specific feedback. Adopted items are rejected:

```text
✗ find-qa-1 is adopted by active cycle C16.
  Complete that cycle's validation or cancel it to return the ToDo to pending.
```

A selected legacy line is promoted and its new `todo-*` ID is shown before
confirmation.

### 11.2 Resolution note and confirmation

```text
Complete project ToDos manually

Selected
  find-qa-2 · migration loses event rows

Resolution note (required):
> Fixed in maintenance commit abc123; migration fixture now passes.

This records resolution outside a Hero validation cycle.
enter review   esc cancel
```

Review:

```text
Mark 1 ToDo resolved?
  find-qa-2
  note: Fixed in maintenance commit abc123; migration fixture now passes.

enter confirm   esc back
```

Empty/whitespace-only notes are rejected. Secret-like content receives the
existing redaction warning and is not persisted until corrected.

Success:

```text
✓ ToDo resolved: find-qa-2
  Removed from Pending; audit retained in Events.
```

The command is retry-idempotent: retrying an already resolved ID reports its
resolution time and does not add a second event or projection edit. A conflicting
new note is rejected rather than overwriting history.

## 12. Events screen

Render lifecycle events as readable rows with IDs before optional detail:

```text
14:32  finding_reopened       find-qa-1 · QA · round 2
14:31  finding_done           find-qa-1 · Implementation
14:02  finding_created        find-qa-1 · QA → GEN
13:58  todo_deferred          find-judge-1 · from C15/Judge
13:57  todo_adopted           todo-4 · C16
13:40  todo_completed_manual  find-qa-2 · outside cycle
```

The raw JSON/detail view includes safe metadata, not secrets or file contents.
Existing scrolling and viewport behavior remain unchanged.

## 13. Runtime and Telegram copy

- Cursor Runtime uses the same command names and decision language but may use
  text lists instead of Bubble Tea dialogs.
- `/hero-add-todo <id>...` in Cursor must repeat the partial-versus-terminal
  consequence and wait for explicit user confirmation before its CLI mutation.
- `/hero-complete-todo <id>...` asks for the required note and confirmation.
- Telegram help lists both commands. Telegram forwards them only to a selected,
  connected project TUI under existing addressing/auth rules.
- Telegram status renders compact finding counts and the first actionable IDs;
  detailed rows remain available through project Status/JSON.
- No command accepts an image attachment.

## 14. Status JSON contract

The existing JSON object gains additive fields conceptually equivalent to:

```json
{
  "workflow": {
    "findings": {
      "counts": {"open": 1, "reopened": 1, "done": 2, "deferred_todo": 1},
      "items": [
        {
          "id": "find-qa-1",
          "sourceStage": "qa",
          "owner": "generic_agent",
          "status": "reopened",
          "round": 2,
          "issue": "assignment omitted open findings"
        }
      ]
    },
    "loopBacks": [
      {
        "from": "qa",
        "to": "implementation",
        "round": 2,
        "findingIds": ["find-qa-1"],
        "occurredAt": "2026-09-11T17:32:00Z"
      }
    ],
    "todos": {"pending": 1, "adopted": 0, "deferredFromCycle": 1},
    "completionDisposition": null,
    "availableActions": ["hero-continue", "hero-add-todo", "hero-cancel", "hero-finish"]
  }
}
```

Exact Go field casing follows existing status JSON conventions. Missing data is
represented by empty arrays/counts or null disposition, never by removing or
renaming existing fields.

## 15. UX acceptance matrix

| Scenario | Required visible outcome |
|---|---|
| QA creates two findings | IDs, owners, issues, and loop-back route shown |
| Judge reopens a finding | Same ID, incremented round, `reopened` shown |
| Report has old task ID | Exact `unassigned_id` error and assigned IDs |
| Empty verification report is correct | Empty arrays accepted |
| Empty assignment reports an ID | `nonempty_empty_assignment`; nothing applied |
| Partial ToDo triage | Deferred IDs shown; stage remains Escalated; `/hero-continue` CTA |
| All blockers deferred | Strong confirmation; no new Execute; disposition and skipped stages shown |
| Projection write fails | Error, safe retry, cycle remains Escalated |
| Research has ToDos | Objective match + other candidates shown before grilling |
| Research adopts legacy item | `todo-*` ID and adopting cycle shown |
| Adopted item manually completed | Rejected with active cycle ID |
| Pending item manually completed | Required note; item leaves Pending; audit retained |
| No findings exist | Existing Status remains clean with `Findings none` at most |

## 16. Non-goals

- Mouse-only interaction.
- Editing finding text, owner, or acceptance criterion in Status.
- Bulk completion without a resolution note.
- Resolving an adopted item outside its active cycle.
- A separate findings/ToDos navigation destination.
- Showing every archived finding in `/hero-cycles`.
