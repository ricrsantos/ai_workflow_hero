# 03 - Runtime

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero AI Loop Runtime
> **Idioma:** Português

---

# 1. Visão Geral

O Hero Runtime é o processo central responsável por executar o AI Development Loop.

É o motor de execução da aplicação e o dono do ciclo de vida do fluxo de trabalho.

O Runtime é responsável por coordenar todos os subsistemas internos, permanecendo completamente independente de qualquer harness de codificação com IA.

O Runtime **não** é um agente de IA.

É um motor de orquestração.

---

# 2. Responsabilidades

O Hero Runtime é responsável por:

* inicializar a aplicação;
* carregar a configuração;
* gerenciar instâncias de fluxo de trabalho;
* orquestrar os estágios do fluxo de trabalho;
* coordenar agentes de IA;
* comunicar-se com adaptadores de harness;
* gerenciar interações com o usuário;
* receber eventos externos;
* persistir o estado do fluxo de trabalho;
* coletar métricas de execução;
* rastrear custos;
* gerenciar artefatos;
* expor o status do runtime;
* lidar com desligamento e recuperação controlados.

O Runtime é dono do ciclo de vida de execução completo.

---

# 3. Arquitetura do Runtime

```
                         Hero Runtime
                               │
        ┌──────────────────────┼──────────────────────┐
        │                      │                      │
        ▼                      ▼                      ▼
 Configuração          Workflow Engine        Barramento de Eventos
        │                      │                      │
        ▼                      ▼                      ▼
 State Store          Agent Orchestrator      Gerenciador de Notificações
        │                      │                      │
        ▼                      ▼                      ▼
 Artifact Store        Harness Adapter       Camada de Conversação
        │                      │                      │
        └──────────────────────┼──────────────────────┘
                               │
                               ▼
                        Harness Externo
```

---

# 4. Ciclo de Vida do Runtime

O Runtime segue um ciclo de vida previsível.

```
Iniciar
  │
  ▼
Carregar Configuração
  │
  ▼
Inicializar Componentes
  │
  ▼
Restaurar Estado
  │
  ▼
Iniciar Barramento de Eventos
  │
  ▼
Iniciar Serviços
  │
  ▼
Pronto
  │
  ▼
Em Execução
  │
  ▼
Desligamento
```

O Runtime deve sempre ser capaz de reiniciar sem perder informações do fluxo de trabalho.

---

# 5. Sequência de Inicialização

Durante a inicialização, o Runtime executa as seguintes etapas.

## Etapa 1

Carregar configuração.

Exemplos:

* configuração do runtime;
* configuração do fluxo de trabalho;
* configuração do harness;
* integrações;
* preferências do usuário.

---

## Etapa 2

Inicializar infraestrutura.

Exemplos:

* logger;
* state store;
* artifact store;
* barramento de eventos;
* rastreador de custos.

---

## Etapa 3

Carregar estado persistido.

Se houver instâncias de fluxo de trabalho não concluídas, elas devem ser restauradas.

---

## Etapa 4

Inicializar canais de comunicação.

Exemplos:

* TUI;
* CLI;
* Telegram;
* Discord;
* futura Web API.

---

## Etapa 5

Inicializar os adaptadores de harness.

Cada harness configurado se torna disponível para execução.

---

## Etapa 6

O Runtime entra no estado **Pronto**.

---

# 6. Estado do Runtime

O próprio Runtime possui um estado de execução independente do estado do fluxo de trabalho.

Possíveis estados do runtime:

```
Iniciando

Pronto

Em Execução

Pausado

Parando

Parado

Recuperando

Falhou
```

Esses estados descrevem o Runtime, não os fluxos de trabalho individuais.

---

# 7. Componentes do Runtime

O Runtime é composto por subsistemas independentes.

---

## 7.1 Configuration Manager

Responsabilidades:

* carregar arquivos de configuração;
* validar a configuração;
* expor as configurações do runtime;
* suportar sobrescritas por variáveis de ambiente.

O Configuration Manager é inicializado apenas uma vez.

---

## 7.2 Workflow Engine

Responsável por:

* executar fluxos de trabalho;
* gerenciar transições de estágio;
* agendar agentes;
* lidar com aprovações.

O Workflow Engine é o coração do Runtime.

---

## 7.3 Agent Orchestrator

Responsável por:

* selecionar agentes;
* preparar o contexto de execução;
* invocar adaptadores de harness;
* receber resultados de execução.

O Orchestrator nunca se comunica diretamente com os usuários.

---

## 7.4 Camada de Conversação

Responsável pela interação com o usuário.

As responsabilidades incluem:

* exibir perguntas;
* receber respostas;
* apresentar resumos;
* solicitar aprovações;
* notificar o progresso.

A Camada de Conversação é independente do Workflow Engine.

---

## 7.5 Harness Adapter Manager

Responsável por:

* registrar adaptadores;
* selecionar adaptadores;
* invocar a execução;
* monitorar a execução;
* coletar informações de uso.

Adaptadores suportados podem incluir:

* Cursor Adapter;
* OpenCode Adapter;
* Codex Adapter
* Claude Code Adapter.

---

## 7.6 Barramento de Eventos

Responsável pela comunicação interna.

Todo subsistema se comunica por meio de eventos sempre que possível.

O Barramento de Eventos reduz o acoplamento entre componentes.

---

## 7.7 State Store

Responsável por:

* persistência do fluxo de trabalho;
* persistência do runtime;
* persistência de sessão;
* informações de recuperação.

A persistência de estado é obrigatória.

---

## 7.8 Artifact Store

Responsável por gerenciar as saídas do fluxo de trabalho.

Exemplos:

* especificações;
* relatórios;
* screenshots;
* documentação gerada;
* logs de execução.

Os artefatos pertencem aos fluxos de trabalho.

---

## 7.9 Cost Tracker

Da mesma que está implementado atualmente no hero:

Responsável por:

* contabilização de tokens;
* rastreamento de duração;
* cálculo de custo estimado;
* resumos de custo do fluxo de trabalho.

Toda execução contribui para as estatísticas do fluxo de trabalho.

---

## 7.10 Notification Manager

Responsável por enviar notificações.

Possíveis destinos:

* Hero TUI;
* Telegram;
* Discord;
* futuras integrações.

As notificações são geradas a partir de eventos do runtime.

---

# 8. Propriedade dos Ciclos (Fluxos) de Trabalho

O Runtime é dono de todo ciclo de trabalho ativo.

```
Hero Runtime
    │
    ├── Ciclo de Trabalho A
    ├── Ciclo de Trabalho B
    ├── Ciclo de Trabalho C
    └── Ciclo de Trabalho D
```

Inicialmente, apenas um único ciclo de trabalho ativo pode ser suportado.

Versões futuras podem suportar ciclos de trabalho concorrentes.

---

# 9. Modelo de Comunicação

A comunicação entre componentes segue uma arquitetura em camadas.

```
Usuário
   │
   ▼
Camada de Conversação
   │
   ▼
Workflow Engine
   │
   ▼
Agent Orchestrator
   │
   ▼
Harness Adapter
   │
   ▼
Harness
```

Camadas superiores nunca contornam camadas inferiores.

---

# 10. Fluxo de Eventos do Runtime

Exemplo:

```
Usuário aprova o planejamento

      │

      ▼

Camada de Conversação

      │

      ▼

approval.received

      │

      ▼

Barramento de Eventos

      │

      ▼

Workflow Engine

      │

      ▼

Planejamento Concluído

      │

      ▼

Implementação Iniciada
```

Toda ação importante produz um evento.

---

# 11. Estrutura de Diretórios do Runtime

Prioritáriamente seguir a estrutura atual de diretórios do hero, adaptando a seguinte sugestão:

```text
ai_workflow_hero/

    runtime/

    workflow/

    harness/

    adapters/

    conversation/

    events/

    state/

    artifacts/

    integrations/

    tui/

    config/
```

Cada pacote deve ter uma única responsabilidade.

---

# 12. Processo de Longa Duração

O Runtime foi projetado para ser executado continuamente.

Características:

* baixo uso de memória;
* baixo uso de CPU enquanto ocioso;
* execução orientada a eventos;
* estado persistente;
* interrupção controlada;
* execução retomável.

O Runtime deve permanecer ativo mesmo enquanto aguarda a aprovação do usuário.

---

# 13. Interação com o Usuário

O Runtime não interage diretamente com os usuários.

Em vez disso:

```
Runtime

↓

Camada de Conversação

↓

Interface

↓

Usuário
```

Possíveis interfaces:

* TUI;
* CLI;
* Telegram;
* Discord;
* Web UI.

O Runtime permanece alheio ao canal de interação.

---

# 14. Independência de Harness

O Runtime nunca deve depender de uma implementação de harness específica.

Arquitetura correta:

```
Runtime

↓

Harness Interface

↓

Cursor Adapter

↓

Cursor
```

Arquitetura incorreta:

```
Runtime

↓

Cursor API
```

O Runtime depende apenas de abstrações.

---

# 15. Tratamento de Erros

Todo subsistema relata falhas por meio de eventos.

Exemplos:

```
agent.failed

harness.failed

workflow.failed

approval.timeout

runtime.failed
```

O Runtime decide como as falhas são tratadas.

Ações possíveis:

* repetir;
* pausar;
* cancelar;
* recuperar.

---

# 16. Recuperação

Se o Runtime falhar, ele deve se recuperar automaticamente.

Etapas de recuperação:

1. carregar o estado persistido;
2. identificar fluxos de trabalho ativos;
3. restaurar aprovações pendentes;
4. reconectar os adaptadores;
5. retomar a execução.

A recuperação não deve exigir intervenção do usuário sempre que possível.

---

# 17. Desligamento Controlado

Ao desligar o Runtime:

1. parar de aceitar novos trabalhos;
2. finalizar as operações de persistência em andamento;
3. descarregar os eventos pendentes;
4. persistir o estado do runtime;
5. desconectar os adaptadores;
6. encerrar com segurança.

Nenhum estado de fluxo de trabalho deve ser perdido.

---

# 18. Modelo de Concorrência

O Runtime deve aproveitar o modelo de concorrência do Go.

Exemplos de goroutines independentes:

* Barramento de Eventos;
* Camada de Conversação;
* Notification Manager;
* Harness Execution Monitor;
* Workflow Engine;
* Cost Tracker.

A comunicação entre goroutines deve favorecer channels em vez de estado compartilhado.

---

# 19. Evolução Futura

A arquitetura do Runtime deve suportar capacidades futuras sem grandes redesenhos.

Exemplos:

* múltiplos fluxos de trabalho simultâneos;
* execução distribuída;
* workers remotos;
* sistema de plugins;
* harnesses adicionais;
* aplicação desktop;
* sincronização em nuvem.

Essas capacidades devem ser habilitadas estendendo componentes, em vez de reescrever o Runtime.

---

# 20. Princípios do Runtime

O Runtime segue estes princípios.

## Dono Único

O Runtime é dono do ciclo de vida de execução.

---

## Interfaces Sem Estado

As interfaces externas não devem ser donas do estado do fluxo de trabalho.

---

## Núcleo Persistente

O estado do fluxo de trabalho sempre sobrevive a reinicializações do processo.

---

## Orientado a Eventos

Os componentes se comunicam principalmente por meio de eventos.

---

## Independente de Harness

Os harnesses são motores de execução intercambiáveis.

---

## Agnóstico de Interface

O Runtime não sabe se o usuário está interagindo por meio de um TUI, Telegram, Discord ou uma futura interface Web.

---

## Recuperável

Todo fluxo de trabalho pode ser retomado após uma interrupção.

---

# 21. Declaração de Arquitetura

O Hero Runtime é um motor de orquestração de longa duração e orientado a eventos, que é dono do ciclo de vida completo do desenvolvimento de software assistido por IA.

Ele coordena fluxos de trabalho, gerencia estado, orquestra agentes, comunica-se com harnesses intercambiáveis e fornece uma plataforma de execução estável, independente de qualquer ferramenta de codificação com IA específica.