# V2 — Multi-Harness Implementation

## Objetivo

Evoluir o Hero para suportar **múltiplos AI Harnesses através exclusivamente da Hero TUI**, permitindo que um mesmo Ciclo utilize diferentes harnesses e diferentes modelos conforme a configuração de cada agente.

O Cursor continuará sendo suportado como está atualmente, inclusive através da integração com a Cursor IDE. A compatibilidade com a Cursor IDE deve ser preservada, mas o modo multi-harness será responsabilidade exclusiva da Hero TUI.

A implementação deve manter a arquitetura atual do Hero simples, utilizando o `HarnessAdapter` existente como ponto de abstração entre o Hero e cada harness.

---

## Contexto atual

O Hero atualmente possui:

- Hero CLI em Go;
- Hero TUI em Bubble Tea;
- `cycle.Service` para operações do Ciclo;
- `engine.Engine` para a máquina de estados determinística;
- SQLite como fonte de verdade operacional;
- `HarnessAdapter` como abstração para execução de harness;
- `CursorAdapter` como implementação atual;
- execução via Cursor Agent CLI na TUI;
- Runtime do Cursor IDE com `orchestration_agent` e subagents;
- `workflow-config.yml` para configuração dos stages e agentes;
- `hero.json` para configurações persistentes do Hero, incluindo o modelo default do harness.

O `HarnessAdapter` existente já possui conceitos como disponibilidade do harness, execução/streaming, cancelamento/status e listagem de modelos.

A V2 deve aproveitar essa arquitetura existente, evitando introduzir novas camadas de orquestração desnecessárias.

---

# Objetivos da V2

## 1. Suporte a múltiplos Harnesses

O Hero deve permitir que diferentes harnesses sejam utilizados pela Hero TUI.

Inicialmente serão suportados:

- Cursor;
- OpenCode.

A arquitetura deve permitir a adição futura de outros harnesses sem alterações relevantes na lógica de Ciclo, Stage ou Agent.

Estrutura esperada:

```text
internal/harness/
    HarnessAdapter

internal/adapters/
    cursor/
        Adapter
    opencode/
        Adapter
```

O `HarnessAdapter` deve continuar sendo a fronteira entre o Hero e os harnesses.

Não criar uma nova camada de `ExecutionManager`, `AgentOrchestrator` ou equivalente apenas para suportar multi-harness.

---

# 2. Hero TUI como orquestrador Multi-Harness

A Hero TUI será o único ponto responsável pela execução multi-harness.

O fluxo esperado será:

```text
Hero TUI
    |
    v
Cycle / Stage
    |
    v
Agent configuration
    |
    +-- harness
    +-- model
    |
    v
HarnessAdapter
    |
    +-- CursorAdapter
    |
    +-- OpenCodeAdapter
    |
    +-- futuros adapters
```

A TUI deve ser capaz de executar diferentes agentes utilizando diferentes combinações de:

```text
Agent → Harness → Model
```

Exemplo:

```text
planning_agent
    → Cursor
    → composer-2.5

frontend_agent
    → OpenCode
    → claude-sonnet

qa_agent
    → OpenCode
    → claude-opus
```

---

# 3. Preservar o Cursor IDE Runtime

A integração atual com a Cursor IDE deve permanecer funcionando.

O Cursor IDE continuará operando como atualmente:

```text
Cursor IDE
    |
    v
orchestration_agent
    |
    v
Cursor Task / Cursor Runtime
    |
    v
Hero CLI
```

Esse modo não precisa se tornar multi-harness.

A V2 deve considerar a Cursor IDE como um modo de compatibilidade existente e preservar seu comportamento atual.

Em particular:

- não substituir o `orchestration_agent` do Cursor IDE;
- não obrigar a Cursor IDE a conhecer OpenCode;
- não modificar o Runtime do Cursor para implementar multi-harness;
- não quebrar os ciclos existentes executados através da Cursor IDE.

O multi-harness será uma capacidade da Hero TUI.

---

# 4. Seleção e habilitação de Harnesses

A Hero TUI deve permitir ao usuário visualizar e selecionar os harnesses disponíveis para execução.

O Hero deve detectar a disponibilidade dos harnesses utilizando a abstração existente do `HarnessAdapter`.

Inicialmente:

```text
Cursor
OpenCode
```

A experiência de configuração deve permitir distinguir:

- harness instalado/disponível;
- harness indisponível;
- harness habilitado para uso pelo Hero.

A solução de UX deve ser refinada durante o Research/Grill-me.

---

# 5. Seleção do modelo Default

O comando `/hero-model` deve evoluir para considerar o harness de origem do modelo.

Atualmente o Hero possui o conceito de modelo default associado ao harness em:

```text
hero.json
  harnesses.<tool>.model
```

Esse conceito deve ser preservado e expandido para múltiplos harnesses.

A seleção deverá apresentar ao usuário informações suficientes para identificar:

```text
Modelo                     Harness
-----------------------------------------
composer-2.5               Cursor
claude-sonnet-4.x          OpenCode
claude-opus-4.x            OpenCode
...
```

O objetivo é que a seleção de um modelo nunca seja ambígua quando diferentes harnesses puderem fornecer modelos com nomes semelhantes.

A solução final de UX e persistência deve ser refinada durante o Research.

---

# 6. Configuração de Harness + Model por Agent

O `workflow-config.yml` deve ser evoluído para que cada agente possa declarar explicitamente o harness e o modelo que deverá utilizar na Hero TUI.

Atualmente:

```yaml
agents:
  backend_agent:
    model: cursor-grok-4.5
```

A V2 deverá suportar conceitualmente:

```yaml
agents:
  backend_agent:
    harness: cursor
    model: cursor-grok-4.5

  frontend_agent:
    harness: opencode
    model: claude-sonnet-4.x

  qa_agent:
    harness: opencode
    model: claude-opus-4.x
```

A configuração deve representar explicitamente:

```text
Agent
 ├── Harness
 └── Model
```

O modelo configurado para um Agent deverá ser resolvido pelo Harness indicado.

---

# 7. Compatibilidade do workflow-config.yml

A evolução deve considerar os arquivos de configuração existentes.

É necessário definir durante o Research:

- se `harness` será obrigatório ou opcional;
- comportamento para configurações antigas sem `harness`;
- comportamento quando o harness configurado não estiver disponível;
- comportamento quando o modelo configurado não estiver disponível;
- como o `fallback_model` atual deverá funcionar em um ambiente multi-harness;
- se o fallback também deverá possuir `harness`;
- como validar combinações inválidas de harness + model;
- como tratar agentes que ainda utilizam configurações legadas.

A compatibilidade com ciclos/configurações existentes deve ser preservada sempre que possível.

---

# 8. Fallback

O `workflow-config.yml` atualmente possui:

```yaml
fallback_model:
  model: cursor-grok-4.5
  reasoning_effort: high
  enable_fast_model: false
  thinking: na
```

A V2 deve revisar esse conceito para o cenário multi-harness.

O Research deve determinar uma estratégia clara para:

```text
Agent
  ↓
Harness configurado
  ↓
Model configurado
  ↓
Modelo indisponível?
  ↓
Fallback
```

Deve ficar definido se o fallback:

- pertence a um harness específico;
- pode escolher outro harness;
- ou permanece restrito ao harness configurado.

O comportamento deve ser explícito e nunca trocar silenciosamente de harness ou modelo.

---

# 9. Execução do Ciclo

Depois da configuração, a seleção de harness deve ser transparente para o restante do workflow.

O usuário configura:

```text
Agent → Harness → Model
```

e o Hero deve executar automaticamente o Ciclo respeitando essa configuração.

Exemplo:

```text
Research
    discover_agent
        → OpenCode
        → Claude Opus

Planning
    planning_agent
        → Cursor
        → Composer

Implementation
    backend_agent
        → Cursor
        → Cursor Grok

    frontend_agent
        → OpenCode
        → Claude Sonnet

QA
    qa_agent
        → OpenCode
        → Claude Sonnet

Judge
    judge_agent
        → OpenCode
        → Claude Opus
```

O usuário não deverá precisar selecionar manualmente o harness a cada execução.

A configuração do Ciclo determina o executor.

---

# 10. Sessões

O Hero atualmente persiste/gerencia referências de sessão do harness por Stage.

A V2 deve garantir que uma sessão seja corretamente associada ao harness que a criou.

Uma sessão Cursor não pode ser tratada como uma sessão OpenCode.

Deve ser possível identificar, no mínimo:

```text
stage
harness
model
session_id
status
usage
```

A solução de persistência deve aproveitar o modelo existente sempre que possível.

Não criar uma nova estrutura de persistência se a estrutura atual puder ser evoluída de forma simples.

---

# 11. TUI

A TUI já apresenta informações como:

```text
[HARN - composer-2.5] · cursor
```

A V2 deve preservar e expandir esse conceito.

O usuário deverá conseguir identificar claramente:

```text
Agent
Model
Harness
```

durante uma execução.

Exemplo:

```text
[HARN - claude-sonnet] · opencode
```

ou:

```text
Build · claude-sonnet · opencode
```

A TUI deve continuar funcionando de forma independente da Cursor IDE.

---

# 12. Novos comandos da TUI

O conjunto atual de comandos deve ser preservado.

Os comandos relacionados à configuração de harness/model deverão ser refinados durante o Research.

Atualmente existem, entre outros:

```text
/hero-model
/hero-start
/hero-new
/hero-sync
/hero-status
/hero-approve
/hero-reject
/hero-continue
/hero-back
/hero-cancel
/hero-finish
/hero-archive
/hero-resume
/hero-cycles
/hero-todos
```

A V2 deve avaliar a necessidade de adicionar um comando específico para:

- listar harnesses;
- habilitar/desabilitar harnesses;
- verificar disponibilidade;
- configurar harness default.

Não assumir antecipadamente o nome ou UX final do comando. Isso deve ser refinado durante o Research.

---

# 13. OpenCode Adapter

O primeiro novo harness a ser implementado será o OpenCode.

O Research deverá investigar detalhadamente:

- CLI;
- modo não-interativo/headless;
- sessões;
- continuidade de sessões;
- streaming;
- formato de eventos/output;
- cancelamento;
- identificação de modelos;
- seleção de provider/model;
- códigos de erro;
- disponibilidade;
- execução no workspace do projeto;
- captura de usage/token/cost quando disponível;
- funcionamento do servidor/API do OpenCode, caso seja útil;
- diferenças em relação ao Cursor Agent CLI.

O objetivo é implementar o `OpenCodeAdapter` utilizando a mesma abstração já utilizada pelo `CursorAdapter`.

---

# 14. Modelo de abstração

O objetivo não é abstrair todas as diferenças entre os harnesses.

O `HarnessAdapter` deve fornecer somente o contrato necessário para que o Hero TUI consiga:

- detectar disponibilidade;
- selecionar/listar modelos;
- iniciar execução;
- receber streaming;
- acompanhar status;
- cancelar execução;
- continuar/resumir sessões quando suportado;
- obter informações de usage quando suportado.

Características específicas de cada harness devem permanecer encapsuladas dentro do respectivo adapter.

---

# 15. Regras importantes de arquitetura

A V2 deve preservar os seguintes princípios:

1. `engine.Engine` continua sendo determinístico.
2. `engine.Engine` não executa LLM.
3. SQLite continua sendo a fonte de verdade operacional.
4. `HarnessAdapter` continua sendo a abstração de execução.
5. Cada harness possui seu próprio adapter.
6. A Hero TUI é responsável pela orquestração multi-harness.
7. A Cursor IDE permanece como modo de compatibilidade Cursor-only.
8. O Hero não deve depender de APIs específicas do Cursor para implementar multi-harness.
9. A lógica específica do OpenCode deve permanecer no `OpenCodeAdapter`.
10. Adicionar novos harnesses no futuro não deve exigir alterações na lógica dos stages.
11. O usuário deve sempre conseguir identificar qual harness e modelo estão sendo utilizados.
12. Trocas de harness/modelo não devem ocorrer silenciosamente.

---

# 16. Fora do escopo inicial

Não implementar nesta V2, salvo se o Research demonstrar necessidade:

- execução simultânea de múltiplos harnesses dentro da mesma execução de um Agent;
- múltiplos Agents concorrentes utilizando diferentes harnesses na mesma Stage;
- distributed execution;
- daemon/RPC;
- event bus distribuído;
- alteração do Runtime da Cursor IDE para suportar multi-harness;
- substituição do `engine.Engine`;
- criação de uma nova camada de orquestração além das abstrações existentes;
- suporte inicial a muitos harnesses além do Cursor e OpenCode.

A prioridade é implementar uma solução simples e sólida para:

```text
1 Cycle
    ↓
vários Stages
    ↓
cada Agent
    ↓
Harness + Model configurados
```

---

# 17. Instalação e gerenciamento de Harnesses

A V2 deve substituir completamente o mecanismo atual de instalação baseado em:

```bash
hero install --tools cursor
```

O conceito de `--tools <harnesses>` deve ser removido da interface do Hero. Não deve existir uma nova variação como --tools cursor,opencode.

A seleção dos Harnesses que o Hero deverá utilizar deve ser feita de forma interativa durante a instalação e, posteriormente, poderá ser modificada pelo usuário através da Hero TUI.

---

## 17.1. Instalação interativa

O comando principal de instalação deverá ser:

```bash
hero install
```
Durante a instalação, o Hero deverá detectar/apresentar os Harnesses suportados e permitir que o usuário selecione quais deseja utilizar.

Exemplo conceitual:

```bash
Hero Installation

Select the AI Harnesses you want to use:

  [x] Cursor
  [x] OpenCode
  [ ] ...

        Continue

```

A experiência e o mecanismo exatos de seleção deverão ser definidos durante o Research/Grill-me.

O objetivo é que, ao final da instalação, o Hero saiba explicitamente quais Harnesses foram selecionados pelo usuário.

---

## 17.2. Instalação do Hero Core versus Harness

A instalação deve separar conceitualmente:

```text
Hero Core
    +
Harness integrations
```
Os componentes comuns do Hero devem ser instalados uma única vez, independentemente dos Harnesses selecionados.

Exemplos de componentes do Hero Core:

- Hero CLI;
- Hero TUI;
- workflow infrastructure;
- Cycle infrastructure;
- SQLite;
- workflow configuration;
- comandos e lógica próprios do Hero.

Já os componentes específicos de cada Harness devem ser instalados/provisionados somente para os Harnesses selecionados.

---

## 17.3. Projeção dos artefatos para cada Harness

Os agentes, skills, comandos, regras e demais artefatos necessários para a execução dos workflows devem possuir uma fonte de verdade única no Hero.

Entretanto, quando um Harness exige uma estrutura própria de arquivos, esses artefatos devem ser provisionados na estrutura esperada pelo respectivo Harness.

Conceitualmente:

```text
                         HERO
                    Source of Truth
                         │
             ┌───────────┴───────────┐
             │                       │
       Cursor selected         OpenCode selected
             │                       │
             ▼                       ▼
    Cursor Projection         OpenCode Projection
             │                       │
             ▼                       ▼
        .cursor/...             .opencode/...
```

Não devem existir versões independentes mantidas manualmente para cada Harness.

A alteração de um agente, skill ou outro artefato pertencente ao Hero deve possuir uma única fonte de verdade, sendo responsabilidade da integração/provisionamento de cada Harness gerar ou manter sua representação específica.

---

## 17.4. Exceção: Cursor IDE

A integração com a Cursor IDE deve ser preservada integralmente.

Mesmo que a Hero TUI seja o principal orquestrador multi-harness, a Cursor IDE continuará dependendo da estrutura própria do Cursor para funcionar.

Portanto, quando o Cursor estiver selecionado, a instalação deve manter os artefatos necessários dentro da estrutura esperada pela Cursor IDE, incluindo, conforme aplicável:

- skills;
- commands;
- agents;
- rules;
demais arquivos específicos utilizados atualmente pelo Runtime da Cursor IDE.

Conceitualmente:

```text
Cursor selected
    │
    ├── Hero TUI integration
    │       └── CursorAdapter
    │
    └── Cursor IDE integration
            ├── .cursor/agents
            ├── .cursor/skills
            ├── .cursor/commands
            ├── .cursor/rules
            └── demais artefatos necessários
```

A V2 não deve remover ou substituir esses arquivos simplesmente porque o Hero TUI passou a utilizar o `CursorAdapter`.

A compatibilidade com a Cursor IDE é um requisito explícito da V2.

---

## 17.5. Gerenciamento posterior através da TUI

A seleção realizada durante hero install não deve ser definitiva.

O usuário deverá conseguir, posteriormente, modificar os Harnesses habilitados através da Hero TUI.

A TUI deverá permitir, no mínimo:

- visualizar os Harnesses instalados/disponíveis;
- visualizar quais Harnesses estão habilitados para o Hero;
- habilitar um Harness;
- desabilitar um Harness;
- verificar a disponibilidade de um Harness;
- provisionar os artefatos necessários quando um Harness for habilitado;
- atualizar/remover a integração de um Harness quando ele for desabilitado, conforme as regras definidas durante o Research.

A UX e os slash commands necessários para essa funcionalidade deverão ser definidos durante o Research/Grill-me.

Não assumir antecipadamente o nome dos comandos.

---

## 17.6. Estado dos Harnesses

O Hero deve distinguir claramente entre:

```text
Installed
Available
Enabled
Disabled
```
Um Harness não deve ser considerado utilizável apenas porque existe uma configuração para ele.

Antes de executar um Agent configurado para determinado Harness, o Hero deve conseguir determinar se a integração está efetivamente disponível.

Exemplo:
```text
backend_agent
    harness: opencode
```
Se o OpenCode não estiver instalado ou habilitado:

```text
ERROR:
Harness 'opencode' is configured for backend_agent
but is not available/enabled.
```
O Hero não deve trocar silenciosamente para outro Harness.

---

## 17.7. Alteração dos Harnesses após a instalação

O usuário pode instalar inicialmente:

`Cursor`

e posteriormente decidir utilizar:

`Cursor + OpenCode`

Nesse caso, não deverá ser necessário executar novamente uma instalação completa do Hero.

A TUI deverá permitir habilitar o OpenCode e realizar o provisionamento necessário.

Da mesma forma, o usuário poderá posteriormente desabilitar um Harness que não deseja mais utilizar.

A estratégia exata para remoção, preservação ou atualização dos artefatos específicos do Harness deverá ser definida durante o Research.

---

## 17.8. Relação com o workflow-config.yml

A seleção dos Harnesses durante a instalação não substitui a configuração de `harness + model` dos Agents no `workflow-config.yml`.

São conceitos diferentes:

```text
Installation / TUI
    ↓
Quais Harnesses o Hero pode utilizar?

workflow-config.yml
    ↓
Qual Harness + Model cada Agent deve utilizar?
```
Exemplo:

```YAML
agents:
  backend_agent:
    harness: cursor
    model: cursor-grok-4.5

  frontend_agent:
    harness: opencode
    model: claude-sonnet-4.x
```
Se `opencode` não estiver habilitado/instalado, o Hero deverá informar explicitamente a inconsistência.

---

## 17.9. Remoção do --tools

O parâmetro:

```bash
hero install --tools ...
```

deve ser removido como mecanismo de configuração de Harness.

Não criar uma nova sintaxe equivalente baseada em lista de Harnesses como substituição direta.

A experiência desejada é:

```text
hero install
      │
      ▼
seleção interativa dos Harnesses
      │
      ▼
provisionamento das integrações
```

Isso reduz a quantidade de opções de linha de comando que o usuário precisa conhecer e mantém a configuração de Harness centralizada na experiência do Hero.

A compatibilidade com scripts existentes que utilizam --tools deverá ser avaliada durante o Research, incluindo se haverá uma estratégia de migração ou se o comando será removido diretamente.

---

## 17.10. Princípios arquiteturais

A implementação deve respeitar os seguintes princípios:

1. O Hero Core não deve ser duplicado por Harness.
2. A fonte de verdade dos agentes, skills e demais artefatos do Hero deve ser única.
3. Cada Harness pode possuir uma projeção/representação específica dos artefatos.
4. A estrutura específica do Cursor deve ser preservada para manter a Cursor IDE funcionando.
5. A Hero TUI deve ser capaz de gerenciar os Harnesses após a instalação.
6. O hero install deve permitir a seleção inicial dos Harnesses.
7. --tools não deve permanecer como mecanismo de seleção de Harness.
8. A seleção de Harness não deve determinar automaticamente o Harness de cada Agent; essa responsabilidade pertence à configuração do workflow.
9. Um Harness indisponível ou desabilitado não deve ser substituído silenciosamente por outro.
10. A instalação de um novo Harness não deve exigir duplicação da lógica de workflow, stages ou engine.
11. A adição futura de novos Harnesses deve seguir o mesmo modelo utilizado para Cursor e OpenCode.

### Resultado esperado

A experiência final deverá ser conceitualmente:

```text
hero install
      │
      ▼
Select Harnesses
      │
      ├── Cursor
      ├── OpenCode
      └── ...
      │
      ▼
Install Hero Core
      │
      ├── Cursor integration
      │      ├── TUI Adapter
      │      └── Cursor IDE artifacts
      │
      └── OpenCode integration
             ├── TUI Adapter
             └── OpenCode artifacts
```

Posteriormente:

```text
Hero TUI
    │
    ▼
Manage Harnesses
    │
    ├── Enable Cursor
    ├── Disable Cursor
    ├── Enable OpenCode
    ├── Disable OpenCode
    └── ...
```

O objetivo é que o usuário não precise conhecer a estrutura interna de instalação de cada Harness. Ele simplesmente informa ao Hero quais Harnesses deseja utilizar, e o Hero se encarrega de provisionar e manter as integrações necessárias.

A Cursor IDE continua sendo uma exceção explícita: quando o Cursor estiver habilitado, os agents, skills, commands, rules e demais artefatos necessários devem continuar presentes na estrutura do Cursor para preservar integralmente a compatibilidade existente.

---

# 18. Critérios de sucesso

A V2 será considerada concluída quando for possível:

1. Instalar/configurar o Hero com Cursor e OpenCode.
2. Detectar ambos os harnesses através da TUI.
3. Selecionar modelos indicando claramente seu harness.
4. Configurar `harness + model` para cada Agent no `workflow-config.yml`.
5. Iniciar um Ciclo pela TUI.
6. Executar diferentes Stages utilizando diferentes harnesses.
7. Receber streaming das execuções.
8. Persistir corretamente as sessões.
9. Exibir harness + modelo na TUI.
10. Registrar usage/custos corretamente quando disponíveis.
11. Tratar indisponibilidade de harness/modelo de maneira explícita.
12. Continuar executando ciclos antigos/configurações legadas de forma compatível.
13. Manter a Cursor IDE funcionando sem alteração de comportamento.
14. Adicionar um novo Harness futuramente apenas implementando seu Adapter e as integrações necessárias, sem modificar a máquina de estados do Hero.

---

# Resultado esperado

Ao final da V2, o Hero deverá deixar de ser conceitualmente:

```text
Hero TUI → Cursor Agent
```

e passar a ser:

```text
Hero TUI
    ↓
Agent
    ↓
Harness + Model
    ↓
Execution
```

com múltiplas implementações:

```text
                 Hero TUI
                    │
              HarnessAdapter
                    │
        ┌───────────┴───────────┐
        │                       │
   CursorAdapter          OpenCodeAdapter
        │                       │
   Cursor Agent              OpenCode
        │                       │
     Models                  Models
```

O objetivo principal da V2 não é criar uma arquitetura mais complexa, mas **habilitar a arquitetura multi-harness utilizando ao máximo as abstrações que o Hero já possui**, mantendo a TUI como o ponto central de orquestração e preservando integralmente a compatibilidade existente com a Cursor IDE.
