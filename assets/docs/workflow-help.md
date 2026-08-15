# AI Workflow Hero — User Guide

> Installed at `.workflow-hero/docs/workflow-help.md` by `hero install`.
> Bilingual: English first, then Português (BR). This is Hero tool documentation (not a project-cycle artifact).

<p align="center">
  <a href="#english">🇺🇸 English</a> |
  <a href="#portugues-br">🇧🇷 Português</a>
</p>

---

<a id="english"></a>

# 🇺🇸 English

## 1. Philosophy

**AI Workflow Hero (Hero)** is an open-source framework for AI-augmented software development. It does **not** replace your coding agent. It coordinates specialized subagents, organizes project artifacts, compresses context, and makes development cycles reproducible — without locking you to a single LLM provider.

Core ideas:

| Idea | Meaning |
|------|---------|
| **CLI vs Runtime** | The `hero` CLI is deterministic (install, upgrade, doctor…). All reasoning runs in Cursor chat via `/hero-<name>` commands. |
| **Development Cycle** | One unit of work (feature, bugfix, greenfield). Stages can be enabled or disabled per cycle. |
| **Project vs Hero artifacts** | Permanent knowledge lives in `AGENTS.md`, `docs/`, `context/`, `openspec/`. Hero-only state lives under `.workflow-hero/`. |
| **Context compression** | Agents keep `current-state.md` and `context-log.md` up to date so later sessions stay cheap and consistent. |
| **Human in the loop** | Stages can require approval; escalation waits for `/hero-continue`; Judge SDD ambiguity uses `/hero-back` or `/hero-approve`. |
| **Determinism where it matters** | Specs, ADRs, tests, logging standards, and scope routing reduce “prompt lottery” outcomes. |

Stage flow:

```text
Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End
```

**Configuration** always runs implicitly and is not toggleable. Every other stage is configured in `workflow-config.yml`.

---

## 2. What Hero produces and enforces

During a cycle, Hero (via Runtime agents) typically:

- **Researches** requirements (grilling) and generates project docs: PRD, **ADR (architecture)**, UI, DEPLOY, TESTING — with cycle numbering where applicable.
- **Plans** an OpenSpec SDD with ordered, testable tasks.
- **Implements** via scope-routed agents (`backend` / `frontend` / `generic`), with parallel Task fan-out when tasks are independent.
- **Requires application logging** on new/changed code: levels `error`, `info`, `debug`; **default level `info`**.
- **Validates** with QA (tests, lint, build, architecture consistency, **logging checks**), Judge (SDD coverage), optional **Browser UI Validation** (Playwright Health + optional Visual vs PNG refs), and End-to-End (HTTP or optional Playwright journeys).
- **Tracks metrics** (estimated tokens/cost per stage) and updates context compression files.
- **Protects secrets** softly: prefer `.env.example`; never commit real secrets.

Architecture rule: do **not** change architecture without an approved ADR.

---

## 3. Requirements

- Linux or macOS (`amd64` or `arm64`)
- [Cursor](https://cursor.com) IDE (V1)
- Git (required for `/hero-cancel` checkpoints; `hero install` can run `git init` with consent)
- [OpenSpec](https://github.com/Fission-AI/OpenSpec) for Planning (installed with consent when needed)

---

## 4. Install the CLI binary

### From a release (recommended)

1. Download the artifact for your OS/arch from [Releases](https://github.com/ricrsantos/ai_workflow_hero/releases).
2. Optionally verify SHA256 against `checksums.txt`.
3. Install on your `PATH`:

```bash
chmod +x hero_v1.0.0_linux_amd64
sudo mv hero_v1.0.0_linux_amd64 /usr/local/bin/hero
hero version
```

### From source

```bash
git clone https://github.com/ricrsantos/ai_workflow_hero.git
cd ai_workflow_hero
go build -ldflags "-X main.version=$(git describe --tags --abbrev=0 2>/dev/null || echo dev)" -o hero ./cmd/hero
sudo mv hero /usr/local/bin/hero
```

---

## 5. Install Hero into a project

Run inside the target project directory:

```bash
# Interactive
hero install --tools cursor

# Fully scripted (CI)
hero install --tools cursor --name "My Project" --summary "Short summary" --yes --git-init
```

Flags:

| Flag | Purpose |
|------|---------|
| `--tools cursor` | Required in V1 (only Cursor) |
| `--name` | Project name (required with `--yes`) |
| `--summary` | Optional project summary |
| `--yes` | Skip interactive prompts |
| `--git-init` | Initialize git if the directory is not a repo |

What install creates (overview):

| Path | Purpose |
|------|---------|
| `.cursor/commands/hero-*.md` | Runtime slash commands |
| `.cursor/agents/*.md` | Specialized agents |
| `.cursor/skills/` | `workflow-hero`, `grilling` |
| `.workflow-hero/config/` | `hero.json`, `project.json`, `documents.json`, `checksums.json` |
| `.workflow-hero/templates/` | Cycle templates (incl. `workflow-config.yml`) |
| `.workflow-hero/models/` | Model pricing YAML |
| `.workflow-hero/cycles/` | Cycle state |
| `.workflow-hero/docs/workflow-help.md` | **This guide** |
| `.workflow-hero/metrics-summary.md` | Project-wide metrics |
| `.env.example` / `.gitignore` patterns | Soft secrets hygiene (when missing) |

After a successful install the CLI prints:

```text
✓ Hero installed successfully.
→ Full user guide: .workflow-hero/docs/workflow-help.md
```

---

## 6. Uninstall

```bash
hero uninstall
```

Removes **only Hero-owned paths**: `.workflow-hero/`, Hero agents/skills under `.cursor/`, and `.cursor/commands/hero-*.md`.

**Preserved:** `AGENTS.md`, `docs/`, `context/`, `openspec/`, `.env.example`, `.gitignore`, and your application code.

---

## 7. Upgrade

```bash
hero upgrade
```

Re-copies Hero assets from the binary. Files you customized are **not** silently overwritten (checksum comparison → warning + skip). Also refreshes soft secrets hygiene files/patterns when needed, and migrates legacy `generic_model` → `fallback_model` in workflow configs when applicable.

---

## 8. Configure a cycle

Typical first Runtime steps:

1. `/hero-sync` — activate Hero on an existing codebase (recommended)
2. `/hero-new` — start a new cycle (writes `.workflow-hero/cycles/current/`)
3. Edit `.workflow-hero/cycles/current/workflow-config.yml` (fill `title` / `objective` / `scope`; optionally set `workflow_config.user_preferred_language`)
4. Open a **new empty chat**, select the agent (model) you want as the Hero **orchestrator / grill-me**, then run `/hero-start`

When prior cycles exist, `/hero-new` **always imports** `workflow_config`, `fallback_model`, `stages`, and `agents` from the previous cycle’s `workflow-config.yml` into the new file. `title`, `objective`, and `scope` are always reset to template defaults (cycle-specific). The first cycle uses the blank template only.

### 8.0 Clean session after configuration

After `/hero-new`, prefer a **new empty chat** for `/hero-start` so the orchestrator session does not carry grilling/Q&A from configuration (saves context window for later stages). Soft guidance — Hero still works if you continue in the same chat, but a clean session is recommended.

In the new chat, **select the IDE agent/model you want as the Hero orchestrator / grill-me** before `/hero-start`. That session model drives orchestration; specialized agents still use models from `workflow-config.yml` via the Task tool.

### 8.1 Key fields in `workflow-config.yml`

| Section | Purpose |
|---------|---------|
| `title` / `objective` | Cycle identity and goal |
| `workflow_config.user_preferred_language` | Chat language for all agents talking to the user (default `EN`). Cycle artifacts stay English. User may ask for a different chat language in-session. |
| `scope` | Booleans: `backend`, `frontend`, `native`, `script`, `infrastructure` (at least one must be true when Implementation is enabled) |
| `stages.*` | Per stage: `enabled`, `purpose`, `max_iterations`, `timeout_minutes`, `require_human_approval` |
| `stages.browser_ui_validation` | Default `enabled: false`. Requires `scope.frontend: true` when enabled. Always runs **Browser Health** (Playwright). Optional nested `visual_validation.enabled` + `visual_validation.reference_dir` (default `docs/ui/visual_reference`) |
| `stages.qa_end_to_end.use_playwright` | `true` → Playwright journeys (requires `scope.frontend: true`); `false` → direct HTTP. Independent of Browser UI Validation |
| `agents.*` | Per-agent model settings (`model`, `reasoning_effort`, `enable_fast_model`, `thinking`) plus nested `subagent` (`same_of_agent` + model fields) for nested Task fan-out |
| `fallback_model` | Used when an agent’s or nested `subagent`’s model is unavailable (`model`, `reasoning_effort`, `enable_fast_model`, `thinking`) |
| `workflow_rules` | Guardrails the orchestrator must follow |

### 8.2 Scope → implementation agent

| Scope flag | Agent |
|------------|-------|
| `backend` | `backend_agent` |
| `frontend` | `frontend_agent` |
| `native` / `script` / `infrastructure` | `generic_agent` |

### 8.3 Browser UI Validation

When `stages.browser_ui_validation.enabled: true` (requires `scope.frontend: true`):

1. **Browser Health** always runs via `browser_ui_agent` + Playwright: open app, render check, console errors, failed network (CSS/JS/images/fonts/APIs), CSS load. Desktop viewport 1280. Playwright missing at execution → Health failure → fix loop to `frontend_agent`.
2. **Visual Validation** runs only if Health passed **and** `visual_validation.enabled: true`. Agent captures screenshots at 1280 / 768 / 375 and compares with agent vision against PNGs named `<screen-id>.png` under `reference_dir`. Missing PNG → warn and continue (not a failure). Empty/missing dir → one warning, skip Visual.
3. Artifacts: `.workflow-hero/cycles/current/browser-ui/` (`health-report.md`, `screenshots/`, optional `visual-report.md`). User reference PNGs are never overwritten.
4. Failure routing: asset/console/render/visual → `frontend_agent`; clearly classified API failures → `backend_agent`.

Prerequisite: install/configure Playwright in the **consumer project** (Hero does not auto-install it).

QA End-to-End Playwright journeys (`use_playwright`) remain separate business flows.

### 8.4 Approval and control

- `require_human_approval: true` → stage waits for `/hero-approve`, `/hero-reject`, `/hero-cancel`, or `/hero-finish`.
- `require_human_approval: false` → stage auto-advances after summary (you can still interrupt before the next stage starts).
- Iteration/timeout exhaustion → escalates; grant more work with `/hero-continue`.
- Judge finds SDD ambiguity → `/hero-back` (reopen Planning) or `/hero-approve` (accept as-is).

### 8.5 Model fallback

1. Agent’s configured model  
2. Top-level `fallback_model` (user is always warned)  
3. Still unavailable → wait for `/hero-continue` after you fix config  

Use **Cursor/Task model ids** in `agents.*.model` (e.g. `cursor-grok-4.5`, not the bare xAI id `grok-4.5`). The orchestrator passes Task a **kebab slug** built from `enable_fast_model` / `reasoning_effort` / `thinking` (e.g. `cursor-grok-4.5-high`, `composer-2.5-fast`). Bracket forms like `id[fast=false,effort=high]` are not accepted by Cursor Task. When an orchestrator-dispatched agent launches a **nested generic Task**, resolve `agents.*.subagent` (`same_of_agent: true` or missing → parent model; `false` → `subagent.model`). Named Hero agents (e.g. `context_agent`) always use their own top-level block.

---

## 9. CLI commands

| Command | Purpose |
|---------|---------|
| `hero install --tools cursor` | Bootstrap Hero into the project |
| `hero upgrade` | Update Hero assets (checksum-safe) |
| `hero uninstall` | Remove Hero-owned paths only |
| `hero doctor` | Health checks (table; `--json` for CI). Secrets issues are **warnings** only |
| `hero status` | Current cycle stage table (`--json` supported) |
| `hero variables` | Show key `project.json` / `hero.json` fields (`--json` supported) |
| `hero update-models` | Refresh structured model pricing from upstream |
| `hero version` | Print CLI version |
| `hero help` | Cobra help |
| Global `--verbose` / `--debug` | Reserved for verbose/debug output (stack traces per UI spec) |

Examples:

```bash
hero doctor
hero status --json
hero variables
hero update-models
```

---

## 10. Runtime commands (Cursor chat)

| Command | Purpose |
|---------|---------|
| `/hero-new` | Start a new development cycle |
| `/hero-start` | Execute configured stages |
| `/hero-approve` | Approve current stage and advance |
| `/hero-reject` | Reject and re-run current stage |
| `/hero-cancel` | Cancel stage and restore git checkpoint |
| `/hero-continue` | Grant extra iterations after escalation |
| `/hero-back` | Reopen Planning after SDD ambiguity |
| `/hero-finish` | Finish the cycle via `hero finish` (records `completed_at` in SQLite) |
| `/hero-archive` | Archive current cycle via `hero cycle archive` (folder date from store `completed_at`) |
| `/hero-resume` | Restore an archived cycle |
| `/hero-sync` | Activate / re-sync Hero on an existing project (also merges pending items from `docs/product/` and `docs/architecture/` into `current-state.md`) |
| `/hero-status` | Show cycle status in chat |
| `/hero-cycles` | List all cycles with per-etapa metrics (SQLite + archive folders) |
| `/hero-todos` | Show pending items from `context/current-state.md` (run `/hero-sync` first when docs changed) |
| `/hero-model` | Select TUI default model (persists to `hero.json`; Chat screen + non-agent dispatches) |
| `/hero-help` | List Runtime commands |

---

## 11. Agents (who does what)

| Agent | Role |
|-------|------|
| `orchestration_agent` | Orchestrates the loop (IDE session model in chat; TUI Execute uses `agents.orchestration_agent`, then `fallback_model`, then `/hero-model`) |
| `discover_agent` | Research / grilling → specs. TUI uses `agents.discover_agent` in workflow-config.yml; Cursor IDE chat ignores that block (same session as the orchestrator) |
| `planning_agent` | OpenSpec SDD |
| `context_agent` | On-demand project context (read-only) |
| `backend_agent` | Backend implementation + logging standard |
| `frontend_agent` | Frontend implementation + logging standard |
| `generic_agent` | Native / script / infrastructure + logging standard |
| `qa_agent` | Tests, coverage, lint, build, architecture, **logging checks** |
| `judge_agent` | SDD requirement coverage only |
| `browser_ui_agent` | Browser Health (Playwright) + optional Visual vs PNG refs |
| `end2end_qa_agent` | End-to-end journeys (HTTP or Playwright) |

---

## 12. Documents, architecture, and logging

### Documents

Research decides which of PRD / ADR / UI / DEPLOY / TESTING to create and registers them in `.workflow-hero/config/documents.json`.

- Numbered cycle docs: `[CATEGORY]-C[XX]-[seq]-[slug].md` (e.g. `PRD-C04-001-checkout.md`)
- Living docs (edited in place): `DEPLOY.md`, `TESTING.md`
- Indexes: `docs/product/PRD.md`, `docs/architecture/ADR.md`
- Cycle artifacts are written in **English** regardless of chat language
- Chat language is set by `workflow_config.user_preferred_language` (default `EN`); agents follow it unless you ask otherwise

### Architecture

Architecture decisions are captured as **ADRs**. Implementation agents must not change architecture without an approved ADR. QA checks architecture consistency (e.g. unapproved dependencies, circular imports).

### Logging

Implementation agents must add application logs on new/changed paths:

| Level | Use |
|-------|-----|
| `error` | Failures that need attention |
| `info` | Significant lifecycle / business events (**default emitting level**) |
| `debug` | Diagnostics; must not emit when the runtime level is `info` |

Never log secrets, tokens, or PII. Prefer the project’s existing logger; otherwise introduce an appropriate leveled logger. `qa_agent` fails the QA stage when this standard is missing or violated.

---

## 13. Metrics and context files

| File / command | Purpose |
|------|---------|
| `.workflow-hero/hero.db` | SQLite operational store (cycle/stage status, events, metrics) |
| `hero status` / `hero metrics` / `hero events` | Query operational state (table or `--json`) |
| `.workflow-hero/metrics-summary.md` | Aggregated across cycles (optional project summary) |
| `context/current-state.md` | Long-lived project truth |
| `context/context-log.md` | Short/medium decision log |
| `AGENTS.md` | Stable agent instructions for the project |

Token/cost estimates use character count ÷ ~4 × prices from `.workflow-hero/models/*.yml`, then persist via `hero … --metrics-json` (not cycle `metrics.md`).

---

## 14. Recommended first session

```text
hero install --tools cursor
# → read .workflow-hero/docs/workflow-help.md

# In Cursor:
/hero-sync
/hero-new
# edit workflow-config.yml (scope, stages, models)

# New empty chat → select orchestrator / grill-me agent → then:
/hero-start
```

Use `/hero-status` or `hero status` anytime. When done: `/hero-finish` or `/hero-archive`.

---

## 15. Further reading (Hero source repository)

| Document | Path |
|----------|------|
| Product Requirements | `docs/product/PRD.md` |
| Terminal UX | `docs/product/UI.md` |
| ADRs | `docs/architecture/ADR.md` |
| Deployment | `docs/deployment/DEPLOY.md` |

---

<a id="portugues-br"></a>

# 🇧🇷 Português (BR)

## 1. Filosofia

**AI Workflow Hero (Hero)** é um framework open-source para desenvolvimento de software aumentado por IA. Ele **não** substitui o agente de código. Ele coordena subagentes especializados, organiza artefatos, comprime contexto e torna ciclos reproduzíveis — sem prender você a um único provedor de LLM.

Ideias centrais:

| Ideia | Significado |
|-------|-------------|
| **CLI vs Runtime** | A CLI `hero` é determinística. Todo raciocínio ocorre no chat do Cursor via `/hero-<name>`. |
| **Ciclo de desenvolvimento** | Unidade de trabalho (feature, bug, projeto novo). Stages ligáveis/desligáveis por ciclo. |
| **Artefatos de projeto vs Hero** | Conhecimento permanente: `AGENTS.md`, `docs/`, `context/`, `openspec/`. Estado do Hero: `.workflow-hero/`. |
| **Compressão de contexto** | `current-state.md` e `context-log.md` mantidos atualizados. |
| **Humano no loop** | Aprovação por stage, escalonamento com `/hero-continue`, ambiguidade de SDD com `/hero-back` ou `/hero-approve`. |
| **Determinismo onde importa** | Specs, ADRs, testes, padrão de logs e roteamento por scope. |

Fluxo de stages:

```text
Configuration → Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End
```

**Configuration** sempre roda e não é configurável. Os demais stages ficam em `workflow-config.yml`.

---

## 2. O que o Hero produz e exige

Em um ciclo, o Hero (via Runtime) tipicamente:

- Faz **Research** (grilling) e gera docs: PRD, **ADR (arquitetura)**, UI, DEPLOY, TESTING.
- **Planeja** um SDD OpenSpec com tarefas ordenadas e testáveis.
- **Implementa** com agentes por scope, com fan-out paralelo quando possível.
- **Exige logging** em código novo/alterado: `error`, `info`, `debug`; **nível padrão `info`**.
- **Valida** com QA (incluindo checagem de logs), Judge (cobertura da SDD), **Browser UI Validation** opcional (Health Playwright + Visual vs PNGs) e End-to-End (HTTP ou jornadas Playwright opcionais).
- Registra **métricas** e atualiza compressão de contexto.
- Protege **secrets** de forma suave (`.env.example`; nunca commit de segredos reais).

Regra de arquitetura: **não** alterar arquitetura sem ADR aprovado.

---

## 3. Requisitos

- Linux ou macOS (`amd64` ou `arm64`)
- IDE [Cursor](https://cursor.com) (V1)
- Git (obrigatório; `hero install` pode rodar `git init` com consentimento)
- [OpenSpec](https://github.com/Fission-AI/OpenSpec) para Planning (instalado com consentimento quando necessário)

---

## 4. Instalar o binário CLI

### A partir de um release (recomendado)

```bash
chmod +x hero_v1.0.0_linux_amd64
sudo mv hero_v1.0.0_linux_amd64 /usr/local/bin/hero
hero version
```

Verifique o checksum em `checksums.txt` quando disponível. Releases: [GitHub Releases](https://github.com/ricrsantos/ai_workflow_hero/releases).

### A partir do código-fonte

```bash
git clone https://github.com/ricrsantos/ai_workflow_hero.git
cd ai_workflow_hero
go build -ldflags "-X main.version=$(git describe --tags --abbrev=0 2>/dev/null || echo dev)" -o hero ./cmd/hero
sudo mv hero /usr/local/bin/hero
```

---

## 5. Instalar o Hero em um projeto

```bash
hero install --tools cursor
# ou
hero install --tools cursor --name "Meu Projeto" --summary "Resumo" --yes --git-init
```

Após o sucesso:

```text
✓ Hero installed successfully.
→ Full user guide: .workflow-hero/docs/workflow-help.md
```

Este arquivo (`workflow-help.md`) é copiado para `.workflow-hero/docs/` na instalação.

---

## 6. Desinstalar

```bash
hero uninstall
```

Remove apenas caminhos do Hero. Preserva `AGENTS.md`, `docs/`, `context/`, `openspec/` e o código da aplicação.

---

## 7. Atualizar (upgrade)

```bash
hero upgrade
```

Atualiza assets com proteção por checksum (customizações locais não são sobrescritas em silêncio).

---

## 8. Configurar um ciclo

1. `/hero-sync` (recomendado em codebases existentes)
2. `/hero-new`
3. Editar `.workflow-hero/cycles/current/workflow-config.yml` (preencher `title` / `objective` / `scope`; opcionalmente `workflow_config.user_preferred_language`)
4. Abrir um **chat novo e vazio**, selecionar o agente (modelo) que deseja como **orchestrator / grill-me** do Hero, e então rodar `/hero-start`

Quando já existem ciclos anteriores, o `/hero-new` **sempre importa** `workflow_config`, `fallback_model`, `stages` e `agents` do ciclo anterior; `title`, `objective` e `scope` voltam ao padrão do template.

Após o `/hero-new`, prefira um chat limpo para o `/hero-start` (orientação soft) — evita carregar grilling/Q&A da configuração na janela de contexto. No chat novo, escolha o agente/modelo da sessão IDE que fará o papel de orchestrator / grill-me antes de iniciar.

Configure `workflow_config.user_preferred_language`, `scope`, `stages`, `agents`, `fallback_model`, `stages.browser_ui_validation` e `stages.qa_end_to_end.use_playwright` conforme a seção em inglês (§8) — os campos são os mesmos. Browser UI Validation exige Playwright no projeto consumidor; artefatos em `.workflow-hero/cycles/current/browser-ui/`.

---

## 9. Comandos CLI

`hero install`, `upgrade`, `uninstall`, `doctor`, `status`, `variables`, `update-models`, `version`, `help` — detalhes na tabela da §9 (inglês). Use `--json` em `doctor` / `status` / `variables` para scripts.

---

## 10. Comandos Runtime (chat do Cursor)

`/hero-new`, `/hero-start`, `/hero-approve`, `/hero-reject`, `/hero-cancel`, `/hero-continue`, `/hero-back`, `/hero-finish`, `/hero-archive`, `/hero-resume`, `/hero-sync`, `/hero-status`, `/hero-cycles`, `/hero-todos`, `/hero-model`, `/hero-help` — ver tabela da §10 (inglês).

---

## 11. Agentes, documentos, arquitetura e logs

- **Agentes:** orquestração, discovery (Research + docs; no TUI o modelo vem de `agents.discover_agent`; no chat do Cursor o YAML é ignorado), planning (OpenSpec), context, backend/frontend/generic (implementação + logs), QA (inclui logs), Judge, Browser UI Validation (`browser_ui_agent`), End-to-End.
- **Documentos:** PRD/ADR/UI/DEPLOY/TESTING; ADRs definem arquitetura.
- **Logs:** `error` / `info` / `debug`, default `info`; QA falha se o padrão não for seguido.
- **Secrets:** só `.env.example` no git; valores reais em `.env` local.

---

## 12. Sessão inicial recomendada

```text
hero install --tools cursor
# leia .workflow-hero/docs/workflow-help.md

/hero-sync
/hero-new
# edite workflow-config.yml

# Chat novo e vazio → selecione orchestrator / grill-me → depois:
/hero-start
```

---

_End of user guide / Fim do guia do usuário._
