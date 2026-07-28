<p align="center">
  <a href="#english">🇺🇸 English</a> |
  <a href="#portugues-br">🇧🇷 Português</a>
</p>

<h1 align="center">AI Workflow Hero</h1>

<p align="center">
  Framework open-source for AI-augmented software development.
  <br>
  Coordinates specialized subagents, compresses context, and makes development cycles reproducible — without locking you to a single LLM.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS-blue" alt="Linux macOS">
  <img src="https://img.shields.io/badge/arch-amd64%20%7C%20arm64-purple" alt="amd64 arm64">
  <img src="https://img.shields.io/badge/built%20with-Go%20%7C%20Cobra-orange" alt="Go Cobra">
  <img src="https://img.shields.io/badge/IDE-Cursor%20(V1)-4A86CF" alt="Cursor V1">
  <img src="https://img.shields.io/badge/license-BSD--2--Clause-green" alt="BSD-2-Clause">
</p>

<p align="center">
  <img src="docs/images/1024_light_transparente.png" alt="AI Workflow Hero" width="512">
</p>

---

<a id="english"></a>

# 🇺🇸 English

**AI Workflow Hero (Hero)** is an open-source framework for AI-augmented software development. It does **not** replace your coding agent — it coordinates specialized subagents, organizes project artifacts, compresses context, and makes AI-driven development cycles reproducible and less dependent on any single LLM provider.

Hero ships as a single Go CLI binary (`hero`) that bootstraps a project with commands, skills, prompts, and templates for **Cursor** (V1). Once installed, the reasoning-driven workflow runs entirely in the IDE chat via `/hero:*` slash commands — never inside the CLI binary.

Designed for an open-source workflow, it helps you move through:

**Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End**

---

## Features

- Deterministic CLI for install, upgrade, uninstall, doctor, status, variables, and model pricing updates
- Cursor Runtime assets: 13 `/hero:*` commands and 11 specialized agents
- Configurable stage flow with human approval, iteration limits, timeouts, and escalation
- Scope routing to `backend_agent`, `frontend_agent`, or `generic_agent`
- Three-level model fallback (`agent model` → `fallback_model` → wait for `/hero:continue`)
- Context compression files (`AGENTS.md`, `context/current-state.md`, `context/context-log.md`)
- OpenSpec integration for SDD planning and task-driven implementation
- Research produces project specifications (PRD, ADR, UI, DEPLOY, TESTING) with cycle numbering; architecture changes require an approved ADR
- Implementation logging standard (`error` / `info` / `debug`, default `info`) enforced by `qa_agent`
- Soft secrets hygiene: `.env.example` + `.gitignore` patterns; `hero doctor` warns, never blocks
- Optional Browser UI Validation (`stages.browser_ui_validation`, requires `scope.frontend`): Playwright Health + optional Visual vs PNGs under `docs/ui/visual_reference`
- Opt-in Playwright for QA End-to-End journeys (`stages.qa_end_to_end.use_playwright`, requires `scope.frontend`) — distinct from Browser UI Validation
- Parallel Task fan-out for independent implementation work; clean subagent sessions via file pointers
- Per-cycle metrics and project-wide cost estimates from structured model pricing files
- Upgrade safety: customized Hero files are never silently overwritten (checksum comparison)
- Uninstall removes only Hero-owned paths; project knowledge (`AGENTS.md`, `docs/`, `context/`, `openspec/`) is preserved
- Installed user guide at `.workflow-hero/docs/workflow-help.md` (path printed after `hero install`)

---

## Installation

Ready-to-use binaries will be published in the repository [Releases](https://github.com/ricrsantos/ai_workflow_hero/releases) section.

### From a release binary (recommended)

1. Download the artifact matching your OS/architecture, for example:
   - `hero_v1.0.0_linux_amd64`
   - `hero_v1.0.0_linux_arm64`
   - `hero_v1.0.0_darwin_amd64`
   - `hero_v1.0.0_darwin_arm64`
2. (Optional) Verify the SHA256 checksum against `checksums.txt`.
3. Make it executable and place it on your `PATH`:

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

## Quick Start

Inside the target project (must be a git repository, or pass `--git-init`):

```bash
# Interactive
hero install --tools cursor

# Fully scripted (CI-friendly)
hero install --tools cursor --name "My Project" --summary "Short project summary" --yes --git-init
```

Then, in Cursor chat:

1. `/hero:sync` — activate Hero in an existing codebase (optional but recommended)
2. `/hero:init` — start a new development cycle
3. Edit `.workflow-hero/cycles/current/workflow-config.yml`
4. Open a **new empty chat**, select the agent you want as the Hero **orchestrator / grill-me**, then `/hero:start`

After install, read the full user guide at `.workflow-hero/docs/workflow-help.md` (philosophy, configuration, CLI and Runtime commands).

Useful CLI commands after install:

```bash
hero doctor
hero status
hero status --json
hero variables
hero upgrade
hero update-models
hero uninstall
```

---

## Runtime Commands (Cursor chat)

| Command | Purpose |
|---|---|
| `/hero:init` | Start a new development cycle (then prefer a new chat for start) |
| `/hero:start` | Execute configured stages (prefer new empty chat; select orchestrator / grill-me first) |
| `/hero:approve` | Approve the current stage |
| `/hero:reject` | Reject and request changes |
| `/hero:cancel` | Cancel the stage and restore the git checkpoint |
| `/hero:continue` | Grant extra iterations after escalation |
| `/hero:back` | Reopen Planning after SDD ambiguity |
| `/hero:finish` | Finish the cycle; writes Completed date used by archive naming |
| `/hero:archive` | Archive the current cycle (folder date = workflow.md Completed) |
| `/hero:resume` | Resume an archived cycle |
| `/hero:sync` | Activate / re-sync Hero on an existing project |
| `/hero:status` | Show cycle status in chat |
| `/hero:help` | List Runtime commands |

---

## Requirements

### End users

- Linux or macOS (`amd64` or `arm64`)
- [Cursor](https://cursor.com) IDE (V1)
- Git (required; `hero install` can run `git init` with consent)
- [OpenSpec](https://github.com/Fission-AI/OpenSpec) for Planning-stage workflows (installed with user consent when needed)

### Development

- Go 1.25+ (see `go.mod`)
- Git

```bash
git clone https://github.com/ricrsantos/ai_workflow_hero.git
cd ai_workflow_hero
go test ./...
go build -o hero ./cmd/hero
```

---

## Build and Test

```bash
go test ./...
go build -ldflags "-X main.version=$(git describe --tags --abbrev=0)" -o hero ./cmd/hero
```

Cross-compiled release artifacts (4 platforms + `checksums.txt`):

```bash
./scripts/release.sh
# → dist/hero_<version>_linux_amd64
# → dist/hero_<version>_linux_arm64
# → dist/hero_<version>_darwin_amd64
# → dist/hero_<version>_darwin_arm64
# → dist/checksums.txt
```

---

## Project Structure

```text
.
├── cmd/hero/                 # CLI entrypoint (Cobra)
├── internal/
│   ├── install/              # hero install
│   ├── upgrade/              # hero upgrade
│   ├── uninstall/            # hero uninstall
│   ├── doctor/               # hero doctor
│   ├── status/               # hero status
│   ├── variables/            # hero variables
│   ├── update_models/        # hero update-models
│   ├── adapters/cursor/      # Cursor path layout
│   ├── common/               # errors, output, templates
│   └── integration/          # lightweight e2e tests
├── assets/                   # embed.FS (commands, agents, skills, templates, models)
├── scripts/release.sh        # manual cross-compile release
├── docs/                     # PRD, UI, ADR, DEPLOY
├── context/                  # this repo's compressed project memory
├── openspec/                 # SDD / change planning for Hero itself
├── AGENTS.md
├── go.mod
└── README.md
```

---

## Documentation

| Document | Path |
|---|---|
| Product Requirements | [docs/product/PRD.md](docs/product/PRD.md) |
| Terminal UX Spec | [docs/product/UI.md](docs/product/UI.md) |
| Architecture Decision Records | [docs/architecture/ADR.md](docs/architecture/ADR.md) |
| Deployment Guide | [docs/deployment/DEPLOY.md](docs/deployment/DEPLOY.md) |
| Agent guidance | [AGENTS.md](AGENTS.md) |
| End-user guide (installed into projects) | [assets/docs/workflow-help.md](assets/docs/workflow-help.md) → `.workflow-hero/docs/workflow-help.md` |

---

## Contributing

Contributions are welcome.

1. Open an issue for bugs, UX feedback, or feature requests.
2. Fork the repository and create a branch from `main`.
3. Keep changes focused; follow the Feature Based + Vertical Slice layout under `internal/<feature>/`.
4. Run:

```bash
go test ./...
```

5. Open a Pull Request with a clear description of **why** the change is needed.

Please do not introduce V1 out-of-scope items (Windows binaries, CI/CD release automation, non-Cursor adapters, or CLI commands that perform LLM reasoning). See [PRD §2.3](docs/product/PRD.md).

---

## License

BSD 2-Clause. See [LICENSE](LICENSE).

---

<a id="portugues-br"></a>

# 🇧🇷 Português (BR)

**AI Workflow Hero (Hero)** é um framework open-source para desenvolvimento de software aumentado por IA. Ele **não** substitui o agente de código — ele coordena subagentes especializados, organiza artefatos do projeto, comprime contexto e torna ciclos de desenvolvimento com IA mais reproduzíveis e menos dependentes de um único provedor de LLM.

O Hero é distribuído como um binário CLI em Go (`hero`) que prepara o projeto com comandos, skills, prompts e templates para o **Cursor** (V1). Depois de instalado, o fluxo que exige raciocínio roda inteiramente no chat da IDE via comandos `/hero:*` — nunca dentro do binário CLI.

Projetado para um fluxo open source, ele ajuda você a avançar em:

**Research → Planning → Implementation → QA → Judge → Browser UI Validation → QA End-to-End**

---

## Recursos

- CLI determinística para install, upgrade, uninstall, doctor, status, variables e atualização de preços de modelos
- Assets de Runtime no Cursor: 13 comandos `/hero:*` e 11 agentes especializados
- Fluxo de stages configurável com aprovação humana, limites de iteração, timeouts e escalonamento
- Roteamento de scope para `backend_agent`, `frontend_agent` ou `generic_agent`
- Fallback de modelo em 3 níveis (`modelo do agente` → `fallback_model` → espera `/hero:continue`)
- Arquivos de compressão de contexto (`AGENTS.md`, `context/current-state.md`, `context/context-log.md`)
- Integração com OpenSpec para planejamento SDD e implementação orientada a tarefas
- Research gera especificações do projeto (PRD, ADR, UI, DEPLOY, TESTING) com numeração por ciclo; mudanças de arquitetura exigem ADR aprovado
- Padrão de logging na implementação (`error` / `info` / `debug`, default `info`) verificado pelo `qa_agent`
- Higiene suave de secrets: `.env.example` + padrões no `.gitignore`; `hero doctor` avisa, não bloqueia
- Browser UI Validation opcional (`stages.browser_ui_validation`, exige `scope.frontend`): Health com Playwright + Visual opcional vs PNGs em `docs/ui/visual_reference`
- Playwright opcional no QA End-to-End para jornadas (`stages.qa_end_to_end.use_playwright`, exige `scope.frontend`) — distinto do Browser UI Validation
- Fan-out paralelo via Task para implementação independente; sessões limpas de subagentes com ponteiros de arquivo
- Métricas por ciclo e estimativas de custo do projeto a partir de arquivos estruturados de pricing
- Segurança no upgrade: arquivos customizados do Hero nunca são sobrescritos em silêncio (checksum)
- Uninstall remove apenas caminhos do Hero; o conhecimento do projeto (`AGENTS.md`, `docs/`, `context/`, `openspec/`) é preservado
- Guia do usuário instalado em `.workflow-hero/docs/workflow-help.md` (caminho exibido após `hero install`)

---

## Instalação

Os binários prontos para uso serão publicados na seção de [Releases](https://github.com/ricrsantos/ai_workflow_hero/releases) do repositório.

### A partir do binário de release (recomendado)

1. Baixe o artefato correspondente ao seu SO/arquitetura, por exemplo:
   - `hero_v1.0.0_linux_amd64`
   - `hero_v1.0.0_linux_arm64`
   - `hero_v1.0.0_darwin_amd64`
   - `hero_v1.0.0_darwin_arm64`
2. (Opcional) Verifique o checksum SHA256 em `checksums.txt`.
3. Torne-o executável e coloque-o no `PATH`:

```bash
chmod +x hero_v1.0.0_linux_amd64
sudo mv hero_v1.0.0_linux_amd64 /usr/local/bin/hero
hero version
```

### A partir do código-fonte

```bash
git clone https://github.com/ricrsantos/ai_workflow_hero.git
cd ai_workflow_hero
go build -ldflags "-X main.version=$(git describe --tags --abbrev=0 2>/dev/null || echo dev)" -o hero ./cmd/hero
sudo mv hero /usr/local/bin/hero
```

---

## Início rápido

Dentro do projeto-alvo (precisa ser um repositório git, ou use `--git-init`):

```bash
# Interativo
hero install --tools cursor

# Totalmente scriptado (amigável a CI)
hero install --tools cursor --name "Meu Projeto" --summary "Resumo curto do projeto" --yes --git-init
```

Em seguida, no chat do Cursor:

1. `/hero:sync` — ativa o Hero em um codebase existente (opcional, mas recomendado)
2. `/hero:init` — inicia um novo ciclo de desenvolvimento
3. Edite `.workflow-hero/cycles/current/workflow-config.yml`
4. Abra um **chat novo e vazio**, selecione o agente que deseja como **orchestrator / grill-me** do Hero, e então `/hero:start`

Após a instalação, leia o guia completo em `.workflow-hero/docs/workflow-help.md` (filosofia, configuração, comandos CLI e Runtime).

Comandos CLI úteis após a instalação:

```bash
hero doctor
hero status
hero status --json
hero variables
hero upgrade
hero update-models
hero uninstall
```

---

## Comandos de Runtime (chat do Cursor)

| Comando | Finalidade |
|---|---|
| `/hero:init` | Inicia um novo ciclo de desenvolvimento (depois prefira um chat novo para o start) |
| `/hero:start` | Executa os stages configurados (prefira chat limpo; selecione orchestrator / grill-me antes) |
| `/hero:approve` | Aprova o stage atual |
| `/hero:reject` | Rejeita e pede alterações |
| `/hero:cancel` | Cancela o stage e restaura o checkpoint git |
| `/hero:continue` | Concede iterações extras após escalonamento |
| `/hero:back` | Reabre o Planning após ambiguidade no SDD |
| `/hero:finish` | Encerra o ciclo; grava a data Completed usada no nome do arquivo |
| `/hero:archive` | Arquiva o ciclo atual (data da pasta = Completed em workflow.md) |
| `/hero:resume` | Retoma um ciclo arquivado |
| `/hero:sync` | Ativa / re-sincroniza o Hero em um projeto existente |
| `/hero:status` | Mostra o status do ciclo no chat |
| `/hero:help` | Lista os comandos de Runtime |

---

## Requisitos

### Usuários finais

- Linux ou macOS (`amd64` ou `arm64`)
- IDE [Cursor](https://cursor.com) (V1)
- Git (obrigatório; `hero install` pode executar `git init` com consentimento)
- [OpenSpec](https://github.com/Fission-AI/OpenSpec) para o fluxo de Planning (instalado com consentimento quando necessário)

### Desenvolvimento

- Go 1.25+ (veja `go.mod`)
- Git

```bash
git clone https://github.com/ricrsantos/ai_workflow_hero.git
cd ai_workflow_hero
go test ./...
go build -o hero ./cmd/hero
```

---

## Build e testes

```bash
go test ./...
go build -ldflags "-X main.version=$(git describe --tags --abbrev=0)" -o hero ./cmd/hero
```

Artefatos de release cross-compilados (4 plataformas + `checksums.txt`):

```bash
./scripts/release.sh
# → dist/hero_<version>_linux_amd64
# → dist/hero_<version>_linux_arm64
# → dist/hero_<version>_darwin_amd64
# → dist/hero_<version>_darwin_arm64
# → dist/checksums.txt
```

---

## Estrutura do projeto

```text
.
├── cmd/hero/                 # Entrypoint da CLI (Cobra)
├── internal/
│   ├── install/              # hero install
│   ├── upgrade/              # hero upgrade
│   ├── uninstall/            # hero uninstall
│   ├── doctor/               # hero doctor
│   ├── status/               # hero status
│   ├── variables/            # hero variables
│   ├── update_models/        # hero update-models
│   ├── adapters/cursor/      # layout de paths do Cursor
│   ├── common/               # erros, output, templates
│   └── integration/          # testes e2e leves
├── assets/                   # embed.FS (commands, agents, skills, templates, models)
├── scripts/release.sh        # release manual cross-compilado
├── docs/                     # PRD, UI, ADR, DEPLOY
├── context/                  # memória comprimida deste repositório
├── openspec/                 # SDD / planejamento de mudanças do próprio Hero
├── AGENTS.md
├── go.mod
└── README.md
```

---

## Documentação

| Documento | Caminho |
|---|---|
| Requisitos de produto | [docs/product/PRD.md](docs/product/PRD.md) |
| Spec de UX do terminal | [docs/product/UI.md](docs/product/UI.md) |
| Architecture Decision Records | [docs/architecture/ADR.md](docs/architecture/ADR.md) |
| Guia de deploy | [docs/deployment/DEPLOY.md](docs/deployment/DEPLOY.md) |
| Orientação para agentes | [AGENTS.md](AGENTS.md) |
| Guia do usuário final (instalado nos projetos) | [assets/docs/workflow-help.md](assets/docs/workflow-help.md) → `.workflow-hero/docs/workflow-help.md` |

---

## Como contribuir

Contribuições são bem-vindas.

1. Abra uma issue para bugs, feedback de UX ou pedidos de funcionalidade.
2. Faça fork do repositório e crie uma branch a partir de `main`.
3. Mantenha as mudanças focadas; siga o layout Feature Based + Vertical Slice em `internal/<feature>/`.
4. Execute:

```bash
go test ./...
```

5. Abra um Pull Request com uma descrição clara do **porquê** da mudança.

Por favor, não introduza itens fora do escopo de V1 (binários Windows, automação CI/CD de release, adapters além do Cursor, ou comandos CLI que façam raciocínio com LLM). Veja [PRD §2.3](docs/product/PRD.md).

---

## Licença

BSD 2-Clause. Veja [LICENSE](LICENSE).
