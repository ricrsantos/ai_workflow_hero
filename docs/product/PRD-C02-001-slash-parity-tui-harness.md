# PRD-C02-001 — Slash Command Parity, TUI Harness Commands & Archive Coupling

> Cycle C2 product requirements. Extends Hero 1.0 dual-UI and Cursor adapter behavior. Does not reopen deferred multi-harness implementation (D1). Index: [PRD.md](PRD.md).

## 1. Overview

After Hero 1.0, user-facing guidance drifted toward CLI verbs (`hero cycle new`, `hero approve`, …) instead of the **0.9 slash vocabulary** (`/hero:new`, `/hero:start`, …). The TUI command palette likewise uses prose labels (`Approve stage`, `Finish cycle`) that do not match Runtime slash names.

This cycle restores **slash-first UX parity** for Hero commands, adds **discovery and markdown-expansion execution** of non-Hero Cursor custom commands, improves **harness detection suggestions**, and couples **`/hero:archive`** with **OpenSpec `archive`**.

## 2. Goals

- Make chat Runtime and Hero TUI speak the same user-facing command names as Hero 0.9 slash commands.
- Keep CLI-as-API verbs as the deterministic implementation layer (agents may still call `hero …`; users should be guided with `/hero:*`).
- Let the TUI list and run non-Hero project/global Cursor commands by expanding their `.md` bodies into agent prompts.
- Detect installed/used harness markers and warn when they diverge from `hero.json` → `cli.tools`, without installing unsupported harness assets.
- Archive the linked OpenSpec change when archiving a Hero cycle, with a force path if OpenSpec fails.

## 3. In Scope (C2)

| Area | Requirement |
|---|---|
| Runtime vocabulary | All user-facing Runtime/orchestrator messages prefer `/hero:*` (0.9 set). CLI verbs are secondary/implementation detail. |
| TUI Hero commands | Palette, footers, and empty-state hints use `/hero:*` names. Execution paths for Hero actions stay on `cycle.Service` / CLI API (rename only). |
| Non-Hero commands | Discover `.cursor/commands/*.md` (project) and `~/.cursor/commands/*.md` (user). Exclude Hero-owned `hero-*.md` from the “imported” list (those are covered by Hero TUI actions). |
| Command execution | On select: read markdown → dispatch content as agent prompt via Cursor harness adapter (cwd = project). Do **not** rely on sending literal `/name` strings to IDE chat (no stable inject API). |
| Skills | Do **not** list `.cursor/skills` in the TUI palette. Rely on Cursor Agent CLI auto-discovery of project skills when dispatching with project cwd. |
| Harness detection | On `hero install`, `hero sync`, and `hero doctor`: detect known harness folders; compare to `cli.tools`; warn-only for unsupported harnesses. |
| Archive coupling | `/hero:archive` / `hero cycle archive`: run `openspec archive <change> -y` first (merge delta specs), then archive Hero cycle. Persist OpenSpec change name from Planning when available; else heuristic (0 skip / 1 auto / N ask). |
| Force archive | If OpenSpec fails: CLI flag `--force` (alias `--skip-openspec`) and Runtime prompt offering force + manual OpenSpec instructions. |

## 4. Out of Scope

- Implementing additional concrete harness adapters (Claude Code, Windsurf, …) — remains D1; detection is suggest/warn only.
- Injecting prompts into an already-open Cursor IDE chat panel.
- Listing skills in the TUI palette.
- Guaranteeing CLI parity for Cursor **plugin** skills or nested monorepo skill paths (upstream Cursor CLI gaps).
- Changing OpenSpec’s own slash UX beyond invoking `openspec archive`.
- Soft dual-mode 0.9 markdown cycles (D11).

## 5. Functional Requirements

### 5.1 Slash-first user vocabulary

- User-visible strings in Runtime assets (`hero-*.md`, orchestration skill guidance) and TUI must prefer `/hero:new`, `/hero:start`, `/hero:approve`, `/hero:reject`, `/hero:cancel`, `/hero:finish`, `/hero:archive`, `/hero:resume`, `/hero:sync`, `/hero:status`, `/hero:continue`, `/hero:back`, `/hero:help`.
- After configuration review, handoff must tell the user to run **`/hero:start`** in a clean chat — not “confirm here so I run `hero cycle new`” as the primary CTA (agents still run `hero cycle new` when appropriate, but the user-facing next step is slash-oriented).
- TUI palette items for Hero actions must be labeled with the slash form (e.g. `/hero:approve`), with optional short hints.

### 5.2 Non-Hero command inheritance (markdown expansion)

- Scan project `.cursor/commands/*.md` and user `~/.cursor/commands/*.md`.
- Present as `/<stem>` (filename without `.md`, Cursor naming conventions preserved).
- On invoke: load file contents (strip frontmatter only if required by SDD) and pass as prompt to the harness adapter dispatch path with `ProjectDir` set.
- If dispatch is unavailable, show a clear message that the user should run the command in Cursor chat; do not silently no-op.
- Document that this simulates IDE slash behavior (markdown-as-prompt), not IDE chat injection.

### 5.3 Harness detection

- Markers (initial set, extensible in SDD): `.cursor/`, `.claude/`, `.windsurf/`, `.codex/` (and equivalents agreed in SDD).
- Compare detected set to `.workflow-hero/config/hero.json` → `cli.tools`.
- `install` / `sync`: suggest registering/supporting tools when markers appear.
- `doctor`: warn on divergence and on unsupported markers (no asset install for unsupported tools).

### 5.4 Archive + OpenSpec

- During Planning (or when the OpenSpec change name is known), persist `openspec_change` (name TBD in SDD) on the cycle record.
- Archive resolution order: stored name → if absent, active changes under `openspec/changes/` (exclude `archive/`): 0 = skip OpenSpec step; 1 = use it; N = require user selection / fail closed until selected.
- Default OpenSpec invocation: `openspec archive <name> -y` (merge specs; no `--skip-specs`).
- Order: OpenSpec success (or skip) → `hero cycle archive`.
- On OpenSpec failure: do not archive Hero unless user/CLI forces with `--force` / `--skip-openspec`; then print manual `openspec archive <name> -y` instructions.
- Runtime `/hero:archive` must offer the force path interactively when OpenSpec fails.

## 6. Non-Functional Requirements

- No LLM reasoning in CLI for detection/archive/dispatch plumbing.
- Keep Feature Based + Vertical Slice; prefer extending `internal/tui`, `internal/doctor`, `internal/cycle`, `internal/adapters/cursor`, Runtime assets.
- Tests: palette naming, command discovery paths, archive orchestration (OpenSpec success/fail/force), doctor warnings.
- Chat language follows `workflow_config.user_preferred_language`; cycle docs remain English.

## 7. Success Criteria

- A user following chat or TUI guidance sees `/hero:*` as the primary command language (0.9 parity).
- TUI can list a non-Hero `.cursor/commands` entry and dispatch its markdown body via the adapter when available.
- `hero doctor` warns when `.claude/` exists but `cli.tools` only lists `cursor`.
- Archiving a cycle with a linked OpenSpec change runs `openspec archive -y` before Hero archive; force path works when OpenSpec fails.

## 8. References

- [PRD-C01-001-hero-1-0.md](PRD-C01-001-hero-1-0.md)
- [UI-C01-001-hero-tui.md](UI-C01-001-hero-tui.md)
- [ADR-C02-001-slash-parity-harness-archive.md](../architecture/ADR-C02-001-slash-parity-harness-archive.md)
- [UI-C02-001-tui-slash-command-parity.md](UI-C02-001-tui-slash-command-parity.md)
- OpenSpec CLI: `openspec archive [change-name] -y`
