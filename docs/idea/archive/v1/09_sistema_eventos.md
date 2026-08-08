# 09 - Sistema de Eventos

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero AI Loop Runtime
> **Idioma:** Português

---

# 1. Visão Geral

O Sistema de Eventos é a espinha dorsal de comunicação do Hero Runtime.

Em vez de permitir que os componentes se comuniquem diretamente, o Hero segue uma **arquitetura orientada a eventos**, na qual toda ação importante é representada como um evento imutável.

Essa abordagem minimiza o acoplamento entre subsistemas, ao mesmo tempo em que melhora a escalabilidade, a extensibilidade, a capacidade de recuperação e a observabilidade.

O Sistema de Eventos é inteiramente interno ao Hero Runtime.

Ele **não** é um message broker distribuído.

---

# 2. Visão

Toda ação significativa dentro do Hero deve produzir um evento.

Exemplos incluem:

* criação de ciclo de trabalho;
* conclusão de etapa;
* solicitações de aprovação;
* decisões do usuário;
* execução de agente;
* falhas de harness;
* geração de artefatos;
* conclusão do ciclo de trabalho.

O Sistema de Eventos se torna o sistema nervoso central do Runtime.

---

# 3. Responsabilidades

O Sistema de Eventos é responsável por:

* publicar eventos;
* entregar eventos;
* rotear eventos;
* persistir eventos;
* notificar assinantes (subscribers);
* manter a ordem de execução;
* possibilitar a recuperação.

Ele **não** é responsável por:

* execução do ciclo de trabalho;
* decisões de negócio;
* interação com o usuário;
* execução de IA.

---

# 4. Arquitetura

```text
                      Hero Runtime

                            │

      ┌─────────────────────┼─────────────────────┐

      ▼                     ▼                     ▼

 Workflow Engine     Camada de Conversação     Cost Tracker

      │                     │                     │

      └─────────────────────┼─────────────────────┘

                            ▼

                      Event Bus

                            │

      ┌─────────────────────┼─────────────────────┐

      ▼                     ▼                     ▼

State Management     Notification Manager   Artifact Store
```

O Event Bus é o hub de comunicação do Runtime.

---

# 5. Princípios de Design

O Sistema de Eventos segue estes princípios.

## Orientado a Eventos

Toda ação significativa produz um evento.

---

## Imutável

Os eventos nunca são modificados após a publicação.

---

## Ordenado

Os eventos são entregues na ordem em que ocorrem.

---

## Observável

Toda ação importante do Runtime é visível por meio de eventos.

---

## Persistente

Os eventos se tornam parte do histórico do ciclo de trabalho.

---

## Fracamente Acoplado

Os componentes se comunicam por meio de eventos, em vez de dependências diretas.

---

# 6. Ciclo de Vida do Evento

Todo evento segue o mesmo ciclo de vida.

```text
Criar Evento

      │

      ▼

Publicar

      │

      ▼

Persistir

      │

      ▼

Distribuir

      │

      ▼

Consumir

      │

      ▼

Concluir
```

Os eventos são imutáveis durante todo o seu ciclo de vida.

---

# 7. Estrutura do Evento

Modelo de evento sugerido:

```go
type Event struct {

    ID string

    WorkflowID string

    Stage string

    Type string

    Timestamp time.Time

    Source string

    Payload any

}
```

A estrutura deve permanecer estável entre as versões do Runtime.

---

# 8. Categorias de Eventos

Os eventos podem ser agrupados em categorias.

Eventos de Ciclo de Trabalho.

Eventos de Etapa.

Eventos de Agente.

Eventos de Conversação.

Eventos de Aprovação.

Eventos de Harness.

Eventos de Artefato.

Eventos de Custo.

Eventos de Runtime.

Cada categoria tem um propósito definido.

---

# 9. Eventos de Ciclo de Trabalho

Exemplos:

```text
workflow.created

workflow.started

workflow.paused

workflow.resumed

workflow.completed

workflow.cancelled

workflow.failed
```

Os eventos de ciclo de trabalho descrevem o ciclo de vida de um ciclo de trabalho inteiro.

---

# 10. Eventos de Etapa

Exemplos:

```text
stage.created

stage.started

stage.completed

stage.failed

stage.cancelled
```

Esses eventos descrevem o progresso da etapa.

---

# 11. Eventos de Agente

Exemplos:

```text
agent.started

agent.completed

agent.failed

agent.retrying
```

Esses eventos se originam do Agent Orchestrator.

---

# 12. Eventos de Conversação

Exemplos:

```text
conversation.question

conversation.answer

conversation.notification

conversation.summary
```

Esses eventos são produzidos pela Camada de Conversação.

---

# 13. Eventos de Aprovação

Exemplos:

```text
approval.required

approval.received

approval.rejected

approval.timeout
```

As aprovações se tornam eventos de primeira classe no Runtime.

---

# 14. Eventos de Harness

Exemplos:

```text
harness.started

harness.completed

harness.failed

harness.timeout

harness.cancelled
```

As implementações de harness publicam eventos normalizados.

---

# 15. Eventos de Artefato

Exemplos:

```text
artifact.generated

artifact.updated

artifact.registered
```

Esses eventos permitem que outros componentes reajam a novos artefatos.

---

# 16. Eventos de Custo

Exemplos:

```text
usage.collected

cost.updated
```

O Cost Tracker se inscreve nos eventos de execução e publica informações de custo atualizadas.

---

# 17. Eventos de Runtime

Exemplos:

```text
runtime.started

runtime.ready

runtime.recovering

runtime.stopped

runtime.failed
```

Esses eventos descrevem o próprio Runtime.

---

# 18. Produtores de Eventos

Exemplos de produtores incluem:

* Workflow Engine;
* Agent Orchestrator;
* Camada de Conversação;
* Harness Adapter;
* State Management;
* Notification Manager;
* Cost Tracker.

Todo componente pode publicar eventos.

---

# 19. Consumidores de Eventos

Exemplos de consumidores incluem:

* Workflow Engine;
* State Management;
* Notification Manager;
* Camada de Conversação;
* Cost Tracker;
* subsistema de Métricas;
* subsistema de Logging.

Um único evento pode ter múltiplos consumidores.

---

# 20. Exemplo de Ciclo de Eventos

O Planejamento é concluído com sucesso.

```text
Planning Agent

      │

      ▼

agent.completed

      │

      ▼

Workflow Engine

      │

      ▼

stage.completed

      │

      ▼

State Management

Camada de Conversação

Cost Tracker

Notification Manager
```

Um evento pode disparar múltiplas reações independentes.

---

# 21. Barramento de Eventos Interno

O Runtime deve expor um Barramento de Eventos interno.

Interface sugerida:

```go
type EventBus interface {

    Publish(ctx context.Context, event Event) error

    Subscribe(eventType string, handler EventHandler)

    Unsubscribe(handlerID string)

}
```

O Barramento de Eventos deve permanecer leve.

---

# 22. Ordenação de Eventos

A ordenação é importante.

Dentro de um ciclo de trabalho:

```text
stage.started

↓

agent.started

↓

agent.completed

↓

stage.completed
```

Os consumidores devem observar os eventos na mesma ordem.

---

# 23. Persistência de Eventos

Todo evento importante deve ser persistido.

Os benefícios incluem:

* recuperação;
* auditoria;
* depuração;
* replay do ciclo de trabalho;
* análises.

A persistência pertence ao subsistema de State Management.

---

# 24. Replay de Eventos

Futuras versões do Runtime podem suportar o replay de eventos.

Exemplo:

```text
Histórico do Ciclo de Trabalho

↓

Replay de Eventos

↓

Reconstruir Estado
```

Isso possibilita depuração e auditoria.

---

# 25. Filtragem de Eventos

Os consumidores podem se inscrever seletivamente.

Exemplos:

```text
approval.*

stage.*

workflow.*

agent.*
```

A filtragem reduz o processamento desnecessário.

---

# 26. Eventos Síncronos vs Assíncronos

Eventos críticos podem ser processados de forma síncrona.

Exemplos:

* approval.received;
* workflow.completed.

Eventos não críticos podem ser assíncronos.

Exemplos:

* métricas;
* notificações;
* análises.

O Barramento de Eventos deve suportar ambos os modelos.

---

# 27. Idempotência de Eventos

Os consumidores devem tolerar a entrega duplicada de eventos.

Processar o mesmo evento múltiplas vezes não deve corromper o estado do ciclo de trabalho.

Todo evento deve conter um identificador único.

---

# 28. Tratamento de Falhas

Se um consumidor falhar:

* o evento permanece persistido;
* outros consumidores continuam o processamento;
* consumidores que falharam podem tentar novamente mais tarde.

A falha de um consumidor não deve interromper o Barramento de Eventos.

---

# 29. Versionamento de Eventos

Os eventos devem suportar a evolução do schema.

Exemplo:

```go
type Event struct {

    Version int

    ...

}
```

O versionamento possibilita a compatibilidade futura.

---

# 30. Monitoramento

O Sistema de Eventos deve expor métricas operacionais.

Exemplos:

* eventos publicados;
* eventos processados;
* entregas falhas;
* tamanho da fila;
* latência de processamento.

Essas métricas melhoram a observabilidade do Runtime.

---

# 31. Futuros Eventos Distribuídos

A implementação inicial é completamente em processo (in-process).

Versões futuras podem substituir o Barramento de Eventos interno por:

* NATS;
* RabbitMQ;
* Kafka;
* AWS EventBridge.

A API de Eventos deve permanecer inalterada.

---

# 32. Separação de Responsabilidades

O Workflow Engine:

* decide o progresso do ciclo de trabalho.

O Sistema de Eventos:

* distribui informações.

O subsistema de State Management:

* persiste os eventos.

A Camada de Conversação:

* apresenta os eventos.

O Notification Manager:

* reage aos eventos.

Cada subsistema permanece independente.

---

# 33. Princípios do Sistema de Eventos

O Sistema de Eventos segue estes princípios.

## Eventos em Primeiro Lugar

Tudo o que é importante se torna um evento.

---

## Imutável

Os eventos são somente para inserção (append-only).

---

## Ordenado

A ordem de execução é preservada.

---

## Observável

O comportamento do Runtime é visível por meio de eventos.

---

## Persistente

Os eventos sobrevivem a reinicializações do Runtime.

---

## Desacoplado

Os componentes se comunicam por meio de eventos, em vez de dependências diretas.

---

## Preparado para o Futuro

A arquitetura suporta futuras infraestruturas de eventos distribuídas.

---

# 34. Declaração de Arquitetura

O Sistema de Eventos é a espinha dorsal de comunicação do Hero Runtime.

Ele possibilita uma coordenação fracamente acoplada e orientada a eventos entre todos os componentes do Runtime, publicando, persistindo e distribuindo eventos imutáveis, garantindo que a execução do ciclo de trabalho permaneça observável, recuperável, extensível e independente de qualquer harness de codificação com IA ou interface de comunicação específica.