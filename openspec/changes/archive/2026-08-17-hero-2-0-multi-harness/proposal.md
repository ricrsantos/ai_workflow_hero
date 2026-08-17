## Why

Hero 1.x executes AI only through the **Cursor** adapter (`hero install --tools cursor`). Users cannot assign different harnesses to different agents, and the TUI cannot run OpenCode. Cycle C4 delivers **Hero 2.0.0**: TUI-orchestrated multi-harness (Cursor + OpenCode) while preserving Cursor IDE Runtime and existing Cursor TUI behavior (PRD-C04-001 §1; ADR-031–037).

## What Changes

- **BREAKING (2.0.0):** Remove `--tools` from `hero install` / `hero upgrade`; require `harness` on every agent and `fallback_model` in `workflow-config.yml`; migrate `cli.tools` → `harnesses.<id>.enabled` (ADR-034, ADR-032).
- Add **`OpenCodeAdapter`** (`internal/adapters/opencode`) implementing `HarnessAdapter` via Hero-managed `opencode serve` + HTTP API (ADR-035).
- **TUI multi-harness orchestration:** route Execute to the adapter named by YAML `harness`; Chat shows **agent + model + harness** (ADR-031, ADR-037).
- New TUI slashes: **`/hero-harness`** (enable/disable + provision); **`/hero-model`** stores **(harness, model)** pair for freechat and `/hero-new` (ADR-037; amends ADR-030).
- Extend fallback chain: agent `(harness, model)` → `fallback_model` pair → stop + `/hero-continue` (ADR-033; amends ADR-008).
- **OpenCode projection** from `assets/` on enable; disable keeps files (ADR-036).
- **Dedicated task:** update `assets/models/*.yml` with OpenCode-native model ids and pricing (PRD-C04-001 §4.10).
- **Compatibility mandate:** Cursor Adapter Execute, Cursor IDE slash Runtime, checksum upgrade, dual-entry, deterministic engine must not regress; minimize Cursor adapter diffs.

### In Scope

- Cursor + OpenCode harnesses in Hero TUI only; Cursor IDE stays Cursor-only (ADR-031).
- Interactive install harness picker (≥1 harness; not PATH-filtered).
- Project `hero.db` serve registry + orphan reap on TUI boot.
- Template `workflow-config.yml` with `harness` on all agents + `fallback_model`.

### Out of Scope

- Claude Code, Codex, VS Code, or harnesses beyond Cursor + OpenCode.
- Hero canonical model ids / cross-harness translation.
- `hero serve` daemon; attaching to foreign `opencode serve`.
- Two concurrent `hero tui` in one project.
- Windows CLI; CI/CD releases; GPG signing.
- Deleting projections on disable; auto-enabling OpenCode on upgrade.

### CLI vs Runtime Classification (ADR-003)

- **CLI:** `internal/adapters/opencode`, `internal/harness`, `internal/install`, `internal/upgrade`, `internal/tui`, `internal/store`, `internal/workflowconfig`, `assets/models/`, `assets/opencode/`.
- **Runtime:** template `workflow-config.yml` comments; workflow-help/README copy; **no** Cursor IDE multi-harness routing.

## Capabilities

### New Capabilities

- `hero-json-harness-state`: `harnesses.<id>.enabled` + model; upgrade migration from `cli.tools`; freechat default pair (ADR-034, ADR-037; PRD-C04-001 §4.11).
- `interactive-harness-install`: Multi-select harness picker; `--tools` explicit error; upgrade preserves Cursor-only (ADR-034; UI-C04-001 §2).
- `workflow-config-harness`: Required `harness` + native `model` per agent; `/hero-new` injection from enabled set (ADR-032; PRD-C04-001 §4.3).
- `opencode-adapter`: `OpenCodeAdapter` with serve lifecycle, HTTP API Execute/stream/cancel/session, injectable deps (ADR-035; PRD-C04-001 §4.2, §4.8).
- `opencode-serve-registry`: SQLite registry (pid, port, url, harness, created_at); orphan reap; stop on quit/disable (ADR-035).
- `opencode-projection`: Provision `.opencode/` from `assets/` on enable; checksum rules; disable keeps files (ADR-036; PRD-C04-001 §4.6).
- `harness-fallback-chain`: Pair-aware fallback with warnings; hard stop + `/hero-continue` (ADR-033; UI-C04-001 §6).
- `hero-harness-command`: `/hero-harness` enable/disable; last-harness guard; provision on enable (ADR-037; UI-C04-001 §3).
- `hero-model-pair`: `/hero-model` two-column picker; persist pair; stage YAML untouched (ADR-037; UI-C04-001 §4).
- `tui-multi-harness-execution`: Adapter registry; Execute routing by harness; speaker `[AGENT - model · harness]` (ADR-031; UI-C04-001 §5).
- `opencode-model-catalog`: OpenCode-native ids in `assets/models/*.yml`; Cursor slugs preserved; metrics lookup (PRD-C04-001 §4.10).

### Modified Capabilities

- `harness-adapter`: Second implementation (OpenCode); registry resolves by harness id; session binding per harness (ADR-016 amended).
- `hero-tui`: `/hero-harness`; `/hero-model` pair picker; harness labels; boot enabled≠available (UI-C04-001).
- `sqlite-operational-store`: Serve registry table; `stages.harness_id` or session harness binding (ADR-035).
- `asset-bootstrap-and-layout`: OpenCode projection paths; install/upgrade for enabled harnesses only.
- `cli-deterministic-command-suite`: `--tools` removed; install harness picker.
- `runtime-workflow-execution`: YAML `harness` field; Cursor IDE ignores harness (ADR-031).

## Impact

- Packages: `internal/adapters/opencode` (new), `internal/adapters/cursor` (minimal diffs), `internal/harness`, `internal/install`, `internal/upgrade`, `internal/tui`, `internal/store`, `internal/workflowconfig`, `assets/opencode/`, `assets/models/`.
- External: `opencode` CLI on PATH when OpenCode enabled; `cursor agent` unchanged for Cursor.
- Tests: injectable HTTP/process for OpenCode; install picker; `--tools` error; upgrade migration; orphan reap; YAML harness validation; model catalog lookup; `go test ./...`.
- Implementation agent: **generic_agent** (`scope.native: true`).
- OpenSpec change: `hero-2-0-multi-harness` (Hero 2.0.0).
