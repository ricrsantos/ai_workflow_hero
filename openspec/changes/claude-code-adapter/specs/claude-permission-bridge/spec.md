## Purpose

Preserve Hero's interactive permission semantics for Claude headless turns through a constrained, one-turn bridge that cannot become a general model tool or credential store.

## ADDED Requirements

### Requirement: Permission profiles SHALL map explicitly to Claude behavior

Claude SHALL reuse the project `ask`, `auto-project`, and `auto-all` profiles. `ask` SHALL use Claude's permission-prompt mechanism and a per-execution local/stdio bridge; `auto-project` SHALL map to `acceptEdits` without granting shell, network, MCP, or external-path access; and `auto-all` SHALL map to the native bypass equivalent with explicit Hero risk confirmation (PRD-C13-001 §2; ADR-072).

#### Scenario: Ask profile requests a decision
- **WHEN** Claude requests permission under `ask`
- **THEN** Hero shows the existing permission decision surface and does not auto-approve or silently deny the request

#### Scenario: Auto-project is constrained
- **WHEN** Claude runs under `auto-project`
- **THEN** Hero passes the project-edit permission mode and does not silently grant shell, network, MCP, or external-path permissions

#### Scenario: Auto-all requires risk confirmation
- **WHEN** a user selects `auto-all`
- **THEN** Hero requires the existing explicit risk confirmation before allowing the native bypass mode

### Requirement: The ask bridge SHALL be one-time authenticated and permission-only

The bridge SHALL authenticate each request with a random one-time token, accept only permission requests, convert them to `harness.PermissionRequest`, wait for the TUI or Telegram-backed decision callback, and return the normalized decision. It SHALL reject invalid tokens and non-permission messages, store no credential, expose no general-purpose model tool, and terminate with the Claude child (PRD-C13-001 §2; ADR-072).

#### Scenario: Valid permission request
- **WHEN** a request presents the execution token and a supported permission payload
- **THEN** the bridge forwards the normalized request, waits for the decision, and returns the approved or denied result to Claude

#### Scenario: Invalid token
- **WHEN** a request presents a missing, reused, or incorrect token
- **THEN** the bridge rejects it without forwarding a TUI permission prompt

#### Scenario: Non-permission message
- **WHEN** a caller sends a general tool, model, or credential operation to the bridge
- **THEN** the bridge rejects the message and does not expose that operation to Claude

#### Scenario: Telegram-backed decision
- **WHEN** the TUI permission flow is forwarded to a paired Telegram conversation
- **THEN** the same normalized approval/denial callback completes the pending Claude request without exposing tokens or credentials

### Requirement: Unsupported ask protocol SHALL fail closed

Before the main Claude adapter path is enabled, a fake-process/fixture protocol check SHALL verify the installed-version wire schema and semantics of `--permission-prompt-tool`. If the mechanism is unsupported or malformed, `ask` SHALL fail explicitly and SHALL never downgrade to `auto-project` or `auto-all` (PRD-C13-001 §2; ADR-072).

#### Scenario: Protocol fixture succeeds
- **WHEN** the fake Claude process emits the documented permission-prompt request and accepts the normalized decision
- **THEN** the adapter records the protocol as supported and the `ask` path is available to later adapter tests

#### Scenario: Protocol is unsupported
- **WHEN** the installed CLI rejects the permission-prompt flag or emits an incompatible request schema
- **THEN** `ask` fails with actionable incompatibility guidance and no permissive profile is attempted

#### Scenario: Bridge cleanup after failure
- **WHEN** a permission request, decision callback, or Claude child fails
- **THEN** the bridge closes and its token becomes unusable before Execute returns

