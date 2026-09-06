# telegram-vault Specification

## Purpose

OS credential vault isolation for Telegram bot token and authorized chat id (PRD-C09-001 §3.4; ADR-062).

## ADDED Requirements

### Requirement: Secrets SHALL live only in the OS credential vault

The bot token and authorized chat id SHALL be stored together through a vault abstraction available to the daemon only. Project/global SQLite SHALL store non-sensitive configuration and queue/audit fields only (ADR-062).

#### Scenario: Settings never reads secret values
- **WHEN** Telegram is configured
- **THEN** Settings displays `Configured` without rendering token or chat id

#### Scenario: Vault clear removes authorization
- **WHEN** the user chooses Clear in Settings
- **THEN** the vault entry is removed and pending pairing state is invalidated

### Requirement: Unsupported vault SHALL fail setup explicitly

If the OS vault is unavailable, pairing/setup SHALL error with remediation guidance. The implementation SHALL NOT silently fall back to environment variables or plaintext files (ADR-062).

#### Scenario: Headless fake vault in tests
- **WHEN** tests inject an in-memory vault
- **THEN** pairing and send paths operate without contacting the OS vault

### Requirement: All serializers SHALL redact secrets

Logs, errors, IPC debug payloads, and diagnostic commands SHALL pass through redaction that removes token-like strings and authorized chat ids (PRD-C09-001 §3.4).

#### Scenario: Bot API error is redacted
- **WHEN** the daemon logs a Bot API failure that echoes the request URL
- **THEN** the token query parameter never appears in the log line
