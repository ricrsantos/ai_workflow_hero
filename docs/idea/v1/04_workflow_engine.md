# 04 - Workflow Engine

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero AI Loop Runtime
> **Idioma:** Português

---

# 1. Visão Geral

O Workflow Engine é o núcleo de tomada de decisão do Hero Runtime.

Sua responsabilidade é executar ciclos de trabalho de desenvolvimento de software de maneira determinística, com estado e orientada a eventos.

O Workflow Engine **não** executa agentes de IA diretamente.

Em vez disso, ele coordena o ciclo de vida de execução completo por meio de:

* gerenciamento do estado do ciclo de trabalho;
* controle das transições de etapas;
* invocação do Agent Orchestrator;
* espera pelos resultados de execução;
* solicitação de aprovações do usuário;
* reação a eventos do runtime;
* decisão sobre a próxima ação.

É o único componente responsável por controlar o progresso do fluxo de trabalho.

---

# 2. Responsabilidades

O Workflow Engine é responsável por:

* criar instâncias de ciclo de trabalho;
* executar as etapas do ciclo de trabalho;
* validar a conclusão das etapas;
* lidar com aprovações;
* decidir as transições de estado;
* reagir a falhas;
* gerenciar novas tentativas (retries);
* pausar a execução;
* retomar a execução;
* concluir ciclos de trabalho;
* cancelar ciclos de trabalho.

Ele nunca se comunica diretamente com os harnesses.

---

# 3. Arquitetura do Engine

```text
                        Workflow Engine
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
          ▼                    ▼                    ▼
   Máquina de Estados    Stage Scheduler      Event Processor
          │                    │                    │
          └────────────────────┼────────────────────┘
                               │
                               ▼
                     Agent Orchestrator
                               │
                               ▼
                      Camada de Adaptador de Harness
```

O Workflow Engine é totalmente independente de provedores de IA e de implementações de harness.

---

# 4. Princípios de Design

O Workflow Engine segue estes princípios:

## Determinístico

A mesma configuração de fluxo de trabalho deve sempre produzir a mesma sequência de execução.

---

## Orientado a Eventos

Toda transição é disparada por um evento.

---

## Com Estado

O estado de execução é sempre persistido.

---

## Recuperável

Fluxos de trabalho interrompidos sempre podem ser retomados.

---

## Independente de Harness

A lógica de execução nunca depende de uma ferramenta de codificação com IA específica.

---

## Governado pelo Usuário

Decisões pré configuradas como do usuário, exigem aprovação explícita do usuário.

---

# 5. Instância de Ciclo de Trabalho

Toda execução cria uma Instância de Ciclo de Trabalho.

A Instância de Ciclo de Trabalho se torna dona de cada execução de estágio.

---

# 6. Máquina de Estados do Ciclo de Trabalho

O Workflow Engine gerencia as transições de estado do ciclo de trabalho.

```text
Criado
   │
   ▼
Inicializando
   │
   ▼
Em Execução
   │
   ├───────────────┐
   ▼               │
Aguardando Aprovação│
   │               │
   ▼               │
Em Execução         │
   │               │
   ▼               │
Concluído           │
                   │
Falhou──────────────┘
   │
   ▼
Cancelado
```

Somente o Workflow Engine pode alterar o estado do ciclo de trabalho.

---

# 7. Stage (etapa) Scheduler

O Stage Scheduler determina qual etapa será executado a seguir.

Responsabilidades:

* identificar a etapa atual;
* verificar dependências;
* validar o estado anterior;
* agendar a execução;
* impedir transições inválidas.

Exemplo:

```
Etapa Atual

Planejamento

↓

Próximo Etapa

Implementação
```

A ordem de execução das etapas é definida pela configuração do fluxo de trabalho.

---

# 8. Ciclo de Vida de uma Etapa

Toda etapa segue o mesmo ciclo de vida.

```text
Criado
   │
   ▼
Preparando
   │
   ▼
Executando
   │
   ▼
Coletando Resultados
   │
   ▼
Gerando Resumo
   │
   ▼
Aguardando Aprovação
   │
   ▼
Concluído
```

Esse ciclo de vida é idêntico para toda etapa do ciclo de trabalho.

---

# 9. Contexto do Estadp

Antes da execução, o Workflow Engine prepara o contexto de execução.

O contexto pode incluir:

* metadados do ciclo de trabalho;
* informações do projeto;
* artefatos relevantes;
* decisões anteriores;
* regras de arquitetura;
* instruções do usuário;
* histórico de execução.

O Engine é responsável por determinar quais informações são relevantes.

---

# 10. Invocação do Agente

O Workflow Engine nunca invoca harnesses diretamente.

Em vez disso:

```text
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

      │

      ▼

Agente de IA
```

Essa separação mantém a lógica do ciclo de trabalho independente da tecnologia de execução.

---

# 11. Resultado da Execução

Todo ciclo retorna um resultado padronizado, da mesma forma que a implementação atual do Hero.

O Workflow Engine avalia esse resultado antes de decidir a próxima ação.

---

# 12. Conclusão da Etapa

Uma etapa é considerado concluída somente após todoa os itens de validação serem bem-sucedidas.

A validação inclui:

* execução concluída;
* artefatos coletados;
* uso coletado;
* resumo gerado;
* aprovação processada (se necessária).

A conclusão é uma decisão do Workflow Engine.

---

# 13. Fluxo de Aprovação

Alguns estágios exigem aprovação do usuário.

Fluxo de execução:

```text
Estágio Concluído

      │

      ▼

Gerar Resumo

      │

      ▼

approval.required

      │

      ▼

Aguardando Aprovação

      │

      ▼

approval.received

      │

      ▼

Continuar Fluxo de Trabalho
```

Enquanto aguarda a aprovação, nenhum estágio adicional pode ser executado.

---

# 14. Decisões do Usuário

Resultados possíveis de aprovação são os mesmos atualmente implementados no Hero.

---

# 15. Gerenciamento de Falhas

Se a execução falhar, o Workflow Engine decide a próxima ação.

Ações possíveis:

```text
Repetir

Pausar

Cancelar

Intervenção Manual
```

As falhas nunca contornam o Workflow Engine.

---

# 16. Estratégia de Repetição

As repetições (retries) em caso de falha devem ser configuráveis.

Exemplo:

```yaml
retry:

max_attempts: 3

backoff: exponential
```

O Workflow Engine rastreia o histórico de repetições.

---

# 17. Processamento de Eventos

O Workflow Engine reage a eventos do runtime.

Exemplos:

```text
workflow.started

stage.started

agent.completed

approval.received

workflow.cancelled

workflow.failed

runtime.recovered
```

Os eventos se tornam o gatilho para as transições de estado.

---

# 18. Fluxo de Eventos

Exemplo:

```text
Agente Concluído

      │

      ▼

Barramento de Eventos

      │

      ▼

Workflow Engine

      │

      ▼

Validar Resultado

      │

      ▼

Gerar Resumo

      │

      ▼

Aprovação Necessária?

      │

      ▼

Próximo Estágio
```

O Workflow Engine consome eventos em vez de fazer polling nos componentes.

---

# 19. Coleta de Custos

O Workflow Engine agrega métricas de execução da mesma forma que está atualmente implementado no Hero.

Os totais do ciclo de trabalho são atualizados após cada estágio concluído.

---

# 20. Registro de Artefatos

O Workflow Engine registra todo artefato gerado durante a execução, da mesma que já está implementado no Hero.

Os artefatos se tornam parte do estado do fluxo de trabalho.

---

# 21. Pausar e Retomar

O Workflow Engine suporta pausar a qualquer momento.

Os motivos de pausa podem incluir:

* solicitação do usuário;
* aprovação pendente;
* desligamento do runtime;
* harness indisponível;
* dependência externa.

Retomar restaura a execução exatamente do ponto de interrupção.

---

# 22. Cancelamento

O cancelamento é explícito.

Etapas de cancelamento:

1. parar de agendar novas etaps;
2. aguardar interrupção segura;
3. persistir o estado final;
4. gerar o resumo de cancelamento;
5. encerrar a etapa de trabalho.

Os ciclos de trabalho cancelados seguem a mesma política de arquivamento atual do Hero.

---

# 23. Recuperação

Após a recuperação do Runtime, o Workflow Engine:

1. carrega o estado persistido do fluxo de trabalho;
2. restaura o estágio atual;
3. restaura as aprovações pendentes;
4. restaura o histórico de execução;
5. retoma o agendamento.

A recuperação deve ser transparente para o usuário sempre que possível.

---

# 24. Múltiplos Ciclos de Trabalho

Versões futuras podem executar múltiplos ciclos de trabalho simultaneamente.

Conceitualmente:

```text
Workflow Engine

    │

    ├── Ciclo de Trabalho A

    ├── Ciclo de Trabalho B

    ├── Ciclo de Trabalho C

    └── Ciclo de Trabalho D
```

Cada ciclo de trabalho mantém uma máquina de estados independente.

O modelo de agendamento deve suportar concorrência sem acoplar os estados dos ciclos de trabalho.

---

# 25. Definições de Ciclos de Trabalho Extensíveis

As etapas do ciclo de trabalho não devem ser fixos no código (hardcoded).

Em vez disso, as etapas de trabalho devem ser definidos de forma declarativa, da mesma forma que é feita na implementação atual do Hero.

Isso permite fluxos de trabalho personalizados sem alterar a implementação do engine.

---

# 26. Separação de Responsabilidades

O Workflow Engine é responsável pelas decisões.

O Agent Orchestrator é responsável pela execução.

O Harness Adapter é responsável pela comunicação.

O Harness é responsável pela execução de IA.

O Runtime é responsável pelo gerenciamento do ciclo de vida.

Essa separação nunca deve ser violada.

---

# 27. Princípios do Workflow Engine

O Workflow Engine segue estes princípios.

## Tomador de Decisão Único

Somente o Workflow Engine decide o progresso do ciclo de trabalho.

---

## Execução Determinística

Toda transição segue regras explícitas.

---

## Orientado a Eventos

Eventos disparam toda decisão.

---

## Persistente

O estado do fluxo de trabalho sobrevive a interrupções do processo.

---

## Observável

Toda decisão importante gera um evento.

---

## Recuperável

A execução pode ser retomada a partir de qualquer estado persistido.

---

## Extensível

Novas etapas do ciclo de trabalho podem ser adicionados sem redesenhar o engine.

---

# 28. Declaração de Arquitetura

O Workflow Engine é o motor de decisão determinístico do Hero Runtime.

Ele é dono do progresso do ciclo de trabalho, das transições de etapas, das aprovações, das repetições, da recuperação e do estado de execução, enquanto delega a execução dos agentes ao Agent Orchestrator por meio de abstrações independentes de harness.

É a fonte autoritativa do comportamento do fluxo de trabalho dentro do Hero AI Loop.