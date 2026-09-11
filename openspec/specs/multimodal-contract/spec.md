# multimodal-contract Specification

## Purpose
TBD - created by archiving change tui-multimodal-images. Update Purpose after archive.

## Requirements

### Requirement: Shared multimodal types SHALL live on the harness boundary

Hero SHALL define `Attachment`, `Asset`, `MediaKind`, and `MediaCapability` in `internal/harness` for use by the TUI, conversation layer, and adapters. Adapters SHALL translate to native protocol internally; the TUI SHALL NOT build provider-specific payloads (PRD-C14-001 §2.1; ADR-077).

#### Scenario: Attachment fields are complete
- **WHEN** a validated local image is represented as an `Attachment`
- **THEN** it carries hero-generated `ID`, `Kind`, original `Name` (metadata only), `MIMEType`, materialized `Path`, `Size`, `Width`, and `Height`

#### Scenario: Asset fields are complete
- **WHEN** a turn produces or receives an image asset
- **THEN** the `Asset` embeds `Attachment` metadata and includes `Source` (`user|model|tool`), `SessionID`, `TurnID`, and `Saved`

### Requirement: Conversation and execute contracts SHALL carry attachments and assets

`conversation.Input` and `harness.ExecuteRequest` SHALL accept `Attachments []Attachment`. `harness.ExecutionResult` SHALL accept `Assets []Asset`. `harness.StreamDelta` SHALL support `Asset *Asset` with `StreamKindAsset`. Final result assets SHALL repair partial stream loss using the existing text-repair pattern (PRD-C14-001 §2.1).

#### Scenario: Image-only input is valid
- **WHEN** Free Chat submits attachments with empty text
- **THEN** the conversation input remains valid and Execute receives the attachments

#### Scenario: Stream asset loss is repaired
- **WHEN** a stream omits an asset delta that later appears in `ExecutionResult.Assets`
- **THEN** the consumer surfaces the asset from the final result without duplicating identical content hashes

### Requirement: Part order SHALL be images then text

When a turn includes both text and attachments, adapters SHALL receive images first, then text, matching Free Chat send semantics (PRD-C14-001 §2.5).

#### Scenario: Mixed turn ordering
- **WHEN** a user sends two images and a caption
- **THEN** the normalized execute payload presents the two attachments before the text part
