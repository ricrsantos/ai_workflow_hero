# Context Log

> Short-term project memory for **this repository** (the Hero CLI + Runtime assets themselves).
>
> Keep only information relevant to the last 3–5 work sessions/cycles. Keep this document under 1,000 words by removing or consolidating outdated entries. Permanent facts (architecture, stack, features) belong in `context/current-state.md`, not here.

---

## 2026-08-21 — C6 §9: Integration + close (native complete)

**Problem**: Close Implementation: integration lock for upgrade→enable→Execute→orphan reap, SemVer 2.5.0, context landing, openspec-change persistence.

**Decision / Outcome**: Added `internal/integration/codex_c6_test.go` (§9.1: 2.4→2.5 upgrade leaves Codex off; `EnableHarnessWithProjection` projects `.codex/`; mock stdio Execute streams text+thinking; Codex orphan reap clears dead registry PID). Default `cmd/hero` version + `scripts/release.sh` tag example → **2.5.0** (ldflags injection unchanged). `hero status` shows OpenSpec change `codex-adapter` on C6. Tasks 9.1–9.4 checked; `go test ./...` green. Phase: native C6 complete → QA/Judge next.

---

## 2026-08-21 — C6 §8 parallel tracks (summary)

- **8A/8B**: `/hero-harness` Codex enable/disable + projection; `/hero-model` Codex harness step + `ListModels` + C5 properties; stage YAML untouched.
- **8C**: Chat `[LABEL - model · harness]` / `Build · model · harness`; Codex auth/CLI/app-server goldens; unknown events → yellow warning.
- **8D**: Codex-native ids in `assets/models/*.yml` (no invented ChatGPT USD rates); metrics warn on unknown id.
- **8E**: `/harness-reset` includes Codex; `PrepareHeroStart` sync/reset/probe tests.
- **8F**: Doctor warn-only `codex-cli` when enabled and missing from PATH.
- **8G**: workflow-help + README EN/PT-BR + DEPLOY 2.5.0; Cursor Runtime assets unchanged.
- **8H**: Cursor/OpenCode Execute untouched; mixed three-harness integration + OpenCode Prepare when Codex unused.

---

## 2026-08-21 — C6 §1–§7 (summary)

**Outcome**: hero.json + SQLite Codex registry; `/hero-new` three-harness injection; install/upgrade Codex opt-in (2.4→2.5 never auto-enables); `internal/adapters/codex` stdio JSON-RPC + stream/permission/auth/C5/usage; `assets/codex/` projection; harnessmgr + TUI boot/orphan/health; fallback + Execute routing + Prepare-on-`/hero-start`.

---

## 2026-08-20 — Releases / pre-C6

- **v2.4.1**: TUI UX patch (bordered prompt, clipboard, context bar).
- **v2.4.0**: OpenCode serve lifecycle + orphan reap.
- **C5 archived** (`model-properties-tui`); Judge approved without formal SDD verify (judge_agent empty in opencode harness).

---
