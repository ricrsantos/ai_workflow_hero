# advanced-inline-preview Specification (discontinued)

## Purpose
TBD - created by archiving change tui-multimodal-images. Update Purpose after archive.

## Status

This archived design is retained for traceability only. Kitty, Sixel, iTerm2, and other terminal image protocols are not implemented, detected, or enabled by the current TUI.

## Requirements

### Requirement: Advanced inline image protocols SHALL remain discontinued

The current TUI MUST NOT detect, enable, or emit Kitty, Sixel, iTerm2, or other terminal image-preview protocols.

#### Scenario: Terminal capability does not activate preview
- **WHEN** Chat displays an image card on any terminal
- **THEN** the card remains textual and uses the external viewer action
