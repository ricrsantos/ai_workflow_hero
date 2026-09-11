# advanced-inline-preview Specification

## Purpose
TBD - created by archiving change tui-multimodal-images. Update Purpose after archive.

## Requirements

### Requirement: Advanced inline image protocols SHALL be opt-in and off by default

Kitty Graphics Protocol, Sixel, and iTerm2 OSC 1337 inline rendering MAY be implemented, but SHALL remain disabled by default and require explicit user or configuration opt-in. Detection is best-effort; a config flag MAY force enable/disable. The text asset card SHALL remain mandatory and navigable when inline rendering is active (PRD-C14-001 §2.8; ADR-081; UI-C14-001 §3.3).

#### Scenario: Default path uses mosaic only
- **WHEN** Free Chat shows an asset card on a fresh install
- **THEN** preview uses Unicode mosaic (or card-only degradation) and does not emit Kitty/Sixel/iTerm2 sequences

#### Scenario: Opt-in inline still keeps card actions
- **WHEN** an advanced protocol is enabled and detected
- **THEN** preview may render pixels while card actions remain keyboard-navigable

#### Scenario: Scroll or resize clears inline pixels
- **WHEN** an inline protocol image is visible and the user scrolls or the terminal resizes
- **THEN** the inline image is cleared and the card returns to mosaic or card-only state
