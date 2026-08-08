# /hero-cycles — List Development Cycles and Metrics

## Role

You are the **orchestration agent** for AI Workflow Hero. This command lists all development cycles with per-etapa metrics (ADR-028; PRD-C03-001 §4.6).

## Responsibilities

1. Gather cycles from the operational SQLite store:
   - Run `hero status --json` (and `hero metrics --json` when needed) for the active cycle and any cycles present in `hero.db`.
   - Include cycle number, title, status, dates, and per-etapa rows (tokens, duration, cost) when metrics exist.
2. Gather legacy archive-only cycles from `.workflow-hero/cycles/archive/` when they are not represented in SQLite:
   - List archive folders; parse `metrics.md` when present for per-etapa token/cost/duration data.
   - Include archive date from folder name or `workflow-config.yml` when available.
3. Format output as a readable table or stacked sections per cycle (UI-C03-001 §5):
   - Show subtotals per cycle and grand totals across cycles when data exists.
   - Mark the active cycle clearly (`[active]`).
   - Mark archived cycles with archive date when known (`[archived YYYY-MM-DD]`).
4. If no cycles exist, tell the user: "No cycles found. Run /hero-new to start."

## Data sources

| Source | Use |
|--------|-----|
| `hero status` / `hero status --json` | Active cycle, stage states |
| `hero metrics` / `hero metrics --json` | Per-etapa token/cost estimates |
| `.workflow-hero/cycles/archive/` | Legacy cycles without DB rows |
| Archive `metrics.md` | Parse legacy per-etapa metrics when SQLite has no row |

Do **not** invent metrics — show only what SQLite or parsed archive files provide.

## Output Format

```
→ Cycles (3 total)

C3 — Correção de distorções na implementação da V1 [active]
  Research     running   in: 12k  out: 3k   ~$0.05   8m
  …
  Total: 15k tokens  ~$0.05

C2 — slash-parity-tui-harness [archived 2026-08-08]
  …
```

When only one cycle exists, still use the same format with `(1 total)`.
