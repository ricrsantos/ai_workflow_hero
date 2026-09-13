## ADDED Requirements

### Requirement: Navbar SHALL insert History immediately after Chat
Project TUI navigation SHALL be `Chat | History | Status | Artifacts | Costs | Events | Settings` and SHALL append `Config` only when a cycle is active. Standalone Free Chat SHALL show `Chat | History | Settings`. Visible numeric shortcuts SHALL match visible order: `alt+1`–`alt+7` without a cycle, `alt+1`–`alt+8` with a cycle, and `alt+1`–`alt+3` in standalone Free Chat. History SHALL be available without an active cycle. Existing navbar focus rules (`Esc`, Up/Down, Enter, Tab/Shift+Tab) SHALL remain (UI-C16-001 §1; PRD-C16-001 §3.4).

#### Scenario: History is visible without a cycle
- **WHEN** the project TUI opens with no active cycle
- **THEN** History is the second navbar item and `alt+2` activates it

#### Scenario: Active cycle renumbers Config
- **WHEN** a cycle is active
- **THEN** Config is the eighth visible item and `alt+8` activates it

#### Scenario: Standalone includes History
- **WHEN** `hero chat` is running
- **THEN** the navbar is Chat, History, Settings with `alt+1`–`alt+3`

### Requirement: History screen SHALL follow the documented list/detail and keymap
History SHALL use the list as primary focus, update the detail panel without I/O in `View`, sort Active by `last_activity_at DESC` then id, search case-insensitively by name within the selected Active/Archived view, and use Left/Right to switch views. Enter SHALL open the selected session in Chat at the newest transcript position; Enter on Archived SHALL restore then open. `r` SHALL rename, `a` SHALL archive or restore, and `d` SHALL open permanent-delete confirmation. In History content focus `/` SHALL start search rather than the command palette, and the footer SHALL make that exception visible. Modal dialogs SHALL own keyboard focus. All database, filesystem, adapter, import/delete, and paging work SHALL run through `tea.Cmd`; `Update` and `View` MUST NOT block (UI-C16-001 §§2,9; PRD-C16-001 FR-14; ADR-093).

#### Scenario: Search slash does not open the palette
- **WHEN** History content is focused and the user presses `/`
- **THEN** name search activates and the command palette does not open

#### Scenario: Archived enter restores and opens
- **WHEN** the user presses Enter on an archived row
- **THEN** the session is active and Chat opens at the newest transcript position

#### Scenario: Detail update is local
- **WHEN** the user moves the list selection
- **THEN** the detail panel updates from already-loaded row data without performing I/O in `View`

### Requirement: History SHALL remain usable across terminal sizes and documented copy states
Width and height SHALL come from `tea.WindowSizeMsg`. Wide layout SHALL show list and detail; medium SHALL stack detail below the list; narrow or short SHALL show the list alone with a details state from which Esc returns. Truncation SHALL be ANSI/rune-aware and MUST NOT split graphemes; harness/model and timestamps SHALL hide before the session name. Empty, busy, legacy, interrupted, fork, import, loading, and error copy SHALL match UI-C16-001 §4. If no meaningful layout fits, use the existing centered `window too small` behavior (UI-C16-001 §§3–4; PRD-C16-001 §5).

#### Scenario: Narrow terminal keeps actions
- **WHEN** History is resized to the narrow layout
- **THEN** list navigation and documented actions remain reachable via the details state

#### Scenario: Empty active copy is documented
- **WHEN** there are no active sessions
- **THEN** History shows `No saved sessions yet.` and `→ Send a message in Chat to create one.`

### Requirement: Chat SHALL restore durable transcripts and honor /new-chat
Chat SHALL restore all locally persisted visible events with original actor styling, agent labels, Telegram origin, attachment cards, and interruption markers. The header SHALL show the saved session name plus existing cycle/freechat and harness information. Scroll SHALL default to newest content. Context occupancy SHALL represent the selected session. `/new-chat` SHALL leave the prior session in History and open an unpersisted blank surface. Resuming an archived session SHALL restore it automatically. Archiving or deleting the currently open idle session SHALL open a new empty Chat surface after persistence succeeds (PRD-C16-001 §3.3, §3.7, §4 FR-13; UI-C16-001 §8).

#### Scenario: Restart restores Free Chat
- **WHEN** the user exchanges text and image turns, exits the TUI, and reopens the session from History
- **THEN** title, visible transcript, attachment cards, model metadata, and native session ID are restored

#### Scenario: /new-chat keeps History
- **WHEN** the user runs `/new-chat` on a persisted session
- **THEN** the previous session remains listed and Chat shows an empty unpersisted surface

## MODIFIED Requirements

### Requirement: Status screen SHALL project the active findings flow
The existing Status screen SHALL keep the stage table first and add compact sections for loop-backs, findings, deferred ToDos, and disposition/CTAs. Owner labels remain BACK/FRNT/GEN. `deferred_todo` renders as `ToDo` in the compact table. Empty cycles MAY show `Findings none` and MUST NOT render noisy empty tables. Findings MUST NOT receive a dedicated navbar item; History is the C16 navbar addition after Chat, so an active cycle MAY show eight visible items with Config last (UI-C15-001 §6; ADR-090; UI-C16-001 §1).

#### Scenario: Escalated Status lists CTAs
- **WHEN** Implementation is Escalated with open findings
- **THEN** Status shows `/hero-continue`, `/hero-add-todo`, `/hero-cancel`, and `/hero-finish`

#### Scenario: Deferred completion copy is explicit
- **WHEN** a cycle closes with `completed_with_deferred_todos`
- **THEN** Status/Chat states the disposition, deferred IDs, and that remaining validation stages were skipped

#### Scenario: Findings stay off the navbar
- **WHEN** an active cycle is open
- **THEN** the navbar includes History after Chat and does not add a Findings item

### Requirement: Transcript SHALL render asset cards with keyboard actions
Model/tool image assets SHALL render as transcript cards with metadata and actions: Enter preview, `o` open in system viewer (xdg-open/open with TUI suspend/resume), `c` copy path, `a` attach to next composer turn, `s` save via inline path dialog. All I/O SHALL be `tea.Cmd`. For sessions registered in durable History, cards SHALL be reconstructed from persisted `session_events` and `session_assets` after TUI restart without loading image bytes into the Bubble Tea model. Cards MUST NOT be invented when `transcript_state` is `unavailable_legacy` (PRD-C14-001 §§2.6,2.14; PRD-C16-001 §3.3; UI-C14-001 §§3,5–6; UI-C16-001 §8; ADR-078; ADR-096).

#### Scenario: Card actions remain keyboard-only
- **WHEN** an asset card is focused
- **THEN** Enter/o/c/a/s perform preview/open/copy/attach/save without requiring a mouse

#### Scenario: Open suspends the TUI
- **WHEN** the user presses `o` on a card
- **THEN** Hero launches the system viewer and restores the TUI afterward

#### Scenario: Restart does not rebuild cards
- **WHEN** the TUI restarts with only a retained C14 session manifest on disk and no matching durable History row
- **THEN** asset cards are not auto-inserted into the transcript

#### Scenario: Restart rebuilds durable cards
- **WHEN** the TUI restarts a session with locally persisted asset events
- **THEN** the restored transcript shows those asset cards from stored metadata without reading image bytes into the Bubble Tea model

#### Scenario: Legacy missing transcript has no invented cards
- **WHEN** a migrated entry is labeled `Local transcript unavailable`
- **THEN** no asset cards are synthesized
