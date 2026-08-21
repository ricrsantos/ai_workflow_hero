package codex

import (
	"context"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// Compile-time check: Codex supports TUI Execute health probes.
var _ harness.HealthChecker = (*Adapter)(nil)

// CheckHealth implements harness.HealthChecker for the Codex app-server harness.
//
// Probes are strictly observational: they MUST NOT call ensureAppServer,
// StopAppServer, or take startMu. Inconclusive session lookups leave
// SessionAlive true to avoid false degraded warnings.
func (a *Adapter) CheckHealth(ctx context.Context, sessionID string) (harness.HarnessHealth, error) {
	_ = ctx
	a.mu.Lock()
	pid := a.appPID
	rpc := a.rpc
	a.mu.Unlock()

	alive := pid > 0 && processAlive(pid) && !processZombie(pid)
	if alive && !IsManagedCodexAppServer(pid) {
		alive = false
	}
	health := harness.HarnessHealth{
		ProcessAlive: alive,
		ServerAlive:  alive && rpc != nil,
		SessionAlive: true,
		Details:      "codex app-server",
	}
	if !alive {
		health.ProcessAlive = false
		health.ServerAlive = false
		health.Details = "codex app-server not running"
		return health, nil
	}
	if rpc == nil {
		health.ServerAlive = false
		health.Details = "codex app-server process alive but RPC not connected"
		return health, nil
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return health, nil
	}
	st, err := a.Status(ctx, sessionID)
	if err != nil {
		return health, err
	}
	switch st.State {
	case harness.StatusFailed:
		health.SessionAlive = false
		if msg := strings.TrimSpace(st.Message); msg != "" {
			health.Details = msg
		}
	case harness.StatusRunning:
		health.Details = "session running"
	case harness.StatusCancelled:
		health.Details = "session cancelled"
	default:
		// Idle / unknown — keep SessionAlive true (inconclusive for hang detection).
	}
	return health, nil
}
