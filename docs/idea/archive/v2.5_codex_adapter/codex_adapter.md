# Ideia: Codex Adapter para o Hero

## Objetivo

Implementar um `CodexAdapter` que permita ao Hero utilizar o OpenAI Codex como Harness/Executor, mantendo o Hero como responsável pela orquestração dos Ciclos e Etapas.

Arquitetura:

Hero → HarnessAdapter → CodexAdapter → Codex CLI / App Server → Codex → ChatGPT Account

O usuário deve poder autenticar o Codex com sua conta ChatGPT, preservando o uso dos limites/subsídios do plano, sem necessidade de API Key.

---

## Estratégia de integração

A implementação deve priorizar o **Codex App Server** em vez de tratar o TUI do Codex como interface de automação.

O App Server é a interface oficial do Codex para integrações profundas e fornece comunicação bidirecional JSON-RPC, via `stdio`/JSONL local ou outros transports. Ele expõe threads, turns, itens, approvals e eventos incrementais. :contentReference[oaicite:1]{index=1}

O Adaptador que será contruido deve utilizar como uma referência do adaptador do opencode. Deverá ter todas as funções, ser cpaz de interpretar todos os tipos de mensagens, eventos, permissões, comandos, etc. recebidos do **Codex APP Server**. Deve tratar do o ciclo de vida do serviço (aubir o serviço, parar o serviço, resetar, etc.) nunca falhar silenciosamente, entre outroa (Análise o adapter do opencode para extrair todas as funcionalidades)

---

## Responsabilidades do CodexAdapter

### 1. Lifecycle do Codex

O adapter deve:

- Detectar se `codex` está instalado.
- Detectar a versão instalada.
- Executar diagnóstico (`codex doctor`) quando necessário.
- Detectar estado de autenticação.
- Detectar/validar compatibilidade da versão.
- Iniciar e supervisionar `codex app-server`.
- Gerenciar stdin/stdout/stderr quando utilizando stdio.
- Detectar processo encerrado inesperadamente.
- Detectar timeout.
- Detectar desconexão.
- Reiniciar o App Server quando apropriado.
- Encerrar o processo de forma limpa ao finalizar o Hero.
- Nunca deixar processos Codex órfãos.

O `codex doctor` fornece diagnóstico de instalação, configuração, autenticação, runtime, Git, terminal, App Server e threads. :contentReference[oaicite:3]{index=3}

---

## 2. Autenticação

O adapter deve verificar se o usuário está autenticado. SE não estiver deve solicitar que o usuário realize o login através do cli do codex:

`codex login`

Não solicitar API Key quando o objetivo for utilizar a conta ChatGPT.

O adapter deve retornar um erro claro quando o Codex não estiver autenticado e orientar o usuário a executar o login.

---

## 3. Inicialização do App Server

Inicializar:

1. processo `codex app-server`
2. conexão via stdio/JSONL
3. `initialize`
4. `initialized`
5. criação ou recuperação de thread
6. execução de turns

O protocolo é JSON-RPC bidirecional e cada versão do Codex pode gerar seus próprios schemas através de:

`codex app-server generate-json-schema`

ou

`codex app-server generate-ts`

O adapter deve trabalhar com o schema correspondente à versão detectada do Codex. :contentReference[oaicite:5]{index=5}

---

## 4. Sessões / Threads

O adapter deve mapear:

Hero `harness_session_id`
→
Codex `thread_id`

Suportar:

- criar thread
- recuperar/resumir thread
- continuar conversa
- fork de thread
- arquivar thread
- desarquivar thread
- encerrar/cancelar turn
- manter contexto entre interações

O App Server possui primitivas explícitas para `thread/start`, `thread/resume`, `thread/fork` e operações de archive/unarchive. :contentReference[oaicite:6]{index=6}

O Hero não deve depender do histórico armazenado pelo TUI. O estado da sessão deve ser controlado pelo adapter e persistido no estado do Hero.

---

## 5. Prompts / Turns

O adapter deve permitir:

- enviar prompt inicial
- enviar prompts subsequentes
- enviar múltiplos itens de input
- anexar imagens quando suportado
- continuar uma interação existente
- interromper uma execução
- fazer steering de um turn em andamento

O App Server utiliza `turn/start` para iniciar uma execução e `turn/steer` para adicionar instruções a um turn que ainda está em execução. :contentReference[oaicite:7]{index=7}

O adapter deve traduzir o modelo de prompt do Hero para o formato de input do Codex.

---

## 6. Streaming de eventos

Este é um dos pontos mais importantes.

O adapter deve consumir TODOS os eventos relevantes do App Server e convertê-los para o modelo interno de eventos do Hero.

No mínimo:

- thread started
- thread resumed
- thread archived/unarchived
- turn started
- turn completed
- turn interrupted
- item started
- item completed
- agent message delta
- agent reasoning/progress quando disponível
- command execution
- command output
- file changes
- tool calls
- MCP activity
- approvals
- errors
- usage/token information quando disponível
- subagent lifecycle
- eventos desconhecidos

O Hero deve:

1. tratar explicitamente todos os eventos assim como no adapter do opencode; 
2. transformar eventos Codex → eventos internos Hero;
3. gerar `warning` para evento desconhecido;
4. NUNCA falhar silenciosamente diante de um evento desconhecido.

O App Server foi projetado justamente para transmitir eventos incrementais de agentes, incluindo mensagens, comandos, mudanças de arquivos e outras atividades. :contentReference[oaicite:8]{index=8}

---

## 7. Respostas

O adapter deve separar claramente:

- resposta final do agente;
- deltas de resposta;
- reasoning/progresso;
- comandos executados;
- output dos comandos;
- alterações em arquivos;
- tool calls;
- approvals;
- erros.

O Hero deve receber a resposta final em seu formato interno padrão, independentemente do Harness.

---

## 8. Erros

Mapear pelo menos:

- Codex não instalado
- versão incompatível
- não autenticado
- autenticação expirada
- App Server não iniciou
- App Server morreu
- conexão perdida
- timeout
- JSON/JSONL inválido
- JSON-RPC error
- modelo indisponível
- limite/quota atingido
- sandbox bloqueando operação
- approval necessário
- comando rejeitado
- MCP error
- tool error
- erro do agente
- erro desconhecido

Erros devem possuir:

`code`
`message`
`category`
`recoverable`
`raw_error`

Quando possível, o Hero deve distinguir erro recuperável de erro fatal.

---

## 9. Aprovações / Human-in-the-loop

O adapter deve integrar o sistema de approvals do Codex ao mecanismo de aprovação do Hero.

O Hero deve conseguir receber:

`Codex → approval request → Hero → usuário → approve/reject → Codex`

Não devemos simplesmente executar Codex com `--yolo`.

O adapter deve respeitar sandbox e approval policy.

O Codex possui políticas como:

- `untrusted`
- `on-request`
- `never`

e sandbox:

- `read-only`
- `workspace-write`
- `danger-full-access`

Além disso existem aprovações granulares para sandbox escalation, MCP elicitation, request permissions, rules e skills. :contentReference[oaicite:9]{index=9}

O Hero deve controlar essas opções através de sua própria configuração, traduzindo-as para o Codex.

---

## 10. Workspace

O adapter deve conseguir definir:

- working directory
- workspace
- diretórios adicionais autorizados
- sandbox mode
- approval policy

O diretório da Etapa/Ciclo deve ser explicitamente enviado ao Codex.

O CLI suporta `--cd`, `--add-dir`, sandbox e approval policy. :contentReference[oaicite:10]{index=10}

---

## 11. Modelos e reasoning

O Hero deve poder selecionar:

- modelo
- reasoning effort
- configurações específicas suportadas pelo Codex

O adapter deve consultar/validar o modelo disponível quando necessário.

O CLI permite sobrescrever o modelo por execução e também possui catálogo de modelos. :contentReference[oaicite:11]{index=11}

Não hardcodar modelos no adapter.

---

## 12. Skills

O Hero possui suas próprias skills e deve conseguir disponibilizá-las ao Codex.

A integração deve considerar:

- instalação de skills;
- descoberta de skills;
- skills globais;
- skills específicas do projeto;
- `SKILL.md`;
- scripts;
- references;
- assets;
- ativação explícita;
- ativação implícita.

O formato atual de skills é baseado em diretórios contendo `SKILL.md`, podendo incluir scripts, referências e assets. Codex suporta ativação explícita e implícita de skills. :contentReference[oaicite:12]{index=12}

O adapter não deve simplesmente copiar indiscriminadamente todas as skills do Hero para o Codex.

Deve existir uma estratégia explícita de:

`Hero Skill → Codex Skill`

com escopo e versionamento.

---

## 13. AGENTS.md

O adapter deve suportar a disponibilização das instruções do projeto ao Codex através de `AGENTS.md`.

Deve ser definido claramente:

- quem é a fonte da verdade;
- como Hero instructions são convertidas;
- onde o arquivo é criado;
- quando é atualizado;
- diferença entre instruções permanentes do projeto e instruções específicas da Etapa.

Evitar duplicar regras entre Hero e Codex.

---

## 14. Subagents

O adapter deve suportar a capacidade de subagents do Codex quando ela estiver disponível.

O Codex atualmente suporta workflows com múltiplos agentes e agentes especializados, inclusive em paralelo. :contentReference[oaicite:13]{index=13}

O Hero deve conseguir:

- detectar subagent started;
- detectar subagent progress;
- detectar subagent completed;
- detectar subagent failed;
- identificar thread do subagent;
- identificar relação parent/child;
- receber resultado do subagent;
- exibir progresso no TUI;
- contabilizar custo/uso quando disponível.

IMPORTANTE:

Não assumir que a configuração de subagents permanece estável entre versões do Codex. Houve mudanças/regressões recentes na seleção de agentes customizados. Portanto, o adapter deve detectar capabilities da versão instalada e degradar de forma segura. 

---

## 15. MCP

O adapter deve permitir que o Codex utilize MCP quando configurado.

Suportar, quando necessário:

- listar MCP servers;
- adicionar;
- remover;
- autenticar;
- detectar tools disponíveis;
- receber MCP tool calls;
- receber MCP errors;
- encaminhar MCP approvals/elicitation para o Hero.

O CLI possui `codex mcp` para gerenciamento de MCP servers. :contentReference[oaicite:14]{index=14}

---

## 16. Plugins

Plugins não devem ser tratados inicialmente como uma responsabilidade central do `CodexAdapter`.

Porém o adapter deve permitir que o ambiente Codex existente do usuário continue funcionando com plugins.

O CLI possui gerenciamento de plugins e marketplaces. :contentReference[oaicite:15]{index=15}

O Hero não deve tentar replicar o marketplace do Codex.

---

## 17. Web Search

O adapter deve respeitar a configuração de web search do Hero.

Quando necessário, habilitar live search no Codex.

O CLI possui configuração explícita de `--search`. :contentReference[oaicite:16]{index=16}

---

## 18. Imagens / multimodal

O adapter deve suportar anexos de imagem quando uma Etapa do Hero necessitar.

O CLI suporta `--image` para anexar imagens ao prompt inicial. :contentReference[oaicite:17]{index=17}

---

## 19. Usage / custos

O adapter deve capturar, quando disponibilizado pelo Codex:

- input tokens;
- output tokens;
- reasoning tokens;
- total tokens;
- uso de subagents;
- informações de quota;
- informações de limite.

Esses dados devem ser convertidos para o modelo de `Usage` do Hero.

O objetivo é permitir que cada Etapa continue apresentando o resumo de uso/custo do Harness.

Não assumir que o Codex fornece preço monetário quando o usuário está utilizando o plano ChatGPT; nesse caso, representar principalmente usage/quota.

---

## 20. Process Management

O `CodexAdapter` deve possuir um pequeno `ProcessManager` responsável por:

- spawn;
- stdin;
- stdout;
- stderr;
- PID;
- health;
- cancellation;
- graceful shutdown;
- force kill;
- restart;
- timeout;
- orphan detection.

O processo Codex não deve ser gerenciado diretamente pelo core do Hero.

---

## 21. Compatibilidade

O adapter deve possuir:

`CodexCapabilities`

para detectar dinamicamente capacidades como:

- app-server;
- streaming;
- resume;
- fork;
- steering;
- approvals;
- skills;
- MCP;
- subagents;
- usage;
- image input;
- web search.

A versão do Codex deve ser detectada no startup.

Features experimentais não devem ser utilizadas sem capability detection.

---

## 22. Arquitetura proposta

Hero:

    HarnessAdapter
        │
        └── CodexAdapter
              │
              ├── CodexProcessManager
              │
              ├── CodexAppServerClient
              │      ├── JSON-RPC
              │      ├── Requests
              │      └── Notifications
              │
              ├── CodexEventMapper
              │
              ├── CodexSessionManager
              │
              ├── CodexApprovalHandler
              │
              ├── CodexCapabilities
              │
              └── CodexUsageMapper

Fluxo principal:

    Hero Stage
       │
       ▼
    CodexAdapter.Execute()
       │
       ▼
    thread/start | thread/resume
       │
       ▼
    turn/start
       │
       ▼
    Codex App Server
       │
       ├── streaming events
       ├── tool execution
       ├── file changes
       ├── approvals
       ├── subagents
       └── final response
       │
       ▼
    CodexEventMapper
       │
       ▼
    Hero Events
       │
       ▼
    Hero TUI / Cycle Engine

---

## 23. Princípios do adapter

- Hero continua sendo o Orchestrator.
- Codex continua sendo o Harness.
- Core do Hero não conhece detalhes do protocolo Codex.
- Eventos Codex são traduzidos para eventos internos Hero.
- Eventos desconhecidos nunca são ignorados silenciosamente.
- Processos Codex nunca ficam órfãos.
- Capabilities são detectadas dinamicamente.
- Features experimentais são isoladas.
- Sessões do Hero devem sobreviver ao restart do adapter quando possível.
- Aprovações continuam sob controle do Hero.
- Skills do Hero não devem ser duplicadas sem uma estratégia explícita.
- Não utilizar API Key quando o objetivo for preservar a autenticação/subsídio da conta ChatGPT.
- O adapter deve ser resiliente a mudanças de versão do Codex.

## 24. Modo debug:

Na primeira versão, todos os eventos /mensagens devem ser exibidas na tela de resposta do hero. Posteriormente em uma nova sessão de desenvolvimento serão escolhidos as mensagens e eventos que serão exibidas apenas quando o hero for executadp em modo debug.

## Resultado esperado

O usuário configura:

    harness: codex
    model: <modelo>

e o Hero passa a tratar o Codex da mesma forma conceitual que trata Cursor e OpenCode:

    Hero
      │
      ├── Ciclo
      │    ├── Etapa
      │    ├── Etapa
      │    └── Etapa
      │
      └── HarnessAdapter
           └── CodexAdapter
                └── Codex App Server
                     └── Codex / ChatGPT Account

O objetivo final é que nenhuma funcionalidade específica do Codex atravesse o core do Hero: toda a adaptação deve permanecer encapsulada no `CodexAdapter`.