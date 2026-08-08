# Architecture Decision Records — AI Workflow Hero

> Index of all ADRs for the Hero project itself (the Go CLI + Runtime assets), consolidated from early design notes. Each ADR follows the standard Context / Decision / Consequences format. **Hero 1.0 ADRs (012–019):** [ADR-C01-001-hero-1-0.md](ADR-C01-001-hero-1-0.md) (includes ADR-003 amendment). **C2 ADRs (020–023):** [ADR-C02-001-slash-parity-harness-archive.md](ADR-C02-001-slash-parity-harness-archive.md).

| # | Title | Status |
|---|---|---|
| [ADR-001](#adr-001-go-cobra-and-embedfs-for-cli-distribution) | Go, Cobra, and `embed.FS` for CLI distribution | Accepted |
| [ADR-002](#adr-002-repository-architecture-feature-based--vertical-slice) | Repository architecture: Feature Based + Vertical Slice | Accepted |
| [ADR-003](#adr-003-cli-vs-runtime-separation) | CLI vs. Runtime separation | Accepted (amended C1 — see ADR-C01-001) |
| [ADR-004](#adr-004-git-as-a-mandatory-prerequisite) | Git as a mandatory prerequisite | Accepted |
| [ADR-005](#adr-005-subagent-invocation-via-task-tool-with-clean-sessions) | Subagent invocation via Task tool, with clean sessions | Accepted |
| [ADR-006](#adr-006-simple-placeholder-templating-no-loop-engine) | Simple placeholder templating, no loop engine | Accepted |
| [ADR-007](#adr-007-openspec-as-the-sdd-framework) | OpenSpec as the SDD framework | Accepted (amended C2 — see ADR-C02-001 / ADR-023) |
| [ADR-008](#adr-008-three-level-model-fallback-chain) | Three-level model fallback chain | Accepted |
| [ADR-009](#adr-009-test-real-dependencies-over-mocks) | Test real dependencies over mocks | Accepted |
| [ADR-010](#adr-010-manual-release-process-via-a-single-script) | Manual release process via a single script | Accepted |
| [ADR-011](#adr-011-one-asset-file-per-runtime-command-and-agent) | One asset file per Runtime command and per agent | Accepted |
| [ADR-012](ADR-C01-001-hero-1-0.md#adr-012-go-owns-deterministic-ai-loop-state-machine) | Go owns deterministic AI Loop state machine | Accepted |
| [ADR-013](ADR-C01-001-hero-1-0.md#adr-013-sqlite-as-sole-hero-operational-store) | SQLite as sole Hero operational store | Accepted |
| [ADR-014](ADR-C01-001-hero-1-0.md#adr-014-cli-as-api-no-daemon-in-10) | CLI as API; no daemon in 1.0 | Accepted |
| [ADR-015](ADR-C01-001-hero-1-0.md#adr-015-dual-entry-ui-chat-and-tui-parity) | Dual entry UI: chat and TUI parity | Accepted |
| [ADR-016](ADR-C01-001-hero-1-0.md#adr-016-harness-adapter-interface-cursor-only-impl) | HarnessAdapter interface; Cursor-only impl | Accepted |
| [ADR-017](ADR-C01-001-hero-1-0.md#adr-017-bubble-tea-tui-claude-code-inspired) | Bubble Tea TUI; Claude Code inspired | Accepted |
| [ADR-018](ADR-C01-001-hero-1-0.md#adr-018-breaking-major-upgrade-from-09x) | Breaking major upgrade from 0.9.x | Accepted |
| [ADR-019](ADR-C01-001-hero-1-0.md#adr-019-archive-idea-v1-cycle-docs-canonical) | Archive idea v1; cycle docs canonical | Accepted |
| [ADR-020](ADR-C02-001-slash-parity-harness-archive.md#adr-020-user-facing-vocabulary-is-hero-slash-commands) | User-facing vocabulary is Hero slash commands | Accepted |
| [ADR-021](ADR-C02-001-slash-parity-harness-archive.md#adr-021-non-hero-commands-execute-via-markdown-prompt-expansion) | Non-Hero commands execute via markdown prompt expansion | Accepted |
| [ADR-022](ADR-C02-001-slash-parity-harness-archive.md#adr-022-harness-detection-filesystem-markers-vs-herojson) | Harness detection: filesystem markers vs hero.json | Accepted |
| [ADR-023](ADR-C02-001-slash-parity-harness-archive.md#adr-023-hero-archive-couples-openspec-archive-with-force-escape) | Hero archive couples OpenSpec archive with force escape | Accepted |

> **Numbering convention**: this index uses `ADR-NNN-title` anchors within a single file. If the number of ADRs grows large enough to hurt readability, split into one file per ADR under `docs/architecture/`, named `ADR-NNN-title.md` (e.g. `ADR-001-stack.md`), and keep this file as the index only. Not required while the set stays this size.

---

## ADR-001: Go, Cobra, and `embed.FS` for CLI distribution

**Context**: Hero must be distributed as a single, easy-to-install binary that also carries all the assets (commands, skills, prompts, templates) needed to bootstrap a project. The target audience is developers, who value fast, dependency-free installation.

**Decision**: Build the Hero CLI in Go, using the [Cobra](https://github.com/spf13/cobra) library to structure commands, and embed all assets into the binary using Go's `embed.FS`.

**Consequences**:
- No external asset files to manage or lose; the binary is self-contained.
- `assets.version` (in `hero.json`) is always equal to `cli.version`, since assets travel inside the same binary (see also ADR-010).
- Cross-compilation for multiple platforms/architectures is native to Go's toolchain.

---

## ADR-002: Repository architecture: Feature Based + Vertical Slice

**Context**: The CLI has multiple independent administrative capabilities (install, upgrade, uninstall, doctor, sync-related helpers) that should be easy to add, test, and reason about in isolation.

**Decision**: Organize the repository using a Feature Based structure, with each capability living in its own `internal/<feature>/` package (e.g. `internal/install/`, `internal/upgrade/`, `internal/doctor/`), each containing its own `command.go` (Cobra wiring), `service.go` (business logic), and `validator.go` (input/state validation) as needed. IDE-specific logic (e.g. Cursor-specific paths and installers) lives under `internal/adapters/cursor/`.

**Consequences**:
- Each feature can be tested independently, colocated with its own `_test.go` files (see ADR-009).
- Adding support for a new IDE/harness in V2 means adding a new adapter, not touching existing feature logic.
- Shared low-level concerns (filesystem, embedded assets, versioning, logging) live in `internal/common/`.

---

## ADR-003: CLI vs. Runtime separation

**Context**: Hero has two very different kinds of behavior: deterministic administrative operations (install, upgrade, version checks) and AI-reasoning-driven workflow orchestration (research, planning, implementation, QA). Mixing them would make the CLI non-deterministic and hard to test, and would make the Runtime dependent on a compiled binary being present and up to date.

**Decision**: Strictly separate:
- **CLI** (Go binary): only installation, update, maintenance, and administration of Hero. Never performs LLM reasoning.
- **Runtime** (IDE chat, slash commands): all reasoning-driven development cycle execution.

Only purely administrative commands may have equivalents in both (e.g. `hero status` / `/hero:status`, `hero help` / `/hero:help`). Any command requiring agent reasoning exists **exclusively** in the Runtime — for example, `hero sync` does not exist; only `/hero:sync` does, because synchronizing `AGENTS.md`/context files from a codebase requires reasoning.

**Consequences**:
- The CLI is fully unit-testable with deterministic assertions (no LLM calls to mock).
- `hero update-models` fetches a pre-structured data file from the Hero GitHub repository rather than scraping/parsing pricing pages with an LLM — keeping it deterministic.
- Every new command must be classified as CLI or Runtime before implementation; this classification governs which layer of the repository it belongs to.

---

## ADR-004: Git as a mandatory prerequisite

**Context**: `/hero:cancel` needs a reliable way to discard changes made during a stage. Ad-hoc file tracking by the agent is error-prone and does not scale to arbitrary file operations (renames, deletions, binary files).

**Decision**: Require the target project to be a git repository. `hero install` and `hero doctor` check for this. If `hero install` detects the project is not a git repository, it interactively offers to run `git init` on the user's behalf; if the user declines, installation is aborted. At the start of every stage, the orchestrator ensures a clean commit/checkpoint exists; `/hero:cancel` runs `git checkout`/`git restore` against that checkpoint.

**Consequences**:
- `/hero:cancel` is reliable and covers all file types and operations.
- Hero cannot be used in a non-git project without the user's explicit consent to initialize one.
- The checkpoint strategy must never mix with the user's own unrelated uncommitted work; this is a UX risk to keep documented and validated in the `research`/`implementation` prompts.

---

## ADR-005: Subagent invocation via Task tool, with clean sessions

**Context**: One of Hero's global goals is reducing token costs. Passing full conversational history and pasted file contents to every subagent invocation (backend_agent, qa_agent, etc.) wastes context window and money, and can leak irrelevant reasoning between agents with very different responsibilities.

**Decision**: Invoke every subagent (`backend_agent`, `frontend_agent`, `generic_agent`, `qa_agent`, `judge_agent`, `browser_ui_agent`, `end2end_qa_agent`, `context_agent`) via the IDE's Task tool, in a **fresh, isolated session** that does not inherit the orchestrator's chat history. Subagents receive **file pointers** (paths to `AGENTS.md`, `current-state.md`, the relevant SDD/tasks) instead of pasted content. The orchestrator absorbs back only the subagent's final structured output (its documented "Output Format" section), not its intermediate reasoning. The `orchestration_agent`'s own main session remains continuous across the whole cycle — only subagents are session-scoped per call.

**Model selection**: Agent `.md` files use Cursor YAML frontmatter with `model: inherit` (plus `name` / `description`). The **effective** model is not taken from frontmatter; it is passed as the Task tool `model` parameter from `workflow-config.yml`, applying `enable_fast_model` / `reasoning_effort` / `thinking` as **kebab Task slugs** (e.g. `cursor-grok-4.5-high`, `composer-2.5-fast`, `claude-sonnet-5-medium`) — **not** bracket options like `id[fast=false,effort=high]` (Cursor Task rejects brackets in current IDE versions). **Orchestrator → named agent** resolves `agents.<name>` (top-level model fields). **Nested generic Task fan-out** (when an orchestrator-dispatched agent launches a nested Task that is not a named Hero agent) resolves `agents.<name>.subagent`: if the block is missing or `same_of_agent: true`, reuse the parent agent's resolved model; if `same_of_agent: false`, resolve from `subagent.model` / `enable_fast_model` / `reasoning_effort` / `thinking`. **Named Hero agent** dispatches (e.g. `backend_agent` → `context_agent`) always use the target's top-level block, never the caller's `subagent`. On fallback, read `fallback_model.*` with the same kebab rules. Omitting Task `model` incorrectly inherits the orchestrator session model. Fallback remains ADR-008. Cursor may still override unavailable or plan-restricted models; Hero cannot bypass IDE limits. When looking up pricing in `models/*.yml`, match the resolved slug or strip known suffixes (`-thinking`, `-fast`, `-high`, `-medium`, `-low`) to find a base rate entry.

**Consequences**:
- Token usage per subagent call is bounded by what it actually needs to read, not by the orchestrator's accumulated history.
- Each `*_agent.md` prompt must be self-sufficient: it must instruct the subagent to read the pointed-to files itself, since it starts with no prior context.
- Debugging a subagent's reasoning requires inspecting its own session/logs, not the orchestrator's transcript.
- Switching an agent's model (or its nested fan-out `subagent` model) requires only editing `workflow-config.yml`; agent asset frontmatter stays `inherit`.

---

## ADR-006: Simple placeholder templating, no loop engine

**Context**: Hero's markdown templates (AGENTS.md, current-state.md, etc.) need variable substitution from JSON config files (`hero.json`, `project.json`, `documents.json`). A full templating engine (e.g. real Mustache with loops/conditionals) adds a dependency and parsing complexity for a use case that is, in practice, simple key lookups.

**Decision**: Templates use only simple placeholder substitution in the form `{{path.key}}` (e.g. `{{project.name}}`, `{{documents.prd.path}}`). There is no support for real `{{#loop}}...{{/loop}}` constructs. Any list-like content (e.g. the documentation map table in `AGENTS.md`) is composed directly by the AI agent as final text, not generated by a template loop.

**Consequences**:
- No templating library dependency is required in the CLI; a straightforward string-replace pass is enough.
- Golden-file tests (ADR-009) can assert exact placeholder substitution without needing to model loop semantics.
- Agents authoring or updating templates must remember that anything resembling a loop is their own responsibility to render as plain text, not the template engine's.

---

## ADR-007: OpenSpec as the SDD framework

**Context**: Hero needs a structured, reviewable way to turn approved product/architecture docs into an implementation plan (SDD) with ordered, testable tasks, without reinventing spec-driven development tooling.

**Decision**: Use the [OpenSpec](https://github.com/Fission-AI/OpenSpec) framework for the Planning stage and part of Implementation/Judge closing: `/opsx-explore` (map existing codebase before planning, when applicable), `/opsx-propose` (create the SDD proposal), `/opsx-apply` (drive task-by-task implementation, dispatching to `backend_agent`/`frontend_agent`/`generic_agent`), `/opsx-sync` (sync `openspec/specs/` with what was actually implemented, after Judge approval), and `/opsx:archive` (archive completed proposals). `openspec/config.yaml`'s `context:` field is generated dynamically from `documents.json`, not hardcoded.

**Consequences**:
- Hero depends on OpenSpec being installed (`openspec init --tools cursor` on first use, requested with user consent).
- `/hero:back` (reopening Planning due to SDD ambiguity) edits the existing OpenSpec proposal in place, rather than archiving and recreating it, preserving change history.

---

## ADR-008: Three-level model fallback chain

**Context**: A key Hero goal is reducing dependency on any single LLM model. If an agent's configured model becomes unavailable (deprecated, rate-limited, not present on the user's plan), the workflow should not hard-fail silently or produce a confusing error.

**Decision**: Implement a 3-level fallback: 1) the agent's model as configured in `workflow-config.yml`; 2) `fallback_model`, a top-level block in the same file (`model`, `reasoning_effort`, `enable_fast_model`, `thinking`) — **the user is always explicitly warned whenever this fallback activates**; 3) if still unavailable, the orchestrator warns the user and waits for `/hero:continue` after they fix the configuration. At invocation time, the chosen id is passed as the Task tool `model` parameter (ADR-005 Model Resolution), not left to frontmatter inherit.

**Consequences**:
- `fallback_model` is scoped per-cycle (top-level in `workflow-config.yml`), not globally in `hero.json`, since budget/availability may vary by cycle.
- Users retain full visibility into which model actually ran, even when the fallback silently kept the cycle moving.

---

## ADR-009: Test real dependencies over mocks

**Context**: Following general engineering guidance for this codebase — prefer clarity over cleverness, test behavior not implementation details, favor real dependencies over excessive mocking, keep tests deterministic and fast, and avoid over-engineered test frameworks — the CLI's own test suite needs a concrete strategy that matches those principles.

**Decision**: 
- Test files are colocated with the code they test, in the same package (e.g. `internal/install/service_test.go` next to `service.go`).
- Use `t.TempDir()` and the real OS filesystem for tests, instead of mocking the filesystem.
- Use the real `embed.FS` for asset-related tests, instead of mocking embedded assets.
- Combine three layers: unit tests per `internal/<feature>/` package, golden-file tests for template rendering (ADR-006), and lightweight integration tests that run the compiled binary end-to-end against a temp directory for `install`/`upgrade`/`uninstall`/`doctor`.

**Consequences**:
- Tests are slightly slower than pure in-memory mocked tests, but remain fast in absolute terms (local filesystem I/O) and catch real integration bugs mocks would hide.
- No mocking framework dependency is needed in `go.mod`.

---

## ADR-010: Manual release process via a single script

**Context**: V1 does not require full CI/CD automation; the priority is getting a working, cross-compiled release out with minimal infrastructure investment.

**Decision**: Releases are built and published manually by the maintainer, but a single shell script (`scripts/release.sh`) automates the repetitive part: it cross-compiles the 4 target combinations (`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`), reads the current git tag as the version, injects it via `-ldflags "-X main.version=..."`, and generates a `checksums.txt` (SHA256) for all 4 binaries — all from a single command invocation. See [DEPLOY.md](../deployment/DEPLOY.md) for full details.

**Consequences**:
- No GitHub Actions workflow is required for V1 releases; this is deferred to V2 (GoReleaser or plain Actions) if/when release frequency grows.
- The maintainer is responsible for manually uploading the script's output artifacts to GitHub Releases.
- Because `assets.version == cli.version` (ADR-001), there is only one version number to manage per release.

---

## ADR-011: One asset file per Runtime command and per agent

**Context**: Hero's embedded assets must map cleanly to Cursor's own conventions (one command = one `.md` file in `.cursor/commands/`, one agent = one `.md` file in `.cursor/agents/`) so `hero install`/`hero upgrade` can copy, checksum, and diff them individually (see ADR-001, and the upgrade behavior in [DEPLOY.md §7](../deployment/DEPLOY.md#7-update--removal)).

**Decision**: Every Runtime slash command has exactly one corresponding asset file named `hero-<command>.md` in `.cursor/commands/`, and every agent has exactly one `<agent_name>.md` file in `.cursor/agents/`. The full V1 lists are:

- **Runtime command files** (13): `hero-new.md`, `hero-start.md`, `hero-approve.md`, `hero-reject.md`, `hero-cancel.md`, `hero-finish.md`, `hero-archive.md`, `hero-resume.md`, `hero-sync.md`, `hero-status.md`, `hero-help.md`, `hero-continue.md`, `hero-back.md`.
- **Agent files** (11): `orchestration_agent.md`, `discover_agent.md`, `planning_agent.md`, `context_agent.md`, `backend_agent.md`, `frontend_agent.md`, `generic_agent.md`, `qa_agent.md`, `judge_agent.md`, `browser_ui_agent.md`, `end2end_qa_agent.md`.

Every agent file is self-sufficient (per ADR-005): it documents its Role, Responsibilities, Rules, and an explicit Output Format section, since the agent starts each invocation with no prior chat context.

**Consequences**:
- `hero doctor` can verify installation integrity file-by-file against this fixed list (see [DEPLOY.md §7](../deployment/DEPLOY.md#7-update--removal)).
- Adding a new Runtime command or agent in a future version always means adding exactly one new asset file, never editing a shared "commands bundle" file.
- The full list above is the source of truth for asset file names; `docs/idea/ai_workflow_hero.md` contains the full markdown content for each file's template.

---

## Appendix: Data File Schemas

These are the concrete JSON/YAML schemas referenced throughout this document and the PRD. They are the source of truth for field names; full field-by-field narrative and worked examples live in [docs/idea/ai_workflow_hero.md § Arquivos / Templates](../idea/ai_workflow_hero.md).

### `.workflow-hero/config/hero.json`

Hero's own installation metadata (never edited by hand).

```json
{
  "cli": {
    "version": "1.0.0",
    "installedAt": "2026-07-11T12:00:00Z",
    "tools": ["cursor"]
  },
  "assets": {
    "version": "1.0.0",
    "installedAt": "2026-07-11T12:00:00Z"
  }
}
```

`assets.version` always equals `cli.version` (ADR-001).

### `.workflow-hero/config/project.json`

The project's own identity and advanced metadata.

```json
{
  "name": "project name",
  "summary": "project summary",
  "repository": "project-repository",
  "createdAt": "2026-07-11T12:00:00Z",

  "workflow": {
    "name": "Feature Development",
    "phase": "Research",
    "cycle": 4
  },

  "technology": {
    "stack": "project stack",
    "backend": "project backend",
    "languages": ["project languages"]
  },

  "platform": {
    "targets": ["project platform targets"]
  },

  "localization": {
    "languages": ["project languages"],
    "defaultLanguage": "project default language"
  },

  "ui": {
    "design": "Design style",
    "theme": { "default": "light", "supportsDarkMode": true }
  },

  "deployment": {
    "target": "project deployment target",
    "domain": "project domain"
  }
}
```

- `workflow.cycle`: sequential global cycle counter, incremented by the `orchestration_agent` on every successful `/hero:new`. Used as the prefix (`C04`) in document numbering (`PRD-C04-001-slug.md`) and in archive folder names (`C04-yyyy-mm-dd-slug/`).
- Advanced fields (`technology`, `platform`, `localization`, `ui`, `deployment`) are empty/`null` right after `hero install`; the `orchestration_agent` fills them during the first `/hero:new`, either by inferring from an existing codebase or by asking the user.

### `.workflow-hero/config/documents.json`

Registry of every document created for the current cycle, maintained automatically by the `discover_agent` (see ADR on document numbering in the PRD).

```json
{
  "documents": [
    {
      "name": "TESTING Strategy",
      "path": "docs/testing/TESTING.md",
      "purpose": "Testing strategy and commands."
    },
    {
      "name": "PRD-C04-001",
      "path": "docs/product/PRD-C04-001-checkout-flow.md",
      "purpose": "Checkout flow requirements"
    },
    {
      "name": "UI / Design Spec",
      "path": "docs/product/UI.md",
      "purpose": "Visual direction, sections, components, tokens"
    },
    {
      "name": "ADR-C04-001",
      "path": "docs/architecture/ADR-C04-001-database-choice.md",
      "purpose": "Database choice for the checkout flow"
    }
  ]
}
```

`openspec/config.yaml`'s `context:` field (Authoritative sources) is generated dynamically by the `planning_agent` by iterating this array — never hardcoded (ADR-006, ADR-007).

### `workflow-config.yml` (per cycle)

The single file the user edits before running `/hero:start`. Full annotated example:

```yaml
title: New Feature
objective: Implement a new feature.

# Workflow-level preferences (chat language, etc.). Cycle artifacts stay English.
workflow_config:
  # Language agents use when talking to the user in chat (e.g. EN, PT-BR).
  user_preferred_language: EN

scope:
  backend: true
  frontend: false
  native: false
  script: false
  infrastructure: false

stages:
  research:
    enabled: true
    purpose: Collaborate with the user to gather requirements and produce the project specifications.
    max_iterations: 50
    timeout_minutes: 15
    require_human_approval: false

  planning:
    enabled: true
    purpose: Convert the approved specifications into a complete SDD ready for implementation.
    max_iterations: 3
    timeout_minutes: 20
    require_human_approval: false

  implementation:
    enabled: true
    purpose: Implement the approved SDD.
    max_iterations: 4
    timeout_minutes: 30
    require_human_approval: false

  qa:
    enabled: true
    purpose: Validate implementation quality (tests, coverage, architecture, lint, build).
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: true

  judge:
    enabled: true
    purpose: Verify SDD requirement coverage.
    max_iterations: 3
    timeout_minutes: 10
    require_human_approval: false

  browser_ui_validation:
    enabled: false
    purpose: Validate browser UI health (render, console, network/CSS) and optional visual comparison.
    max_iterations: 2
    timeout_minutes: 15
    require_human_approval: true
    # Visual Validation runs only after Browser Health passes.
    # Health has no separate toggle — it always runs when this stage is enabled.
    visual_validation:
      enabled: false
      reference_dir: docs/ui/visual_reference

  qa_end_to_end:
    enabled: true
    purpose: Validate the complete feature end-to-end.
    max_iterations: 1
    timeout_minutes: 15
    require_human_approval: true
    # When true and scope.frontend is true, end2end_qa_agent uses Playwright
    # for browser journeys. Must be false when scope.frontend is false
    # (direct HTTP calls only). Independent of browser_ui_validation.
    use_playwright: false

agents:
  planning_agent:
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
    # Nested Task fan-out from this agent (not named Hero agents like context_agent).
    # same_of_agent: true → reuse this agent's model; false → use subagent.model below.
    subagent:
      same_of_agent: false
      model: composer-2.5
      reasoning_effort: na
      enable_fast_model: false
      thinking: na

  context_agent:
    model: composer-2.5
    reasoning_effort: na
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: false
      model: composer-2.5
      reasoning_effort: na
      enable_fast_model: false
      thinking: na

  backend_agent:
    model: cursor-grok-4.5
    reasoning_effort: high
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: false
      model: composer-2.5
      reasoning_effort: na
      enable_fast_model: false
      thinking: na

  frontend_agent:
    model: claude-sonnet-5
    reasoning_effort: high
    enable_fast_model: false
    thinking: false
    subagent:
      same_of_agent: false
      model: composer-2.5
      reasoning_effort: na
      enable_fast_model: false
      thinking: na

  generic_agent:
    model: claude-sonnet-5
    reasoning_effort: medium
    enable_fast_model: false
    thinking: false
    subagent:
      same_of_agent: false
      model: composer-2.5
      reasoning_effort: na
      enable_fast_model: false
      thinking: na

  qa_agent:
    model: gpt-5.3-codex
    reasoning_effort: medium
    enable_fast_model: false
    thinking: na
    subagent:
      same_of_agent: false
      model: composer-2.5
      reasoning_effort: na
      enable_fast_model: false
      thinking: na

  judge_agent:
    model: claude-sonnet-5
    reasoning_effort: high
    enable_fast_model: false
    thinking: false
    subagent:
      same_of_agent: false
      model: composer-2.5
      reasoning_effort: na
      enable_fast_model: false
      thinking: na

  browser_ui_agent:
    model: claude-sonnet-5
    reasoning_effort: high
    enable_fast_model: false
    thinking: false
    subagent:
      same_of_agent: false
      model: composer-2.5
      reasoning_effort: na
      enable_fast_model: false
      thinking: na

  end2end_qa_agent:
    model: claude-4.6-sonnet
    reasoning_effort: medium
    enable_fast_model: false
    thinking: false
    subagent:
      same_of_agent: false
      model: composer-2.5
      reasoning_effort: na
      enable_fast_model: false
      thinking: na

# Fallback model used when an agent's configured model is unavailable.
# The user is always warned explicitly when this fallback is used.
fallback_model:
  model: claude-sonnet-5
  reasoning_effort: medium
  enable_fast_model: false
  thinking: na

workflow_rules:
  - Skip any stage that is not enabled.
  - Do not start implementation until the PRD has been approved, if the research stage is enabled.
  - If the research stage is disabled, require the `objective` field above to be well described and ask the user for explicit scope confirmation before starting implementation.
  - At least one of the five `scope` fields must be true when implementation is enabled.
  - `stages.browser_ui_validation.enabled` may be true only when `scope.frontend` is true; otherwise block and ask for correction.
  - When Browser UI Validation is enabled, Browser Health always runs (Playwright required at execution); Visual Validation runs only when `visual_validation.enabled` is true and only after Health passes.
  - `stages.qa_end_to_end.use_playwright` may be true only when `scope.frontend` is true; otherwise block and ask for correction.
  - When `use_playwright` is true, `end2end_qa_agent` uses Playwright; when false, it uses direct HTTP calls.
  - All agents must communicate with the user in chat using `workflow_config.user_preferred_language` (default `EN`), unless the user explicitly asks for a different chat language. Cycle artifacts remain English.
  - On `/hero:new` with prior cycles, import previous `workflow_config` + `fallback_model` + `stages` + `agents`; always reset `title`, `objective`, and `scope` to template defaults.
  - Do not change the architecture without an approved ADR.
  - Update workflow.md after completing each stage.
  - Before finishing the workflow, ensure current-state.md is up to date.
```

### `.workflow-hero/cycles/current/workflow.md` — allowed field values

| Field | Allowed values |
|---|---|
| `Status` | `Waiting`, `Disable`, `In Progress`, `Completed`, `Cancelled`, `Paused` (also cycle-level `Finished by User` when closed via `/hero:finish` early) |
| `Started` / `Completed` | `YYYY-MM-DD` local calendar dates. **Started** set on `/hero:new` via `date +%Y-%m-%d`. **Completed** set on `/hero:finish` (or when the cycle is marked completed) the same way. `/hero:archive` MUST use **Completed** as the date segment in `C<N>-YYYY-MM-DD-<slug>/` — never invent “today” from chat context. |
| `Human Approval` | `N/A`, `Disable`, `Pending`, `Escalated`, `Rejected`, `Approved`, `Cancelled` |
| `Extra Iterations Granted` | Integer, default `+0`, incremented on every `/hero:continue` for that stage |

### `models/*.yml` (pricing reference files)

One file per provider (`openai.yml`, `anthropic.yml`, `google.yml`, `cursor.yml`, `moonshot.yml`, `zhipu.yml`, `xai.yml`), refreshed by `hero update-models` (see [DEPLOY.md §8](../deployment/DEPLOY.md#8-pricing-data-updates)). Common shape:

```yaml
provider: anthropic
version: 1
last_updated: 2026-07-14
currency: usd
unit: per_1m_tokens

models:
  claude-sonnet-5:
    input: 3
    cache_write: 3.75
    cache_read: 0.3
    output: 15
```

Used by the token/cost estimation heuristic (character count ÷ ~4, multiplied by the model's price here) referenced in [PRD.md §5.10](../product/PRD.md#510-metrics).
