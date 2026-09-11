## ADDED Requirements

### Requirement: Free Chat composer SHALL support image attachments only

Image attachment UX SHALL be available in Free Chat and SHALL NOT appear in Research or workflow-stage sessions. Bindings: `Alt+A`/`/attach` open a filtered file picker; `Alt+V`/`/attach-clipboard` capture via native OS clipboard APIs (not OSC 52); `/attach <path>` attaches an explicit path; bracketed paste of a filesystem path offers attachment. Planning validates `Alt+A`/`Alt+V` as free relative to existing Alt bindings (PRD-C14-001 §2.5; UI-C14-001 §§2,4,7).

#### Scenario: Alt+A opens picker in Free Chat
- **WHEN** Free Chat is focused and the user presses Alt+A
- **THEN** a Bubbles file picker filtered to png/jpeg/gif/webp opens and Escape returns focus to the text input

#### Scenario: Research ignores attach bindings
- **WHEN** a Research or workflow-stage session is active
- **THEN** attach keybindings/slash commands do not enable multimodal send for that session

#### Scenario: Clipboard capture uses native APIs
- **WHEN** the user invokes Alt+V and the OS clipboard contains a PNG
- **THEN** Hero reads image bytes through a native clipboard path and shows a spinner chip until validation completes

### Requirement: Attachment chips SHALL sit outside the text field

Validated attachments SHALL render as chips below the text input and above the status line with name, MIME, dimensions, size, and dismiss control. The text cursor SHALL NOT traverse chips; Backspace in text SHALL NOT remove chips; removal requires chip focus + x/Delete. External paths show a warning badge. Validation failures render error chips that cannot be sent. Image-only turns are allowed. Send order is images then text (UI-C14-001 §2; ADR-078).

#### Scenario: Chip independent of backspace
- **WHEN** chips exist and the user presses Backspace in the text field
- **THEN** only text changes and chips remain until explicitly dismissed

#### Scenario: Image-only send
- **WHEN** chips exist and the text field is empty
- **THEN** Enter/send submits an image-only turn

#### Scenario: Validation error chip blocks send
- **WHEN** a selected file fails validation
- **THEN** an error chip replaces the pending chip and submit cannot include that file

### Requirement: Transcript SHALL render asset cards with keyboard actions

Model/tool image assets SHALL render as transcript cards with metadata and actions: Enter preview, `o` open in system viewer (xdg-open/open with TUI suspend/resume), `c` copy path, `a` attach to next composer turn, `s` save via inline path dialog. All I/O SHALL be `tea.Cmd`. Cards are session-ephemeral and are not reconstructed after TUI restart (PRD-C14-001 §§2.6,2.14; UI-C14-001 §§3,5–6; ADR-078/080).

#### Scenario: Card actions remain keyboard-only
- **WHEN** an asset card is focused
- **THEN** Enter/o/c/a/s perform preview/open/copy/attach/save without requiring a mouse

#### Scenario: Open suspends the TUI
- **WHEN** the user presses `o` on a card
- **THEN** Hero launches the system viewer and restores the TUI afterward

#### Scenario: Restart does not rebuild cards
- **WHEN** the TUI restarts with a retained session manifest on disk
- **THEN** asset cards are not auto-inserted into the transcript

### Requirement: Capability and progress states SHALL be visible and non-blocking

Unsupported-model submit errors, validation/clipboard/mosaic/save/open progress indicators, and footer hints for Alt+A/Alt+V SHALL follow UI-C14-001. Indicators MUST NOT block keyboard handling (UI-C14-001 §§2.4,6–7).

#### Scenario: Capability error copy
- **WHEN** submit is blocked for an incapable model
- **THEN** the inline error names the model and harness and tells the user to remove attachments or switch models

#### Scenario: Mosaic spinner is async
- **WHEN** mosaic rendering is in progress
- **THEN** the card shows a spinner and the TUI continues to accept key events
