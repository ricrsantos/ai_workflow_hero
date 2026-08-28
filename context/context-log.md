# Context Log

> Short-term project memory for this repository (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Permanent facts belong in `context/current-state.md`.

## 2026-08-28 — TUI timer label

**Change**: Renamed the blue navbar timer label from `Sessão` to `Session`.
The existing lifecycle remains unchanged: zero at TUI boot, first free-chat
prompt, and cycle transitions according to ADR-058.

**Validation**: `go test ./internal/tui` passes. The full suite passes outside
the two pre-existing restricted-sandbox OpenCode spawn cases documented in the
previous entry.

## 2026-08-28 — TUI navbar hint and Session timer lifecycle

**Problem**: The `alt+1-6` hint was attached to the navigation rows, and a fresh TUI restored the persisted cycle Session timer before the user explicitly resumed/started a cycle. Free-chat prompts also reset the Session timer on every turn.

**Change**: Anchored the shortcut hint to the last row of the navbar navigation area, immediately above the timer divider, with responsive clipping for short terminals. TUI startup now begins Session at zero; `/hero-start` and `/hero-resume` explicitly request persisted cycle recovery, while `/hero-new` starts a zeroed timer. Free chat starts at the first prompt and keeps the same process-local timer across later prompts. AI timing remains per Execute.

**Validation**: Focused layout/timer tests and the complete `internal/tui` suite pass with `GOCACHE=/tmp/hero-go-cache CC=/usr/bin/gcc CXX=/usr/bin/g++`. Full `go test ./... -count=1 -p 1` passes all other packages; the two known restricted-sandbox OpenCode serve-spawn tests still fail because their simulated process exits before exposing a listening URL.

## 2026-08-27 — TUI test helper and conditional navigation

**Outcome**: Test-only models skip the 30-second health probe, and command draining stops after the first business message. Esc cancels active Executes/preflight while Ctrl+C remains protected quit. Sidebar numbering follows visible screens (`alt+1-5`, or `alt+1-6` with Config).

**Validation**: TUI and focused navigation/cancellation tests passed.

## 2026-08-27 — `hero chat` OpenCode workspace routing

**Outcome**: Free chat stores configuration under `~/.workflow-hero/` but executes in cwd. All OpenCode session-scoped calls now use the same `directory` query as event subscription/recovery, preventing hangs and `context canceled` failures.

**Validation**: Adapter and full-suite checks passed apart from the two known restricted-sandbox OpenCode serve-spawn tests.

## 2026-08-26 — Chat composer and harness wait UX

**Outcome**: Enter inserts ordinary newlines while recognized slash commands execute; Alt+Enter submits ordinary prompts. Composer caret movement follows visual lines, and watchdog alerts are suppressed during permission/question waits.

**Validation**: TUI and repository tests passed except the known restricted-sandbox OpenCode serve-spawn cases.

## 2026-08-25 — C7 Config and C8 TUI-direct execution

**Outcome**: Added the active-cycle Config form with managed YAML saves/retry and TUI-direct named stage Executes after orchestrator handoff. Parallel Implementation agents and nested Task labels are represented in Chat.

**Validation**: Feature and repository tests passed during the implementation cycle.
