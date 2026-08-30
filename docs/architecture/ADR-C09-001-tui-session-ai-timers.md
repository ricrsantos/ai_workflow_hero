# ADR-058 — Shared TUI Session and AI Timers

> Architecture decision for the Hero TUI counters requested after C8. Status: Accepted.

## Context

The TUI previously displayed an elapsed value owned by the `/hero-start` status
footer. That made the value action-specific and left `hero chat`, ordinary
conversation, and direct stage Executes without a common timing model. The
product needs three second-resolution counters in the navbar:

- `Session`: the active cycle session or the current free-chat session.
- `AI wk`: the currently executing demand only.
- `AI rp`: elapsed time since the latest harness response content, whether or
  not the active transcript-detail profile displays it.

Cycle session time must be available for explicit recovery after a TUI restart
and must not include time after a cycle is finished or archived. Free-chat time
and both AI counters must not be persisted.

## Decision

1. Put all counters in `internal/tui/timers.go`, driven by one Bubble Tea
   one-second tick. Format values as `HH:MM:SS`; hours are unbounded rather
   than wrapping at 24.
2. Add `cycles.session_duration_seconds` in SQLite schema v8. TUI ticks write
   the greatest known number of active cycle seconds through the cycle service;
   monotonic writes protect against an older asynchronous tick overwriting a
   newer value. TUI startup deliberately begins at zero; `/hero-start` and
   `/hero-resume` explicitly request recovery from the persisted value and
   start a new local segment, so closed-TUI time is excluded.
3. Start the cycle Session timer at zero when `/hero-new` is dispatched. Stop
   and save it on a terminal cycle state; archive resets the displayed Session
   value. An ordinary chat prompt starts free chat at zero on the first
   submitted prompt unless this TUI has explicitly restored a cycle session;
   it keeps that value across later prompts. Free-chat Session state is
   process-local.
4. Start `AI wk` on the first Execute of a demand, stop it when the last
   concurrent Execute returns or is cancelled, and reset it for the next
   demand. `AI wk` is process-local and never written to SQLite.
5. Keep `AI rp` at zero when the TUI opens. Start it only after a harness
   response-content event, restart it for every such response (including text,
   thinking, tools, activity, and warnings), and let it continue after an
   Execute ends. This applies even when the selected transcript-detail profile
   filters the event from Chat. Interactive callbacks restart it when they are
   presented to the user. Session metadata and local watchdog alerts do not
   restart it, so a silent harness remains observable. `AI rp` is process-local
   and never written to SQLite.
6. Render `Session`, `AI wk`, and `AI rp` as the bottom subdivision of the left navbar,
   beneath the navigation shortcut hint, using the existing blue info accent.
   The footer status bar remains responsible only for action state/messages.

## Consequences

- Cycle duration is available consistently in `hero tui` and `hero chat`'s
  shared rendering model without coupling the footer to `/hero-start`.
- Opening the TUI never silently restores persisted cycle time; only an
  explicit cycle continuation command does so.
- Existing databases migrate additively; cycles created before schema v8 retain
  timestamp fallback behavior for completed states.
- Timer persistence is operational state, not a cost/metrics measurement; both
  AI counters continue to be transient UI state.
