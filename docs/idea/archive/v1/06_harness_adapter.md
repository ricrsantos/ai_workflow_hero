# 06 - Harness Adapter

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero AI Loop Runtime
> **Idioma:** Português

---

# 1. Visão Geral

A Camada de Harness Adapter é a abstração que permite ao Hero Runtime executar agentes de IA usando diferentes harnesses de codificação com IA, sem alterar a implementação do fluxo de trabalho.

É um dos componentes arquiteturais mais importantes do Hero.

Seu propósito é isolar completamente o Runtime dos detalhes de implementação de cada harness.

O Runtime nunca deve saber se um agente está sendo executado pelo Cursor, OpenCode, Claude Code ou qualquer harness futuro.

---

# 2. Visão

O Hero Runtime deve ser completamente agnóstico em relação ao harness.

Todo harness é considerado um motor de execução.

O Hero é dono de:

* fluxo de trabalho;
* estado;
* conversação;
* aprovações;
* contexto;
* artefatos;
* ciclo de vida de execução.

Os harnesses possuem apenas uma responsabilidade:

> Executar tarefas de IA.

Tudo o mais pertence ao Hero.

---

# 3. Papel Arquitetural

O Harness Adapter atua como uma camada de tradução.

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

Harness Interface

        │

────────┼──────────────────────────────

        │

        ▼

Cursor Adapter

OpenCode Adapter

Codex Adapter

Claude Adapter

Future Adapter

        │

────────┼──────────────────────────────

        ▼

Harness Externo
```

O Runtime se comunica apenas com a Harness Interface.

Ele nunca se comunica com adaptadores concretos.

---

# 4. Responsabilidades

O Harness Adapter é responsável por:

* iniciar sessões de harness;
* retomar sessões existentes;
* enviar prompts;
* coletar respostas;
* coletar métricas de execução;
* coletar artefatos;
* monitorar a execução;
* cancelar execuções;
* normalizar resultados.

O adaptador **não** é responsável por:

* lógica do ciclo de trabalho;
* interação com o usuário;
* aprovações;
* transições de etapas;
* cálculos de custo.

---

# 5. Princípios de Design

## Independência de Harness

A lógica do ciclo de trabalho nunca deve depender de uma implementação de harness.

---

## Contrato Padronizado

Todo adaptador implementa exatamente a mesma interface.

---

## Substituibilidade

Substituir o Cursor pelo OpenCode não deve exigir alterações no fluxo de trabalho.

---

## Isolamento

O comportamento específico de cada harness permanece dentro dos adaptadores.

---

## Extensibilidade

Adicionar um novo harness deve exigir apenas uma nova implementação de adaptador.

---

# 6. Harness Interface

Todo adaptador deve implementar o mesmo contrato.

Interface sugerida em Go:

```go
type HarnessAdapter interface {

    Name() string

    Version() string

    IsAvailable(ctx context.Context) error

    CreateSession(ctx context.Context) (*Session, error)

    ResumeSession(ctx context.Context, sessionID string) error

    Execute(ctx context.Context, request ExecuteRequest) (*ExecutionResult, error)

    Cancel(ctx context.Context, sessionID string) error

    Status(ctx context.Context, sessionID string) (*ExecutionStatus, error)

}
```

Essa interface se torna a única dependência do Runtime.

---

# 7. Requisição de Execução

O Runtime envia uma requisição de execução normalizada.

Exemplo:

```go
type ExecuteRequest struct {

    WorkflowID string

    Stage string

    Agent string

    Prompt string

    Context []ContextItem

    Artifacts []Artifact

}
```

Todo adaptador recebe a mesma estrutura de requisição.

---

# 8. Resultado da Execução

Todo adaptador retorna a mesma estrutura de resultado.

Exemplo:

```go
type ExecutionResult struct {

    Status Status

    SessionID string

    Summary string

    Output string

    Usage Usage

    Artifacts []Artifact

    Duration time.Duration

}
```

O Workflow Engine nunca processa saídas específicas de um harness.

---

# 9. Gerenciamento de Sessão

O Hero é dono das sessões de fluxo de trabalho.

Os harnesses são donos das sessões de execução.

Exemplo:

```text
Ciclo de Trabalho

↓

Etapa de Planejamento

↓

Sessão

o do Harness

↓

Execução

↓

Resultado

↓

Sessão Encerrada
```

Uma sessão de harness é temporária.

Um ciclo de trabalho é persistente.

---

# 10. Ciclo de Vida da Sessão

```text
Criar Sessão

      │

      ▼

Executar Agente

      │

      ▼

Coletar Resultado

      │

      ▼

Persistir Metadados da Sessão

      │

      ▼

Reutilizar ou Encerrar
```

Os adaptadores decidem como as sessões são gerenciadas internamente.

---

# 11. Cursor Adapter

O Cursor Adapter encapsula o Cursor Agent CLI.

Exemplo:

```bash
cursor-agent \
  --print \
  --output-format json \
  --resume SESSION_ID \
  -p "Execute Planning Agent"
```

O adaptador é responsável por:

* criação de processo;
* parsing de JSON;
* rastreamento de sessão;
* extração de uso;
* tratamento de erros.

O Runtime nunca executa o Cursor CLI diretamente.

---

# 12. OpenCode Adapter

O OpenCode Adapter segue a mesma interface.

Internamente, pode utilizar:

* CLI;
* API;
* IPC;
* futuro SDK.

O Runtime permanece inalterado.

---

# 13. Codex Adapter

O Codex Adapter é outra implementação da mesma interface.

Seu protocolo interno pode ser completamente diferente.

O Runtime não se importa com isso.

---

# 14. Claude Code Adapter

O Claude Code Adapter é outra implementação da mesma interface.

Seu protocolo interno pode ser completamente diferente.

O Runtime não se importa com isso.

---

# 15. Seleção de Adaptador

O Runtime seleciona um adaptador por meio de configuração.

Exemplo:

```yaml
runtime:

  harness:

    provider: cursor
```

Ou:

```yaml
runtime:

  harness:

    provider: opencode
```

Trocar de provedor deve exigir apenas mudanças de configuração.

---

# 16. Registro de Adaptadores

O Runtime mantém um registro.

Exemplo:

```text
Registro de Harness

↓

Cursor

↓

OpenCode

↓

Codex

↓

Claude

↓

Harness Futuro
```

A seleção ocorre por meio do registro.

---

# 17. Tratamento de Erros

Os adaptadores traduzem falhas específicas de cada harness em erros normalizados.

Exemplo:

Erro interno do Cursor:

```text
CLI process exited with code 127
```

Traduzido para:

```text
HarnessUnavailable
```

O Workflow Engine nunca deve receber erros específicos de implementação.

---

# 18. Monitoramento de Execução

Os adaptadores monitoram a execução.

Possíveis responsabilidades:

* monitoramento de processo;
* detecção de timeout;
* cancelamento;
* atualizações de progresso;
* streaming de saída.

O monitoramento permanece específico de cada adaptador.

---

# 19. Suporte a Streaming

Futuros adaptadores podem suportar respostas em streaming.

Conceitualmente:

```text
Harness

↓

Tokens em Streaming

↓

Adaptador

↓

Eventos Normalizados

↓

Runtime
```

O suporte a streaming não deve alterar o Workflow Engine.

---

# 20. Coleta de Uso

Os adaptadores coletam métricas de uso.

Exemplo:

```go
type Usage struct {

    InputTokens int

    OutputTokens int

    CachedTokens int

    EstimatedCost float64

}
```

O Runtime agrega o uso entre os estágios do fluxo de trabalho.

---

# 21. Coleta de Artefatos

Alguns harnesses geram artefatos.

Exemplos:

* código-fonte;
* documentação;
* relatórios;
* screenshots;
* especificações.

O adaptador registra todo artefato gerado.

---

# 22. Descoberta de Capacidades

Nem todo harness suporta os mesmos recursos.

Cada adaptador deve expor suas capacidades.

Exemplo:

```go
type Capabilities struct {

    Resume bool

    Streaming bool

    Images bool

    MCP bool

    MultiSession bool

}
```

O Runtime pode adaptar seu comportamento com base nessas capacidades.

---

# 23. Estado do Adaptador

Os adaptadores devem permanecer o mais stateless possível.

As informações persistentes pertencem ao Runtime.

Informações temporárias de execução podem existir dentro dos adaptadores enquanto uma requisição está ativa.

---

# 24. Segurança

Os adaptadores nunca devem persistir:

* chaves de API;
* tokens de autenticação;
* credenciais do usuário.

O gerenciamento de credenciais pertence ao Runtime ou ao próprio harness.

---

# 25. Futuros Harnesses Remotos

A arquitetura deve suportar execução remota.

Exemplo:

```text
Hero Runtime

↓

Remote Adapter

↓

REST API

↓

Harness na Nuvem
```

Nenhuma alteração no Workflow Engine deve ser necessária.

---

# 26. Arquitetura de Plugins

Futuros harnesses devem ser instaláveis como plugins.

Conceitualmente:

```text
Hero Runtime

↓

Registro de Harness

↓

Adaptador Dinâmico

↓

Novo Harness
```

Isso possibilita integrações de harness desenvolvidas pela comunidade.

---

# 27. Estratégia de Testes

Todo adaptador deve passar pela mesma suíte de testes de conformidade.

Exemplos de validações:

* criar sessão;
* retomar sessão;
* executar tarefa;
* coletar uso;
* cancelar execução;
* tradução de erros.

Isso garante a compatibilidade com o Runtime.

---

# 28. Separação de Responsabilidades

O Runtime:

* é dono do ciclo de vida de execução.

O Workflow Engine:

* é dono das decisões do ciclo de trabalho.

O Agent Orchestrator:

* é dono da execução dos agentes.

O Harness Adapter:

* é dono da comunicação com o harness.

O Harness:

* é dono da execução de IA.

As responsabilidades devem permanecer claramente separadas.

---

# 29. Princípios do Adaptador

A Camada de Harness Adapter segue estes princípios.

## Contrato Padrão

Todo harness implementa a mesma interface.

---

## Isolamento Completo

A lógica específica de cada harness nunca vaza para o Runtime.

---

## Substituibilidade

Os harnesses podem ser substituídos sem alterações no fluxo de trabalho.

---

## Normalização

Resultados, erros e métricas são traduzidos para o modelo interno do Hero.

---

## Extensibilidade

Adicionar um novo harness exige apenas uma nova implementação de adaptador.

---

## Independência de Tecnologia

O Runtime nunca depende de um CLI, API ou SDK.

Somente o adaptador depende.

---

# 29. Declaração de Arquitetura

A Camada de Harness Adapter é a fronteira de abstração entre o Hero Runtime e os harnesses externos de codificação com IA.

Ela isola o Runtime de detalhes específicos de implementação, fornecendo uma interface de execução padronizada que permite ao Hero orquestrar fluxos de trabalho de forma consistente em múltiplas plataformas de codificação com IA, permanecendo completamente independente de harness.