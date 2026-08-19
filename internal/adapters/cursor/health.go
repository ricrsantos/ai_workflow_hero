package cursor

import (
	"context"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// CheckHealth implements harness.HealthChecker for the Cursor CLI harness.
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
			health.Details = "session running"
		default:
			if !alive {
				health.SessionAlive = false
				health.Details = "session idle"
			}
		}
	} else if !alive {
		health.ProcessAlive = false
		health.Details = "no in-flight cursor agent process"
	}
	return health, nil
}
