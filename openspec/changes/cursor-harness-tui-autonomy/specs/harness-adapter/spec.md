# harness-adapter Specification

## MODIFIED Requirements

### Requirement: Dispatch SHALL accept expanded command markdown as prompt
`HarnessAdapter.Dispatch` SHALL accept a `Prompt` that may be the full body of a Cursor custom command markdown file (after optional frontmatter strip). When the Cursor Agent CLI is available and authenticated, the adapter SHALL execute via CLI and set `Dispatched` true with result output — not only return chat fallback (ADR-025 amends ADR-016/021).

#### Scenario: Dispatch with markdown body succeeds via CLI
- **WHEN** a caller dispatches with `Prompt` set to a command markdown body, `ProjectDir` set, and CLI is available
- **THEN** the adapter executes through the Cursor CLI and returns `Dispatched` true when execution succeeds

#### Scenario: Unavailable dispatch returns actionable message
- **WHEN** dispatch cannot run because CLI is missing or not authenticated
- **THEN** `Dispatched` is false and `Message` includes remediation (e.g. `cursor agent login` or install guidance)

### Requirement: HarnessAdapter SHALL expose full execution contract
The harness package SHALL define `IsAvailable`, `CreateSession`, `ResumeSession`, `Execute`, `Cancel`, and `Status` on `HarnessAdapter` (design D2; ADR-025).

#### Scenario: Execute is callable
- **WHEN** a caller invokes `Execute` on the Cursor adapter with a valid prompt
- **THEN** the adapter returns an `ExecutionResult` with output text
