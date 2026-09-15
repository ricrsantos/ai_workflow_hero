# unicode-mosaic-preview Specification (discontinued)

## Purpose
TBD - created by archiving change tui-multimodal-images. Update Purpose after archive.

## Status

This archived design is retained for traceability only. The current TUI does not decode or render image previews; asset cards and composer chips use textual metadata plus external open/copy/save/remove actions.

## Requirements

### Requirement: Unicode mosaic preview SHALL remain discontinued

The current TUI MUST NOT decode image files or render Unicode mosaic previews. Textual asset-card and composer-chip actions remain the supported image surface.

#### Scenario: Asset card stays text-only
- **WHEN** an image asset card is shown
- **THEN** it exposes open/copy/attach/save actions without a preview region
