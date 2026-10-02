# UI-C17-001 — Browser Validation, Blocking, and Screenshot UX

Date: 2026-09-30. Status: confirmed Research design; not yet implemented. Source: PRD-C17-001-browser-validation-execution-budgets.md. TUI only; chat follows configured language.

## 1. Config and setup checklist

The Test users toggle reads and atomically persists `test_access.enabled` in `workflow-config.yml`, default Off (`false`) when absent. The same setting governs required-login preparation after a reload/restart; disabled required login blocks with enable instructions. No credential values are written to workflow YAML. This persistence key was confirmed by the user on 2026-09-30.

For browser-enabled frontend cycles, prioritize authentication setup immediately after /hero-new. Show Not needed, Setup pending, Users configured, or Access verified, with `.env.hero` lifecycle explanation. Unresolved authentication prevents test-plan approval.

Config → Test users exposes the opt-in setup toggle and a scrollable account collection. When off, explain how to enable it and that protected tests remain blocked. When on, offer Add/Edit/Remove with ID, profile, login and masked password; no password reveal/copy/export. Save validates field names and atomically updates root .env.hero; Discard restores the draft; Cancel returns without writing. Preserve unrelated dotenv entries. External modification displays “File changed outside Config. Reload before saving”; no stale password overwrite. Busy Config remains read-only under existing execution guards. Users may edit the same root file manually.

Browser UI and E2E stage sections each show Screenshots: Off/On (default Off). This controls per-screen communication captures, independently of Visual Validation and mandatory evidence. HTTP-only E2E cannot enable browser screenshots. The existing Telegram Always send reply setting controls forwarding; do not invent a second screenshot routing setting.

## 2. Blocking and retry

Render a durable blocked state instead of Running, with stage, reason, affected IDs/profile, corrective steps, remaining budget and retry command. A prerequisite is not a code failure. Do not expose account passwords or raw login/provider responses.

Example in PT-BR:

```text
→ Browser UI bloqueado: falta o usuário administrator com perfil admin.
→ Não validadas: screen-02 e screen-03.
→ Habilite Config → Test users e configure administrator, ou edite .env.hero na raiz.
→ Depois execute /hero-continue. Restam 8min20s de execução ativa.
```

Invalid password: instruct local correction and /hero-continue without printing it. Missing tool: name the selected method, missing prerequisite and declared setup command/location, assigning provisioning to the user/Implementation. Unsupported MFA/SSO: explain that interactive authentication is unsupported and request an approved test account with the supported flow. No useful screenshot exists yet: state that plainly. Mixed outcomes show “Defects recorded; validation blocked” with both finding and coverage IDs.

Edits do not resume automatically. /hero-continue reloads and rechecks before validation. At expired budget, explain how to increase the limit explicitly before retry. Human approval uses /hero-approve, /hero-reject, /hero-cancel or /hero-finish in the TUI; no agent advances stages on its own.

## 3. Active budget and progress

Show state, active elapsed/remaining, preparation phase, screen/journey ID, selected user/profile and completed/required count. Active elapsed counts parallel agents once; waiting/offline intervals are excluded. If a sibling is still executing, elapsed continues. On restart, show Interrupted, consumed/remaining, and /hero-continue; no silent automatic resumption.

At 60 seconds without coverage progress or 30 repeated events, emit one concise diagnostic for the current phase, with remaining budget. Suppress repeated warning spam and suspend warning accounting during human waits. The warning does not cancel/restart. Expiry clearly reports timeout, preserved evidence and intervention state after scoped termination; never remain indefinitely Running.

## 4. Screenshot cards and busy-state access

Each safely captured tested screen produces an image card with screenshot ID, coverage ID, stage, attempt, user/profile, timestamp and result. Store ready images under current/screenshots, never show partially written files. Use the existing Enter/o viewer, copy path and Save actions. Screenshots remain accessible while agents execute, from a dedicated collection shortcut and control-command path independent of ordinary blocked chat input. Planning chooses an unused key after checking the keyboard guide; list it in /help and the footer. Do not require a harness conversational turn to retrieve images. No inline terminal pixel preview is added.

The collection displays all ready captures with scrolling. Repeated screens under different users/attempts remain distinct. Login capture is suppressed until safe; show the reason when capture is omitted. Required evidence capture failure blocks its item; an optional communication failure warns. Archived screenshots remain tied to their original cycle.

## 5. /hero-screenshot

Accept commands asynchronously during active agents through a TUI control input/palette and Telegram routing:

```text
/hero-screenshot          latest ready capture
/hero-screenshot list     ready IDs and metadata
/hero-screenshot <id>     selected capture
/hero-screenshot todos    all ready captures, one request
```

Always show cards locally. `todos` snapshots the ready set, populates all cards, and asynchronously delivers batches when forwarding is enabled. The collection provides an Open all action using the system viewer with bounded launch scheduling; no per-image request is required. Show batch progress and precise failed IDs, allowing retry without rerunning tests. Do not silently omit files due to size/count or duplicate event delivery. No capture generated yet and unknown ID are distinct errors.

Telegram receives actual screenshots only when project always_send is enabled and the addressed chat is paired/connected; automatically captured images follow the same setting. When disabled, a remote request explains “Screenshots are available in the TUI. Enable Always send reply in Telegram settings to receive images here.” Text replies/status retain existing routing. No image input in unrelated control/approval forms is added.

## 6. Archive and disclosure

Archive retains safe screenshots and removes .env.hero only after archive prerequisites. Cleanup error: “Archive pending: could not remove project-root .env.hero. Resolve the file-access problem and run /hero-archive again.” Do not claim successful cleanup. Uninstall explicitly reports retained credentials. Captions and cards may contain ordinary test application data, but never credential values, tokens, raw authentication responses or secret paths.
