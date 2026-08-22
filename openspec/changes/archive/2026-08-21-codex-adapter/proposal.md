## Why

Hero 2.4.x executes AI from the TUI through **Cursor** and **OpenCode** only. Users who rely on OpenAI Codex (ChatGPT account, no API key) cannot assign `harness: codex` in cycle YAML. Cycle C6 delivers **Hero 2.5.0**: a third TUI harness (`CodexAdapter`) with OpenCode-level contract parity over Hero-managed `codex app-server` stdio/JSON-RPC (PRD-C06-001 §1; ADR-043–048).

**Behavioral spec:** `internal/adapters/opencode/` is normative. The idea file `docs/idea/v2.5_codex_adapter/codex_adapter.md` yields on divergence (ADR-045).

## What Changes

- Add **`CodexAdapter`** (`internal/adapters/codex`) implementing `HarnessAdapter` via Hero-managed `codex app-server` stdio/JSON-RPC (ADR-044).
- **OpenCode parity:** lazy app-server lifecycle, Execute/stream/cancel/sessions, `ListModels`, C5 property mapping, permissions, watchdog, `/harness-reset`, Prepare-on-`/hero-start`, unknown events → warning, never attach foreign process (ADR-045; PRD-C06-001 §4.3).
- **Codex-specific:** ChatGPT auth via `codex login` outside TUI (no API key); accept PATH `codex` binary (no version pin); native model ids only (ADR-047; PRD-C06-001 §4.4).
- **Projection:** `assets/codex/` → `.codex/` on enable; disable keeps files (ADR-046).
- **Install/`/hero-harness`:** third supported checkbox; upgrade 2.4.x does **not** auto-enable Codex (ADR-048).
- **`/hero-model`:** Codex in harness step when enabled; C5 property picker unchanged.
- **Catalog:** add Codex-native ids to `assets/models/*.yml` (no invented ChatGPT USD rates).
- **Doctor:** warn-only `codex-cli` when Codex enabled and CLI missing.
- **Compatibility mandate:** Cursor Adapter Execute, Cursor IDE slash Runtime, OpenCode serve/HTTP, C5 model properties, checksum upgrade, dual-entry, deterministic engine must not regress.

### In Scope

- Codex as third TUI harness only; Cursor IDE stays Cursor-only (ADR-043).
- Interactive install + `/hero-harness` third checkbox (PATH-independent).
- Project `hero.db` app-server registry + orphan reap (stdio analog of OpenCode serve registry).
- Chat speaker `[LABEL - model · codex]`; auth/CLI/app-server error copy (UI-C06-001).
- Template `workflow-config.yml` may show `harness: codex` examples.

### Out of Scope

- MCP management, MCP elicitation, MCP tool wiring (idea §15).
- Image / multimodal attachments (idea §18).
- Hero web-search configuration / `--search`; dump-all-events default Chat (idea §17, §24).
- Hero → Codex skill mapping and AGENTS.md conversion pipeline (idea §12–13).
- Codex plugin marketplace replication; `--yolo`; Hero sandbox/approval YAML.
- Embedding `codex login` or browser OAuth inside the TUI.
- Minimum Codex CLI version pin; attaching to user-started `codex app-server`.
- Cursor IDE multi-harness routing; Claude Code / VS Code harnesses.
- Canonical Hero model ids / cross-harness translation; `hero serve` daemon.
- Two concurrent `hero tui` in one project; Windows CLI; CI/CD releases; GPG signing.
- Auto-enabling Codex on upgrade; deleting `.codex/` on disable.

### CLI vs Runtime Classification (ADR-003)

- **CLI:** `internal/adapters/codex`, `internal/harness`, `internal/install`, `internal/upgrade`, `internal/tui`, `internal/store`, `internal/harnessmgr`, `assets/codex/`, `assets/models/`.
- **Runtime:** template `workflow-config.yml` comments; workflow-help/README copy; **no** Cursor IDE Codex routing.

## Capabilities

### New Capabilities

- `codex-adapter`: `CodexAdapter` with app-server lifecycle, JSON-RPC Execute/stream/cancel/session, auth detection, C5 property mapping, injectable stdio/process (ADR-044, ADR-045, ADR-047; PRD-C06-001 §4.2–4.4).
- `codex-app-server-registry`: SQLite registry (pid, harness, project_path, identity fields); orphan reap; stop on quit/disable (ADR-044).
- `codex-projection`: Provision `.codex/` from `assets/codex/` on enable; checksum rules; disable keeps files (ADR-046; PRD-C06-001 §4.7).
- `codex-model-catalog`: Codex-native model ids in `assets/models/*.yml`; context_window and C5 properties when known (PRD-C06-001 §4.8).

### Modified Capabilities

- `harness-adapter`: Third implementation (Codex); registry resolves by `codex`; session binding per harness; Codex property mapping (ADR-016 amended; ADR-045).
- `hero-tui`: Codex in install/`/hero-harness`/`/hero-model`; speaker `· codex`; auth/CLI errors; `/harness-reset` row (UI-C06-001).
- `sqlite-operational-store`: App-server registry rows for Codex; session harness binding for Codex thread ids (ADR-044).
- `asset-bootstrap-and-layout`: Codex projection paths; install/upgrade for enabled harnesses only.
- `runtime-workflow-execution`: Prepare-on-`/hero-start` when agents use `harness: codex` (OpenCode analog).
- `model-property-discovery`: Codex capability discovery via adapter/ListModels where supported.
- `model-property-catalog`: Codex-native ids with C5 property metadata.
- `cli-deterministic-command-suite`: Doctor warn-only Codex CLI check.
- `hero-json-harness-state`: `harnesses.codex.enabled` + model; upgrade preserves disabled (ADR-048).
- `interactive-harness-install`: Third checkbox Codex (UI-C06-001 §2).
- `hero-harness-command`: Codex enable/disable + projection (UI-C06-001 §3).
- `hero-model-pair`: Codex in harness step (UI-C06-001 §4).
- `tui-multi-harness-execution`: Execute routing to Codex adapter; never cross-resume sessions.
- `harness-fallback-chain`: Codex pair in two-step fallback; never invent third harness.

## Impact

- Packages: `internal/adapters/codex` (new), `internal/adapters/cursor` (minimal), `internal/adapters/opencode` (minimal), `internal/harness`, `internal/harnessmgr`, `internal/install`, `internal/upgrade`, `internal/tui`, `internal/store`, `internal/modelprops`, `assets/codex/`, `assets/models/`.
- External: `codex` CLI on PATH when Codex enabled; user runs `codex login` outside Hero.
- Tests: injectable stdio/process; no live LLM; install picker; upgrade 2.4 → Codex disabled; projection checksums; unknown event → warning; unauthenticated → login error; no cross-harness session resume; catalog lookup; `go test ./...`.
- Implementation agent: **generic_agent** (`scope.native: true`).
- Release: SemVer **2.5.0** minor (ADR-048).
- OpenSpec change: `codex-adapter`.
