# 02 - AI Loop

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero AI Loop Runtime
> **Idioma:** Português

---

# 1. Visão Geral

O AI Loop é o coração do Hero Runtime.

Ele é responsável por orquestrar o ciclo de vida completo do desenvolvimento de software através de uma sequência de estágios bem definidos, executados por agentes de IA especializados.

Diferente dos assistentes de IA tradicionais baseados em chat, o Hero AI Loop é **stateful** (mantém estado), **orientado a eventos** e de **longa duração**.

Um fluxo de trabalho pode ser executado por minutos, horas ou até dias, preservando seu estado de execução completo.

O AI Loop é responsável por:

* controlar a execução do fluxo de trabalho;
* coordenar agentes de IA;
* interagir com o usuário;
* gerenciar aprovações;
* invocar harnesses;
* rastrear custos;
* persistir estado;
* gerar artefatos.

---

# 2. Conceito Central

O AI Loop é um motor de execução determinístico.

Toda execução de fluxo de trabalho segue o mesmo ciclo de vida:

```text
Criar Loop
      │
      ▼
Inicializar
      │
      ▼
Executar Estágio
      │
      ▼
Coletar Resultados
      │
      ▼
Gerar Resumo
      │
      ▼
Aprovação Necessária?
      │
   Sim │ Não
      ▼
Aguardar Usuário
      │
      ▼
Próximo Estágio
      │
      ▼
Fluxo de Trabalho Concluído
```

Cada ciclo é controlada pelo Hero.

---

# 3. Ciclo de Vida do Loop

Cada execução de ciclo de trabalho cria uma nova Instância de Loop, o processo é igual ao que é realizado na implementação atual do Hero dentro do Cursor.

Uma Instância de Loop é dona de cada artefato de execução até a conclusão.

---

# 4. Estados do Ciclo

Um ciclo (loop) segue os mesmos estados da implementação atual do Hero.

As transições de estado são estritamente controladas pelo Workflow Engine (Motor de Fluxo de Trabalho).

---

# 5. Ciclo de Vida de uma Etapa

Toda etapa segue deve seguir um processo semelhante ao autlamente implementado no Hero, porém agora com o Harness sendo controlado diretamente pelo Hero.

```text
Etapa Criada
      │
      ▼
Preparar Contexto
      │
      ▼
Executar Agente
      │
      ▼
Coletar Artefatos
      │
      ▼
Coletar Uso
      │
      ▼
Gerar Resumo
      │
      ▼
Aprovação Necessária?
      │
      ▼
Concluir Etapa
```

Isso garante um comportamento consistente em todos as etapas do fluxo de trabalho.

---

# 6. Fluxo de Trabalho Padrão

O fluxo de trabalho padrão segue o mesmo processo atual do Hero.


---

# 7. Definições das Etapas

Seguem o mesmo processo atual do Hero, porém agora gerenciando também o Harness e os eventos:

* barramento de eventos;
* adaptador de harness.

---

# 8. Execução dos Agente

Todo estágio executa um agente principal, que deve ser incentivado a utilizar subagents do mesmo tipo, quando aplicável.

Fluxo de execução:

```text
Hero Runtime

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

      │

      ▼

Agente de IA
```

O Workflow Engine nunca se comunica diretamente com o harness.

Toda comunicação passa pelo Harness Adapter.

---

# 9. Fluxo de Contexto

Todo agente recebe um contexto de execução controlado, assim como ocorre atualmente no Hero.

Exemplo:

```
Contexto do Fluxo de Trabalho

Estágio Atual

Artefatos Relevantes

Arquivos do Projeto

Decisões Anteriores

Regras de Arquitetura

Instruções do Usuário
```

Os agentes nunca devem receber informações desnecessárias.

O contexto é minimizado para reduzir os custos de execução.

---

# 10. Resultado de cada etapa

Toda etapa executada retorna um resultado padronizado, assim como ocorre atualmente no Hero.

O formato do resultado deve ser idêntico para todo harness.

---

# 11. Canais de Comunicação

As aprovações podem chegar por meio de diferentes interfaces.

Exemplos:

* Hero TUI
* Hero CLI
* Telegram
* Discord
* Futura Web UI

Todos os canais de comunicação geram o mesmo evento interno.

O Workflow Engine não sabe de onde a aprovação se originou.

---

# 12. Capacidade de Retomada

Um fluxo de trabalho sempre pode ser retomado.

Requisitos:

* restaurar o estágio;
* restaurar o contexto;
* restaurar os artefatos;
* restaurar os custos;
* restaurar as aprovações pendentes;
* reconectar o harness, se necessário.

A retomada deve ser determinística.

---

# 13. Execução Orientada a Eventos

O Workflow Engine reage a eventos.

Exemplos:

```
workflow.started

stage.started

agent.started

agent.completed

approval.required

approval.received

workflow.completed

workflow.failed
```

Os eventos são imutáveis.

Eles se tornam parte do histórico do fluxo de trabalho.

---

# 14. Princípios de Design

O AI Loop segue estes princípios.

## Dono Único

O Hero é dono da execução do fluxo de trabalho.

---

## Determinístico

O mesmo fluxo de trabalho produz a mesma sequência de execução.

---

## Orientado a Eventos

Tudo acontece por causa de um evento.

---

## Longa Duração

A execução pode se estender por múltiplas sessões.

---

## Independente de Harness

Trocar o harness não altera o fluxo de trabalho.


---

## Recuperável

Falhas nunca perdem o estado do fluxo de trabalho.

---

# 15. Diagrama de Sequência

O fluxo de execução completo é ilustrado abaixo.

```text
Usuário
 │
 │ Iniciar Fluxo de Trabalho
 ▼
Hero Runtime
 │
 ▼
Workflow Engine
 │
 ▼
Preparação
 │
 ▼
Pesquisa
 │
 ▼
Resumo
 │
 ▼
Aprovação (se necessária)
 │
 ▼
Planejamento
 │
 ▼
Resumo
 │
 ▼
Aprovação (se necessária)
 │
 ▼
Implementação
 │
 ▼
Resumo
 │
 ▼
QA
 │
 ▼
Resumo
 │
 ▼
UI QA
 │
 ▼
Resumo
 │
 ▼
E2E
 │
 ▼
Resumo
 │
 ▼
Fluxo de Trabalho Concluído
```

---

# 16. Declaração de Arquitetura

O AI Loop é um motor de execução determinístico, stateful e orientado a eventos que orquestra o ciclo de vida completo do desenvolvimento de software, enquanto delega a execução das tarefas a harnesses de codificação com IA intercambiáveis.

O AI Loop é o núcleo operacional do Hero Runtime.