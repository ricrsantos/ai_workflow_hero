## Purpose

TBD - Runtime stage flow, approval/control loops, iteration/timeout escalation, backtracking, scope routing, model fallback, metrics, and isolated subagent sessions.

## Requirements

### Requirement: Runtime SHALL execute the documented stage flow
The Runtime SHALL orchestrate the cycle in this order: Configuration -> Research -> Planning -> Implementation -> QA -> Judge -> Browser UI Validation -> QA End-to-End, with Configuration implicit and non-configurable (PRD §5.1). Stages that are disabled in `workflow-config.yml` SHALL be skipped while preserving relative order of enabled stages.

#### Scenario: Starting a cycle with all stages enabled
- **WHEN** a cycle starts from Runtime with all configurable stages enabled
- **THEN** stages execute in documented order after Configuration, including Browser UI Validation between Judge and QA End-to-End

#### Scenario: Browser UI Validation disabled
- **WHEN** `stages.browser_ui_validation.enabled` is false
- **THEN** Runtime skips Browser UI Validation and advances from Judge to the next enabled stage (typically QA End-to-End)

### Requirement: Runtime SHALL apply uniform approval/control-loop semantics
Every stage closure SHALL summarize output, request approval when required, update workflow and metrics state, and advance according to control commands (`/hero:approve`, `/hero:reject`, `/hero:cancel`, `/hero:finish`) (PRD §5.3, §5.9).

#### Scenario: Stage requiring human approval
- **WHEN** a stage has `require_human_approval: true`
- **THEN** Runtime waits for an approval command and does not auto-advance until command resolution

#### Scenario: Stage without required human approval
- **WHEN** a stage has `require_human_approval: false`
- **THEN** Runtime auto-completes the stage, records state, posts summary, and proceeds to the next configured stage

### Requirement: Runtime SHALL enforce iteration and timeout escalation behavior
Each stage SHALL honor `max_iterations` and `timeout_minutes`, escalate with `Human Approval = Escalated` when exhausted, and support `/hero:continue` extra-iteration grants tracked in workflow state (PRD §5.4).

#### Scenario: Iteration budget exhausted
- **WHEN** a stage reaches its iteration or timeout limit
- **THEN** Runtime escalates and waits for `/hero:continue` before additional iterations

### Requirement: Runtime SHALL implement backtracking and retry loops as specified
QA/Judge failures SHALL route back to implementation agents for fixes; `/hero:back` SHALL reopen Planning and reset downstream stage statuses for re-execution (PRD §5.4).

#### Scenario: Judge reports SDD ambiguity after retries
- **WHEN** Judge identifies unresolved SDD ambiguity after implementation-gap retries
- **THEN** Runtime requests explicit user decision between reopening Planning and accepting as-is

### Requirement: Runtime SHALL enforce scope-to-agent routing
When implementation is enabled, at least one scope flag SHALL be true; `backend` and `frontend` SHALL map to `backend_agent` and `frontend_agent`, and `native`, `script`, and `infrastructure` SHALL map to `generic_agent` (PRD §5.6).

#### Scenario: Invalid scope configuration for implementation
- **WHEN** implementation is enabled and all scope flags are false
- **THEN** Runtime blocks execution until scope configuration is corrected

### Requirement: Runtime SHALL honor Playwright selection for QA End-to-End
`stages.qa_end_to_end.use_playwright` SHALL control whether `end2end_qa_agent` uses Playwright. `use_playwright: true` SHALL require `scope.frontend: true`; otherwise Runtime SHALL block until corrected. When `use_playwright` is false, the agent SHALL use direct HTTP calls.

#### Scenario: Playwright selected with frontend in scope
- **WHEN** `use_playwright` is true and `scope.frontend` is true
- **THEN** `end2end_qa_agent` runs browser journeys with Playwright

#### Scenario: Playwright selected without frontend
- **WHEN** `use_playwright` is true and `scope.frontend` is false
- **THEN** Runtime blocks execution until configuration is corrected

#### Scenario: Playwright disabled
- **WHEN** `use_playwright` is false
- **THEN** `end2end_qa_agent` uses direct HTTP calls for e2e validation

### Requirement: Runtime SHALL apply model fallback chain with explicit user warnings
Agent model resolution SHALL follow configured model -> `fallback_model` (with explicit warning) -> wait for user correction and `/hero:continue` if still unavailable (PRD §5.5; ADR-008).

#### Scenario: Primary model unavailable, fallback model available
- **WHEN** an agent's configured model is unavailable but `fallback_model` is available
- **THEN** Runtime executes using `fallback_model` and emits explicit fallback warning

### Requirement: Runtime SHALL maintain cycle and project metrics artifacts
Runtime SHALL update per-cycle `metrics.md` and project-level `metrics-summary.md` with stage-level and aggregate values, including token and cost estimates based on model pricing references (PRD §5.10).

#### Scenario: Closing a stage with metrics update
- **WHEN** a stage closes successfully or with explicit terminal state
- **THEN** Runtime updates cycle metrics and aggregated summary before proceeding

### Requirement: Runtime subagent invocations SHALL run in isolated sessions
Every subagent invocation for implementation, validation, and context SHALL run in a fresh Task session with file pointers and structured outputs, not inherited chat history (PRD §6; ADR-005).

#### Scenario: Dispatching backend agent from implementation stage
- **WHEN** Runtime dispatches `backend_agent`
- **THEN** the task starts in isolated session context with references to required files and returns structured completion output

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

### Requirement: Runtime SHALL enforce Browser UI Validation configuration gates
`stages.browser_ui_validation.enabled: true` SHALL require `scope.frontend: true`. Runtime SHALL block at `/hero:start` (and before dispatch) until corrected. When the stage is enabled, Browser Health SHALL always run. Visual Validation SHALL run only when `stages.browser_ui_validation.visual_validation.enabled` is true and only after Browser Health passes.

#### Scenario: Stage enabled without frontend scope
- **WHEN** `browser_ui_validation.enabled` is true and `scope.frontend` is false
- **THEN** Runtime blocks execution and asks the user to correct `workflow-config.yml`

#### Scenario: Visual Validation after Health pass
- **WHEN** Browser Health passes and `visual_validation.enabled` is true
- **THEN** Runtime proceeds to Visual Validation within the same stage

#### Scenario: Health failure skips Visual Validation
- **WHEN** Browser Health fails
- **THEN** Runtime MUST NOT run Visual Validation and MUST enter the stage failure loop

### Requirement: Runtime SHALL dispatch browser_ui_agent for Browser UI Validation
When Browser UI Validation is enabled, Runtime SHALL invoke `browser_ui_agent` via an isolated Task session with Model Resolution from `agents.browser_ui_agent`, using Playwright for browser instrumentation. The agent SHALL discover how to open the application from project artifacts (not from `base_url`/`start_command` config fields). Playwright unavailability at execution time SHALL be treated as a Browser Health failure.

#### Scenario: Dispatching browser_ui_agent
- **WHEN** Runtime reaches an enabled Browser UI Validation stage
- **THEN** it starts `browser_ui_agent` in an isolated Task session with file pointers and required structured output

#### Scenario: Playwright missing at execution
- **WHEN** `browser_ui_agent` cannot use Playwright in the project
- **THEN** Browser Health fails with an actionable report and Runtime routes the failure loop to `frontend_agent`

### Requirement: Runtime SHALL define Browser Health checks
Browser Health SHALL verify: application opens; page renders; browser console errors; failed network requests for CSS, JS, images, fonts, and APIs; and that CSS assets loaded successfully. Health SHALL use a desktop viewport (1280px width).

#### Scenario: CSS network failure detected
- **WHEN** a stylesheet request fails during Browser Health
- **THEN** the stage fails Health and reports the failure for remediation routing

### Requirement: Runtime SHALL define optional Visual Validation behavior
When Visual Validation is enabled after Health passes, `browser_ui_agent` SHALL discover screen candidates from the cycle (PRD/UI docs/routes), capture screenshots via Playwright at viewports 1280, 768, and 375, and compare against PNG references under `visual_validation.reference_dir` (default `docs/ui/visual_reference`) using agent vision judgment. A missing PNG for a candidate SHALL produce a warning and continue (not a failure). An empty or missing reference directory SHALL produce a single warning and skip the Visual block without failing the stage. User reference PNGs SHALL NEVER be overwritten.

#### Scenario: Reference PNG present
- **WHEN** a screen candidate has a matching `<screen-id>.png` under the reference directory
- **THEN** the agent captures screenshots and performs visual comparison against that reference

#### Scenario: Reference PNG absent
- **WHEN** a screen candidate has no matching PNG
- **THEN** the agent warns and continues without treating the absence as a stage failure

#### Scenario: Empty reference directory
- **WHEN** Visual Validation is enabled but the reference directory is missing or contains no PNGs
- **THEN** the agent emits one warning and skips Visual Validation without failing the stage

### Requirement: Runtime SHALL route Browser UI Validation failures to implementation agents
Browser Health and Visual Validation failures SHALL consume one stage iteration per retry loop. Static asset, console, render, and Visual failures SHALL route to `frontend_agent`. Failures clearly classified as backend API errors SHALL route to `backend_agent`. Missing reference PNGs SHALL NOT trigger a failure loop.

#### Scenario: Frontend asset Health failure
- **WHEN** Browser Health fails due to CSS/JS/image/font or console/render issues
- **THEN** Runtime reinvokes `frontend_agent` and consumes one Browser UI Validation iteration

#### Scenario: Backend API Health failure
- **WHEN** Browser Health fails due to a clearly classified backend API request failure
- **THEN** Runtime reinvokes `backend_agent` and consumes one Browser UI Validation iteration

### Requirement: Runtime SHALL write Browser UI Validation cycle artifacts
Runtime SHALL write stage artifacts under `.workflow-hero/cycles/current/browser-ui/`, including a Health report, captured screenshots under `screenshots/`, and a Visual report when Visual Validation ran. These artifacts SHALL be archived with the cycle.

#### Scenario: Successful Health run
- **WHEN** Browser Health completes
- **THEN** a health report and any captured diagnostic screenshots exist under `.workflow-hero/cycles/current/browser-ui/`

### Requirement: Runtime SHALL keep QA End-to-End Playwright journeys distinct from Browser UI Validation
`stages.qa_end_to_end.use_playwright` SHALL continue to control whether `end2end_qa_agent` runs business journeys in the browser versus HTTP-only mode. Enabling Browser UI Validation SHALL NOT disable or redefine that setting.

#### Scenario: Both stages enabled with Playwright journeys
- **WHEN** Browser UI Validation is enabled and `qa_end_to_end.use_playwright` is true with `scope.frontend` true
- **THEN** Browser UI Validation performs Health/Visual checks and QA End-to-End still runs Playwright journeys for business flows

### Requirement: Workflow execution SHALL use agent YAML properties, not freechat selections

During a workflow/runtime execution, Hero SHALL derive normalized `fs` from `enable_fast_model`, `th` from `thinking`, and `ef` from `reasoning_effort` in the active agent/fallback resolution. It SHALL send those values to the selected adapter and project them in Chat as unvalidated/gray when capability validation is unavailable. `/hero-model` selections SHALL remain limited to ordinary Chat and `/hero-new`, and stage YAML SHALL not be modified (PRD-C05-001 §4.1.4/5; §4.5.6; §4.6.7; §5.1; ADR-040/042).

#### Scenario: Workflow agent overrides freechat properties
- **WHEN** ordinary Chat has `ef=high` saved but the active `qa_agent` YAML has `reasoning_effort: low`
- **THEN** the QA execution request and Chat property line use `ef=low`, shown as unvalidated/gray if not capability-validated, and do not change `hero.json`

#### Scenario: `/hero-new` uses freechat properties
- **WHEN** the user runs `/hero-new` from an empty Chat after saving a freechat pair with `fs=true`
- **THEN** the request uses the saved freechat `fs=true` value rather than an arbitrary stage-agent property

#### Scenario: Cursor IDE Runtime remains isolated
- **WHEN** the same project runs a Cursor IDE Runtime command outside the Hero TUI
- **THEN** the Runtime continues to resolve model properties from its workflow configuration and does not read or write `hero.json.model_properties`

### Requirement: C4 harness routing and lazy behavior SHALL remain unchanged

C5 property projection SHALL preserve C4 harness/model pair selection, two-level harness fallback, harness-scoped sessions, and lazy OpenCode serve lifecycle. A property rejection SHALL be reported as an execution error rather than triggering a silent C4 fallback (PRD-C05-001 §2/§5; ADR-041; C4 compatibility boundary).

#### Scenario: Workflow pair still follows C4 resolution
- **WHEN** the configured workflow pair is unavailable but the configured `fallback_model` pair is available
- **THEN** Hero uses the existing explicit fallback warning and pair routing while carrying only the resolved workflow property map

#### Scenario: OpenCode remains lazy at boot
- **WHEN** Hero TUI starts with OpenCode enabled and model properties persisted
- **THEN** it does not start OpenCode solely to preload metadata; the managed serve process is eligible only after explicit `/hero-model` refresh or execution
