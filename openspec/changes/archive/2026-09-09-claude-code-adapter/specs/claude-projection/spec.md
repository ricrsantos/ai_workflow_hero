## Purpose

Project Claude-native Hero commands, skills, agents, and shared memory safely while keeping Claude opt-in and preserving every user-owned file outside the marked Hero block.

## ADDED Requirements

### Requirement: Enabling Claude SHALL project embedded assets with ownership tracking

Selecting Claude during install or through `/hero-harness` SHALL project the embedded commands, `workflow-hero` and `grilling` skills, and native agent templates into `.claude/`, record managed files in the existing checksum system, and report that `.claude/` was projected. The projection SHALL not copy or invent the root `AGENTS.md` (PRD-C13-001 §2; ADR-073).

#### Scenario: Install selects Claude
- **WHEN** a user selects Claude during a fresh install
- **THEN** install creates the expected `.claude/` commands, skills, and agent files before reporting success

#### Scenario: Runtime enables Claude
- **WHEN** the user enables Claude through `/hero-harness`
- **THEN** the TUI projects `.claude/`, persists `harnesses.claude.enabled=true`, and reports `Claude enabled (projected .claude/)`

#### Scenario: Root instructions are not duplicated
- **WHEN** Claude projection runs in a project with `AGENTS.md`
- **THEN** no copy of `AGENTS.md` is written under `.claude/`

### Requirement: Disabled or upgraded Claude SHALL remain unprojected until explicit enablement

An upgrade SHALL add or preserve disabled Claude state without creating `.claude/` or `CLAUDE.md`. Disabling Claude SHALL keep existing `.claude/` files and only change the enabled state, subject to the existing last-harness guard (PRD-C13-001 §2; ADR-070, ADR-073).

#### Scenario: Upgrade from a prior Hero version
- **WHEN** a project upgrades without Claude explicitly enabled
- **THEN** `harnesses.claude.enabled` is false and neither `.claude/` nor a new `CLAUDE.md` is created

#### Scenario: Disable keeps files
- **WHEN** Claude is disabled while another harness remains enabled
- **THEN** `.claude/` and user-added files remain on disk and the TUI reports `Claude disabled (files kept)`

#### Scenario: Last-harness guard
- **WHEN** Claude is the last enabled harness and the user attempts to disable it
- **THEN** the operation is rejected and the enabled state and projection remain unchanged

### Requirement: Hero SHALL manage only a marked CLAUDE.md block

When the user explicitly chooses managed context, Hero SHALL create or update only a marker-delimited root `CLAUDE.md` block that imports `@AGENTS.md` and identifies the Hero workflow/context paths. Existing user text SHALL remain byte-for-byte outside that block. The user SHALL be offered managed insert/update with a diff or leave-unchanged; a missing `AGENTS.md` SHALL fail with guidance rather than cause Hero to invent a copy (PRD-C13-001 §2; ADR-073; UI-C13-001 §2).

#### Scenario: New managed memory file
- **WHEN** Claude is enabled and the user accepts managed context in a project with `AGENTS.md`
- **THEN** Hero creates `CLAUDE.md` containing the marked Hero block with `@AGENTS.md` and Hero context paths

#### Scenario: Existing file preservation choice
- **WHEN** `CLAUDE.md` contains user instructions outside a Hero block
- **THEN** the user can inspect a diff and insert/update only the marked block or leave the file unchanged, with user text preserved in either case

#### Scenario: Missing AGENTS.md
- **WHEN** managed context is requested but `AGENTS.md` is absent
- **THEN** Hero refuses to create the import block and explains how to provide the required project instructions

#### Scenario: Upgrade and uninstall ownership
- **WHEN** upgrade or uninstall processes an existing `CLAUDE.md`
- **THEN** only the Hero-marked block is updated or removed; all unmarked instructions survive

### Requirement: Prepare SHALL update only marked Claude agent fields

Before a Claude-backed Hero start, preparation SHALL synchronize only Hero-managed model, effort, and applicable skill fields in projected agent frontmatter. User-authored body text and unmarked frontmatter SHALL remain unchanged (PRD-C13-001 §2; ADR-073).

#### Scenario: Managed frontmatter refresh
- **WHEN** workflow configuration changes the model or supported effort for a Claude agent
- **THEN** preparation updates the marked/managed fields in `.claude/agents/<agent>.md` without replacing the agent body

