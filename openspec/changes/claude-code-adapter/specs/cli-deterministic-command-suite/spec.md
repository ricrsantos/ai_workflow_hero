## ADDED Requirements

### Requirement: Deterministic CLI diagnostics SHALL account for Claude without starting it

Install, upgrade, Doctor, Status, help, and harness-state operations SHALL recognize `claude` as an opt-in supported harness. Doctor SHALL warn when enabled Claude is missing or below 2.1.261, while disabled Claude SHALL not make Doctor fail. These commands SHALL not create a Claude session, authenticate, or launch login (PRD-C13-001 §§2–3; UI-C13-001 §2).

#### Scenario: Disabled Claude is warn-free
- **WHEN** a project has `harnesses.claude.enabled=false` and no Claude CLI
- **THEN** deterministic Doctor and Status retain the existing healthy result without attempting Claude execution

#### Scenario: Enabled CLI is unavailable
- **WHEN** Claude is enabled but missing or incompatible on PATH
- **THEN** Doctor emits a warn-only actionable diagnostic naming Claude and the required version, while unrelated harness checks continue

#### Scenario: Status exposes Claude state
- **WHEN** the user runs Status in table or JSON mode
- **THEN** the output identifies Claude's enabled state, configured native model, and availability diagnostic without exposing credentials

