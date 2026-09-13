# Testing — AI Workflow Hero

> Test strategy and commands for the Hero CLI repository.

## Test command

```bash
go test ./...
```

Run from the repository root after any code change. All tests must pass before marking work complete.

Development requires **Go 1.26+** (`go.mod`). Rebuild `staticcheck` and `golangci-lint` with that same toolchain (`go install honnef.co/go/tools/cmd/staticcheck@latest` and `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`). Binaries built with Go 1.23 cannot analyze this module.

## Build artifact policy

- Tests, validation commands, and local test builds **MUST NOT** write executable or other binary files to the repository root. In particular, `./hero` and `./hero-telegram-daemon` are prohibited.
- Every temporary binary generated for a test, validation, or local build check **MUST** be written below `./temp/` using an explicit output path (for example, `go build -o ./temp/hero ./cmd/hero`). Never rely on Go's default output path.
- `./temp/` is scratch space. The test or script that creates an artifact **MUST** remove its own files after the run, including when the run fails; shell scripts should register cleanup on `EXIT`.
- Release artifacts intentionally produced by `scripts/build_dev.sh` or `scripts/release.sh` belong in `./dist/` and are not temporary test artifacts.

## Strategy

- **Unit tests**: colocated `*_test.go` in each `internal/<feature>/` package; same package; test behavior, not implementation details.
- **Golden tests**: template rendering and asset output fixtures.
- **Integration tests**: compiled `hero` binary against `t.TempDir()` for `install`, `upgrade`, `uninstall`, and `doctor`. Cover install without `--tools`, `--tools` error, 1.x-style upgrade leaving OpenCode disabled, 2.4-style upgrade leaving Codex disabled, OpenCode projection on enable, and Codex projection on enable (`.codex/` from `assets/codex/`).
- **Codex adapter tests**: injectable stdio/process (no live LLM or ChatGPT account); `IsAvailable` without CLI; unauthenticated → explicit `codex login` error; incompatible/missing `app-server` → explicit error; unknown JSON-RPC event → warning (never silent); session id never resumed across harnesses; ListModels native ids; permission request mapping to `OnPermissionRequest`.
- **Claude adapter tests**: fake CLI/process plus NDJSON fixtures (no live account); required-version/flag and authentication errors; incremental stream/result repair; official stream-json event mapping vs `hero --debug`-only observability; native session persistence and cross-harness guard; SIGINT/kill cancellation and child/bridge cleanup; supervised health/watchdog with permission pause; catalog/properties; unknown-event warning; and end-to-end `ask` bridge protocol/decision mapping.
- **OpenCode adapter tests**: SSE idle/gone/busy probes; serve-restart continue of a stuck tool turn; completed-turn recovery after restart; SSE blip without process restart must not abort or re-prompt.
- **C14 multimodal tests**: fixture-only image inputs and outputs for the shared harness contract, session-store permissions and SHA-256 dedupe, magic-byte/header and dimension limits, retention cleanup, capability intersection/admission, Free Chat attachment chips and image-only submit, async asset cards/mosaic, optional terminal preview protocols, and Codex/OpenCode/Claude/Cursor native/degraded/tool-written paths. No test invokes a real provider account or logs image bytes.
- **Model capability tests**: API-first model/capability discovery, adapter normalization, SQLite cache persistence, stale-cache fallback, local catalog fallback, dynamic value replacement, per-harness/model property persistence, and explicit harness rejection.
- **TUI property tests**: `/hero-model` background refresh, immediate cache/catalog rendering, boolean and multi-value property pickers, `ENTER to save`, Escape cancellation, gray unavailable labels, green configured labels, warning clearing, and responsive property-line rendering.
- **C8 TUI-direct stage Execute tests**: Planning/Judge handoff uses the stage agent pair; Implementation scope agents run concurrent Executes; nested generic Tasks chip `TASK`; named `context_agent` chips `CTX`; sibling `executeDone` does not clear the other stream; Codex/OpenCode Task start events carry `CallID` and agent/generic name.
- **C15 findings and ToDo lifecycle tests**: schema-v10 fixture migration to v11 without loss; constrained finding/occurrence/ToDo/adoption records; deterministic fingerprints and explicit `reopen_id`; atomic failed-stage close + finding persistence + loop-back for QA/Judge/Browser UI/E2E; Go recursive `./...` evidence accepted while `..` path segments are rejected; exact `task-*` + `find-*` assignment unions and field-specific report diagnostics; deferred-finding recurrence warnings; partial versus all-item escalation triage; recoverable/idempotent SQLite-to-`current-state.md` projection; `completed_with_deferred_todos`; Research startup adoption and release on non-validating terminal outcomes; legacy ToDo promotion; pending-only manual completion with required note; additive Status JSON (`internal/cycle/status_view_test.go`) and TUI assignment/stage-handoff coverage; canonical four-harness agent/command projection parity. TUI Status table rendering and Telegram C15 status text are not yet covered here. Use real temporary SQLite databases and `t.TempDir()` files; inject failures at every transaction/projection boundary.
- **C16 durable session-history tests**: copied schema-v11 fixture migration to v12 with row-for-row preservation and idempotent legacy binding import; create-on-first-turn and deterministic titles; ordered incremental event append from local/Telegram and concurrent stream callbacks; bounded transcript paging; active/archive/name-search ordering; rename/archive/restore/delete semantics; original-versus-managed asset deletion and partial-cleanup retry; exact harness/model/property resume and explicit context fork; confirmed idempotent remote import; unsupported/failed remote delete warning after local success; interrupted execution recovery; lease acquire/heartbeat/release/stale takeover using injected clocks; independent parallel-agent sessions; and completed-stage continuation without workflow-state mutation. Use real temporary SQLite/filesystems and fake adapter capabilities only—no live provider, network, wall clock, or user home.
- **C16 History TUI tests**: navbar order and dynamic Alt ranges in project/freechat/active-cycle modes; list/detail focus, search, active/archive switching, rename and destructive dialogs; busy-action disabling; async commands rather than I/O in `Update`; wide/stacked/narrow resize behavior; ANSI/rune-safe truncation; empty/loading/error/interrupted/legacy states; transcript reconstruction including Telegram origin and image cards; and scroll-to-latest on open. Follow the `golang-tui` and `go-engineering` skill guidance and prefer behavior assertions over large ANSI snapshots.
- **Harness session isolation tests**: schema v10 orchestrator pair on `cycles`; stage-agent OpenCode `ses_…` must not replace the Cursor orchestrator UUID; empty harness owner does not resume; Cancel uses the execute's session, not a global foreign id; Cursor Execute rejects non-UUID `--resume`.
- **Telegram plugin tests**: injected Bot API/vault/clock/IPC/process launcher; one-chat pairing and 10-minute expiry; redaction; private-socket registration; stable project/free-chat address allocation; numbered `/list` and persistent `/select n` routing, including selected-instance disconnection; daemon `/help` command catalog without selection or harness forward; TUI `/status` and project-local `0`/`1–300` minute auto-reporting (wall-clock / last-sent guard against stale ticks; queued remote turns send status once and drain after Execute); active-agent/model rows in status (`harness` for Free Chat); `/interrupt` Ctrl+C-equivalent cancellation; `/kill` IPC-goroutine force exit with injectable `SIGKILL` and best-effort outbound; exactly-once update de-duplication; pending delivery/24-hour expiry/cancellation; daemon restart backoff; notification filtering; transcript labels; log rotation and old-path migration; plugin install/upgrade and `.gitignore` preservation; daemon pidfile ownership, unknown-frame errors, registration version/capabilities, idempotent client close, atomic daemon/manifest installation, and stale deleted-inode daemon recovery. No test contacts Telegram or an OS credential vault.
- **Development auto-update tests**: host-targeted build of both Hero and the Telegram daemon; optional installed-plugin update of daemon plus manifest; old-daemon shutdown before TUI restart; bounded restart IPC when a daemon does not reply; direct `SIGUSR2` discovery of installed and deleted-inode TUIs; successful coupled installation clears `needs-update.txt` and preserves `hero.previous`; failed build/manifest/installation preserves the update request; systemd timeout and shell-script contracts. Tests use temporary directories and clean every generated executable.
- **Telegram cycle-config wizard tests**: address-scoped in-memory drafts; title/objective/language/scope/stage progression and validation; canonical-vs-draft `/hero-config-show`; explicit save/cancel behavior; numbered model/property reuse; atomic YAML persistence and cycle synchronization; isolation from free-chat `hero.json`.
- **C7 Config screen tests**: conditional Config navigation (active-cycle only), round-trip YAML golden fixtures preserving comments/order/unknown keys/`workflow_rules`, managed-field merge after parallel edits, atomic-write failure safety, field-level validation, responsive form states, dirty-exit confirmation, read-only execution state, harness/model/property filtering, Save/Save and start routing, cycle synchronization, completed-stage protection, and explicit failed-stage retry with counter reset and preserved events/metrics.
- **Dependencies**: prefer real filesystem and `embed.FS` over mocks; keep tests deterministic and fast.

## Coverage areas

| Area | Packages / paths |
|---|---|
| Install / upgrade / uninstall | `internal/install`, `internal/upgrade`, `internal/uninstall` |
| Doctor / status | `internal/doctor`, `internal/status` |
| Cycle / store / engine | `internal/cycle`, `internal/store`, `internal/engine` |
| TUI / harness | `internal/tui`, `internal/harness`, `internal/adapters/cursor`, `internal/adapters/opencode`, `internal/adapters/codex`, `internal/adapters/claude` |
| Model properties / metadata cache | `internal/tui`, `internal/harness`, `internal/harnessmgr`, `internal/store`, `internal/adapters/cursor`, `internal/adapters/opencode`, `internal/adapters/codex`, `assets/models/` |
| Templates / assets | `internal/common/template`, `assets/` |
| Multimodal media | `internal/media`, `internal/harness`, `internal/conversation`, `internal/tui`, `internal/adapters/{codex,opencode,claude,cursor}` |
| Findings / ToDos / loop-back | `internal/store`, `internal/engine`, `internal/cycle`, `internal/tui`, `internal/status`, `internal/todos`, `internal/conversation`, `internal/telegram`, `assets/` |
| Durable session history | `internal/conversation`, `internal/store`, `internal/tui`, `internal/harness`, `internal/adapters/{codex,opencode,claude,cursor}`, `internal/media`, `internal/telegram` |
| Release contract | `scripts/release_test.go` |

## CI

No CI workflow is required for V1 (manual release via `scripts/release.sh`). CI/CD automation is deferred to V2 (see PRD §2.3).
