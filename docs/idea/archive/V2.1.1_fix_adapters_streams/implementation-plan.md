# Plano de Implementação — V2.1.1 Eventos e Streaming nos Adaptadores

> Implementado em 2026-08-18. Referência de design: [event_streams_improvements.md](event_streams_improvements.md).

## Objetivo

Normalizar eventos de harness (Cursor NDJSON + OpenCode SSE) em `StreamDelta`, eliminar perda silenciosa de eventos, suportar permissões do harness na TUI, e alinhar OpenCode com a paridade de streaming já existente no Cursor.

## Entregas

| Entrega | Caminho |
|---------|---------|
| Contrato de streaming | `internal/harness/stream.go` |
| Normalizador OpenCode | `internal/adapters/opencode/events.go` |
| Endurecimento Cursor | `internal/adapters/cursor/parse.go`, `adapter.go` |
| TUI (warnings, permissão) | `internal/tui/conversation.go`, `app.go`, `status_bar.go` |
| OpenSpec | `openspec/specs/harness-adapter/spec.md`, change `openspec/changes/harness-stream-events/` |
| Arquitetura | `docs/architecture/architecture-overview.md` |

## Fases concluídas

### Fase 1 — Contrato (`internal/harness`)

- Novos `StreamKind`: `warning`, `permission`, `activity`, `session`
- Campos em `StreamDelta`: `HarnessType`, `SessionID`, `Metadata`
- `PermissionRequest` / `PermissionResponse` + `OnPermissionRequest` em `ExecuteRequest`
- Helpers: `WarningDelta`, `SessionDelta`, `ActivityDelta`

### Fase 2 — OpenCode

- `processSSEEvent` com dispatch por `evt.type` para todos os eventos documentados
- `permission.asked` → bloqueio + `POST /permission/{id}/reply`
- `readSSEEvents` emite warning para JSON malformado
- Tipos desconhecidos → `StreamKindWarning`

### Fase 3 — Cursor

- `system` / `user` ignorados silenciosamente (lifecycle)
- Tipos desconhecidos → warning
- Saída sem `result` + exit code ≠ 0 → erro com stderr

### Fase 4 — TUI

- `appendStreamDelta` para warning/activity/session
- `harnessPermissionPending` + prompt no status bar (`Allow? [y/N]`)
- Wire `OnPermissionRequest` em `startConversationExecute`

### Fase 5 — Documentação

- OpenSpec atualizado
- `architecture-overview.md` atualizado
- Context files atualizados

## Fora de escopo (V2.1.2+)

- Cursor CLI hooks (`beforeShellExecution`, `afterFileEdit`, …)
- Persistência de eventos de atividade em SQLite `events`
- Engine consumindo eventos normalizados diretamente

## Critérios de aceite

- [x] OpenCode: handlers para tipos documentados em `event_streams_improvements.md`
- [x] Nenhum evento ignorado silenciosamente (warning visível)
- [x] `permission.asked` com prompt TUI e reply HTTP
- [x] Thinking/tools OpenCode no chat
- [x] `session.error` encerra com erro
- [x] Cursor sem `result` + exit ≠ 0 falha explicitamente
- [x] `go test ./...` passa
