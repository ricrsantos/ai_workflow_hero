# PRD-C06-001 — Codex Adapter (Hero 2.5.0)

> Cycle C6 product requirements. Ships **Hero 2.5.0**: a third TUI harness (`CodexAdapter`) with OpenCode-level contract parity. Cursor IDE Runtime stays Cursor-only. Index: [PRD.md](PRD.md). Idea notes (non-normative; yield to OpenCodeAdapter on divergence): [docs/idea/v2.5_codex_adapter/codex_adapter.md](../idea/v2.5_codex_adapter/codex_adapter.md).

## 1. Overview

Hero 2.4.x executes AI from the TUI through **Cursor** and **OpenCode** only. Users who already use OpenAI Codex (ChatGPT account, no API key) cannot assign Codex as a cycle harness.

This cycle delivers **Hero 2.5.0**:

1. **Codex as a third TUI harness** — `harness: codex` + a Codex-native `model` id, executed by `CodexAdapter`.
2. **OpenCodeAdapter is the behavioral spec** — lifecycle, Execute/stream/cancel, sessions, ListModels, permissions, watchdog, `/harness-reset`, Prepare-on-`/hero-start`, event mapping, never-silent-unknown, projection, install/`/hero-harness` opt-in. Where [the idea file](../idea/v2.5_codex_adapter/codex_adapter.md) diverges from OpenCode, **OpenCode wins**.
3. **Codex-specific transport** — Hero-managed `codex app-server` over stdio/JSON-RPC (JSONL). Protocol details stay inside the adapter.
4. **ChatGPT authentication** — no API key prompt; unauthenticated Codex fails with a clear instruction to run `codex login` outside the TUI.
5. **Minor release** — Cursor and OpenCode behavior stay intact; Codex is opt-in (never auto-enabled on upgrade).

**Compatibility mandate:** do not regress Cursor Adapter CLI Execute, Cursor IDE slash Runtime, OpenCode serve/HTTP, C5 model properties, checksum upgrade, dual-entry, or the deterministic engine.

## 2. Goals

- One TUI cycle can run agents on Cursor, OpenCode, and/or Codex.
- User always sees **agent + model + harness** (`codex` as lowercase id).
- Harness/model switches are never silent (existing two-step fallback, ADR-033).
- Adding Codex means a new adapter + projection, not engine/stage changes.
- Existing Cursor/OpenCode projects keep working after `hero upgrade` (Codex stays disabled until selected).

## 3. Terminology

| Term | Definition |
|---|---|
| **Enabled** | User selected this harness for the project (`hero.json`). Independent of PATH. |
| **Available** | Adapter `IsAvailable` is true (Codex CLI present and `app-server` usable). |
| **Native model id** | The model string Codex itself uses. No Hero translation table. |
| **Projection** | Harness-specific files from Hero `assets/codex/` into project `.codex/`. |
| **App Server** | Long-lived `codex app-server` child owned by `CodexAdapter` (stdio JSON-RPC). |
| **Idea file** | `docs/idea/v2.5_codex_adapter/codex_adapter.md` — inspiration only. |

## 4. In Scope (C6 / Hero 2.5.0)

### 4.1 Compatibility (do not regress)

- Cursor Adapter `Execute` / stream-json / cancel / sessions keep working.
- OpenCode Adapter serve lifecycle, HTTP/SSE, PrepareHeroStart, `/harness-reset` keep working.
- Cursor IDE `/hero-*` Runtime keeps working without learning Codex.
- With Codex disabled, TUI behavior matches 2.4.x aside from Codex appearing as an **unchecked** option in install/`/hero-harness`.
- `hero upgrade` does not overwrite customized assets (existing checksum rules).
- Engine remains deterministic; CLI still performs no LLM reasoning (ADR-003).
- **No `hero serve` daemon** (ADR-014). Codex’s process is owned by `CodexAdapter`.

### 4.2 HarnessAdapter + CodexAdapter

- Keep `internal/harness.HarnessAdapter` as the only execution boundary.
- Add `internal/adapters/codex` implementing the same contract as OpenCode (`Name`, `IsAvailable`, sessions, Execute, Cancel, Status, Dispatch, `ListModels`, property mapping, health/watchdog hooks used by the TUI).
- Cursor and OpenCode adapters stay; **do not** rewrite them to go through Codex.
- Codex-specific protocol (app-server lifecycle, JSON-RPC, native model ids, ChatGPT auth detection) stays inside the Codex adapter.
- Core / engine / TUI must not import Codex JSON-RPC types.

### 4.3 Behavioral parity with OpenCodeAdapter

The Codex adapter MUST provide the OpenCode-equivalent of:

- Managed child process: spawn, stdin/stdout/stderr, PID, health, cancel, graceful stop, force kill, restart, timeout, orphan detection/reap.
- Lazy start on first Codex Execute in the TUI session; stop on TUI quit and on `/hero-harness` disable Codex.
- Attach **only** to the app-server **this Hero process created**. Never attach to a foreign Codex process or socket.
- Persist registry in **project `hero.db`** (pid and identity; stdio has no serve URL — store what the OpenCode registry analog needs for reap/stop without inventing a HTTP URL).
- Stream mapping to Hero `StreamDelta` (text, thinking, tool, warning, permission, activity, session). Unknown Codex events → **warning**, never silent drop.
- Permission/approval requests → existing TUI `Allow? [y/N]` (`OnPermissionRequest`) unless the selected per-harness profile is `auto-all`, which uses Codex's native non-interactive approval policy and danger-full-access sandbox.
- Session binding: Hero `harness_session_id` ↔ Codex thread id; never resume a Cursor/OpenCode session as Codex.
- `/harness-reset` includes Codex when enabled (stop Hero-managed app-server; warn if not started).
- Prepare-on-`/hero-start` when any agent uses `harness: codex`: sync projection agent definitions from workflow-config (OpenCode analog), reset managed app-server, probe one configured agent. Probe failure stops `/hero-start` with instructions to exit TUI and retry (same copy pattern as OpenCode).
- Doctor: warn-only Codex CLI check when Codex is enabled (PATH), complementary to OpenCode’s warn-only CLI check.
- C5 properties: adapter-owned mapping of `fs` / `th` / `ef`; unsupported → `na`; harness rejection is explicit.

OpenCode features that do **not** exist as Hero product surface stay absent for Codex too (no new Hero sandbox YAML, no web-search config, no Hero skill-copy pipeline, no root `AGENTS.md` duplication into `.codex/`).

### 4.4 Codex-specific requirements (no OpenCode analog)

These do not contradict OpenCode; they fill gaps OpenCode does not have:

1. **Transport:** `codex app-server` over stdio/JSONL JSON-RPC (initialize / initialized / thread / turn). Prefer the CLI’s stdio app-server invocation for the installed binary; do not require a Hero-pinned Codex version.
2. **Version:** accept whatever `codex` is on PATH. If the binary is missing, `app-server` is missing, or the handshake is incompatible → **explicit error** (code/message; never silent).
3. **Auth:** detect ChatGPT/Codex login state. If not authenticated (or expired) → error that tells the user to run `codex login` in a normal terminal. Never prompt for an API key. Never embed interactive login inside the TUI.
4. **Capabilities:** detect app-server features at startup where practical; degrade safely (e.g. subagent events if the installed CLI emits them). Do not use experimental Codex features without detection.
5. **Usage:** map Codex usage/quota into Hero `Usage` when the App Server provides it. ChatGPT plans may lack USD prices — do not invent rates; catalog miss → cost unset/zero with warning (same spirit as C4 unknown ids).

### 4.5 `workflow-config.yml`

No new required keys. Agents already declare `harness` + native `model` (ADR-032). `harness: codex` is a valid id once the adapter is registered.

- `/hero-new` injects missing `harness` from **enabled** project harnesses (extend C4: one enabled → that id; multiple including Cursor → `cursor`; never inject a disabled harness).
- Invalid Codex model id → unavailable → existing fallback chain (ADR-033). Never invent a fourth harness beyond the YAML fallback pair.

### 4.6 Install, upgrade, `/hero-harness`

- `hero install` lists **supported** harnesses: Cursor, OpenCode, **Codex**. User selects ≥1. Selection does not depend on PATH.
- Enable Codex = mark enabled **and provision** `.codex/` immediately from `assets/codex/` (ADR-036 analog).
- Disable = `enabled: false` only. **Do not delete** `.codex/`.
- Cannot disable the last enabled harness.
- `hero upgrade` from 2.4.x: **do not** auto-enable Codex; do not write `.codex/` until enable.
- Uninstall/checksum rules: Hero-managed files under `.codex/` follow the same backup/replace as `.opencode/`. Files the user added that Hero does not checksum are not deleted by disable/upgrade.

### 4.7 Projection (`assets/codex/` → `.codex/`)

- Same lifecycle as OpenCode: single source `assets/`; enable provisions; disable keeps files; checksum non-overwrite; conflict backup `{filename}_{timestamp}.conflict` for customized Hero-managed files.
- Project the same **families** OpenCode projects (agents, commands, skills, plus a minimal harness config file only if the adapter requires it — analog of `opencode.json`).
- Root `AGENTS.md` is **not** copied into `.codex/`.
- Do not implement a separate Hero-Skill → Codex-Skill mapping or versioning pipeline (idea file §12–13 yield to OpenCode).

### 4.8 `/hero-model` and catalog

- `/hero-model` lists Codex when Codex is enabled (same two-step pair picker).
- Native Codex ids only. No cross-harness translation.
- **Mandatory catalog task:** add `assets/models/` entries for Codex-native ids (pricing when known; `context_window`; C5 `properties` when known). Do not invent USD rates for ChatGPT-subsidized models; unknown cost stays unset/zero with a clear warning.
- `hero update-models` documentation includes Codex ids alongside Cursor and OpenCode.

### 4.9 TUI Chat chrome

- Speaker / agents box / status: harness id `codex`.
- Auth, missing CLI, app-server crash, timeout, and JSON-RPC errors use existing red/yellow conventions (see [UI-C06-001](UI-C06-001-tui-codex-adapter.md)).
- Event mapping follows OpenCode (known deltas on the green pane; unknown → warning). Idea-file “dump every event on screen in v1” is **out**.

### 4.10 Persistence / `hero.json` / SQLite

- `harnesses.codex.enabled` and `harnesses.codex.model` (plus C5 `model_properties` keyed by harness+native id).
- Freechat default pair may be `(codex, <native id>)` when the user selects it.
- Schema bump as needed for Codex process registry and session↔harness binding (a Codex thread id must never be resumed as Cursor/OpenCode).
- One TUI per project remains in force (second concurrent TUI out of scope, same as C4).

### 4.11 Docs / Runtime assets

- Template `workflow-config.yml` comments/examples may show `harness: codex`.
- `workflow-help.md`, README (EN + PT-BR): Codex as optional TUI harness, `codex login`, projection `.codex/`.
- Cursor Runtime assets remain Cursor-only.

## 5. Out of Scope

- MCP management, MCP elicitation, and MCP tool wiring in the adapter (idea §15).
- Image / multimodal attachments (idea §18).
- Hero web-search configuration / `--search` (idea §17). Dump-all-events debug mode as the default Chat view (idea §24).
- Dedicated Hero → Codex skill mapping and AGENTS.md conversion pipeline (idea §12–13).
- Replicating the Codex plugin marketplace (idea §16). Existing user Codex plugins/config must not be willfully destroyed; Hero only manages checksummed projection files.
- Embedding `codex login` or a browser OAuth flow inside the TUI.
- Requiring a minimum Codex CLI version (accept PATH; fail explicit if incompatible).
- Cursor IDE Runtime becoming a Codex/multi-harness orchestrator.
- Claude Code, VS Code, or other new harnesses.
- Canonical Hero model ids / cross-harness name translation.
- `hero serve` / daemon / RPC (D7).
- Attaching to user-started or foreign `codex app-server`.
- Two concurrent `hero tui` processes in one project.
- Windows CLI; CI/CD-automated releases; GPG signing.
- Auto-enabling Codex on upgrade.
- Deleting `.codex/` on disable.
- New Hero YAML for Codex sandbox / approval policy (use Codex defaults + TUI permission prompts, like OpenCode).

## 6. Non-Functional Requirements

- Feature Based + Vertical Slice: `internal/adapters/codex`, plus install/upgrade/TUI/store/harnessmgr registration; **minimize** Cursor and OpenCode adapter diffs.
- Tests: no live LLM; injectable process/stdio (OpenCode HTTP-injection analog); install picker includes Codex; upgrade 2.4 → Codex disabled; projection checksums; unknown event → warning; unauthenticated → login error; missing CLI → IsAvailable error; no cross-harness session resume; catalog lookup for a Codex id.
- Platforms: Linux and macOS, `amd64` and `arm64`.
- Cycle artifacts in English; chat follows `workflow_config.user_preferred_language`.

## 7. Release (Hero 2.5.0)

- **SemVer minor** 2.5.0 (not a breaking major). Adding an opt-in harness is compatible with 2.4.x configs.
- Version injection remains `-ldflags` / `scripts/release.sh` (DEPLOY.md).
- Manual release process unchanged (ADR-010).

## 8. Verification and acceptance

The implementation is accepted when:

1. TUI Execute with `harness: codex` runs a turn through Hero-managed `codex app-server` and streams mapped events.
2. Unauthenticated Codex fails with an explicit `codex login` instruction (no API key prompt).
3. Missing or incompatible `codex` / `app-server` fails explicitly.
4. `/hero-harness` can enable Codex (projects `.codex/`) and disable it (files kept).
5. `hero upgrade` from 2.4.x does not enable Codex.
6. Cursor IDE slash Runtime still ignores `harness` and never starts Codex.
7. OpenCode and Cursor adapters remain green under the existing suite.
8. Unknown App Server events produce warnings, not silent ignores or panics.
9. Ask and project-scoped profiles use the existing TUI Allow? [y/N] path as needed; `auto-all` completes native approvals without TUI prompts.
10. `go test ./...` passes.

## 9. Reference

- Behavioral template: `internal/adapters/opencode/`
- Idea (non-normative): `docs/idea/v2.5_codex_adapter/codex_adapter.md`
- Prior multi-harness: [PRD-C04-001](PRD-C04-001-multi-harness.md), [ADR-C04-001](../architecture/ADR-C04-001-multi-harness.md)
