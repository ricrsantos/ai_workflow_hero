# Plano: alinhamento `/hero-approve` (TUI × Chat)

> **Status:** implementado (2026-08-12).

Non-normative idea note. Para verdade de produto, ver PRD/UI/ADR (C2 slash parity, C3 harness autonomy).

---

## Contexto e gap atual

| Aspecto | TUI hoje | Chat (Cursor) |
|---|---|---|
| Mecanismo | **CLI direto** — `beginAction` → `approveCmd()` → `svc.Approve("", "")` em [`internal/tui/app.go`](../../../internal/tui/app.go) | **Orquestrador** — expande [`hero-approve.md`](../../../assets/cursor/commands/hero-approve.md) + `orchestration_agent` |
| Markdown | Ignorado | Lê comando completo + Metrics Procedure |
| UI | Action panel / status bar ("Stage approved.") | Chat IDE com resumo de métricas formatado |
| Modelo | N/A (sem LLM) | Modelo da sessão do orquestrador |
| Métricas | Nenhuma (`--metrics-json` vazio) | Calcula tokens/custo/duração e persiste via CLI |
| Summary | Vazio | Opcional via `--summary` |

Referência de padrão já implementado: [`hero-start-alignment-plan.md`](hero-start-alignment-plan.md) e linha `/hero-start` em [`comparation.md`](comparation.md).

---

## Objetivo de paridade

Na TUI, `/hero-approve` deve:

1. **Validar pré-condições em Go** (feedback imediato, sem round-trip LLM):
   - Ciclo ativo no SQLite (`svc.Status()`, `CycleNumber != 0`)
   - Etapa em `PendingApproval` (reutilizar [`pendingApprovalStage`](../../../internal/tui/screens.go))
   - Modelo do orquestrador resolvido (`workflowconfig.OrchestratorModelSlug`)
   - Não executar se já houver streaming ativo
2. Abrir a tela **Chat** e executar `HarnessAdapter.Execute` com stream (**Runtime Execute**).
3. Montar o prompt em três camadas (mesmo padrão de `/hero-start`):
   - **Preamble TUI** — `tuiHeroApprovePreamble()` em [`chat_format.go`](../../../internal/tui/chat_format.go)
   - **Corpo do agente** — `.cursor/agents/orchestration_agent.md`
   - **Corpo do comando** — `.cursor/commands/hero-approve.md` via `ReadCommandPrompt`
4. O agente deve: confirmar status (`hero status`), aplicar Metrics Procedure, chamar `hero approve --metrics-json '…'` (+ `--summary` opcional), exibir métricas e indicar avanço.
5. Usar modelo de `agents.orchestration_agent` em `workflow-config.yml` (**não** `/hero-model`).
6. Definir `ExecuteRequest.AgentName = "orchestration_agent"`; sessão **nova** (`harnessSessionID = ""`).

**Diferenças intencionais (TUI-only):** modelo do YAML; plain text; sem handoff Cursor; sessão fresh com contexto via `hero status` / `hero metrics --json` — paridade **parcial**.

---

## Mudanças de código

### 1. [`internal/tui/app.go`](../../../internal/tui/app.go)

- `beginHeroApprove()` com gates (ciclo, pending approval, modelo orquestrador, streaming).
- Substituir palette `actionApprove` e tecla `a` em Approvals.
- Remover `approveCmd()`.

### 2. [`internal/tui/conversation.go`](../../../internal/tui/conversation.go)

- Tratar `approve` como `start` (agent + command composite, `AgentName = orchestration_agent`).
- Helper `orchestratorRuntimePrompt(projectDir, cmdBody string)`.

### 3. [`internal/tui/chat_format.go`](../../../internal/tui/chat_format.go)

- `tuiHeroApprovePreamble()` + `case "approve"` em `tuiRuntimeCommandPrompt`.

---

## Testes

| Teste | Foco |
|---|---|
| `TestHeroApproveRuntimeConversation` | Chat, streaming, prompt, modelo, agent, sessão nova |
| `TestHeroApproveRequiresActiveCycle` | Sem ciclo → erro |
| `TestHeroApproveRequiresPendingApproval` | Sem pending → erro |
| `TestHeroApproveRequiresOrchestratorModel` | Sem model → erro |
| `TestTUIRuntimeCommandPrompt_HeroApproveOverrides` | Preamble approve |
| `TestApproveActionWithService` | Tecla `a` → Runtime Execute |

Rodar `go test ./...` após implementação.

---

## Documentação

- Este arquivo em `docs/idea/commands_alignments/hero-approve-alignment-plan.md`
- Atualizar [`comparation.md`](comparation.md)
- [`context/current-state.md`](../../../context/current-state.md) e [`context/context-log.md`](../../../context/context-log.md)
