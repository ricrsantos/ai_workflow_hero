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
	pausedAt       time.Time
	paused         bool
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
	w.pausedAt = time.Time{}
	w.paused = false
}

// Pause excludes an expected, user-driven harness wait from stall detection.
// Repeated calls while paused leave the original start time intact.
func (w *Watchdog) Pause(now time.Time) {
	if w.paused {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	w.pausedAt = now
	w.paused = true
}

// Resume continues stall detection while preserving elapsed active time from
// before Pause. Time spent waiting for the user is not counted as inactivity.
func (w *Watchdog) Resume(now time.Time) {
	if !w.paused {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	pausedFor := now.Sub(w.pausedAt)
	if pausedFor > 0 {
		w.startedAt = w.startedAt.Add(pausedFor)
		if w.hasEvent {
			w.lastEventAt = w.lastEventAt.Add(pausedFor)
		}
		if w.hasActivity {
			w.lastActivityAt = w.lastActivityAt.Add(pausedFor)
		}
	}
	w.pausedAt = time.Time{}
	w.paused = false
}

// IsPaused reports whether stall-time accounting is currently suspended.
func (w *Watchdog) IsPaused() bool {
	return w.paused
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

// HasRecentActivity reports whether substantive stream activity occurred within window.
func (w *Watchdog) HasRecentActivity(now time.Time, window time.Duration) bool {
	if !w.hasActivity || window <= 0 {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	return now.Sub(w.lastActivityAt) < window
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
		return HealthFailed
	}
	if w.paused {
		return HealthHealthy
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
	case StreamKindText, StreamKindThinking, StreamKindTool, StreamKindQuestion, StreamKindPermission:
		return true
	default:
		return false
	}
}
