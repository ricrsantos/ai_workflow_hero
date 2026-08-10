# Metrics — Cycle 3

**Title**: Correção de distorções na implementação da V1

## Stage Metrics

| Stage | Agent | Model | Input Tokens | Output Tokens | Cost (USD) | Duration |
|-------|-------|-------|-------------|---------------|------------|----------|
| Configuration | orchestration_agent | inherit | 6,250 | 1,000 | ~$0.00 | 2m |
| Research | discover_agent | inherit | 18,500 | 12,000 | ~$0.00 | 45m |
| Planning | planning_agent | composer-2.5 | 22,000 | 28,000 | ~$0.00 | 25m |
| Implementation | generic_agent | — | — | — | — | — |
| QA | qa_agent | — | — | — | — | — |
| Judge | judge_agent | — | — | — | — | — |
| Browser UI Validation | browser_ui_agent | — | — | — | — | — |
| QA End-to-End | end2end_qa_agent | — | — | — | — | — |
| **Subtotal** | | | 46,750 | 41,000 | ~$0.00 | 72m |

**Grand Total**: 87,750 tokens, ~$0.00 USD

## Notes

Token estimation (orchestrator Metrics Procedure):

1. `input_tokens = round(input_chars / 4)`, `output_tokens = round(output_chars / 4)`
2. Look up `input` / `output` rates in `.workflow-hero/models/<provider>.yml` (`unit: per_1m_tokens`)
3. `cost_usd = (input_tokens * input_rate + output_tokens * output_rate) / 1_000_000`

Never leave Input/Output/Cost as `—` for a stage that ran.
