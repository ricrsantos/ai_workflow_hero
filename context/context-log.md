# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-08-10 — Chat panes: response box + scroll + wait animation

**Problem**: Conversation response area was plain text; user wanted OpenCode-style response pane matching the send box (green accent), scroll, wait animation; later black gutters and dead vertical gaps.

**Decision / Outcome**: Dual OpenCode panes. Black “scrollbar” blocks were nested Background / Width underfill — in-box text is fg-only; box uses `Background`+`Width`; accent rows pad to exact inner width. Sticky frame: footer chrome fixed; response line count = leftover height (probe `buildConversation(0)`). Etapa hint under `ready`; session inline with `│`. Tests green.

---

## 2026-08-10 — `/hero-sync` (orchestration agent, refresh)

**Problem**: `/hero-sync` requested to refresh Hero artifacts and merge pending items from product/architecture docs (ADR-029).

**Decision / Outcome**: Refreshed `current-state.md` technical debt; verified `AGENTS.md`, `project.json`, secrets hygiene. Pending-doc scan already covered v1.0.0 / D1–D13 / V2; merged GPG + upstream Cursor CLI gaps into debt.

---

## 2026-08-10 — Empty Artifacts/Costs/Events without active cycle

**Decision / Outcome**: Empty views say `No active cycle. Run /hero-new to start.` when cycle number is 0.

---

## 2026-08-10 — TUI status bar + `/hero-sync` UX fixes

**Decision / Outcome**: Fixed footer status bar (running/ok/error with wrap); close palette on select + busy-guard; `Execute` only resumes when `SessionID` is set; `defaultPush` no longer truncates at 240 chars.

---

## 2026-08-10 — TUI Chat OpenCode UX (consolidated)

**Decision / Outcome**: Chat input boxed accent bar (Build/Plan via Tab → `--mode plan`); `/hero-model` picker; homogeneous `chatBg`; focus caret; accent flush left; screen nav `ctrl/alt+N`, quit `ctrl+q`. Live stream via `--stream-partial-output` + thinking/tool deltas in transcript. Title: "AI Hero".
