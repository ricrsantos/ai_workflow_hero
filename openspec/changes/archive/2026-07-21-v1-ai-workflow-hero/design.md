## Context

Hero V1 is fully specified in project documents, but implementation has not started (`context/current-state.md`). The design must translate the approved product model into code while preserving hard constraints: Feature Based + Vertical Slice architecture (ADR-002), strict CLI vs Runtime boundary (ADR-003), and simple placeholder templating (ADR-006).  
The change covers both deterministic CLI capabilities (PRD §5.8) and reasoning-driven Runtime orchestration assets (PRD §5.9), with V1 scope and exclusions explicitly constrained by PRD §2.2, §2.3, and §7.

## Goals / Non-Goals

**Goals:**

- Implement all V1 CLI commands with deterministic behavior and testability (PRD §5.8, §6; ADR-003, ADR-009).
- Materialize Runtime orchestration behavior in command/agent assets with one-file-per-command/agent mapping (PRD §5.9; ADR-011).
- Ensure installation/bootstrap and lifecycle operations honor safety rules (git prerequisite, non-overwrite upgrade behavior, scoped uninstall) (PRD §6; DEPLOY §6-§8; ADR-004).
- Keep template rendering limited to `{{path.key}}` substitutions without loops/conditionals (ADR-006).
- Preserve schema compatibility for all documented config/data files (ADR Appendix).

**Non-Goals:**

- Supporting non-Cursor environments or V2 optional stages (PRD §2.3, §7).
- Implementing advanced sync/drift detection in this change (PRD §2.3, §7).
- Adding Windows release targets, CI/CD release automation, or GPG signing (PRD §2.3, §7; DEPLOY §2, §4, §5).
- Introducing any CLI command that performs LLM reasoning (ADR-003).

## Decisions

1. **Repository decomposition by feature packages (ADR-002).**
   - Decision: implement each CLI capability as its own `internal/<feature>/` vertical slice (`command.go`, `service.go`, `validator.go` as needed), with shared concerns in `internal/common/` and Cursor-specific logic in `internal/adapters/cursor/`.
   - Alternative considered: command-centric monolith with a shared giant service layer.
   - Rationale: better isolation, test colocation, and extensibility aligned with ADR-002/ADR-009.

2. **Explicit split between deterministic CLI and reasoning Runtime assets (ADR-003).**
   - Decision: all orchestration intelligence remains in Runtime slash-command and agent markdown assets; CLI only installs, validates, reports, and maintains.
   - Alternative considered: adding `hero sync/start` orchestration behavior directly in Go CLI.
   - Rationale: rejected because it violates determinism and ADR-003.

3. **Install-first dependency chain for lifecycle commands.**
   - Decision: implement `install` before `upgrade`/`uninstall`/`doctor`, because these commands depend on known installed layout and metadata (`hero.json`, asset checksums/version state) (DEPLOY §6-§8).
   - Alternative considered: implementing all commands independently in parallel.
   - Rationale: dependency ordering reduces rework and contract drift.

4. **Template rendering strategy constrained to simple key substitution (ADR-006).**
   - Decision: implement a minimal renderer for `{{path.key}}` only; list/table generation is authored directly in content assets, not template loops.
   - Alternative considered: full Mustache/Go template engine.
   - Rationale: unnecessary complexity for V1 and contradicts ADR-006.

5. **Schema-first validation for config/data files (ADR Appendix).**
   - Decision: validators enforce documented required fields and allowed values for `hero.json`, `project.json`, `documents.json`, `workflow-config.yml`, and `workflow.md` status fields before command success.
   - Alternative considered: permissive parsing with best-effort behavior.
   - Rationale: deterministic failures with actionable errors are required by UI §5 and PRD §6.

6. **Runtime asset completeness governed by ADR-011 mapping.**
   - Decision: ship exactly one file per Runtime command and per agent prompt in the expected directories; CLI install/doctor verify this inventory.
   - Alternative considered: partial asset set and lazy generation.
   - Rationale: deterministic bootstrap and integrity checks require a known complete set.

## Risks / Trade-offs

- **[Risk] Full V1 breadth in one change is large** -> **Mitigation:** enforce milestone ordering (`install` first), incremental feature tests, and integration gates after each batch.
- **[Risk] Boundary leaks between CLI and Runtime responsibilities** -> **Mitigation:** code review checklist keyed to ADR-003 and explicit package ownership.
- **[Risk] Asset overwrite regressions during `upgrade`** -> **Mitigation:** checksum-based customization detection and warning-only behavior (PRD §6; DEPLOY §7).
- **[Risk] Incomplete Runtime asset mapping** -> **Mitigation:** ADR-011 inventory test asserting required command/agent files exist.
- **[Risk] Template/rendering edge cases** -> **Mitigation:** golden tests for placeholder substitution and explicit non-support tests for loop syntax (ADR-006, ADR-009).

## Migration Plan

1. Scaffold module and feature package skeleton per ADR-002.
2. Implement and test `install` with real filesystem temp dirs.
3. Implement `upgrade`, `uninstall`, and `doctor` against installed-layout contracts.
4. Implement read/admin commands (`status`, `variables`, `update-models`, plus command wiring for `version`/`help`) with UI-conformant output modes.
5. Add Runtime assets, templates, and model files per PRD §5.9/ADR-011.
6. Add integration tests for install/upgrade/uninstall/doctor.
7. Run `go test ./...`; fix failures until green (AGENTS.md Testing; ADR-009).

Rollback strategy: abort release, discard unmerged branch, and if release artifacts were generated, regenerate from the last known-good tag only after the test gate is green (DEPLOY §4).

## Open Questions

- None blocking for artifact creation. Current scope assumptions are conservative and directly anchored in PRD/ADR/DEPLOY/current-state.
