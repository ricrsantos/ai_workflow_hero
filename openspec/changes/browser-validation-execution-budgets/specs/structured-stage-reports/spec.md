# structured-stage-reports Specification (delta)

## Purpose
Blocked outcomes and validated coverage for Browser UI/E2E typed reports. PRD-C17-001 FR-07–FR-08; ADR-104.

## MODIFIED Requirements

### Requirement: Browser reports SHALL support durable blocked outcomes
Browser UI/E2E typed reports SHALL accept `passed`, `failed`, and `blocked`, with validated preparation and coverage metadata. Missing/invalid credentials, insufficient access, disabled required setup, unavailable browser/tool/service, missing declared suite/fixtures, and unsupported interactive login SHALL produce prerequisite blockers, never application findings. Each blocker SHALL carry a stable reason, affected coverage/profile IDs, diagnostic uncertainty where relevant, and a next action. Blocked SHALL stop auto-advance and automatic Implementation repair. Every notification SHALL include stage, concrete reason, affected IDs, exact corrective action, and `/hero-continue`; credential values SHALL never appear.

#### Scenario: Missing administrator account
- **WHEN** screens screen-02 and screen-03 require profile admin and user administrator is unusable
- **THEN** the stage blocks naming the stage, reason, coverage IDs, the Config/file correction, and `/hero-continue`

### Requirement: Mixed reports SHALL preserve genuine findings while blocked
A mixed validated report transaction SHALL persist genuine C15 findings/occurrences (identity, repro admission, owner, loop ceilings unchanged) together with blocker/coverage data, set Blocked, and schedule no repair until explicit unblock. After unblock, existing correction SHALL apply before outstanding coverage passes. Findings SHALL never be lost, prerequisites SHALL never become owner assignments, and incomplete coverage SHALL never be approved.

#### Scenario: Real defect found before prerequisite blocks
- **WHEN** validation records one genuine finding and then blocks on a missing fixture
- **THEN** the finding survives with its identity and the stage stays blocked until `/hero-continue` plus correction

### Requirement: Coverage accounting SHALL gate passing
Reports SHALL account planned, executed, passed, failed, blocked, and optional/skipped items with known denominators and reasons. No stage SHALL pass with a mandatory item unexecuted, skipped, or blocked. Mandatory scope changes SHALL require explicit user approval and an updated plan. Missing optional reference PNGs SHALL warn without findings or Implementation loops. E2E SHALL assert business outcomes; navigation alone SHALL NOT complete an item.

#### Scenario: Login-only evidence against protected scope
- **WHEN** only the login screen has evidence while protected screens are mandatory
- **THEN** the gate refuses to pass and lists the unvalidated mandatory coverage IDs
