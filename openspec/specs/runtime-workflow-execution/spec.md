## Purpose

TBD - Runtime stage flow, approval/control loops, iteration/timeout escalation, backtracking, scope routing, model fallback, metrics, and isolated subagent sessions.

## Requirements

### Requirement: Runtime SHALL execute the documented stage flow
The Runtime SHALL orchestrate the cycle in this order: Configuration -> Research -> Planning -> Implementation -> QA -> Judge -> QA End-to-End, with Configuration implicit and non-configurable (PRD §5.1).

#### Scenario: Starting a cycle with all stages enabled
- **WHEN** a cycle starts from Runtime with all configurable stages enabled
- **THEN** stages execute in documented order after Configuration

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
