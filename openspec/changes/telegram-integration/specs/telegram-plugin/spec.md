# telegram-plugin Specification

## Purpose

Optional official Telegram plugin install lifecycle and release coupling (PRD-C09-001 §3.1; ADR-059).

## ADDED Requirements

### Requirement: Telegram SHALL NOT install with default Hero bootstrap

`hero install` and `hero upgrade` SHALL NOT install or enable the Telegram plugin automatically. Installation SHALL require explicit `hero plugin install telegram` (PRD-C09-001 §3.1).

#### Scenario: Fresh install has no Telegram plugin
- **WHEN** a project runs `hero install` without plugin commands
- **THEN** Settings shows the not-installed guidance string

### Requirement: Plugin install SHALL verify matching release artifacts

Install SHALL copy the platform-matched daemon binary and write plugin metadata including Hero semver and protocol version. Mismatched or missing artifacts SHALL fail with an actionable error (ADR-059).

#### Scenario: Install succeeds with matching artifact
- **WHEN** `hero plugin install telegram` runs on a supported platform with bundled artifacts
- **THEN** plugin state records installed version and daemon path

#### Scenario: Missing artifact fails closed
- **WHEN** the release layout lacks a daemon for the current GOOS/GOARCH
- **THEN** install errors without partial plugin state

### Requirement: Doctor SHALL report plugin and daemon health

Doctor/status SHALL report whether the plugin is installed, whether the daemon binary exists, and whether plugin version matches the running `hero` version (warning on mismatch).

#### Scenario: Version mismatch warns
- **WHEN** an older daemon binary remains after `hero upgrade`
- **THEN** doctor emits a compatibility warning with reinstall guidance
