package harness

import "time"

// Watchdog tracks stream activity during a single TUI Execute and evaluates
// hang suspicion by combining adapter probes with last-activity timestamps.
type Watchdog struct {
	startedAt      time.Time
	lastEventAt    time.Time
	lastActivityAt time.Time
	hasEvent       bool
	hasActivity    bool
}

// Reset starts a new watchdog window (call at beginConversationExecute).
func (w *Watchdog) Reset(now time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	w.startedAt = now
	w.lastEventAt = time.Time{}
	w.lastActivityAt = time.Time{}
	w.hasEvent = false
	w.hasActivity = false
}

// RecordDelta updates event/activity timestamps from a live stream delta.
func (w *Watchdog) RecordDelta(d StreamDelta, now time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	w.lastEventAt = now
	w.hasEvent = true
	if isActivityDelta(d) {
		w.lastActivityAt = now
		w.hasActivity = true
	}
}

// LastEventAt returns the most recent stream event time, if any.
func (w *Watchdog) LastEventAt() time.Time {
	return w.lastEventAt
}

// LastActivityAt returns the most recent substantive activity time, if any.
func (w *Watchdog) LastActivityAt() time.Time {
	return w.lastActivityAt
}

// Evaluate merges adapter probe results with local activity timestamps.
// Absence of events alone does not imply a hang; process/server/session must
// be considered together with last activity (v2.3 design).
func (w *Watchdog) Evaluate(now time.Time, probe HarnessHealth, stallTimeout time.Duration) HealthStatus {
	if now.IsZero() {
		now = time.Now()
	}
	if !probe.ProcessAlive {
		return HealthFailed
	}
	if !probe.ServerAlive {
		return HealthDegraded
	}
	if !probe.SessionAlive {
		return HealthDegraded
	}

	activityAt := w.lastActivityAt
	if !w.hasActivity {
		activityAt = w.startedAt
	}
	if stallTimeout > 0 && now.Sub(activityAt) >= stallTimeout && probe.ProcessAlive {
		return HealthSuspected
	}
	return HealthHealthy
}

func isActivityDelta(d StreamDelta) bool {
	switch d.Kind {
	case StreamKindText, StreamKindThinking, StreamKindTool, StreamKindActivity:
		return true
	case StreamKindSession:
		if d.Metadata != nil {
			switch d.Metadata["state"] {
			case SessionStateRunning, SessionStateIdle:
				return true
			}
		}
	}
	return false
}
