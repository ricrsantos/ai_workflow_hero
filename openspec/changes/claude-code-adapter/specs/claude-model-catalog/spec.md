## Purpose

Offer fast, offline Claude-native model selection and truthful C5/cost metadata while allowing Claude Code runtime initialization to report the account's effective model and capabilities.

## ADDED Requirements

### Requirement: The Claude catalog SHALL use native identifiers and dated metadata

The embedded and installed Claude catalog SHALL include the stable aliases `sonnet`, `opus`, `haiku`, and `fable`, permitted full ids and context suffixes such as `[1m]` when supported, dated official metadata, context windows, and C5 property declarations. It SHALL preserve project-local catalog overlays and SHALL not require a live model request merely to populate `/hero-model` (PRD-C13-001 §2; ADR-074).

#### Scenario: Native alias picker
- **WHEN** the user selects Claude in `/hero-model`
- **THEN** the picker shows native aliases and permitted ids without translating them into Cursor or OpenCode names

#### Scenario: Project overlay
- **WHEN** a project has a local Claude catalog overlay
- **THEN** the overlay is preserved and takes part in the same lookup path as other harness catalogs

#### Scenario: Account policy substitution
- **WHEN** Claude Code resolves a selected alias to a different effective model during `system/init`
- **THEN** the TUI reports the effective native model and treats runtime capabilities as authoritative without rewriting the selected id as a cross-provider id

### Requirement: Claude C5 properties SHALL map only to documented native controls

The Claude adapter SHALL expose `ef` only for values marked supported by the catalog and map it to native `--effort`. `th` and `fs` SHALL remain unavailable as `na` in the first version. Values rejected by Claude policy or runtime SHALL produce an explicit warning/error and SHALL not be silently stripped or retried (PRD-C13-001 §2; ADR-074).

#### Scenario: Supported effort
- **WHEN** a catalog-supported Claude model is executed with `ef=high`
- **THEN** the adapter passes the native effort value and the TUI displays the configured effort

#### Scenario: Unsupported properties
- **WHEN** the user views `th` or `fs` for Claude
- **THEN** both properties are visible as unavailable `na` and are not sent as invented headless flags

#### Scenario: Runtime rejects effort
- **WHEN** Claude rejects a selected effort because of version or organization policy
- **THEN** Hero shows an explicit property-aware warning and does not retry after removing the effort

### Requirement: Claude pricing SHALL never be fabricated

Catalog, metrics, context-bar, and update-model flows SHALL preserve official dated pricing when available and leave cost unset or zero with a visible warning for subscription plans, gateways, or ids without a public tariff. Unknown model ids SHALL not panic or block execution (PRD-C13-001 §2; ADR-074).

#### Scenario: Known priced model
- **WHEN** usage is recorded for a Claude id with official catalog rates
- **THEN** metrics and the context bar resolve those rates using the existing cost path

#### Scenario: Unknown or subscription model
- **WHEN** usage is recorded for a Claude id with no public rate
- **THEN** execution continues, cost remains unset or zero, and Hero reports that pricing is unavailable rather than inventing a value

