## Context

Hero C1–C2 delivered SQLite state machine, TUI palette, and best-effort `HarnessAdapter.Dispatch` with chat fallback. C3 restores the intended V1 harness architecture: Cursor Agent CLI executes AI; Go orchestrates workflow state; TUI is a first-class autonomous entry (PRD-C03-001; ADR-C03-001).

Scope: **native only** → `generic_agent`. Terminology: **etapa** = workflow stage; **interação** = one conversational round within an etapa.

## Goals / Non-Goals

**Goals:**
- Full `HarnessAdapter` + working Cursor CLI adapter (`cursor agent` / `cursor-agent`).
- TUI conversation UI with `stream-json` streaming.
- Harness boot selection → validate → persist `cli.tools`.
- Hyphen slash vocabulary `/hero-<name>` (amend ADR-020).
- `/hero-cycles`, `/hero-todos`; extended `/hero-sync` pending-doc scan.
- Dual-entry: chat + TUI parity.

**Non-Goals:**
- Multi-harness adapters (D1).
- IDE chat injection.
- Go LLM calls.
- Windows.

## Decisions

### D1 — Hyphen slash vocabulary (ADR-024)

Replace colon form in user-facing copy:

| File stem | Label |
|---|---|
| `hero-new.md` | `/hero-new` |
| `hero-start.md` | `/hero-start` |
| … | `/hero-<stem without hero->` |

Runtime asset headers update from `# /hero:start` to `# /hero-start`. TUI palette uses same labels. CLI verbs unchanged (`hero approve`, …).

### D2 — HarnessAdapter interface (ADR-025)

```go
type HarnessAdapter interface {
    Name() string
    IsAvailable(ctx context.Context) error
    CreateSession(ctx context.Context, req SessionRequest) (*Session, error)
    ResumeSession(ctx context.Context, sessionID string) error
    Execute(ctx context.Context, req ExecuteRequest) (*ExecutionResult, error)
    Cancel(ctx context.Context, sessionID string) error
    Status(ctx context.Context, sessionID string) (*ExecutionStatus, error)
    // Dispatch retained as thin wrapper over Execute for legacy callers
    Dispatch(ctx context.Context, req DispatchRequest) (DispatchResult, error)
}
```

`ExecuteRequest` includes: `ProjectDir`, `Prompt`, `SessionID` (optional), `Stream bool`, `StageName`, `AgentName`.

`ExecutionResult`: `SessionID`, `Output`, `Summary`, `Usage` (input/output tokens if parseable), `Duration`, `StreamDone`.

Workflow Engine / TUI / `cycle.Service` call interface only — **no** `exec` in `internal/tui` or `internal/cycle`.

### D3 — Cursor CLI runner (ADR-025)

**Binary resolution order:**
1. `exec.LookPath("cursor-agent")`
2. `cursor` on PATH with subcommand `agent` (e.g. `cursor agent …`)

**Verify (no LLM):** `--version` or equivalent.

**Auth check:** attempt lightweight command or parse login error from failed execute; message must suggest `cursor agent login`.

**Execute:**
```bash
cursor-agent --print --output-format stream-json -p "<prompt>"
cursor-agent --print --output-format json --resume=<id> -p "<prompt>"
```

Inject `ProjectDir` as `cmd.Dir`. Parse stdout lines for stream deltas and final JSON.

**Pusher:** implement default `Pusher` in adapter (remove nil fallback when CLI available).

### D4 — TUI conversation orchestration (ADR-026)

- New screen `screenConversation` (or split layout on status screen).
- State: `harnessSessionID`, `transcript []message`, `input string`, `streaming bool`.
- On user submit → `Execute` with `Stream: true` → append user message → stream agent deltas to transcript.
- Etapa transitions still via `cycle.Service` / `hero stage *` — not inferred by harness.
- Interactive etapas (Research): orchestrator prompt = Runtime `discover_agent.md` + workflow context file pointers assembled in Go (paths only).

### D5 — TUI harness boot (ADR-027)

On `hero` / `hero tui` start:
1. Read `hero.json` → `cli.tools`.
2. If empty: huh/Bubble Tea select (V1 options: `cursor` only); auto-detect `.cursor/` as hint.
3. `adapter.IsAvailable()` — on error: print message + hint (`cursor agent login`) → `os.Exit(1)`.
4. Write `cli.tools` back to `hero.json`.

Independent of `hero doctor`.

### D6 — SQLite harness session (optional v3 migration)

Add to `stages` or new `harness_sessions` table:

```sql
-- Option A: column on stages
ALTER TABLE stages ADD COLUMN harness_session_id TEXT NOT NULL DEFAULT '';
```

Store session ID when etapa starts interactive harness work; clear on etapa complete.

### D7 — `/hero-cycles` (ADR-028)

- **CLI helpers (deterministic):** query `ListCycles`, `ListMetrics`; walk `.workflow-hero/cycles/archive/*/metrics.md` for legacy.
- **Runtime asset `hero-cycles.md`:** orchestrator formats output per UI-C03-001 §5.
- **TUI:** palette action `/hero-cycles` → render in flash or dedicated view.

### D8 — `/hero-todos` (ADR-028)

- Read `context/current-state.md`; extract `## Pending Features` (and SDD-defined sections).
- Always append sync notice (UI-C03-001 §6).
- No harness required (local file read). TUI + chat.

### D9 — Extended `/hero-sync` (ADR-029)

After existing context_agent scan:
- Read `docs/product/PRD.md`, cycle PRDs, `docs/architecture/ADR.md`, cycle ADRs.
- Extract explicit pending/deferred/out-of-scope-for-later bullets.
- Merge into `current-state.md` pending sections (dedupe by normalized text).
- Orchestrator-only (Runtime); no new CLI verb.

### D10 — New Runtime command files

- `assets/cursor/commands/hero-cycles.md`
- `assets/cursor/commands/hero-todos.md`
- Update `hero-sync.md` for pending-doc scan step.

## Risks / Trade-offs

- **Cursor CLI API drift:** injectable runner + version gate; tests use recorded stdout fixtures.
- **Streaming in Bubble Tea:** line-buffered updates; may throttle viewport refresh.
- **ADR-020 amendment:** bulk find/replace in assets/tests; C2 specs superseded on hyphen requirement.

## Migration Plan

1. Ship harness interface + cursor runner behind feature-complete adapter.
2. TUI boot + conversation screen.
3. Slash rename + new commands.
4. hero-sync extension.
5. Update `openspec/config.yml` context for C3.

## Open Questions

- Exact JSON schema from `cursor agent --output-format json` — validate against live CLI during implementation task 2.1.
