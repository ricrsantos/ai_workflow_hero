# Plano: alinhamento `/hero-reject` (TUI × Chat)

> **Status:** implementado (2026-08-12).

Non-normative idea note. Para verdade de produto, ver PRD/UI/ADR (C2 slash parity, C3 harness autonomy).

---

## Contexto e gap atual

| Aspecto | TUI hoje | Chat (Cursor) |
|---|---|---|
| Mecanismo | **CLI direto** — `beginAction` → `rejectCmd()` → `svc.Reject("")` em [`internal/tui/app.go`](../../../internal/tui/app.go) | **Orquestrador** — expande [`hero-reject.md`](../../../assets/cursor/commands/hero-reject.md) + `orchestration_agent` |
| Markdown | Ignorado | Lê comando completo |
| UI | Action panel / status bar ("Stage rejected.") | Chat IDE com streaming |
| Modelo | N/A (sem LLM) | Modelo da sessão do orquestrador |
| Motivo | Sempre vazio | `hero reject --reason '<feedback>'` |
| Reexecução | Não re-dispara agente da etapa | Task subagent com feedback |

Referência de padrão já implementado: [`hero-approve-alignment-plan.md`](hero-approve-alignment-plan.md) e linha `/hero-approve` em [`comparation.md`](comparation.md).

---

## Objetivo de paridade

Na TUI, `/hero-reject` deve:

1. **Validar pré-condições em Go** (feedback imediato, sem round-trip LLM):
   - Ciclo ativo no SQLite (`svc.Status()`, `CycleNumber != 0`)
   - Etapa em `PendingApproval` (reutilizar [`pendingApprovalStage`](../../../internal/tui/screens.go))
   - Modelo do orquestrador resolvido (`workflowconfig.OrchestratorModelSlug`)
   - Não executar se já houver streaming ativo
2. **Coletar motivo no Chat** (two-step):
   - Abrir tela **Chat** com `awaitingRejectReason = true`
   - Usuário digita feedback e pressiona Enter
   - Aceitar atalho `/hero-reject <motivo>` no input do Chat
3. Abrir **Runtime Execute** com stream após motivo conhecido:
   - **Preamble TUI** — `tuiHeroRejectPreamble(reason)` em [`chat_format.go`](../../../internal/tui/chat_format.go)
   - **Corpo do agente** — `.cursor/agents/orchestration_agent.md`
   - **Corpo do comando** — `.cursor/commands/hero-reject.md` via `ReadCommandPrompt`
4. O agente deve: confirmar status (`hero status`), `hero reject --reason`, re-disparar agente da etapa via Task.
5. Usar modelo de `agents.orchestration_agent` em `workflow-config.yml` (**não** `/hero-model`).
6. Definir `ExecuteRequest.AgentName = "orchestration_agent"`; sessão **nova** (`harnessSessionID = ""`).

**Diferenças intencionais (TUI-only):** modelo do YAML; plain text; coleta de motivo em passo explícito no Chat; paridade **parcial**.

---

## Mudanças de código

### 1. [`internal/tui/app.go`](../../../internal/tui/app.go)

- `beginHeroReject()` com gates + modo coleta no Chat.
- `beginHeroRejectExecute(reason)` → Runtime Execute.
- Substituir palette `actionReject` e tecla `r` em Approvals.
- Remover `rejectCmd()`.

### 2. [`internal/tui/conversation.go`](../../../internal/tui/conversation.go)

- Tratar `reject` como `start`/`approve` (agent + command composite, `AgentName = orchestration_agent`).
- `submitConversation`: `awaitingRejectReason` e `/hero-reject <motivo>` inline.

### 3. [`internal/tui/chat_format.go`](../../../internal/tui/chat_format.go)

- `tuiHeroRejectPreamble(reason)` + `case "reject"` em `tuiRuntimeCommandPrompt`.

---

## Testes

| Teste | Foco |
|---|---|
| `TestHeroRejectRuntimeConversation` | Chat, streaming, prompt, modelo, agent, sessão nova, motivo no preamble |
| `TestHeroRejectRequiresActiveCycle` | Sem ciclo → erro |
| `TestHeroRejectRequiresPendingApproval` | Sem pending → erro |
| `TestHeroRejectRequiresOrchestratorModel` | Sem model → erro |
| `TestHeroRejectRequiresReason` | Enter vazio em modo coleta → erro |
| `TestHeroRejectInlineReason` | `/hero-reject fix tests` → execute direto |
| `TestRejectActionWithService` | Tecla `r` → coleta; submit → Runtime Execute |
| `TestTUIRuntimeCommandPrompt_HeroRejectOverrides` | Preamble reject + motivo |

Rodar `go test ./...` após implementação.

---

## Documentação

- Este arquivo em `docs/idea/commands_alignments/hero-reject-alignment-plan.md`
- Atualizar [`comparation.md`](comparation.md)
- [`context/current-state.md`](../../../context/current-state.md) e [`context/context-log.md`](../../../context/context-log.md)
