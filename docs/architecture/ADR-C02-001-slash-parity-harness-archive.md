# ADR-C02-001 — Slash Parity, Harness Command Expansion & Archive Coupling

> Cycle C2 ADRs. Index: [ADR.md](ADR.md). Product: [PRD-C02-001](../product/PRD-C02-001-slash-parity-tui-harness.md).

| # | Title | Status |
|---|---|---|
| [ADR-020](#adr-020-user-facing-vocabulary-is-hero-slash-commands) | User-facing vocabulary is Hero slash commands | Accepted |
| [ADR-021](#adr-021-non-hero-commands-execute-via-markdown-prompt-expansion) | Non-Hero commands execute via markdown prompt expansion | Accepted |
| [ADR-022](#adr-022-harness-detection-filesystem-markers-vs-herojson) | Harness detection: filesystem markers vs hero.json | Accepted |
| [ADR-023](#adr-023-hero-archive-couples-openspec-archive-with-force-escape) | Hero archive couples OpenSpec archive with force escape | Accepted |

**Amends:** [ADR-007](ADR.md#adr-007-openspec-as-the-sdd-framework) (archive coupling), [ADR-015](ADR-C01-001-hero-1-0.md#adr-015-dual-entry-ui-chat-and-tui-parity) / [ADR-017](ADR-C01-001-hero-1-0.md#adr-017-bubble-tea-tui-claude-code-inspired) (TUI command naming), [ADR-016](ADR-C01-001-hero-1-0.md#adr-016-harness-adapter-interface-cursor-only-impl) (dispatch payload may be expanded command markdown).

---

## ADR-020: User-facing vocabulary is Hero slash commands

**Context**: Hero 1.0 Runtime prompts began emphasizing CLI-as-API verbs (`hero cycle new`, `hero approve`) in messages shown to users. The TUI palette used unrelated prose labels. Users expected 0.9 slash names (`/hero:start`, …) as the primary UX language across chat and TUI.

**Decision**: Treat the `/hero:*` slash set as the **canonical user-facing command vocabulary** for Runtime chat guidance and TUI labels. CLI verbs remain the deterministic API that agents and the TUI invoke internally. Do not rename CLI subcommands back to 0.9-only shapes solely for display parity.

**Consequences**:
- Runtime assets and TUI copy must be audited for slash-first wording.
- Clean-session handoff after `/hero:new` tells users to run `/hero:start`.
- Dual-UI parity (ADR-015) includes **naming** parity, not only capability parity.

---

## ADR-021: Non-Hero commands execute via markdown prompt expansion

**Context**: Users want the Hero TUI to surface Cursor custom commands from `.cursor/commands` (project) and `~/.cursor/commands` (global). There is no stable API to inject a slash into an open IDE chat. Cursor Agent CLI can sometimes accept `/name`, but behavior is inconsistent; a custom command’s semantic is the markdown file body.

**Decision**:
1. Discover non-Hero command files from project and user command directories.
2. On invoke, **read the `.md` contents** and dispatch that text as the agent prompt through `HarnessAdapter` with project cwd.
3. Do not list `.cursor/skills` in the TUI; rely on Cursor Agent auto-loading project skills when cwd is the project root.
4. Hero-owned `hero-*.md` commands are **not** executed via this expansion path — they map to TUI/CLI Hero actions (ADR-020).

**Consequences**:
- Reliable simulation of IDE slash semantics without IDE injection.
- Skills remain harness-owned; Hero does not re-implement skill discovery UI.
- Upstream CLI gaps (plugin skills, nested skill dirs) are accepted limitations, documented in PRD out-of-scope.

---

## ADR-022: Harness detection — filesystem markers vs hero.json

**Context**: Install currently requires explicit `--tools cursor`. Projects may already contain markers for other harnesses. Users want Hero to notice and suggest, without pretending unsupported harnesses are installed.

**Decision**: On `hero install`, `hero sync`, and `hero doctor`, detect known harness directories on disk and compare to `cli.tools` in `hero.json`. Emit suggestions/warnings. For markers without a Hero adapter/assets path, **warn only** — do not materialize assets or claim support (D1 remains deferred).

**Consequences**:
- Doctor gains actionable harness divergence warnings.
- No expansion of concrete adapters in C2.
- Marker list is versioned in code/SDD and can grow without changing product intent.

---

## ADR-023: Hero archive couples OpenSpec archive with force escape

**Context**: ADR-007 uses OpenSpec for SDD lifecycle (`/opsx:archive` historically separate). Users want `/hero:archive` to also finalize the cycle’s OpenSpec change. OpenSpec CLI supports `openspec archive <name> -y` (required non-interactive).

**Decision**:
1. Persist the OpenSpec change name on the cycle when Planning knows it; otherwise resolve by counting active `openspec/changes/*` (0 skip / 1 auto / N ask).
2. Default: run `openspec archive <name> -y` **before** `hero cycle archive` (merge delta specs; do not default to `--skip-specs`).
3. If OpenSpec fails, leave the Hero cycle unarchived unless the user forces via CLI `--force` / `--skip-openspec` and/or Runtime confirmation; then instruct manual OpenSpec archive.

**Consequences**:
- Amends ADR-007 operationally: Hero archive owns the coupled sequence; `/opsx:archive` remains available for manual/OpenSpec-only workflows.
- Archive becomes a multi-step deterministic CLI orchestration with an explicit escape hatch.
- Requires `openspec` on PATH when a change is linked; absence is treated as OpenSpec failure (force path available).

---

## Amendment notes

- **ADR-007**: Coupling does not replace OpenSpec; it invokes the official CLI.
- **ADR-016**: Cursor adapter remains Cursor-only; expansion dispatch uses existing best-effort Dispatch with clearer prompt payloads. IDE chat injection remains out of scope.
