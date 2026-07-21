## Why

The repository has complete V1 product and architecture documentation but still has no Go implementation (`context/current-state.md`). This change is needed to convert the approved V1 definition into executable CLI features plus Runtime assets, preserving deterministic CLI behavior and the Runtime orchestration model (PRD §5.8, §5.9; ADR-003).

## What Changes

- Implement the full V1 command suite in the Go CLI (`hero install --tools cursor`, `upgrade`, `uninstall`, `doctor`, `version`, `variables`, `update-models`, `status`, `help`) as deterministic behavior only (PRD §5.8, §6; ADR-003).
- Deliver the Runtime asset set for V1 workflow orchestration (stage flow, approval/control loop, iteration/timeout handling, scope routing, model fallback, metrics) under Cursor command/agent files (PRD §5.1-§5.6, §5.9-§5.10; ADR-011).
- Implement installation/bootstrap behavior that writes Hero-owned assets and config files, preserving project artifacts and user customizations where required (DEPLOY §6-§8; PRD §6).
- Implement template rendering using simple placeholder substitution only (`{{path.key}}`), with no loop engine (ADR-006).
- Add test coverage by feature package and integration gates consistent with repository policy (AGENTS.md Testing; ADR-009; PRD §6; UI §7).

### In Scope (V1)

- Cursor-only workflow and assets (PRD §2.2).
- Stages Research -> Planning -> Implementation -> QA -> Judge -> QA End-to-End (PRD §2.2, §5.1).
- Basic `/hero:sync` activation behavior (PRD §2.2).
- Linux/macOS binaries for `amd64` and `arm64`; manual script-assisted release model (PRD §2.2; DEPLOY §2, §4).

### Out of Scope (V1)

- Additional IDE/harness environments beyond Cursor (PRD §2.3, §7).
- Optional V2 stages (UX, Observability, Security review, Architecture review, Deployment specifications) (PRD §2.3).
- Advanced sync/drift detection for existing projects (PRD §2.3, §7).
- Windows binaries and CI/CD-automated releases (PRD §2.3, §7; DEPLOY §2, §4).
- GPG artifact signing and non-interactive-only CLI model (PRD §7; DEPLOY §5).

### CLI vs Runtime Classification (ADR-003)

- **CLI (deterministic):** all `hero` commands in PRD §5.8, file operations, validation, upgrade/uninstall safety checks, status/variables/model updates, release-supporting local behavior.
- **Runtime (reasoning-driven):** all `/hero:*` orchestration semantics in PRD §5.1-§5.6 and §5.9, implemented as command and agent asset files consumed in chat.
- This change intentionally includes both layers, but keeps behavior partitioned by artifact/package boundaries mandated by ADR-003.

## Capabilities

### New Capabilities

- `cli-deterministic-command-suite`: Deterministic implementation of all V1 `hero` commands, including UX/output conventions and safety constraints.
- `runtime-workflow-execution`: Runtime stage orchestration, approval loop, retries/escalation, scope routing, fallback chain, and metrics updates.
- `asset-bootstrap-and-layout`: Embedded asset packaging, install/upgrade/uninstall lifecycle, one-file-per-command/agent mapping, and simple template substitution.

### Modified Capabilities

- None.

## Impact

- New/expanded CLI feature packages under `internal/<feature>/` per ADR-002, including at minimum: `install`, `upgrade`, `uninstall`, `doctor`, `status`, `variables`, `update_models` (plus command wiring and shared support in `internal/common/`).
- Cursor adapter logic under `internal/adapters/cursor/` for path/layout handling (ADR-002).
- Runtime assets in `.cursor/commands/`, `.cursor/agents/`, `.cursor/skills/workflow-hero/`, and Hero templates/config scaffolding under `.workflow-hero/`.
- Config/data schema compliance for `.workflow-hero/config/hero.json`, `project.json`, `documents.json`, `workflow-config.yml`, and `models/*.yml` (ADR Appendix; PRD §5.10).
- Test suite additions: feature-level `_test.go`, template golden tests, and lightweight integration tests; final gate `go test ./...` (ADR-009; UI §7; AGENTS.md Testing).
