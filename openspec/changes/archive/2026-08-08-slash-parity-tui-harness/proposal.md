## Why

After Hero 1.0, user-facing guidance drifted to CLI verbs (`hero cycle new`, `hero approve`) and TUI prose labels (`Approve stage`), breaking the **0.9 slash vocabulary** users expect (`/hero:new`, `/hero:approve`). Cycle C2 restores slash-first parity across Runtime and TUI, adds markdown-expansion for non-Hero Cursor commands, warn-only harness marker detection, and couples Hero archive with OpenSpec archive (PRD-C02-001 §1–2; ADR-020–023).

## What Changes

- Restore **slash-first user vocabulary** in Runtime assets and orchestrator guidance (`/hero:*`); CLI verbs remain the deterministic API for agents/TUI internals (ADR-020; PRD-C02-001 §5.1).
- Rename **TUI Hero action labels** to `/hero:*` (execution still via `cycle.Service` / CLI API — rename only) (UI-C02-001 §2; ADR-015 naming parity).
- Discover and run **non-Hero** Cursor custom commands from `.cursor/commands` and `~/.cursor/commands` by expanding `.md` bodies into harness `Dispatch` prompts; exclude `hero-*.md` from the import list (ADR-021; PRD-C02-001 §5.2).
- Do **not** list skills in the TUI; rely on Cursor Agent CLI loading `.cursor/skills` when cwd is the project (ADR-021).
- On `hero install` / `hero sync` / `hero doctor`: detect harness filesystem markers vs `hero.json` → `cli.tools`; **warn-only** for unsupported harnesses (ADR-022; PRD-C02-001 §5.3).
- Persist OpenSpec change name on the cycle (`openspec_change`); on archive run `openspec archive <name> -y` before Hero cycle archive; add `--force` / `--skip-openspec` and Runtime force prompt on OpenSpec failure (ADR-023; PRD-C02-001 §5.4).

### In Scope

- Slash-first Runtime + TUI naming; imported command discovery/expansion; harness marker detection (suggest/warn); archive ↔ OpenSpec coupling with force escape.
- Native scope only → implementation via `generic_agent`.

### Out of Scope

- Concrete multi-harness adapters (D1) — detection only.
- IDE chat panel injection.
- Listing `.cursor/skills` in the TUI palette.
- Soft dual-mode 0.9 markdown cycles (D11).
- Guaranteeing Cursor plugin / nested monorepo skill paths.

### CLI vs Runtime Classification (ADR-003)

- **CLI**: store field + archive orchestration, doctor/install/sync detection, TUI palette/import UI, Cursor dispatch payload from markdown — `internal/store`, `internal/cycle`, `internal/doctor`, `internal/install`, `internal/tui`, `internal/adapters/cursor` (and sync path if present).
- **Runtime**: `hero-*.md`, orchestration/skill copy for slash-first CTAs and `/hero:archive` force path.

## Capabilities

### New Capabilities

- `harness-command-import`: Discover non-Hero Cursor command markdown (project + user dirs), present as `/<stem>`, expand body into harness adapter Dispatch with project cwd; exclude `hero-*.md`; no skill listing (ADR-021; PRD-C02-001 §5.2; UI-C02-001 §3).
- `harness-marker-detection`: Detect known harness directories on install/sync/doctor; compare to `cli.tools`; suggest/warn; never install assets for unsupported tools (ADR-022; PRD-C02-001 §5.3; UI-C02-001 §5).
- `openspec-archive-coupling`: Persist `openspec_change` on cycle; resolve name (stored → 0/1/N heuristic); run `openspec archive -y` before Hero archive; `--force`/`--skip-openspec` + Runtime prompt (ADR-023; PRD-C02-001 §5.4; UI-C02-001 §4).

### Modified Capabilities

- `runtime-workflow-execution`: User-facing Runtime/orchestrator strings prefer `/hero:*`; clean-session handoff after `/hero:new` tells users to run `/hero:start` (ADR-020; PRD-C02-001 §5.1).
- `cli-deterministic-command-suite`: Extend `hero cycle archive` with OpenSpec pre-step and force flags; expose cycle `openspec_change` set/get for Planning; keep CLI verbs as API (ADR-023; ADR-014).
- `hero-tui`: Palette/footer/empty-state Hero actions use `/hero:*` labels; optional archive/resume/help actions; group imported harness commands; keep screen nav as non-slash (UI-C02-001 §2–3). Baseline introduced in change `hero-1-0` (not yet synced to `openspec/specs/`); this change adds a full delta/spec under the C2 change tree.
- `asset-bootstrap-and-layout`: Install/doctor (and sync when implemented) surface harness-marker warnings without claiming unsupported support (ADR-022).
- `sqlite-operational-store`: Add nullable `openspec_change` (TEXT) on `cycles` via schema migration v2 (ADR-013 extension for ADR-023). Baseline from `hero-1-0`; delta under this change.
- `harness-adapter`: Dispatch accepts expanded command markdown as `Prompt` (no IDE inject); clearer failure when dispatch unavailable (ADR-016 amended by ADR-021). Baseline from `hero-1-0`.

## Impact

- Packages: `internal/tui` (palette labels + import list), `internal/cycle` (archive orchestration, openspec_change API), `internal/store` (migration v2), `internal/doctor` / `internal/install` (+ sync if/when present), `internal/adapters/cursor` / `internal/harness`, Runtime assets under `assets/cursor/commands/` and orchestration skill.
- External: requires `openspec` on PATH when a change is linked; absence = OpenSpec failure → force path.
- Tests: palette naming, command discovery paths, archive success/fail/force, doctor warnings; `go test ./...`.
- Implementation agent: **generic_agent** (`scope.native: true`).
- Do **not** overwrite OpenSpec change `hero-1-0`.
