# AI Workflow Hero — Integração com Telegram

## 1. Objetivo

Adicionar suporte ao Telegram como uma **interface remota do AI Workflow Hero**, permitindo que o usuário interaja com o Hero pelo Telegram da mesma forma que já pode interagir pela TUI.

O Telegram deve ser **bidirecional**:

- O usuário pode **enviar mensagens/comandos para o Hero pelo Telegram**.
- O Hero pode **enviar mensagens, eventos, status, solicitações de aprovação e resultados para o Telegram**.
- O Telegram não substitui a TUI.
- TUI e Telegram devem operar sobre o **mesmo estado do Hero**, podendo inclusive ser utilizados simultaneamente durante o mesmo Ciclo.



### Objetivo do V0

Para evitar complexidade desnecessária, o V0 terá:

- Um único Telegram Bot.
- Uma única instância do Hero Runtime conectada ao bot por vez.
- Nenhum processo Gateway adicional.
- Nenhum serviço externo intermediário.
- O próprio Hero Runtime será responsável pela integração com a Telegram Bot API.

A arquitetura deve, entretanto, ser projetada para permitir múltiplas instâncias e múltiplos projetos no futuro sem necessidade de reescrever o Conversation Service.

---



# 2. Terminologia oficial

É importante manter estes termos de forma consistente no projeto.

### Ciclo

Um processo completo de execução do Hero.

Exemplo:

```text
Preparation
    ↓
Research
    ↓
Plan
    ↓
Implementation
    ↓
QA
    ↓
UI QA
    ↓
E2E
    ↓
Completed
```

Isso é um Ciclo.

Nunca chamar um Ciclo de "Loop".

### Etapa

Cada estágio individual de um Ciclo.

Exemplos:

- Preparation
- Research
- Plan
- Implementation
- QA
- UI QA
- E2E



### Free Chat

Conversa direta do usuário com o Harness, sem um Ciclo ativo.
É equivalente ao Free Chat que já existe atualmente na TUI.

Exemplo:

```text
Usuário:
Em qual projeto você está trabalhando?

Harness:
Estou trabalhando no AI Workflow Hero...
```



### Cycle Chat

Conversa relacionada a um Ciclo/Etapa específica.

Quando existe um Ciclo ativo, o usuário pode enviar mensagens relacionadas à execução e também conversar com o Harness associado à Etapa atual.

# 3. Arquitetura atual do Hero

A arquitetura existente é aproxidamante:



```text

Hero TUI (Bubble Tea)

│

├───────────────────────┬───────────────────────┐
│                       │                       │
CICLO DE EXECUÇÃO      CONVERSATION (UI)         OUTRAS TELAS
(state machine)        (tui/conversation.go)    Status · Approvals · …
│                       │
│                       │
cycle.Service              syncConversationContext()
│                       │
├────┬────┐             ├────┬────┐
│    │    │             │         │
engine store.Store   FREE CHAT   CYCLE CHAT
       (hero.db)       stage=""    stage≠""
                          │           │
                          │     ConversationContext()
                          │     + harness_session_id
                          │           │
                          └─────┬─────┘
                                │
                     HarnessAdapter.Execute (stream)
                                │
                         Cursor Adapter
                                │
                         cursor-agent CLI

Run / Dispatch ──► HarnessAdapter.Dispatch

(palette /hero-start, /hero-sync, key `d`)

```

O objetivo da integração com Telegram não é redesenhar essa arquitetura.

A principal alteração será separar a lógica de Conversation da implementação específica da TUI.


# 4. Arquitetura proposta

A arquitetura deve evoluir para:

```text

                         HERO RUNTIME
                              │
             ┌────────────────┴────────────────┐
             │                                 │
            TUI                            TELEGRAM
       (Bubble Tea)                     (Bot API Client)
             │                                 │
             └────────────────┬────────────────┘
                              │
                     Conversation Service
                              │
                 ┌────────────┴────────────┐
                 │                         │
             FREE CHAT                CYCLE CHAT
             stage=""                stage != ""
                 │                         │
                 └────────────┬────────────┘
                              │
                     ConversationContext
                              │
                              ▼
                      HarnessAdapter
                              │
                       Cursor Adapter
                              │
                       cursor-agent CLI

```

O princípio fundamental é:

> TUI e Telegram são apenas interfaces diferentes para o mesmo Conversation Service e para o mesmo estado do Hero Runtime.

Não devem existir implementações diferentes da lógica de conversação para TUI e Telegram.


# 5. Conversation Layer

A lógica atualmente existente em:

```bash
tui/conversation.go
```
deve ser gradualmente extraída para uma camada independente da TUI.

Uma estrutura possível:

```bash
conversation/
├── service.go
├── context.go
├── message.go
├── session.go
└── types.go
```

A TUI passa a utilizar esse serviço:

```bash
TUI
 │
 ▼
ConversationService
 │
 ▼
HarnessAdapter
``` 

E o Telegram também:

```bash
Telegram
 │
 ▼
ConversationService
 │
 ▼
HarnessAdapter
```

# 6. Telegram Adapter

O Telegram Adapter será implementado dentro do próprio Hero Runtime.

Não criar um Gateway separado no V0.

Arquitetura:
```bash
Hero Runtime
│
├── Cycle Service
├── Conversation Service
├── Harness Adapter
├── Event System
├── State / Store
└── Telegram Adapter
       │
       ▼
Telegram Bot API
``` 

O Telegram Adapter será responsável por:

- Conectar ao Telegram Bot API.
- Receber mensagens.
- Receber comandos.
- Receber callbacks de botões inline.
- Converter mensagens Telegram em eventos/mensagens do Hero.
- Enviar mensagens do Hero para o Telegram.
- Enviar status.
- Enviar notificações.
- Enviar solicitações de aprovação.
- Enviar resultados de Etapas.
- Enviar erros.
- Manter o vínculo entre o usuário Telegram e a instância atual do Hero.

O Telegram Adapter não deve implementar lógica de negócio do Ciclo.


# 7. O Telegram é bidirecional

Este requisito é fundamental.

## Telegram → Hero

O usuário pode enviar:

```code 
/hero-status
```
ou

```code 
/hero-approve
```
ou uma mensagem livre

```code 
Quero que a implementação use PostgreSQL
```

ou uma mensagem para o Free Chat

```code 
Em qual projeto vc está trabalhando?
```
ou uma mensagem durante um Ciclo

```code 
Não use Redis nesta implementação
```

Essas mensagens entram no Hero através do Conversation Service ou Cycle Service, conforme o contexto.

## Hero → Telegram

O Hero também deve enviar mensagens ao Telegram.

Exemplos:

```text
Ciclo #42 iniciado.

Etapa:
Research

Harness:
Cursor
```
ou

```text
Etapa Plan concluída.

Custo: $0.42
Duração: 18m

Aguardando aprovação.
```
ou

```text
QA falhou.

3 problemas encontrados:
- ...
- ...
```
ou

```text
Ciclo #42 concluído.

Custo total: $4.82
Duração: 1h 37m
``` 

Portanto, Telegram não é somente um canal de comandos. Ele é uma interface remota bidirecional do Hero.

## 8. V0 — Limitação deliberada

No V0 haverá:

```text
1 Telegram Bot
        │
        ▼
1 Hero Runtime
        │
        ▼
1 Project
``` 

Não haverá:

```text
Telegram
   │
   ├── Gateway
   ├── Runtime Registry
   ├── Hero #1
   ├── Hero #2
   └── Hero #3
```

O objetivo é validar a experiência de interação remota antes de introduzir infraestrutura adicional.

# 9. Configuração do Telegram 

O token do BotFather e o chatId não devem ser armazenado no código-fonte, neste primeiro momento eles devem ser configurados através de um slash command /hero-telegram-config e devem ser salvos no SQLite. 
O usuário deve ter a opção de excluir o token e o chatId e também enviar uma mensagem de teste, na mesma caixa de configuração.

No código deve ser tratado como uma váriável de ambiante

```bash
export HERO_TELEGRAM_BOT_TOKEN="..."
export HERO_TELEGRAM_CHAT_ID="..."
```

O token nunca deve:

- entrar no contexto do Harness;
- ser enviado para o agente;
- ser armazenado em arquivos de workflow;
- aparecer em logs;
- aparecer em mensagens do Telegram;
- aparecer em mensagens de erro.


# 10. Segurança

O Hero deve validar o chat_id do Telegram autorizado.

Mensagens de usuários não autorizados devem ser ignoradas ou rejeitadas sem revelar informações sobre o projeto.

O token do BotFather é uma credencial sensível.

O Telegram Adapter deve ser o único componente que conhece essa credencial.

```text
Telegram Bot Token
       │
       ▼
Telegram Adapter
       │
       ├── Conversation Service
       ├── Cycle Service
       └── Event System
```

Nunca

```text
Telegram Bot Token
       │
       ▼
Harness / Agent
```

# 11. Free Chat

O Free Chat já existe na TUI e deve continuar existindo.

O Telegram deverá permitir entrar em um Free Chat existente ou criar um novo.

Exemplo conceitual:

```text
/free-chat
``` 
Resposta:

```text
Free Chat

Harness: Cursor

[New Session]
[Continue Existing]
```

Depois:

```text
Usuário:
Em qual projeto você está trabalhando?
```

Fluxo:

```text
Telegram
   ↓
Telegram Adapter
   ↓
Conversation Service
   ↓
Free Chat
   ↓
HarnessAdapter
   ↓
Cursor Adapter
   ↓
cursor-agent
```

O resultado retorna pelo caminho inverso:

```text
cursor-agent
   ↓
Cursor Adapter
   ↓
HarnessAdapter
   ↓
Conversation Service
   ↓
Telegram Adapter
   ↓
Telegram
``` 

# 12. Cycle Chat

Quando existe um ciclo ativo

```text
Cycle #42
    │
    └── Etapa: Implementation
              │
              └── harness_session_id
```

O Telegram deve permitir interação com o contexto do Ciclo/Etapa.

Exemplo:

```text
/hero-status
```

Resposta:

```text
Cycle #42

Preparation      ✓
Research         ✓
Plan             ✓
Implementation   ▶
QA               ○
UI QA            ○
E2E              ○

Harness: Cursor
Status: Running
```

O usuário pode então enviar:

```text
Por que você escolheu esta abordagem?
```

A mensagem será associada ao contexto do Ciclo/Etapa atual e poderá ser encaminhada para a sessão do Harness correspondente.


# 13. TUI e Telegram simultaneamente

Isso é um requisito importante.

O usuário pode iniciar um Ciclo na TUI e continuar pelo Telegram.

Ou iniciar pelo Telegram e acompanhar pela TUI.

Exemplo:

```text
                    Hero Runtime
                         │
              ┌──────────┴──────────┐
              │                     │
             TUI                 Telegram
              │                     │
              └──────────┬──────────┘
                         │
                 Same Hero State
                         │
                      Cycle #42
``` 

Se o usuário aprovar pela TUI:

```text
TUI
 ↓
/hero-approve
 ↓
Cycle Service
 ↓
Cycle continues
 ↓
Telegram notification
```

Se aprovar pelo Telegram:

```text
Telegram
 ↓
/hero-approve
 ↓
Cycle Service
 ↓
Cycle continues
 ↓
TUI updates
``` 

Nunca devem existir dois estados diferentes.


# 14. Comandos no Telegram

Os comandos atuais da TUI devem ser preservados conceitualmente.

## Navegação

```text
/status
/approvals
/artifacts
/costs
/events
```