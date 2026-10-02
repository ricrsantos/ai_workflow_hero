# browser-test-access Specification (delta)

## Purpose
Implementation delta on the confirmed C17 target contract. PRD-C17-001 FR-01–FR-04; ADR-103.

## MODIFIED Requirements

### Requirement: Test credentials SHALL use one root dotenv source and local editor
The `internal/testaccess` slice SHALL parse, validate, and serialize project-root `.env.hero` directly as data (never sourced, never shell-expanded). Arbitrary indexed users SHALL be supported with IDs matching `[a-z][a-z0-9_]*`; duplicate IDs/keys and ambiguous normalization SHALL be rejected, never merged. Required accounts with missing or empty login/password/profile SHALL be unusable. Writes SHALL be atomic 0600 with no residual backups; root-anchored `/.env.hero` gitignore insertion SHALL be idempotent and content-preserving; tracked files SHALL block use with removal instructions; symlinks and unsafe targets SHALL be refused. Stale Config drafts SHALL be refused with a Reload instruction.

The Test users opt-in SHALL persist as the top-level `test_access.enabled` boolean in `workflow-config.yml`, default `false` when absent. Config and execution SHALL use this same setting. Disabled preparation for a required login SHALL block with enable instructions; no credential values SHALL enter this configuration.

#### Scenario: Persistent opt-in survives a new session
- **WHEN** Config saves Test users as enabled and a new TUI session loads the cycle
- **THEN** Config and required-login preparation use the saved `test_access.enabled` value

#### Scenario: Required login with setup disabled
- **WHEN** required authenticated coverage is planned and `test_access.enabled` is false or absent
- **THEN** preparation blocks with instructions to enable Test users and configure the required account

#### Scenario: External edit conflicts with Config
- **WHEN** manual edits modify .env.hero after a Config draft loads
- **THEN** Save refuses overwrite, preserves the newer file and directs Reload

#### Scenario: Quoted special values round-trip
- **WHEN** passwords contain spaces, `#`, quotes, backslashes, `$`, or newlines
- **THEN** serialization round-trips them exactly and diagnostics never include values

#### Scenario: Required account missing
- **WHEN** a planned protected screen lacks a usable assigned user
- **THEN** preparation blocks with affected coverage IDs and exact Config/file correction plus `/hero-continue`, never printing credential values

### Requirement: Authentication SHALL prepare the selected isolated context privately
A deterministic configured executor SHALL consume only the selected account, bind the non-secret form-login recipe to approved origins, authenticate afresh every execution, verify protected access in the same context used for tests, and return only sanitized outcomes. Recipe structure SHALL be shared; its application-specific values SHALL be resolved dynamically during Research/Planning and SHALL NOT be hardcoded globally by Hero. Expected denial in a negative authorization test SHALL pass; a broken login for an independently known-valid account MAY be recorded as an application finding. Unsupported interactive MFA/CAPTCHA/SSO SHALL block with a specific next action. Sentinel secrets SHALL be absent from prompts, tool arguments, SQLite, YAML, logs, events, Telegram, command lines, diagnostics, and captures.

#### Scenario: Separate browser login is not valid preparation
- **WHEN** the chosen tool cannot continue testing in the prepared context
- **THEN** the method blocks with integration instructions instead of exposing passwords or claiming authenticated coverage

### Requirement: Archive SHALL clean credentials without archiving secrets
Successful archive SHALL remove only the exact project-root `.env.hero` after archive prerequisites succeed; missing SHALL succeed idempotently. Failed OpenSpec archive, refused Hero archive, unsafe targets, or removal failure SHALL leave archive pending with safe retry instructions. Finish, cancel, upgrade, and uninstall SHALL preserve the file; uninstall SHALL disclose retention. Screenshots SHALL be retained.

#### Scenario: OpenSpec archive fails
- **WHEN** a required OpenSpec archive prerequisite fails
- **THEN** `.env.hero` remains and Hero does not report archive success

#### Scenario: Credential cleanup fails
- **WHEN** exact-file removal cannot complete
- **THEN** archive is pending, no secret archive copy is made and correction/retry is explicit
