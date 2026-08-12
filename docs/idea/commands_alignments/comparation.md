# TUI × Cursor Chat — Command Execution Comparison

> **Source:** code analysis (`internal/tui/app.go`, `assets/cursor/commands/hero-*.md`, `internal/cycle/service.go`) from the Hero 1.0 alignment discussion (Aug 2026).  
> **Last updated:** 2026-08-12 — `/hero-start` marked **aligned** (paridade parcial); `/hero-new` aligned in **v1.0.1**.

Non-normative idea note. For product truth, see PRD/UI/ADR cycles (C2 slash parity, C3 harness autonomy).

---

## Mechanism legend

| Mechanism | TUI | Chat (Cursor) |
|---|---|---|
| **CLI direto** | TUI calls `cycle.Service` → `engine` → SQLite (no LLM) | Orchestrator may call the same `hero …` verbs, usually with extra context/metrics |
| **Go local** | TUI formats data in Go (no LLM) | Orchestrator may read the same files with richer narrative output |
| **Dispatch** | TUI reads command `.md` and sends it to `cursor-agent` via `HarnessAdapter.Dispatch` (best-effort, no Chat stream UI) | N/A as primary path — chat expands slash in the IDE panel |
| **Runtime Execute** | TUI reads `hero-*.md` (+ `orchestration_agent.md` for `/hero-start`), opens **Chat** screen, runs `HarnessAdapter.Execute` with configured model + stream | N/A — IDE uses orchestrator instead of Agent CLI from TUI |
| **Orquestrador** | TUI embeds `orchestration_agent.md` in `/hero-start` prompt; Agent CLI orquestra via Task | Chat expands `.md`; `orchestration_agent` interpreta, chama `hero …` e/or Task subagents |

---

## Slash commands — TUI vs Chat

| Comando | TUI | Chat (Cursor) | Diferenças | Alinhamento |
|---|---|---|---|---|
| `/hero-new` | ✅ **Runtime Execute:** Chat screen + stream `hero-new.md` via `HarnessAdapter.Execute` (default model from `/hero-model`; TUI preamble skips `hero cycle new` and Cursor handoff; ends with “run `/hero-start`”) | **Orquestrador:** prepara/importa `workflow-config.yml`, pede revisão ao usuário, depois `hero cycle new`; handoff para chat limpo + `/hero-start` | **Paridade parcial (v1.0.1):** mesmo markdown e preparação via LLM; TUI **não** cria o ciclo SQLite neste passo (inicia com `/hero-start`); sem handoff “novo chat”; execução via Agent CLI na TUI, não `orchestration_agent` do IDE | ✅ **Alinhado** (v1.0.1) |
| `/hero-start` | ✅ **Runtime Execute:** exige ciclo ativo (erro + `/hero-new` se ausente); Chat + stream `orchestration_agent.md` + `hero-start.md`; modelo `agents.orchestration_agent` em workflow-config (TUI-only); `AgentName=orchestration_agent` | **Orquestrador:** bootstrap do disco, valida config, executa etapas via Task, persiste com `hero approve/finish` + métricas; modelo = sessão IDE | **Paridade parcial:** mesma orquestração via markdown; TUI não cria ciclo; modelo orquestrador via YAML (não `/hero-model`); chat usa modelo da sessão | ✅ **Alinhado** (paridade parcial) |
| `/hero-approve` | **CLI direto:** `svc.Approve("", "")` — sem `--summary`, sem `--metrics-json` | **Orquestrador:** confirma status, calcula métricas, `hero approve --metrics-json` (+ summary opcional) | TUI aprova “seco”; chat registra métricas e pode resumir a aprovação | ⏳ Pendente |
| `/hero-reject` | **CLI direto:** `svc.Reject("")` — motivo vazio | **Orquestrador:** `hero reject --reason`, reexecuta o agente da etapa com feedback | TUI só muda estado no SQLite; chat **re-dispara** o trabalho da etapa | ⏳ Pendente |
| `/hero-cancel` | **CLI direto:** `svc.Cancel("")` | **Orquestrador:** `hero cancel` + **git checkout/restore** para rollback | TUI **não** faz rollback git (só marca ciclo cancelado no store) | ⏳ Pendente |
| `/hero-finish` | **CLI direto:** `svc.Finish("")` — sem métricas | **Orquestrador:** valida etapas, `hero finish --metrics-json`, atualiza `context-log.md` / `current-state.md`, `metrics-summary.md` | TUI só fecha no SQLite; chat faz fechamento “completo” com artefatos de contexto | ⏳ Pendente |
| `/hero-continue` | **CLI direto:** `svc.Continue(1)` — sempre **+1** fixo | **Orquestrador:** `hero continue --extra N` (N opcional) e **retoma execução** da etapa | TUI não aceita N customizado nem reexecuta a etapa | ⏳ Pendente |
| `/hero-back` | **Dispatch:** lê `hero-back.md` → `HarnessAdapter.Dispatch` (best-effort) | **Orquestrador:** reabre Planning via `planning_agent` (Task), reexecuta Implementation → QA → Judge | TUI tenta empurrar o markdown ao Agent CLI; chat faz orquestração real (sem verbo CLI `hero back`) | ⏳ Pendente |
| `/hero-sync` | **Dispatch:** lê `hero-sync.md` → `HarnessAdapter.Dispatch` (best-effort) | **Orquestrador:** Task `context_agent`, gera `AGENTS.md` / `context/*`, scan de docs pendentes, `hero doctor` | TUI depende do Agent CLI aceitar o prompt; chat executa o sync completo com raciocínio e escrita de arquivos | ⏳ Pendente |
| `/hero-status` | **CLI direto:** `svc.Status()` → **uma linha** (`Cycle Cn — title (status)`) | **Orquestrador:** `hero status` (tabela completa de etapas) | TUI mostra resumo mínimo; chat mostra visão detalhada | ⏳ Pendente |
| `/hero-archive` | **CLI direto:** `svc.Archive()` → `hero cycle archive` (OpenSpec acoplado no Go) | **Orquestrador:** confirma via `hero status`, chama `hero cycle archive`, pode atualizar `metrics-summary.md` | **Mesmo CLI** no núcleo; chat adiciona validação narrativa e artefatos opcionais | ⏳ Parcial |
| `/hero-resume` | **CLI direto:** `svc.Resume(0)` — **sempre o último** ciclo | **Orquestrador:** `hero cycle resume [--number N]` (N opcional) | TUI não permite escolher número do ciclo | ⏳ Pendente |
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
- Comandos importados e `/hero-sync`, `/hero-back` ainda usam **Dispatch**.
- Free chat na tela Chat = `conversationStage == ""`; sessão do harness fica só em memória da TUI.

---

## Schema — padrão de execução (TUI × Chat)

```
                    TUI                              Chat
                     │                                │
         ┌───────────┼───────────┬──────────┐        │
         │           │           │          │        │
    CLI direto   Go local    Dispatch   Runtime     Orquestrador
    (approve,    (cycles,    (sync,     Execute    (lê hero-*.md,
     archive,     todos)      back)     (new,start) chama hero CLI,
     etc.)                               +orch.md*  Task, git, files)
         │           │           │          │        │
         └───────────┴───────────┴──────────┘        │
                     │                                │
                     └────────────┬───────────────────┘
                                  ▼
                         hero.db (SQLite)
                    (mesmo store quando o CLI é chamado)
```

\* `/hero-start` na TUI lê `orchestration_agent.md` + `hero-start.md` via Runtime Execute (não mais `RunWith()` → Dispatch).

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

- Comandos de **controle de estado** (`approve`, `reject`, `cancel`, `finish`, `archive`, `resume`, `continue`) na TUI são atalhos CLI **sem LLM**, muitas vezes **sem métricas, motivo ou git**.
- Comandos de **trabalho** (`sync`, `back`) no chat são orquestração completa; na TUI ainda são dispatch best-effort — **`/hero-new` e `/hero-start`** alinhados via Runtime Execute.
- Comandos de **consulta** (`status`, `cycles`, `todos`) na TUI são determinísticos; no chat passam pelo orquestrador com saída mais rica.
- `/hero-model` e `/hero-help` têm comportamentos **assimétricos** por design.

**Próximo alvo de alinhamento sugerido:** `/hero-approve` (métricas + summary na TUI).
