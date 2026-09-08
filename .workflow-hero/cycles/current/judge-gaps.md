# C13 Judge loop-back — implementation gaps (iteration 4)

Source: Judge (`judge_agent`, `cursor-grok-4.6-high`) on `openspec/changes/claude-code-adapter`.
`all_requirements_met: false`. No SDD ambiguity. Re-run `generic_agent` (native scope).

SDD: `openspec/changes/claude-code-adapter/` (proposal, design, 12 specs, 42 tasks).
Authoritative product docs: `docs/product/PRD-C13-001-claude-code-adapter.md`,
`docs/architecture/ADR-C13-001-claude-code-adapter.md`,
`docs/product/UI-C13-001-tui-claude-code-adapter.md`.

Do not reimplement landed work. Keep and extend it.

## What already landed (keep)

- Protocol gate, registry/state/health, adapter core, NDJSON normalizer (text/thinking/tool/retry/hook/plugin/unknown), result assembly with NativeModel/EffectiveProperties.
- Execute starts a PermissionBridge, injects a one-time token, adds `--permission-prompt-tool`, fail-closes ask without TokenSource or OnPermissionRequest, maps auto-project/auto-all, deferred cleanup on Execute return.
- TUI conversation already supplies OnPermissionRequest/OnQuestionRequest; appendStreamDelta ignores permission/question kinds by design because the callback path owns the gate.
- In-memory native session rebind; `--resume` only when Execute is given that id; resume failure does not relaunch; TUI persistHarnessSession on stream SessionID; harnessSessionIDForPair rejects harness mismatch.
- SIGINT-first Cancel with 2s kill fallback.
- `assets/claude/` (agents, commands, workflow-hero/grilling skills) embedded in `assets.FS`.
- Marked `CLAUDE.md` insert/update/leave-unchanged, ProvisionClaude, EnableHarnessWithProjection, uninstall RemoveClaudeProjection/RemoveClaudeManagedContext, upgrade only when Claude is enabled.
- `assets/models/claude.yml` aliases plus full ids/`[1m]`; local ListModels; provider-namespaced C5; effort reject classified from stderr without silent strip/retry.
- TUI label `Claude`, enable copy `Claude enabled (projected .claude/)`, Claude omitted from `/harness-reset` picker, reset default `Unsupported harness.`.
- Doctor `claude_cli.go` / Status `claude.go`; `.claude/` Supported:true; unconfigured marker stays warn-only and is not auto-enabled.
- Docs/registries: architecture-overview, DEPLOY, TESTING, PRD/ADR/UI indexes, documents.json.
- Registry wires four adapters (`TestIntegration_DefaultRegistryWiresFourAdapters`).

## Required work (follow tasks.md execution order)

Finish the live ask transport. Starting a disconnected net.Pipe is not enough: production `execProcess` never implements `PermissionTransportProvider`, so Claude has no stdio to the bridge and cannot complete a permission-prompt round-trip.

1. task-04.1 remainder — Connect the execution-scoped stdio/MCP permission bridge to the real Claude child. Production Execute must give Claude a bidirectional permission-prompt channel (not only env vars + an in-process `net.Pipe` that the CLI never attaches). Keep one-time token, permission-only forwarding, and bounded cleanup on success, failure, and cancel. No credential/plugin/general MCP exposure.
2. task-04.2 remainder — Keep auto-project / auto-all mapping. Map ask to the proven live transport plus the bridge (not argv/env-only). Fail closed when the CLI cannot prove the transport. Never silently approve.
3. task-04.3 remainder — Once the live bridge emits permission/question callbacks, keep the existing TUI/Telegram contract (`OnPermissionRequest` / `OnQuestionRequest` → `harnessPermissionRequestMsg` / `harnessQuestionRequestMsg`), including watchdog Pause/Resume and cancel while waiting. Stream-kind emission alone is still ignored by `appendStreamDelta`.
4. task-03.3 remainder — Drive permission/question frames (and unknown-event warnings) through `resultAssembler`. `testdata/stream.ndjson` still has no permission/question records; keep using the permission JSON fixtures.
5. task-05.1 remainder — Reject Cursor/OpenCode/Codex session ids at the Claude adapter boundary before invoking `--resume` (TUI pair matching is not enough if Execute is given a foreign id). Add a Claude-specific test.
6. task-05.2 remainder — Bridge cleanup on cancel/abnormal exit for the live transport (not only Execute defer on a disconnected pipe). Kill-escalation coverage with the fake launcher.
7. task-07.2 remainder — Surface `system/init` effective native model (`ExecutionResult.NativeModel`) in the TUI turn identity. Adapter already records it; TUI never reads `NativeModel`.
8. task-10.1 remainder — Doctor unconfigured `.claude/` must not claim "unsupported in this Hero version — not installed". Warn only that the marker is not managed until `/harness` enables Claude. Status must populate permission-pause from the active turn (`ClaudeStatus.PermissionPaused` is always false).
9. task-11.1 / 11.2 remainder — After the live bridge exists, cover Telegram approval/rejection/timeout/cancel with the fake bridge (existing Telegram permission tests are harness-generic).
10. task-12.1 / 12.2 — Four-harness install/boot/model resolution/one execution/cancellation/fallback with Cursor+OpenCode+Codex+Claude enabled together; no adapter may receive another harness's session or process signals. Registry wiring alone is not enough. Record C13 acceptance scenarios in tests (protocol fixtures, live ask bridge, asset/`CLAUDE.md` preservation, catalog/C5/cost/fallback, Doctor/Status/Telegram, watchdog, cancel).
11. task-13.1 / 13.2 remainder — Golden normalized events including permission/question; fault injection for bridge replay, resume failure, process-group cancel, kill escalation. Do not add a live Claude account.

## Unmet spec requirements

- claude-permission-bridge: live Execute-scoped stdio/MCP round-trip to the Claude child. Codec, token, argv flag, and disconnected pipe exist; production `execProcess` never attaches a transport, so Claude cannot request or receive a decision.
- claude-adapter: adapter-level rejection of foreign resume ids before `--resume`.
- claude-model-catalog: TUI reports the runtime effective native model from `system/init`.
- harness-marker-detection / cli-deterministic-command-suite: unconfigured `.claude/` copy still calls Claude unsupported in this version; Status permission pause is unused.
- runtime-workflow-execution / hero-tui: mixed four-harness execution/cancel/fallback acceptance still missing (registry-only).
- Telegram-backed Claude permission decisions depend on the live bridge (generic TUI callback path is already present).

Tests must use fake processes, NDJSON fixtures, temporary directories, and injected clocks/launchers. No live Claude account, login flow, daemon, process registry, or credential storage.
