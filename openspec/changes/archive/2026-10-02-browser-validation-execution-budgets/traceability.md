# Traceability: browser-validation-execution-budgets

PRD-C17-001 acceptance criteria map explicitly to functional requirements and owned implementation tasks. Every implementation task has one canonical owner: `[agent:generic_agent]` (native scope).

| Acceptance | Functional requirement | Implementation tasks | Owner |
|---|---|---|---|
| B01 — setup checklist and required test accounts | FR-02 | task-01, task-14, task-17 | generic_agent |
| B02 — arbitrary user IDs and profiles | FR-02 | task-01, task-14 | generic_agent |
| B03 — safe dotenv round-trip and Config editing | FR-02 | task-01, task-02, task-14, task-17 | generic_agent |
| B04 — atomic saves, ignore/tracked-file protection, concurrent edits | FR-02, FR-03 | task-02, task-14, task-19 | generic_agent |
| B05 — private executor and lifecycle boundary | FR-01, FR-03, FR-04 | task-03, task-09, task-15 | generic_agent |
| B06 — credential sentinel absence across sinks | FR-03 | task-03, task-19 | generic_agent |
| B07 — fresh login, expected denial, archive and lifecycle safety | FR-01, FR-04 | task-03, task-15, task-19 | generic_agent |
| B08 — operational blocker, explicit continue and no automatic repair | FR-07, FR-08 | task-05, task-06, task-08 | generic_agent |
| B09 — mixed blocker and genuine finding preservation | FR-08 | task-05, task-06 | generic_agent |
| B10 — no login-only pass and optional-reference warnings | FR-07 | task-05, task-10 | generic_agent |
| B11 — selected-method preference, live capability and bounded preparation | FR-05, FR-06 | task-03, task-09, task-16 | generic_agent |
| B12 — no QA provisioning or silent HTTP fallback | FR-05, FR-06 | task-03, task-09, task-16 | generic_agent |
| B13 — cumulative active budgets and expiry across stages/workers | FR-09, FR-10 | task-07, task-08, task-19 | generic_agent |
| B14 — pause, restart and resume without budget reset | FR-09 | task-07, task-19 | generic_agent |
| B15 — serialized cancellation, expiry and late-result rejection | FR-10 | task-07, task-19 | generic_agent |
| B16 — passive health and useful progress | FR-10, FR-11 | task-07, task-08, task-19 | generic_agent |
| B17 — four-harness parity, no IDE execution or removal | PRD §6 | task-16, task-18 | generic_agent |
| B18 — per-stage screenshot controls, safe capture and archive retention | FR-12 | task-11, task-14, task-15, task-17 | generic_agent |
| B19 — latest/list/id/todos during streaming without capture or dispatch | FR-13 | task-12 | generic_agent |
| B20 — addressed Telegram delivery under always_send with partial-failure safety | FR-12, FR-13 | task-11, task-13 | generic_agent |
| B21 — actionable user intervention and help across TUI/Telegram | FR-08, FR-11, FR-13 | task-06, task-08, task-13, task-16 | generic_agent |

Cross-cutting work is decomposed by implementation boundary: reports (task-05), scheduler state (task-06), budgets (task-07), budget/progress display (task-08), capture ingestion (task-11), screenshot controls/collection (task-12), and Telegram image transport (task-13). Release checks and regression coverage are consolidated in task-19. Task ownership is not inferred from prose.
