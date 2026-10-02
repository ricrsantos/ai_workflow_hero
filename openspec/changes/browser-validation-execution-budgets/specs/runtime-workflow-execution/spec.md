# runtime-workflow-execution Specification (delta)

## Purpose
Durable Blocked state, `/hero-continue` recheck semantics, and active budget enforcement in the TUI scheduler. PRD-C17-001 FR-08–FR-10; ADR-104–105.

## MODIFIED Requirements

### Requirement: Blocked SHALL be durable scheduler state
The engine SHALL persist operational Blocked/interrupted reasons with budget state and expose them additively through cycle/status and Telegram. Blocked SHALL render as blocked, never Running. Editing Config alone SHALL never resume; each explicit `/hero-continue` SHALL recheck prerequisites from current configuration, then apply genuine-finding repair and outstanding coverage gates. Agents SHALL report observations; only the scheduler SHALL transition state or accept findings/tasks.

#### Scenario: Config edit without continue
- **WHEN** the user fixes `.env.hero` while Browser UI is blocked but does not run `/hero-continue`
- **THEN** the stage remains blocked with its reason and retry command

### Requirement: Preparation SHALL be bounded and spend active time
Preparation SHALL default to 120 seconds total per attempt (capped by remaining stage budget) with at most two attempts per prerequisite, using bounded locators and readiness waits (no fixed sleep loops). A prerequisite block before validation SHALL NOT consume a validation iteration but SHALL consume active time. Repeated preflight retries SHALL NOT reset the stage budget. Timeout values MAY be configured with validation.

#### Scenario: Second attempt exhausts the prerequisite
- **WHEN** a fixture check fails twice within the ceiling
- **THEN** the stage blocks with the fixture blocker and no third automatic attempt starts

### Requirement: Planned execution contracts SHALL NOT be rediscovered in QA
Planning SHALL supply a project-specific, non-secret execution contract dynamically for each consuming application: environment/base URL, start/readiness commands, explicit browser method and executable/tool, version expectations, E2E command where applicable, login recipe, selected users/profiles, fixtures, action/test timeouts, and evidence paths. Hero SHALL NOT hardcode application origins, routes, locators, fixtures, or runner commands. An existing E2E suite SHALL be preferred for repeatable E2E; without one, Planning SHALL select exploratory validation with assertions or assign suite setup to Implementation. For coding-agent browser control, prefer Playwright CLI with its official skill, use the CLI's skills-less mode if skills cannot be loaded, and use MCP for persistent-session/iterative-inspection needs or when verified CLI capability is insufficient. The skill is CLI guidance, not a separate executor. The minimum Playwright package baseline SHALL be 1.63.0; Planning SHALL verify the actual selected project/harness version and capabilities. QA SHALL NOT install or upgrade browser tools/frameworks, repair dependencies, or create application code. A missing or below-minimum tool SHALL block preparation with a setup action. `qa_end_to_end.use_playwright` and frontend/browser stage guards SHALL stay; HTTP mode SHALL be explicit and SHALL never substitute for planned browser coverage.

`planning_agent` SHALL create `.workflow-hero/cycles/current/browser-plan.json` during Planning for consuming applications with browser validation. This shared non-secret artifact SHALL contain the planned execution contract, approved method, and coverage denominator. Deterministic preparation SHALL load and validate it before browser execution; Browser UI/E2E agents SHALL consume it. Credential values SHALL remain outside this artifact, in project-root `.env.hero` and private executor memory.

#### Scenario: Planning produces the browser execution artifact
- **WHEN** Planning prepares a consuming application with browser validation
- **THEN** it writes the application-specific non-secret recipe, approved method, and coverage to `.workflow-hero/cycles/current/browser-plan.json` for deterministic preparation and Browser UI/E2E consumption

#### Scenario: Missing playwright.config with usable CLI automation
- **WHEN** the project has no suite config but Planning declared a compatible exploratory method
- **THEN** actual launch/navigation is validated in the stage session and suite absence alone does not block
