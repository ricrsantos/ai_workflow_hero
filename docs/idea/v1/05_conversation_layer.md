# 05 - Camada de Conversação

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero AI Loop Runtime
> **Idioma:** Português

---

# 1. Visão Geral

A Camada de Conversação é a interface de comunicação entre o usuário e o Hero Runtime.

Sua principal responsabilidade é fornecer uma experiência conversacional unificada, independentemente do canal de comunicação subjacente.

A Camada de Conversação é dona de toda interação com o usuário.

O Workflow Engine nunca se comunica diretamente com os usuários.

Da mesma forma, interfaces externas nunca se comunicam diretamente com o Workflow Engine.

A Camada de Conversação atua como a ponte entre os dois mundos.

---

# 2. Visão

Um dos princípios arquiteturais centrais para a V1 do Hero, fora da IDE do cursor é:

> **Os usuários interagem com o Hero, não com o harness de IA.**

A Camada de Conversação torna isso possível.

Independentemente de um agente de IA ser executado usando Cursor CLI, OpenCode, Claude Code ou outro harness futuro, a experiência do usuário permanece idêntica.

O harness se torna um detalhe de implementação.

---

# 3. Responsabilidades

A Camada de Conversação é responsável por:

* apresentar informações do fluxo de trabalho;
* fazer perguntas;
* coletar respostas do usuário;
* exibir o progresso da execução;
* apresentar resumos de estágio;
* solicitar aprovações;
* entregar notificações;
* receber comandos;
* encaminhar decisões do usuário;
* normalizar a comunicação vinda de múltiplas interfaces.

Ela nunca executa fluxos de trabalho.

Ela nunca se comunica diretamente com harnesses.

---

# 4. Arquitetura de Alto Nível

```text
                        Usuário
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ▼                 ▼                 ▼
      Hero TUI       Telegram        Futura Web UI
        │                 │                 │
        └─────────────────┼─────────────────┘
                          │
                          ▼
                 Camada de Conversação
                          │
                          ▼
                     Barramento de Eventos
                          │
                          ▼
                   Workflow Engine
```

Todo canal de comunicação compartilha a mesma Camada de Conversação.

---

# 5. Princípios de Design

A Camada de Conversação segue estes princípios.

## Independente de Canal

A interação do usuário é independente do canal de comunicação.

---

## Independente do Ciclo de Trabalho

A Camada de Conversação não entende a lógica do ciclo de trabalho.

Ela apenas apresenta informações e coleta a entrada do usuário.

---

## Orientada a Eventos

Toda interação produz um evento.

---

## Interface Sem Estado

Os canais de comunicação permanecem leves.

O estado persistente da conversa pertence ao Runtime.

---

## Experiência Consistente

Os usuários recebem a mesma experiência independentemente da interface.

---

# 6. Canais de Comunicação

A implementação inicial deve suportar:

* Hero TUI

Implementações futuras podem incluir:

* Hero CLI
* Telegram
* Discord
* Painel Web
* Aplicação Desktop
* Aplicações Móveis

Toda interface implementa o mesmo contrato de comunicação.

---

# 7. Tipos de Interação com o Usuário

A Camada de Conversação suporta várias categorias de interação.

---

## Mensagens Informativas

Exemplo:

```text
Fluxo de trabalho iniciado.

Carregando projeto...

Preparando ambiente de execução...
```

Essas mensagens não exigem nenhuma ação do usuário.

---

## Perguntas

Exemplo:

```text
Qual problema estamos tentando resolver?
```

As perguntas esperam respostas estruturadas do usuário.

---

## Aprovações

Exemplo:

```text
O planejamento foi concluído.

Continuar?

[Aprovar]

[Solicitar Alterações]

[Cancelar]
```

As aprovações sempre geram eventos do fluxo de trabalho.

---

## Notificações

Exemplo:

```text
QA concluído com sucesso.
```

As notificações não exigem respostas.

---

## Atualizações de Progresso

Exemplo:

```text
Estágio Atual

Implementação

Progresso

42%
```

As informações de progresso mantêm os usuários informados sem interromper a execução.

---

# 8. Fluxo de Conversação

Exemplo:

```text
Workflow Engine

        │

approval.required

        │

        ▼

Camada de Conversação

        │

        ▼

Exibir Resumo

        │

        ▼

Usuário

        │

Aprovar

        │

        ▼

Camada de Conversação

        │

approval.received

        │

        ▼

Workflow Engine
```

O Workflow Engine permanece alheio à interface do usuário.

---

# 9. Conversas de Pesquisa

A Pesquisa é o estágio de fluxo de trabalho mais conversacional.

Exemplo:

```text
Hero

Qual problema estamos resolvendo?

Usuário

Quero criar uma plataforma de monitoramento de ações.

Hero

Quem são os usuários principais?

Usuário

Investidores individuais.

Hero

O que diferencia essa plataforma?
```

Internamente:

```text
Camada de Conversação

↓

Discover Agent

↓

Harness

↓

Resposta

↓

Camada de Conversação
```

O usuário nunca se comunica diretamente com o harness.

---

# 10. Resumos de Etapa

Toda etpa concluída gera um resumo padronizado, da memsa forma que está implementado atualmente no Hero.

Os resumos são gerados pelo Runtime e apresentados pela Camada de Conversação.

---

# 11. Solicitações de Aprovação

As aprovações seguem os padrões atuais.

A interface pode diferir visualmente, mas a semântica permanece idêntica entre os canais.

---

# 12. Comandos do Usuário

A Camada de Conversação aceita comandos de alto nível, assim como os atuais comandos /hero-xxx

Os comandos são traduzidos em eventos do runtime.

---

# 13. Tradução de Eventos

As ações recebidas do usuário se tornam eventos normalizados.

Exemplos:

```text
Botão Aprovar

↓

approval.received
```

```text
Mensagem do Telegram

↓

approval.received
```

```text
Comando do TUI

↓

approval.received
```

O Workflow Engine não consegue determinar a origem do evento.

Essa abstração é intencional.

---

# 14. Visualização de Progresso

A Camada de Conversação apresenta o progresso do fluxo de trabalho.

Exemplo:

```text
Fluxo de Trabalho

Stock Hero

Estágio Atual

Implementação

Concluído

Pesquisa

Planejamento

Atual

Implementação

Restante

QA

UI QA

E2E
```

O formato de apresentação depende da interface.

---

# 15. Notificações do Runtime

O Runtime gera notificações.

Exemplos:

```text
Ciclo de Trabalho Iniciado

Etapa Concluído

Aprovação Necessária

Ciclo de Trabalho Pausado

Ciclo de Trabalho Retomado

Ciclo de Trabalho Falhou

Ciclo de Trabalho Concluído
```

A Camada de Conversação decide como essas notificações são apresentadas.

---

# 16. Múltiplas Interfaces

Versões futuras podem suportar interfaces simultâneas.

Exemplo:

```text
                Camada de Conversação

        ┌───────────┼───────────┐
        │           │           │
        ▼           ▼           ▼
      Hero TUI   Telegram   Painel Web
```

Um usuário pode aprovar um fluxo de trabalho pelo Telegram enquanto monitora o progresso no TUI.

O Workflow Engine recebe apenas um evento normalizado.

---

# 17. Contexto da Conversa

O contexto da conversa pertence ao Hero.

É independente da sessão do harness de IA.

O histórico de conversas pode incluir:

* perguntas do usuário;
* respostas do usuário;
* aprovações;
* notificações;
* resumos;
* decisões.

Esse histórico pertence ao Runtime.

---

# 18. Independência de Interface

O Runtime se comunica usando solicitações de conversa abstratas.

Exemplo:

```text
Solicitar Aprovação

↓

Camada de Conversação

↓

Interface Atual

↓

Usuário
```

O Runtime nunca formata mensagens para uma interface específica.

A apresentação é delegada.

---

# 19. Apresentação de Erros

Os erros devem ser traduzidos em mensagens amigáveis para o usuário.

Exemplo:

Evento interno:

```text
agent.execution.failed
```

Exibido ao usuário:

```text
O Planning Agent falhou durante a execução.

Deseja tentar novamente?

[Tentar Novamente]

[Cancelar]
```

Detalhes internos de implementação devem permanecer ocultos.

---

# 20. Conversas de Longa Duração

A Camada de Conversação suporta fluxos de trabalho que se estendem por horas ou dias.

As capacidades incluem:

* reconectar usuários;
* restaurar o histórico da conversa;
* retomar aprovações pendentes;
* exibir o estado atual do fluxo de trabalho.

Os usuários devem sempre entender o status atual do fluxo de trabalho.

---

# 21. Futuro Chat com IA

Embora a implementação inicial se concentre na interação com o fluxo de trabalho, a arquitetura deve suportar capacidades conversacionais futuras.

Exemplos:

```text
Hero,

Por que o QA falhou?

---

Explique a arquitetura gerada pelo Planejamento.

---

Mostre o resumo da implementação.

---

Compare este fluxo de trabalho com a execução anterior.
```

Essas capacidades devem aproveitar o conhecimento do fluxo de trabalho do Runtime, em vez de depender da memória do harness.

---

# 22. Separação de Responsabilidades

A Camada de Conversação:

* interage com os usuários;
* traduz comandos;
* exibe informações.

O Workflow Engine:

* toma decisões;
* controla o progresso do fluxo de trabalho.

O Agent Orchestrator:

* executa agentes.

O Harness:

* realiza tarefas de IA.

As responsabilidades devem permanecer isoladas.

---

# 23. Futuros Canais de Comunicação

A arquitetura deve permitir novas interfaces sem modificar a lógica do fluxo de trabalho.

Possíveis integrações futuras:

* Slack
* Microsoft Teams
* E-mail
* Assistentes de Voz
* REST API
* Clientes WebSocket

Apenas novos adaptadores de interface devem ser necessários.

---

# 24. Princípios da Camada de Conversação

A Camada de Conversação segue estes princípios.

## O Hero é Dono da Conversa

O usuário sempre se comunica com o Hero.

---

## Independente de Canal

O canal de comunicação é transparente para o Runtime.

---

## Orientada a Eventos

Toda interação se torna um evento do runtime.

---

## Interfaces Sem Estado

As interfaces permanecem camadas de apresentação.

---

## Experiência Unificada

A experiência do usuário permanece consistente em todas as interfaces.

---

## Runtime em Primeiro Lugar

O estado da conversa pertence ao Hero Runtime, nunca ao harness.

---

# 25. Declaração de Arquitetura

A Camada de Conversação é a abstração de comunicação do Hero Runtime.

Ela fornece uma experiência conversacional unificada em múltiplas interfaces, traduzindo interações do usuário em eventos do runtime, enquanto oculta a complexidade dos agentes de IA, da execução do fluxo de trabalho e das implementações de harness.

Por meio da Camada de Conversação, o Hero se torna o ponto único de interação entre os usuários e o AI Development Loop.