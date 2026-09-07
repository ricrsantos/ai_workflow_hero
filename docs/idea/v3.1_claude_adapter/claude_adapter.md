# Ideia: Adapter Claude Code para o Hero

> Status: ideia ativa, não normativa. Este documento orienta a etapa de Research de um ciclo futuro. PRD, ADR e UI posteriores prevalecem em caso de conflito.

## Objetivo

Adicionar o Claude Code como o quarto harness da TUI do Hero, identificado por `claude`. O usuário poderá selecionar Claude por agente e por etapa em `workflow-config.yml`, manter sessões, acompanhar streaming, interromper execuções, aprovar operações e obter métricas, sem alterar o engine determinístico, os estágios ou o modo Cursor IDE.

```text
Hero TUI → HarnessAdapter → ClaudeAdapter → claude -p (subprocesso) → Claude Code
                                      └── bridge MCP temporário para permissões
```

A integração deve preservar o princípio de multi-harness atual: cada agente declara um par explícito `harness + model`, modelos usam identificadores nativos, o fallback é somente o par declarado em `fallback_model`, e toda troca de par é visível ao usuário.

## Decisões confirmadas

1. **Transporte: CLI headless.** `ClaudeAdapter` será escrito em Go e executará `claude -p` uma vez por `Execute`; não haverá bridge Node/TypeScript com o Agent SDK, automação do TUI do Claude, nem daemon Hero.
2. **Streaming e sessão:** usar `--output-format stream-json --verbose --include-partial-messages`; capturar o `session_id` do fluxo e usar `--resume <id>` nos turnos seguintes. O processo é efêmero, mas a sessão do Claude Code permanece retomável.
3. **TUI-only:** como OpenCode e Codex, Claude será um harness da TUI. O Runtime de slash commands no Cursor continua exclusivamente Cursor e nunca inicia Claude Code.
4. **Assets nativos:** habilitar Claude provisiona `assets/claude/` em `.claude/`, com commands, skills e agentes no formato do Claude Code. Não criar uma cópia de `AGENTS.md`.
5. **Memória comum:** o Hero cria/gerencia uma seção marcada em `CLAUDE.md` na raiz que importa `@AGENTS.md`. Assim Claude recebe as mesmas instruções de projeto sem duplicação e o usuário pode manter instruções específicas do Claude abaixo da seção.
6. **Permissões com paridade:** o perfil `ask` não pode simplesmente negar operações no modo headless. O adapter terá uma bridge MCP temporária para encaminhar pedidos ao `OnPermissionRequest` da TUI. O contrato de fio da bridge é um spike obrigatório antes da implementação principal.

## Escopo funcional

### Adapter e ciclo de execução

`internal/adapters/claude` implementará `harness.HarnessAdapter` e os contratos opcionais já usados pela TUI (`ModelLister`, descoberta de propriedades e `HealthChecker` quando aplicável). O engine, `internal/harness`, `harnessmgr`, store e TUI continuam usando somente os tipos normalizados atuais.

Para uma execução normal, o adapter:

1. valida `claude` no `PATH` e cria um processo supervisionado no `ProjectDir`;
2. compõe argumentos equivalentes a `claude -p --output-format stream-json --verbose --include-partial-messages --model <modelo>`, mais `--resume <sessão>` quando houver sessão Hero vinculada;
3. aplica as propriedades e o perfil de permissão aprovados;
4. decodifica cada linha NDJSON sem bloquear o leitor de stdout;
5. persiste o primeiro `session_id` emitido antes de concluir o turno;
6. mapeia o resultado final, uso, duração e saída para `harness.ExecutionResult`;
7. encerra o processo filho e a bridge temporária, sem apagar a sessão nativa do usuário.

`Cancel` envia inicialmente SIGINT ao grupo de processo, aguarda um período curto e faz kill apenas se o filho não terminar. SIGTERM não é o cancelamento preferido: a documentação do Claude Code informa que ele encerra com o turno incompleto. Ao retomar, o adapter deve preservar a sessão e nunca reenviar silenciosamente o prompt original.

Não haverá registro SQLite de processo servidor nem reap de órfãos: diferentemente de OpenCode e Codex, o subprocesso Claude pertence a um único `Execute` e deve desaparecer no fim dele. O SQLite continua sendo a fonte da ligação `harness_session_id ↔ claude session_id`, com o harness `claude` gravado para impedir retomada cruzada.

### Eventos e streaming

O parser deve mapear, no mínimo:

| Evento Claude Code | Destino Hero |
|---|---|
| `system/init` | sessão, modelo efetivamente resolvido, capacidades, plugins/MCP carregados |
| `stream_event` com `text_delta` | `StreamKindText` |
| pensamento/reasoning quando emitido | `StreamKindThinking` |
| uso de ferramenta e resultado | `StreamKindTool` ou `StreamKindActivity` |
| mensagens de subagente com `parent_tool_use_id` | `TASK`/ciclo de vida de subagente existente |
| retry da API, hook e plugin | atividade ou warning conforme o perfil de verbosidade |
| negação de permissão, erro de autenticação, modelo ou protocolo | warning/erro explícito |
| `result` final | `ExecutionResult`, sessão, uso e custo informado |

Eventos não reconhecidos nunca podem causar panic ou ser descartados em silêncio: geram `StreamKindWarning`, com tipo e payload redigido/truncado somente no modo Debug. O `result` final é a fonte de verdade para reparar qualquer delta parcial perdido.

### Permissões, perguntas e segurança

Os perfis persistidos em `hero.json → harnesses.claude.permission_profile` mantêm o vocabulário já existente:

| Perfil Hero | Mapeamento desejado no Claude Code |
|---|---|
| `ask` | modo manual + `--permission-prompt-tool` apontando à bridge MCP temporária; a TUI mostra `Allow? [y/N]` e devolve a decisão |
| `auto-project` | `--permission-mode acceptEdits`, sem liberar shell, rede, MCP ou caminhos externos além das regras nativas do Claude |
| `auto-all` | modo nativo equivalente a `bypassPermissions`; requer confirmação explícita e copy de risco |

A bridge MCP será iniciada pelo mesmo `Execute`, autenticada por token aleatório de uso único e exposta apenas por IPC local/stdio. Ela terá uma única responsabilidade: receber a solicitação de permissão do Claude Code, convertê-la em `harness.PermissionRequest`, esperar a resposta da TUI e devolver a decisão. Ela não será uma ferramenta geral para o modelo, não persistirá credenciais e será encerrada junto com o processo Claude.

O primeiro task de implementação deve validar, com fixture e processo falso, o schema e a semântica de `--permission-prompt-tool`; se a versão instalada não suportar o mecanismo, o adapter falha de modo explícito no perfil `ask`, sem degradação silenciosa para `auto-project` ou `auto-all`.

O modo headless pode carregar `CLAUDE.md`, hooks, plugins e MCP do projeto sem diálogo de confiança. Isso é desejado para compatibilidade com o ambiente Claude do projeto, mas deve aparecer no aviso ao habilitar o harness e na ajuda. `--bare` fica fora da primeira versão: ele desabilita justamente o contexto e as capacidades provisionadas pelo Hero.

### Projeção Claude Code e `CLAUDE.md`

Ao selecionar Claude no install ou em `/hero-harness`, o Hero deve provisionar, com checksums e regras de conflito iguais às demais projeções:

```text
assets/claude/                         projeto/
├── commands/hero-*.md       →        .claude/commands/hero-*.md
├── skills/workflow-hero/     →        .claude/skills/workflow-hero/
├── skills/grilling/          →        .claude/skills/grilling/
└── agents/*.md               →        .claude/agents/*.md

                                      CLAUDE.md
                                      └── bloco Hero: @AGENTS.md + instruções Claude/Hero
```

- **Commands:** todos os `/hero-*` existentes, preservando a gramática slash-first do Hero.
- **Skills:** `workflow-hero` e `grilling`, como diretórios `<nome>/SKILL.md`, com frontmatter nativo e conteúdo equivalente às skills de Runtime.
- **Agentes:** os agentes Hero existentes serão templates Claude nativos. Seus frontmatters usam somente campos suportados (`name`, `description`, `model`, `effort`, `maxTurns`, `tools`, `disallowedTools`, `skills`, `background`, `isolation` quando cabível). A descrição é curta para não inflar o contexto de inicialização.
- **Contexto:** `CLAUDE.md` é criado na raiz, não dentro do plugin. O bloco gerenciado deve importar `@AGENTS.md`, apontar `context/current-state.md`, `context/context-log.md` e `.workflow-hero/` como fontes operacionais, e remeter procedimentos longos às skills.
- **Sem plugin Hero:** um plugin não atende sozinho a este caso, pois `CLAUDE.md` na raiz de um plugin não se torna memória do projeto e agentes distribuídos por plugin não aceitam `permissionMode`. A árvore `.claude/` é a integração correta.

Formato proposto para arquivo novo:

```md
<!-- BEGIN AI WORKFLOW HERO MANAGED CONTEXT -->
@AGENTS.md

## AI Workflow Hero

Use the project workflow provided by `.claude/skills/workflow-hero/`.
Keep operational cycle state in `.workflow-hero/`; project knowledge belongs in `context/`.
<!-- END AI WORKFLOW HERO MANAGED CONTEXT -->
```

Se `CLAUDE.md` já existir, o Hero nunca o sobrescreve. A TUI/instalador deve oferecer uma escolha explícita para inserir ou atualizar apenas esse bloco marcado, mostrar o diff, ou manter o arquivo intacto. Em upgrade/uninstall, somente o bloco que o Hero criou é atualizado/removido; as demais instruções do usuário sobrevivem. Se `AGENTS.md` estiver ausente, a criação do bloco deve falhar com orientação clara, pois não é permitido inventar uma cópia divergente.

Durante `PrepareHeroStart`, o adapter atualiza somente os campos Hero-gerenciados de `.claude/agents/<agent>.md` a partir do `workflow-config.yml` (modelo, effort e, quando aplicável, skills). Customizações fora dos marcadores/frontmatter gerenciados não são apagadas.

### Seleção de harness, modelo e propriedades

`claude` entra em `SupportedHarnessIDs`, no picker do install, `/hero-harness`, Doctor, Status, `/hero-model`, Config e wizard Telegram. É opt-in: upgrades de projetos anteriores adicionam `harnesses.claude.enabled=false` e não criam `.claude/` nem `CLAUDE.md` até o usuário habilitar Claude.

Os modelos serão identificadores nativos do Claude Code. O catálogo inicial lista aliases estáveis `sonnet`, `opus`, `haiku` e `fable`, com suporte a IDs completos e sufixos de contexto (`sonnet[1m]`, `opus[1m]`) quando permitidos pela conta. O picker deve explicar que uma organização pode restringir ou substituir aliases; a validação definitiva ocorre no primeiro `system/init`, que informa o modelo resolvido.

Não existe uma API estável e gratuita para `ListModels` refletir exatamente a conta/allowlist. Portanto `ClaudeAdapter.ListModels` retorna primeiro os modelos do catálogo local; não deve lançar uma chamada de modelo só para preencher o picker. Erro ou substituição do modelo por política aparece de forma explícita no streaming e ativa o fallback Hero somente quando o par não puder executar.

O catálogo será adicionado como `assets/models/claude.yml` e instalado em `.workflow-hero/models/claude.yml`, seguindo o schema atual (`provider`, `version`, `last_updated`, `currency`, `unit`, `models`, preços, cache, `context_window`, `properties`). A tarefa de catálogo inclui:

1. fonte oficial e data para cada janela de contexto e preço por milhão de tokens;
2. aliases e IDs completos separados quando suas capacidades/preços diferirem;
3. preço ausente/zero com warning para planos por assinatura, gateways e IDs sem tarifa pública — nunca preço inventado;
4. atualização de `hero update-models`, checksums e testes de lookup/custo/context bar;
5. overlay de catálogo local do projeto preservado, como nos outros harnesses.

Mapeamento C5 inicial:

| Propriedade Hero | Claude Code | Decisão inicial |
|---|---|---|
| `ef` (effort) | `--effort` | suportada; valores vêm do catálogo por modelo (`low`…`max`, e `ultracode` apenas se a versão/capacidade o permitir) |
| `th` (thinking) | raciocínio adaptativo governado por effort | indisponível (`na`) no adapter; não criar um toggle fictício |
| `fs` (fast) | `/fast` é uma mudança de sessão, sem flag headless documentada | indisponível (`na`) na primeira versão |

O adapter só transforma propriedades validadas pelo snapshot C5. Valores suportados pelo CLI, mas recusados por política da organização, geram warning explícito e registram o valor efetivo quando o fluxo o informar.

### Health check, watchdog e cancelamento

Como não há servidor persistente, `CheckHealth` não pode abrir uma segunda sessão Claude nem gastar tokens. Durante um `Execute`, ele avalia somente estado supervisionado:

- `ProcessAlive`: filho `claude` ainda está vivo;
- `ServerAlive`: leitor NDJSON/stdout e bridge de permissão, quando usada, permanecem saudáveis;
- `SessionAlive`: `session_id` conhecido e turno ainda não terminou com erro terminal;
- detalhes: último evento, stderr redigido e motivo normalizado de saída.

`IsAvailable` verifica presença de `claude`, versão e compatibilidade mínima das flags necessárias. `claude doctor` pode enriquecer Doctor, em modo warn-only, mas não deve iniciar sessão, autenticar ou ser pré-requisito para o boot da TUI. Autenticação é confirmada no Execute: `authentication_failed` deve instruir o usuário a rodar `claude`/login no terminal normal; o Hero não pede nem armazena API key.

Adicionar `ClaudeStallTimeout`, inicialmente **5 minutos**, e manter `HealthProbeInterval` de 30 segundos. O watchdog conta texto, thinking, ferramenta, retry, hook progress e lifecycle de subagente como atividade; ignora ruído de arquivo/LSP; pausa enquanto a TUI aguarda permissão ou pergunta. `HealthSuspected` apenas alerta e preserva a chance de Claude concluir uma tarefa longa; `HealthFailed` cancela pelo caminho comum. A implementação deve ter fixtures para ausência de eventos, retry, espera por permissão, processo morto e resultado final atrasado.

### Verbosidade da TUI

`chat_verbosity` continua global e já persistida. Claude deve obedecer a mesma taxonomia, sem esconder sinais de segurança:

| Perfil | Exibição Claude |
|---|---|
| Compact | saída final e deltas de texto do agente |
| Standard | Compact + lifecycle de ferramenta/subagente, permissões, perguntas, retries resumidos |
| Detailed | Standard + thinking, atividades, hook progress, uso e estado de sessão |
| Debug | Detailed + tipo bruto de evento e payload NDJSON truncado/redigido |

Pedidos de permissão/pergunta, falhas de sessão, warnings e estado de agentes continuam visíveis em todos os perfis. Eventos ocultos ainda atualizam watchdog, uso e `AI rp`; a filtragem é visual, não altera o comportamento do adapter.

## Fora de escopo inicial

- Bridge TypeScript/Python Agent SDK, automação do TUI Claude e daemon `hero serve`.
- Iniciar login OAuth/navegador dentro da TUI ou armazenar `ANTHROPIC_API_KEY`.
- Configurar, copiar ou administrar MCP, plugins, hooks, LSP, monitores ou marketplace do usuário; o adapter apenas respeita as configurações nativas já carregadas pelo Claude Code.
- Anexos multimodais, fork/arquivamento manual de sessões, agent teams e dynamic workflows como novas superfícies Hero.
- `--bare`/modo isolado, Windows Hero e qualquer mudança no Runtime Cursor.
- Tradução universal de modelos ou uma terceira etapa de fallback.

## Critérios de aceitação para o futuro PRD/SDD

1. Claude pode ser habilitado/desabilitado sem apagar arquivos `.claude/` do usuário e nunca é habilitado automaticamente em upgrade.
2. Um ciclo TUI pode misturar Cursor, OpenCode, Codex e Claude; a UI sempre mostra agente, modelo e harness.
3. Um Execute Claude transmite texto, ferramentas, uso, sessão, cancelamento e warnings; eventos desconhecidos não desaparecem.
4. O par `claude + model` persiste e retoma a sessão correta; nenhuma sessão de outro harness é reutilizada.
5. Os três perfis de permissão funcionam conforme esta proposta; `ask` tem teste end-to-end da bridge MCP.
6. `CLAUDE.md` importa `AGENTS.md`, os assets `.claude` tornam commands/skills/agentes Hero descobríveis, e conflitos de arquivo preservam conteúdo do usuário.
7. Catálogo, propriedades C5, context bar e custo funcionam sem inventar preço para planos não tarifados.
8. Health check não cria processo extra, watchdog não denuncia falso hang durante resposta de permissão e o cancelamento não deixa filhos/bridge órfãos.
9. `go test ./...` permanece verde, com testes sem LLM real para processo, NDJSON, bridge, projeção, catálogo, health e TUI.

## Referências oficiais consultadas

- [Claude Code — execução programática/headless](https://code.claude.com/docs/en/headless): `-p`, JSON/NDJSON, streaming, sessões, cancelamento, permissões e aviso sobre carregamento de configuração no headless.
- [Claude Code — referência da CLI](https://code.claude.com/docs/en/cli-usage): flags de print, formato de saída, modelo, permissões e orçamento.
- [Claude Code Agent SDK — visão geral](https://code.claude.com/docs/en/agent-sdk/overview): CLI como subprocesso para linguagens fora de Python/TypeScript.
- [Claude Code — memória e CLAUDE.md](https://code.claude.com/docs/en/memory): importação `@AGENTS.md`, escopos e preservação de instruções.
- [Claude Code — skills e commands](https://code.claude.com/docs/en/slash-commands): estrutura `.claude/skills/<nome>/SKILL.md` e descoberta de commands.
- [Claude Code — subagentes](https://code.claude.com/docs/en/subagents): frontmatter, modelo, effort, skills e permissões de agentes.
- [Claude Code — configuração de modelos](https://code.claude.com/docs/en/model-config): aliases, modelo efetivo, allowlist, effort, contexto e fallback nativo.
- [Claude Code — permissões](https://code.claude.com/docs/en/permissions): modos manual, `acceptEdits`, `dontAsk` e `bypassPermissions`.
- [Claude Code — plugins](https://code.claude.com/docs/en/plugins): estrutura de plugin e limite de `CLAUDE.md`/`permissionMode` em plugins, que fundamenta a decisão pela projeção `.claude/`.
