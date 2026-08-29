# workflow-config-document Specification

## Purpose

Keep `workflow-config.yml` as the only cycle configuration document. Provide a round-trip-safe managed-node API so the TUI Config screen can edit known fields without destroying comments, order, `workflow_rules`, or future keys (PRD-C07-001 §4.2/§4.5–§4.6; ADR-050).

## ADDED Requirements

### Requirement: Document load SHALL project managed fields without requiring a full struct rewrite

`internal/workflowconfig` SHALL load the active cycle file as a `yaml.v3.Node` tree plus a typed `ManagedConfig`. Load SHALL fail closed when the file is missing, unreadable, or not a YAML mapping. It SHALL never fall back to the installed template for the Config screen (PRD-C07-001 §4.1/§4.5; ADR-049).

#### Scenario: Valid file loads managed identity
- **WHEN** the active `workflow-config.yml` contains `title`, `objective`, scopes, stages, and agents
- **THEN** `ManagedConfig` exposes those managed fields and the node tree still contains comments and unmanaged keys

#### Scenario: Missing file is not replaced
- **WHEN** `.workflow-hero/cycles/current/workflow-config.yml` is absent
- **THEN** load returns an error naming the path and no template file is created

#### Scenario: Invalid YAML is not replaced
- **WHEN** the active file cannot be parsed as a mapping
- **THEN** load returns a parse error and the file bytes are unchanged

### Requirement: Save SHALL merge the TUI draft onto the latest valid file for managed paths only

Save SHALL read the current file, apply the in-memory draft only to managed YAML paths, preserve unmanaged content from the latest file, validate, and atomically replace the file via a same-directory temporary file and rename. A revision mismatch SHALL NOT fail Save and SHALL NOT open a conflict dialog. If the latest file is missing or invalid, Save SHALL fail closed and leave the file untouched (PRD-C07-001 §4.5; UI-C07-001 §7; ADR-050).

#### Scenario: Comments and workflow_rules survive a title change
- **WHEN** the user saves a new `title` on a document that has comments, `workflow_rules`, and an unknown key
- **THEN** the written file still contains those comments, `workflow_rules`, and unknown key, with the new title

#### Scenario: Parallel unmanaged edit is kept
- **WHEN** another process adds an unmanaged key while Config is open and the user saves a managed title change
- **THEN** the written file contains the TUI title and the new unmanaged key

#### Scenario: Parallel managed edit loses to the TUI draft
- **WHEN** another process changes `title` while Config is open and the user saves a different title
- **THEN** the written file contains the TUI title

#### Scenario: Invalid latest file is not overwritten
- **WHEN** the on-disk file has become syntactically invalid before Save
- **THEN** Save returns an actionable error and the invalid file is left unchanged

### Requirement: Validation SHALL enforce Config semantic gates before any write

`ManagedConfig.Validate` SHALL reject empty `title`, `objective`, and `user_preferred_language`; enabled-stage `max_iterations <= 0` or `timeout_minutes <= 0`; implementation enabled with no scope; Browser UI Validation enabled without frontend; Playwright without frontend; required visible agent/fallback missing harness or model; harness not in the project-enabled list; and a `same_of_agent=false` subagent model that is known not to belong to the parent harness. Missing catalog/capability data and PATH/auth unavailability SHALL NOT be validation failures. Errors SHALL name the field path (PRD-C07-001 §4.6).

#### Scenario: Empty title blocks write
- **WHEN** the draft title is blank
- **THEN** Validate fails with a title path error and Save does not write

#### Scenario: Enabled stage requires positive budgets
- **WHEN** `stages.implementation.enabled` is true and `max_iterations` is 0
- **THEN** Validate fails naming `stages.implementation.max_iterations`

#### Scenario: Playwright without frontend is rejected
- **WHEN** `qa_end_to_end.use_playwright` is true and `scope.frontend` is false
- **THEN** Validate fails and no file is written

#### Scenario: Missing catalog does not fail Validate
- **WHEN** capability metadata for the selected model is unknown
- **THEN** Validate succeeds if harness and model are non-empty and otherwise well-formed

### Requirement: ManagedDiff SHALL report which managed paths changed

The document API SHALL compare two `ManagedConfig` values and return the managed paths that differ so Config can enable stage-specific Retry (PRD-C07-001 §4.8; ADR-052).

#### Scenario: Title-only save does not mark QA
- **WHEN** the only managed change is `title`
- **THEN** `ManagedDiff` does not include paths under `stages.qa` or `agents.qa_agent`

#### Scenario: Failed-stage budget change is visible
- **WHEN** `stages.qa.max_iterations` changes
- **THEN** `ManagedDiff` includes that path
