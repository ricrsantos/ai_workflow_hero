# Plano: alinhamento `/hero-sync`, `/hero-status`, `/hero-archive`, `/hero-resume` (TUI × Chat)

> **Status:** implementado (2026-08-12).

Non-normative idea note. Para verdade de produto, ver PRD/UI/ADR (C2 slash parity, C3 harness autonomy).

---

## Contexto

| Comando | TUI antes | Chat (Cursor) |
|---|---|---|
| `/hero-sync` | **Dispatch** — `heroAssetCmd("sync")` + `/hero-model` | Orquestrador + Task `context_agent`, `AGENTS.md` / `context/*`, `hero doctor` |
| `/hero-status` | **CLI direto** — uma linha via `statusCmd()` | Orquestrador + `hero status` (tabela completa) |
| `/hero-archive` | **CLI direto** — `svc.Archive()` | Orquestrador + `hero status`, `hero cycle archive`, opcional `metrics-summary.md` |
| `/hero-resume` | **CLI direto** — `svc.Resume(0)` sempre o último | Orquestrador + `hero cycle resume [--number N]` + `hero status` |

Referência de padrão: [`hero-approve-alignment-plan.md`](hero-approve-alignment-plan.md), [`hero-control-alignment-plan.md`](hero-control-alignment-plan.md).

---

## Objetivo de paridade (todos)

1. **Gates em Go** (feedback imediato).
2. **Runtime Execute** — Chat + stream, `orchestration_agent.md` + `hero-*.md`.
3. Modelo: `agents.orchestration_agent` quando `workflow-config.yml` existe; fallback `/hero-model` para sync/status/resume (bootstrap).
4. `ExecuteRequest.AgentName = "orchestration_agent"`; sessão nova.

---

## `/hero-sync`

- Gates: não streaming; `svc` disponível; modelo (YAML ou `/hero-model`).
- Sem ciclo ativo exigido.
- Preamble: Task `context_agent`, artefatos, ADR-029 scan, `hero doctor`, CTA `/hero-todos` e `/hero-new` na TUI.

## `/hero-status`

- Gates: não streaming; `svc`; modelo híbrido.
- Sem ciclo exigido; abre Chat (não Status screen).
- Preamble: `hero status` tabela completa.

## `/hero-archive`

- Gates: `validateOrchestratorPreconditions()` (ciclo ativo + modelo orquestrador).
- Agente invoca `hero cycle archive` (OpenSpec acoplado no CLI).

## `/hero-resume`

- Gates: não streaming; `svc`; modelo híbrido.
- `/hero-resume [N]` no Chat input (`parseHeroResumeInline`).
- `heroRuntimeOpts.ResumeCycleNumber` (0 = último).

---

## Arquivos alterados

- `internal/tui/app.go` — `beginHeroSync/Status/Archive/Resume`, `resolveOrchestratorOrDefaultModel`; remover `statusCmd`, `archiveCmd`, `resumeCmd`, `heroAssetCmd`
- `internal/tui/conversation.go` — `usesOrchestratorRuntime`, `ResumeCycleNumber`, `parseHeroResumeInline`
- `internal/tui/chat_format.go` — 4 preambles
- Testes em `app_test.go`, `conversation_test.go`, `chat_format_test.go`, `status_bar_test.go`
