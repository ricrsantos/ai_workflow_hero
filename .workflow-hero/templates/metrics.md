# Metrics — Cycle {{project.cycle}}

> **Legacy / optional template (Hero 1.0):** per-cycle metrics live in `.workflow-hero/hero.db`.
> Query with `hero metrics` (or `hero metrics --json`). Persist via `hero … --metrics-json`.
> Kept for upgrade import fixtures and Metrics Procedure formula reference only.

**Title**: {{workflow.title}}

## Stage Metrics

| Stage | Agent | Model | Input Tokens | Output Tokens | Cost (USD) | Duration |
|-------|-------|-------|-------------|---------------|------------|----------|
| Configuration | orchestration_agent | — | — | — | — | — |
| Research | discover_agent | — | — | — | — | — |
| Planning | planning_agent | — | — | — | — | — |
| Implementation | backend_agent | — | — | — | — | — |
| Implementation | frontend_agent | — | — | — | — | — |
| QA | qa_agent | — | — | — | — | — |
| Judge | judge_agent | — | — | — | — | — |
| Browser UI Validation | browser_ui_agent | — | — | — | — | — |
| QA End-to-End | end2end_qa_agent | — | — | — | — | — |
| **Subtotal** | | | | | | |

**Grand Total**: — tokens, ~$— USD

## Notes

Token estimation (orchestrator Metrics Procedure):

1. `input_tokens = round(input_chars / 4)`, `output_tokens = round(output_chars / 4)`
2. Look up `input` / `output` rates in `.workflow-hero/models/<provider>.yml` (`unit: per_1m_tokens`)
3. `cost_usd = (input_tokens * input_rate + output_tokens * output_rate) / 1_000_000`

Never leave Input/Output/Cost unset for a stage that ran — persist via the `hero` CLI.
