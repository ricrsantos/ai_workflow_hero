# Architecture Decision Records — AI Workflow Hero

> Index of all ADRs for the Hero project itself (the Go CLI + Runtime assets), consolidated from `docs/idea/ai_workflow_hero.md`. Each ADR follows the standard Context / Decision / Consequences format.

| # | Title | Status |
|---|---|---|
| [ADR-001](#adr-001-go-cobra-and-embedfs-for-cli-distribution) | Go, Cobra, and `embed.FS` for CLI distribution | Accepted |
| [ADR-002](#adr-002-repository-architecture-feature-based--vertical-slice) | Repository architecture: Feature Based + Vertical Slice | Accepted |
| [ADR-003](#adr-003-cli-vs-runtime-separation) | CLI vs. Runtime separation | Accepted |
| [ADR-004](#adr-004-git-as-a-mandatory-prerequisite) | Git as a mandatory prerequisite | Accepted |
| [ADR-005](#adr-005-subagent-invocation-via-task-tool-with-clean-sessions) | Subagent invocation via Task tool, with clean sessions | Accepted |
| [ADR-006](#adr-006-simple-placeholder-templating-no-loop-engine) | Simple placeholder templating, no loop engine | Accepted |
| [ADR-007](#adr-007-openspec-as-the-sdd-framework) | OpenSpec as the SDD framework | Accepted |
| [ADR-008](#adr-008-three-level-model-fallback-chain) | Three-level model fallback chain | Accepted |
| [ADR-009](#adr-009-test-real-dependencies-over-mocks) | Test real dependencies over mocks | Accepted |
| [ADR-010](#adr-010-manual-release-process-via-a-single-script) | Manual release process via a single script | Accepted |

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

**Decision**: Invoke every subagent (`backend_agent`, `frontend_agent`, `generic_agent`, `qa_agent`, `judge_agent`, `end2end_qa_agent`, `context_agent`) via the IDE's Task tool, in a **fresh, isolated session** that does not inherit the orchestrator's chat history. Subagents receive **file pointers** (paths to `AGENTS.md`, `current-state.md`, the relevant SDD/tasks) instead of pasted content. The orchestrator absorbs back only the subagent's final structured output (its documented "Output Format" section), not its intermediate reasoning. The `orchestration_agent`'s own main session remains continuous across the whole cycle — only subagents are session-scoped per call.

**Consequences**:
- Token usage per subagent call is bounded by what it actually needs to read, not by the orchestrator's accumulated history.
- Each `*_agent.md` prompt must be self-sufficient: it must instruct the subagent to read the pointed-to files itself, since it starts with no prior context.
- Debugging a subagent's reasoning requires inspecting its own session/logs, not the orchestrator's transcript.

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

**Decision**: Implement a 3-level fallback: 1) the agent's model as configured in `workflow-config.yml`; 2) `generic_model`, a top-level fallback field in the same file — **the user is always explicitly warned whenever this fallback activates**; 3) if still unavailable, the orchestrator warns the user and waits for `/hero:continue` after they fix the configuration.

**Consequences**:
- `generic_model` is scoped per-cycle (top-level in `workflow-config.yml`), not globally in `hero.json`, since budget/availability may vary by cycle.
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
