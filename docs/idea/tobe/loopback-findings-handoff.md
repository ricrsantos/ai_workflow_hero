# Handoff de findings: QA, Judge, Browser UI, E2E → Implementation

> Status: proposta de ideia, não normativa.
> Local: `docs/idea/tobe/` (fora do Discover do ciclo corrente).
>
> Direção escolhida: estado operacional dos findings no `hero.db` (emenda ao
> ADR-075 / ADR-013); agentes só emitem JSON; o scheduler Go é o único a
> mudar status; `/hero-add-todo` estaciona findings na escalação humana e deixa o
> ciclo avançar; Status / `hero status` / `/hero-status` e `/hero-todos`
> mostram o vai-e-vem e o que foi enviado aos ToDos.
>
> Antes da implementação, esta proposta deve originar um novo ciclo com PRD,
> especificação de UI, ADR e especificações OpenSpec. Artefactos de ciclo
> permanecem em inglês.

## 1. Contexto e falha observada (C14)

O ciclo C14 (`tui-multimodal-images`) parou na Implementation Running (2/4)
depois de um loop-back da QA. O `generic_agent` corrigiu as regressões e
devolveu `status: complete` com gates e testes verdes, mas o completion gate
da TUI recusou o relatório:

`one or more stage-agent reports are invalid or missing required gate fields`

A causa real não era campo JSON em falta. O `openspec/changes/tui-multimodal-images/tasks.md`
já estava 38/38 `[x]`. A wave pós-loop-back era verification-only: assignment
vazia. O agente listou 8 IDs antigos (`task-02.2-validate`, …) em
`tasks_completed`. O validador exige que `tasks_completed ∪ tasks_remaining`
seja exactamente a assignment da wave. IDs não atribuídos falham fechado.

O mesmo padrão já tinha ocorrido no C13 (wave vazia + ID não atribuído).
`qa-gaps.md` / `judge-gaps.md` existiram como prosa em ciclos anteriores e
**não** entram no scheduler.

## 2. O que o produto faz hoje

### 2.1 Máquina de estados (SQLite)

`.workflow-hero/hero.db` (schema v10) é a fonte de verdade operacional
(ADR-013). Tabelas relevantes:

- `cycles` — ciclo ativo, `openspec_change` (só o slug), snapshot YAML.
- `stages` — Waiting / Running / Completed / Failed / Escalated / …; iterações.
- `events` — append-only, incluindo `loop_back` com `{from, to, reason}`.
- `metrics` — tokens/custo/duração.
- `conversation` — auditoria append-only de assignment/resultado da wave
  (`task_ids`). Não é checklist viva.
- `artifacts` — metadados de ficheiros para o ecrã Artifacts.

No loop-back, `Engine.LoopBackToImplementation` grava o `--reason` em
`stages.summary` da Implementation e emite o evento. A TUI injeta esse texto
no prompt como “Orchestrator assignment (loop-back)”.

Não há tabela de findings, IDs `find-*`, status open/done/reopened, nem
checkbox operacional fora do OpenSpec.

### 2.2 Assignment de Implementation (ADR-075)

A TUI lê o `tasks.md` ligado, parte só os `[ ]` por owner
(`[agent:backend_agent|frontend_agent|generic_agent]`), e exige prefixo
`task-`. Agentes nunca escrevem checkbox. Se Implementation recomeça sem
pendentes, pode correr uma única wave de verificação; relatório `complete`
nessa wave **tem** de usar arrays vazios.

### 2.3 Quatro origens de loop-back

Todas já podem reabrir Implementation (`hero stage loop-back --from …`):

- QA — `failures[]` com `agent` / `file` / `issue`.
- Judge — `implementation_gaps[]`; ambiguidade de SDD é outro caminho
  (`/hero-back` vs `/hero-approve`), não loop-back de código.
- Browser UI Validation — `failure_class` frontend|backend; Health antes de
  Visual; PNG de referência em falta é warning, não falha.
- QA End-to-End — falhas de jornada (Playwright ou HTTP) com agente
  responsável.

Nenhuma destas origens alimenta o scheduler. O Judge ainda está instruído a
escrever um artefacto de gaps e a chamar `loop-back` ele próprio — isso
contradiz o handoff TUI (agente emite JSON e pára).

### 2.4 ToDos hoje

`/hero-todos` (ADR-028) é **só leitura** da secção `## Pending Features` (e
outras `## Pending …`) em `context/current-state.md`. Não cria itens, não
está ligado a findings, não continua o ciclo. `/hero-sync` (ADR-029) é quem
funde pendências vindas de docs de produto/arquitectura.

Escalação humana hoje: `Escalated` + `/hero-continue [N]`. Não há “aceitar
esta pendência para mais tarde e seguir”.

### 2.5 Visibilidade hoje

- Ecrã Status / `hero status` / `/hero-status` — tabela de estágios
  (nome, status, iteração, aprovação). Sem findings, sem histórico de
  loop-back, sem ToDos do ciclo.
- Ecrã Events — eventos crus, incluindo `loop_back`, sem narrativa de
  findings.
- `/hero-todos` — pendências long-lived do repo, não o board do ciclo.

## 3. Decisões

1. Findings são estado **Hero-exclusivo** → `hero.db`, não `qa-gaps.md`.
   Um MD, se existir, é projeção só de leitura (Artifacts / prompt), nunca
   fonte de verdade nem escrita por agentes.
2. `tasks.md` do OpenSpec continua o checklist de **planeamento**. Não se
   desmarcam IDs `task-*` no loop-back. Finding é regressão/gap com ciclo de
   vida próprio (`find-qa-1`, `find-judge-2`, `find-bui-1`, `find-e2e-1`).
3. Agentes de QA / Judge / Browser UI / E2E / Implementation **só emitem
   JSON**. Não marcam checkbox, não escrevem gaps MD, não chamam
   `hero stage loop-back`. A TUI/CLI persiste findings, fecha `--failed` e
   faz loop-back. Close failed sem pelo menos um finding aberto é recusado.
4. Scheduler Go é o único a mudar status de finding (`open` → `done` →
   `reopened`, ou `deferred_todo`).
5. Assignment da Implementation = `[ ]` OpenSpec **união** findings
   `open`/`reopened`, particionada pelo owner. Relatório só pode citar IDs
   atribuídos. Prefixos válidos: `task-` e `find-`.
6. Segunda ronda da mesma falha **reabre o mesmo ID** e acrescenta contexto
   (round, estágio, issue). Não se clona `find-qa-3.1` para a mesma falha.
7. Judge: gap de implementação → finding + loop-back. Ambiguidade de SDD →
   `/hero-back` ou `/hero-approve`. Não misturar.
8. Browser UI: owner a partir de `failure_class`; Visual → frontend; PNG
   em falta continua warning. Relatórios `browser-ui/*.md` e screenshots
   ficam evidência em disco; o finding aponta o path.
9. Wave com zero OpenSpec pendente e zero findings abertos: assignment
   vazia; `complete` só com `tasks_completed: []` e `tasks_remaining: []`.
   Mensagem de recusa deve citar a causa real (ID não atribuído, assignment
   omitida, etc.), não um genérico “missing gate fields”.
10. Escalação humana no loop ganha `/hero-add-todo`: estacionar finding(s) nos
    ToDos do projeto e permitir o ciclo avançar. `/hero-todos` permanece a
    listagem.
11. Status, `hero status`, `/hero-status` e o ecrã Status da TUI mostram o
    fluxo de vai-e-vem e o que foi enviado aos ToDos. `/hero-todos` inclui
    os itens deferidos deste ciclo. Não se cria um oitavo item de navbar.

## 4. Modelo de findings (`hero.db`)

Campos mínimos por finding:

- `id` estável (`find-qa-1`, …)
- `cycle_id`
- `source_stage` (`qa` | `judge` | `browser_ui_validation` | `qa_end_to_end`)
- `owner` (`backend_agent` | `frontend_agent` | `generic_agent`)
- `status` (`open` | `done` | `reopened` | `deferred_todo`)
- `file` e/ou `requirement`
- `issue` + critério de aceitação
- `round`
- notas append-only (reaberturas)
- `todo_text` / âncora no `current-state.md` quando `deferred_todo`
- evidência opcional (path de health-report, screenshot, …)

Owner:

- QA e E2E: `failures[].agent`
- Browser UI: `failure_class` → frontend ou backend
- Judge: owner do gap (explícito no JSON; default ao único agente de
  implementação ativo, senão falha fechado)

Auditoria: continuar a usar `conversation` com `task_ids` a incluir `find-*`.
Eventos novos ou payload alargado de `loop_back` devem referir IDs, não só
prosa.

## 5. Contrato dos relatórios JSON

Estender os Output Format existentes (sem os agentes fecharem estágios):

- QA / E2E: cada `failures[]` vira finding (id opcional; Go atribui se
  faltar). Campo `reopen_id` quando for regressão de um finding `done`.
- Judge: `implementation_gaps[]` → findings; `sdd_ambiguity: true` não
  gera finding.
- Browser UI: falhas com `failure_class`; warnings de PNG não geram finding.
- Implementation: `tasks_completed` / `tasks_remaining` = disjoint union da
  assignment (`task-*` e/ou `find-*`).

Persistência determinística sugerida: `hero stage close --failed --findings-json`
e/ou parse TUI do JSON já emitido, depois `hero stage loop-back`. O `--reason`
deixa de ser a assignment; no máximo aponta “see findings find-qa-1,…”.

## 6. Escalação humana e `/hero-add-todo`

Quando o estágio do loop está `Escalated` (iterações esgotadas):

- `/hero-continue [N]` — mais iterações; findings `open`/`reopened` ficam
  atribuíveis.
- `/hero-add-todo` — estaciona os findings abertos do loop atual em ToDos e
  permite o ciclo continuar **sem** os implementar agora.
- `/hero-add-todo find-qa-1` — estaciona um ID; se restarem `open`, Implementation
  não fecha.
- `/hero-cancel` / `/hero-finish` — inalterados.

`/hero-add-todo` é o comando **mutante**. `/hero-todos` continua **só leitura**.
Deve haver verbo CLI determinístico (`hero add-todo`) para a TUI/orquestrador
persistirem sem o agente editar markdown.

Efeito de estacionar:

1. Finding → `deferred_todo` no SQLite.
2. Linha acrescentada em `context/current-state.md` numa secção de pendências
   reconhecida pelo parser atual (ex. `## Pending Features` ou
   `## Pending (deferred from cycle)` — o SDD escolhe; o parser de
   `internal/todos` já aceita `## Pending …`).
3. Texto estável, em inglês, com ID do finding, ciclo, estágio origem e
   síntese. Sem bytes/segredos.
4. Se não restarem OpenSpec `[ ]` nem findings `open`/`reopened`, a
   Implementation pode fechar (assignment efetivamente vazia com relatório
   vazio) e o ciclo avança para o próximo estágio habilitado.
5. No **mesmo ciclo**, QA / Judge / Browser UI / E2E **não** recriam um
   finding equivalente ao `deferred_todo` (dedupe por ID ou fingerprint
   file+issue). Caso contrário o loop recomeça e o “continuar” é falso.
6. O item no `current-state.md` sobrevive ao arquivo do ciclo para um ciclo
   futuro; o finding na DB do ciclo arquivado fica histórico.

Isto não substitui `/hero-sync`. Sync continua a fundir pendências de PRD/ADR.
`/hero-add-todo` só move findings **deste** loop.

## 7. Status, comando e TUI

Concordo com enriquecer as superfícies **já existentes**, não um ecrã novo na
navbar.

### 7.1 Ecrã Status + `hero status` + `/hero-status`

Além da tabela de estágios, o estado do ciclo ativo deve mostrar:

- Histórico curto de loop-back: origem → Implementation, round, data.
- Board de findings: ID, origem, owner, status (`open` / `reopened` /
  `done` / `deferred_todo`), one-line issue.
- Contagem: N abertos, N reabertos, N feitos, N nos ToDos.
- CTA quando Escalated: `/hero-continue`, `/hero-add-todo`, `/hero-cancel`,
  `/hero-finish`.

`--json` inclui o mesmo bloco para Telegram e orquestrador.

### 7.2 Ecrã Events

Manter o log append-only. Eventos de finding (created / done / reopened /
deferred_todo) devem ser legíveis (tipo + IDs), não só o blob `reason`.

### 7.3 `/hero-todos` e ecrã que lista pendências

A listagem inclui:

- pendências long-lived já existentes em `current-state.md`;
- itens `deferred_todo` deste (ou do último) ciclo, claramente marcados
  (ex. `C14 find-qa-1 · chips restaurados na rejeição de capacidade`).

Aviso de `/hero-sync` permanece. `/hero-todos` continua a não mutar.

### 7.4 Chat / paleta

`/hero-add-todo` entra no vocabulário slash (ADR-024), paleta TUI, help,
Telegram `/help`, e nos prompts de Escalated. Não anexar imagens a este
comando (fora de âmbito C14 multimodal).

## 8. Fora de escopo deste desenho

- Reabrir checkboxes do OpenSpec `tasks.md` no loop-back.
- Agentes a escrever `current-state.md` ou findings MD.
- Usar `/hero-add-todo` fora de `Escalated` no primeiro incremento (evitar
  atalho para “ignorar QA” a meio da iteração).
- ToDos como tracker genérico de produto além de `current-state.md`.
- Ecrã de navbar extra só para findings.
- Corrigir o C14 em curso com este desenho (C14 precisa de wave com
  arrays vazios ou `/hero-start` de verificação; esta ideia é ciclo
  seguinte).

## 9. Consequências e ciclo seguinte

- Emenda ADR-075 (assignment `find-*`, wave vazia, mensagens de gate).
- Emenda ADR-013 / schema SQLite (v11: tabela de findings).
- Emenda ADR-028 (`/hero-add-todo` mutante vs `/hero-todos` leitura).
- CLI: persistir findings no close failed; `hero add-todo`; `hero status`
  com bloco de findings.
- TUI: parse dos quatro Output Format; Status/Events; paleta; Escalated
  CTAs.
- Prompts: QA, Judge, Browser UI, E2E, Implementation, orchestration;
  Judge deixa de escrever gaps MD e de chamar loop-back.
- Testes: `tasks.md` todo `[x]` + findings abertos fecha só depois de
  `find-*` done; ID `task-*` numa wave de findings falha; `/hero-add-todo`
  + QA seguinte não reabre o mesmo fingerprint.

O C14 multimodal **não** deve absorver este trabalho. Ficheiro em
`docs/idea/tobe/` para o Discover do ciclo corrente o ignorar.
