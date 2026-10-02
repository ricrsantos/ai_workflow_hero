# browser-test-access Specification

## Purpose
Confirmed C17 target contract for authenticated browser validation. Implementation is pending; PRD-C17-001 and ADR-C17-002 are canonical design references.

## Requirements

### Requirement: Test credentials SHALL use one root dotenv source and local editor
Project-root .env.hero SHALL hold arbitrary indexed test users with login/password/profile. Root .env.hero.example SHALL contain only committed placeholders. Optional Config Test users SHALL edit the same source with masked passwords, safe atomic 0600 writes, idempotent root ignore, format/tracking/path validation and stale-edit rejection. Authentication required/unresolved SHALL never be waived by disabled setup or absent accounts. Parser SHALL read data without shell evaluation or variable expansion.

#### Scenario: External edit conflicts with Config
- **WHEN** manual edits modify .env.hero after a Config draft loads
- **THEN** Save refuses overwrite, preserves the newer file and directs Reload

#### Scenario: Required account missing
- **WHEN** a planned protected screen lacks a usable assigned user
- **THEN** preparation blocks with affected IDs and exact Config/file correction plus /hero-continue, never printing credential values

### Requirement: Authentication SHALL prepare the selected isolated context privately
A deterministic configured executor SHALL consume only selected credentials, bind login to approved test destinations and verify protected access in the same context used for tests. Each execution SHALL authenticate afresh; persisted auth state and cross-stage reuse SHALL be excluded. Unsupported interactive MFA/SSO/CAPTCHA SHALL block. Agents SHALL receive only identifiers and sanitized outcomes. No secret values SHALL enter ordinary prompts/tool arguments, SQLite, YAML, logs, Telegram or captures. A helper SHALL not be represented as a sandbox against unrestricted harness filesystem access.

#### Scenario: Separate browser login is not valid preparation
- **WHEN** the chosen tool cannot continue testing in the prepared context
- **THEN** the method blocks with integration instructions instead of exposing passwords to the agent or claiming authenticated coverage

### Requirement: Preparation SHALL be bounded and planned
Planning SHALL declare the test environment, methods/commands, accounts, fixtures, compatible versions and coverage. Default preparation ceiling SHALL be 120 seconds with at most two attempts per prerequisite, capped by remaining active stage time. Browser-tool availability SHALL be distinct from suite availability. QA SHALL not install frameworks or silently fall back to HTTP. Coverage SHALL use exact users and isolated contexts; unsafe shared-fixture execution SHALL be sequential.

#### Scenario: Missing project suite but usable browser automation
- **WHEN** the project lacks playwright.config but Planning explicitly chose a usable exploratory tool
- **THEN** actual stage-session navigation capability is checked and suite absence alone does not block that mode

### Requirement: Archive SHALL clean credentials without archiving secrets
Successful archive SHALL remove only safe exact root .env.hero after prerequisites succeed and retain safe cycle screenshots. Missing file SHALL succeed idempotently; unsafe path or removal failure SHALL leave archive pending with safe retry instructions. Finish/cancel/upgrade/uninstall SHALL preserve credentials; uninstall SHALL disclose retention. Resume SHALL require credentials to be configured again.

#### Scenario: OpenSpec archive fails
- **WHEN** a required OpenSpec archive prerequisite fails
- **THEN** .env.hero remains and Hero does not report archive success

#### Scenario: Credential cleanup fails
- **WHEN** exact-file removal cannot complete
- **THEN** archive is pending, no secret archive copy is made and correction/retry is explicit
