# Hero Adapters — Health Check e Harness Watchdog

## Objetivo

Nenhuma Etapa do Hero deve permanecer indefinidamente aguardando um Harness.

O Hero deve possuir um **Harness Watchdog** genérico capaz de detectar:

1. processo morto;
2. server indisponível;
3. sessão inexistente ou inválida;
4. ausência prolongada de atividade;
5. stream de eventos interrompido;
6. término inesperado do Harness;
7. execução aparentemente travada.

## Contrato genérico

O `HarnessAdapter` deve expor:

```go
type HarnessHealth struct {
    ProcessAlive   bool
    ServerAlive    bool
    SessionAlive   bool
    LastEventAt    time.Time
    LastActivityAt time.Time
    Status         HealthStatus
    Details        string
}

type HealthStatus string

const (
    HealthHealthy   HealthStatus = "healthy"
    HealthDegraded  HealthStatus = "degraded"
    HealthSuspected HealthStatus = "suspected_hang"
    HealthFailed    HealthStatus = "failed"
)
```

O Watchdog deve executar verificações periódicas e manter `lastEventAt` / `lastActivityAt`.

### Regra importante

**Ausência de eventos não significa automaticamente que o Harness travou.**

O watchdog deve combinar:

```text
processo
   +
server
   +
sessão
   +
última atividade
   +
estado conhecido
```

antes de declarar `suspected_hang`.

Quando houver suspeita:

```text
SUSPECTED_HANG
      ↓
health check
      ↓
┌───────────────┐
│ Harness vivo? │
└───────┬───────┘
        │
   ┌────┴─────┐
   │          │
  SIM        NÃO
   │          │
   ↓          ↓
degraded    FAILED
```

O usuário deve ser informado e a Etapa nunca deve permanecer esperando indefinidamente.

Possíveis ações:

```text
[ Tentar novamente ]
[ Reiniciar Harness ]
[ Cancelar Etapa ]
```

## OpenCode

Como o OpenCode Server é um processo HTTP, o adapter deve monitorar:

### 1. Processo

Manter PID e verificar se o processo continua vivo.

### 2. Server

Executar periodicamente:

```text
GET /global/health
```

O endpoint retorna `healthy` e a versão do servidor.

### 3. SSE

Monitorar o stream:

```text
GET /event
```

O recebimento de eventos atualiza `lastEventAt`.

### 4. Sessão

Consultar/acompanhar o estado da sessão e utilizar:

* `session.status`
* `session.idle`
* `session.error`
* `session.updated`

como sinais adicionais de saúde.

### 5. Atividade

Eventos como:

* `message.part.updated`
* `tool.execute.before`
* `tool.execute.after`
* `file.edited`

atualizam `lastActivityAt`.

### 6. Recovery

Se o processo morreu:

```text
FAILED → restart server → health check → READY
```

Se o processo está vivo mas o servidor não responde:

```text
DEGRADED → tentar recuperação → restart se necessário
```

Se servidor responde mas a sessão está aparentemente travada:

```text
SUSPECTED_HANG
       ↓
informar usuário
       ↓
permitir abort/restart/retry
```

## Cursor CLI

O Cursor CLI headless é diferente do OpenCode: não existe um server persistente equivalente que o Hero precise monitorar. O processo `cursor-agent` representa a execução.

O watchdog deve monitorar:

### 1. Processo

* PID;
* processo ainda vivo;
* exit code;
* término inesperado;
* sinal recebido.

### 2. Stream

Monitorar o `stream-json`.

O Cursor informa que o stream pode terminar sem um evento `result` em caso de falha. Portanto:

```text
process exited
+
não recebeu result
=
execução anormal
```

deve ser tratado como falha, e não como sucesso.

### 3. Atividade

Atualizar `lastActivityAt` ao receber:

* `assistant`;
* `tool_call`;
* `user`;
* `system`;
* `result`.

### 4. Stalled process

Se o processo permanece vivo mas não produz saída por período configurado:

```text
SUSPECTED_HANG
```

O Hero deve informar o usuário e permitir:

```text
[ Aguardar mais ]
[ Cancelar ]
[ Reiniciar ]
```

O timeout deve ser configurável e não excessivamente agressivo, pois o Cursor pode passar algum tempo sem emitir conteúdo durante conexão/resolução do modelo. A própria documentação recomenda `stream-json` para acompanhamento em tempo real.

## Princípio geral

O Harness Watchdog pertence ao **Hero**, enquanto os detalhes de implementação pertencem ao `HarnessAdapter`.

```text
                  Hero
                   │
            Harness Watchdog
                   │
        ┌──────────┴──────────┐
        │                     │
 OpenCode Adapter       Cursor Adapter
        │                     │
 process/server/session      process/stream
        │                     │
        └──────────┬──────────┘
                   ↓
            HarnessHealth
```

O objetivo é garantir:

> **Nenhum Harness pode bloquear indefinidamente uma Etapa do Hero sem que o usuário seja informado.**
