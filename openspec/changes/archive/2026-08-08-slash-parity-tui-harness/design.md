## Context

Hero 1.0 shipped dual UI (chat Runtime + Bubble Tea TUI) over a CLI-as-API and SQLite store (`hero-1-0` change). User-facing copy drifted to CLI verbs and prose TUI labels, diverging from the 0.9 `/hero:*` vocabulary. C2 restores naming parity, adds non-Hero Cursor command import via markdown expansion, warn-only harness marker detection, and couples Hero cycle archive with OpenSpec archive (PRD-C02-001; ADR-020–023).

Scope: **native only** → `generic_agent`. Prior change `hero-1-0` remains untouched.

## Goals / Non-Goals

**Goals:**
- Slash-first Runtime + TUI labels (`/hero:*`); CLI verbs stay the deterministic API.
- Discover/run non-Hero `.cursor/commands` (project + `~/.cursor/commands`) via markdown → `HarnessAdapter.Dispatch`.
- Detect harness markers vs `cli.tools` on install/doctor (and via `/hero:sync` → doctor); warn-only for unsupported.
- Persist `openspec_change` on the cycle; archive runs `openspec archive <name> -y` then Hero archive; force escape on failure.

**Non-Goals:**
- New harness adapters (D1).
- IDE chat injection.
- Skills in TUI palette.
- Soft dual-mode 0.9 markdown cycles (D11).
- Inventing a `hero sync` CLI (ADR-003: sync stays Runtime-only).

## Decisions

### D1 — Slash-first is display/guidance only (ADR-020)

Runtime assets and TUI labels use `/hero:*`. Internal execution remains `cycle.Service` / `hero …` verbs. Do not rename Cobra commands solely for display parity.

**Canonical slash set:** `/hero:new`, `/hero:start`, `/hero:approve`, `/hero:reject`, `/hero:cancel`, `/hero:finish`, `/hero:archive`, `/hero:resume`, `/hero:sync`, `/hero:status`, `/hero:continue`, `/hero:back`, `/hero:help`.

### D2 — TUI Hero actions: label map (UI-C02-001 §2)

| Action | Label |
|---|---|
| Approve | `/hero:approve` |
| Reject | `/hero:reject` |
| Cancel | `/hero:cancel` |
| Finish | `/hero:finish` |
| Archive (when exposed) | `/hero:archive` |
| Resume (when exposed) | `/hero:resume` |
| Help (when exposed) | `/hero:help` |
| Status as action | prefer `/hero:status`; `Go: *` screen jumps stay navigation |
| Dispatch | keep as harness dispatch affordance (hint “harness dispatch”); no invented `/hero:run` slash |

Empty-state hint: primary `/hero:new`; CLI secondary.

### D3 — Non-Hero command import (ADR-021)

- **Discovery package**: prefer `internal/adapters/cursor` (or small helper used by TUI) scanning:
  - `<project>/.cursor/commands/*.md`
  - `~/.cursor/commands/*.md` (`os.UserHomeDir`)
- **Exclude** filenames matching `hero-*.md` (Hero-owned).
- **Label**: `/` + stem (filename without `.md`).
- **Hint**: source (`project` vs `user`).
- **Invoke**: read file; strip leading YAML frontmatter (`---` … `---`) if present; `Dispatch(DispatchRequest{ProjectDir, Prompt: body})`.
- **Failure**: clear message to run the same command in Cursor chat (UI-C02-001 §3).
- **Skills**: never listed; Cursor Agent loads `.cursor/skills` when cwd = project.

### D4 — Harness marker detection (ADR-022) vs ADR-003 sync

**Marker table (versioned in code):**

| Marker dir | Tool id | Supported assets in C2 |
|---|---|---|
| `.cursor/` | `cursor` | yes |
| `.claude/` | `claude` | no (warn only) |
| `.windsurf/` | `windsurf` | no |
| `.codex/` | `codex` | no |

Shared pure function (e.g. `internal/doctor` or `internal/harness/detect`): scan project root → compare to `hero.json` → `cli.tools`.

**Call sites:**
- `hero install` — suggest when markers diverge / unsupported present.
- `hero doctor` — warn with UI-C02-001 §5 copy.
- `/hero:sync` (Runtime only) — after sync, instruct agent to run `hero doctor` so harness warnings surface. **Do not** add `hero sync` CLI (ADR-003).

### D5 — Persist `openspec_change` (ADR-023)

**Schema migration v2** on `cycles`:

```sql
ALTER TABLE cycles ADD COLUMN openspec_change TEXT NOT NULL DEFAULT '';
```

- Empty string = unset.
- `store.Cycle.OpenspecChange string`
- CLI API (deterministic):
  - `hero cycle openspec-change <name>` — set on active cycle (Planning records after propose).
  - `hero cycle openspec-change --clear` — clear.
  - `hero status` / `--json` includes `openspec_change`.
- Planning completion report already returns `sdd_path`; Runtime Planning handoff MUST call the setter with the change slug (e.g. `slash-parity-tui-harness`).

### D6 — Archive orchestration (ADR-023)

`cycle.Service.Archive` (or thin orchestrator wrapping it) becomes:

1. Resolve OpenSpec name:
   - If `openspec_change` non-empty → use it.
   - Else list `openspec/changes/*` excluding `archive/`:
     - 0 → skip OpenSpec step.
     - 1 → use that directory name.
     - N → fail closed with message requiring `hero cycle openspec-change <name>` or interactive selection flag (CLI: `--openspec-change <name>` required when N>1 and unset).
2. If name resolved: run `openspec archive <name> -y` (merge specs; no `--skip-specs` by default). Missing binary / non-zero exit = failure.
3. On OpenSpec success or skip → existing Hero filesystem+store archive.
4. On OpenSpec failure: **do not** archive Hero unless `--force` / `--skip-openspec`; print manual `openspec archive <name> -y`.
5. Runtime `/hero:archive`: on failure, offer retry | force (then CLI with `--force`) + manual instructions (UI-C02-001 §4).

Flags on `hero cycle archive`: `--force`, `--skip-openspec` (alias), optional `--openspec-change <name>` override for this invocation.

### D7 — Packages / slices (ADR-002)

| Concern | Package |
|---|---|
| `openspec_change` column + CRUD | `internal/store` |
| Archive orchestration + openspec-change CLI | `internal/cycle` |
| Marker detect + doctor warnings | `internal/doctor` (+ install hook) |
| Command discovery + frontmatter strip | `internal/adapters/cursor` (or `internal/tui` calling cursor helper) |
| Palette labels + import section | `internal/tui` |
| Dispatch payload | existing `internal/harness` + cursor adapter |
| Slash-first copy | `assets/cursor/**`, tests in `internal/common/runtime_assets_test.go` |

### D8 — Parallelism for implementation

After shared contracts (D3 discovery API shape, D5 column name, D6 archive flags) land in store/cycle stubs:

- **PARALLEL**: Runtime asset wording ‖ harness detection (doctor/install) ‖ TUI palette rename+import ‖ archive coupling tests can proceed on separate slices once store migration + cycle API exist.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| `openspec` missing on PATH | Treat as OpenSpec failure; force path + manual instructions |
| N active OpenSpec changes | Fail closed until name set/selected |
| Markdown expansion ≠ IDE slash UX | Document simulation; clear fallback to chat |
| PRD says `hero sync` CLI | Design follows ADR-003: Runtime `/hero:sync` + `hero doctor` |
| Overlapping `hero-1-0` change | New change only; no overwrite |

## Migration Plan

1. Schema v2 on open/upgrade/install (idempotent migrate).
2. Existing cycles get empty `openspec_change`; Planning/archive heuristics apply.
3. Asset refresh via upgrade/install embeds slash-first Runtime prompts.
4. No breaking CLI verb renames.

## Open Questions

None blocking — ADR-020–023 and PRD-C02-001 are accepted. Sync vs CLI resolved in D4 per ADR-003.
