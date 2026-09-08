# C13 QA loop-back — implementation gaps (iteration 3)

Source: QA (`qa_agent`, `opencode-go/deepseek-v4-pro`). `tests_passed: false`. `logging: pass`.
Build, go vet, and gofmt passed. Exactly one test failure across `go test ./...`.

Scope: native → `generic_agent`.

## Required work (this Implementation pass)

1. Update `internal/tui/harness_picker_test.go` `TestHarnessPickerPersistsAutoProjectPermissionProfileInline`.
   - The test asserts `strings.Count(view, "Permissions:") != 3` (exactly 3 headings).
   - C13 added `claude` as a 4th supported harness (`SupportedHarnessIDs` / `SupportedToolIDs`).
   - The picker now renders 4 harnesses (Cursor, OpenCode, Codex, claude), including disabled/available `claude`, so there are 4 `Permissions:` headings.
   - This is a stale test expectation, not a product bug. Update the count (and any other hardcoded 3-harness assumptions in that file) so the picker tests follow discovered supported harnesses.

Do not treat this as a reason to hide or skip the Claude harness in the picker.

## Already verified by QA (do not rework)

- Logging: Claude adapter uses structured slog (error/info/debug), default `slog.LevelInfo`. Permission-bridge "rejected token" logs the error only, never the token.
- Coverage noted (no explicit TESTING.md threshold): internal/tui 69.7%, internal/adapters/claude 61.4%, internal/harness 86.2%, internal/harnessmgr 69.9%, internal/install 68.0%.
