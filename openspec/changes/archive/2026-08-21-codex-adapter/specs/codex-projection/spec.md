# codex-projection Specification

## Purpose
Provision project `.codex/` from embedded `assets/codex/` when Codex is enabled, following OpenCode projection lifecycle (ADR-046; PRD-C06-001 §4.7).

## ADDED Requirements

### Requirement: Enabling Codex SHALL provision .codex from assets
When the user enables Codex via install or `/hero-harness`, Hero SHALL immediately project agents, commands, skills, and any minimal adapter-required config from `assets/codex/` into `.codex/` and track Hero-managed paths in checksums (ADR-046).

#### Scenario: Install enable provisions
- **WHEN** the user selects Codex during `hero install`
- **THEN** `.codex/` is written before install completes successfully

#### Scenario: Runtime enable provisions
- **WHEN** the user enables Codex via `/hero-harness`
- **THEN** `.codex/` is projected and success copy names projected `.codex/`

### Requirement: Disabling Codex SHALL keep projection files
Disabling Codex SHALL set `harnesses.codex.enabled=false` only. Hero SHALL NOT delete `.codex/` or user-added files (PRD-C06-001 §4.6; ADR-046).

#### Scenario: Disable keeps files
- **WHEN** the user disables Codex
- **THEN** `.codex/` remains on disk and success copy states files were kept

### Requirement: Root AGENTS.md SHALL NOT be copied into .codex
Hero SHALL NOT duplicate root `AGENTS.md` into `.codex/` as part of projection (ADR-046; PRD-C06-001 §4.7).

#### Scenario: No AGENTS duplication
- **WHEN** projection runs
- **THEN** root `AGENTS.md` is not written under `.codex/`

### Requirement: Customized Hero-managed .codex files SHALL use conflict backup
When upgrade or enable would overwrite a user-customized Hero-managed file, Hero SHALL back up `{filename}_{timestamp}.conflict` and replace with the embedded version, matching OpenCode checksum rules (ADR-046).

#### Scenario: Conflict backup on upgrade
- **WHEN** upgrade detects a customized Hero-managed `.codex/` file differs from embedded checksum
- **THEN** the prior file is backed up with a `.conflict` suffix before replacement
