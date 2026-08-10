# hero-sync-pending-docs Specification

## Purpose
Extend `/hero-sync` to merge pending items from product and architecture docs into `current-state.md` (ADR-029; PRD-C03-001 §4.8).

## ADDED Requirements

### Requirement: hero-sync SHALL scan product and architecture docs for pending items
During `/hero-sync`, after the existing codebase scan, the orchestrator SHALL analyze `docs/product/` and `docs/architecture/` (including cycle PRDs/ADRs) for explicit pending, deferred, or not-yet-implemented items (design D9).

#### Scenario: Pending item in PRD index
- **WHEN** a cycle PRD lists deferred or pending work
- **THEN** sync incorporates that item into `context/current-state.md` pending sections

### Requirement: hero-sync SHALL dedupe pending entries
Merged items SHALL avoid duplicate bullets when the same pending work already exists in `current-state.md` (design D9).

#### Scenario: Duplicate pending text
- **WHEN** an item from docs matches an existing pending bullet
- **THEN** sync does not add a second identical entry
