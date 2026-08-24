# Ideia V2.8 — Tela de Configuração do Ciclo na TUI

**Status:** consenso de produto, pronto para Research/Planning.

## Objetivo

Oferecer na Hero TUI uma tela guiada para configurar o ciclo ativo sem substituir o arquivo `.workflow-hero/cycles/current/workflow-config.yml`.

O YAML continua sendo a fonte única de verdade e permanece editável diretamente. A tela reduz a carga de configurar manualmente harnesses, modelos, propriedades e dependências entre etapas/agentes.

## Compatibilidade

O fluxo do Cursor IDE não muda:

1. `/hero-new` prepara os assets e cria/importa o `workflow-config.yml`.
2. O usuário pode editar o YAML no Cursor.
3. `/hero-start` inicia o ciclo a partir do arquivo.

No uso pela TUI, a mesma configuração pode ser preenchida visualmente. A tela não cria uma segunda configuração, não grava preferências do ciclo em `hero.json` e não altera os comandos Runtime do Cursor.

## Disponibilidade e ciclo de edição

- Adicionar **Config** como o segundo item da barra lateral: `Chat | Config | Status | Artifacts | Costs | Events`.
- Mostrar o item somente quando existir um ciclo ativo; ele não existe no `hero chat` (free chat).
- Permitir edição durante todo o ciclo ativo, desde que nenhum agente esteja executando ou em preflight.
- Enquanto houver execução, mostrar a configuração em modo somente leitura e explicar que a gravação ficará disponível quando a execução terminar.
- Cada `Save` grava o YAML e chama a sincronização do ciclo. Título, objetivo, snapshot e budgets das etapas ainda abertas são atualizados no SQLite; etapas concluídas ou falhas não são alteradas.
- Oferecer `Save` e `Save and start`. O segundo só fica disponível para uma configuração válida e então inicia o mesmo fluxo de `/hero-start`.

## Experiência proposta

```text
Config · Cycle C12

Identity
  Title
  Objective
  Chat language ▾

Scope
  [x] Backend  [ ] Frontend  [ ] Native  [ ] Script  [ ] Infrastructure

Stages
  [x] Research
      Purpose · Iterations · Timeout · Human approval
      Discover agent
        Harness ▾  Model ▾  supported properties ▾

  [ ] Browser UI validation

  [x] Implementation
      Purpose · Iterations · Timeout · Human approval
      Backend agent
        Harness ▾  Model ▾  supported properties ▾

Shared / Advanced
  Orchestration agent
  Context agent
  Fallback model

[Save]  [Save and start]
```

Campos descritivos (`title`, `objective` e `purpose`) continuam sendo texto. Booleanos usam toggles, limites usam controles numéricos, e escolhas fechadas usam listas navegáveis por teclado.

## Revelação progressiva

A tela deve esconder controles irrelevantes sem apagar seus valores do YAML. Assim, reativar uma etapa ou escopo recupera a configuração anterior.

### Etapas e escopo

- Uma etapa desativada mostra apenas seu toggle; propósito, budgets, aprovação, opções específicas e agentes vinculados ficam ocultos.
- `implementation` ativa exibe somente os agentes correspondentes aos escopos ativos:
  - `backend_agent` para `scope.backend`;
  - `frontend_agent` para `scope.frontend`;
  - `generic_agent` para qualquer um de `native`, `script` ou `infrastructure`.
- `discover_agent`, `planning_agent`, `qa_agent`, `judge_agent`, `browser_ui_agent` e `end2end_qa_agent` aparecem somente quando sua etapa estiver ativa.
- `orchestration_agent`, `context_agent` e `fallback_model` ficam em **Shared / Advanced**, pois não pertencem exclusivamente a uma etapa.
- `browser_ui_validation` só pode ser habilitada com `scope.frontend=true`; então revela `visual_validation` e `browser_ui_agent`.
- `qa_end_to_end.use_playwright` só pode ser habilitado com `scope.frontend=true`.
- O bloco `subagent` só aparece quando `same_of_agent=false`. Ele usa o harness do agente pai, como no schema atual.

### Harness, modelo e propriedades

Para cada agente e para o fallback, a escolha é sequencial:

```text
Harness habilitado → modelo nativo daquele harness → propriedades suportadas pelo modelo
```

- O menu de harness mostra somente harnesses habilitados no projeto; indisponibilidade é apresentada como aviso, não como troca silenciosa.
- O menu de modelo é filtrado pelo harness escolhido e reutiliza os catálogos/cache e a atualização em background já definidos para `/model`.
- As propriedades `fs` (fast), `th` (thinking) e `ef` (reasoning effort) usam as capacidades normalizadas da C5. Somente controles aceitos pelo modelo são exibidos.
- Essas escolhas gravam diretamente `harness`, `model`, `enable_fast_model`, `thinking` e `reasoning_effort` no bloco do YAML em edição. Elas não usam `hero.json.model_properties`, que é reservado ao free chat e `/hero-new`.
- Metadados de modelo ausentes não bloqueiam a seleção: a tela avisa e preserva os valores compatíveis do YAML (`false`/`na` quando não houver escolha suportada).

## Integridade do YAML e validação

O arquivo pode ter comentários, `workflow_rules` e chaves de versões futuras. Portanto, a implementação não pode desserializar o documento inteiro em uma struct e regravá-lo.

1. Carregar o YAML como `yaml.v3.Node`, além de decodificar os campos administrados para o estado do formulário.
2. No salvamento, alterar somente os nós pertencentes à tela e preservar comentários, ordem e chaves desconhecidas.
3. Gravar atomically (arquivo temporário no mesmo diretório, `rename`) somente após validação completa.
4. Registrar a revisão/hash lida ao abrir a tela. Se o arquivo for editado externamente, oferecer: recarregar, reaplicar as mudanças da tela sobre a versão recente, ou cancelar.

Validações antes de salvar:

- título e objetivo obrigatórios;
- `max_iterations > 0` e `timeout_minutes` válido para etapas ativas;
- pelo menos um escopo ativo quando `implementation.enabled=true`;
- gates de frontend para Browser UI Validation e Playwright;
- harness habilitado e modelo não vazio para cada bloco exibido/necessário;
- propriedades selecionadas aceitas pelo snapshot de capacidade quando essa informação existir.

Os erros devem apontar o campo e a regra, sem gravar um estado parcial.

## Direção de implementação

- **`internal/tui`**: adicionar `screenConfig`, navegação condicional, estado do formulário, renderização por seções, teclado, mensagens de confirmação e ações Save/Save and start.
- **`internal/workflowconfig`**: criar um documento editável/round-trip seguro, validação semântica e mutações direcionadas por caminho YAML. Centralizar aqui as regras para que a TUI não conheça detalhes de serialização.
- **`internal/modelprops` e `internal/harnessmgr`**: reutilizar descoberta, cache e capability snapshots da C5 para os menus de modelo/propriedades; a tela persiste o resultado no draft de workflow, não em `hero.json`.
- **`internal/cycle` / `engine`**: após uma gravação bem-sucedida, reutilizar `SyncCycleConfig`; não criar uma fonte paralela de estado.
- Manter os adapters como donos de protocolos específicos de harness. A TUI consome somente harness, id de modelo e propriedades normalizadas.

## Testes de aceitação

1. A navegação mostra Config somente com ciclo ativo e nunca em free chat.
2. Uma etapa/agente desativado deixa de expor os controles dependentes, sem perder seus dados.
3. Scope altera corretamente os agentes de implementação visíveis.
4. Harness → modelo → propriedades filtra valores e persiste no bloco YAML correto.
5. Propriedades indisponíveis ou catálogo ausente têm aviso explícito e não causam falha silenciosa.
6. Um save preserva comentários, `workflow_rules` e chaves desconhecidas.
7. Alteração externa do YAML não é sobrescrita sem escolha explícita do usuário.
8. Save sincroniza o ciclo ativo; etapas concluídas não mudam e etapas abertas refletem os novos budgets/flags.
9. Save and start segue a mesma preflight, validação e execução de `/hero-start`.
10. O fluxo Cursor IDE, incluindo edição direta e `/hero-start`, permanece inalterado.
11. `go test ./...` permanece verde.

## Fora de escopo inicial

- Substituir o YAML como interface de configuração.
- Alterar o Runtime ou a experiência de slash commands do Cursor IDE.
- Criar uma configuração global de agentes em `hero.json`.
- Permitir alteração concorrente enquanto um agente executa.
- Alterar o contrato dos HarnessAdapters ou introduzir uma nova camada de orquestração.
