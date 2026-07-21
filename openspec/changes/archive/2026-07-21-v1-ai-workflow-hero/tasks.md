## 1. Repository Scaffold and Command Wiring (Series)

- [x] 1.1 Create Go module scaffold (`cmd/hero`, root command wiring, shared bootstrap) aligned with ADR-002
- [x] 1.2 Create base package structure `internal/common/`, `internal/adapters/cursor/`, and `internal/<feature>/` placeholders
- [x] 1.3 Implement CLI command registration for all V1 commands listed in PRD §5.8
- [x] 1.4 Add foundational validation and error helpers to enforce UI §5 error structure
- [x] 1.5 Add initial command wiring tests in `cmd/hero/*_test.go`

## 2. Install Feature First (Series, prerequisite for lifecycle commands)

- [x] 2.1 Implement `internal/install/command.go` for `hero install --tools cursor` with interactive and non-interactive behavior (UI §4)
- [x] 2.2 Implement `internal/install/validator.go` for git prerequisite and required input checks (ADR-004)
- [x] 2.3 Implement `internal/install/service.go` to materialize Hero-owned assets and config from embedded files (DEPLOY §6; ADR-001)
- [x] 2.4 Add `internal/install/service_test.go` using `t.TempDir()` and real filesystem behavior (ADR-009)
- [x] 2.5 Add install output-format tests for success/warn/error conventions per UI §2 and §5

## 3. Lifecycle Maintenance Features (Parallel after 2.x)

Parallelizable: 3.1/3.2, 3.3/3.4, and 3.5/3.6 can run in parallel once 2.x is done.

- [x] 3.1 Implement `internal/upgrade/command.go` and `service.go` with non-overwrite customization protection (PRD §6; DEPLOY §7)
- [x] 3.2 Add `internal/upgrade/service_test.go` covering checksum-based overwrite prevention
- [x] 3.3 Implement `internal/uninstall/command.go` and `service.go` removing only Hero-owned paths (DEPLOY §7)
- [x] 3.4 Add `internal/uninstall/service_test.go` verifying project artifacts are preserved
- [x] 3.5 Implement `internal/doctor/command.go`, `validator.go`, and `service.go` for integrity/version/config checks (DEPLOY §7)
- [x] 3.6 Add `internal/doctor/service_test.go` for missing files, schema issues, and version mismatch scenarios

## 4. Read/Admin Commands (Parallel after 1.x; independent from 3.x except shared helpers)

Parallelizable: 4.1/4.2, 4.3/4.4, 4.5/4.6, and 4.7/4.8 can run in parallel.

- [x] 4.1 Implement `internal/status/command.go` and `service.go` with table + `--json` output modes (UI §3.1)
- [x] 4.2 Add `internal/status/service_test.go` and golden output tests for table/JSON modes
- [x] 4.3 Implement `internal/variables/command.go` and `service.go` with table + `--json` output modes (UI §3.1)
- [x] 4.4 Add `internal/variables/service_test.go` and output-format tests
- [x] 4.5 Implement `internal/update_models/command.go` and `service.go` using structured upstream source (DEPLOY §8; ADR-003)
- [x] 4.6 Add `internal/update_models/service_test.go` covering rewrite behavior and failure handling
- [x] 4.7 Implement deterministic `version` and `help` command behavior and wiring in CLI command layer
- [x] 4.8 Add `cmd/hero/version_help_test.go` for output and exit behavior

## 5. Runtime Assets and Template System (Series with selective parallel work)

Parallelizable after 5.1: 5.2 and 5.3 can run in parallel; 5.4 depends on both.

- [x] 5.1 Add embedded asset loading pipeline (`embed.FS`) and installation copy logic used by install/upgrade (ADR-001)
- [x] 5.2 Author `.cursor/commands/hero-*.md` Runtime command assets matching PRD §5.9 and ADR-011
- [x] 5.3 Author `.cursor/agents/*.md` agent assets matching PRD §5.2 and ADR-011
- [x] 5.4 Add `.workflow-hero/templates/` artifacts including workflow/config/metrics templates with ADR-006 placeholder constraints
- [x] 5.5 Add `internal/common/template` renderer supporting only `{{path.key}}` replacement (ADR-006)
- [x] 5.6 Add `internal/common/template/template_test.go` and golden tests proving no loop-engine behavior
- [x] 5.7 Add asset inventory validation tests (command/agent one-file mapping) in `internal/common/assets_test.go`

## 6. Runtime-Orchestration Contract Assets (Series)

- [x] 6.1 Encode stage flow semantics (PRD §5.1) in Runtime command/agent assets
- [x] 6.2 Encode approval/control loop command semantics (PRD §5.3, §5.9) in Runtime assets
- [x] 6.3 Encode iteration/timeout/escalation/backtracking semantics (PRD §5.4) in Runtime assets
- [x] 6.4 Encode scope routing and model fallback semantics (PRD §5.5-§5.6; ADR-008) in Runtime assets
- [x] 6.5 Encode metrics update behavior and references to `metrics.md` and `metrics-summary.md` (PRD §5.10)
- [x] 6.6 Validate Runtime assets against PRD/ADR checklists in `internal/common/runtime_assets_test.go`

## 7. Integration, Release-Safety, and Final Validation (Series)

- [x] 7.1 Add lightweight integration tests for install/upgrade/uninstall/doctor in `internal/integration/*_test.go` using temp directories (ADR-009)
- [x] 7.2 Add release-script contract test coverage for expected artifact naming/checksum assumptions where feasible (`scripts/release.sh` behavior from DEPLOY §4)
- [x] 7.3 Run `go test ./...`
- [x] 7.4 Fix any failing tests and re-run `go test ./...` until green
- [x] 7.5 Confirm no V1 out-of-scope items were introduced (PRD §2.3, §7) before marking change ready
