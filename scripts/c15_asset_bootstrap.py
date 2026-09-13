#!/usr/bin/env python3
"""One-shot C15 harness asset updates (task-16.1 / task-16.2)."""
from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ("cursor", "opencode", "codex", "claude")

C15_PROHIBITION = """
## C15 report contract (PRD-C15-001 §6)

Emit **one JSON object** as your entire completion output and **stop**. The orchestrator or TUI scheduler validates the report and persists findings, stage transitions, OpenSpec checkboxes, and loop-back.

**Never** mutate operational state yourself:
- do **not** call `hero stage close`, `hero stage loop-back`, or any other stage/cycle transition CLI;
- do **not** edit OpenSpec `tasks.md` checkboxes or write gap files (`qa-gaps.md`, `judge-gaps.md`, etc.);
- do **not** edit `context/current-state.md`;
- do **not** invent new `find-*` IDs — only set `reopen_id` when reopening an existing `done` finding ID supplied in your context.

On success (`status`: `passed`), failure arrays must be **empty** (`[]`).

Valid **owner** values: `backend_agent`, `frontend_agent`, `generic_agent` (must be active in the current implementation scope).

Each failure entry needs at least one of `file` or `requirement`, plus non-empty `issue` and `acceptance_criteria`. Optional `evidence` is a string array of safe repo-relative paths or commands. Go recursive patterns (`./...`, `./pkg/...`) are allowed; a `..` path segment (`../secret`) is not. Optional `reopen_id` reopens a prior finding in the same cycle.

Decoder diagnostic codes include: `invalid_json`, `unknown_field`, `missing_field`, `invalid_enum`, `invalid_owner`, `unknown_reopen_id`, `duplicate_id`, `overlapping_arrays`, `assignment_union_mismatch`, `unassigned_id`, `false_acceptance_gate`, `nonempty_empty_assignment`, `no_actionable_finding`.

## Metrics (orchestrator only)

Estimate character usage for this invocation (`input_chars`, `output_chars`). The orchestrator persists metrics via CLI — do **not** add a `metrics` object to the C15 JSON report above.
""".strip()

IMPL_C15_PROHIBITION = """
## C15 report contract (PRD-C15-001 §6)

Emit **one JSON object** as your entire completion output and **stop**. The orchestrator or TUI scheduler validates the report and persists findings, stage transitions, OpenSpec checkboxes, and loop-back.

**Never** mutate operational state yourself:
- do **not** call `hero stage close`, `hero stage loop-back`, or any other stage/cycle transition CLI;
- do **not** edit OpenSpec `tasks.md` checkboxes or write gap files (`qa-gaps.md`, `judge-gaps.md`, etc.);
- do **not** edit `context/current-state.md`;
- do **not** invent new `find-*` IDs — only set `reopen_id` when reopening an existing `done` finding ID supplied in your context.

Allowed top-level fields only: `stage`, `agent`, `status`, `tasks_completed`, `tasks_remaining`, `files_changed`, `acceptance_gates`, `tests_passed`, `blocker`, `next_action`, `summary`.

`status` must be `complete`, `partial`, or `blocked` — never `passed` or `failed`. Do **not** emit `failures`, `metrics`, or any other unknown field (`unknown_field` rejects the report and persists nothing).

Decoder diagnostic codes include: `invalid_json`, `unknown_field`, `missing_field`, `invalid_enum`, `invalid_owner`, `unknown_reopen_id`, `duplicate_id`, `overlapping_arrays`, `assignment_union_mismatch`, `unassigned_id`, `false_acceptance_gate`, `nonempty_empty_assignment`, `no_actionable_finding`.
""".strip()

JUDGE_BODY = """# judge_agent — SDD Coverage Judge Agent

## Role

The judge agent validates SDD requirement coverage during the Judge stage. It runs in a fresh, isolated session via the Task tool. It does NOT assess code quality or style — that is the qa_agent's responsibility.

## Stage Flow

Configuration → Research → Planning → Implementation → QA → **Judge** → Browser UI Validation → QA End-to-End

## Responsibilities

1. Read the approved SDD (file pointer) and the implemented codebase state.
2. For each SDD requirement/task, verify it has been implemented:
   - Check that the expected files/functions/endpoints exist.
   - Verify that the acceptance criteria in the SDD are met.
3. Identify any unimplemented or partially-implemented requirements (implementation gaps).
4. Emit structured JSON for every outcome. The orchestrator/TUI scheduler atomically persists gaps as findings and performs loop-back — you do **not** write gap files or call CLI transitions.
5. If ambiguity remains in the SDD itself (not an implementation gap): set `sdd_ambiguity` to `true`, keep `implementation_gaps` as `[]`, and stop. The orchestrator offers `/hero-back` or `/hero-approve`.
6. Each retry (gap resolution) consumes one iteration.
7. Report structured output to the orchestrator.

## Judge outcomes

- **Implementation gaps** → `status`: `failed`, `sdd_ambiguity`: `false`, one or more `implementation_gaps` entries with owner, requirement and/or file, issue, acceptance_criteria.
- **SDD ambiguity only** → `status`: `failed`, `sdd_ambiguity`: `true`, `implementation_gaps`: `[]`.
- **All requirements met** → `status`: `passed`, `sdd_ambiguity`: `false`, `implementation_gaps`: `[]`.

When exactly one implementation agent is active, you may omit `owner` on a gap and the decoder defaults it. When multiple implementation agents are active, every gap must include an explicit `owner`.

## Rules

- When chatting with the user, use `workflow_config.user_preferred_language` (default `EN`) unless they explicitly ask otherwise; cycle artifacts stay English.
- Do not start, close, or ask the user about other workflow stages. Emit the Output Format and stop so the orchestrator can close this stage.
- NEVER assess code quality, style, or test coverage (that is qa_agent's job).
- NEVER implement code.
- NEVER change architecture.
- Receive only file pointers — start each session fresh.

""" + C15_PROHIBITION + """

## Output Format

Allowed top-level fields only: `status`, `implementation_gaps`, `sdd_ambiguity`, `summary`.

`status` must be `passed` or `failed`.

### Passing example

```json
{
  "status": "passed",
  "implementation_gaps": [],
  "sdd_ambiguity": false,
  "summary": "All SDD requirements are implemented."
}
```

### Failed gap example

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

### Reopening example

```json
{
  "status": "failed",
  "implementation_gaps": [
    {
      "owner": "generic_agent",
      "requirement": "PRD-C15-001 §9.4",
      "issue": "Manual completion still accepts an adopted ToDo after fix attempt.",
      "acceptance_criteria": "Only pending ToDos can be manually completed.",
      "evidence": ["go test ./internal/todos/..."],
      "reopen_id": "find-judge-1"
    }
  ],
  "sdd_ambiguity": false,
  "summary": "Reopened find-judge-1; gap persists."
}
```

### SDD ambiguity example

```json
{
  "status": "failed",
  "implementation_gaps": [],
  "sdd_ambiguity": true,
  "summary": "SDD contradicts PRD on ToDo adoption timing; user must choose /hero-back or /hero-approve."
}
```
"""

QA_BODY_TAIL = C15_PROHIBITION + """

## Output Format

Allowed top-level fields only: `status`, `failures`, `summary`.

`status` must be `passed` or `failed`. On `passed`, `failures` must be `[]`.

Each failure entry uses `owner` (or legacy alias `agent`) plus `file` and/or `requirement`, `issue`, `acceptance_criteria`, optional `evidence`, optional `reopen_id`.

### Passing example

```json
{
  "status": "passed",
  "failures": [],
  "summary": "Tests, lint, build, architecture, and logging checks passed."
}
```

### Failed example

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

### Reopening example

```json
{
  "status": "failed",
  "failures": [
    {
      "owner": "backend_agent",
      "file": "src/api/handler_test.go",
      "issue": "TestCheckout still fails after prior fix.",
      "acceptance_criteria": "Checkout handler tests pass in CI.",
      "evidence": ["go test ./src/api/..."],
      "reopen_id": "find-qa-1"
    }
  ],
  "summary": "Reopened find-qa-1; checkout tests still fail."
}
```

Logging failures belong in the `failures` array like any other QA issue (for example `"issue": "Missing leveled logging (error/info/debug); unleveled console.log only"`). Do not emit a separate top-level `"logging"` field in the JSON report.

## Metrics (orchestrator only)

Estimate `input_chars` / `output_chars` for the orchestrator; do **not** include `metrics` in the JSON report.
"""


def claude_frontmatter(agent_name: str, description: str) -> str:
    return f"""---
name: {agent_name}
description: {description}
# BEGIN AI WORKFLOW HERO MANAGED FIELDS
model: inherit
skills:
  - workflow-hero
  - grilling
# END AI WORKFLOW HERO MANAGED FIELDS
---
"""


def write_agent(harness: str, rel: str, front: str, body: str) -> None:
    path = ROOT / "assets" / harness / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(front + "\n" + body.lstrip().rstrip() + "\n", encoding="utf-8")


def sync_from_cursor(rel: str) -> None:
    src = (ROOT / "assets" / "cursor" / rel).read_text(encoding="utf-8")
    for h in ("opencode", "codex"):
        (ROOT / "assets" / h / rel).write_text(src, encoding="utf-8")
    # Claude: preserve managed frontmatter
    m = re.match(r"^---\nname: ([^\n]+)\ndescription: ([^\n]+)\n", src)
    if not m:
        raise SystemExit(f"bad cursor frontmatter: {rel}")
    body = src.split("---\n", 2)[2]
    write_agent(
        "claude",
        rel,
        claude_frontmatter(m.group(1), m.group(2)),
        body,
    )


def patch_implementation_agent(path: Path, agent: str) -> None:
    text = path.read_text(encoding="utf-8")
    if "C15 report contract" in text:
        return
    c15_impl = """
## C15 assignment IDs (PRD-C15-001 §6.5)

Your explicit assignment may contain `task-*` IDs, `find-*` IDs, or both. `tasks_completed` and `tasks_remaining` must list only IDs from **your** assignment, be disjoint, and together cover every assigned ID exactly once.

A verification wave with **no** assigned IDs accepts only empty arrays in both fields.

Include verified `find-*` IDs in `tasks_completed` when you fixed that finding.

""" + IMPL_C15_PROHIBITION
    text = text.replace(
        "A green test subset does not make an incomplete assignment complete. Never claim `complete` merely because the tests you chose passed.\n## Output Format",
        "A green test subset does not make an incomplete assignment complete. Never claim `complete` merely because the tests you chose passed."
        + c15_impl
        + "\n## Output Format",
    )
    text = text.replace(
        f'"tasks_completed": ["task-1"]',
        f'"tasks_completed": ["task-1", "find-qa-1"]',
    )
    if "Empty verification wave" not in text:
        text = text.replace(
            "Never claim that an unassigned or differently owned task was completed.",
            "Never claim that an unassigned or differently owned task was completed.\n\n### Empty verification wave example\n\n```json\n{\n  \"stage\": \"implementation\",\n  \"agent\": \""
            + agent
            + "\",\n  \"status\": \"complete\",\n  \"tasks_completed\": [],\n  \"tasks_remaining\": [],\n  \"files_changed\": [],\n  \"acceptance_gates\": {\n    \"completed_tasks_verified\": true,\n    \"task_ownership_respected\": true,\n    \"required_tests_passed\": true\n  },\n  \"tests_passed\": true,\n  \"blocker\": null,\n  \"next_action\": null,\n  \"summary\": \"Verification wave: no assigned task or finding IDs.\"\n}\n```",
        )
    path.write_text(text, encoding="utf-8")


def patch_discover(path: Path) -> None:
    text = path.read_text(encoding="utf-8")
    if "Research ToDo adoption" in text:
        return
    block = """
2. **Research ToDo adoption (PRD-C15-001 §9.3):** After reading the cycle objective and any active idea notes, query pending project ToDos via the deterministic CLI (or read the Pending projection in `context/current-state.md` when instructed). Present concise IDs and summaries in the user's chat language. Ask which remaining items, if any, should enter this cycle. Persist adopted selections through the orchestrator/CLI — do **not** edit OpenSpec checkboxes or finding state yourself. Include adopted items in requirements handed to Planning.
"""
    text = text.replace(
        "2. **Active idea notes (optional):**",
        block + "\n3. **Active idea notes (optional):**",
    )
    text = text.replace("\n3. Conduct a structured grilling", "\n4. Conduct a structured grilling")
    text = text.replace("\n4. Summarize the agreed", "\n5. Summarize the agreed")
    text = text.replace("\n5. **Pre-document gate", "\n6. **Pre-document gate")
    text = text.replace("\n6. If the user adds", "\n7. If the user adds")
    text = text.replace("\n7. Only after the gate", "\n8. Only after the gate")
    text = text.replace("\n8. Generate the decided", "\n9. Generate the decided")
    text = text.replace("\n9. Number cycle documents", "\n10. Number cycle documents")
    text = text.replace("\n10. Write all documents", "\n11. Write all documents")
    text = text.replace("\n11. Report completion", "\n12. Report completion")
    path.write_text(text, encoding="utf-8")


def patch_orchestration(path: Path) -> None:
    text = path.read_text(encoding="utf-8")
    if "/hero-add-todo" in text and "find-*" in text and "judge-gaps" not in text:
        return
    text = text.replace(
        "`/hero-cycles`, `/hero-todos`, `/hero-help`.",
        "`/hero-cycles`, `/hero-todos`, `/hero-add-todo`, `/hero-complete-todo`, `/hero-help`.",
    )
    text = text.replace(
        "- QA / Browser UI Validation / QA End-to-End failures loop back to implementation agents.",
        "- QA / Judge / Browser UI Validation / QA End-to-End failures: subagents emit JSON only; the orchestrator/TUI scheduler atomically persists findings and loop-back (never gap files or agent-driven `hero stage loop-back`).",
    )
    insert = """
## C15 findings and assignments (PRD-C15-001 §7)

- Validation agents return JSON only. Failed QA/Judge/BUI/E2E entries require `repro: {package, test, source}` (`package` like `./internal/tui`, `test` a `Test*` name, `source` the full failing `func Test…(`). The TUI scheduler extracts the first JSON object with `status` from mixed agent output, including `repro.source` that contains Go braces; in Cursor IDE Runtime pass that object to `hero stage close --name <stage> --failed --findings-json '<JSON>'`. Never ask a subagent to loop back or write gap markdown.
- Implementation assignments may mix `task-*` and `find-*` IDs per owner. Pass the exact ID list in every Implementation Task prompt.
- When the loop is **Escalated**, the user may run `/hero-add-todo` to defer selected open/reopened findings to project ToDos, then `/hero-continue` for remaining blockers. Deferring every blocker closes the cycle with disposition `completed_with_deferred_todos` (distinct from `/hero-finish`).
- `/hero-complete-todo` resolves pending structured or legacy ToDos fixed outside Hero (never items adopted by the active cycle).
- `/hero-todos` stays read-only; it lists pending/adopted projection items and the `/hero-sync` notice.

"""
    if "C15 findings and assignments" not in text:
        text = text.replace(
            "## Implementation task ownership",
            insert + "## Implementation task ownership",
        )
    text = text.replace(
        "- every `tasks_completed` and `tasks_remaining` ID belongs to that report's explicit assignment;",
        "- every `tasks_completed` and `tasks_remaining` ID belongs to that report's explicit assignment (including any assigned `find-*` IDs);",
    )
    path.write_text(text, encoding="utf-8")


def patch_browser_ui(path: Path) -> None:
    text = path.read_text(encoding="utf-8")
    if "C15 report contract" in text:
        return
    out = """
## Output Format

Allowed top-level fields: `status`, `health_passed`, `visual_ran`, `visual_passed`, `failure_class`, `failures`, `warnings`, `artifacts_dir`, `summary`.

`status` must be `passed` or `failed`. On `passed`, `failures` must be `[]`. `warnings` must be present (use `[]` when none).

Browser failure entries include `failure_class` (`frontend` or `backend`); owner is derived — do not set `owner` manually. Each failure needs `file` and/or `requirement`, `issue`, `acceptance_criteria`, optional `evidence`, optional `reopen_id`.

""" + C15_PROHIBITION + """

### Passing example

```json
{
  "status": "passed",
  "health_passed": true,
  "visual_ran": false,
  "visual_passed": null,
  "failure_class": null,
  "failures": [],
  "warnings": [],
  "artifacts_dir": ".workflow-hero/cycles/current/browser-ui/",
  "summary": "Browser Health passed. Visual Validation skipped (disabled)."
}
```

### Failed Health example

```json
{
  "status": "failed",
  "health_passed": false,
  "visual_ran": false,
  "visual_passed": null,
  "failure_class": "frontend",
  "failures": [
    {
      "failure_class": "frontend",
      "file": ".workflow-hero/cycles/current/browser-ui/health-report.md",
      "issue": "CSS bundle failed to load.",
      "acceptance_criteria": "All required CSS assets load without network errors.",
      "evidence": [".workflow-hero/cycles/current/browser-ui/screenshots/health.png"],
      "reopen_id": null
    }
  ],
  "warnings": [],
  "artifacts_dir": ".workflow-hero/cycles/current/browser-ui/",
  "summary": "Browser Health failed (frontend)."
}
```

### Reopening example

```json
{
  "status": "failed",
  "health_passed": false,
  "visual_ran": false,
  "visual_passed": null,
  "failure_class": "backend",
  "failures": [
    {
      "failure_class": "backend",
      "file": ".workflow-hero/cycles/current/browser-ui/health-report.md",
      "issue": "API health check still returns 500.",
      "acceptance_criteria": "Health endpoint returns 200 with valid payload.",
      "evidence": [],
      "reopen_id": "find-bui-1"
    }
  ],
  "warnings": [],
  "artifacts_dir": ".workflow-hero/cycles/current/browser-ui/",
  "summary": "Reopened find-bui-1; backend health still failing."
}
```
"""
    text = re.sub(r"## Output Format\n.*", out.strip() + "\n", text, flags=re.DOTALL)
    path.write_text(text, encoding="utf-8")


def patch_e2e(path: Path) -> None:
    text = path.read_text(encoding="utf-8")
    if "C15 report contract" in text:
        return
    out = """
## Output Format

Allowed top-level fields: `status`, `use_playwright`, `tests_passed`, `flows_validated`, `failures`, `summary`.

`status` must be `passed` or `failed`. On `passed`, `failures` must be `[]`. Include `flows_validated` (use `[]` when none).

Each failure entry requires `owner`, plus `file` and/or `requirement`, `issue`, `acceptance_criteria`, optional `evidence`, optional `reopen_id`.

""" + C15_PROHIBITION + """

### Passing example

```json
{
  "status": "passed",
  "use_playwright": false,
  "tests_passed": true,
  "flows_validated": ["checkout", "payment", "confirmation"],
  "failures": [],
  "summary": "All 3 user flows validated successfully."
}
```

### Failed example

```json
{
  "status": "failed",
  "use_playwright": true,
  "tests_passed": false,
  "flows_validated": ["login"],
  "failures": [
    {
      "owner": "frontend_agent",
      "file": "e2e/checkout.spec.ts",
      "issue": "Checkout flow times out on payment step.",
      "acceptance_criteria": "Checkout journey completes without errors.",
      "evidence": ["playwright test e2e/checkout.spec.ts"],
      "reopen_id": null
    }
  ],
  "summary": "Checkout flow failed."
}
```

### Reopening example

```json
{
  "status": "failed",
  "use_playwright": false,
  "tests_passed": false,
  "flows_validated": [],
  "failures": [
    {
      "owner": "backend_agent",
      "requirement": "PRD acceptance: order confirmation email",
      "issue": "Confirmation API still returns 500.",
      "acceptance_criteria": "POST /orders returns 201 and triggers email job.",
      "evidence": ["curl -f http://localhost:8080/orders"],
      "reopen_id": "find-e2e-1"
    }
  ],
  "summary": "Reopened find-e2e-1; confirmation API still failing."
}
```
"""
    text = re.sub(r"## Output Format\n.*", out.strip() + "\n", text, flags=re.DOTALL)
    path.write_text(text, encoding="utf-8")


def patch_qa(path: Path) -> None:
    text = path.read_text(encoding="utf-8")
    if "C15 report contract" in text:
        return
    text = re.sub(r"## Metrics \(required.*?\n\n", "", text, flags=re.DOTALL)
    text = re.sub(r"## Output Format\n.*", QA_BODY_TAIL.strip() + "\n", text, flags=re.DOTALL)
    path.write_text(text, encoding="utf-8")


def write_judge_cursor() -> None:
    front = """---
name: judge_agent
description: Validates SDD requirement coverage during the Judge stage. Does not assess code style.
model: inherit
---
"""
    write_agent("cursor", "agents/judge_agent.md", front, JUDGE_BODY)


HERO_ADD_TODO = """# /hero-add-todo — Defer Findings to Project ToDos (Escalated)

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Usage

```
/hero-add-todo [find-id ...]
```

In Cursor Runtime, supply one or more open or reopened finding IDs (for example `/hero-add-todo find-qa-1 find-judge-2`). The TUI may open a checklist when IDs are omitted.

## Responsibilities

1. Run `hero status` and confirm the relevant stage loop is **Escalated** with at least one open or reopened finding.
2. If preconditions fail, explain precisely (for example: not Escalated, no open findings, unknown ID, already deferred) and **stop** without mutating state.
3. When valid, invoke the deterministic CLI (exact flags evolve with C15 engine work — prefer `hero add-todo` with finding IDs). The service:
   - moves selected findings to `deferred_todo` and creates/links pending project ToDos;
   - updates the Pending projection in `context/current-state.md` idempotently;
   - leaves unselected open findings and unchecked OpenSpec tasks as blockers when only a subset is deferred.
4. After **partial** deferral: tell the user the loop stays Escalated and they must run `/hero-continue [N]` before more productive work.
5. When **every** blocker is deferred (no unchecked tasks, no open/reopened findings): the cycle closes with disposition `completed_with_deferred_todos` — no empty Implementation wave, enabled downstream stages are skipped. State this explicitly in chat.
6. Do not edit OpenSpec checkboxes or call `hero stage loop-back` from this command.

## Output Format

```
→ Escalation triage via /hero-add-todo …
✓ Added <N> finding(s) to project ToDos: find-…
⚠ … (Escalated remainder or completed_with_deferred_todos summary)
```
"""

HERO_COMPLETE_TODO = """# /hero-complete-todo — Manually Resolve a Pending ToDo

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Usage

```
/hero-complete-todo <id> [<id>...]
```

Accepts structured ToDo IDs (often the same as a deferred finding ID) or legacy pending items promoted to `todo-*` on first use.

## Responsibilities

1. List pending items with `/hero-todos` when the user is unsure of IDs.
2. Collect a **non-empty, safe resolution note** (no secrets) explaining how the work was completed outside Hero.
3. Require explicit user confirmation before mutating state (Cursor: confirm in chat; TUI uses its confirmation dialog).
4. Invoke the deterministic CLI (`hero complete-todo` or equivalent) which:
   - accepts only **pending** items (rejects items **adopted** by the active cycle);
   - sets state to `resolved` with timestamp and note;
   - removes the item from the Pending projection and default `/hero-todos` output;
   - retains history in SQLite/events (idempotent retry for the same resolved ID).
5. `/hero-todos` remains read-only — this command performs the mutation.

## Output Format

```
→ Manual ToDo completion: <id> …
✓ Resolved <id> with note recorded.
→ Run /hero-todos to verify Pending projection.
```
"""


def patch_command_help_table(text: str) -> str:
    if "/hero-add-todo" in text:
        return text
    row = "| /hero-add-todo | Defer Escalated findings to project ToDos (partial or completed_with_deferred_todos) |\n"
    row2 = "| /hero-complete-todo | Manually resolve pending ToDos fixed outside Hero (note + confirmation) |\n"
    return text.replace("| /hero-todos |", row + row2 + "| /hero-todos |")


def patch_hero_continue(text: str) -> str:
    if "hero-add-todo" in text:
        return text
    add = """
## Escalation triage (PRD-C15-001 §8)

After granting iterations with `/hero-continue`, remind the user they may also run `/hero-add-todo` to defer selected findings to project ToDos. Partial deferral keeps the loop Escalated until remaining blockers are resolved or every blocker is deferred (terminal `completed_with_deferred_todos` closure).

"""
    return text.replace("## Output Format", add + "## Output Format")


def patch_hero_finish(text: str) -> str:
    if "completed_with_deferred_todos" in text:
        return text
    add = """
## Emergency finish vs deferred closure

`/hero-finish` remains available for emergency cycle termination. When open or reopened findings still exist, require strong confirmation that those findings will **not** become deferred ToDos. This is distinct from `/hero-add-todo` deferring every blocker, which closes with disposition `completed_with_deferred_todos` without treating the cycle as a normal validated completion.

"""
    return text.replace("## Metrics", add + "## Metrics")


def patch_hero_todos(text: str) -> str:
    if "hero.db" in text and "adopted" in text:
        return text
    add = """
## C15 ToDo sources (PRD-C15-001 §9)

- `hero.db` is authoritative for structured ToDo lifecycle (`pending`, `adopted`, `resolved`).
- `context/current-state.md` Pending sections are the human-readable projection (updated by deterministic CLI operations such as `hero add-todo` and `hero complete-todo`).
- Default output lists **pending** and **adopted** items when present. Resolved items are omitted unless the user asks for history via CLI/events.

"""
    return text.replace("## Scope", add + "## Scope")


def patch_hero_status(text: str) -> str:
    if "findings" in text and "completed_with_deferred_todos" in text:
        return text
    add = """
## C15 additive status (PRD-C15-001 §10)

When using `hero status --json`, relay additive blocks when present: finding counts and rows, loop-back history, deferred ToDo IDs, completion disposition (`completed_with_deferred_todos`), and escalation CTAs (`/hero-continue`, `/hero-add-todo`, `/hero-cancel`, `/hero-finish`).

"""
    return text.replace("## Output Format", add + "## Output Format")


def patch_hero_start(text: str) -> str:
    if "/hero-add-todo" in text:
        return text
    return text.replace(
        "Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End",
        "Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End\n\nEscalation controls include `/hero-continue`, `/hero-add-todo`, and `/hero-complete-todo` (see workflow-help and orchestration_agent).",
        1,
    )


def patch_workflow_help(text: str) -> str:
    if "/hero-add-todo" in text:
        return text
    en = """
## 10.1 Escalation triage and deferred ToDos (C15)

When a validation loop hits **Escalated** (iteration limit), you choose what still blocks the cycle:

| Command | Purpose |
|---------|---------|
| `/hero-continue [N]` | Grant extra iterations for remaining open findings and unchecked OpenSpec tasks |
| `/hero-add-todo [id...]` | Defer selected open/reopened findings to durable project ToDos (TUI checklist when IDs omitted) |
| `/hero-finish` | Emergency finish — strong confirmation when findings remain; does **not** auto-create deferred ToDos |
| `/hero-cancel` | Cancel with rollback semantics unchanged |

Partial deferral keeps the loop Escalated until blockers are gone. Deferring **every** blocker closes the cycle with disposition **`completed_with_deferred_todos`** (no empty Implementation wave; downstream validation stages skipped).

At **Research** startup, pending ToDos are offered for adoption into the new cycle (see PRD-C15-001 §9.3). Use **`/hero-complete-todo <id>`** with a resolution note to mark pending items resolved when fixed outside Hero. **`/hero-todos`** stays read-only.

"""
    text = text.replace("## 10. Runtime commands (Cursor chat)", en + "## 10. Runtime commands (Cursor chat)")
    text = text.replace(
        "| `/hero-continue` | Grant extra iterations after escalation |",
        "| `/hero-continue` | Grant extra iterations after escalation |\n| `/hero-add-todo` | Defer Escalated findings to project ToDos |\n| `/hero-complete-todo` | Manually resolve pending ToDos with a note |",
    )
    pt = """
## 10.1 Triagem de escalonamento e ToDos adiados (C15)

Quando um loop de validação fica **Escalated**, use `/hero-continue`, `/hero-add-todo` e `/hero-complete-todo` conforme a §10.1 em inglês. Adiar todos os bloqueadores fecha o ciclo com **`completed_with_deferred_todos`**. `/hero-todos` continua somente leitura.

"""
    text = text.replace("## 10. Comandos Runtime (chat Cursor)", pt + "## 10. Comandos Runtime (chat Cursor)", 1)
    text = text.replace(
        "`/hero-new`, `/hero-start`, `/hero-approve`, `/hero-reject`, `/hero-cancel`, `/hero-continue`, `/hero-back`, `/hero-finish`, `/hero-archive`, `/hero-resume`, `/hero-sync`, `/hero-status`, `/hero-cycles`, `/hero-todos`, `/hero-model`, `/hero-help`",
        "`/hero-new`, `/hero-start`, `/hero-approve`, `/hero-reject`, `/hero-cancel`, `/hero-continue`, `/hero-back`, `/hero-finish`, `/hero-archive`, `/hero-resume`, `/hero-sync`, `/hero-status`, `/hero-cycles`, `/hero-todos`, `/hero-add-todo`, `/hero-complete-todo`, `/hero-model`, `/hero-help`",
    )
    return text


def main() -> None:
    write_judge_cursor()
    sync_from_cursor("agents/judge_agent.md")

    for agent in ("backend_agent", "frontend_agent", "generic_agent"):
        patch_implementation_agent(ROOT / "assets/cursor/agents" / f"{agent}.md", agent)
        sync_from_cursor(f"agents/{agent}.md")

    patch_discover(ROOT / "assets/cursor/agents/discover_agent.md")
    sync_from_cursor("agents/discover_agent.md")

    patch_orchestration(ROOT / "assets/cursor/agents/orchestration_agent.md")
    sync_from_cursor("agents/orchestration_agent.md")

    patch_qa(ROOT / "assets/cursor/agents/qa_agent.md")
    sync_from_cursor("agents/qa_agent.md")

    patch_browser_ui(ROOT / "assets/cursor/agents/browser_ui_agent.md")
    sync_from_cursor("agents/browser_ui_agent.md")

    patch_e2e(ROOT / "assets/cursor/agents/end2end_qa_agent.md")
    sync_from_cursor("agents/end2end_qa_agent.md")

    cmd_names = [
        "hero-add-todo.md",
        "hero-complete-todo.md",
        "hero-help.md",
        "hero-continue.md",
        "hero-finish.md",
        "hero-todos.md",
        "hero-status.md",
        "hero-start.md",
    ]
    for h in HARNESS:
        cmd_dir = ROOT / "assets" / h / "commands"
        cmd_dir.mkdir(parents=True, exist_ok=True)
        (cmd_dir / "hero-add-todo.md").write_text(HERO_ADD_TODO, encoding="utf-8")
        (cmd_dir / "hero-complete-todo.md").write_text(HERO_COMPLETE_TODO, encoding="utf-8")

    for name in cmd_names[2:]:
        src = ROOT / "assets/cursor/commands" / name
        text = src.read_text(encoding="utf-8")
        if name == "hero-help.md":
            text = patch_command_help_table(text)
        elif name == "hero-continue.md":
            text = patch_hero_continue(text)
        elif name == "hero-finish.md":
            text = patch_hero_finish(text)
        elif name == "hero-todos.md":
            text = patch_hero_todos(text)
        elif name == "hero-status.md":
            text = patch_hero_status(text)
        elif name == "hero-start.md":
            text = patch_hero_start(text)
        src.write_text(text, encoding="utf-8")
        for h in ("opencode", "codex", "claude"):
            (ROOT / "assets" / h / "commands" / name).write_text(text, encoding="utf-8")

    help_path = ROOT / "assets/docs/workflow-help.md"
    help_path.write_text(patch_workflow_help(help_path.read_text(encoding="utf-8")), encoding="utf-8")

    print("C15 assets updated.")


if __name__ == "__main__":
    main()
