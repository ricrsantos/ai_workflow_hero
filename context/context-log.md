# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-07-20 — Grilling session: full V1 design definition

**Problem:** `docs/idea/ai_workflow_hero.md` contained a broad initial design for Hero with many open questions (approval loops, model fallback, scope handling, document numbering, CLI vs. Runtime boundaries, testing strategy, release process, terminal UX, etc.).

**Investigation:** Ran an extensive `/grill-me` session covering every open question in the source document, plus three additional recommendations from the user: clean subagent sessions (Task tool, file pointers only), Go testing principles (real dependencies, colocated tests), and Feature Based Architecture for the CLI.

**Decision:** Resolved 70+ individual decisions (see the "Decisões da Sessão de Grilling" sections at the end of `docs/idea/ai_workflow_hero.md` for the full log), covering: approval/control loop semantics (`/hero:continue`, `/hero:back`, `/hero:resume`), implementation orchestration (Task tool, OpenSpec integration), the new `generic_agent` and extended `scope` (native/script/infrastructure), the 3-level model fallback chain, document numbering (`[CATEGORY]-C[XX]-[seq]-[slug].md`), stage-dependency validation, `/hero:sync` promoted to V1, Git as a mandatory prerequisite, concurrency locking, metrics structure, and CLI terminal UX (colors/icons, `--json`, survey prompts, error format).

**Outcome:** All decisions incorporated into `docs/idea/ai_workflow_hero.md`. No code written — this was a pure design/planning session.

**Next step:** Consolidate all decisions into formal documents (`AGENTS.md`, PRD, UI spec, ADRs, DEPLOY.md) so another agent can act on them without re-reading the full grilling transcript.

---

## 2026-07-20 — Documentation creation (AGENTS.md, PRD, UI, ADR, DEPLOY)

**Problem:** Needed formal, cross-referenced project documents capturing all grilling decisions, so any agent (not just the one that ran the session) can understand and act on them.

**Investigation:** Reviewed the full grilling decisions log and mapped each decision to the document(s) it belongs in (product requirement, architecture rationale, UX spec, or deployment process).

**Decision:** Created 5 documents: `AGENTS.md` (permanent prompt), `docs/product/PRD.md`, `docs/product/UI.md`, `docs/architecture/ADR.md` (10 ADRs), `docs/deployment/DEPLOY.md`. Used the `ADR-NNN-title` convention inside a single indexed `ADR.md` file rather than splitting into separate files per ADR, since the set was small enough (10) to stay readable in one document with anchors.

**Outcome:** All 5 documents created and committed (`61924f8`). Found and fixed an inconsistency: `AGENTS.md`/`PRD.md` referenced `ADR-004` for "CLI vs. Runtime separation" but the actual ADR index had this as `ADR-003` (`ADR-004` was "Git as a mandatory prerequisite"). Fixed both references.

**Next step:** Add context compression files (`context/current-state.md`, `context/context-log.md`) and strengthen `AGENTS.md` with explicit ambiguity-questioning policy, documentation map (including context files), reference lookup order, and a concrete test-loop procedure.

---

## 2026-07-20 — Context files + AGENTS.md hardening

**Problem:** `AGENTS.md` was missing: (1) an explicit instruction to question ambiguous/missing information rather than assume, (2) pointers to context compression files, (3) a concrete reference lookup order (project docs → Context7 → web), and (4) a concrete run-tests-until-green loop. The project itself also lacked its own `context/` files.

**Investigation:** The user's requested test loop used `npm test`, but this repository's stack is Go (Cobra CLI), with no Node/npm involved. Asked the user to confirm the correct test command via `AskQuestion` rather than silently substituting one — per the same ambiguity policy being added to `AGENTS.md`.

**Decision:** User confirmed `go test ./...` as the equivalent command. Updated `AGENTS.md` Testing section to use it. Created `context/current-state.md` and `context/context-log.md` for this repository (describing Hero's own codebase state, not an end-user project managed by Hero).

**Outcome:** `AGENTS.md` trimmed and reorganized to stay within the 700-word target while adding all requested sections. Two new context files created.

**Next step:** Extend `docs/architecture/ADR.md`, `docs/product/PRD.md`, and `docs/deployment/DEPLOY.md` with the full data-file schemas (`hero.json`, `project.json`, `documents.json`, `models/*.yml`) and testing-requirement cross-references, per user request to make the documents fully self-sufficient with code snapshots.

---

_To be maintained by agents. Prune entries older than the last 3–5 sessions once they no longer inform current work._
