# cycle-screenshots Specification (new)

## Purpose
Optional per-stage automatic captures with deterministic retrieval. PRD-C17-001 FR-12–FR-13; ADR-106; UI §§4–5. Narrowly amends C14 text-only Telegram scope for generated cycle screenshots.

## ADDED Requirements

### Requirement: Captures SHALL be opt-in per stage with safe handling
Browser UI and browser-based E2E SHALL each expose `screenshots.enabled` (default false) in their Config stage section; HTTP-only E2E SHALL NOT enable browser screenshots. Each tested screen SHALL capture after its assertion at a stable point into immutable ready assets under `current/screenshots`, correlated with coverage/user/attempt; attempts/roles SHALL never overwrite. Capture, traces, video, snapshots, and raw login responses SHALL be suspended during credential fill/submit; sensitive-field/token masks SHALL apply; omission SHALL record an actionable reason. Missing mandatory safe evidence SHALL block its item; optional delivery failure SHALL only warn. Existing text image cards (Enter/o viewer, save/copy-path) SHALL be reused; no terminal inline image protocol SHALL be added.

#### Scenario: Login secret suppression
- **WHEN** a tested screen cannot be captured without exposing credentials or tokens
- **THEN** no capture is stored and the manifest records the omission reason

### Requirement: Retrieval SHALL be asynchronous and read-only during streaming
`/hero-screenshot` SHALL accept no argument (latest ready), `list` (IDs with screen/profile/stage/attempt, no secrets), `<id>` (selected), and `todos` (all ready in one request; `list`/`todos` reserved). It SHALL run during active agents without invoking a harness, starting captures, interrupting tests, or waiting for agents. `todos` SHALL snapshot the ready set and deliver bounded async batches with progress and explicit failed IDs. Missing cycle/captures/ID SHALL produce precise explanations. A dedicated collection shortcut SHALL work while agents run; Planning SHALL choose a collision-free key and list it in `/help` and the footer.

#### Scenario: All-images request while streaming
- **WHEN** `/hero-screenshot todos` runs during an active Browser UI wave with 6 ready captures
- **THEN** all 6 cards populate from the request-time snapshot with no harness turn and no silent omission

### Requirement: Telegram SHALL receive actual images only under always_send
Automatically captured and retrieved screenshots SHALL reach the paired addressed chat as actual images only when project `always_send` is enabled; otherwise a remote request SHALL explain TUI availability and the setting. Selected-project addressing SHALL remain mandatory. Queue jobs SHALL carry validated managed references and safe captions only. Daemon mismatch, disconnect, or partial-batch failure SHALL warn while local cards remain; optional delivery failure SHALL NOT fail validation. Captions SHALL contain no credential values, tokens, or raw authentication responses.

#### Scenario: always_send disabled
- **WHEN** Telegram requests `/hero-screenshot list` with Always send reply off
- **THEN** text metadata returns locally-styled and explains that images stay in the TUI until the setting is enabled
