## ADDED Requirements

### Requirement: Runtime user-facing vocabulary SHALL be slash-first
User-visible strings in Runtime assets (`hero-*.md`, orchestration skill / agent guidance) SHALL prefer the Hero 0.9 slash set (`/hero:new`, `/hero:start`, `/hero:approve`, `/hero:reject`, `/hero:cancel`, `/hero:finish`, `/hero:archive`, `/hero:resume`, `/hero:sync`, `/hero:status`, `/hero:continue`, `/hero:back`, `/hero:help`). CLI verbs MAY appear as secondary implementation detail for agents but MUST NOT be the primary user CTA (PRD-C02-001 §5.1; ADR-020).

#### Scenario: Post-configuration handoff
- **WHEN** configuration review completes and the user should start the cycle in a clean chat
- **THEN** the primary CTA tells the user to run `/hero:start` (not “confirm here so I run `hero cycle new`” as the primary message)

#### Scenario: Approve guidance uses slash form
- **WHEN** Runtime instructs the user to approve a pending stage
- **THEN** the user-facing instruction uses `/hero:approve`

### Requirement: Runtime archive guidance SHALL include OpenSpec force path
`/hero:archive` assets SHALL document the coupled OpenSpec archive sequence and, on OpenSpec failure, the force path plus manual `openspec archive <name> -y` instructions (PRD-C02-001 §5.4; UI-C02-001 §4).

#### Scenario: Archive asset mentions force flags
- **WHEN** an agent follows `/hero:archive` after OpenSpec failure
- **THEN** guidance includes retry, `hero cycle archive --force` / `--skip-openspec`, and the manual OpenSpec command
