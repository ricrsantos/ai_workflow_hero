# interactive-harness-install Specification

## MODIFIED Requirements

### Requirement: Install picker SHALL offer three supported harnesses
Interactive install SHALL list Cursor, OpenCode, and Codex as supported harness checkboxes independent of PATH availability (UI-C06-001 §2; PRD-C06-001 §4.6).

#### Scenario: Codex appears in install list
- **WHEN** the user runs `hero install` interactively
- **THEN** the harness multi-select includes Codex even if `codex` is not on PATH

#### Scenario: Zero selection still invalid
- **WHEN** the user clears all harness checkboxes
- **THEN** install cannot continue with the same validation as C4

### Requirement: Successful install SHALL report enabled harness names including Codex
The install success line SHALL name every harness the user enabled, including Codex-only selections (UI-C06-001 §2).

#### Scenario: Success lists Codex
- **WHEN** install completes with Cursor and Codex enabled
- **THEN** the success message names both harnesses with ✓ formatting per UI.md
