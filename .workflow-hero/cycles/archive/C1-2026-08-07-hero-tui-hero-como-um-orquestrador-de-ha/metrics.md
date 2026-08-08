# Metrics — Cycle C1

**Title**: Hero TUI, Hero como um Orquestrador de Harness com AI Loop

## Stage Metrics

| Stage | Agent | Model | Input Tokens | Output Tokens | Cost (USD) | Duration |
|-------|-------|-------|-------------|---------------|------------|----------|
| Configuration | orchestration_agent | cursor-grok-4.5 | 10000 | 800 | 0.0420 | 2m |
| Research | discover_agent | cursor-grok-4.5 | 45000 | 18000 | 0.4050 | 35m |
| Planning | planning_agent | cursor-grok-4.5-high | 23000 | 13000 | 0.2640 | 12m |
| Implementation | generic_agent | cursor-grok-4.5-high | 105000 | 46250 | 1.0088 | 20m |
| Implementation (QA fix) | generic_agent | cursor-grok-4.5-high | 7000 | 1125 | 0.0379 | 2m |
| Implementation (Judge fix) | generic_agent | cursor-grok-4.5-high | 13000 | 3000 | 0.0840 | 3m |
| QA | qa_agent | cursor-grok-4.5-high | 31750 | 1250 | 0.1140 | 5m |
| Judge | judge_agent | cursor-grok-4.5-high | 42500 | 1325 | 0.1474 | 5m |
| Browser UI Validation | browser_ui_agent | — | — | — | — | — |
| QA End-to-End | end2end_qa_agent | cursor-grok-4.5-high | 18000 | 1050 | 0.0698 | 3m |
| **Subtotal** | | | 295250 | 85800 | 2.1729 | |

**Grand Total**: 381050 tokens, ~$2.1729 USD

## Notes

QA E2E: 7 CLI journeys passed (no Playwright). end2end_qa_agent 72000/4200 chars → cursor-grok-4.5-high.
