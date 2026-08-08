## ADDED Requirements

### Requirement: Dispatch SHALL accept expanded command markdown as prompt
`HarnessAdapter.Dispatch` SHALL accept a `Prompt` that may be the full body of a Cursor custom command markdown file (after optional frontmatter strip). Cursor adapter behavior remains best-effort; IDE chat injection remains out of scope (ADR-016 as amended by ADR-021; PRD-C02-001 §5.2).

#### Scenario: Dispatch with markdown body
- **WHEN** a caller dispatches with `Prompt` set to a command markdown body and `ProjectDir` set
- **THEN** the adapter uses that prompt text for the dispatch attempt (or returns a non-silent unavailable result)

#### Scenario: Unavailable dispatch returns actionable message
- **WHEN** dispatch cannot run
- **THEN** `Dispatched` is false and `Message` tells the user to run the command in Cursor chat
