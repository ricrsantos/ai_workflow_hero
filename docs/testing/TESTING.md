# Testing — AI Workflow Hero

> Test strategy and commands for the Hero CLI repository.

## Test command

```bash
go test ./...
```

Run from the repository root after any code change. All tests must pass before marking work complete.

## Strategy

- **Unit tests**: colocated `*_test.go` in each `internal/<feature>/` package; same package; test behavior, not implementation details.
- **Golden tests**: template rendering and asset output fixtures.
- **Integration tests**: compiled `hero` binary against `t.TempDir()` for `install`, `upgrade`, `uninstall`, and `doctor`. Cover install without `--tools`, `--tools` error, 1.x-style upgrade leaving OpenCode disabled, and OpenCode projection on enable.
- **Dependencies**: prefer real filesystem and `embed.FS` over mocks; keep tests deterministic and fast.

## Coverage areas

| Area | Packages / paths |
|---|---|
| Install / upgrade / uninstall | `internal/install`, `internal/upgrade`, `internal/uninstall` |
| Doctor / status | `internal/doctor`, `internal/status` |
| Cycle / store / engine | `internal/cycle`, `internal/store`, `internal/engine` |
| TUI / harness | `internal/tui`, `internal/harness`, `internal/adapters/cursor`, `internal/adapters/opencode` |
| Templates / assets | `internal/common/template`, `assets/` |
| Release contract | `scripts/release_test.go` |

## CI

No CI workflow is required for V1 (manual release via `scripts/release.sh`). CI/CD automation is deferred to V2 (see PRD §2.3).
