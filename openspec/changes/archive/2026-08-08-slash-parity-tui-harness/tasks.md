# Tasks — slash-parity-tui-harness

Scope: **native** → all implementation via `generic_agent`.  
Nested generic Task fan-out (when used): model `composer-2.5` (`agents.planning_agent.subagent` / `agents.generic_agent.subagent`).  
Do **not** implement Deferred D1 multi-harness adapters, IDE chat injection, skill palette listing, or D11 soft dual-mode.  
Do **not** overwrite OpenSpec change `hero-1-0`.

**Parallelism legend:** groups marked **PARALLEL** may run as concurrent Task subagents once their dependencies are met. Groups marked **SERIES** must complete in order.

---

## 1. Shared contracts (store + cycle API stubs) — SERIES

- [x] 1.1 Implement SQLite migration v2: add `cycles.openspec_change TEXT NOT NULL DEFAULT ''`; bump `currentSchemaVersion`; update `store.Cycle` + CRUD/read paths; colocated `store_test.go` (spec `sqlite-operational-store`; design D5)
- [x] 1.2 Add `hero cycle openspec-change <name>` / `--clear` and include `openspec_change` in `hero status` / `--json`; wire Cobra in `internal/cycle`; colocated tests (spec `cli-deterministic-command-suite`; `openspec-archive-coupling`)
- [x] 1.3 Export shared helper signatures used by later parallel work: (a) harness marker detect API, (b) Cursor command discovery + frontmatter strip API — stubs or full impl acceptable if tests green; document in package comments (design D3–D4)

## 2. PARALLEL tracks after §1 — Runtime ‖ Detection ‖ Command import ‖ Archive ‖ TUI labels

### 2A. Runtime slash-first assets — PARALLEL with 2B–2E

- [x] 2A.1 Audit/update `assets/cursor/commands/hero-*.md`, `orchestration_agent.md`, and `workflow-hero/SKILL.md` so user-facing CTAs prefer `/hero:*`; post-`/hero:new` handoff primary CTA is `/hero:start` (PRD-C02-001 §5.1; ADR-020; spec `runtime-workflow-execution`)
- [x] 2A.2 Update `hero-archive.md` for OpenSpec-first sequence + force path (`--force` / `--skip-openspec` + manual `openspec archive <name> -y`) (UI-C02-001 §4)
- [x] 2A.3 Update `hero-sync.md` to run/recommend `hero doctor` after sync for harness warnings; keep slash-first wording (design D4; ADR-003 — no CLI `hero sync`)
- [x] 2A.4 Extend `internal/common/runtime_assets_test.go` assertions for slash-first CTAs and archive force guidance

### 2B. Harness marker detection — PARALLEL with 2A, 2C–2E

- [x] 2B.1 Implement marker detection (`.cursor/`, `.claude/`, `.windsurf/`, `.codex/`) vs `hero.json` `cli.tools`; warn-only for unsupported (spec `harness-marker-detection`; ADR-022)
- [x] 2B.2 Hook detection into `internal/doctor` with UI-C02-001 §5 copy; colocated doctor tests
- [x] 2B.3 Hook detection into `internal/install` (suggest/warn, never install unsupported assets); update install tests (spec `asset-bootstrap-and-layout`)

### 2C. Command discovery + adapter — PARALLEL with 2A–2B, 2D–2E

- [x] 2C.1 Implement discover non-Hero commands from project + `~/.cursor/commands`; exclude `hero-*.md`; strip YAML frontmatter; unit tests with temp dirs (spec `harness-command-import`)
- [x] 2C.2 Ensure Cursor `Dispatch` path accepts expanded markdown `Prompt` and returns actionable unavailable messages; adapter tests (spec `harness-adapter`)

### 2D. OpenSpec archive coupling — PARALLEL with 2A–2C, 2E (needs §1)

- [x] 2D.1 Implement archive name resolution (stored → 0/1/N heuristic) and `openspec archive <name> -y` pre-step in `internal/cycle` Archive orchestration (spec `openspec-archive-coupling`; design D6)
- [x] 2D.2 Add `--force` / `--skip-openspec` and `--openspec-change` flags to `hero cycle archive`; block Hero archive on OpenSpec failure unless forced; print manual instructions; colocated tests with fake openspec runner
- [x] 2D.3 Ensure Planning handoff docs/tests note calling `hero cycle openspec-change <slug>` when the change name is known (Runtime planning_agent / orchestration guidance touch if needed)

### 2E. TUI slash labels + import section — PARALLEL with 2A–2D (needs 2C discovery API)

- [x] 2E.1 Rename Hero palette action labels to `/hero:*` per UI-C02-001 §2; keep `Go:` navigation; update empty-state hint for `/hero:new`; update `app_test.go` / palette tests (spec `hero-tui`)
- [x] 2E.2 Add palette group for imported harness commands; wire select → markdown expansion → Dispatch; progress + failure UX (UI-C02-001 §3); TUI tests with temp command files
- [x] 2E.3 Optionally expose `/hero:archive`, `/hero:resume`, `/hero:help` palette actions when wiring is straightforward; do not invent `/hero:run`; ensure skills are never listed

## 3. Integration, docs, context — SERIES (closing)

- [x] 3.1 Add/extend focused integration or package tests covering: palette slash labels, command discovery paths, archive success/fail/force, doctor harness warnings; run `go test ./...` until green
- [x] 3.2 Update `context/current-state.md` and append `context/context-log.md` for C2 implementation outcome when code lands
- [x] 3.3 Final `go test ./...` green check before marking change ready for QA/Judge

---

## Parallel groups (orchestrator fan-out)

After §1 complete:

| Group | Tasks | Agent |
|---|---|---|
| A | 2A.1–2A.4 | `generic_agent` (nested `composer-2.5` OK for asset batches) |
| B | 2B.1–2B.3 | `generic_agent` |
| C | 2C.1–2C.2 | `generic_agent` |
| D | 2D.1–2D.3 | `generic_agent` |
| E | 2E.1–2E.3 (after 2C.1 API available) | `generic_agent` |

Hard series spine: **1 → (2A ‖ 2B ‖ 2C ‖ 2D ‖ 2E*) → 3**.  
\*2E may start label-only (2E.1) in parallel with 2C; import wiring (2E.2) waits on 2C.1.
