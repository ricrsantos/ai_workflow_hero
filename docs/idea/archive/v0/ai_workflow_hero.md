# AI Workflow Hero

O AI Workflow hero (**Hero**), é um framework para desenvolvimento de software aumentado por IA.

O Hero não substitui o agente, ele coordena múltiplos agentes, organiza o trabalho, reduz o contexto e torna o processo reproduzível. O Hero objetiva reduzir a dependência do projeto com  modelos específicos de LLM, dando maior liberdade ao desenvolvedor para escolher o modelo que caiba em suas possibilidades.

A cada novo **Ciclo de Desenvolvimento**, seja ele a criação do projeto, a implementação de uma nova feature, correção de um bug, melhorias ou qualquer outra atividade no projeto, o **Hero**  abstrai para o desenvolvedor, a organização do projeto, construção da documentação, memória do projeto, orquestração de agentes, elaboração dos prompts, validação e integração com outros frameworks. Para atingir este objetivo, o Hero propõe uma estrutura flexível onde cada etapa de desenvolvimento pode ser ligada ou desligada de acordo com a necessidade do desenvolvedor. Por exemplo, se o desenvolvedor estiver iniciando um Ciclo de Desenvolvimento para correção de bug, é muito provável que a etapa de Research não seja necessária, por outro lado, caso o desenvolvedor esteja iniciando um Ciclo para implementar uma nova feature, a etapa de Research costuma ser muito útil.         

Seu projeto não fica refém do Hero, os arquivos são organizados de forma que os artefatos do projeto gerados pelo Hero, ficam no domínio do projeto, enquanto os artefatos específicos do Hero ficam em uma pasta separada. Desta forma o desenvolvedor contínua com o controle total do seu projeto, optando por seguir ou não utilizando o Hero, sem prejuízo nenhum o projeto.

---
## Itens que ainda preciso colocar:

> ✅ Todos os itens abaixo foram resolvidos na sessão de grilling de 2026-07-20. Ver [Decisões da Sessão de Grilling](#decisões-da-sessão-de-grilling--2026-07-20) para o detalhamento completo de cada decisão.

- ~~Citar que o Hero orquestra frameworks de research e SDD~~ → já refletido em "Ferramentas que integram o Framework Hero" e nas decisões sobre OpenSpec (opsx-propose, opsx-sync, opsx-explore).
- ~~Corrigir a nomenclatura das pastas / Revisar todas~~ → ver decisões #Nomenclatura.
- ~~Exemplos de todos os arquivos e templates~~ → ver decisões #Templates.
	- ~~Falta concluir o template do arquivo de configuração do openspec~~ → resolvido (config.yaml gerado a partir do documents.json).
	- ~~Falta definir onde serão preenchidos os arquivos com os documentos de research e como separar as categorias~~ → resolvido (templates fixos por categoria + numeração por ciclo).
- ~~Adicionar no flow que se não tiver marcada a opção de aprovação pelo usuário, aprovar automaticamente~~ → resolvido (auto-aprovação interrompível, ver decisões #Aprovação).

---
## Termos e Definições

- Bootstrap: É o processo de inicialização do Hero em um projeto, no bootstrap o Hero cria os seus artefatos de execução dentro do projeto do usuário;

- Cilo de Desenvolvimento (ou apenas **Ciclo**): Corresponde ao inicio de uma nova atividade no projeto, uma nova interação como por exemplo, criar o projeto, adicionar uma nova feature, corrigir um BUG, etc;

- Etapa de desenvolvimento (Ou apenas **Etapa**): Trata-se de cada um dos passos que podem ser realizados durante um ciclo de desenvolvimento, por exemplo, research, plan, implementation, test, validation, etc;

- Research: É uma das Etapas do Ciclo de desenvolvimento oferecidas pelo Hero, trate de uma etapa onde são exploradas as alternativas para implementar as funcionalidades desejadas no software; 

- Plan: É a Etapa de Desenvolvimento onde as especificações de produto e decisões arquiteturais do software são convertidas em um plano de implementação, contendo as tasks e a ordem em que elas podem ser executadas para permitir a testabilidade do projeto;

- Implementation: Nesta Etapa de Desenvolvimento ocorre efetivamente a implementação do código conforme o planejamento ou conforme orientado pelo desenvolvedor;

- Validation: Em geral é a última Etapa de um Ciclo de Desenvolvimento, aqui são realizados testes e interações com o desenvolvedor, buscando atingir a qualidade desejada e garantir o atendimento dos requisitos do software.

---
## Objetivos

### Globais

- Obter resultados o mais determinísticos possíveis;
- Reduzir dependência de modelos específicos;
- Reduzir custos com tokens;
- Permitir que o usuário escolha quais etapas devem ser executadas a cada ciclo de desenvolvimento;
- Implementar orquestração avançada (multi-agente + loops);
- Utilizar sub-agents especializados para cada tipo de atividade;
- Possuir estáticas de consumo de token para cada etapa do projeto.
- Manter histórico e contexto das alterações;
- Implementar compressão contínua de contexto:
	- arquitetura atual;
	- decisões recentes;
	- fluxo real;
	- exceções;
	- hacks temporários que viraram permanentes.
### V1

- Primeira versão focada apenas no Cursor AI;
- Cobrir as etapas de:
	- Research;
	- Plan;
	- Implementation;
	- Validation.
- `/hero:sync` cobre a ativação básica do Hero em projetos já existentes (análise de código via `context_agent` + geração de `AGENTS.md`/`current-state.md`) — promovido de V2 para V1 na sessão de grilling de 2026-07-20, por ser pré-requisito natural de adoção em projetos reais.

### V2

- Adicionar compatibilidade com os seguintes ambientes de desenvolvimento agêntico:
	- Open Code
	- Claude Code
	- Claude APP
	- Codex CLI
	- Codex APP
	- Vs Code
- Permitir opcionalmente as etapas de 
	- UX;
	- Observabilidade;
	- especificações de deploy;
	- Avaliação de segurança do código;
	- Avaliação na arquitetura do código;
- Adicionar workflow para sincronização **avançada** com projetos existentes (detecção de divergências entre código e documentação, sync incremental contínuo — além da ativação básica já coberta na V1 pelo `/hero:sync`);
- Adicionar workflow para melhorias na arquitetura de projetos existentes;
- Adicionar AI hooks para controlar o workflow de forma mais precisa;
- Adicionar código determinístico entre as etapas para economizar token e tornar o processo mais padronizado;
- Adicionar fontes de memória mais sofisticadas para o projeto (ex. Banco de dados, RAG, etc).
- Avaliar a possibilidade de sempre operar via CLI.


---
## Premissas

- Organização de pastas Feature Based;

- Arquitetura de Vertical Slice ou dependendo da complexidade Vertical Slice + Architecture and Domain Modules ([[arquitetura_software#Vertical Slice Architecture and Domain Modules]]); 

- Documentação de instruções para os agentes em inglês;

- A estatística de uso de tempo gasto, uso de tokens e custos por seção será realizada através de estimativas feitas pelo próprio agente;

- O AI Workflow Hero distingue entre artefatos de projeto e artefatos de execução. 
	- Os artefatos de projeto representam o conhecimento permanente e continuam úteis independente do AI Workflow Hero, por exemplo, `AGENTS.md`, `docs/`, `context/`, `openspec/`;
	- Os artefatos do Workflow Hero são armazenados em `.workflow-hero/` e guardam apenas a configuração, métricas e histórico das interações com este framework;

---
## Ferramentas que integram o Framework Hero

- Research com a Skill de grilling própria do Hero (`.cursor/skills/grilling/`, distribuída junto com o CLI);
- SDD com o framework OpenSpec (`opsx-propose`, `opsx-explore`, `opsx-apply`, `opsx-sync`, `opsx:archive`);
- Pesquisa com o MCP context7;
- Teste de front end e ponta-a-ponta com o framework Playwright (etapa QA End-to-End, quando `stages.qa_end_to_end.use_playwright=true` e `scope.frontend=true`);
- Versionamento com Git (pré-requisito obrigatório: usado pelo `/hero:cancel` para checkpoints/rollback).

---
## Fluxo funcional

### Etapas:

Configuration
   ↓
Research
   ↓
Planning
   ↓
Implementation
   ↓
  QA
   ↓
Judge
   ↓
QA End-to-End


### Fluxo dos Agents

O Fluxo dos agentes depende das configurações selecionadas pelo usuário. A chamada do Context Agent é opcional e depende da necessidade de buscar contexto para a execução da tarefa.

Orchestration Agent
        |
        |
        +-------> Context Agent

   Planning Agent
        |
        |
        +-------> Context Agent

Backend Agent
        |
        +-------> Context Agent

Frontend Agent
        |
        +-------> Context Agent

QA Agent
        |
        +-------> Context Agent

Judge Agent
        |
        +-------> Context Agent

End2End QA Agent
        |
        +-------> Context Agent
        
---
## Agents

>orchestration_agent:
 Agente que vai fazer a orquestração do loop. No caso do cursor, este agente é selecionado pelo usuário quando ele inicia o chat com a IA.

>discover_agent: 
 Agente responsável por rodar o ciclo do gril-me. No caso do Cursor na V1 do Hero, este agente será o mesmo que o **orchestration_agent** devido as peguntas e respostas dinâmicas no chat.

>planning_agent:
 Agente que vai ser utilizado com o open spec. Este agente deve ser configurado em:  `.workflow-hero/config/workflow-config.yml`

>context_agent
> Agente responsável por buscar o contexto na documentação de projeto e no código. Este agente deve ser configurado em:  `.workflow-hero/config/workflow-config.yml`

> backend_agent:
> Agente que vai realizar a implementação de código no backend. Este agente deve ser configurado em:  `.workflow-hero/config/workflow-config.yml`

>frontend_agent:
> Agente que vai fazer a implementação do frontend. Este agente deve ser configurado em:  `.workflow-hero/config/workflow-config.yml`

>generic_agent: *(adicionado na sessão de grilling de 2026-07-20)*
> Agente responsável pela implementação de tarefas que não se encaixam em backend/frontend: aplicações nativas (Linux/Windows), scripts e infraestrutura (`scope.native`, `scope.script`, `scope.infrastructure`). Modelo inicial recomendado: Claude Sonnet 5. Este agente deve ser configurado em: `.workflow-hero/config/workflow-config.yml`

>qa_agent:
 Agente responsável por executar os testes no que foi implementado e avaliar itens de qualidade como cobertura de teste, arquitetura utilizada, etc. Adapta os critérios de validação conforme o scope (ex: `terraform plan`/`docker build` para infrastructure, execução sem erro para script, build da plataforma alvo para native). Em caso de erro, devolve o erro detalhadamente para o Loop. Se tudo estiver ok, devolve OK. Este agente deve ser configurado em:  `.workflow-hero/config/workflow-config.yml`

>judge_agent:
 Agente responsável por avaliar se realmente foi implementado tudo que foi planejado, exclusivamente sob a ótica de cobertura da SDD (não avalia qualidade/arquitetura/estilo — isso é responsabilidade do qa_agent). Se sim ele devolve OK, se não ele devolve para o loop o que falta ser implementado. Em caso de ambiguidade na própria SDD (e não lacuna de implementação), escala para o usuário decidir entre `/hero:back` (reabrir Planning) ou `/hero:approve` (aceitar e seguir). Este agente deve ser configurado em:  `.workflow-hero/config/workflow-config.yml`

>end2end_qa_agent:
Agente responsável por fazer o teste end to end da aplicação, em caso de erro ele deve devolver o erro detalhado para o loop para que ele seja corrigido. Se tudo estiver funcionando de acordo com o esperado, devolve OK para o loop. Este agente deve ser configurado em:  `.workflow-hero/config/workflow-config.yml`

---
## Recomendação de Modelos:

### Recomendação por custo x desempenho

| Agente                                       | 🥇 Melhor escolha | 🥈 2ª             | 🥉 3ª           | 4ª                | 5ª                 |
| -------------------------------------------- | ----------------- | ----------------- | --------------- | ----------------- | ------------------ |
| **orchestration_agent** _(modelo da sessão)_ | GPT-5.4           | Claude Sonnet 5   | GPT-5.6 Terra   | Claude 4.6 Sonnet | GPT-5.5            |
| **discover_agent**                           | Claude Sonnet 5   | Claude 4.6 Sonnet | GPT-5.6 Sol     | Grok 4.5          | Claude 4 Sonnet 1M |
| **planning_agent**                           | GPT-5.3 Codex     | GPT-5.2 Codex     | Claude Sonnet 5 | GPT-5.6 Sol       | GPT-5.6 Terra      |
| **context_agent**                            | Composer 2.5      | GLM 5.2           | Claude Sonnet 5 | Claude 4.6 Sonnet | GPT-5.6 Sol        |
| **backend_agent**                            | GPT-5.3 Codex     | Grok 4.5          | GPT-5.2 Codex   | GPT-5.6 Sol       | Claude 4.6 Opus    |
| **frontend_agent**                           | GPT-5.3 Codex     | Claude 4.6 Sonnet | Grok 4.5        | GPT-5.2 Codex     | GPT-5.6 Sol        |
| **generic_agent** _(native/script/infra)_    | Claude Sonnet 5   | Claude 4.6 Sonnet | GPT-5.3 Codex   | Grok 4.5          | GPT-5.6 Sol        |
| **qa_agent**                                 | GPT-5.3 Codex     | Claude 4.6 Sonnet | GPT-5.2 Codex   | GPT-5.6 Sol       | Grok 4.5           |
| **judge_agent**                              | Claude Sonnet 5   | Claude 4.6 Sonnet | GPT-5.6 Sol     | Claude Opus 4.8   | GPT-5.5            |
| **end2end_qa_agent**                         | Claude 4.6 Sonnet | Claude Sonnet 5   | GPT-5.6 Sol     | GPT-5.3 Codex     | Grok 4.5           |

### Recomendação Custo Beneficio em 15 de julho de 2026
| Agente                                       | Modelo            |
| -------------------------------------------- | ----------------- |
| **orchestration_agent** _(modelo da sessão)_ | Claude Sonnet 5   |
| **discover_agent**                           | Claude Sonnet 5   |
| **planning_agent**                           | GPT-5.3 Codex     |
| **context_agent**                            | Composer 2.5      |
| **backend_agent**                            | Grok 4.5          |
| **frontend_agent**                           | Claude 4.6 Sonnet |
| **generic_agent**                            | Claude Sonnet 5   |
| **qa_agent**                                 | GPT-5.3 Codex     |
| **judge_agent**                              | Claude Sonnet 5   |
| **end2end_qa_agent**                         | Claude 4.6 Sonnet |

---
## Tabela de preços utilizada como referência

Retirada de https://cursor.com/pt-BR/docs/models-and-pricing

| Nome                        | Entrada | Gravação no cache | Leitura no cache | Saída |
| --------------------------- | ------: | ----------------: | ---------------: | ----: |
| GPT-5.4 Nano                |    $0.2 |                 - |            $0.02 | $1.25 |
| GPT-5 Mini                  |   $0.25 |                 - |           $0.025 |    $2 |
| GPT-5.1 Codex Mini          |   $0.25 |                 - |           $0.025 |    $2 |
| Composer 2.5                |    $0.5 |                 - |             $0.2 |  $2.5 |
| Gemini 2.5 Flash            |    $0.3 |                 - |            $0.03 |  $2.5 |
| Gemini 3 Flash              |    $0.5 |                 - |            $0.05 |    $3 |
| Kimi K2.7 Code              |   $0.95 |                 - |            $0.19 |    $4 |
| GLM 5.2                     |    $1.4 |                 - |            $0.26 |  $4.4 |
| GPT-5.4 Mini                |   $0.75 |                 - |           $0.075 |  $4.5 |
| Claude 4.5 Haiku            |      $1 |             $1.25 |             $0.1 |    $5 |
| GPT-5.6 Luna                |      $1 |             $1.25 |             $0.1 |    $6 |
| Grok 4.5                    |      $2 |                 - |             $0.5 |    $6 |
| Gemini 3.5 Flash            |    $1.5 |                 - |            $0.15 |    $9 |
| Composer 1                  |   $1.25 |                 - |           $0.125 |   $10 |
| GPT-5                       |   $1.25 |                 - |           $0.125 |   $10 |
| GPT-5-Codex                 |   $1.25 |                 - |           $0.125 |   $10 |
| GPT-5.1 Codex               |   $1.25 |                 - |           $0.125 |   $10 |
| GPT-5.1 Codex Max           |   $1.25 |                 - |           $0.125 |   $10 |
| Gemini 3 Pro                |      $2 |                 - |             $0.2 |   $12 |
| Gemini 3 Pro Image Preview  |      $2 |                 - |             $0.2 |   $12 |
| Gemini 3.1 Pro              |      $2 |                 - |             $0.2 |   $12 |
| GPT-5.2                     |   $1.75 |                 - |           $0.175 |   $14 |
| GPT-5.2 Codex               |   $1.75 |                 - |           $0.175 |   $14 |
| GPT-5.3 Codex               |   $1.75 |                 - |           $0.175 |   $14 |
| Composer 2.5 (Fast)         |      $3 |                 - |             $0.5 |   $15 |
| Claude 4 Sonnet             |      $3 |             $3.75 |             $0.3 |   $15 |
| Claude 4.5 Sonnet           |      $3 |             $3.75 |             $0.3 |   $15 |
| Claude 4.6 Sonnet           |      $3 |             $3.75 |             $0.3 |   $15 |
| Claude Sonnet 5             |      $3 |             $3.75 |             $0.3 |   $15 |
| GPT-5.4                     |    $2.5 |                 - |            $0.25 |   $15 |
| GPT-5.6 Terra               |    $2.5 |            $3.125 |            $0.25 |   $15 |
| GPT-5 Fast                  |    $2.5 |                 - |            $0.25 |   $20 |
| Claude 4 Sonnet 1M          |      $6 |              $7.5 |             $0.6 | $22.5 |
| Claude 4.5 Opus             |      $5 |             $6.25 |             $0.5 |   $25 |
| Claude 4.6 Opus             |      $5 |             $6.25 |             $0.5 |   $25 |
| Claude 4.7 Opus             |      $5 |             $6.25 |             $0.5 |   $25 |
| Claude Opus 4.8             |      $5 |             $6.25 |             $0.5 |   $25 |
| GPT-5.5                     |      $5 |                 - |             $0.5 |   $30 |
| GPT-5.6 Sol                 |      $5 |             $6.25 |             $0.5 |   $30 |
| Claude Fable 5              |     $10 |             $12.5 |               $1 |   $50 |
| Claude Opus 4.7 (fast mode) |     $30 |             $37.5 |               $3 |  $150 |

---
## Estratégia de Desenvolvimento e Distribuição

### Visão Geral

O Workflow Hero será distribuído através de uma CLI construída em Go Lang. O binário da CLI vai utilizar o recurso Embed.FS do Go lang, para portar todos os assets necessários para a instalação do Hero no harness utilizado pelo desenvolvedor. Na primeira versão do Hero será implementado suporte apenas ao Cursor AI Ide.

O Hero também deve utilizar a biblioteca Cobra do Go para estruturar o CLI. 

### Arquitetura

A arquitetura prevista para o desenvolvimento do Hero é a Feature Based + Vertical Slice.

> **Princípio de Arquitetura**
>
> - **CLI:** responsável apenas pela instalação, atualização, manutenção e administração do Hero.
> - **Runtime:** responsável pela execução do ciclo de desenvolvimento utilizando IA.
> - Apenas comandos administrativos podem possuir equivalentes no CLI e no Runtime (como `sync`). Comandos que envolvem raciocínio dos agentes existem exclusivamente no Runtime.

### Estratégia de variáveis e templates.

A estratégia de preenchimento de variáveis e templates do Hero deve seguir as seguintes definições:

- As variáveis devem ser adicionadas em arquivos no formato `Json`, onde cada arquivo representa um domínio do Hero, por exemplo, `hero.json, project.json, docs.json`, etc.

- Os arquivos que possuem as variáveis do projeto devem ser armazenados em:
```
Projeto
│
└── .workflow-hero/
    └── config/
	    ├─ hero.json
		├─ documents.json
		├─ ...
		└─ project.json    
```

- Os arquivos markdown devem implementar um place holder composto por `{{nomedoarquivo.objeto}}`, exemplos:
	- `{{documents.prd.path}}`
	- `{{project.name}`
	- `{{project.name}`
	- `{{hero.cli.version}`
	- `{{hero.assets.version}`

- O agente é responsável por substituir o placeholder pelo valor extraído a partir dos arquivos `json`

### Repositório

O Hero é um projeto de código aberto sob a licença BSD-2, um repositório público no GitHub será o local onde o Hero estará disponível para a comunidade. Os binários do Hero estarão disponíveis na seção de releases do repositório.

A estrutura do repositório do Hero deve ser algo semelhante ao exemplo abaixo:

```text
workflow-hero/
│
├── cmd/
│   └── hero/
│       └── main.go
│
├── internal/
│   ├── install/
│   │   ├── command.go
│   │   ├── service.go
│   │   └── validator.go
│   │
│   ├── upgrade/
│   │   ├── command.go
│   │   └── service.go
│   │
│   ├── uninstall/
│   │   ├── command.go
│   │   └── service.go
│   │
│   ├── doctor/
│   │   ├── command.go
│   │   └── service.go
│   │
│   ├── adapters/
│   │   └── cursor/
│   │       ├── installer.go
│   │       ├── updater.go
│   │       ├── validator.go
│   │       └── paths.go
│   │
│   └── common/
│       ├── filesystem.go
│       ├── embed.go
│       ├── version.go
│       └── logger.go
│
├── assets/
│   └── cursor/
│       │
│       ├── commands/
│       │   ├── hero-new.md
│       │   ├── hero-start.md
│       │   ├── hero-approve.md
│       │   ├── hero-reject.md
│       │   ├── hero-cancel.md
│       │   ├── hero-continue.md
│       │   ├── hero-back.md
│       │   ├── hero-finish.md
│       │   ├── hero-archive.md
│       │   ├── hero-resume.md
│       │   ├── hero-sync.md
│       │   ├── hero-status.md
│       │   └── hero-help.md
│       │
│       ├── agents/
│       │   ├── planning_agent.md
│       │   ├── context_agent.md
│       │   ├── backend_agent.md
│       │   ├── frontend_agent.md
│       │   ├── generic_agent.md
│       │   ├── qa_agent.md
│       │   ├── judge_agent.md
│       │   └── end2end_qa_agent.md
│       │
│       ├── skills/
│       │   ├── workflow-hero/
│       │   │   └── SKILL.md
│       │   └── grilling/
│       │       └── SKILL.md
│       │
│       └── workflow-hero/
│           ├── config/
│           │   ├── models.md
│           │   └── variables.md
│           │
│           ├── models/
│           │   ├── openai.yml
│           │   ├── anthropic.yml
│           │   ├── google.yml
│           │   ├── cursor.yml
│           │   ├── moonshot.yml
│           │   ├── zhipu.yml
│           │   └── xai.yml
│           │
│           ├── prompts/
│           │   ├── init.md
│           │   ├── start.md
│           │   ├── research.md
│           │   ├── planning.md
│           │   ├── implementation.md
│           │   ├── qa.md
│           │   ├── judge.md
│           │   ├── archive.md
│           │   └── help.md
│           │
│           └── templates/
│               ├── AGENTS.md
│               ├── context/
│               │   ├── current-state.md
│               │   └── context-log.md
│               │
│               ├── docs/
│               │   ├── PRD.md
│               │   ├── ADR.md
│               │   ├── UI.md
│               │   ├── DEPLOY.md
│               │   └── TESTING.md
│               │
│               └── cycles/
│                   ├── workflow.md
│                   ├── workflow-config.yml
│                   └── metrics.md
│
├── scripts/
│
├── docs/
│
├── go.mod
├── go.sum
├── README.md
├── LICENSE
└── .gitignore
```

### Comandos

Inicialmente o CLI do Hero deve implementar os seguintes comandos:

> **Nota (grilling 2026-07-20):** `hero sync` foi removido do CLI (a sincronização exige raciocínio de agente e existe apenas como `/hero:sync` no Runtime). `hero status` e `hero help` ganharam equivalentes CLI somente-leitura. Foram adicionados `/hero:continue`, `/hero:back` e `/hero:resume` ao Runtime. Ver [Decisões da Sessão de Grilling](#decisões-da-sessão-de-grilling--2026-07-20) para detalhes.

| Categoria | Comando                       | Equivalente     | Descrição                                                                                                                               |
| --------- | ----------------------------- | --------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| CLI       | `hero install --tools cursor` | —               | Instala o Hero no projeto, copiando comandos, skills, prompts, templates e estrutura inicial do `.workflow-hero`. Verifica se o projeto é um repositório git; se não for, oferece rodar `git init`. |
| CLI       | `hero upgrade`                | —               | Atualiza a instalação do Hero preservando os ciclos e configurações do projeto. Não sobrescreve arquivos customizados pelo usuário (detecção por checksum); apenas avisa. |
| CLI       | `hero uninstall`              | —               | Remove o Hero do projeto (`.cursor/agents/`, `.cursor/commands/hero-*.md`, `.cursor/skills/workflow-hero/`, `.cursor/skills/grilling/`, `.workflow-hero/`), preservando os artefatos que pertencem ao projeto (`AGENTS.md`, `context/`, `docs/`, `openspec/`, etc.). |
| CLI       | `hero doctor`                 | —               | Verifica a instalação: presença de arquivos/pastas esperados, consistência de versão entre `hero.json` e o binário, sintaxe dos arquivos de config, e se o projeto é um repositório git. |
| CLI       | `hero version`                | —               | Exibe a versão instalada do Hero CLI e dos artefatos do projeto.                                                                        |
| CLI       | `hero variables`              | —               | Exibe (somente leitura) todas as variáveis que estão nos arquivos `json` em `.workflow-hero/config`.                                     |
| CLI       | `hero update-models`          | —               | Atualiza os arquivos `models/*.yml` a partir de um arquivo de dados estruturado publicado no repositório oficial do Hero (sem scraping). |
| CLI       | `hero status`                 | `/hero:status`  | Exibe (somente leitura, fora do chat) o estado atual do ciclo lendo `workflow.md`/`metrics.md`.                                          |
| CLI       | `hero help`                   | `/hero:help`    | Lista todos os comandos disponíveis e uma breve descrição de cada um.                                                                    |
| Runtime   | `/hero:new`                  | —               | Cria um novo ciclo de desenvolvimento. Na primeira execução também inicializa os artefatos do projeto (`AGENTS.md`, `context/`, etc.). Incrementa `project.json → workflow.cycle`. Se já houver um ciclo em andamento, avisa e pede confirmação antes de arquivar. |
| Runtime   | `/hero:start`                 | —               | Inicia a execução do ciclo conforme as etapas configuradas em `workflow-config.yml`. Valida dependências entre etapas antes de prosseguir. |
| Runtime   | `/hero:approve`               | —               | Aprova o resultado da etapa atual quando ela exigir aprovação manual.                                                                   |
| Runtime   | `/hero:reject`                | —               | Reprova a etapa atual e solicita sua reexecução.                                                                                        |
| Runtime   | `/hero:cancel`                | —               | Cancela definitivamente a etapa atual, restaura o checkpoint git anterior à etapa, e avança para a próxima etapa configurada.           |
| Runtime   | `/hero:finish`                | —               | Finaliza o ciclo de desenvolvimento, atualiza os artefatos necessários e encerra a execução.                                            |
| Runtime   | `/hero:archive`               | —               | Força o arquivamento do ciclo atual (mesmo em andamento) para `cycles/archive`, marcando a etapa em andamento como `Paused`.            |
| Runtime   | `/hero:resume [ciclo]`        | —               | Retoma um ciclo pausado/arquivado, movendo-o de volta para `cycles/current/` e continuando da etapa marcada como `Paused`.              |
| Runtime   | `/hero:sync`                  | —               | Sincroniza o projeto, recriando/atualizando `AGENTS.md` e os arquivos de contexto compactado. Também ativa o Hero em projetos existentes, escaneando a base de código via `context_agent`. |
| Runtime   | `/hero:status`                | `hero status`   | Exibe o estado atual do ciclo, etapa em execução, próximas etapas e métricas acumuladas.                                                |
| Runtime   | `/hero:help`                  | `hero help`     | Lista todos os comandos Runtime disponíveis e uma breve descrição de cada um.                                                           |
| Runtime   | `/hero:continue`               | —              | Concede mais N iterações (o usuário informa quantas) quando `max_iterations` ou `timeout_minutes` se esgotam; retoma de onde parou.      |
| Runtime   | `/hero:back`                   | —              | Retorna para a etapa de Planning quando o Judge identifica ambiguidade na SDD; reseta Implementation/QA/Judge para `Waiting`.            |

### Acompanhamento do progresso.

O acompanhamento do progresso da execução de cada ciclo do Hero ocorre através do arquivo `.workflow-hero/cycles/current/workflow.md`. Os campos de dados contidos neste arquivo podem conter os seguintes valores:

| Campo          | Valores possíveis                                                              | Descrição                                                                                                                   |
| -------------- | ------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| Status         | Waiting \| Disable \| In Progress \| Completed \| Cancelled \| Paused          | Indica o estado de uma determinada etapa do ciclo atual. `Paused`: etapa em andamento no momento de um `/hero:archive` manual. |
| Iteration      | current / max  \| current                                                       | Indica a interação atual e o máximo de iterações permitidas. Para a etapa de configuração indica apenas a iteração corrente. Iterações extras concedidas via `/hero:continue` aparecem em um campo adicional `Extra Iterations Granted`. |
| Human Approval | N/A \| Disable \| Pending \| Escalated \| Rejected \| Approved \| Cancelled     | Indica o status da aprovação humana em uma etapa. Na etapa de Configuração não é aplicável. `Escalated`: `max_iterations`/`timeout_minutes` esgotados, aguardando decisão do usuário (distinto de `Pending`, que é aprovação normal de resultado concluído). |

---
## Workflow

### Instalação:

A instalação do Hero deve ser simples, principalmente considerando o público alvo que são os desenvolvedores.

1. Baixar o binário correspondente a arquitetura do computador do usuário na seção de Releases do repositório no GitHub;

2. Colocar o binário em algum path do sistema operacional que permita a execução em qualquer pasta do sistema;

3. Acessar a página do projeto no qual o usuário pretende utilizar o Hero;

4. Executar o comando de inicialização do hero. Exemplo para o Cursor AI: 
```bash
hero install --tools cursor

🚀 Hero Project Setup

Project name: 
> Indoor Location
 
Project summary (Opcional): 
> Indoor positioning platform using BLE gateways.

✔ Hero installed successfully.

``` 

Após a instalação do Hero no projeto os seguintes arquivos devem adicionados a pasta do projeto: 
```
Projeto
│
├── .cursor/
|	├── agents/
|	|   ├── planning_agent.md
|	|   ├── context_agent.md
|	|   ├── backend_agent.md
|	|   ├── frontend_agent.md
|	|   ├── generic_agent.md
|	|   ├── qa_agent.md
|	|   ├── judge_agent.md
|	|   └── end2end_qa_agent.md
|   |
│   ├── commands/
│   │   ├── hero-new.md
│   │   ├── hero-start.md
│   │   ├── hero-approve.md
│   │   ├── hero-reject.md
│   │   ├── hero-cancel.md
│   │   ├── hero-continue.md
│   │   ├── hero-back.md
│   │   ├── hero-finish.md
│   │   ├── hero-archive.md
│   │   ├── hero-resume.md
│   │   ├── hero-sync.md
│   │   ├── hero-status.md
│   │   └── hero-help.md
│   │
│   └── skills/
│       ├── workflow-hero/
│       │   └── SKILL.md
│       └── grilling/
│           └── SKILL.md
│
└── .workflow-hero/
    ├── config/
	│   ├─ hero.json
	│	├─ documents.json
	│	└─ project.json
	│
    ├── docs/
	│   ├─ README_PT_BR.md
	│   └─ README.md
	│
	├── models/
	│   ├─ openai.yml
	│   ├─ anthropic.yml
	|	├─ google.yml
	|	├─ cursor.yml
	|	├─ moonshot.yml
	|	├─ zhipu.yml
	|	└─ xai.yml
	|
    ├── prompts/
    |
    ├── metrics-summary.md
    |
    └── templates/
	    ├─ AGENTS.md
	    ├─ current-state.md
	    ├─ context-log.md
	    ├─ workflow-config.yml
	    ├─ workflow.md
	    └─ docs/
	       ├─ PRD.md
	       ├─ ADR.md
	       ├─ UI.md
	       ├─ DEPLOY.md
	       └─ TESTING.md
	    
```

> `metrics-summary.md`: arquivo de métricas agregadas de todo o projeto (todos os ciclos), fora de `cycles/`. Atualizado pelo `orchestration_agent` ao final de cada ciclo.
>
> `cycles/current/.lock`: arquivo de lock (timestamp + PID de sessão) criado ao iniciar uma etapa, usado para detectar sessões concorrentes.

### Bootstrap do Hero para um novo Ciclo de Desenvolvimento:

1. O usuário inicializa um novo Ciclo no Hero através do comando `/hero:new`

>A partir daqui assume o **orchestration_agent** e inicia o processo de contagem dos elementos de estatística.

2. O **orchestration_agent** cria ou caso necessário atualiza, os arquivos de **Contexto Compactado** no peojeto:
```bash
.
└── context
    ├── context-log.md
    └── current-state.md
```

3. O **orchestration_agent** cria ou caso necessário atualiza, o arquivo `agents.md` no projeto.

4. O **orchestration_agent** cria os arquivos de controle de fluxo do projeto, seguindo os casos de uso:

- Caso os aquivos abaixo não existirem, criar os mesmos.
```
.workflow-hero/
└─ cycles/
	└─ current/
	   ├── workflow.md
	   ├── workflow-config.yml
	   └── metrics.md
```

- Caso os arquivos acima referidos já existirem, arquivar os mesmos movendo para a pasta `archive`, seguindo este padrão para nomenclatura das pastas: **C[XX]-yyyy-mm-dd-[slug]/** (número do ciclo, gerado a partir de `project.json → workflow.cycle`, seguido da data e de um slug derivado do campo `title` do ciclo). Exemplo:
```
.workflow-hero/
└─ cycles/
	├─ current/
	|   ├── workflow.md
	|   ├── workflow-config.yml
	|   ├── metrics.md
	|   └── .lock
	|
	└── archive/
		 ├── C04-2026-06-21-upload-imgurl/
		 │   ├── workflow.md
		 │   ├── workflow-config.yml
		 |   └── metrics.md
		 |	 
		 |
		 └── ...
```

 >[!question] Importar o arquivo de configuração do último ciclo executado? ✅ Resolvido (grilling 2026-07-20; esclarecido 2026-07-29)
>Caso o usuário já tenha executado algum ciclo do Hero neste projeto, o `/hero:new` **sempre importa** as seções `workflow_config`, `fallback_model`, `stages` e `agents` (idioma de chat, modelos, iterações, aprovação e opções aninhadas de stage) do ciclo anterior para `.workflow-hero/cycles/current/workflow-config.yml` — sem perguntar. Os campos `title`, `objective` e `scope` sempre voltam para os valores padrão do template, pois são específicos de cada ciclo. O arquivo novo parte do template instalado (para preservar chaves novas de upgrade) e faz deep-merge dessas seções.
>
>Se já existir um ciclo em `cycles/current/` ainda em andamento (Status diferente de `Completed`/`Cancelled`/`Finished by User`), o `orchestration_agent` avisa o usuário, mostra a etapa atual, e pergunta explicitamente se ele quer arquivar mesmo assim (perdendo o progresso não finalizado) ou cancelar o `/hero:new` e continuar o ciclo atual com `/hero:start`.

O resultado final dos arquivos e pastas criadas deve ser algo semelhante ao exemplo abaixo:
```
.
├── AGENTS.md
|
├── context
|    ├── context-log.md
|    └── current-state.md
|
└──.workflow-hero/
    ├── config/
	│   ├─ hero.json
	│	├─ documents.json
	│	└─ project.json
	│
    ├── docs/
	│   ├─ README_PT_BR.md
	│   └─ README.md
	│
	├── models/
	│   ├─ openai.yml
	│   ├─ anthropic.yml
	|	├─ google.yml
	|	├─ cursor.yml
	|	├─ moonshot.yml
	|	├─ zhipu.yml
	|	└─ xai.yml
	|
    ├── prompts/
    |
    ├── metrics-summary.md
    |
    ├── templates/
	|   ├─ AGENTS.md
	|   ├─ current-state.md
	|   ├─ context-log.md
	|   ├─ workflow-config.yml
	|   ├─ workflow.md
	|   └─ docs/
	|      ├─ PRD.md
	|      ├─ ADR.md
	|      ├─ UI.md
	|      ├─ DEPLOY.md
	|      └─ TESTING.md
	|
	└─ cycles/
		├─ current/
		|  ├── workflow.md
		|  ├── workflow-config.yml
		|  ├── metrics.md
		|  └── .lock
		|
		└── archive/
			├── C04-2026-06-21-upload-imgurl/
			│   ├── workflow.md
			│   ├── workflow-config.yml
			|   └── metrics.md
			|	 
			|
			└── ...
```

>[!info] OBS
>Se for o primeiro ciclo executado no projeto, a pasta `.workflow-hero/cycles/archive` não vai existir.

6.  O **orchestration_agent** atualiza o arquivo de métricas na etapa de inicialização do projeto: `.workflow-hero/cycles/current/metrics.md` (usando estimativa heurística de tokens por caracteres, ver seção Métricas) e mostra ao usuário um  resumo das métricas de IA desta etapa.
	- Tempo total da sessão;
	- Número de tokens gastos;
	- Custo aproximado. 

> A etapa **Configuration** é sempre executada implicitamente (corresponde aos passos do `/hero:new` e `/hero:start`); nunca é configurável em `workflow-config.yml` e seu `Human Approval` é sempre `N/A`.

7. O  **orchestration_agent** pede para o usuário escolher as opções que ele deseja utilizar no Ciclo través do preenchimento do arquivo `.workflow-hero/cycles/current/workflow-config.yml` e posteriormente utilizar o comando `/hero:start` para começar.

### Início do Ciclo:

1. O usuário inicia o Ciclo de Desenvolvimento através do comando `/hero:start`

>A partir daqui assume o **orchestration_agent** e inicia o processo de contagem dos elementos de estatística.

2. O **orchestration_agent** lê o arquivo de configuração `.workflow-hero/cycles/current/workflow-config.yml`:
	- Caso o arquivo possua algum erro, o agente deve notificar o usuário, sugerir a correção e pedir para o usuário repetir o comando `/hero:start` após a correção.
	- **Validação de dependências entre etapas:** se uma etapa dependente estiver habilitada mas sua etapa pré-requisito estiver desabilitada (ex: `implementation.enabled=true` com `planning.enabled=false`), o agente bloqueia e pede correção. Excepcionalmente, se `research.enabled=false`, a implementação pode seguir direto sobre o campo `objective`, desde que o agente peça confirmação explícita do usuário sobre o escopo (substituto leve de aprovação de PRD).
	- **Validação de `scope`:** se `implementation.enabled=true`, ao menos um dos 5 campos de `scope` (`backend`, `frontend`, `native`, `script`, `infrastructure`) deve ser `true`; caso contrário, bloqueia e pede correção.
	- Caso o preenchimento do arquivo esteja correto, o agente  faz um resumo do que será executado e pede consentimento ao usuário para prosseguir.

3.  O **orchestration_agent** atualiza as informações dos arquivos de **contexto compactado** 

4.  O **orchestration_agent** atualiza o arquivo `.workflow-hero/cycles/current/workflow.md` com as etapas que não foram configuradas pelo usuário no arquivo `.workflow-hero/cycles/current/workflow-config.yml`

5.  O **orchestration_agent** marca como realizada a etapa *configuration*  no arquivo `.workflow-hero/cycles/current/workflow.md`, preenchendo todos os campos pertinentes.

6.  O **orchestration_agent** atualiza o arquivo de métricas na etapa de inicialização do projeto: `.workflow-hero/cycles/current/metrics.md` e mostra ao usuário um  resumo das métricas de IA desta etapa.
- Tempo total da sessão;
- Número de tokens gastos;
- Custo aproximado. 

7. O **orchestration_agent** move o fluxo para a próxima etapa configurada pelo usuário.

> **Fechamento de etapa (regra geral, válida para todas as etapas a partir daqui):** toda etapa do ciclo — Research, Planning, Implementation, QA, Judge, QA End-to-End — termina com a mesma sequência: (a) resumo + pedido de aprovação (respeitando `require_human_approval` e a regra de auto-aprovação interrompível), (b) atualizar `workflow.md`, (c) atualizar `metrics.md` + mostrar resumo de métricas, (d) avançar para a próxima etapa configurada. Essa sequência não será repetida em detalhe a cada etapa abaixo.

### Research

>A partir daqui o **discover_agent** é acionado e inicia o processo de contagem dos elementos de estatística.

2. O **discover_agent** deve buscar contexto nos arquivos abaixo, caso eles existam:

```bash
.
├── AGENTS.md
└── context
    ├── context-log.md
    └── current-state.md
``` 

3. O **discover_agent** verifica se a skill de grilling própria do Hero (`.cursor/skills/grilling/SKILL.md`, distribuída nos assets do Hero) está instalada; se não estiver ele deve pedir permissão para o usuário para instalar. Caso o usuário não aprove a instalação, a etapa de research não poderá ser concluída.

> A skill de grilling é distribuída pelo próprio Hero (sem dependência do repositório externo do mattpocock).

4. O **discover_agent** deve iniciar a sessão de grilling através do comando `/grill-me`

5. Após a sessão de grill-me, baseado nos resultados obtidos e do contexto previamente carregado, o **discover_agent** decide quantos/quais documentos criar, cria os arquivos em `docs/` a partir dos templates fixos por categoria (`assets/cursor/workflow-hero/templates/docs/{PRD,ADR,UI,DEPLOY,TESTING}.md`) e já registra cada um no `documents.json` automaticamente (name, path, purpose), sem perguntar ao usuário quais categorias criar. Numeração: reinicia por ciclo, com prefixo do número do ciclo (`project.json → workflow.cycle`), no formato `[CATEGORIA]-C[XX]-[seq]-[slug].md`. `DEPLOY.md` e `TESTING.md` são documentos vivos, sem numeração, editados no lugar a cada ciclo que os afeta. `docs/architecture/ADR.md` e `docs/product/PRD.md` funcionam como índice de todos os ADRs/PRDs do projeto e são atualizados a cada novo documento criado.

```bash
.
├── AGENTS.md
├── context
│   ├── context-log.md
│   └── current-state.md
└── docs
    ├── architecture
    │   ├── ADR.md                       # índice de todos os ADRs do projeto
    │   ├── ADR-C04-001-[adr-name].md
    │   ├── ADR-C04-002-[adr-name].md
    │   └── ...
    ├── deployment
    │   └── DEPLOY.md                     # documento vivo, sem numeração
    ├── testing
	│   └── TESTING.md                    # documento vivo, sem numeração
    └── product
        ├── PRD.md                        # índice de todos os PRDs do projeto
        ├── PRD-C04-001-[prd-name].md
        ├── PRD-C04-002-[prd-name].md
        ├── ...
        ├── UI-C04-001-[ui-name].md
        ├── UI-C04-002-[ui-name].md 
        └── ...
```

6. Caso o usuário tenha marcado a opção de solicitar aprovação, o **discover_agent** deve fazer um resumo do que foi criado e solicitar uma para o usuário uma das respostas abaixo: 
	- `/hero:approve` para aprovar a etapa de research;
	- `/hero:reject`para rejeitar a etapa de research e iniciar os ajustes com o agente;
	- `/hero:cancel` para cancelar a etapa de research, excluir todas as modificações realizadas e passar para a próxima etapa configurada;
	- `/hero:finish` para finalizar totalmente o Ciclo Atual e arquivar as modificações realizadas.

7. Caso o usuário rejeite a documentação gerada (`/hero:reject`), o agente deve questionar o usuário em relação a quais modificações devem ser realizadas e permanecer no loop até que o usuário digite um dos outros comandos do item 6.

8.  Caso o usuário cancele a etapa de research (`/hero:cancel`), o **discover_agent** deve preencher a etapa de research com o status **cancelled** e com o Human Approval como **Rejected**, bem como as demais seções associadas no arquivo `.workflow-hero/cycles/current/workflow.md`. Posteriormente  pular para para o item 11. 

9. Caso o usuário opte por finalizar o Ciclo (`/hero:finish`), o **discover_agent** deve marcar o `Workflow Execution, Status` como **Finished by User** no arquivo `.workflow-hero/cycles/current/workflow.md` e o Ciclo deve ser encerrada imediatamente. 

10. Caso o usuário aprove a etapa de research (`/hero:approve`),  o **discover_agent** deve preencher a etapa de research como **completed**, bem como as demais seções associadas  no arquivo `.workflow-hero/cycles/current/workflow.md`.

11.  O **discover_agent**  atualiza o arquivo de métricas na etapa de research: `.workflow-hero/cycles/current/metrics.md` e mostra ao usuário um  resumo das métricas de IA desta etapa.
	- Tempo total da sessão;
	- Número de tokens gastos;
	- Custo aproximado. 

12. O **discover_agent** move o fluxo para a próxima etapa configurada pelo usuário.

### Planning

>A partir daqui o **planning_agent** é acionado e inicia o processo de contagem dos elementos de estatística.

1. O **planning_agent** deve verificar se o Openspec está instalado, se não estiver, deve solicitar permissão para o usuário para instalar e instalar. Caso o usuário não permita, não será possível completar a etapa de planejamento.

2. Inicialização do Openspec:

- Caso o Openspec já tenha sido inicializado os seguintes arquivos estarão presentes no projeto:

```text
.
├── .cursor
│   ├── commands
│   │   ├── opsx-apply.md
│   │   ├── opsx-archive.md
│   │   ├── opsx-explore.md
│   │   ├── opsx-propose.md
│   │   └── opsx-sync.md
│   └── skills
│       ├── openspec-apply-change
│       │   └── SKILL.md
│       ├── openspec-archive-change
│       │   └── SKILL.md
│       ├── openspec-explore
│       │   └── SKILL.md
│       ├── openspec-propose
│       │   └── SKILL.md
│       └── openspec-sync-specs
│           └── SKILL.md
└── openspec
    ├── changes
    │   └── archive
    └── specs
```

- Caso não estejam presentes, o  **planning_agent**  deve inicia o Openspec através do comando:
```bash
openspec init --tools cursor
```

- Caso as pastas estejam presentes e tiver conteúdo na pasta `openspec/changes`, além dos conteúdos da pasta `openspec/changes/archive`, como no exemplo abaixo, significa que existe um planejamento previamente realizado que ainda não foi arquivado. Neste caso o   **planning_agent**  deve arquivar este planejamento com o comando:

```chat
/opsx:archive
``` 

Exemplo de conteúdo da pasta `openspec/changes/:
```text
.
└── openspec
    └── changes
        ├── archive
        └── v1-landing-page
            ├── design.md
            ├── proposal.md
            ├── specs
            │   ├── app-scaffold
            │   │   └── spec.md
            │   ├── cookie-consent
            │   │   └── spec.md
            │   ├── core-services
            │   │   └── spec.md
            │   ├── deployment-artifacts
            │   │   └── spec.md
            │   ├── download-distribution
            │   │   └── spec.md
            │   ├── home-page
            │   │   └── spec.md
            │   ├── internationalization
            │   │   └── spec.md
            │   ├── layout-shell
            │   │   └── spec.md
            │   ├── privacy-page
            │   │   └── spec.md
            │   ├── seo-metadata
            │   │   └── spec.md
            │   ├── unit-tests
            │   │   └── spec.md
            │   └── visual-assets
            │       └── spec.md
            └── tasks.md
```

3.b. Se o ciclo afeta um projeto com código já existente (não é um projeto novo do zero), o **planning_agent** chama `/opsx-explore` antes do `/opsx-propose`, para o OpenSpec mapear a base de código relevante antes de planejar.

4. Baseado na documentação do projeto, o **planning_agent** deve então criar ou se necessário atualizar o  arquivo de configuração do Openspec `openspec/config.yaml`. O campo `context:` (lista de "Authoritative sources") é gerado **dinamicamente a partir das entradas do `documents.json`** do ciclo atual — não lista nomes fixos de documentos. Exemplo de resultado gerado:

```yml
schema: spec-driven

context: |
	Project: {{project.name}}.
	Phase: {{project.workflow.phase}}.

	Authoritative sources — read before planning or writing artifacts:
	{{#each documents}}
		- {{path}}
	{{/each}}
		- context/current-state.md
		- context/context-log.md
		- AGENTS.md
 
	Do not invent requirements. If docs are silent, ask the user.
	Stack: {{project.technology.stack}}.

rules:
	proposal:
		- Reference PRD sections by number
		- State what is in scope vs out of scope for this change
	specs:
		- Use Given/When/Then acceptance scenarios
		- Align with PRD acceptance criteria and ADR constraints
	design:
		- Follow the ADRs referenced in documents.json for this cycle
	tasks:
		- Atomic, ordered checklist; include test commands where relevant
		- Mark tasks that can run in parallel vs. tasks that must run in series, so the orchestration_agent can dispatch backend_agent/frontend_agent/generic_agent accordingly
	  
``` 

> Nota: a linha `{{#each documents}}...{{/each}}` acima representa a lógica que o agente de IA executa ao montar o texto final (iterando as entradas do `documents.json`); não é um motor de template com loops reais — o Hero usa apenas substituição simples de placeholders `{{caminho.chave}}` (ver seção Templates do Hero).

5. O **planning_agent** deve então atualizar o Openspec para que ele carrega as configurações, através do comando: 
```bash
	openspec update
``` 

6. O **planning_agent** deve então criar uma seção limpa e passar o prompt  para o Openspec fazer o planejamento do Ciclo atual. Exemplo de prompt, substitua os itens entre [], pelos parâmetros do Ciclo:

```text
/opsx-propose [nome-do-ciclo]

Antes de gerar artefatos, leia todos os documentos listados em @.workflow-hero/config/documents.json, além de:
@context/current-state.md
@AGENTS.md

Planeje a implementação completa do ciclo em uma única change.
Cubra todos os itens P0 e P1 do(s) PRD(s) relevantes. Organize tasks.md em fases ordenadas,
identificando explicitamente quais tarefas podem ser executadas em paralelo (ex: backend + frontend
com contrato de API já definido) e quais devem ser executadas em série.
Não invente requisitos fora das docs.

Utilize subagents sempre que possível.

Ao finalizar o planejamento, atualize os arquivos de contexto compactado: @context/current-state.md e @context/context-log.md
```

> Se `/hero:back` reabrir esta etapa por ambiguidade identificada pelo Judge, o **planning_agent** edita a proposta OpenSpec existente **no lugar** (não arquiva, não recria): atualiza `design.md`/`tasks.md`/`specs/` usando o relatório de ambiguidade do Judge como input, preservando o histórico da mudança no OpenSpec.

### Implementation

>A partir daqui os agentes de implementação (**backend_agent**, **frontend_agent**, **generic_agent**) são acionados conforme o `scope` configurado, e iniciam o processo de contagem dos elementos de estatística.

1. O **orchestration_agent** chama `/opsx-apply` para iniciar a implementação a partir do `tasks.md` gerado no Planning. O próprio OpenSpec conduz a implementação task-by-task, invocando os subagentes correspondentes (backend/frontend/generic) via **Task tool do Cursor**, no modelo configurado em `workflow-config.yml` para cada agente, respeitando a divisão de paralelismo/série definida na SDD.

2. Cada subagente, ao iniciar, avalia se o contexto já recebido (AGENTS.md, current-state.md, SDD) é suficiente; se identificar uma lacuna específica, invoca o **context_agent** por conta própria.

3. Se o modelo configurado para um agente não estiver disponível, aplica-se a cadeia de fallback: 1º modelo do agente → 2º `fallback_model` (bloco no topo do `workflow-config.yml`) com aviso explícito ao usuário → 3º aviso ao usuário pedindo correção da configuração, aguardando `/hero:continue`.

4. Ao final da implementação de cada subagente, segue-se a sequência de fechamento de etapa (ver regra geral definida em "Início do Ciclo").

### QA

>A partir daqui o **qa_agent** é acionado.

1. O **qa_agent** valida testes automatizados, cobertura, build, lint, qualidade e consistência arquitetural do que foi implementado. Critérios são adaptados por `scope`: para `infrastructure`, valida `terraform plan`/`docker build` sem erros; para `script`, valida execução sem erro e idempotência quando aplicável; para `native`, valida o build da plataforma alvo.

2. Se **FAILED**: o `qa_agent` retorna o relatório detalhado; o **orchestration_agent** direciona o fluxo de volta para o(s) agente(s) de implementação apontados no erro, passando o relatório como contexto. Ao terminar a correção, o `qa_agent` roda novamente. Cada rodada consome 1 iteração do `max_iterations` da etapa QA.

3. Ao esgotar `max_iterations` ou `timeout_minutes`: o orchestrator escala para o usuário (Human Approval = `Escalated`), aguardando `/hero:continue` (concede N iterações extras, informadas pelo usuário) ou `/hero:cancel`.

4. Se **OK**: segue-se a sequência de fechamento de etapa.

### Judge

>A partir daqui o **judge_agent** é acionado.

1. O **judge_agent** compara a SDD aprovada com o código implementado, validando exclusivamente cobertura de requisitos (não avalia qualidade/arquitetura — isso é responsabilidade do `qa_agent`).

2. Se **FAILED** por lacuna de implementação: mesmo padrão do QA — volta para o(s) agente(s) de implementação com o relatório do Judge como contexto, consumindo 1 iteração do `max_iterations` do Judge.

3. Se, após esgotadas as lacunas de implementação, o Judge identificar **ambiguidade na própria SDD** (não lacuna de implementação): para o loop e pergunta ao usuário se deve retornar para o planejamento (`/hero:back`, ver seção Planning) ou continuar como está (`/hero:approve` — a etapa é marcada Completed, e o Summary registra explicitamente a ambiguidade aceita pelo usuário, para constar no `context-log.md`).

4. Ao esgotar `max_iterations`/`timeout_minutes` sem chegar a uma conclusão: mesmo escalonamento via `Escalated` + `/hero:continue`.

5. Se **OK**: o `planning_agent` chama `/opsx-sync` (garante que `openspec/specs/` reflita o que foi de fato implementado) e segue-se a sequência de fechamento de etapa.

### QA End-to-End

>A partir daqui o **end2end_qa_agent** é acionado.

1. O **end2end_qa_agent** executa cenários ponta a ponta do ponto de vista do usuário final. O método é selecionado em `workflow-config.yml → stages.qa_end_to_end.use_playwright`: se `use_playwright=true` (exige `scope.frontend=true`), usa **Playwright** para simular jornadas de usuário no navegador; se `use_playwright=false`, usa chamadas HTTP diretas simulando a jornada do cliente da API, sem Playwright. `use_playwright=true` com `scope.frontend=false` é inválido — o orchestrator bloqueia e pede correção.

2. Se **FAILED**: mesmo padrão do QA técnico — volta direto para o(s) agente(s) de implementação com o relatório do erro, consumindo 1 iteração do `max_iterations` da etapa QA End-to-End. Ao esgotar, mesmo escalonamento via `/hero:continue`.

3. Se **OK**: segue-se a sequência de fechamento de etapa. Esta é tipicamente a última etapa do ciclo; ao concluir, o `orchestration_agent` conduz o encerramento do ciclo (`/hero:finish` ou avanço automático conforme configuração).

---
## Arquivos / Templates

### Arquivos que armazenam "variáveis" do sistema:

#### hero.json

Armazena as informações do Hero, exemplo:

```json
{
  "cli": { 	
	  "version": "x.x.x",
	  "installedAt": "2026-07-11T12:00:00Z",
	  "tools": [
	    "cursor"
	  ]
  },
  "assets": {
	  "version": "x.x.x",
	  "installedAt": "2026-07-11T12:00:00Z"
  }
}
```

#### project.json

Armazena as informações relacionadas ao projeto, exemlo:
```json
{
  "name": "project name",
  "summary": "project summary",
  "repository": "project-repository",
  "createdAt": "2026-07-11T12:00:00Z",

  "workflow": {
    "name": "Feature Development",
    "phase": "Research",
    "cycle": 4
  },

  "technology": {
    "stack": "project stack",
    "backend": "project backend",
    "languages": [
      "project languages"
    ]
  },

  "platform": {
    "targets": [
      "project platform targests"
    ]
  },

  "localization": {
    "languages": [
      "project languages"
    ],
    "defaultLanguage": "project default language"
  },

  "ui": {
    "design": "Design style",
    "theme": {
      "default": "light",
      "supportsDarkMode": true
    }
  },

  "deployment": {
    "target": "projet deployment target",
    "domain": "project domain"
  }
}
```

> `workflow.cycle`: contador sequencial global de ciclos, incrementado pelo `orchestration_agent` a cada `/hero:new` bem-sucedido. Usado como prefixo (`C04`) na numeração de documentos (`PRD-C04-001-slug.md`) e no nome da pasta de arquivamento (`C04-yyyy-mm-dd-slug/`). Campos avançados (`technology`, `platform`, `localization`, `ui`, `deployment`) ficam vazios/`null` na instalação via CLI e são preenchidos pelo `orchestration_agent` durante o primeiro `/hero:new`.

#### documents.json

Armazena as informações relacionadas a documentação do projeto, exemplo:

```json
{
  "documents": [
	"testing": {
		"path": "docs/testing/TESTING.md",
		"purpose": "Testing strategy and commands."
	},
    {
      "name": "PRD name",
      "path": "docs/product/PRD-001.md",
      "purpose": "PRD-001 purpose descripition"
    },
    {
      "name": "PRD name",
      "path": "docs/product/PRD-002.md",
      "purpose": "PRD-002 purpose descripition"
    },
    {
      "name": "UI / Design Spec",
      "path": "docs/product/UI.md",
      "purpose": "Visual direction, sections, components, tokens"
    },
    {
      "name": "Architecture Decision Records",
      "path": "docs/architecture/ADR.md",
      "purpose": "Index of all ADRs"
    },
    {
      "name": "ADR-001 Stack",
      "path": "docs/architecture/ADR-001-stack.md",
      "purpose": "Angular 21, Tailwind, zoneless"
    }
  ]
}
```
 

### Documentação do Hero para o Usuário

#### README.md e README_PT_BR.md

Estes dois arquivos devem ser gerados pelo agente que implementar o Hero. Devem conter a descrição do Hero, o que o Hero faz e todas as instruções necessárias para que o usuário precisa para usar o Hero.

### Arquivos de Referência:

#### openai.yml
```yaml
provider: openai
version: 1 
last_updated: 2026-07-14
currency: usd
unit: per_1m_tokens

models:
  gpt-5.4-nano:
    input: 0.2
    cache_write: null
    cache_read: 0.02
    output: 1.25

  gpt-5-mini:
    input: 0.25
    cache_write: null
    cache_read: 0.025
    output: 2

  gpt-5.1-codex-mini:
    input: 0.25
    cache_write: null
    cache_read: 0.025
    output: 2

  gpt-5.4-mini:
    input: 0.75
    cache_write: null
    cache_read: 0.075
    output: 4.5

  gpt-5.6-luna:
    input: 1
    cache_write: 1.25
    cache_read: 0.1
    output: 6

  gpt-5:
    input: 1.25
    cache_write: null
    cache_read: 0.125
    output: 10

  gpt-5-codex:
    input: 1.25
    cache_write: null
    cache_read: 0.125
    output: 10

  gpt-5.1-codex:
    input: 1.25
    cache_write: null
    cache_read: 0.125
    output: 10

  gpt-5.1-codex-max:
    input: 1.25
    cache_write: null
    cache_read: 0.125
    output: 10

  gpt-5.2:
    input: 1.75
    cache_write: null
    cache_read: 0.175
    output: 14

  gpt-5.2-codex:
    input: 1.75
    cache_write: null
    cache_read: 0.175
    output: 14

  gpt-5.3-codex:
    input: 1.75
    cache_write: null
    cache_read: 0.175
    output: 14

  gpt-5.4:
    input: 2.5
    cache_write: null
    cache_read: 0.25
    output: 15

  gpt-5-fast:
    input: 2.5
    cache_write: null
    cache_read: 0.25
    output: 20

  gpt-5.6-terra:
    input: 2.5
    cache_write: 3.125
    cache_read: 0.25
    output: 15

  gpt-5.5:
    input: 5
    cache_write: null
    cache_read: 0.5
    output: 30

  gpt-5.6-sol:
    input: 5
    cache_write: 6.25
    cache_read: 0.5
    output: 30
```


#### anthropic.yml

```yaml
provider: anthropic
version: 1 
last_updated: 2026-07-14
currency: usd
unit: per_1m_tokens

models:
  claude-4.5-haiku:
    input: 1
    cache_write: 1.25
    cache_read: 0.1
    output: 5

  claude-4-sonnet:
    input: 3
    cache_write: 3.75
    cache_read: 0.3
    output: 15

  claude-4.5-sonnet:
    input: 3
    cache_write: 3.75
    cache_read: 0.3
    output: 15

  claude-4.6-sonnet:
    input: 3
    cache_write: 3.75
    cache_read: 0.3
    output: 15

  claude-sonnet-5:
    input: 3
    cache_write: 3.75
    cache_read: 0.3
    output: 15

  claude-4-sonnet-1m:
    input: 6
    cache_write: 7.5
    cache_read: 0.6
    output: 22.5

  claude-4.5-opus:
    input: 5
    cache_write: 6.25
    cache_read: 0.5
    output: 25

  claude-4.6-opus:
    input: 5
    cache_write: 6.25
    cache_read: 0.5
    output: 25

  claude-4.7-opus:
    input: 5
    cache_write: 6.25
    cache_read: 0.5
    output: 25

  claude-opus-4.8:
    input: 5
    cache_write: 6.25
    cache_read: 0.5
    output: 25

  claude-fable-5:
    input: 10
    cache_write: 12.5
    cache_read: 1
    output: 50

  claude-opus-4.7-fast:
    input: 30
    cache_write: 37.5
    cache_read: 3
    output: 150
```

#### google.yml
```yaml
provider: google
version: 1 
last_updated: 2026-07-14
currency: usd
unit: per_1m_tokens

models:
  gemini-2.5-flash:
    input: 0.3
    cache_write: null
    cache_read: 0.03
    output: 2.5

  gemini-3-flash:
    input: 0.5
    cache_write: null
    cache_read: 0.05
    output: 3

  gemini-3.5-flash:
    input: 1.5
    cache_write: null
    cache_read: 0.15
    output: 9

  gemini-3-pro:
    input: 2
    cache_write: null
    cache_read: 0.2
    output: 12

  gemini-3-pro-image-preview:
    input: 2
    cache_write: null
    cache_read: 0.2
    output: 12

  gemini-3.1-pro:
    input: 2
    cache_write: null
    cache_read: 0.2
    output: 12
```

#### cursor.yml
```yaml
provider: cursor
version: 1 
last_updated: 2026-07-14
currency: usd
unit: per_1m_tokens

models:
  composer-1:
    input: 1.25
    cache_write: null
    cache_read: 0.125
    output: 10

  composer-2.5:
    input: 0.5
    cache_write: null
    cache_read: 0.2
    output: 2.5

  composer-2.5-fast:
    input: 3
    cache_write: null
    cache_read: 0.5
    output: 15
```

#### moonshot.yml
```yaml
provider: moonshot
version: 1 
last_updated: 2026-07-14
currency: usd
unit: per_1m_tokens

models:
  kimi-k2.7-code:
    input: 0.95
    cache_write: null
    cache_read: 0.19
    output: 4
```

#### zhipu.yml
```yaml
provider: zhipu
version: 1 
last_updated: 2026-07-14
currency: usd
unit: per_1m_tokens

models:
  glm-5.2:
    input: 1.4
    cache_write: null
    cache_read: 0.26
    output: 4.4
```

#### xai.yml
```yaml
provider: xai
version: 1 
last_updated: 2026-07-14
currency: usd
unit: per_1m_tokens

models:
  grok-4.5:
    input: 2
    cache_write: null
    cache_read: 0.5
    output: 6
```


### Subagents

#### planning_agent.md

```markdown
# Planning Agent

## Role

You are the Planning Agent of Workflow Hero.

Your responsibility is to transform approved product and architecture specifications into a complete Software Design Document (SDD) that is ready for implementation.

You are responsible for using the OpenSpec methodology to create a detailed and implementation-oriented specification.

## Responsibilities

- Analyze the approved research artifacts:
  - PRD
  - ADR
  - Architecture documentation
  - Deployment documentation
  - Other approved specifications

- Create a complete SDD specification.

- Define:
  - System behavior
  - Components involved
  - Data flows
  - APIs
  - Database changes
  - Integration points
  - Technical constraints
  - Acceptance criteria

- Ensure the SDD is precise enough that another agent can implement it without additional clarification.

## Rules

- Do not implement code.
- Do not change product requirements.
- Do not introduce architectural decisions without an approved ADR.
- If requirements are unclear, report the ambiguity.

## Output Format

Return:

- SDD specification
- Identified assumptions
- Open questions
- Risks or technical concerns


``` 

#### context_agent.md

```markdown
# Context Agent

## Role

You are the Context Agent of Workflow Hero.

Your responsibility is to retrieve relevant project context whenever requested by another agent.

You are a shared service that can be invoked during any workflow stage.

## Responsibilities

Retrieve information from:

- Project documentation
- Source code
- Architecture documents
- Workflow artifacts
- Existing implementations
- Configuration files

Identify:

- Existing patterns
- Related implementations
- Coding conventions
- Architectural decisions
- Dependencies
- Potential impacts

## Rules

- Never modify project files.
- Never implement code.
- Never make architectural decisions.
- Never infer missing information without stating assumptions.
- Only provide relevant context requested by the caller.

## Output Format

## Summary

Short summary of the retrieved context.

## Relevant Files

Files that should be considered.

## Existing Patterns

Coding or architectural patterns found.

## Dependencies

Relevant dependencies.

## Risks

Potential concerns.

## References

Links to documentation or implementation files.
```

#### backend_agent.md

```markdown
# Backend Agent

## Role

You are the Backend Implementation Agent of Workflow Hero.

Your responsibility is to implement backend changes according to the approved SDD specification.

## Responsibilities

- Analyze the SDD.
- Implement backend features.
- Follow existing project architecture and coding standards.
- Reuse existing patterns.
- Create or update backend components.
- Implement required tests when applicable.

## Rules

- Do not modify frontend code.
- Do not change architecture without an approved ADR.
- Do not implement features that are not described in the SDD.
- Prefer simple and maintainable solutions.

## Validation

Before finishing:

- Run available backend tests.
- Verify build succeeds.
- Verify implementation follows project conventions.

## Output Format

Return:

## Implementation Summary

Changes implemented.

## Files Changed

List of modified files.

## Validation

Tests and checks executed.

## Issues

Problems or blockers found.
```

#### frontend_agent.md

```markdown
# Frontend Agent

## Role

You are the Frontend Implementation Agent of Workflow Hero.

Your responsibility is to implement frontend changes according to the approved SDD specification.

## Responsibilities

- Analyze the SDD.
- Implement frontend features.
- Follow existing frontend architecture and coding standards.
- Respect existing UI patterns.
- Implement required state management, components, services, and tests.

## Rules

- Do not modify backend code.
- Do not introduce architectural changes without approval.
- Do not implement behavior not described in the SDD.

## Validation

Before finishing:

- Run frontend tests.
- Validate build process.
- Verify implementation follows existing conventions.

## Output Format

Return:

## Implementation Summary

Changes implemented.

## Files Changed

List of modified files.

## Validation

Tests and checks executed.

## Issues

Problems or blockers found.
```

#### generic_agent.md

> Adicionado na sessão de grilling de 2026-07-20, para cobrir `scope.native`, `scope.script` e `scope.infrastructure`.

```markdown
# Generic Agent

## Role

You are the Generic Implementation Agent of Workflow Hero.

Your responsibility is to implement changes that do not fall under backend or frontend scope: native applications (Linux/Windows), standalone scripts, and infrastructure code (IaC, Dockerfiles, CI/CD pipelines).

## Responsibilities

- Analyze the SDD.
- Implement the tasks assigned to the `native`, `script`, or `infrastructure` scope, as applicable.
- Follow existing project conventions for the relevant scope.
- Reuse existing patterns.

## Rules

- Do not modify backend or frontend application code unless explicitly instructed by the SDD.
- Do not introduce architectural changes without an approved ADR.
- Do not implement behavior not described in the SDD.

## Validation

Before finishing, validate according to the active scope:

- `native`: verify the build succeeds for the target platform(s).
- `script`: verify the script executes without error; verify idempotency when applicable.
- `infrastructure`: verify `terraform plan` / `docker build` (or equivalent) completes without error.

## Output Format

Return:

## Implementation Summary

Changes implemented.

## Files Changed

List of modified files.

## Validation

Tests and checks executed.

## Issues

Problems or blockers found.
```

#### qa_agent.md

```markdown
# QA Agent

## Role

You are the Quality Assurance Agent of Workflow Hero.

Your responsibility is to validate the technical quality of the implementation.

You verify whether the implementation follows engineering quality standards.

## Responsibilities

Validate:

- Automated tests.
- Test coverage.
- Build success.
- Linting.
- Code quality.
- Architecture consistency.
- Implementation patterns.
- Potential technical issues.

Adapt validation criteria to the active `scope` for this cycle:

- `backend` / `frontend`: unit/integration tests, coverage, lint, build, architecture and pattern consistency (as above).
- `native`: verify the build succeeds for the target platform(s) declared in the SDD.
- `script`: verify the script executes without error; verify idempotency when the SDD calls for it.
- `infrastructure`: verify `terraform plan` / `docker build` (or the declared equivalent) completes without error.

## Rules

- Do not implement features.
- Do not modify code unless explicitly requested.
- Focus on validation and reporting.

## Output Rules

If everything is correct, return:

OK

If problems are found, return:

FAILED

with:

- Detailed issue description.
- Affected files.
- Expected behavior.
- Suggested correction.
```

#### judge_agent.md

```markdown
# Judge Agent

## Role

You are the Judge Agent of Workflow Hero.

Your responsibility is to verify whether the implementation completely satisfies the approved SDD specification.

## Responsibilities

Compare:

- Approved SDD
- Implemented code
- Acceptance criteria

Validate:

- All requirements were implemented.
- No planned functionality is missing.
- Implementation matches specifications.
- No unauthorized changes were introduced.

## Rules

- Do not evaluate coding style, architecture consistency, or implementation patterns — these are the exclusive responsibility of the `qa_agent`.
- Do not replace QA validation.
- Focus exclusively on specification compliance: coverage of SDD requirements vs. what was implemented.
- If a gap is caused by ambiguity in the SDD itself (not a missing implementation), do not loop indefinitely with the implementation agents. Escalate to the user, asking whether to reopen Planning (`/hero:back`) or accept as-is (`/hero:approve`).

## Output Rules

If implementation satisfies the SDD:

Return:

OK

If something is missing:

Return:

FAILED

with:

- Missing requirement.
- Related SDD section.
- Expected implementation.
- Recommended action.
```

#### end2end_qa_agent.md

```markdown
# End-to-End QA Agent

## Role

You are the End-to-End QA Agent of Workflow Hero.

Your responsibility is to validate the complete user workflow from an external perspective.

## Responsibilities

- Execute end-to-end scenarios.
- Validate user journeys.
- Verify integration between system components.
- Identify functional problems.
- Confirm expected behavior.

## Rules

- Do not modify implementation.
- Do not evaluate internal code quality.
- Focus on user-visible behavior.

## Output Rules

If everything works as expected:

Return:

OK

If problems are found:

Return:

FAILED

with:

- Scenario executed.
- Expected result.
- Actual result.
- Error details.
- Steps to reproduce.
```

### Templates do Hero

Os templates do Hero são um **Bootstrap** para os arquivos do projeto. Os Agentes do Hero devem evoluir estes arquivos de acordo com o desenvolvimento do projeto.

#### AGENTS.md

Arquivo destinado a orientar os agentes de IA que operam no projeto com as regras gerais do projeto.
Este Arquivo deve ser mantido pequeno e objetivo, entre 300 e 700 palavras.

O objetivo é ser um **prompt permanente**, não uma documentação.

Deve conter apenas:
- Resumo do projeto
- Índice da documentação
- Restrições
- Regras de desenvolvimento
- Regras de teste
- Definition of Done

**Nunca** deve conter:
- Arquitetura detalhada
- Requisitos completos
- ADRs completos
- Grandes listas de features
Tudo isso deve ser referenciado.

```markdown
# AGENTS.md  

Guidance for AI agents working on the {{project.name}} repository.

> Stable project instructions.
>
> Keep this document concise. Target maximum size: 700 words.

## Project Summary

{{project.summary}}

## Documentation Map

Always read project documentation **before** writing code or making architectural decisions.  

| Document | Path | Purpose |
|---|---|---|
{{#documents}}
| {{name}} | [{{path}}]({{path}}) | {{purpose}} |
{{/documents}}  

### Context Compression Files

These files must be **kept up to date after every code interaction**.  

| File | Path | Lifetime | Purpose |

|---|---|---|---|

| Current State | [context/current-state.md](context/current-state.md) | Long-lived | Single source of truth: project name, goal, stack, implemented/pending features, architecture, constraints |

| Context Log | [context/context-log.md](context/context-log.md) | Short/medium-lived | Operational memory: timestamps, problems, investigations, decisions, outcomes, refactors, rationale |

**After every session:**

1. Update `context/current-state.md` to reflect what was built, what changed, and what remains.

2. Append a new entry to `context/context-log.md` with the session's decisions and outcomes.

  
## Testing

Follow the project's testing strategy described in {{documents.testing.path}}.

Always:

- Add or update tests for any business logic changes.
- Run the project's automated test suite before completing a task.
- Fix any failing tests before finishing.
- Never leave the project in a failing state.
  

## Reference Lookup Order

When an agent needs external or internal reference material, follow this order:

1. **Project documents** — PRD, UI spec, ADRs, DEPLOY.md, and context files listed above.

2. **Context7 MCP** — for library/framework documentation (Angular, Tailwind, nginx, etc.).

3. **Web search** — only when project docs and Context7 do not answer the question.

Do not guess or invent requirements. If project docs are silent on a topic, ask the user.

## Ambiguity and Missing Information

**Any ambiguous requirement or missing information must be questioned to the user before proceeding.**

Do not assume defaults that contradict documented decisions. Do not silently fill gaps in the PRD, UI spec, or ADRs. When in doubt:

1. Check project docs and context files first.

2. If still unclear, ask the user a specific question with a recommended option.

3. Record the user's answer in `context/context-log.md` and update affected docs if the decision changes scope.


## Project Constraints

The following constraints are mandatory and must not be violated.

- Technology stack: {{project.technology.stack}}
- Supported platforms: {{project.platform.targets}}
- Supported languages: {{project.localization.languages}}
- Deployment target: {{project.deployment.target}}
- UI guidelines: {{project.ui.design}}

For complete details, see:

- {{documents.prd.path}}
- {{documents.architecture.path}}
- {{documents.testing.path}}

  
_To be maintained by agents._

    
```

#### current-state.md

Este arquivo é de longa duração, é a fonte da verdade do projeto. Seu objetivo principal é dar contexto para os agentes, sem que seja necessário ler todo o código.  Trata-se do contexto operacional do projeto e deve conter **apenas o estado atual** do projeto, coisas como:

- Features implementadas
- Features em andamento
- Decisões recentes
- Débito técnico
- Próximos passos
- Resumo da arquitetura atual

Evite histórico completo. Em vez de:

```
2026-05...
2026-06...
2026-07...
```

mantenha apenas o **estado consolidado**.

Este arquivo deve ser mantido pelos agentes e não deve ultrapassar 2000 palavras.

```markdown
# Current State

> Long-lived document. Single source of truth for the project.
>
> Must be updated after every implementation cycle.
> 
> Keep this document under 2,000 words by consolidating information and removing obsolete content.

---
## Project Identity

| Field | Value |

|---|---|

| **Name** | {{project.name}} |

| **Repository** | {{project.repository}} | 

| **Domain** | {{project.deployment.domain}} |

| **Phase** | {{project.workflow.phase}}|

---

_To be maintained by Hero agents._

```

#### context-log.md

Este arquivo é a memória operacional recente do projeto. armazena o  histórico de decisões dentro de um ciclo. 

**O que deve conter:**
Apenas informações úteis para a **próxima iteração**, por exemplo:
- O que acabou de ser implementado.
- Problemas encontrados.
- Decisões temporárias.
- Próximos passos imediatos.
- Pendências conhecidas.

** O que não deve conter:**
- Histórico completo.
- Lista de todas as features.
- Arquitetura.
- PRD resumido.
- ADRs.
- Informações permanentes (essas pertencem ao `current-state.md`).

Este arquivo deve ser mantido pelos agentes e não deve ultrapassar 1000 palavras.

```markdown
# Context Log

> Short-term project memory.
> 
> Keep only information relevant to the last 3–5 implementation cycles.
>
> Keep this document under 1,000 words by removing or consolidating outdated entries.

---  

## 2026-06-23 — Planning session (grilling)

**Problem:** Empty repository. Need to define all requirements for the Screenshot Hero landing page before implementation.

**Investigation:** Ran a structured grilling session (/grill-me) covering product goals, download strategy, UX, architecture, deployment, and coding standards. Reviewed Screenshot Hero README (beta_version branch) for product context.

 
**Decisions:**

| # | Topic | Decision | Rationale |

|---|---|---|---|

| 1 | Download hierarchy | Flathub (coming soon) → direct .flatpak → local build | Flathub not live yet; .flatpak aligns with existing Flatpak pipeline |

| 2 | Binary format | `.flatpak` bundle, AMD64 only | App is Flatpak-first; tested on Fedora + Ubuntu |

| 3 | Cookie consent | Accept/decline banner; persist theme/locale only if accepted | LGPD compliance without blocking session UX |

| 4 | i18n | Runtime JSON + custom TranslationService | Single build, instant toggle, no @angular/localize |

| 5 | Theme | Light default, manual toggle, no OS detection | User preference for simplicity |

| 6 | Page sections | Header → Hero → Problem → How It Works → Features → Download → Contribute → Footer | Standard landing page flow |

| 7 | Visual design | GNOME/Libadwaita inspired | Matches target platform and product identity |

| 8 | Release delivery | releases.json + /downloads/ volume | Update releases without rebuilding Angular |

| 9 | Deployment | nginx 1.30.3-alpine, hardened, Dockerploy | Security-first static serving; pinned version for CVE patches |

| 10 | Angular architecture | Zoneless + signals, feature-based, lazy routes | Modern idiomatic Angular; strong isolation |

| 11 | Privacy | /privacy page, bilingual | Required for cookie consent compliance |

| 12 | SEO | Canonical schero.codethings.dev, OG image, JSON-LD | Organic discovery |

| 13 | a11y | Minimal v1 | Ship fast; improve later |

| 14 | Download fallback | Graceful degradation when releases.json fails | Page never breaks |

| 15 | Assets | Bundled in repo, NgOptimizedImage | No fragile GitHub hotlinks |

| 16 | Project structure | core/shared/layout/features | Feature isolation with lazy privacy route |

| 17 | Coding standards | Strict types, signals, @if/@for, inject(), no Material | Team conventions documented in ADR-007 |

| 18 | Footer | Code Things credit (codethings.dev) | Company attribution |

| 19 | Domain | schero.codethings.dev primary; codethings.dev redirects | No separate Code Things landing yet |
  

**Result:** Planning complete. All decisions documented in PRD, UI spec, 7 ADRs, and DEPLOY.md. No code written.
 
**Next step:** Another agent should scaffold the Angular 21 project following documented architecture and coding standards.  

--- 

## 2026-06-23 — Documentation creation

**Problem:** Need formal project documents so another agent can plan and implement without re-running the grilling session.
  
**Investigation:** Consolidated all grilling decisions into structured documentation.

**Actions taken:**

- Created `AGENTS.md` with doc map, reference lookup order, ambiguity policy

- Created `docs/product/PRD.md` — full product requirements

- Created `docs/product/UI.md` — design specification with tokens, components, layout

- Created `docs/architecture/ADR.md` + ADR-001 through ADR-007

- Created `docs/deployment/DEPLOY.md` — Docker, nginx, release procedure

- Created `context/current-state.md` — project source of truth

- Created `context/context-log.md` — this file

**Result:** Repository ready for implementation planning. All decisions captured in English.  

**Next step:** Implementation agent reads AGENTS.md → PRD → ADRs → begins Angular scaffold.

---

_To be maintained by agents._

```

#### openspec/config.yaml

```yml
schema: spec-driven

context: |
	Project: Screenshot Hero Landing Page (schero.codethings.dev).
	Phase: planning complete; no app code yet.

	Authoritative sources — read before planning or writing artifacts:
		- docs/product/PRD.md
		- docs/product/UI.md
		- docs/architecture/ADR.md and ADR-001 through ADR-007
		- docs/deployment/DEPLOY.md
		- context/current-state.md
		- context/context-log.md
		- AGENTS.md
 
	Do not invent requirements. If docs are silent, ask the user.
	Stack: Angular 21 zoneless, Tailwind, i18n en + pt-BR, .flatpak AMD64.

rules:
	proposal:
		- Reference PRD sections by number
		- State what is in scope vs out of scope for this change
	specs:
		- Use Given/When/Then acceptance scenarios
		- Align with PRD acceptance criteria and ADR constraints
	design:
		- Follow ADR-001 (stack), ADR-002 (structure), ADR-007 (standards)
	tasks:
		- Atomic, ordered checklist; include npm test where relevant
	  
``` 


#### workflow-config.yml

Este arquivo deve conter todas as configurações que o usuário deve escolher para executar o Hero.

```yaml
# Workflow Hero Configuration
#
# This file defines the development cycle configuration.
# Configure the workflow stages, AI models, and execution settings
# before starting a new workflow.

title: New Feature

objective: Implement a new feature.

# Fallback model used when an agent's configured model is unavailable.
# The user is always warned explicitly when this fallback is used.
fallback_model:
	model: claude-sonnet-5
	reasoning_effort: medium
	enable_fast_model: false
	thinking: na

scope:
	backend: true
	frontend: false
	native: false
	script: false
	infrastructure: false

stages:

	research:
	    enabled: true
	    purpose: Collaborate with the user to gather requirements and produce the project specifications (e.g., PRD, ADR, Deployment Guide, Architecture Notes).
	    max_iterations: 50
	    timeout_minutes: 15
	    require_human_approval: false
    
	planning:
		enabled: true
		 purpose: Convert the approved project specifications into a complete Software Design Document (SDD) ready for implementation.
		 max_iterations: 3
		 timeout_minutes: 20
		 require_human_approval: false

	implementation:
		enabled: true
	    purpose: Implement the approved SDD specifications or direct implements the PRDs and ADRS, depends of the user choices
	    max_iterations: 4
	    timeout_minutes: 30
	    require_human_approval: false
	    
	qa:
		enabled: true
		purpose: Validate implementation quality through automated tests, code coverage, software architecture, linting, build verification, and implementation-level checks. 
		max_iterations: 2
		timeout_minutes: 15
		require_human_approval: true

	judge:
	    enabled: true
	    purpose: Verify that all approved SDD specifications have been fully implemented and that the implementation satisfies the defined requirements and acceptance criteria.
	    max_iterations: 3
	    timeout_minutes: 10
	    require_human_approval: false

	qa_end_to_end:
	    enabled: true
	    purpose: Validate the complete feature through end-to-end testing to ensure the system behaves as expected.
	    max_iterations: 1
	    timeout_minutes: 15
	    require_human_approval: true
	    # When true and scope.frontend is true, end2end_qa_agent uses Playwright.
	    use_playwright: false

agents:
  
	planning_agent:
		model: gpt-5.3-codex
		reasoning_effort: medium
		enable_fast_model: false
		thinking: na
    
	context_agent:
		model: composer-2.5
		reasoning_effort: na
		enable_fast_model: false
		thinking: na
		
	backend_agent:
		model: grok-4.5
		reasoning_effort: high
		enable_fast_model: false
		thinking: na
		
	frontend_agent:
		model: claude-sonnet-5
		reasoning_effort: high
		enable_fast_model: false
		thinking: false

	generic_agent:
		model: claude-sonnet-5
		reasoning_effort: medium
		enable_fast_model: false
		thinking: false

	qa_agent:
		model: gpt-5.3-codex
		reasoning_effort: medium
		enable_fast_model: false
		thinking: na

	judge_agent:
		model: claude-sonnet-5
		reasoning_effort: high
		enable_fast_model: false
		thinking: false
		
	end2end_qa_agent:
		model: claude-4.6-sonnet
		reasoning_effort: medium
		enable_fast_model: false
		thinking: false
				
workflow_rules:

  - Skip any stage that is not enabled.
  - Do not start implementation until the PRD has been approved, if the research stage is enabled.
  - If the research stage is disabled, require the `objective` field above to be well described and ask the user for explicit scope confirmation before starting implementation.
  - At least one of the five `scope` fields must be true when implementation is enabled.
  - Do not change the architecture without an approved ADR.
  - Update workflow.md after completing each stage.
  - Before finishing the workflow, ensure current-state.md is up to date.

```

#### workflow.md

Este arquivo deve conter qual etapa do ciclo de desenvolvimento está sendo executada e seu resultado final.

> Valores possíveis de `Status`: `Waiting | Disable | In Progress | Completed | Cancelled | Paused`.
> Valores possíveis de `Human Approval`: `N/A | Disable | Pending | Escalated | Rejected | Approved | Cancelled`.
> Cada etapa individual (seção "Stage Execution") ganha um campo adicional `Extra Iterations Granted` (default `+0`), incrementado a cada `/hero:continue`.

```markdown
# Workflow Execution

## Title

New Feature

## Status

## Started At

## Completed At

## Current Stage

---

# Execution Summary

|Stage|Status|Iteration|Human Approval|
|---|---|---|---|
|Configuration|Waiting|1|NA|
|Research|Waiting|1/1|Pending|
|Planing|Waiting|1/1|Pending|
|Implementation|Waiting|2/4|Pending|
|QA|Waiting|1/1|Pending|
|Judge|Waiting|1/1|Pending|
|QA End-to-End|Waiting|1/1|Pending|

---

# Stage Execution

## Configuration

**Status:** -

**Iteration:** -

**Started:** -

**Finished:** -

**Human Approval:** -

### Summary

Not started.

---

## Research

**Status:** -

**Iteration:** -

**Started:** -

**Finished:** -

**Human Approval:** -

### Summary

Not started.

---

## Planing

**Status:** -

**Iteration:** -

**Started:** -

**Finished:** -

**Human Approval:** -

### Summary

Not started.

---

## Implementation

**Status:** -

**Iteration:** -

**Started:** -

**Finished:** -

**Human Approval:** -

### Summary

Not started.

---

## QA

**Status:** -

**Iteration:** -

**Started:** -

**Finished:** -

**Human Approval:** -

### Summary

Not started.

---

## Judge

**Status:** -

**Iteration:** -

**Started:** -

**Finished:** -

**Human Approval:** -

### Summary

Not started.

---

## QA End-to-End

**Status:** -

**Iteration:** -

**Started:** -

**Finished:** -

**Human Approval:** - 

### Summary

Not started.

```

#### metrics.md

Este arquivo registra o consumo estimado de tokens, tempo e custo de cada etapa do ciclo atual. As estimativas usam a heurística de aproximação por caracteres (~4 caracteres por token) multiplicada pelo preço do modelo em `models/*.yml`. Quando mais de um agente atua na mesma etapa (ex: Implementation com backend_agent + frontend_agent), a etapa ganha sub-linhas — uma por agente — com uma linha de subtotal.

```markdown
# Metrics — Current Cycle

| Stage | Agent | Model | Time | Input Tokens | Output Tokens | Cache Tokens | Estimated Cost |
|---|---|---|---|---|---|---|---|
| Configuration | orchestration_agent | claude-sonnet-5 | - | - | - | - | - |
| Research | discover_agent | claude-sonnet-5 | - | - | - | - | - |
| Planning | planning_agent | gpt-5.3-codex | - | - | - | - | - |
| Implementation (subtotal) | — | — | - | - | - | - | - |
| ↳ Implementation | backend_agent | grok-4.5 | - | - | - | - | - |
| ↳ Implementation | frontend_agent | claude-sonnet-5 | - | - | - | - | - |
| QA | qa_agent | gpt-5.3-codex | - | - | - | - | - |
| Judge | judge_agent | claude-sonnet-5 | - | - | - | - | - |
| QA End-to-End | end2end_qa_agent | claude-4.6-sonnet | - | - | - | - | - |
| **Total** | — | — | **-** | **-** | **-** | **-** | **-** |

_To be maintained by Hero agents. Updated at the end of every stage._
```

#### metrics-summary.md

Arquivo agregado, fora de `cycles/`, com o total acumulado de tokens/custo/tempo de todos os ciclos do projeto desde a instalação do Hero. Atualizado pelo `orchestration_agent` ao final de cada ciclo, somando os totais do `metrics.md` que está sendo arquivado.

```markdown
# Metrics Summary — All Cycles

| Cycle | Title | Completed At | Total Time | Total Tokens | Total Estimated Cost |
|---|---|---|---|---|---|
| C01 | ... | ... | - | - | - |
| C02 | ... | ... | - | - | - |
| **Project Total** | — | — | **-** | **-** | **-** |

_To be maintained by Hero agents. Updated at the end of every cycle._
```

---
## Decisões da Sessão de Grilling — 2026-07-20

> Registro consolidado das decisões tomadas em sessão de `/grill-me` sobre este documento. Cada decisão referencia o tópico que ela resolve ou complementa. Este log deve ser incorporado ao corpo do documento numa próxima revisão editorial; por ora, serve como fonte da verdade para as lacunas identificadas.

### Aprovação e Loops de Controle

1. **Auto-aprovação (`require_human_approval: false`)**: o agente completa a etapa automaticamente (Status=Completed, Human Approval=Disable), posta um resumo curto e já segue para a próxima etapa no mesmo turno. Como ainda é um chat, o usuário pode interromper com `/hero:reject` ou `/hero:cancel` antes da próxima etapa começar, se for rápido — não há travamento retroativo.

2. **Padrão de aprovação idêntico em todas as etapas**: o padrão definido para Research (`/hero:approve`, `/hero:reject`, `/hero:cancel`, `/hero:finish`) é idêntico em Planning, Implementation, QA, Judge e QA End-to-End, trocando apenas o agente e os artefatos envolvidos. `/hero:reject` faz o agente perguntar o que ajustar e permanecer no loop; `/hero:cancel` limpa as mudanças da etapa (via checkpoint git) e avança.

3. **Sequência de fechamento de etapa (regra geral, não repetida por etapa)**: toda etapa termina com (a) resumo + pedido de aprovação, (b) atualizar `workflow.md`, (c) atualizar `metrics.md` + mostrar resumo de métricas, (d) avançar para a próxima etapa configurada.

4. **Esgotamento de `max_iterations`**: o orchestration_agent para o loop e escala para o usuário, mostrando o que falhou nas tentativas e aguardando uma decisão explícita. Novo comando: **`/hero:continue`**.

5. **Semântica de `/hero:continue`**: ao ser chamado, o agente **pergunta ao usuário quantas iterações extras conceder**, e retoma exatamente de onde parou. As iterações extras concedidas são registradas apenas no `workflow.md` daquela etapa (campo `Extra Iterations Granted: +N`), sem alterar o `max_iterations` original do `workflow-config.yml`.

6. **Loop de falha do QA**: volta direto para o(s) agente(s) de implementação apontados no erro, passando o relatório do QA como contexto; ao terminar, roda o `qa_agent` de novo. Consome 1 iteração do `max_iterations` da etapa QA.

7. **Loop de falha do Judge**: para lacunas de implementação, mesmo padrão do QA (volta para implementação, consome 1 iteração do Judge). Após esgotadas as lacunas, **se existir ambiguidade na SDD**, o Judge para e pergunta ao usuário se deve retornar para o planejamento ou continuar como está. Novo comando: **`/hero:back`**.
   - Se o usuário optar por "continuar como está": usa `/hero:approve` — a etapa Judge é marcada Completed, mas o Summary registra explicitamente a ambiguidade aceita pelo usuário (para constar no `context-log.md`), e o fluxo segue para QA End-to-End normalmente.

8. **Semântica de `/hero:back`**: reabre o Planning (Status volta para In Progress, com o relatório de ambiguidade do Judge como contexto). O `planning_agent` **edita a proposta OpenSpec existente no lugar** (não arquiva, não recria) — atualiza `design.md`/`tasks.md`/`specs/` diretamente, preservando o histórico da mudança no OpenSpec. Ao ser reaprovado, Implementation, QA e Judge são **resetados para Waiting** e re-executados do zero, pois a SDD mudou.

9. **Loop de falha do QA End-to-End**: mesmo padrão do QA técnico (volta para implementação, consome 1 iteração, mesma lógica de escalonamento via `/hero:continue` ao esgotar).

10. **Novo valor de `Human Approval`**: adicionado **`Escalated`**, distinto de `Pending`, usado quando `max_iterations` se esgota e o Hero aguarda decisão do usuário (`/hero:continue`, `/hero:back`, `/hero:approve` ou `/hero:cancel`).

11. **Novo valor de `Status`**: adicionado **`Paused`**, usado na etapa que estava em andamento no momento de um arquivamento manual via `/hero:archive`.

12. **`/hero:archive` manual**: usado para arquivar o ciclo atual mesmo em andamento (sem iniciar um novo ciclo), diferente de `/hero:cancel` (que descarta o progresso). A etapa em andamento recebe Status=`Paused`.

13. **Retomando um ciclo pausado**: novo comando **`/hero:resume [nome-ou-id-do-ciclo]`**, que move a pasta de volta de `archive/` para `current/` (arquivando o que estiver em `current/` atualmente, se houver) e continua da etapa marcada como `Paused`.

14. **Mecanismo de reversão do `/hero:cancel`**: baseado em git. Ao iniciar cada etapa, o orchestrator garante um commit/checkpoint limpo; `/hero:cancel` faz `git checkout`/`git restore` dos arquivos alterados desde esse checkpoint. **Git passa a ser pré-requisito obrigatório** do Hero (ver seção Pré-requisitos).

### Orquestração de Implementação

15. **Backend vs. Frontend em paralelo**: o `planning_agent` deve prever, na própria SDD, quais tarefas podem ser executadas em paralelo e quais devem ser executadas em série. Sempre incentivar o uso de subagents quando possível. Não há uma ordem fixa (backend-first); a ordem/paralelismo vem da SDD.

16. **Mecanismo técnico de invocação de subagentes**: via **Task tool do Cursor** (subagentes em background/paralelo quando possível), passando o modelo configurado em `workflow-config.yml` para aquele agente específico, e lendo o resultado retornado ao final da execução.

17. **Uso do OpenSpec na etapa Implementation**: o `orchestration_agent` chama **`/opsx-apply`** no início da etapa Implementation, e o próprio OpenSpec conduz a implementação task-by-task (chamando os subagents backend/frontend internamente conforme cada task).

18. **`/opsx-sync`**: o `planning_agent` chama `/opsx-sync` automaticamente **após o Judge aprovar a etapa**, garantindo que `openspec/specs/` reflita o que foi de fato implementado, antes do `/opsx:archive` final.

19. **`/opsx-explore`**: o `planning_agent` chama `/opsx-explore` **antes do `/opsx-propose`**, quando o ciclo afeta um projeto com código já existente (não é projeto novo do zero), para o OpenSpec mapear a base de código relevante antes de planejar.

### Escopo Estendido e Novo Agente Genérico

20. **Scope sem backend e sem frontend**: adicionadas 3 novas opções de scope — **`native`** (Linux/Windows), **`script`** e **`infrastructure`** — cobertas por um novo agente: **`generic_agent`**. Bloco final no `workflow-config.yml`:

```yaml
scope:
  backend: true
  frontend: false
  native: false
  script: false
  infrastructure: false
```

  - Validação: se `implementation.enabled: true`, ao menos um dos 5 scopes deve ser `true`; caso contrário o `/hero:start` bloqueia e pede correção.
  - O `generic_agent` só assume os scopes genéricos marcados (native, script, infrastructure); roda em paralelo/série (conforme a SDD) junto com `backend_agent`/`frontend_agent` quando esses também estiverem marcados.
  - Modelo inicial: **Claude Sonnet 5**. Adicionar linha completa (5 opções) na Tabela de Recomendação de Modelos, mesmo padrão dos demais agentes.

21. **QA para o `generic_agent`**: o `qa_agent.md` ganha uma seção de critérios adaptativos por tipo de scope (ex: para `infrastructure`, validar `terraform plan`/`docker build` sem erros; para `script`, validar execução sem erro + idempotência quando aplicável; para `native`, validar build da plataforma alvo). Mantém-se um único `qa_agent` genérico e inteligente, sem criar um QA agent separado por scope.

22. **Sobreposição QA Agent / Judge Agent**: removido "Architecture consistency" e "Implementation patterns" da lista de responsabilidades do `judge_agent.md`. O `judge_agent` passa a validar exclusivamente cobertura de requisitos da SDD vs. o que foi implementado; toda questão de qualidade/arquitetura fica exclusivamente com o `qa_agent`.

### Fallback de Modelos

23. **Cadeia de fallback (3 níveis)**: 1º tenta o modelo do agente configurado em `workflow-config.yml` → 2º tenta o bloco `fallback_model` global → 3º avisa o usuário e espera `/hero:continue` (usuário corrige a configuração do agente e retenta).

24. **Localização do `fallback_model`**: definido no topo do `workflow-config.yml` (mesmo nível de `agents:`), com `model`, `reasoning_effort`, `enable_fast_model` e `thinking`, pois pode variar por ciclo dependendo do orçamento/necessidade daquele ciclo específico. **Sempre que o `fallback_model` for ativado, o usuário deve ser avisado explicitamente**, mesmo que a execução continue automaticamente com esse fallback.

### Documentos de Research: Templates, Numeração e Índices

25. **Quem decide e registra documentos**: o `discover_agent` decide quantos/quais documentos criar com base no resultado do `grill-me`, cria os arquivos em `docs/` e já registra cada um no `documents.json` (name, path, purpose) automaticamente, sem perguntar ao usuário.

26. **Templates fixos por categoria**: cada categoria (PRD, ADR, UI, DEPLOY, TESTING) tem um template fixo com seções padronizadas em `assets/cursor/workflow-hero/templates/docs/`, e o `discover_agent` apenas copia e preenche as seções.

27. **Padrão de numeração**: numerar (prefixo `-001`, `-002`...) apenas documentos que podem se repetir dentro do mesmo projeto (PRD, ADR, UI). Documentos únicos por natureza (`DEPLOY.md`, `TESTING.md`, `AGENTS.md`) ficam sem número, sempre em singular e maiúsculo. `DEPLOY.md`/`TESTING.md` são **documentos vivos**: editados no lugar a cada ciclo que os afeta; histórico de mudanças fica no `context-log.md` e no git, não dentro do próprio arquivo.

28. **Numeração reinicia por ciclo, com prefixo de ciclo**: padrão final de nome de arquivo:

```
[CATEGORIA]-C[XX]-[seq]-[slug].md

Exemplos:
PRD-C04-001-checkout-flow.md
ADR-C04-001-database-choice.md
UI-C04-001-dashboard.md
```

  - O contador de ciclo (`C04`) vem do campo `workflow.cycle` (numérico) em `project.json`, incrementado pelo `orchestration_agent` a cada `/hero:new` bem-sucedido (mesmo que o ciclo não chegue a criar nenhum documento).
  - Esse mesmo número de ciclo é adicionado ao nome da pasta de arquivamento: **`C[XX]-yyyy-mm-dd-[slug]/`** (número do ciclo primeiro), ex: `C04-2026-06-21-upload-imgurl/`.
  - O `[slug]` do nome da pasta de arquivo vem de um slug gerado automaticamente a partir do campo `title` do `workflow-config.yml` do ciclo arquivado.

29. **Arquivos-índice centrais**: `docs/architecture/ADR.md` e `docs/product/PRD.md` funcionam como índice/sumário (tabela com número, título, ciclo, status) de todos os `ADR-CXX-NNN-*.md`/`PRD-CXX-NNN-*.md` gerados ao longo dos ciclos, atualizados automaticamente pelo `discover_agent` sempre que um novo documento é criado.

30. **`openspec/config.yaml` templatizado**: o `planning_agent` lê o `documents.json` e gera a lista de "Authoritative sources" dinamicamente a partir das entradas registradas ali (todos os documentos do ciclo atual), sem precisar listar nomes fixos no template.

31. **Motor de templates**: apenas substituição simples de placeholders `{{caminho.chave}}` (sem loops/condicionais). Tabelas como a de documentos no `AGENTS.md` são montadas pelo próprio agente de IA como texto final, não por um motor de template com `{{#loop}}`.

32. **Idioma dos artefatos**: sempre em inglês, para todos os projetos, independente do idioma em que o usuário conversa no chat ou do `localization.languages` do produto final. Exceção: `README.md`/`README_PT_BR.md` são documentação estática do próprio Hero (ferramenta), mantida pelos mantenedores, não são artefatos de ciclo — a regra de "sempre inglês" não se aplica a eles.

33. **Skill de grilling própria**: o Hero passa a distribuir sua própria skill de grilling dentro dos assets do CLI (`assets/cursor/skills/grilling/SKILL.md`), removendo a dependência do repositório externo do mattpocock citado originalmente.

### Configuração, Dependências entre Etapas e project.json

34. **Validação de dependências entre etapas**: o `orchestration_agent` valida no `/hero:start` (ao ler o `workflow-config.yml`): se detectar dependência quebrada (ex: `implementation.enabled=true` com `planning.enabled=false`), bloqueia e pede correção antes de continuar.

35. **Regra de aprovação do PRD quando Research está desabilitado**: a regra "não iniciar Implementation sem PRD aprovado" só vale se `research.enabled=true`. Quando desabilitada, mesmo assim o Hero exige que o campo `objective` do `workflow-config.yml` esteja bem descrito, e o `orchestration_agent` pede confirmação explícita do usuário sobre o escopo antes de liberar Implementation (substituto leve de aprovação de PRD).

36. **Etapa Configuration**: sempre executada implicitamente (corresponde aos passos do `/hero:new` e `/hero:start`), nunca configurável em `workflow-config.yml`, Human Approval sempre "N/A". Serve apenas de registro no `workflow.md`.

37. **Timeout aplicado entre iterações**: o `timeout_minutes` não interrompe uma execução em andamento; o `orchestration_agent` registra o horário de início da etapa e, **antes de iniciar cada nova iteração/loop de correção**, verifica se o tempo decorrido já excedeu o timeout — se sim, trata como esgotamento (mesmo fluxo de escalonamento do `max_iterations`), mesmo com iterações sobrando.

38. **Campos avançados do `project.json`** (stack, plataformas, idiomas, design de UI, domínio de deploy): ficam vazios/null na instalação (CLI) e são preenchidos pelo `orchestration_agent` durante o primeiro `/hero:new`, inferindo do repositório existente ou perguntando ao usuário quando não houver código.

39. **Importar `workflow-config.yml` do ciclo anterior**: em todo `/hero:new` com ciclo(s) anterior(es), **sempre** importa `workflow_config`, `fallback_model`, `stages` e `agents` (idioma de chat, modelos, iterações, aprovação) do ciclo anterior (deep-merge sobre o template); `title`, `objective` e `scope` sempre voltam para os valores padrão do template, pois são específicos de cada ciclo. Não perguntar — a importação é obrigatória.

40. **`/hero:new` com ciclo em andamento**: avisa que há um ciclo em andamento, mostra a etapa atual, e pergunta explicitamente se o usuário quer arquivar mesmo assim (perdendo o progresso não finalizado) ou cancelar o `/hero:new` e continuar o ciclo atual com `/hero:start`.

41. **Estimativa de tokens/custo**: heurística simples — o agente estima tokens contando/aproximando caracteres do que leu e escreveu (ex: ~4 caracteres por token) e multiplica pelo preço do modelo em `models/*.yml`.

### Métricas

42. **Estrutura do `metrics.md`**: tabela com uma linha por etapa (Configuration, Research, Planning, Implementation, QA, Judge, QA End-to-End), colunas: Tempo, Tokens (input/output/cache), Custo estimado, Modelo usado; linha de Total ao final.

43. **Múltiplos agentes na mesma etapa**: quebra em sub-linhas dentro da mesma etapa (uma linha por agente chamado), com uma linha de subtotal ao final da etapa.

44. **Métricas agregadas do projeto**: novo arquivo `.workflow-hero/metrics-summary.md` (fora de `cycles/`), atualizado pelo `orchestration_agent` ao final de cada ciclo, somando os totais do `metrics.md` arquivado à visão global do projeto — cobre o objetivo "estatísticas de consumo de token para cada etapa do projeto".

### Playwright e QA End-to-End

45. **Uso do Playwright**: exclusivamente pelo `end2end_qa_agent`, na etapa QA End-to-End, quando `stages.qa_end_to_end.use_playwright: true` (requer `scope.frontend: true`), para simular jornadas de usuário real no navegador.

46. **Seleção explícita**: `use_playwright` é um boolean em `qa_end_to_end` no `workflow-config.yml`. Default `false`. `true` só é válido com `scope.frontend: true`; caso contrário o Runtime bloqueia. Com `use_playwright: false`, o `end2end_qa_agent` usa chamadas HTTP diretas (curl/requests), mesmo se `scope.frontend` for `true`.

### `/hero:sync` e Ativação em Projetos Existentes

47. **`hero sync` removido do CLI**: mantido apenas `/hero:sync` (Runtime, com IA). O objetivo do comando é também **ativar o Hero em projetos já existentes**, criando o contexto necessário para executar o Hero — este uso foi **promovido de V2 para V1**, por ser pré-requisito natural de adoção em projetos reais. O item de V2 passa a significar algo mais avançado (ex: detecção de divergências, sync incremental contínuo).

48. **Escopo da análise do `/hero:sync`** em projeto com código já existente (nunca usou o Hero): aciona o `context_agent` para escanear a estrutura do repositório (linguagens, frameworks, pastas, dependências, padrões de código) e qualquer documentação pré-existente (README, docs/), e a partir disso infere e preenche `AGENTS.md`, `current-state.md` e os campos avançados do `project.json`; pontos ambíguos são perguntados ao usuário.

### Comandos, CLI e Assets

49. **Fonte da verdade para nomenclatura de comandos**: a tabela de "Comandos" já existente no documento (seção "Comandos") é a fonte da verdade. Confirmado: CLI usa `hero install --tools cursor`; Runtime usa `/hero:new` para novo ciclo (arquivo de asset correspondente: `hero-new.md`, alinhado ao comando).

50. **Lista de assets `commands/` corrigida**: um arquivo `.md` por comando Runtime, seguindo o padrão `hero-<comando>.md`. Lista completa (12 arquivos): `hero-new.md`, `hero-start.md`, `hero-approve.md`, `hero-reject.md`, `hero-cancel.md`, `hero-finish.md`, `hero-archive.md`, `hero-sync.md`, `hero-status.md`, `hero-help.md`, `hero-continue.md`, `hero-back.md`, mais os novos `hero-resume.md` (decisão #13).

51. **`hero status` e `hero help` também como comandos CLI**: adicionados como comandos CLI (sem IA, lendo direto `workflow.md`/`metrics.md` e imprimindo no terminal), úteis para quando o desenvolvedor não está numa sessão de chat.

52. **`hero variables`**: somente leitura na V1. Lista todas as variáveis de `hero.json`/`project.json`/`documents.json` formatadas no terminal; edição continua sendo feita direto nos arquivos `.json` ou pelos agentes durante os ciclos.

53. **`hero update-models`**: o CLI busca um arquivo de dados já estruturado (JSON/YAML) publicado no próprio repositório oficial do Hero no GitHub (atualizado manualmente pelos mantenedores sempre que os preços do Cursor mudam); nunca faz scraping/parsing de páginas HTML.

54. **`hero doctor`**: verificações estruturais e de versão — presença de todos os arquivos/pastas esperados (`.cursor/agents`, `.workflow-hero/config`, etc.), consistência entre `hero.json` (versão instalada) e a versão do binário, validação de sintaxe YAML/JSON dos arquivos de config, e se o projeto é um repositório git.

55. **`hero uninstall` preserva o OpenSpec**: mesma filosofia do Hero para `docs/`/`context/` — `openspec/` tem valor próprio mesmo sem o Hero. `hero uninstall` remove apenas `.cursor/agents/`, `.cursor/commands/hero-*.md`, `.cursor/skills/workflow-hero/`, `.cursor/skills/grilling/`, e `.workflow-hero/`.

56. **`hero upgrade` com templates customizados**: detecta arquivos modificados (hash/checksum diferente do que foi instalado originalmente) e **não sobrescreve**; apenas avisa o usuário quais arquivos ficaram desatualizados e onde ver o diff/nova versão para mesclar manualmente.

### Pré-requisitos e Concorrência

57. **Git como pré-requisito obrigatório**: `hero install` e `hero doctor` verificam se o projeto é um repositório git. Se `hero install` detectar que não é, **pergunta ao usuário** se deseja que o Hero rode `git init` automaticamente antes de continuar; se o usuário recusar, a instalação é abortada.

58. **Lock de sessão concorrente**: arquivo de lock simples (`.workflow-hero/cycles/current/.lock`) com timestamp/PID de sessão, escrito ao iniciar uma etapa; se outra sessão tentar rodar um comando Runtime e o lock existir (e não estiver expirado), avisa o usuário que há outra sessão ativa antes de proceder.

### Comandos Novos Introduzidos Nesta Sessão

| Comando | Descrição |
|---|---|
| `/hero:continue` | Concede mais N iterações (usuário informa quantas) quando `max_iterations` esgota; retoma de onde parou. |
| `/hero:back` | Retorna para a etapa de Planning quando o Judge identifica ambiguidade na SDD (não lacuna de implementação); reseta Implementation/QA/Judge para Waiting. |
| `/hero:resume [ciclo]` | Retoma um ciclo pausado (arquivado manualmente via `/hero:archive`), movendo-o de volta para `cycles/current/`. |

---
## Decisões da Sessão de Grilling — 2026-07-20 (parte 2)

### Sessões Limpas com Subagentes

59. **Escopo de "sessões limpas"**: aplica-se apenas aos **subagentes** (backend_agent, frontend_agent, generic_agent, qa_agent, judge_agent, end2end_qa_agent, context_agent), invocados via Task tool. Cada chamada roda em sessão nova/isolada, sem herdar o histórico de chat do orchestrator. O subagente recebe apenas **ponteiros para arquivos** (caminhos de `AGENTS.md`, `current-state.md`, da SDD/tasks relevantes) em vez de conteúdo completo colado no prompt. O orchestrator absorve de volta apenas o **resultado final estruturado** do subagente (ex: seção "Output Format" de cada `*_agent.md`), não seu raciocínio intermediário. A sessão principal do `orchestration_agent` permanece contínua ao longo de todo o ciclo — apenas os subagentes são "descartáveis" por chamada.

### Padrões de Código e Testes do Hero CLI (Go)

60. **Princípios de código e teste** (aplicam-se a todo o código Go do repositório `workflow-hero`, não aos projetos-usuário):
    - Prefer **clarity over cleverness**.
    - Test **behavior**, not implementation details.
    - Favor **real dependencies** over excessive mocking.
    - Keep tests **deterministic and fast**.
    - Avoid over-engineered test frameworks.

61. **Estratégia de testes do Hero CLI**: testes unitários (`go test`) para a lógica de negócio de cada `internal/*` (install, upgrade, doctor, etc.) + golden-file tests para validação de renderização de templates (garante que `{{placeholder}}` é substituído corretamente) + testes de integração leves rodando o binário compilado contra um diretório temporário para `install`/`upgrade`/`uninstall`/`doctor`.

62. **Organização de arquivos de teste**: colocados junto ao código, mesmo pacote (`internal/install/service_test.go` ao lado de `service.go`), usando `t.TempDir()` e o filesystem real do SO como dependência real (sem mocks de filesystem); `embed.FS` de assets também usado real (não mockado) nos testes de integração.

### Plataformas, Build e Release

63. **Plataformas alvo (V1)**: Linux e macOS, ambos em `amd64` e `arm64` (4 combinações via cross-compilation do Go). Windows fica para V2.

64. **Processo de build/release (V1)**: manual, sem CI/CD — um script `.sh` próprio no repositório (`scripts/release.sh`) gera as 4 combinações a partir de um único comando, usando a tag git atual como versão.

65. **Versionamento**: SemVer (`vMAJOR.MINOR.PATCH`), injetado no binário via `-ldflags "-X main.version=..."` a partir da tag git no momento do build. `assets.version` (em `hero.json`) é sempre igual a `cli.version`, pois os assets viajam embarcados no mesmo binário via `embed.FS`.

66. **Checksums**: o script de release gera um arquivo `checksums.txt` (SHA256) para os 4 binários, publicado na mesma release do GitHub. Verificação é manual pelo usuário (`sha256sum -c`); sem assinatura GPG na V1.

### UX de Terminal (CLI)

67. **Estilo visual**: cores + ícones semânticos consistentes — verde/✓ sucesso, amarelo/⚠ aviso, vermelho/✗ erro, azul/→ progresso. Detecta suporte a cor do terminal (TTY / variável `NO_COLOR`) e degrada para texto puro quando não suportado. A mesma convenção de ícones/semântica é usada também pelos agentes no **Runtime** (chat do Cursor), para consistência visual em toda a experiência do Hero — por exemplo nos resumos de fechamento de etapa e nos avisos de fallback de modelo.

68. **Saída de comandos de leitura** (`hero status`, `hero variables`, `hero doctor`): tabela legível por humanos por padrão, com flag `--json` opcional para saída estruturada consumida por scripts/CI.

69. **Prompts interativos**: usar uma lib Go estilo survey (ex: `AlecAivazis/survey` ou `charmbracelet/huh`), com navegação por setas para escolhas múltiplas (ex: confirmação de `git init`) e validação inline de campos obrigatórios. Comandos com prompts interativos (como `hero install`) também aceitam flags equivalentes (`--name`, `--summary`, `--yes`) para pular os prompts; se todas as flags necessárias forem fornecidas, o comando roda sem interatividade — caso contrário, cai para o prompt interativo apenas nos campos faltantes.

70. **Mensagens de erro**: estrutura consistente — ícone/cor de erro + descrição clara do problema + sugestão de correção (quando aplicável) + código de saída não-zero. Stack traces de erros inesperados (panics) só aparecem com uma flag `--verbose`/`--debug`.

71. **`workflow_config.user_preferred_language`**: seção `workflow_config` no `workflow-config.yml` (antes de `scope`) com default `EN`. Todos os agentes falam com o usuário no chat nessa língua, salvo pedido explícito do usuário em contrário. Artefatos de ciclo continuam em inglês. `fallback_model` fica depois de `agents` e antes de `workflow_rules`. Em `/hero:new`, `workflow_config` é importado do ciclo anterior junto com `fallback_model` / `stages` / `agents`.

