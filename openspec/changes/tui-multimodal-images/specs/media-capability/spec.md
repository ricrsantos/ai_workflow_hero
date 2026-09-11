## ADDED Requirements

### Requirement: Media capabilities SHALL combine adapter transport and model sources

Hero SHALL expose per harness/model capability flags at minimum: `image_input_native`, `image_input_file_reference`, `image_output_native`, `image_output_file`, `supported_image_mime_types`, and `max_attachment_bytes`. Adapter transport and native model discovery/catalog SHALL be independent sources (PRD-C14-001 §2.4).

#### Scenario: Unknown means unsupported
- **WHEN** capability information is absent for the active harness/model
- **THEN** Hero treats image input as unsupported rather than assuming support

#### Scenario: Intersection is required
- **WHEN** the adapter can transport images but the selected model does not advertise input support
- **THEN** image input remains unsupported for that turn

### Requirement: Unsupported image turns SHALL fail closed

Before submitting a turn with attachments, Hero SHALL require `image_input_native` or `image_input_file_reference`. Otherwise the turn is blocked with an actionable error naming harness and model; chips remain; attachments are never silently removed; fallback is not invoked silently (PRD-C14-001 §2.4; ADR-079; UI-C14-001 §2.4).

#### Scenario: Blocked turn keeps chips
- **WHEN** the user submits attachments on an incapable model
- **THEN** Free Chat shows an inline error identifying model and harness, does not call Execute, and leaves chips in the composer

#### Scenario: Switching to a capable model unblocks
- **WHEN** the user switches to a model with image input capability and resubmits
- **THEN** the turn proceeds without requiring re-attachment
