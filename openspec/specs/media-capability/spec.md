# media-capability Specification

## Purpose
TBD - created by archiving change tui-multimodal-images. Update Purpose after archive.

## Requirements

### Requirement: Media capabilities SHALL combine adapter transport and model sources

Hero SHALL expose per harness/model capability flags at minimum: `image_input_native`, `image_input_file_reference`, `image_output_native`, `image_output_file`, `supported_image_mime_types`, and `max_attachment_bytes`. Adapter transport and native model discovery/catalog SHALL be independent sources (PRD-C14-001 §2.4). Missing model metadata SHALL remain distinguishable from an explicit unsupported capability.

#### Scenario: Unknown model metadata may use optimistic transport admission
- **WHEN** the harness transport is known to carry image input but model capability metadata is absent or stale
- **THEN** an attachment-bearing Execute may be attempted once with the known transport capability, leaving the provider as the final compatibility authority

#### Scenario: Unknown transport remains unsupported
- **WHEN** transport capability information is absent for the active harness
- **THEN** Hero blocks image input because it cannot safely construct a provider request

#### Scenario: Intersection is required
- **WHEN** the adapter can transport images but the selected model does not advertise input support
- **THEN** image input remains unsupported for that turn

### Requirement: Unsupported image turns SHALL fail closed

Before submitting a turn with attachments, Hero SHALL block when the known harness/model intersection lacks `image_input_native` and `image_input_file_reference`, or when the harness transport itself is unknown. If only the model-side fact is unknown while transport is usable, Hero SHALL pass the unchanged attachments to Execute optimistically. Provider rejection SHALL be surfaced as an actionable error naming harness/model when possible; chips remain; fallback and automatic retry are never invoked silently (PRD-C14-001 §2.4; ADR-079; UI-C14-001 §2.4).

#### Scenario: Blocked turn keeps chips
- **WHEN** the user submits attachments on an incapable model
- **THEN** Chat shows an inline error identifying model and harness, does not call Execute, and leaves chips in the composer

#### Scenario: Unknown model reaches the provider
- **WHEN** the selected model has no reliable modality metadata but its harness transport is image-capable
- **THEN** Chat calls one logical Execute with the original attachments and does not invoke TUI fallback or retry automatically

#### Scenario: Provider rejection keeps the turn recoverable
- **WHEN** an optimistically admitted provider rejects the image input
- **THEN** Chat shows the provider error and leaves the attachment chips available for remove, model switch, or explicit resend

#### Scenario: Switching to a capable model unblocks
- **WHEN** the user switches to a model with image input capability and resubmits
- **THEN** the turn proceeds without requiring re-attachment
