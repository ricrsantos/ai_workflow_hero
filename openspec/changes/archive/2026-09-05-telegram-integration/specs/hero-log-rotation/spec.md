# hero-log-rotation Specification

## Purpose

Rotating project and daemon logs with safe migration from the legacy TUI log path (PRD-C09-001 §3.5; ADR-064).

## ADDED Requirements

### Requirement: Project TUI logs SHALL rotate under `.workflow-hero/logs/`

Technical TUI logs SHALL write to `.workflow-hero/logs/tui.log` with rotation at 10 MB retaining at most ten files. Upgrade/install SHALL migrate `.workflow-hero/tui.log` when the legacy file exists and the new path does not (ADR-064).

#### Scenario: Legacy log migrates on upgrade
- **WHEN** upgrade finds `.workflow-hero/tui.log` and no `logs/tui.log`
- **THEN** the legacy file is moved safely and logging continues at the new path

#### Scenario: Rotation retains ten files
- **WHEN** the active log exceeds 10 MB
- **THEN** it rotates and at most ten historical files remain

### Requirement: Daemon logs SHALL rotate globally under the user Hero directory

The daemon SHALL write to `~/.workflow-hero/logs/telegram-daemon.log` with the same 10 MB × 10 file policy (ADR-064).

#### Scenario: Daemon rotation is independent of project logs
- **WHEN** multiple projects register with one daemon
- **THEN** daemon diagnostics accumulate in the user-global rotated log only

### Requirement: Log output SHALL redact Telegram credentials

Rotating writers SHALL apply the shared redaction helper before persisting any line (PRD-C09-001 §3.5).

#### Scenario: Inbound audit line omits chat id when marked secret
- **WHEN** the daemon logs an authorization rejection
- **THEN** the authorized chat id is not present in plaintext
