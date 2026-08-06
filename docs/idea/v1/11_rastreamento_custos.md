# 11 - Rastreamento de Custos

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero AI Loop Runtime
> **Idioma:** Português

---

# 1. Visão Geral

O subsistema de Rastreamento de Custos é responsável por coletar, calcular, agregar e apresentar as métricas de execução geradas ao longo do Hero AI Development Loop.

Um dos objetivos centrais do Hero é tornar o desenvolvimento de software assistido por IA transparente.

Os usuários devem sempre entender:

* quantos tokens foram consumidos;
* quanto cada etapa custou;
* qual agente gerou o custo;
* qual harness executou o trabalho;
* quanto tempo a execução levou;
* quantos loops foram necessários;
* o custo acumulado do ciclo de trabalho.

O Rastreamento de Custos é, portanto, um subsistema de primeira classe do Hero Runtime.

---

# 2. Visão

O desenvolvimento assistido por IA deve ser mensurável.

Todo ciclo de trabalho deve fornecer visibilidade completa sobre:

* tempo de execução;
* consumo de tokens;
* utilização de cache;
* número de loops;
* custo monetário estimado;
* uso de recursos.

O Hero trata as métricas de execução como dados operacionais, e não apenas como informação de diagnóstico.

---

# 3. Responsabilidades

O subsistema de Rastreamento de Custos é responsável por:

* coletar métricas de uso;
* calcular custos de execução;
* agregar métricas por etapa;
* agregar métricas do ciclo de trabalho;
* gerar resumos de execução;
* expor métricas históricas;
* publicar eventos de custo.

Ele **não** é responsável por:

* executar ciclos de trabalho;
* comunicar-se com provedores de IA;
* determinar modelos de precificação;
* interação com o usuário.

---

# 4. Princípios de Design

## Transparente

Toda etapa expõe métricas de execução.

---

## Incremental

As métricas se acumulam ao longo do ciclo de trabalho.

---

## Independente de Harness

A coleta de custos funciona em diferentes harnesses.

---

## Independente de Provedor

As métricas são normalizadas independentemente do provedor de LLM subjacente.

---

## Histórico

O histórico de execução é preservado.

---

## Observável

As métricas ficam disponíveis para outros componentes do Runtime.

---

# 5. Arquitetura de Alto Nível

```text id="m8yrzt"
                 Harness Adapter

                        │

                        ▼

                 Resultado da Execução

                        │

                        ▼

                 Cost Tracker

                        │

        ┌───────────────┼───────────────┐

        ▼               ▼               ▼

 Totais do Ciclo   Totais da Etapa   Eventos de Custo

                        │

                        ▼

              Camada de Conversação
```

O Cost Tracker consome as métricas de execução e produz informações agregadas.

---

# 6. Fontes de Métricas

As métricas se originam principalmente do Harness Adapter.

Exemplos incluem:

* tokens de entrada;
* tokens de saída;
* tokens em cache;
* duração da execução;
* quantidade de retrais / loops;
* metadados do provedor.

Futuras versões do Runtime podem coletar métricas adicionais.

---

# 7. Modelo de Uso

Estrutura sugerida em Go:

```go id="bz3fw7"
type Usage struct {

    InputTokens int

    OutputTokens int

    CacheReadTokens int

    CacheWriteTokens int

    EstimatedCost float64

    Duration time.Duration

}
```

O modelo Usage é neutro em relação ao provedor.

---

# 8. Métricas da Etapa

Cada etapa do ciclo de trabalho mantém suas próprias métricas.

As métricas da etapa se tornam imutáveis após a conclusão.

---

# 9. Métricas do Ciclo de Trabalho

As métricas do ciclo de trabalho agregam todas as etapas concluídas.

Os totais do ciclo de trabalho são recalculados de forma incremental.

---

# 10. Métricas do Agente

As métricas também podem ser agrupadas por agente.

Exemplo:

```text id="rj4vgv"
Discover Agent

$0,21

Planning Agent

$0,42

Backend Agent

$0,73

QA Agent

$0,18
```

Isso possibilita futuras análises de desempenho.

---

# 11. Métricas do Harness

As métricas podem ser agrupadas por harness.

Exemplo:

```text id="n4b6u2"
Cursor

$1,84

OpenCode

$0,91

Claude Code

$1,42
```

Isso permite que os usuários comparem plataformas de execução.

---

# 12. Métricas do Provedor

Quando disponíveis, as informações específicas do provedor devem ser preservadas.

Exemplos:

* OpenAI;
* Anthropic;
* Google;
* modelos locais.

O Runtime armazena métricas normalizadas, mantendo os metadados do provedor quando úteis.

---

# 13. Modelos de Precificação

Os modelos de precificação devem ser configuráveis.

O Runtime não deve fixar a precificação no código (hardcode).

---

# 14. Custo Estimado

O custo estimado é calculado usando métricas de uso normalizadas.

Conceitualmente:

```text id="rrnkrj"
Tokens de Entrada

+

Tokens de Saída

+

Modelo de Precificação

↓

Custo Estimado
```

As fórmulas de precificação podem evoluir de forma independente do Runtime.

---

# 15. Duração da Execução

O tempo de execução é registrado para cada etapa.

Exemplo:

```text id="lhwbrs"
Preparação

1m

Pesquisa

26m

Planejamento

18m

Implementação

52m

QA

12m

UI QA

5m

E2E

14m
```

A duração do ciclo de trabalho é cumulativa.

---

# 16. Métricas de Cache

Alguns provedores expõem a utilização de cache.

Exemplos:

* tokens de leitura de cache;
* tokens de escrita de cache.

Esses valores devem ser preservados sempre que disponíveis.

---

# 17. Eventos de Custo

O Cost Tracker publica eventos.

Exemplos:

```text id="uzwrfm"
usage.collected

cost.updated

workflow.cost.updated
```

Outros componentes do Runtime podem se inscrever nesses eventos.

---

# 18. Resumo da Etapa

Toda etapa concluída gera um resumo de execução padronizado.

Exemplo:

```text id="0j2kv4"
===================================

Planejamento Concluído

Duração

18m

Tokens de Entrada

24.000

Tokens de Saída

3.200

Leitura de Cache

11.000

Custo Estimado

$0,42

===================================
```

A Camada de Conversação é responsável pela apresentação.

---

# 19. Resumo do Ciclo de Trabalho

A conclusão do ciclo de trabalho gera um relatório agregado.

Exemplo:

```text id="6mhd8z"
Ciclo de Trabalho Concluído

Duração Total

2h 08m

Tokens de Entrada

96.000

Tokens de Saída

12.800

Custo Estimado

$2,41
```

Esse resumo se torna parte do histórico do ciclo de trabalho.

---

# 20. Métricas Históricas

As métricas históricas devem permanecer disponíveis.

Os usuários podem inspecionar:

* ciclos de trabalho anteriores;
* etapas anteriores;
* agentes anteriores;
* custos anteriores.

As informações históricas apoiam a otimização.

---

# 21. Análise de Tendências

Futuras versões do Runtime podem analisar a execução histórica.

Exemplos:

* custo médio do ciclo de trabalho;
* duração média do planejamento;
* agentes mais caros;
* etapas de maior duração.

A análise de tendências está fora do escopo da implementação inicial, mas deve ser suportada pelo modelo de dados.

---

# 22. API de Relatórios

Abstração sugerida:

```go id="d8bpjz"
type CostTracker interface {

    RecordUsage(ctx context.Context, usage Usage) error

    StageSummary(ctx context.Context, workflowID, stage string) (*StageCostSummary, error)

    WorkflowSummary(ctx context.Context, workflowID string) (*WorkflowCostSummary, error)

}
```

As implementações podem evoluir sem afetar o Runtime.

---

# 23. Persistência

As informações de custo devem ser persistidas após cada etapa concluída.

---

# 24. Métricas Futuras

Futuras versões do Runtime podem coletar:

* latência de API;
* contagem de execução de ferramentas;
* operações de arquivo;
* esforço de raciocínio (reasoning effort);
* consumo de memória;
* contagem de novas tentativas (retries).

A arquitetura deve permanecer extensível.

---

# 25. Visualização

Futuras interfaces podem apresentar métricas graficamente.

Exemplos:

* linhas do tempo do ciclo de trabalho;
* distribuição de tokens;
* comparação entre etapas;
* evolução de custo;
* mapas de calor de execução.

O subsistema de Rastreamento de Custos deve expor dados normalizados adequados para visualização.

---

# 26. Tratamento de Erros

Se as informações de uso estiverem indisponíveis:

* a execução continua;
* os valores ausentes são claramente identificados;
* o custo estimado é omitido em vez de estimado por aproximação.

A transparência é preferível à aproximação.

---

# 27. Separação de Responsabilidades

O Harness Adapter:

* extrai o uso bruto.

O subsistema de Rastreamento de Custos:

* normaliza e agrega as métricas.

O Workflow Engine:

* determina o fluxo de execução.

A Camada de Conversação:

* apresenta os resumos.

O subsistema de State Management:

* persiste as métricas.

Cada subsistema tem uma responsabilidade distinta.

---

# 28. Princípios do Rastreamento de Custos

O subsistema de Rastreamento de Custos segue estes princípios.

## Transparente

Os usuários sempre entendem o consumo de recursos.

---

## Incremental

As métricas se acumulam ao longo da execução.

---

## Normalizado

Diferentes provedores expõem um modelo de dados comum.

---

## Histórico

As métricas de execução são preservadas.

---

## Configurável

Os modelos de precificação são externalizados.

---

## Extensível

Futuras métricas se integram sem redesenho.

---

# 29. Capacidades Futuras

Melhorias futuras potenciais incluem:

* painéis de custo em tempo real;
* limites de orçamento;
* alertas de custo do ciclo de trabalho;
* relatórios de comparação entre provedores;
* recomendações de otimização;
* previsão automática de custos.

A arquitetura deve suportar essas capacidades por meio de extensão, e não de modificação.

---

# 30. Declaração de Arquitetura

O subsistema de Rastreamento de Custos fornece visibilidade operacional completa sobre o Hero AI Development Loop, coletando, normalizando, agregando e preservando métricas de execução em ciclos de trabalho, etapas, agentes e harnesses.

Ao tratar as informações de custo e uso como dados de runtime de primeira classe, o Hero possibilita um desenvolvimento de software assistido por IA transparente, mensurável e otimizável, permanecendo independente de provedores de IA, modelos de precificação e harnesses de codificação específicos.