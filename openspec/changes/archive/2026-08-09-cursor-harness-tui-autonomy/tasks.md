# Tasks — cursor-harness-tui-autonomy

Scope: **native** → all implementation via `generic_agent`.  
Nested generic Task fan-out: `composer-2.5` (`agents.generic_agent.subagent`).  
Terminology: **etapa** = workflow stage; **interação** = conversational round within an etapa.  
Do **not** implement D1 multi-harness adapters, IDE chat injection, or Go LLM calls.

**Parallelism legend:** **PARALLEL** = concurrent Task subagents after deps met; **SERIES** = ordered.

---

## 1. Harness interface + Cursor CLI runner — SERIES

- [x] 1.1 Expand `internal/harness` types: `Session`, `ExecuteRequest`, `ExecutionResult`, `ExecutionStatus`; extend `HarnessAdapter` per design D2; keep `Dispatch` as wrapper; colocated tests
- [x] 1.2 Implement Cursor CLI discovery (`cursor-agent` then `cursor agent`); `IsAvailable` (version + auth hints); injectable `CommandRunner` for tests (design D3)
- [x] 1.3 Implement `Execute` with `--print`, `--output-format json|stream-json`, `--resume`; JSON + stream-json parsers; default `Pusher` wired (remove nil-only path); adapter tests with fixture stdout (spec `cursor-cli-harness-execution`, `harness-adapter`)
- [x] 1.4 Optional SQLite `harness_session_id` on stages (migration v3) + store helpers; tests (design D6)

## 2. PARALLEL tracks after §1 — TUI boot ‖ Conversation ‖ Slash assets ‖ Cycles/todos ‖ Sync extension

### 2A. TUI harness boot — PARALLEL

- [x] 2A.1 Harness selection prompt when `cli.tools` empty (V1: cursor only); auto-detect `.cursor/`; persist `hero.json` (spec `tui-harness-boot`; design D5)
- [x] 2A.2 Validate on boot via `IsAvailable`; failure UX with `cursor agent login` guidance; exit non-zero; TUI tests (UI-C03-001 §2)

### 2B. TUI conversation orchestration — PARALLEL (needs §1.3)

- [x] 2B.1 Add conversation screen: transcript, input, streaming flag; wire `Execute` stream-json to viewport (spec `tui-conversation-orchestration`; UI-C03-001 §3)
- [x] 2B.2 Store/resume `harness_session_id` per etapa when interactive; cancel on Ctrl+C via `Cancel` (design D4, D6)
- [x] 2B.3 Colocated TUI tests (mock harness interface)

### 2C. TUI palette + hyphen slash parity — PARALLEL

- [x] 2C.1 Rename all Hero palette labels to `/hero-<name>`; add missing commands (`/hero-new`, `/hero-start`, `/hero-sync`, `/hero-status`, `/hero-continue`, `/hero-back`, `/hero-cycles`, `/hero-todos`); update `app_test.go` (spec `hero-tui`; ADR-024)
- [x] 2C.2 Wire new palette actions to `cycle.Service` or conversation/harness paths per command semantics

### 2D. Runtime assets — PARALLEL

- [x] 2D.1 Update all `assets/cursor/commands/hero-*.md` headers and body refs: `/hero:*` → `/hero-<name>`; add `hero-cycles.md`, `hero-todos.md` (ADR-024, ADR-028)
- [x] 2D.2 Extend `hero-sync.md` for product/architecture pending scan → `current-state.md` (ADR-029)
- [x] 2D.3 Update orchestration skill, workflow-help, README EN/PT-BR; extend `runtime_assets_test.go` to ban colon-form user labels and require hyphen form

### 2E. hero-cycles deterministic helpers — PARALLEL

- [x] 2E.1 Add `cycle.Service` or `internal/cycle` helper: aggregate cycles + metrics from SQLite + archive dirs; format per UI-C03-001 §5; tests with temp DB + archive fixtures (spec `hero-cycles-command`)

### 2F. hero-todos read path — PARALLEL

- [x] 2F.1 Parser for `context/current-state.md` pending sections + sync notice; shared by TUI action and Runtime asset guidance; tests (spec `hero-todos-command`)

## 3. Doctor + integration — SERIES

- [x] 3.1 Optional `hero doctor` checks: cursor CLI on PATH, login hint; warn-only complementary to TUI boot (PRD-C03-001 §4.10)
- [x] 3.2 Integration tests: boot abort when CLI missing; dispatch succeeds with mock runner; `go test ./...` green
- [x] 3.3 Update `context/current-state.md` and `context/context-log.md` when implementation lands
- [x] 3.4 `hero cycle openspec-change cursor-harness-tui-autonomy` on active cycle; final `go test ./...`

---

## Parallel groups (orchestrator fan-out)

After §1 complete:

| Group | Tasks | Agent |
|---|---|---|
| A | 2A.1–2A.2 | `generic_agent` |
| B | 2B.1–2B.3 | `generic_agent` |
| C | 2C.1–2C.2 | `generic_agent` |
| D | 2D.1–2D.3 | `generic_agent` |
| E | 2E.1 | `generic_agent` |
| F | 2F.1 | `generic_agent` |

Hard series spine: **1 → (2A ‖ 2B ‖ 2C ‖ 2D ‖ 2E ‖ 2F) → 3**.  
2B depends on 1.3; 2C label-only can start with 1.1.
