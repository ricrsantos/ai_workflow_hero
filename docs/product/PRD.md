# PRD — AI Workflow Hero (Hero) CLI & Runtime

> Product Requirements Document. Source: `docs/idea/ai_workflow_hero.md` (design notes and grilling session decisions, 2026-07-20).

## 1. Overview

**Hero** is a framework for AI-augmented software development. It does not replace the coding agent — it coordinates multiple specialized subagents, organizes project work, compresses context, and makes AI-driven development cycles reproducible.

Hero reduces project dependency on any single LLM provider, giving developers freedom to choose models that fit their budget and needs, and reduces token costs through context compression and configurable stages.

Every new **Development Cycle** — creating a project, implementing a feature, fixing a bug, or any other activity — is abstracted by Hero: project organization, documentation, project memory, agent orchestration, prompt construction, validation, and integration with other frameworks (OpenSpec, Playwright, Context7).

Hero's artifacts are split so the user is never locked in:

- **Project artifacts** (`AGENTS.md`, `docs/`, `context/`, `openspec/`) represent permanent knowledge, useful with or without Hero.
- **Hero artifacts** (`.workflow-hero/`) hold only Hero's own configuration, metrics, and interaction history.

## 2. Goals

### 2.1 Global Goals

- Obtain the most deterministic results possible.
- Reduce dependency on specific LLM models.
- Reduce token costs.
- Let the user choose which stages run in each development cycle.
- Implement advanced orchestration (multi-agent + loops).
- Use specialized subagents for each activity type.
- Provide token consumption statistics for every stage of the project.
- Maintain history and context of changes.
- Implement continuous context compression: current architecture, recent decisions, real flow, exceptions, temporary hacks that became permanent.

### 2.2 V1 Scope

- First version focused exclusively on Cursor AI.
- Covers the stages: Research, Planning, Implementation, Validation (QA, Judge, QA End-to-End).
- `/hero:sync` covers **basic activation** of Hero in existing projects (codebase scan via `context_agent` + generation of `AGENTS.md`/`current-state.md`). Promoted from V2 to V1 on 2026-07-20, since it is a natural prerequisite for adoption in real projects.
- CLI target platforms: Linux and macOS, `amd64` and `arm64` (see [DEPLOY.md](../deployment/DEPLOY.md)).
- Manual release process via a single build script (no CI/CD required for V1).

### 2.3 V2 Scope (out of scope for V1)

- Compatibility with additional agentic development environments: Open Code, Claude Code, Claude App, Codex CLI, Codex App, VS Code.
- Optional stages: UX, Observability, Deployment specifications, Security review, Architecture review.
- **Advanced** synchronization with existing projects (drift detection between code and docs, continuous incremental sync — beyond the basic activation already covered in V1).
- Workflow for architecture improvements in existing projects.
- AI hooks for finer-grained workflow control.
- Deterministic code between stages to save tokens and standardize the process further.
- More sophisticated project memory sources (e.g. database, RAG).
- Evaluate always operating via CLI (no chat dependency).
- Windows support for the CLI binary.
- CI/CD-automated releases (GoReleaser or equivalent).

## 3. Terminology

| Term | Definition |
|---|---|
| Bootstrap | The process of initializing Hero in a project; Hero creates its execution artifacts inside the user's project. |
| Development Cycle (Cycle) | The start of a new activity in the project: creating the project, adding a feature, fixing a bug, etc. |
| Stage | Each of the steps that can run during a cycle: research, plan, implementation, test, validation, etc. |
| Research | The stage where alternatives for the desired functionality are explored. |
| Plan | The stage where product/architecture specs are converted into an implementation plan (SDD) with ordered, testable tasks. |
| Implementation | The stage where code is actually implemented, per the plan or explicit developer guidance. |
| Validation | The final stages of a cycle (QA, Judge, QA End-to-End): tests and developer interaction to reach the desired quality bar. |

## 4. Personas

- **Solo developer / small team using Cursor**, who wants a repeatable, documented AI development process without hand-writing prompts and boilerplate docs for every feature or bugfix.
- **Maintainer of the Hero project itself** (Go CLI), who needs a clear, testable, feature-based codebase.

## 5. Functional Requirements

### 5.1 Stage Flow

```
Configuration → Research → Planning → Implementation → QA → Judge → QA End-to-End
```

- Each stage can be enabled/disabled by the user via `.workflow-hero/cycles/current/workflow-config.yml`, except **Configuration**, which always runs implicitly and is never configurable.
- The **Configuration** stage always has `Human Approval = N/A`.
- Every configurable stage supports: `enabled`, `purpose`, `max_iterations`, `timeout_minutes`, `require_human_approval`.

### 5.2 Agents

| Agent | Responsibility |
|---|---|
| `orchestration_agent` | Orchestrates the loop; is the session model selected by the user in the IDE. |
| `discover_agent` | Runs the grilling cycle for Research (same as `orchestration_agent` in V1/Cursor). |
| `planning_agent` | Drives OpenSpec-based planning (SDD creation). |
| `context_agent` | Retrieves project/code context on demand; read-only, never implements or decides architecture. |
| `backend_agent` | Implements backend code per the approved SDD. |
| `frontend_agent` | Implements frontend code per the approved SDD. |
| `generic_agent` | Implements native apps (Linux/Windows), scripts, and infrastructure code (`scope.native`/`script`/`infrastructure`). |
| `qa_agent` | Validates technical quality: tests, coverage, build, lint, architecture consistency, and scope-specific checks. |
| `judge_agent` | Validates SDD requirement coverage only (not code quality/style). |
| `end2end_qa_agent` | Validates the full user journey end-to-end. Uses Playwright when `stages.qa_end_to_end.use_playwright: true` (requires `scope.frontend: true`); otherwise uses direct HTTP calls. |

### 5.3 Approval and Control Loop

- When `require_human_approval: false`, a stage auto-completes (`Status=Completed`, `Human Approval=Disable`), posts a short summary, and moves to the next stage in the same turn. The user can still interrupt with `/hero:reject` or `/hero:cancel` before the next stage starts.
- When `require_human_approval: true`, the agent summarizes the result and waits for one of: `/hero:approve`, `/hero:reject`, `/hero:cancel`, `/hero:finish`. This pattern is identical across all stages.
- Every stage closes with the same sequence: (a) summary + approval request, (b) update `workflow.md`, (c) update `metrics.md` + show a metrics summary, (d) advance to the next configured stage.

### 5.4 Iteration and Timeout Handling

- Each stage has a `max_iterations` and `timeout_minutes` budget. Timeout is checked **between** iterations (not mid-execution); if exceeded, treated the same as exhausting `max_iterations`.
- On exhaustion, the orchestrator escalates to the user (`Human Approval = Escalated`) and waits for `/hero:continue` (user specifies how many extra iterations to grant; recorded in `workflow.md` as `Extra Iterations Granted`, without altering `workflow-config.yml`).
- **QA / QA End-to-End failure loop**: returns to the implementation agent(s) referenced in the error report; each retry consumes one iteration.
- **Judge failure loop**: implementation gaps follow the same pattern as QA. If, after exhausting implementation gaps, the Judge identifies **ambiguity in the SDD itself**, it stops and asks the user to choose between `/hero:back` (reopen Planning) or `/hero:approve` (accept as-is, noted in `context-log.md`).
- **`/hero:back`**: reopens Planning (the `planning_agent` edits the existing OpenSpec proposal in place). Implementation, QA, and Judge reset to `Waiting` and re-run from scratch.
- **`/hero:archive` (manual)**: archives the current cycle even mid-progress, marking the in-progress stage as `Paused`. `/hero:resume [cycle]` restores it back to `cycles/current/`.

### 5.5 Model Selection and Fallback

- Each agent's model is configured per-cycle in `workflow-config.yml`.
- Fallback chain (2 levels + escalation): 1) the agent's configured model → 2) `fallback_model` (top-level block in `workflow-config.yml`, with `model`, `reasoning_effort`, `enable_fast_model`, and `thinking`), **with an explicit warning to the user every time it activates** → 3) if still unavailable, warn the user and wait for `/hero:continue` after they fix the configuration.

### 5.6 Scope

`workflow-config.yml → scope` has 5 boolean fields: `backend`, `frontend`, `native`, `script`, `infrastructure`. At least one must be `true` when `implementation.enabled: true`. `backend`/`frontend` map to `backend_agent`/`frontend_agent`; the other three map to `generic_agent`.

`workflow-config.yml → stages.qa_end_to_end.use_playwright` selects the e2e method: `true` requires `scope.frontend: true` and tells `end2end_qa_agent` to use Playwright for browser journeys; `false` uses direct HTTP calls only.

### 5.7 Documents and Numbering

- The `discover_agent` decides which documents to create (PRD, ADR, UI, DEPLOY, TESTING) and registers them automatically in `documents.json`.
- PRD/ADR/UI documents are numbered per cycle with a cycle prefix: `[CATEGORY]-C[XX]-[seq]-[slug].md` (e.g. `PRD-C04-001-checkout-flow.md`). `DEPLOY.md`/`TESTING.md` are living documents, unnumbered, edited in place.
- `docs/architecture/ADR.md` and `docs/product/PRD.md` act as indexes of all ADRs/PRDs across cycles.
- All project-cycle artifacts (PRD, ADR, AGENTS.md, context files) are always written in **English**, regardless of the chat language, except Hero's own static tool documentation (`README.md`/`README_PT_BR.md`), which is bilingual and maintained separately.

### 5.8 CLI Commands (deterministic, no AI reasoning)

`hero install --tools cursor`, `hero upgrade`, `hero uninstall`, `hero doctor`, `hero version`, `hero variables` (read-only), `hero update-models`, `hero status`, `hero help`.

### 5.9 Runtime Commands (require agent reasoning)

`/hero:init`, `/hero:start`, `/hero:approve`, `/hero:reject`, `/hero:cancel`, `/hero:finish`, `/hero:archive`, `/hero:resume [cycle]`, `/hero:sync`, `/hero:status`, `/hero:help`, `/hero:continue`, `/hero:back`.

Each Runtime command maps to exactly one embedded asset file (`hero-<command>.md` in `.cursor/commands/`), and each agent above maps to exactly one `<agent_name>.md` file in `.cursor/agents/` — see [ADR-011](../architecture/ADR.md#adr-011-one-asset-file-per-runtime-command-and-agent) for the full file lists.

### 5.10 Metrics

- `metrics.md` (per cycle): one row per stage, with sub-rows when multiple agents act in the same stage (e.g. backend + frontend in Implementation), plus a subtotal and a grand total.
- `metrics-summary.md` (project-wide, outside `cycles/`): aggregates totals across all cycles.
- Token/cost estimation uses a simple heuristic (character count ÷ ~4, multiplied by the model's price from `models/*.yml`).

## 6. Non-Functional Requirements

- **Determinism**: the CLI never performs LLM reasoning; only the Runtime does. This boundary must never be crossed (see [ADR-003](../architecture/ADR.md#adr-003-cli-vs-runtime-separation)).
- **Concurrency safety**: a lock file (`cycles/current/.lock`) prevents two chat sessions from corrupting the same cycle state.
- **Git dependency**: the project must be a git repository (required for `/hero:cancel` checkpoint/rollback). `hero install` offers to run `git init` if missing.
- **Backward-safe upgrades**: `hero upgrade` never silently overwrites user-customized templates; it detects modifications via checksum and warns instead.
- **Clean subagent sessions**: every subagent invocation (backend/frontend/generic/qa/judge/end2end_qa/context) runs in a fresh, isolated session via the Task tool, receiving only file pointers (not full pasted content) to conserve context window budget. Only the final structured output is absorbed back by the orchestrator.
- **Testing (this repository)**: `go test ./...` must pass before any task is considered complete. On failure: analyze, fix, re-run, and only stop once green — never leave the repository in a failing state. See [ADR-009](../architecture/ADR.md#adr-009-test-real-dependencies-over-mocks) and the Testing section of [AGENTS.md](../../AGENTS.md).
- **Testing (Hero-managed end-user projects)**: every cycle's `TESTING.md` template documents the target project's own test command and pass/fail policy; the `qa_agent` and `end2end_qa_agent` enforce it during the QA and QA End-to-End stages (§5.2).

## 7. Out of Scope for V1

- Any environment other than Cursor.
- Windows binaries and CI/CD-automated releases.
- GPG-signed release artifacts.
- Non-interactive-only CLI (V1 keeps interactive prompts, with flag overrides for scripting).
- Advanced sync / drift detection with existing projects.

## 8. Success Metrics

- A development cycle (Research → QA End-to-End) can run end-to-end inside Cursor without the user manually writing PRDs, ADRs, or prompts by hand.
- Token/cost estimates are visible after every stage.
- Switching the LLM model for any agent requires only editing `workflow-config.yml` — no prompt rewrites.

## 9. References

- Full design discussion and decisions log: [docs/idea/ai_workflow_hero.md](../idea/ai_workflow_hero.md)
- Architecture: [docs/architecture/ADR.md](../architecture/ADR.md)
- Terminal UX: [docs/product/UI.md](UI.md)
- Deployment: [docs/deployment/DEPLOY.md](../deployment/DEPLOY.md)
