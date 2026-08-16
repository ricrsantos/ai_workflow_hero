# ADR-C04-001 — Multi-Harness (Cursor + OpenCode)

> Cycle C4 ADRs for Hero **2.0.0**. Index: [ADR.md](ADR.md). Product: [PRD-C04-001](../product/PRD-C04-001-multi-harness.md).

| # | Title | Status |
|---|---|---|
| [ADR-031](#adr-031-multi-harness-is-tui-only-cursor-ide-stays-cursor-only) | Multi-harness is TUI-only; Cursor IDE stays Cursor-only | Accepted |
| [ADR-032](#adr-032-agents-declare-harness--native-model-id-no-hero-canonical-id) | Agents declare harness + native model id; no Hero canonical id | Accepted |
| [ADR-033](#adr-033-fallback-may-use-explicit-fallback-harness-never-a-third) | Fallback may use explicit fallback harness; never a third | Accepted |
| [ADR-034](#adr-034-hero-200-interactive-harness-install-tools-removed) | Hero 2.0.0: interactive harness install; `--tools` removed | Accepted |
| [ADR-035](#adr-035-opencode-via-hero-managed-serve--http-api-project-sqlite-registry) | OpenCode via Hero-managed serve + HTTP API; project SQLite registry | Accepted |
| [ADR-036](#adr-036-single-asset-source-projections-enable-provisions-disable-keeps-files) | Single asset source; projections; enable provisions; disable keeps files | Accepted |
| [ADR-037](#adr-037-hero-harness-and-hero-model-pair-chat-shows-harness) | `/hero-harness` and `/hero-model` pair; Chat shows harness | Accepted |

**Amends:** [ADR-008](ADR.md#adr-008-three-level-model-fallback-chain) (fallback includes `harness`), [ADR-014](ADR-C01-001-hero-1-0.md#adr-014-cli-as-api-no-daemon-in-10) (no Hero daemon; adapter-owned subprocess allowed), [ADR-016](ADR-C01-001-hero-1-0.md#adr-016-harness-adapter-interface-cursor-only-impl) (second impl: OpenCode), [ADR-027](ADR-C03-001-cursor-harness-tui-autonomy.md#adr-027-tui-harness-selection-at-boot) (Cursor + OpenCode), [ADR-030](ADR-C03-001-cursor-harness-tui-autonomy.md#adr-030-harness-default-model-in-herojson-tui-freechat-without-cycle) (freechat default is a pair).

---

## ADR-031: Multi-harness is TUI-only; Cursor IDE stays Cursor-only

**Context**: Users want Cursor and OpenCode in one cycle. Changing Cursor IDE Runtime to dispatch OpenCode would couple Hero to two IDE ecosystems and risk breaking the working slash workflow.

**Decision**: Hero TUI is the **only** multi-harness orchestrator. Cursor IDE remains a **Cursor-only** compatibility path: it ignores `agents.*.harness`, passes `model` through as today, and never starts OpenCode. Do not replace `orchestration_agent` in the IDE with a multi-harness router.

**Consequences**:
- Dual-entry (ADR-015) remains: chat = Cursor; TUI = Cursor and/or OpenCode.
- YAML still carries `harness` for TUI even when the user works in Cursor IDE.
- Cursor Adapter behavior must not regress.

---

## ADR-032: Agents declare harness + native model id; no Hero canonical id

**Context**: A shared Hero model id would need a translation table (`grok-4.6` → `cursor-grok-4.6` vs `xai/grok-4.6`). That drifts, breaks Cursor Task slugs, and was rejected after grilling.

**Decision**:
1. Every agent and `fallback_model` **requires** `harness` and `model`.
2. `model` is the **harness-native** identifier. Hero does **not** translate names.
3. Missing `harness` at runtime → **error** (no default inference).
4. `/hero-new` **injects** missing `harness` from **enabled** harnesses (single enabled → that id; both → `cursor`). Never inject a disabled harness. Explicit imported values are kept.
5. Invalid native id for that harness → unavailable → ADR-033.

**Consequences**:
- Template and docs show Cursor slugs vs OpenCode `provider/model` side by side.
- Cursor IDE keeps working because YAML `model` stays a Cursor slug when `harness: cursor`.
- Pricing lookup keys must include OpenCode native ids ([PRD-C04-001 §4.10](../product/PRD-C04-001-multi-harness.md#410-pricing-catalog--mandatory-implementation-task)).

---

## ADR-033: Fallback may use explicit fallback harness; never a third

**Context**: ADR-008 is model-only. With two harnesses, a down CLI must not silently hop to another product.

**Decision**: Extend the chain: agent pair → `fallback_model` pair (harness may differ **only** because YAML says so) → stop and tell the user to fix, then `/hero-continue`. Warn on every fallback. Never pick a third harness. Disabled/unavailable agent harness still tries fallback (user opted into that YAML).

**Consequences**:
- `fallback_model.harness` is required in templates (injected like agents).
- Orchestrator/TUI messages must name both harness and model on fallback and on hard stop.

---

## ADR-034: Hero 2.0.0: interactive harness install; `--tools` removed

**Context**: `--tools cursor` cannot express multi-harness and contradicts interactive selection in the idea notes. 1.x scripts will break; that is an accepted major.

**Decision**:
- Release as **SemVer 2.0.0**.
- `hero install` presents **supported** harnesses (not PATH-filtered); user must pick **≥1**. OpenCode-only is allowed. Cursor is not mandatory.
- `--tools` on install/upgrade → **error** + suggestion (interactive / `/hero-harness`). No deprecation window. No PATH auto-enable in CI.
- Upgrade 1.x: migrate to **Cursor enabled**; do not enable OpenCode; do not write `.opencode/` until enable.

**Consequences**:
- README, DEPLOY, UI install examples, and workflow-help must drop `--tools`.
- `cli.tools` is superseded by `harnesses.<id>.enabled` (keep a compatibility read during upgrade).

---

## ADR-035: OpenCode via Hero-managed serve + HTTP API; project SQLite registry

**Context**: OpenCode offers `opencode run` and `opencode serve`. Hero 1.x has no daemon (`hero serve` is D7). Users chose serve for execution quality, with strict process ownership.

**Decision**:
1. `OpenCodeAdapter` starts **`opencode serve`** lazily on first OpenCode Execute (localhost, ephemeral port).
2. Chat/stream/cancel/session use that server’s **HTTP API**. Do not spawn `opencode run` per prompt.
3. Attach **only** to the child Hero started. Never attach to an unknown `:4096`.
4. Registry lives in **this project’s `hero.db`**. On `hero tui` start, reap recorded orphans whose PID is still `opencode serve` (unexpected TUI exit only).
5. Normal quit and disabling OpenCode **stop** the child Hero created. Recreate on the next OpenCode Execute.
6. This is **not** `hero serve`. Engine still never talks HTTP to OpenCode — only the adapter.

**Consequences**:
- SQLite schema version bump for serve rows + session harness binding.
- One TUI per project in 2.0 (second TUI would reap the first’s server).
- `IsAvailable` covers OpenCode CLI presence when an Execute needs serve; enabled ≠ available.

---

## ADR-036: Single asset source; projections; enable provisions; disable keeps files

**Context**: Each harness wants its own on-disk layout. Duplicating agent markdown would fork the Runtime.

**Decision**: `assets/` remains the source of truth. Enabling a harness **provisions** its projection immediately (Cursor → `.cursor/…` as today; OpenCode → `.opencode/agents|commands|skills`, plus `opencode.json` only if required). Disabling sets `enabled: false` and **does not delete** files (preserves customizations; re-enable is cheap). Last enabled harness cannot be disabled. Root `AGENTS.md` is not copied into `.opencode/`. Checksum non-overwrite applies to both projections.

**Consequences**:
- Install/upgrade/uninstal paths must know OpenCode-owned files without deleting user `AGENTS.md` / `docs/`.
- Projection format adapters live next to each harness adapter, not in `engine`.

---

## ADR-037: `/hero-harness` and `/hero-model` pair; Chat shows harness

**Context**: ADR-030 stores `harnesses.<tool>.model` for freechat. With two harnesses, a model name alone is ambiguous. Users also need enable/disable after install.

**Decision**:
1. `/hero-harness` enables/disables supported harnesses (provision on enable; last-one guard).
2. `/hero-model` picks a **pair** `(harness, model)` as default for **freechat and `/hero-new`**, and writes `harnesses.<harness>.model`. It does not edit cycle agent YAML.
3. Chat green pane / speaker line shows **agent, model, and harness**.

**Consequences**:
- Palette gains `/hero-harness`. `/hero-status` may display harness state but does not replace `/hero-harness`.
- ADR-030 freechat remains; the default is a pair, not a bare slug.

---

## Amendment notes

- **ADR-008**: Fallback payload includes `harness`; still three levels (configured pair → fallback pair → human `/hero-continue`).
- **ADR-014**: Still no Hero daemon/RPC. Adapter-managed `opencode serve` is a harness subprocess.
- **ADR-016**: Interface stays; implementations are Cursor **and** OpenCode. Cursor-only IDE path preserved.
- **ADR-027**: Supported list at boot/install is Cursor + OpenCode; PATH does not hide unsupported-but-compatible entries at install time.
- **ADR-030**: Freechat/`/hero-new` model comes from the stored pair; stage Execute uses YAML `harness` + native `model`.
