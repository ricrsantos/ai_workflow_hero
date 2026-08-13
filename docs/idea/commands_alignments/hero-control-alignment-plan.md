# Plano: alinhamento `/hero-cancel`, `/hero-finish`, `/hero-continue`, `/hero-back` (TUI × Chat)

> **Status:** implementado (2026-08-12).

Non-normative idea note. Para verdade de produto, ver PRD/UI/ADR (C2 slash parity, C3 harness autonomy).

---

## Contexto

Comandos de controle ainda usavam CLI direto ou Dispatch na TUI; no Chat do Cursor o orquestrador executa o fluxo completo (CLI + git + métricas + Task subagents).

Referência de padrão: [`hero-approve-alignment-plan.md`](hero-approve-alignment-plan.md), [`hero-reject-alignment-plan.md`](hero-reject-alignment-plan.md).

| Comando | TUI antes | Chat (Cursor) |
|---|---|---|
| `/hero-cancel` | CLI direto — `svc.Cancel("")` | `hero cancel` + git rollback |
| `/hero-finish` | CLI direto — `svc.Finish("")` | `hero finish --metrics-json`, `context/*` |
| `/hero-continue` | CLI direto — `svc.Continue(1)` fixo | `hero continue --extra N` + retoma etapa |
| `/hero-back` | Dispatch best-effort | Task `planning_agent`, re-run Impl→QA→Judge |

---

## Objetivo de paridade (todos)

1. **Gates em Go** (feedback imediato).
2. **Runtime Execute** — Chat + stream, `orchestration_agent.md` + `hero-*.md`.
3. Modelo de `agents.orchestration_agent` em `workflow-config.yml` (não `/hero-model`).
4. `ExecuteRequest.AgentName = "orchestration_agent"`; sessão nova.

---

## `/hero-cancel`

- Gates: ciclo ativo, modelo orquestrador, não streaming.
- Suporte `/hero-cancel <motivo>` no Chat.
- Preamble: git rollback, `hero cancel`, plain text.

## `/hero-finish`

- Gates: ciclo ativo, modelo orquestrador, não streaming.
- Preamble: Metrics Procedure, `hero finish --metrics-json`, `context-log.md` / `current-state.md`.

## `/hero-continue`

- Gates: ciclo ativo, etapa **Escalated** (`escalatedStage`), modelo orquestrador.
- `/hero-continue [N]` — default N=1.
- Preamble inclui N; agente retoma etapa após `hero continue`.

## `/hero-back`

- Gates: ciclo ativo, **Judge** em `PendingApproval`, modelo orquestrador.
- Substitui Dispatch; preamble: Task `planning_agent`, re-run pipeline.

---

## Arquivos alterados

- `internal/tui/app.go` — `beginHeroCancel/Finish/Continue/Back`, remover CLI cmds
- `internal/tui/conversation.go` — `heroRuntimeOpts`, composite orquestrador, inline parsers
- `internal/tui/chat_format.go` — 4 preambles
- `internal/tui/screens.go` — `escalatedStage()`
- Testes em `app_test.go`, `conversation_test.go`, `chat_format_test.go`
