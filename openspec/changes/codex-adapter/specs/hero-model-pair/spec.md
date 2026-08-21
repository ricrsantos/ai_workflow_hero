# hero-model-pair Specification

## MODIFIED Requirements

### Requirement: Harness-first picker SHALL offer Codex when enabled
`/hero-model` step 1 SHALL include Codex alongside Cursor and OpenCode when `harnesses.codex.enabled` is true (UI-C06-001 §4; PRD-C06-001 §4.8).

#### Scenario: Codex omitted when disabled
- **WHEN** Codex is not enabled
- **THEN** step 1 does not list Codex

#### Scenario: Esc returns from model list to harness list including Codex
- **WHEN** the user presses Escape on step 2 after choosing Codex
- **THEN** step 1 harness list is shown again with Codex present when enabled

### Requirement: Codex model list SHALL use adapter ListModels
Step 2 for Codex SHALL enumerate native ids from `CodexAdapter.ListModels`, which MAY start app-server lazily (OpenCode analog) (PRD-C06-001 §4.8).

#### Scenario: Native ids only
- **WHEN** step 2 lists Codex models
- **THEN** entries are native Codex ids with no Hero translation table
