# telegram-ipc Specification

## Purpose

Versioned local IPC between the Telegram daemon and Hero TUIs (ADR-060).

## ADDED Requirements

### Requirement: IPC SHALL use an OS-user-private endpoint

The daemon SHALL listen on a socket/pipe under the user’s Hero state directory with permissions restricted to the effective UID. Clients running as another user SHALL be rejected (ADR-060).

#### Scenario: Wrong user cannot connect
- **WHEN** a client with a different UID opens the socket
- **THEN** the connection is rejected without exposing registration data

### Requirement: Registration SHALL assign stable instance addresses

On `register`, the daemon SHALL return the allocated address (`base`, `base_2`, …, or `free_N`) atomically under concurrent registrations. Suffixes SHALL remain stable while any sibling instance for the same project/free-chat pool is connected (PRD-C09-001 §3.2).

#### Scenario: Two project TUIs get base and _2
- **WHEN** two TUIs for the same project register concurrently
- **THEN** one receives `myproj` and the other `myproj_2` without collision

#### Scenario: Suffix reused after last instance exits
- **WHEN** all instances for a project unregister
- **THEN** the next registration MAY receive the base abbreviation again

### Requirement: Live delivery SHALL use push events with acknowledgements

The daemon SHALL push `inbound` and lifecycle `event` frames to registered clients. Clients SHALL send `ack_delivery` for idempotent delivery tracking. Protocol version mismatch SHALL close the connection with an explicit error (design D2).

#### Scenario: Reconnect receives pending deliveries
- **WHEN** a TUI reconnects after an outage
- **THEN** the daemon pushes queued `inbound` messages once and waits for `ack_delivery`

#### Scenario: Incompatible client is rejected
- **WHEN** a TUI with an older protocol version registers
- **THEN** the daemon returns an error instructing upgrade/reinstall
