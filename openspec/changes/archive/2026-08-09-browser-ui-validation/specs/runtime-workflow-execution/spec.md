## MODIFIED Requirements

### Requirement: Runtime SHALL execute the documented stage flow
The Runtime SHALL orchestrate the cycle in this order: Configuration -> Research -> Planning -> Implementation -> QA -> Judge -> Browser UI Validation -> QA End-to-End, with Configuration implicit and non-configurable (PRD §5.1). Stages that are disabled in `workflow-config.yml` SHALL be skipped while preserving relative order of enabled stages.

#### Scenario: Starting a cycle with all stages enabled
- **WHEN** a cycle starts from Runtime with all configurable stages enabled
- **THEN** stages execute in documented order after Configuration, including Browser UI Validation between Judge and QA End-to-End

#### Scenario: Browser UI Validation disabled
- **WHEN** `stages.browser_ui_validation.enabled` is false
- **THEN** Runtime skips Browser UI Validation and advances from Judge to the next enabled stage (typically QA End-to-End)

## ADDED Requirements

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
