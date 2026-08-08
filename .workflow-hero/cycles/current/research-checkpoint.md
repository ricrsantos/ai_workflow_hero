# Research Checkpoint — Cycle C3

> Decisions from Research grilling (2026-08-08). Cycle docs are canonical; this file is operational memory for Planning.

## Decisions

| # | Topic | Decision |
|---|---|---|
| 1 | TUI orchestration | TUI manages etapas; each **interação** dispatches to harness (`cursor agent --resume`); Go never calls LLM |
| 2 | Dual-entry | Maintained: TUI primary autonomous path; Cursor chat parity |
| 3 | hero-todos / hero-sync | Todos reads only `current-state.md`; sync scans product/architecture pending items into current-state; todos warns to run sync |
| 4 | hero-cycles | SQLite + archive folders; per-etapa metrics |
| 5 | Slash vocabulary | `/hero-<name>` hyphen canonical; amend ADR-020 |
| 6 | Streaming | `stream-json` in TUI conversation panel |
| 7 | Harness boot | TUI prompts harness if `cli.tools` empty; validates; persists; aborts on failure; independent of doctor |
| 8 | cli.tools persistence | `hero.json` → `cli.tools` |
| 9 | HarnessAdapter | Full contract per `06_harness_adapter.md` |

## Terminology

- **Etapa** — workflow stage (Research, Planning, …)
- **Interação** — one conversational round within an etapa (not “turno”)

## OpenSpec

- Change name: `cursor-harness-tui-autonomy`

## Deferred (unchanged)

- D1 multi-harness adapters
- IDE chat injection
- Windows
