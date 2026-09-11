# Implementation Tasks: TUI Multimodal Images

Every task has an executable verification criterion. Artifacts stay English.
`[SERIES]` = ordering constraint. `[PARALLEL]` = independent `generic_agent` fan-out after prerequisites.
Scope: **native** → all tasks use `[agent:generic_agent]`. Apply `go-engineering` and `golang-tui` skills.

## Execution order and fan-out

```text
task-01 multimodal contract
  -> PARALLEL: task-02 asset store | task-03 capability | task-04 mosaic
  -> task-05 conversation/execute plumbing (after 01; uses 02+03)
  -> PARALLEL after 05 (+02/03 as needed):
       task-06 Codex | task-07 OpenCode | task-08 Cursor | task-09 Claude spike→adapter
       task-10 composer UX | task-11 asset cards (needs 04)
  -> task-12 tool-written detection integration (after adapters + cards)
  -> task-13 advanced inline preview (after 11; optional protocols default off)
  -> PARALLEL: task-14 docs/registry | task-15 regression suite
  -> task-16 final verification
```

Keymap decision (Planning): keep `Alt+A` and `Alt+V` (unused in `internal/tui`).
Claude decision (Planning): series fixture spike; native if proven without account; else labeled file-reference degraded path in this cycle.

## 1. Multimodal contract — [SERIES]

- [x] 1.1 [task-01.1-types] [agent:generic_agent] Add `Attachment`, `Asset`, `MediaKind`, `MediaCapability` in `internal/harness` with documented fields from PRD-C14-001 §2.1 / ADR-077; cover JSON/stable field tests.
- [x] 1.2 [task-01.2-stream-result] [agent:generic_agent] Extend `ExecuteRequest.Attachments`, `ExecutionResult.Assets`, `StreamDelta.Asset`, and `StreamKindAsset`; assert result repair helper merges stream+final assets by content hash without duplicates.
- [x] 1.3 [task-01.3-conversation-input] [agent:generic_agent] Extend `conversation.Input` with `Attachments` and allow image-only inputs; unit-test classification/submit handoff does not require text when attachments exist.

## 2. Session asset store and validation — [PARALLEL after task-01]

- [x] 2.1 [task-02.1-store] [agent:generic_agent] Implement session asset materialization under `XDG_DATA_HOME`/`~/.local/share/hero/sessions/<session-id>/assets/` with UUID filenames, `0700`/`0600`, SHA-256 dedupe, and JSON manifest (PRD §2.2; ADR-080).
- [x] 2.2 [task-02.2-validate] [agent:generic_agent] Implement magic-byte + header validation, limits (5 / 20MB / 4096 / 100MP), symlink canonicalize, external-path warning signal, decompression-bomb abort; table-driven tests with tiny fixtures (PRD §2.3).
- [x] 2.3 [task-02.3-retention] [agent:generic_agent] Implement retention cleanup (default 7d) hooks for startup/shutdown; test expired vs retained temp dirs; ensure no bytes logged.

## 3. Media capability registry — [PARALLEL after task-01]

- [x] 3.1 [task-03.1-registry] [agent:generic_agent] Define capability registry combining adapter transport ∩ model discovery/catalog; unknown ⇒ unsupported; fixture tests for intersection logic (PRD §2.4; ADR-079).
- [x] 3.2 [task-03.2-admit] [agent:generic_agent] Implement pre-Execute admission for attachment turns; return actionable harness+model error; verify fallback is not silently invoked and attachments are not stripped.

## 4. Unicode mosaic renderer — [PARALLEL after task-01]

- [x] 4.1 [task-04.1-mosaic] [agent:generic_agent] Implement async Unicode block+ANSI mosaic renderer as injectable pure function + `tea.Cmd` wrapper; respect width/height; degrade <256 colors (PRD §2.7; ADR-081).
- [x] 4.2 [task-04.2-mosaic-tests] [agent:generic_agent] Add invariant/golden tests on tiny fixtures (dimensions, non-empty output, no panic on resize bounds); forbid huge ANSI snapshots.

## 5. Execute plumbing — [SERIES after task-01; uses task-02 and task-03]

- [x] 5.1 [task-05.1-wire-input] [agent:generic_agent] Wire Free Chat submit path to validate/materialize attachments, run capability admission, and populate `ExecuteRequest.Attachments` in images-then-text order; unit-test with fake adapter (PRD §§2.1,2.4,2.5).
- [x] 5.2 [task-05.2-wire-output] [agent:generic_agent] Wire stream `StreamKindAsset` + final `ExecutionResult.Assets` into conversation/TUI update messages without blocking `Update`; race-focused test that workers do not mutate model maps directly (ADR-078).

## 6. Codex adapter multimodal — [PARALLEL after task-05]

- [x] 6.1 [task-06.1-codex-schema] [agent:generic_agent] Probe App Server schema for `localImage` / `imageGeneration` / `imageView`; explicit degrade when absent; fake-process tests (PRD §2.9).
- [x] 6.2 [task-06.2-codex-input] [agent:generic_agent] Map attachments to ordered `turn/start.input` `localImage` entries before text; argv/RPC fixture assertions.
- [x] 6.3 [task-06.3-codex-output] [agent:generic_agent] Normalize `imageGeneration`/`imageView` to `Asset` + stream deltas + final repair; preserve savedPath/status/safe errors.
- [x] 6.4 [task-06.4-codex-tool-assets] [agent:generic_agent] Detect tool-written images via structured items + path extraction; dedupe by hash; fixture coverage (ADR-082).

## 7. OpenCode adapter multimodal — [PARALLEL after task-05]

- [x] 7.1 [task-07.1-opencode-input] [agent:generic_agent] Prefer `file:` absolute URI parts; `data:` only when serve host cannot read file; validate provider/model limits at admission (PRD §2.10).
- [x] 7.2 [task-07.2-opencode-output] [agent:generic_agent] Extend SSE normalizer for file/image parts and tool-result images → `Asset` stream/final.
- [x] 7.3 [task-07.3-opencode-tool-assets] [agent:generic_agent] Correlate tool-result/file.watcher image writes into tool-sourced assets with hash dedupe; fake SSE fixtures (ADR-082).

## 8. Cursor adapter multimodal — [PARALLEL after task-05]

- [x] 8.1 [task-08.1-cursor-file-ref] [agent:generic_agent] Compose file-reference prompts for materialized attachments; fail closed with diagnostic when model/harness cannot read images; never silent-drop (PRD §2.12).
- [x] 8.2 [task-08.2-cursor-tool-watch] [agent:generic_agent] Turn-scoped workspace watch + stream-json tool path extraction for tool-written images; hash dedupe; fake FS + stream fixtures (ADR-082).

## 9. Claude spike and adapter — [SERIES internally; PARALLEL package after task-05]

- [x] 9.1 [task-09.1-claude-spike] [agent:generic_agent] Fixture/fake-process spike for `--input-format stream-json` image wire format without a real account; record pass/fail in test output (PRD §2.11).
- [x] 9.2 [task-09.2-claude-native-or-degraded] [agent:generic_agent] If spike passes: implement native stdin attachment path. If spike fails: implement labeled file-reference degraded path and explicit failure when neither path works; never silent-drop.
- [x] 9.3 [task-09.3-claude-tool-assets] [agent:generic_agent] Tool-written detection via tool_result paths + FS watch on degraded path; hash dedupe; fixtures only (ADR-082).

## 10. Free Chat composer UX — [PARALLEL after task-05]

- [x] 10.1 [task-10.1-keys-slash] [agent:generic_agent] Bind `Alt+A`/`Alt+V`, `/attach`, `/attach-clipboard`, `/attach <path>` in Free Chat only; update footer hints; assert Research/workflow ignore attach (UI-C14-001 §§2,7).
- [x] 10.2 [task-10.2-picker-clipboard-paste] [agent:generic_agent] Add Bubbles file picker (image filters), native clipboard image read Cmd (not OSC 52), bracketed-paste path offer; spinner chips while validating (UI §§4,6).
- [x] 10.3 [task-10.3-chips] [agent:generic_agent] Render chips between text and status; independent dismiss; external warning badge; error chips; image-only submit; height accounting; Bubble Tea tests (UI §2; ADR-078).

## 11. Asset cards + mosaic integration — [PARALLEL after task-04 and task-05]

- [x] 11.1 [task-11.1-cards] [agent:generic_agent] Render transcript asset cards with metadata and keyboard actions Enter/o/c/a/s; open suspend/resume; copy path; save dialog; attach-to-composer; all via `tea.Cmd` (UI §§3,5–6).
- [x] 11.2 [task-11.2-card-mosaic] [agent:generic_agent] Integrate mosaic toggle under cards; resize re-render async; low-color degradation copy; session-ephemeral cards (no restart rebuild) (PRD §§2.7,2.14).

## 12. Cross-adapter tool-asset UX integration — [SERIES after tasks 6–11]

- [x] 12.1 [task-12.1-tool-cards] [agent:generic_agent] Ensure tool-sourced assets from each adapter appear as `[tool wrote]` cards after text; multi-image order stable; dedupe across detection paths (UI §3.4; ADR-082).

## 13. Advanced inline preview — [SERIES after task-11]

- [x] 13.1 [task-13.1-protocols] [agent:generic_agent] Implement optional Kitty/Sixel/iTerm2 preview behind config flag default off; best-effort detect + override; clear on scroll/resize; card remains navigable (PRD §2.8; ADR-081).
- [x] 13.2 [task-13.2-protocol-tests] [agent:generic_agent] Fixture tests proving default-off behavior and that enabling emits protocol sequences only when flagged.

## 14. Docs and registry — [PARALLEL after task-05; finalize after features land]

- [x] 14.1 [task-14.1-registry-indexes] [agent:generic_agent] Register C14 multimodal PRD/UI/ADR in `.workflow-hero/config/documents.json`; update `docs/product/PRD.md`, `docs/product/UI.md`, `docs/architecture/ADR.md`, architecture overview, testing notes as needed.
- [x] 14.2 [task-14.2-context] [agent:generic_agent] Update `context/current-state.md` and append `context/context-log.md` with C14 planning/implementation outcomes (no image bytes/paths secrets).

## 15. Regression and acceptance — [PARALLEL after tasks 6–13]

- [x] 15.1 [task-15.1-acceptance] [agent:generic_agent] Automate PRD §5 acceptance scenarios with fixtures/fakes: picker chip→Codex send, Codex asset card+mosaic, clipboard chip, capability block, tool-written card, no real accounts.
- [x] 15.2 [task-15.2-race-regression] [agent:generic_agent] Package regressions for harness/conversation/tui/adapters; race tests for async workers; ensure non-Free-Chat sessions unchanged.

## 16. Final verification — [SERIES]

- [x] 16.1 [task-16.1-traceability] [agent:generic_agent] Trace PRD/UI/ADR requirements to tasks/specs; resolve gaps before handoff.
- [x] 16.2 [task-16.2-full-verify] [agent:generic_agent] Run `openspec validate tui-multimodal-images --strict` and `go test ./...`; fix until green.
