# Hero Adapters — Eventos e Streaming

## Objetivo

O `HarnessAdapter` deve transformar os eventos específicos de cada Harness em eventos normalizados para o Hero. Atualmente o único adapter implementado no Hero é o do Opencode, e ele apresenta vários problemas devido a não tratar todos os eventos, por exemplo:

1. Os agentes acionados pelo harness não são refletidos na TUI do Hero
2. As solicitações de aprovação de acesso do Harness não chegam no Hero
3. A cadeia de pensamento e os outputs dos agentes / modelos não são plotados no chat do Hero
4. O processo "fica parado indefinidamente" porque o harness necessita de alguma ação do usuário e o Hero não informa ao user.

O Hero **nunca deve ignorar silenciosamente um evento desconhecido**. Eventos não reconhecidos devem gerar um `WARNING` visível ao usuário e ser registrados no log, permitindo evolução do adapter sem perda silenciosa de informação.

## OpenCode

O adapter consome o SSE `GET /event` (instância) ou o stream global equivalente. A lista canônica de eventos está em [`packages/schema/src/event-manifest.ts`](https://github.com/anomalyco/opencode/blob/dev/packages/schema/src/event-manifest.ts) (`EventManifest.Definitions`, branch `dev`).

### Formato do payload SSE

Cada evento é um JSON com:

```json
{
  "id": "evt_…",
  "type": "<event-type>",
  "properties": { }
}
```

Eventos duráveis podem chegar também encapsulados:

```json
{
  "type": "sync",
  "properties": {
    "syncEvent": {
      "id": "evt_…",
      "type": "<inner-type>",
      "seq": 1,
      "aggregateID": "…",
      "data": { }
    }
  }
}
```

O adapter deve desencapsular `sync` e processar o evento interno.

### Eventos de transporte (fora do schema)

Emitidos diretamente pelos handlers SSE, não constam em `EventManifest`:

| Evento | Descrição |
|---|---|
| `server.connected` | Primeiro evento após conexão |
| `server.heartbeat` | Keepalive (~10s); ignorar no Hero |
| `server.instance.disposed` | Instância encerrada; encerrar stream |
| `global.disposed` | Servidor global encerrado |
| `sync` | Wrapper de eventos duráveis (desencapsular) |

O OpenCode também disponibiliza `GET /global/health` para verificar a saúde do servidor.

### Eventos do schema (`EventManifest.Definitions`)

#### Command

* `command.executed`

#### File

* `file.edited`
* `file.watcher.updated`

#### Catalog / Integration / Reference / Plugin / Project

* `catalog.updated`
* `integration.updated`
* `integration.connection.updated`
* `reference.updated`
* `plugin.added`
* `project.updated`
* `project.directories.updated`

#### Installation

* `installation.updated`
* `installation.update-available`

#### LSP

* `lsp.updated`

> `lsp.client.diagnostics` foi removido na branch `dev`.

#### MCP

* `mcp.browser.open.failed`
* `mcp.tools.changed`

#### Message (v1)

* `message.updated`
* `message.removed`
* `message.part.updated`
* `message.part.removed`
* `message.part.delta`

#### Permission

* `permission.asked` / `permission.replied` (v1)
* `permission.v2.asked` / `permission.v2.replied` (v2)

#### Question

* `question.asked` / `question.replied` / `question.rejected` (v1)
* `question.v2.asked` / `question.v2.replied` / `question.v2.rejected` (v2)

#### Session (v1)

* `session.created`
* `session.updated`
* `session.deleted`
* `session.compacted`
* `session.diff`
* `session.error`
* `session.idle`
* `session.status`

#### Session (v2 — `session.next.*`)

Streaming e ciclo de vida da sessão no modelo EventV2:

* `session.next.agent.switched`
* `session.next.model.switched`
* `session.next.moved`
* `session.next.prompted`
* `session.next.prompt.admitted`
* `session.next.context.updated`
* `session.next.synthetic`
* `session.next.shell.started` / `session.next.shell.ended`
* `session.next.step.started` / `session.next.step.ended` / `session.next.step.failed`
* `session.next.text.started` / `session.next.text.delta` / `session.next.text.ended`
* `session.next.reasoning.started` / `session.next.reasoning.delta` / `session.next.reasoning.ended`
* `session.next.tool.input.started` / `session.next.tool.input.delta` / `session.next.tool.input.ended`
* `session.next.tool.called` / `session.next.tool.progress` / `session.next.tool.success` / `session.next.tool.failed`
* `session.next.retried`
* `session.next.compaction.started` / `session.next.compaction.delta` / `session.next.compaction.ended`
* `session.next.revert.staged` / `session.next.revert.cleared` / `session.next.revert.committed`

> `tool.execute.before` / `tool.execute.after` foram substituídos por `session.next.tool.*`.
> `shell.env` não é evento SSE — é hook de plugin (`plugin.trigger("shell.env", …)`).

#### Todo / TUI / PTY / VCS / Workspace / Worktree / IDE

* `todo.updated`
* `tui.prompt.append`
* `tui.command.execute`
* `tui.toast.show`
* `tui.session.select`
* `pty.created` / `pty.updated` / `pty.exited` / `pty.deleted`
* `vcs.branch.updated`
* `workspace.ready` / `workspace.status` / `workspace.failed`
* `worktree.ready` / `worktree.failed`
* `ide.installed`

#### Server (schema)

* `server.connected`
* `global.disposed`

### Regras do adapter

1. **Streaming v2 (preferencial):** `session.next.text.delta` → texto; `session.next.reasoning.delta` → thinking; `session.next.tool.*` e `session.next.shell.*` → atividade de ferramenta.
2. **Streaming v1 (legado):** `message.part.updated` e `message.part.delta` devem permitir reconstruir a resposta incremental.
3. `permission.asked` e `permission.v2.asked` devem ser convertidos em solicitação de aprovação do Hero.
4. `session.status`, `session.idle` e `session.error` alimentam o estado da Etapa.
5. `session.diff`, `file.edited` e `todo.updated` preservados para observabilidade.
6. `session.next.agent.switched` atualiza o agente ativo exibido na TUI.
7. Eventos TUI tratados mesmo sem impacto visual imediato.
8. `server.heartbeat` ignorado; `server.instance.disposed` encerra o stream.
9. Eventos `sync` desencapsulados antes do roteamento.
10. **Evento desconhecido:**
   * registrar `WARNING`;
   * informar o tipo do evento;
   * preservar o payload bruto quando possível;
   * continuar a execução;
   * nunca falhar silenciosamente.

Exemplo:

```text
WARNING Harness event not recognized
harness: opencode
event: <event-type>
session: <session-id>
payload: <raw payload>
```

A lista de eventos deve ser considerada versionada: novas versões do OpenCode podem introduzir novos eventos. Fonte: `EventManifest.Definitions` em `anomalyco/opencode` branch `dev`.

### Modo debug (`hero --debug`)

Alguns eventos de observabilidade são suprimidos no chat por padrão e só aparecem com `--debug`:

* `session.updated`
* `session.diff`
* `plugin.added`
* `reference.updated`
* `integration.updated`
* `catalog.updated` (sem dados úteis)
* part types `step-start` / `step-finish` (marcadores de etapa)
* part type `tool`

Exceções fora do modo debug:

* `catalog.updated` com dados: exibir título e lista de elementos do catálogo.
* Conteúdo de texto/reasoning **entre** `step-start` e `step-finish` continua visível; marcadores de etapa são ocultados.

### Streaming (OpenCode `session.next.*`)

Conforme o schema oficial (`packages/schema/src/session-event.ts`):

> *Stream fragments are live-only; Text.Ended / Reasoning.Ended is the replayable full-value boundary.*

O adapter **não** imprime `*.delta` diretamente no chat (tokens sem espaço). Emite:

* `message.part.updated` com texto crescente (v1) — streaming principal da resposta
* `session.next.text.ended` / `session.next.reasoning.ended` — texto completo autoritativo (v2)

## Cursor CLI

O adapter deve utilizar `--print --output-format stream-json` para execução headless.

Os eventos documentados atualmente incluem:

* `system` / `init`
* `user`
* `assistant`
* `tool_call` / `started`
* `tool_call` / `completed`
* `result` / `success`

O stream é NDJSON e o `result` é o evento terminal esperado em uma execução bem-sucedida. Em caso de falha, o processo pode terminar sem emitir `result`, portanto o adapter também deve considerar `exit code`, `stderr` e término inesperado do processo.

O Cursor documenta ainda hooks de CLI como:

* `sessionStart`
* `beforeShellExecution`
* `afterShellExecution`
* `afterFileEdit`
* `postToolUse`
* `stop`

Os hooks devem ser utilizados quando forem necessários eventos que não estejam disponíveis adequadamente no `stream-json`.

## Normalização

O Hero não deve depender dos nomes dos eventos dos Harnesses.

Exemplos de eventos internos:

```text
HarnessStarted
HarnessMessage
HarnessThinking
HarnessToolStarted
HarnessToolCompleted
HarnessPermissionRequired
HarnessActivity
HarnessIdle
HarnessCompleted
HarnessFailed
HarnessWarning
```

Fluxo:

```text
Harness
   ↓
HarnessAdapter
   ↓
Normalização
   ↓
Hero Engine
   ↓
UI / Ciclo / Etapa
```

O adapter deve manter a informação específica do Harness quando necessária para diagnóstico, mas o Engine deve trabalhar predominantemente com eventos normalizados.
