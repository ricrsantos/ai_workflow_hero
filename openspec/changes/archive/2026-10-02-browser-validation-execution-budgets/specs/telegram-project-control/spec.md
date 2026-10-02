# telegram-project-control Specification (delta)

## Purpose
Image-delivery edge for cycle screenshots under the existing addressed IPC/outbound path. ADR-106; UI §5; PRD FR-12–FR-13.

## MODIFIED Requirements

### Requirement: Image delivery SHALL follow always_send with addressed isolation
The daemon alone SHALL call Bot API using existing vault credentials. Actual screenshot images SHALL forward only when project `always_send` is enabled and the addressed chat is paired/connected; `todos` SHALL use ordered bounded batches with progress, partial-failure IDs, and durable sanitized delivery IDs for retry/dedup. Local cards SHALL survive disconnected or failed delivery; optional delivery failure SHALL NOT block validation. No direct agent-to-Telegram sending SHALL exist; no credential entry over Telegram SHALL be added.

#### Scenario: Partial batch failure
- **WHEN** 2 of 5 forwarded images fail delivery
- **THEN** the chat reports the 2 failed IDs for retry and validation continues on local evidence
