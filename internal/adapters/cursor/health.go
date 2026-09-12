package cursor

import (
	"context"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// CheckHealth implements harness.HealthChecker for the Cursor CLI harness.
//
// HasInFlight is false both when the CLI process crashed and when Execute has
// already returned successfully (clearRunning in defer). The TUI watchdog must
// not treat that second case as HealthFailed: a probe that races executeDone
// or the next handoff Execute would warn (or previously auto-cancel) with
// "session idle". Terminal success/cancel/idle for a known session stays
// alive; only StatusFailed marks the session dead.
func (a *Adapter) CheckHealth(ctx context.Context, sessionID string) (harness.HarnessHealth, error) {
	_ = ctx
	alive := a.HasInFlight()
	health := harness.HarnessHealth{
		ProcessAlive: alive,
		ServerAlive:  true,
		SessionAlive: true,
		Details:      "cursor cli",
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID != "" {
		st, err := a.Status(ctx, sessionID)
		if err != nil {
			return health, err
		}
		switch st.State {
		case harness.StatusFailed:
			health.SessionAlive = false
			health.Details = st.Message
		case harness.StatusRunning:
			health.ProcessAlive = true
			health.Details = "session running"
		default:
			// completed / cancelled / idle: Execute is gone or not yet
			// setRunning. Keep process+session alive so Evaluate does not
			// classify a clean complete as HealthFailed.
			health.ProcessAlive = true
			health.SessionAlive = true
			if st.State != "" {
				health.Details = "session " + st.State
			} else {
				health.Details = "session idle"
			}
		}
	} else if !alive {
		health.ProcessAlive = false
		health.Details = "no in-flight cursor agent process"
	}
	return health, nil
}
