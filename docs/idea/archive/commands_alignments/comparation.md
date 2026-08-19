# TUI × Cursor Chat — Command Execution Comparison

> **Source:** code analysis (`internal/tui/app.go`, `assets/cursor/commands/hero-*.md`, `internal/cycle/service.go`) from the Hero 1.0 alignment discussion (Aug 2026).  
> **Last updated:** 2026-08-12 — `/hero-sync`, `/hero-status`, `/hero-archive`, `/hero-resume` aligned (paridade parcial); `/hero-cancel`, `/hero-finish`, `/hero-continue`, `/hero-back` aligned; `/hero-reject`, `/hero-approve`, `/hero-start`, `/hero-new` aligned in **v1.0.1**.

Non-normative idea note. For product truth, see PRD/UI/ADR cycles (C2 slash parity, C3 harness autonomy).

---

## Mechanism legend

| Mechanism | TUI | Chat (Cursor) |
|---|---|---|
| **CLI direto** | TUI calls `cycle.Service` → `engine` → SQLite (no LLM) | Orchestrator may call the same `hero …` verbs, usually with extra context/metrics |
| **Go local** | TUI formats data in Go (no LLM) | Orchestrator may read the same files with richer narrative output |
| **Dispatch** | TUI reads command `.md` and sends it to `cursor-agent` via `HarnessAdapter.Dispatch` (best-effort, no Chat stream UI) | N/A as primary path — chat expands slash in the IDE panel |
| **Runtime Execute** | TUI reads `hero-*.md` (+ `orchestration_agent.md` for orchestrated commands), opens **Chat** screen, runs `HarnessAdapter.Execute` with configured model + stream | N/A — IDE uses orchestrator instead of Agent CLI from TUI |
| **Orquestrador** | TUI embeds `orchestration_agent.md` in control/workflow slash prompts; Agent CLI orquestra via Task / CLI | Chat expands `.md`; `orchestration_agent` interpreta, chama `hero …` e/or Task subagents |

---

## Slash commands — TUI vs Chat

| Comando | TUI | Chat (Cursor) | Diferenças | Alinhamento |
|---|---|---|---|---|
| `/hero-new` | ✅ **Runtime Execute:** Chat screen + stream `hero-new.md`; after stream, TUI calls `PrepareCycle()` (`hero cycle new` semantics: active cycle, empty title/objective); default model from `/hero-model`; ends with “run `/hero-start`” | **Orquestrador:** prepara/importa `workflow-config.yml`, roda `hero cycle new` (ciclo ativo, title/objective vazios no SQLite), sync `project.json`; handoff para chat limpo + `/hero-start` | **Paridade:** ambos criam ciclo SQLite ao concluir preparação do config (sem title/objective); TUI sem handoff “novo chat”; execução via Agent CLI na TUI | ✅ **Alinhado** (v1.0.1) |
| `/hero-start` | ✅ **Runtime Execute:** exige ciclo ativo; `SyncCycleConfig()` antes do stream (title/objective do YAML → SQLite); Chat + stream `orchestration_agent.md` + `hero-start.md`; modelo `agents.orchestration_agent`; `AgentName=orchestration_agent` | **Orquestrador:** `hero cycle sync-config`, bootstrap do disco, valida config, Task subagents, `hero approve/finish` + métricas; modelo = sessão IDE | **Paridade parcial:** sync-config no TUI em Go; chat orquestrador roda CLI; modelo orquestrador via YAML (não `/hero-model`) | ✅ **Alinhado** (paridade parcial) |
| `/hero-approve` | ✅ **Runtime Execute:** exige ciclo ativo + etapa `PendingApproval`; Chat + stream `orchestration_agent.md` + `hero-approve.md`; modelo `agents.orchestration_agent`; `AgentName=orchestration_agent`; agente confirma status, aplica Metrics Procedure, `hero approve --metrics-json` (+ summary opcional) | **Orquestrador:** confirma status, calcula métricas, `hero approve --metrics-json` (+ summary opcional) | **Paridade parcial:** sessão fresh (sem histórico `/hero-start`); contexto via CLI; modelo orquestrador via YAML (não `/hero-model`) | ✅ **Alinhado** (paridade parcial) |
| `/hero-reject` | ✅ **Runtime Execute:** exige ciclo ativo + etapa `PendingApproval`; coleta motivo no Chat; depois stream `orchestration_agent.md` + `hero-reject.md`; modelo `agents.orchestration_agent`; agente chama `hero reject --reason` e re-dispara etapa | **Orquestrador:** `hero reject --reason`, reexecuta o agente da etapa com feedback | **Paridade:** motivo obrigatório no Chat; reexecução via orquestrador; modelo orquestrador via YAML (não `/hero-model`) | ✅ **Alinhado** (paridade parcial) |
| `/hero-cancel` | ✅ **Runtime Execute:** exige ciclo ativo; Chat + stream `orchestration_agent.md` + `hero-cancel.md`; modelo orquestrador; agente roda `hero cancel`, git rollback, status pós-cancel; `/hero-cancel <motivo>` opcional | **Orquestrador:** `hero cancel` + git checkout/restore | **Paridade parcial:** rollback git via agente; modelo via YAML | ✅ **Alinhado** (paridade parcial) |
| `/hero-finish` | ✅ **Runtime Execute:** exige ciclo ativo; Chat + stream orquestrador + `hero-finish.md`; agente valida etapas, `hero finish --metrics-json`, atualiza `context/*` | **Orquestrador:** valida etapas, `hero finish --metrics-json`, `context-log.md` / `current-state.md` | **Paridade parcial:** fechamento completo via agente | ✅ **Alinhado** (paridade parcial) |
| `/hero-continue` | ✅ **Runtime Execute:** exige etapa Escalated; `/hero-continue [N]` (default 1); Chat + stream orquestrador + `hero-continue.md`; agente `hero continue --extra N` e retoma etapa | **Orquestrador:** `hero continue --extra N` e retoma execução | **Paridade parcial:** N customizável; retomada via Task | ✅ **Alinhado** (paridade parcial) |
| `/hero-back` | ✅ **Runtime Execute:** exige Judge em `PendingApproval`; Chat + stream orquestrador + `hero-back.md`; Task `planning_agent`, re-run Impl→QA→Judge | **Orquestrador:** reabre Planning via `planning_agent` (Task) | **Paridade parcial:** sem verbo CLI `hero back`; orquestração real | ✅ **Alinhado** (paridade parcial) |
| `/hero-sync` | ✅ **Runtime Execute:** Chat + stream `orchestration_agent.md` + `hero-sync.md`; modelo YAML ou fallback `/hero-model`; Task `context_agent`, artefatos, `hero doctor` | **Orquestrador:** Task `context_agent`, gera `AGENTS.md` / `context/*`, scan de docs pendentes, `hero doctor` | **Paridade parcial:** sync completo via agente; modelo YAML ou `/hero-model` na TUI (bootstrap) | ✅ **Alinhado** (paridade parcial) |
| `/hero-status` | ✅ **Runtime Execute:** Chat + stream orquestrador + `hero-status.md`; agente roda `hero status` e relay da tabela completa | **Orquestrador:** `hero status` (tabela completa de etapas) | **Paridade parcial:** tabela via agente CLI; modelo YAML ou `/hero-model` | ✅ **Alinhado** (paridade parcial) |
| `/hero-archive` | ✅ **Runtime Execute:** exige ciclo ativo; Chat + stream orquestrador + `hero-archive.md`; agente confirma `hero status`, `hero cycle archive`, opcional `metrics-summary.md` | **Orquestrador:** confirma via `hero status`, chama `hero cycle archive`, pode atualizar `metrics-summary.md` | **Paridade parcial:** OpenSpec via CLI; narrativa e artefatos opcionais via agente | ✅ **Alinhado** (paridade parcial) |
| `/hero-resume` | ✅ **Runtime Execute:** Chat + stream orquestrador + `hero-resume.md`; `/hero-resume [N]` no Chat; agente `hero cycle resume [--number N]` + `hero status` | **Orquestrador:** `hero cycle resume [--number N]` (N opcional) | **Paridade parcial:** N customizável; modelo YAML ou `/hero-model` | ✅ **Alinhado** (paridade parcial) |
| `/hero-cycles` | **Go local:** `svc.Cycles()` + `cycle.FormatCycles` | **Orquestrador:** `hero status/metrics --json` + scan de `.workflow-hero/cycles/archive/` legado | Mesma intenção; chat pode incluir parsing de `metrics.md` legado com raciocínio; TUI usa só o formatter Go | ⏳ Parcial |
| `/hero-todos` | **Go local:** `todos.ReadProject` + `todos.Format` (parse determinístico) | **Orquestrador:** lê `current-state.md` e formata no chat | Saída similar; TUI é 100% determinístico, chat passa pelo LLM (mas só leitura) | ⏳ Parcial |
| `/hero-model` | **UI nativa:** abre picker de modelos → persiste em `hero.json` (`harnesses.<tool>.model`); obrigatório antes de Chat/dispatches | **Orquestrador:** instrui o usuário a usar a **TUI** (`/hero-model` no palette) — **não altera modelo no chat** | Comando **efetivo só na TUI**; no chat é redirecionamento | ✅ Por design |
| `/hero-help` | **Go local:** mostra caminho `See .workflow-hero/docs/workflow-help.md` | **Orquestrador:** imprime tabela completa de comandos no chat | TUI só aponta o arquivo; chat entrega o help inline | ⏳ Parcial |

---

## Itens do menu TUI que não são slash commands

| Item TUI | Chat equivalente | Diferença |
|---|---|---|
| `Go to - Status/Approvals/…` | Não existe | Navegação exclusiva da TUI |
| `Refresh` | Não existe | Recarrega views do SQLite na TUI |
| `Quit` | Não existe | Sai da TUI |
| Comandos importados (`.cursor/commands/`) | Slash custom no chat | Ambos expandem markdown; TUI usa **Dispatch** (Agent CLI); chat injeta no painel do IDE |

---

## Schema — entrada TUI (shell atual)

```
`hero` (default) / `hero tui`
             │
      Bubble Tea App (`internal/tui`)
             │
   ┌─────────┼─────────┬──────────┬──────────┬──────────────┐
   │         │         │          │          │              │
Status  Approvals  Artifacts   Costs    Events    Conversation (Chat)
   │         │         │          │          │              │
   └─────────┴─────────┴──────────┴──────────┘              │
             │                                              │
        cycle.Service (read views)                    HarnessAdapter
             │                                    Execute / Cancel / Stream
   Command Palette (`/hero-*`)                              │
   + cmds importados de `.cursor/commands/`          Cursor Adapter
   (expansão markdown → Dispatch ou Execute)                 │
             │                                         cursor-agent
             ▼                                              ▼
                    hero.db (SQLite)
```

**Notas:**

- `/hero-new` usa **Runtime Execute** na tela Chat (não mais `svc.NewCycle` direto).
- `/hero-start` usa **Runtime Execute** com `orchestration_agent.md` + `hero-start.md`; exige ciclo SQLite ativo; modelo de `agents.orchestration_agent` (TUI-only).
- `/hero-approve` usa **Runtime Execute** com `orchestration_agent.md` + `hero-approve.md`; exige ciclo ativo e etapa `PendingApproval`; mesmo modelo orquestrador.
- `/hero-reject` usa **Runtime Execute** com `orchestration_agent.md` + `hero-reject.md`; coleta motivo no Chat antes do stream; exige ciclo ativo e etapa `PendingApproval`; mesmo modelo orquestrador.
- `/hero-cancel`, `/hero-finish`, `/hero-continue`, `/hero-back` usam **Runtime Execute** com `orchestration_agent.md` + comando; gates em Go (ciclo ativo; Escalated para continue; Judge PendingApproval para back); modelo orquestrador via YAML.
- `/hero-sync`, `/hero-status`, `/hero-archive`, `/hero-resume` usam **Runtime Execute** com `orchestration_agent.md` + comando; sync/status/resume usam modelo YAML ou fallback `/hero-model`; archive exige ciclo ativo + modelo orquestrador; resume suporta `/hero-resume [N]` no Chat.
- Comandos importados (não-Hero) ainda usam **Dispatch**.
- Free chat na tela Chat = `conversationStage == ""`; sessão do harness fica só em memória da TUI.

---

## Schema — padrão de execução (TUI × Chat)

```
                    TUI                              Chat
                     │                                │
         ┌───────────┼───────────┬──────────┐        │
         │           │           │          │        │
    CLI direto   Go local    Dispatch   Runtime     Orquestrador
    (cycles,     (todos,     (imported  Execute    (lê hero-*.md,
     help)        help)       cmds)     (new,start, chama hero CLI,
                                         approve,    Task, git, files)
                                         reject,
                                         cancel,
                                         finish,
                                         continue,
                                         back,
                                         sync,
                                         status,
                                         archive,
                                         resume)
         │           │           │          │        │
         └───────────┴───────────┴──────────┘        │
                     │                                │
                     └────────────┬───────────────────┘
                                  ▼
                         hero.db (SQLite)
                    (mesmo store quando o CLI é chamado)
```

\* `/hero-start`, `/hero-approve`, `/hero-reject`, `/hero-cancel`, `/hero-finish`, `/hero-continue`, `/hero-back`, `/hero-sync`, `/hero-status`, `/hero-archive`, `/hero-resume` na TUI leem `orchestration_agent.md` + comando via Runtime Execute.

---

## `/hero-approve` — o que mudou no alinhamento

| Aspecto | Antes | Depois |
|---|---|---|
| Mecanismo TUI | **CLI direto** — `svc.Approve("", "")` sem métricas | **Runtime Execute** — `orchestration_agent.md` + `hero-approve.md` + preamble TUI |
| Markdown | Ignorado | Lê agente + comando |
| UI | Action panel / status bar | Tela **Chat** com streaming |
| Modelo | N/A (sem LLM) | `workflow-config.yml` → `agents.orchestration_agent` |
| Gates | Nenhum (falha no engine) | Ciclo ativo + `PendingApproval` + modelo orquestrador |
| Métricas | Não persistidas | Agente chama `hero approve --metrics-json` |

---

## `/hero-start` — o que mudou no alinhamento

| Aspecto | Antes | Depois |
|---|---|---|
| Mecanismo TUI | **Dispatch** — `svc.RunWith()` + prompt genérico | **Runtime Execute** — `orchestration_agent.md` + `hero-start.md` + preamble TUI |
| Markdown | Ignorado | Lê agente + comando (`.cursor/agents/` + `.cursor/commands/`) |
| UI | Action panel | Tela **Chat** com streaming |
| Modelo | `/hero-model` → `hero.json` | `workflow-config.yml` → `agents.orchestration_agent` (TUI-only; sem gate `/hero-model`) |
| Ciclo SQLite | Exigia ciclo (sem criar) | Exige ciclo ativo; erro + `/hero-new` se ausente (não cria ciclo) |
| Agente | Não usado | `ExecuteRequest.AgentName = orchestration_agent` |

---

## `/hero-new` — o que mudou no alinhamento (v1.0.1)

| Aspecto | Antes (pré-v1.0.1) | Depois (v1.0.1) |
|---|---|---|
| Mecanismo TUI | **CLI direto** — `svc.NewCycle("", "")` | **Runtime Execute** — `beginHeroRuntimeConversation("new")` |
| Markdown | Ignorado | Lê `assets/cursor/commands/hero-new.md` (+ preamble TUI) |
| UI | Status / action panel | Tela **Chat** com streaming |
| Modelo | Fallback fixo `composer-2.5` | Exige `/hero-model` uma vez; usa slug em `hero.json` |
| Criação do ciclo | Imediata no SQLite | **Não** neste passo; usuário edita config e roda `/hero-start` |
| Handoff Cursor | N/A na TUI | Preamble/remove handoff “novo chat” no output TUI |

---

## Conclusão

- Comandos de **controle de estado** (`archive`, `resume`, `cancel`, `finish`, `continue`, `back`, `approve`, `reject`) na TUI usam **Runtime Execute** (orquestrador + CLI).
- Comandos de **trabalho** (`sync`, `start`, `new`) e **consulta** (`status`) na TUI usam **Runtime Execute** com orquestrador quando alinhados; `cycles`, `todos`, `help` permanecem determinísticos em Go.
- `/hero-model` e `/hero-help` têm comportamentos **assimétricos** por design.

**Próximo alvo de alinhamento sugerido:** `/hero-cycles`, `/hero-todos`, `/hero-help` (saída mais rica ou help inline).
