# unicode-mosaic-preview Specification

## Purpose
TBD - created by archiving change tui-multimodal-images. Update Purpose after archive.

## Requirements

### Requirement: Unicode mosaic preview SHALL be mandatory and async

Every image asset card SHALL offer a Unicode block-character + ANSI-color mosaic preview computed asynchronously as a `tea.Cmd`, never inside `View()`. Mosaic SHALL respect pane width/height, re-render on resize without blocking, and degrade to card-only guidance when terminal color depth is below 256 colors (PRD-C14-001 §2.7; ADR-081; UI-C14-001 §3.2).

#### Scenario: Enter toggles mosaic
- **WHEN** the user focuses an asset card and presses Enter
- **THEN** a mosaic region expands below the card without freezing input, and Enter again collapses it

#### Scenario: Low-color terminal degrades
- **WHEN** preview is requested on a terminal without 256+ colors
- **THEN** the UI states preview is unavailable and points the user to open-in-viewer

#### Scenario: Tests avoid huge ANSI snapshots
- **WHEN** mosaic tests run
- **THEN** they use golden invariants or bounded fixtures rather than large ANSI sequence snapshots
