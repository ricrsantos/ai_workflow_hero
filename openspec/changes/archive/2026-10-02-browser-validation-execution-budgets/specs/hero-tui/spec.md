# hero-tui Specification (delta)

## Purpose
Test users setup, blocked/budget/progress rendering, screenshot collection, and `/hero-screenshot`. UI-C17-001 §§1–5; PRD FR-02, FR-08–FR-13.

## MODIFIED Requirements

### Requirement: Config SHALL expose Test users and per-stage screenshot toggles
For browser-enabled frontend cycles, authentication setup SHALL be prioritized after `/hero-new` with states Not needed, Setup pending, Users configured, or Access verified; unresolved authentication SHALL prevent test-plan approval. Config Test users SHALL offer the opt-in toggle plus scrollable Add/Edit/Remove (stable ID, profile, login, masked password; no reveal/copy/export) with Save/Discard/Cancel, atomic updates, unrelated-entry preservation, external-change Reload gate, and read-only behavior under execution guards. Disabled setup with a required login test SHALL block with enable instructions. Browser UI and browser E2E sections SHALL each show Screenshots Off/On (default Off), reusing the Telegram Always-send setting for forwarding.

#### Scenario: External edit conflicts with Config
- **WHEN** `.env.hero` changes after the Config draft loads
- **THEN** Save refuses, preserves the newer file, and directs Reload without overwriting passwords

### Requirement: Blocked, budget, and progress SHALL render durably
Blocked stages SHALL show stage, reason, affected coverage/profile IDs, corrective steps, remaining budget, and `/hero-continue` (never merely "blocked"; never passwords or raw login responses). The TUI SHALL show active elapsed/remaining, preparation phase, current coverage/profile IDs, and completed/required counts. Restart SHALL show Interrupted with consumed/remaining and `/hero-continue`. Expiry SHALL report timeout with preserved evidence and intervention state. Human approval SHALL use `/hero-approve`, `/hero-reject`, `/hero-cancel`, or `/hero-finish`; no agent SHALL advance stages on its own.

#### Scenario: Expired budget blocks retry
- **WHEN** continuation is requested with zero remaining budget
- **THEN** the TUI explains the explicit limit increase required before retry

### Requirement: Screenshot collection SHALL stay usable while agents run
Ready captures SHALL appear as cards (ID, coverage, stage, attempt, user/profile, timestamp, result) with Enter/o viewer, copy-path, Save, and Open-all actions, reachable via the documented shortcut during execution. Repeated screens across users/attempts SHALL stay distinct. Suppressed login captures SHALL show the omission reason. Archived screenshots SHALL stay tied to their original cycle.

#### Scenario: Collection opened mid-wave
- **WHEN** the user opens the collection while a Browser UI agent executes
- **THEN** ready cards render without dispatching any harness turn
