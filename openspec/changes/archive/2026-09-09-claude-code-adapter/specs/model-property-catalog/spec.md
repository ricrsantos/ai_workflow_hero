## ADDED Requirements

### Requirement: Catalog resolution SHALL preserve Claude-native property and price semantics

The shared catalog resolver SHALL resolve Claude aliases, full ids, and allowed context suffixes without translating them into another harness. It SHALL expose `ef` only when the Claude catalog marks it available, expose `th` and `fs` as unavailable `na`, and preserve unknown-price warnings through existing metrics/context-bar flows.

#### Scenario: Claude catalog lookup
- **WHEN** the resolver receives `claude` and a cataloged native id
- **THEN** it returns the Claude context window and supported effort values while leaving unsupported properties unavailable

#### Scenario: Unknown Claude id
- **WHEN** the resolver receives an uncataloged Claude id
- **THEN** it returns a non-panicking unknown snapshot, retains the native id, and emits the existing missing-catalog or missing-price warning

