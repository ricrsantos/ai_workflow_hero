# asset-bootstrap-and-layout Specification

## MODIFIED Requirements

### Requirement: Release artifacts SHALL include optional plugin daemon binaries

Hero releases SHALL publish platform-matched Telegram daemon binaries alongside the main `hero` binary with checksum coverage. Embed/extract layout SHALL version the daemon with the Hero release it matches (PRD-C09-001 §3.1; DEPLOY.md).

#### Scenario: Release contract lists daemon artifacts
- **WHEN** `scripts/release_test.go` validates a release manifest
- **THEN** each supported GOOS/GOARCH includes a telegram daemon entry

#### Scenario: Install extracts daemon next to plugin metadata
- **WHEN** plugin install runs on linux/arm64
- **THEN** the arm64 daemon binary is written to the plugin directory recorded in metadata
