# 08 - Gerenciamento de Estado

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero AI Loop Runtime
> **Idioma:** Português

---

# 1. Visão Geral

O Gerenciamento de Estado é um dos componentes fundamentais do Hero Runtime.

Seu propósito é persistir e restaurar o estado de execução completo do AI Development Loop.

Diferente dos assistentes de IA tradicionais baseados em chat, os fluxos de trabalho do Hero são projetados para serem de **longa duração** e **recuperáveis**.

Um fluxo de trabalho pode ser executado por minutos, horas ou dias, preservando seu histórico de execução completo.

O subsistema de Gerenciamento de Estado garante que a execução sempre possa continuar após:

* reinicialização da aplicação;
* falha do runtime;
* reinicialização do sistema operacional;
* interrupção pelo usuário;
* falha do harness.

---

# 2. Visão

O Hero é dono do estado do fluxo de trabalho.

O harness de IA é dono apenas das sessões de execução temporárias.

Essa distinção é fundamental.

```text
Hero Runtime
    │
    ├── Estado do Fluxo de Trabalho
    ├── Estágio Atual
    ├── Eventos
    ├── Artefatos
    ├── Custos
    ├── Decisões
    └── Conversação
            │
            ▼
     Sessão do Harness
```

Perder uma sessão de harness nunca deve implicar em perder o ciclo de trabalho.

---

# 3. Responsabilidades

O subsistema de Gerenciamento de Estado é responsável por:

* persistir o estado do ciclo de trabalho;
* restaurar o estado do ciclo de trabalho;
* rastrear o progresso do ciclo de trabalho;
* armazenar metadados de execução;
* armazenar aprovações;
* armazenar o histórico de execução;
* rastrear artefatos;
* armazenar informações de custo;
* armazenar metadados do runtime.

Ele **não** é responsável por:

* execução do ciclo de trabalho;
* execução de IA;
* interação com o usuário;
* geração de eventos.

---

# 4. Princípios de Design

## O Runtime é Dono do Estado

O estado persistente pertence exclusivamente ao Hero Runtime.

---

## Fonte Única de Verdade

Existe apenas um estado de fluxo de trabalho autoritativo.

---

## Persistente por Padrão

Toda mudança importante é persistida.

---

## Recuperável

Todo fluxo de trabalho persistido pode ser retomado.

---

## Histórico Imutável

As informações históricas nunca são modificadas.

---

## Independente de Tecnologia

As estruturas de estado não devem depender de uma implementação de armazenamento específica.

---

# 5. Hierarquia de Estado

O Runtime gerencia múltiplos níveis de estado.

```text
Runtime
    │
    ├── ciclo de Trabalho
    │      │
    │      ├── Estágio
    │      ├── Eventos
    │      ├── Custos
    │      ├── Artefatos
    │      └── Decisões
    │
    └── Metadados do Runtime
```

Cada nível tem responsabilidades independentes.

---

# 6. Estado do Runtime

O Runtime mantém informações globais de execução.

O estado do runtime é independente do estado do ciclo de trabalho.

---

# 7. Estado do Ciclo de Trabalho

Cada ciclo de trabalho possui seu próprio estado persistido.

O estado do ciclo de trabalho é o objeto central do AI Loop.

---

# 8. Estado da Etapa

Toda etapa mantém um estado independente.

As etapas são imutáveis após a conclusão.

---

# 9. Status do Ciclo de Trabalho

Possíveis estados do fluxo de trabalho:

```text
Criado

Inicializando

Em Execução

AguardandoAprovação

Pausado

Concluído

Cancelado

Falhou
```

As transições de estado são controladas exclusivamente pelo Workflow Engine.

---

# 10. Status da Etapa

Cada etapa pode ter um dos seguintes estados:

```text
Pendente

Preparando

Executando

ColetandoResultados

AguardandoAprovação

Concluído

Falhou

Cancelado
```

As etapas não podem pular estados.

---

# 11. Informações Persistidas

As seguintes informações devem ser persistidas.

Metadados do Ciclo de trabalho.

Etapa  atual.

Etapas concluídas.

Artefatos gerados.

Histórico de execução.

Aprovações.

Decisões do usuário.

Custos de execução.

Sessões do harness.

Histórico de conversação.

Snapshot da configuração.

Versão do runtime.

---

# 12. Metadados do Ciclo de Trabalho

Exemplo:

```yaml
workflow:

  id: wf-001

  project: ai-workflow-hero

  workflow: default

  owner: local-user

  version: 1
```

Os metadados identificam o ciclo de trabalho.

---

# 13. Histórico de Execução

Toda etapa de execução deve ser registrada.

Exemplo:

```text
Ciclo de Trabalho Criado

Preparação Iniciada

Preparação Concluída

Pesquisa Iniciada

Pesquisa Concluída

Planejamento Iniciado

Aprovação Solicitada

Aprovação Recebida
```

O histórico é somente para inserção (append-only).

---

# 14. Persistência de Eventos

Todo evento do runtime se torna parte do histórico do ciclo de trabalho.

Exemplo:

```yaml
event:

  id: evt-100

  type: approval.received

  timestamp: ...

  payload: ...
```

Os eventos nunca devem ser modificados após a criação.

---

# 15. Estado da Conversação

O Hero é dono do histórico de conversação.

A conversação inclui:

* perguntas do usuário;
* respostas do usuário;
* resumos;
* aprovações;
* notificações;
* mensagens do runtime.

O histórico de conversação é independente das sessões do harness.

---

# 16. Estado de Aprovação

As aprovações exigem persistência.

Exemplo:

```yaml
approval:

  stage: planning

  status: pending

  requested_at: ...

  approved_at: null
```

As aprovações pendentes sobrevivem a reinicializações do Runtime.

---

# 17. Estado da Sessão do Harness

As sessões do harness devem ser armazenadas.

Exemplo:

```yaml
cursor:

  session_id: 069c5368...

  provider: cursor

  reusable: true
```

Essa informação permite execuções retomadas.

---

# 18. Estado de Custos

Os custos do ciclo de trabalho se acumulam durante a execução.

Exemplo:

```yaml
cost:

  total_tokens:

    input: 45000

    output: 6200

  estimated_cost: 1.82
```

As informações de custo são atualizadas após cada etapa concluído.

---

# 19. Registro de Artefatos

Os artefatos gerados são registrados.

Exemplo:

```yaml
artifacts:

  - discovery.md

  - open-spec.md

  - qa-report.md
```

O registro faz referência aos artefatos.

Os artefatos em si são armazenados separadamente.

---

# 20. Layout de Armazenamento

Partir da estrutura atual do Hero.

O layout deve permanecer independente de implementação.

---

# 21. Estratégia de Persistência de Estado

O estado deve ser persistido após toda operação significativa.

Exemplos:

* etapa concluída;
* aprovação recebida;
* ciclo de trabalho pausado;
* ciclo de trabalho retomado;
* artefato gerado;
* execução falhou.

A persistência frequente minimiza a perda em caso de recuperação.

---

# 22. Processo de Recuperação

A recuperação segue esta sequência.

```text
Runtime Inicia

      │

      ▼

Carregar Estado do Runtime

      │

      ▼

Carregar Ciclos de Trabalho Ativos

      │

      ▼

Restaura a Etapa Atual

      │

      ▼

Restaurar Aprovações Pendentes

      │

      ▼

Retomar Fluxo de Trabalho
```

A recuperação deve ser determinística.

---

# 23. Suporte a Snapshots

Versões futuras podem suportar snapshots de fluxo de trabalho.

Exemplo:

```text
Ciclo de Trabalho

↓

Snapshot

↓

Continuar Execução
```

Os snapshots permitem rollback e depuração.

---

# 24. Versionamento

O estado persistido deve ser versionado.

Exemplo:

```yaml
schema:

  version: 1
```

Futuras versões do Runtime devem suportar migrações.

---

# 25. Integridade de Dados

A persistência de estado deve garantir consistência.

Requisitos:

* escritas atômicas;
* detecção de corrupção;
* validação de schema;
* recuperação após interrupção.

Um estado parcialmente escrito nunca deve ser considerado válido.

---

# 26. Concorrência

Inicialmente, apenas uma instância do Runtime deve modificar um fluxo de trabalho.

A execução distribuída futura pode exigir:

* bloqueio otimista;
* leases de fluxo de trabalho;
* sincronização distribuída.

O modelo de persistência deve antecipar essa expansão futura.

---

# 27. Backend de Armazenamento

O subsistema de Gerenciamento de Estado deve depender de abstrações.

Implementações possíveis:

* arquivos JSON;
* SQLite;
* PostgreSQL;
* DynamoDB;
* Cloud Storage.

O Runtime deve permanecer independente da tecnologia de armazenamento.

---

# 28. Interface do Repositório de Estado

Abstração sugerida:

```go
type StateRepository interface {

    SaveWorkflow(ctx context.Context, workflow *Workflow) error

    LoadWorkflow(ctx context.Context, id string) (*Workflow, error)

    SaveEvent(ctx context.Context, event *Event) error

    SaveApproval(ctx context.Context, approval *Approval) error

    SaveConversation(ctx context.Context, message *ConversationMessage) error

}
```

As implementações concretas de armazenamento devem satisfazer essa interface.

---

# 29. Melhorias Futuras

Possíveis capacidades futuras incluem:

* snapshots de fluxo de trabalho;
* sincronização em nuvem;
* fluxos de trabalho colaborativos;
* execução distribuída;
* armazenamento de estado criptografado;
* persistência incremental.

A arquitetura deve suportar essas capacidades sem redesenho.

---

# 30. Separação de Responsabilidades

O Runtime:

* é dono do ciclo de vida.

O Workflow Engine:

* é dono das transições do ciclo de trabalho.

O subsistema de Gerenciamento de Estado:

* é dono da persistência.

A Camada de Conversação:

* é dona da comunicação.

O Harness Adapter:

* é dono da integração com o harness.

Cada subsistema tem uma responsabilidade claramente definida.

---

# 31. Princípios do Gerenciamento de Estado

O subsistema de Gerenciamento de Estado segue estes princípios.

## Runtime em Primeiro Lugar

O Runtime é dono do estado persistente.

---

## Persistente por Padrão

As informações críticas são sempre persistidas.

---

## Recuperável

Todo fluxo de trabalho persistido pode ser retomado.

---

## Histórico Imutável

As informações históricas são somente para inserção (append-only).

---

## Independente de Armazenamento

A tecnologia de persistência é um detalhe de implementação.

---

## Preparado para o Futuro

O modelo de dados suporta execução distribuída futura.

---

# 32. Declaração de Arquitetura

O subsistema de Gerenciamento de Estado é o alicerce de persistência do Hero Runtime.

Ele garante que fluxos de trabalho, histórico de execução, aprovações, conversações, artefatos, custos e metadados do runtime sobrevivam a interrupções e possam ser restaurados de forma determinística, permitindo que o Hero AI Loop execute fluxos de trabalho de desenvolvimento de software de longa duração, independentemente de qualquer harness de codificação com IA.