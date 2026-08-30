# ADR-C06-001 — Codex Adapter (Hero 2.5.0)

> Cycle C6 ADRs for Hero **2.5.0**. Index: [ADR.md](ADR.md). Product: [PRD-C06-001](../product/PRD-C06-001-codex-adapter.md).

| # | Title | Status |
|---|---|---|
| [ADR-043](#adr-043-codex-is-a-third-tui-harness-cursor-ide-stays-cursor-only) | Codex is a third TUI harness; Cursor IDE stays Cursor-only | Accepted |
| [ADR-044](#adr-044-codex-via-hero-managed-app-server-stdio-json-rpc) | Codex via Hero-managed app-server (stdio JSON-RPC); project SQLite registry | Accepted |
| [ADR-045](#adr-045-opencodeadapter-is-the-behavioral-spec-idea-file-yields-on-divergence) | OpenCodeAdapter is the behavioral spec; idea file yields on divergence | Accepted |
| [ADR-046](#adr-046-codex-projection-assets-codex-to-codex) | Codex projection: `assets/codex/` → `.codex/`; enable provisions; disable keeps files | Accepted |
| [ADR-047](#adr-047-chatgpt-login-outside-tui-no-api-key-no-cli-version-pin) | ChatGPT login outside TUI; no API key; no CLI version pin | Accepted |
| [ADR-048](#adr-048-hero-250-opt-in-codex-minor) | Hero 2.5.0: opt-in Codex minor | Accepted |

**Amends:** [ADR-016](ADR-C01-001-hero-1-0.md#adr-016-harness-adapter-interface-cursor-only-impl) (third impl: Codex), [ADR-031](ADR-C04-001-multi-harness.md#adr-031-multi-harness-is-tui-only-cursor-ide-stays-cursor-only) (TUI harness set is Cursor + OpenCode + Codex), [ADR-036](ADR-C04-001-multi-harness.md#adr-036-single-asset-source-projections-enable-provisions-disable-keeps-files) (`.codex/` projection), [ADR-014](ADR-C01-001-hero-1-0.md#adr-014-cli-as-api-no-daemon-in-10) (adapter-owned subprocess, still not `hero serve`).

---

## ADR-043: Codex is a third TUI harness; Cursor IDE stays Cursor-only

**Context**: Users want OpenAI Codex in Hero cycles. Teaching Cursor IDE Runtime to dispatch Codex would couple two ecosystems and break the working slash path (same reason OpenCode stayed TUI-only in ADR-031).

**Decision**: Hero TUI is the **only** place Codex runs. Cursor IDE remains a **Cursor-only** compatibility path: it ignores `agents.*.harness`, never starts `codex app-server`, and never reads `.codex/` as a Hero execution backend. `harness: codex` is valid in YAML for TUI Execute. Do not replace `orchestration_agent` in the IDE with a Codex router.

**Consequences**:
- Dual-entry (ADR-015) remains: chat = Cursor; TUI = Cursor and/or OpenCode and/or Codex.
- YAML may carry `harness: codex` even when the user works in Cursor IDE (ignored there).
- Cursor and OpenCode adapters must not regress.

---

## ADR-044: Codex via Hero-managed app-server (stdio JSON-RPC); project SQLite registry

**Context**: Codex’s supported deep-integration surface is `codex app-server` (JSON-RPC), not automating the Codex TUI. Hero 1.x has no daemon (`hero serve` is D7). OpenCode already established adapter-owned long-lived children (ADR-035) with a project SQLite registry and “never attach foreign”.

**Decision**:
1. `CodexAdapter` starts **`codex app-server`** lazily on first Codex Execute (stdio/JSONL JSON-RPC). Do not spawn an ephemeral `codex exec` per prompt as the primary path.
2. Chat/stream/cancel/session use that child’s JSON-RPC (threads/turns/events/approvals). Protocol types stay in `internal/adapters/codex`.
3. Attach **only** to the child Hero started. Never attach to a user-started app-server, Unix socket, or WebSocket.
4. Registry lives in **this project’s `hero.db`**. On `hero tui` start, reap recorded Codex orphans that still match Hero-owned identity (unexpected TUI exit). Reuse the serve-registry pattern; stdio has no HTTP URL — persist pid, harness id, project path, and whatever identity fields OpenCode uses that still apply.
5. Normal quit and disabling Codex **stop** the child Hero created. Recreate on the next Codex Execute.
6. This is **not** `hero serve`. Engine still never speaks Codex JSON-RPC — only the adapter.
7. Process ownership follows the OpenCode lifecycle bar: graceful SIGTERM then kill, no silent spawn failure, no orphans, parent-death signal where the OS allows.

**Consequences**:
- SQLite schema version bump as needed for Codex rows + session harness binding.
- One TUI per project remains (second TUI would reap the first’s child).
- `IsAvailable` covers Codex CLI presence and a usable app-server; enabled ≠ available.

---

## ADR-045: OpenCodeAdapter is the behavioral spec; idea file yields on divergence

**Context**: The idea note `docs/idea/v2.5_codex_adapter/codex_adapter.md` is a full Codex integration wishlist (skills mapping, MCP, images, web search, dump-all events, sandbox YAML). Implementing it verbatim would fork Hero’s harness product model away from OpenCode. Grilling decided OpenCodeAdapter is the contract users already have.

**Decision**:
1. Treat `internal/adapters/opencode` as the **normative behavioral template** for Codex (HarnessAdapter methods, streaming, permissions, watchdog, reset, Prepare-on-start, projection lifecycle, doctor, catalog, TUI labels).
2. Treat the idea file as **non-normative**. On any conflict, OpenCode wins.
3. Explicitly out of C6 because they are idea-only or contradict OpenCode: MCP, image attachments, Hero web-search config, dump-all-events Chat, Hero→Codex skill/AGENTS.md conversion pipelines, Hero sandbox/approval YAML, plugin marketplace replication. The later per-harness `auto-all` profile is an explicit user-authorized exception for yolo behavior.
4. Codex-only gaps OpenCode cannot analogize (ChatGPT login, app-server stdio, native model catalog, capability detection for the installed CLI) are specified in [PRD-C06-001](../product/PRD-C06-001-codex-adapter.md) §4.4.

**Consequences**:
- Planning/SDD should trace OpenCode packages (`adapter.go`, `server.go`, `events.go`, `prepare.go`, `health.go`) rather than re-litigate the idea file.
- Unknown Codex events follow OpenCode: warning, never silent.
- Subagent/thread extras in the idea file ship only insofar as OpenCode already surfaces analogous stream deltas.

---

## ADR-046: Codex projection: `assets/codex/` → `.codex/`; enable provisions; disable keeps files

**Context**: OpenCode provisions `.opencode/` from `assets/opencode/` (ADR-036). Codex’s native project directory is also `.codex/` (user `config.toml`, hooks). Hero still needs a projection so enable/upgrade/checksum match OpenCode. Users may already have a `.codex/` tree.

**Decision**:
1. Canonical source remains `assets/` (`assets/codex/`). Enabling Codex **provisions** `.codex/` immediately (agents/commands/skills families mirroring OpenCode, plus a minimal config file only if the adapter requires it).
2. Disabling sets `enabled: false` and **does not delete** `.codex/`.
3. Last enabled harness cannot be disabled.
4. Root `AGENTS.md` is not copied into `.codex/`.
5. Checksum non-overwrite and conflict backup apply to **Hero-managed** projection files only. Unmanaged user files under `.codex/` are not deleted by disable, upgrade, or uninstall of Hero-owned paths.
6. Do not set `CODEX_HOME` to the project `.codex/` in a way that hijacks the user’s global `~/.codex` auth unless Planning proves it is required for app-server; default is: project cwd + projection files, user auth stays in Codex’s normal home.

**Consequences**:
- Install/upgrade/uninstall must list Codex-owned checksum paths without wiping user Codex config they did not install.
- Collision with an existing project `.codex/config.toml` uses the same conflict-backup rules as OpenCode customized files when the path is Hero-managed; if Hero does not ship that filename, leave the user’s file alone.

---

## ADR-047: ChatGPT login outside TUI; no API key; no CLI version pin

**Context**: Codex App Server is designed for Sign in with ChatGPT so users keep plan limits. Embedding `codex login` in Bubble Tea is brittle (browser/device flow). Pinning a Codex CLI version would fight rolling OpenAI releases; OpenCode also does not pin a CLI SemVer.

**Decision**:
1. Never prompt for or store an OpenAI API key for the Codex harness.
2. If Codex is not authenticated (or auth expired), Execute/`IsAvailable` (as appropriate) **fails explicitly** with guidance to run `codex login` in a regular terminal, then retry. Do not open a login UI inside Hero TUI.
3. Accept the `codex` binary on PATH. No minimum version constant. If `app-server` is missing or the JSON-RPC handshake fails → explicit error (incompatible/not installed), never hang or swallow.
4. `codex doctor` may be used as a diagnostic aid when implementing availability checks; user-facing copy must still name the actionable command (`codex login` / install Codex CLI).

**Consequences**:
- Doctor warn-only when Codex is enabled and CLI is missing (OpenCode analog).
- Tests inject auth/version failures; no live ChatGPT account in CI.

---

## ADR-048: Hero 2.5.0: opt-in Codex minor

**Context**: C4 was a breaking 2.0.0 (`--tools` removed, required `harness`). Adding a third opt-in harness does not break 2.4.x YAML or enabled Cursor/OpenCode projects.

**Decision**:
- Release as **SemVer 2.5.0** (minor).
- Upgrade 2.4.x → Codex **disabled**; no `.codex/` until install picker or `/hero-harness` enables it.
- Install picker adds Codex as a third supported checkbox (still ≥1 harness; PATH-independent).
- Fallback chain stays two-step (agent pair → `fallback_model` pair → stop). Codex may appear in YAML as agent or fallback; Hero still never invents a third pair.

**Consequences**:
- README, DEPLOY, UI install examples, workflow-help mention Codex as optional.
- `harnesses.codex` appears in `hero.json` when enabled; absent/false remains valid.
