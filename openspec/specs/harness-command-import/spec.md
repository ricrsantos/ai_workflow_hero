# harness-command-import Specification

## Purpose
TBD - created by archiving change slash-parity-tui-harness. Update Purpose after archive.
## Requirements
### Requirement: TUI SHALL discover non-Hero Cursor command markdown
The system SHALL scan `<project>/.cursor/commands/*.md` and `~/.cursor/commands/*.md` for custom commands, present each as `/<stem>` (filename without `.md`), and MUST exclude Hero-owned files matching `hero-*.md` from the imported list (PRD-C02-001 §5.2; ADR-021; UI-C02-001 §3).

#### Scenario: Project and user commands listed
- **WHEN** the project has `.cursor/commands/opsx-propose.md` and the user home has `.cursor/commands/my-tool.md`
- **THEN** the TUI imported-commands section includes `/opsx-propose` (project) and `/my-tool` (user) with source hints

#### Scenario: Hero-owned commands excluded from import list
- **WHEN** `.cursor/commands` contains `hero-approve.md` and a non-Hero `foo.md`
- **THEN** only `/foo` appears in the imported list; Hero actions remain the dedicated `/hero:*` palette entries

### Requirement: TUI SHALL execute imported commands via markdown prompt expansion
On select, the system SHALL read the command `.md` file, strip leading YAML frontmatter when present, and dispatch the remaining body as `HarnessAdapter.Dispatch` prompt with `ProjectDir` set to the Hero project root. The system MUST NOT rely on injecting a literal `/name` string into an open IDE chat (PRD-C02-001 §5.2; ADR-021).

#### Scenario: Successful markdown expansion dispatch
- **WHEN** the user selects an imported command and the Cursor adapter can dispatch
- **THEN** the adapter receives the markdown body as `Prompt` and the TUI shows a brief running progress line then the adapter result

#### Scenario: Dispatch unavailable
- **WHEN** the user selects an imported command and dispatch is unavailable
- **THEN** the TUI shows a clear failure advising the user to run the same command in Cursor chat and MUST NOT silently no-op

### Requirement: TUI MUST NOT list skills in the command palette
The TUI MUST NOT enumerate `.cursor/skills` (or user skills) as palette entries. Skill loading remains the harness responsibility when the agent runs with project cwd (PRD-C02-001 §3; ADR-021).

#### Scenario: Skills directory present
- **WHEN** the project contains `.cursor/skills/**`
- **THEN** those skills do not appear as TUI palette items

