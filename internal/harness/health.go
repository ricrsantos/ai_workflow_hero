package harness

import (
	"context"
	"time"
)

// HealthStatus classifies harness runtime health during TUI Execute.
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthSuspected HealthStatus = "suspected_hang"
	HealthFailed    HealthStatus = "failed"
)

// HarnessHealth is a normalized health snapshot from adapter probes plus
// activity timestamps maintained by the Hero watchdog.
type HarnessHealth struct {
	ProcessAlive   bool
	ServerAlive    bool
	SessionAlive   bool
	LastEventAt    time.Time
	LastActivityAt time.Time
	Status         HealthStatus
	Details        string
}

// HealthChecker is implemented by harness adapters that support runtime probes.
type HealthChecker interface {
	CheckHealth(ctx context.Context, sessionID string) (HarnessHealth, error)
}

// Default stall-detection timeouts (conservative; no hero.json schema in v2.3).
const (
	CursorStallTimeout   = 5 * time.Minute
	OpenCodeStallTimeout = 3 * time.Minute
	HealthProbeInterval  = 10 * time.Second
)

// StallTimeoutForHarness returns the inactivity threshold before suspected_hang.
func StallTimeoutForHarness(harnessID string) time.Duration {
	switch harnessID {
	case "opencode":
		return OpenCodeStallTimeout
	default:
		return CursorStallTimeout
	}
}
