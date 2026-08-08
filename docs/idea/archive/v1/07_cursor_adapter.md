# 07 - Cursor Adapter

> **Versão do Documento:** 1.0
> **Status:** Rascunho
> **Aplica-se a:** Hero Runtime - Cursor Harness Adapter
> **Idioma:** Português

---

# 1. Visão Geral

O Cursor Adapter é a primeira implementação concreta da interface `HarnessAdapter`.

Sua responsabilidade é integrar o Hero Runtime com o Cursor Agent CLI, permitindo que o Hero execute agentes de IA usando o Cursor, mantendo o Runtime completamente independente dos detalhes de implementação específicos do Cursor.

O Cursor Adapter é um componente de infraestrutura.

Ele **não** contém lógica de fluxo de trabalho, regras de negócio, interação com o usuário ou orquestração de agentes.

---

# 2. Propósito

O adaptador existe para traduzir o modelo de execução interno do Hero em comandos do Cursor Agent CLI.

Conceitualmente:

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
Cursor Adapter
      │
      ▼
Cursor Agent CLI
      │
      ▼
LLM
```

O Runtime nunca executa o Cursor CLI diretamente.

---

# 3. Responsabilidades

O Cursor Adapter é responsável por:

* localizar o executável do Cursor Agent CLI;
* validar a disponibilidade do CLI;
* criar sessões do Cursor;
* retomar sessões do Cursor;
* executar prompts;
* fazer o parsing de respostas JSON;
* coletar métricas de uso;
* coletar a duração da execução;
* traduzir erros do Cursor;
* expor as capacidades do Cursor.

O adaptador **não** é responsável por:

* estado do fluxo de trabalho;
* aprovações;
* conversação;
* seleção de contexto;
* agregação de custos;
* persistência de artefatos.

---

# 4. Arquitetura

```text
                 Cursor Adapter

                     │

     ┌───────────────┼────────────────┐
     │               │                │

     ▼               ▼                ▼

CLI Discovery   Process Runner   JSON Parser

     │               │                │

     └───────────────┼────────────────┘
                     │
                     ▼
             Execution Result Mapper
                     │
                     ▼
              Resultado do HarnessAdapter
```

Cada componente tem uma única responsabilidade.

---

# 5. Ciclo de Vida

O ciclo de vida do adaptador é:

```text
Inicializar

      │

      ▼

Validar CLI

      │

      ▼

Criar Sessão

      │

      ▼

Executar Prompt

      │

      ▼

Fazer Parsing do Resultado

      │

      ▼

Retornar ExecutionResult
```

---

# 6. Descoberta do CLI

Durante a inicialização, o adaptador deve localizar o Cursor Agent CLI.

Estratégias possíveis:

1. Variável de ambiente
2. Configuração do usuário
3. Busca no PATH
4. Caminhos de instalação conhecidos

Exemplo:

```bash
which cursor-agent
```

Se o executável não for encontrado, o adaptador reporta:

```text
HarnessUnavailable
```

---

# 7. Verificação de Disponibilidade

O adaptador implementa:

```go
IsAvailable(ctx context.Context) error
```

A validação inclui:

* o executável existe;
* o executável é executável;
* versão mínima suportada;
* saída JSON disponível.

O Runtime deve realizar essa validação durante a inicialização.

---

# 8. Detecção de Versão

O adaptador deve identificar a versão instalada do Cursor Agent.

Exemplo:

```bash
cursor-agent --version
```

A versão detectada deve ser armazenada para diagnóstico.

Exemplo:

```text
Cursor Agent

v2026.07.23
```

---

# 9. Gerenciamento de Sessão

O Cursor Agent cria sessões conversacionais.

O Hero mapeia essas sessões para os estágios do fluxo de trabalho.

Exemplo:

```text
Fluxo de Trabalho

Planejamento

↓

Sessão do Cursor

069c5368-0805...

↓

Resultado da Execução
```

O Runtime é dono do fluxo de trabalho.

O Cursor é dono apenas da sessão de execução.

---

# 10. Criação de Sessão

Para uma nova execução:

```bash
cursor-agent \
  --print \
  --output-format json \
  -p "<prompt>"
```

A resposta JSON contém:

* identificador da sessão;
* resposta gerada;
* métricas de uso.

O adaptador armazena o identificador da sessão dentro do ExecutionResult.

---

# 11. Retomada de Sessão

Se um estágio do fluxo de trabalho exigir interação adicional, o adaptador retoma a sessão existente.

Exemplo:

```bash
cursor-agent \
  --resume=<session-id> \
  --print \
  -p "<prompt>"
```

O adaptador oculta todos os detalhes do CLI do Runtime.

---

# 12. Execução do Prompt

Etapas de execução:

```text
Receber ExecuteRequest

      │

      ▼

Gerar Comando do CLI

      │

      ▼

Executar Processo

      │

      ▼

Capturar Saída JSON

      │

      ▼

Fazer Parsing da Resposta

      │

      ▼

Criar ExecutionResult
```

O Runtime recebe apenas objetos normalizados.

---

# 13. Parsing de JSON

As respostas atuais do Cursor Agent incluem campos semelhantes a:

```json
{
  "type":"result",
  "subtype":"success",
  "session_id":"069c...",
  "result":"Planning completed.",
  "usage":{
    "inputTokens":10421,
    "outputTokens":49,
    "cacheReadTokens":5248
  }
}
```

O adaptador é responsável por mapear esses campos para o modelo interno do Hero.

---

# 14. Mapeamento do Resultado de Execução

Os campos específicos do Cursor são convertidos em:

```go
type ExecutionResult struct {

    Status Status

    SessionID string

    Summary string

    Output string

    Usage Usage

    Duration time.Duration

}
```

O Workflow Engine nunca depende do JSON do Cursor.

---

# 15. Coleta de Uso

O adaptador extrai:

* tokens de entrada;
* tokens de saída;
* tokens de leitura de cache;
* tokens de escrita de cache (se disponíveis);
* duração da execução.

Valores ausentes devem receber valores padrão de forma segura.

---

# 16. Tradução de Erros

Os erros do Cursor CLI nunca devem se propagar diretamente.

Exemplos:

Cursor:

```text
command not found
```

Hero:

```text
HarnessUnavailable
```

---

Cursor:

```text
invalid session
```

Hero:

```text
SessionNotFound
```

---

Cursor:

```text
process timeout
```

Hero:

```text
ExecutionTimeout
```

O Runtime recebe apenas erros normalizados.

---

# 17. Gerenciamento de Processo

O adaptador é dono do processo externo.

Responsabilidades:

* criação de processo;
* captura de stdout;
* captura de stderr;
* tratamento de timeout;
* cancelamento;
* limpeza.

Nenhum subprocesso deve permanecer após a execução.

---

# 18. Tratamento de Timeout

Toda execução deve suportar timeouts configuráveis.

Exemplo:

```yaml
cursor:

  timeout: 30m
```

Se o timeout expirar:

* encerrar o processo;
* liberar recursos;
* retornar ExecutionTimeout.

---

# 19. Cancelamento

O adaptador deve suportar cancelamento.

Fluxo de execução:

```text
Fluxo de Trabalho Cancelado

      │

      ▼

Runtime

      │

      ▼

Cursor Adapter

      │

      ▼

Encerrar Processo
```

O cancelamento deve ser controlado sempre que possível.

---

# 20. Descoberta de Capacidades

O Cursor Adapter deve expor as capacidades suportadas.

Exemplo:

```go
type CursorCapabilities struct {

    Resume bool

    JsonOutput bool

    Streaming bool

    Images bool

    MCP bool

}
```

As capacidades permitem que o Runtime adapte seu comportamento sem conhecer os detalhes internos do Cursor.

---

# 21. Configuração

Exemplo de configuração:

```yaml
runtime:

  harness:

    provider: cursor

cursor:

  executable: cursor-agent

  timeout: 30m

  json_output: true
```

A configuração pertence ao Runtime.

O adaptador apenas a consome.

---

# 22. Logging

O adaptador deve gerar logs estruturados.

Exemplo:

```text
Adaptador inicializado

Versão do Cursor detectada

Sessão criada

Execução iniciada

Execução concluída

Execução falhou
```

Os logs nunca devem conter prompts, credenciais ou dados confidenciais do usuário, a menos que explicitamente habilitados para depuração.

---

# 23. Segurança

O Cursor Adapter nunca deve:

* armazenar tokens de autenticação;
* persistir prompts do usuário fora dos artefatos do fluxo de trabalho;
* expor identificadores de sessão desnecessariamente;
* registrar dados confidenciais do projeto.

Informações sensíveis à segurança permanecem sob controle do Runtime.

---

# 24. Futuro Suporte a Streaming

Se o Cursor Agent CLI introduzir respostas em streaming, o adaptador deve expô-las por meio de eventos normalizados do runtime.

Exemplo:

```text
Cursor Agent

↓

Saída em Streaming

↓

Cursor Adapter

↓

execution.output.chunk

↓

Barramento de Eventos
```

Nenhuma alteração no Workflow Engine deve ser necessária.

---

# 25. Estratégia de Testes

O adaptador deve ser validado por meio de testes de integração automatizados.

Cenários mínimos:

* descoberta do executável;
* verificação de disponibilidade;
* detecção de versão;
* criação de sessão;
* retomada de sessão;
* execução bem-sucedida;
* executável inválido;
* sessão inválida;
* timeout;
* cancelamento;
* parsing de JSON;
* extração de uso.

O uso de mocks deve ser evitado sempre que possível.

A execução real do CLI fornece garantias mais fortes.

---

# 26. Limitações Conhecidas

As limitações atuais do Cursor Agent CLI incluem:

* nenhum mecanismo oficial de assinatura de eventos;
* nenhuma interface direta de callback para o Runtime;
* nenhuma API pública de orquestração de fluxo de trabalho;
* as sessões são separadas do histórico de chat do Cursor IDE;
* as conversas via CLI são independentes da interface gráfica do Cursor.

Essas limitações são intencionalmente encapsuladas dentro do adaptador.

Futuras melhorias do Cursor devem exigir apenas alterações no adaptador.

---

# 27. Evolução Futura

O Cursor Adapter deve evoluir para suportar novas capacidades do Cursor sem afetar o Runtime.

Possíveis melhorias futuras incluem:

* respostas em streaming;
* saída estruturada mais rica;
* metadados de execução de ferramentas;
* eventos de progresso;
* execução remota;
* integração com SDK oficial.

O contrato com o Runtime deve permanecer estável.

---

# 28. Separação de Responsabilidades

O Hero Runtime:

* é dono do ciclo de vida do fluxo de trabalho.

O Workflow Engine:

* é dono das decisões de execução.

O Agent Orchestrator:

* prepara as requisições de execução.

O Cursor Adapter:

* comunica-se com o Cursor Agent CLI.

O Cursor Agent CLI:

* executa tarefas de IA.

Cada camada deve permanecer independente.

---

# 29. Princípios do Adaptador

O Cursor Adapter segue estes princípios.

## Camada de Integração Fina

A lógica de negócio pertence ao Hero, não ao adaptador.

---

## Isolamento de Processo

Todo comportamento específico do Cursor é encapsulado.

---

## Resultados Padronizados

Toda resposta do Cursor é traduzida para o modelo interno do Hero.

---

## Independência do Runtime

O Runtime nunca deve depender de APIs ou estruturas JSON específicas do Cursor.

---

## Compatibilidade Futura

Novas capacidades do Cursor devem exigir apenas alterações no adaptador.

---

# 30. Declaração de Arquitetura

O Cursor Adapter é a ponte de infraestrutura entre o Hero Runtime e o Cursor Agent CLI.

Ele encapsula todo o comportamento específico do Cursor, expondo uma interface de execução estável e padronizada ao Hero Runtime, garantindo que a lógica do fluxo de trabalho, o gerenciamento de estado, o tratamento da conversação e a orquestração permaneçam completamente independentes do harness de codificação com IA subjacente.