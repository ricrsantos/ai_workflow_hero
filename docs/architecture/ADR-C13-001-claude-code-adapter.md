# ADR-C13-001 — Claude Code Adapter

> Cycle C13 decisions. Product: [PRD-C13-001](../product/PRD-C13-001-claude-code-adapter.md). UI: [UI-C13-001](../product/UI-C13-001-tui-claude-code-adapter.md).

## ADR-070: Claude is an opt-in TUI-only fourth harness

**Decision:** Add `claude` to the existing registry and adapter implementations, selected only by explicit `harness + model` and explicit enable. Cursor IDE remains Cursor-only; engine, stages, store abstraction, and two-step fallback are unchanged.

**Consequences:** Upgrade leaves Claude disabled/unprojected. Extension is an adapter plus projection, not an orchestrator or canonical-model layer. This extends ADR-031–033, ADR-036, and ADR-048.

## ADR-071: Use a supervised headless CLI subprocess per turn

**Decision:** `ClaudeAdapter.Execute` invokes `claude -p` with stream JSON and resumes native sessions. Its child belongs to one execution. SIGINT targets its process group; kill is fallback. Health observes that child and stream only.

**Consequences:** Hero has no Claude daemon, registry, or orphan reaper. Session mapping prevents cross-harness resumes. Minimum supported CLI is 2.1.261; missing/incompatible flags fail visibly.

## ADR-072: Preserve `ask` with a temporary MCP permission bridge

**Decision:** A one-turn local/stdio bridge, protected by a random one-time token, accepts only Claude permission requests, forwards normalized requests to the TUI, and returns the decision. A fixture/fake-process spike validates the permission-prompt protocol first.

**Consequences:** `ask` does not become denial-only or silently escalate. Unsupported protocol fails explicitly. The bridge stores no credentials or general model tools and dies with the execution context.

## ADR-073: Use `.claude/` projection plus a marked `CLAUDE.md` block

**Decision:** Embedded native commands, skills, and agents project to `.claude/`. A marker-delimited root `CLAUDE.md` block imports `@AGENTS.md`. Existing files require explicit insert/update-with-diff or preserve choices.

**Consequences:** Hero neither duplicates `AGENTS.md` nor overwrites user instructions. Checksum and disable/uninstall preservation rules stay consistent; only Hero's marked block can be updated/removed.

## ADR-074: Use local native Claude catalog and validated C5 mapping

**Decision:** `claude.yml` is a local native alias/ID catalog, not a live account probe. `ef` maps to `--effort` only when catalog-valid; `th`/`fs` remain unavailable. Runtime `system/init` is authoritative for effective model/capability.

**Consequences:** Pickers remain fast/offline, substitutions are visible, and subscription/unknown pricing is never invented. Existing overlays, metrics, and context bars require no special cost path.
