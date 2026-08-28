package tui

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// timerTickMsg drives both counters.  The generation prevents a timer that
// was already queued when a session was reset from starting a second loop.
type timerTickMsg struct {
	at         time.Time
	generation uint64
}

// Keep the old name as a source-compatible alias for package tests and older
// callers.  The footer status no longer owns a timer; all ticks belong to the
// shared TUI counters.
type statusTickMsg = timerTickMsg

type sessionTimerMode uint8

const (
	sessionTimerNone sessionTimerMode = iota
	sessionTimerCycle
	sessionTimerFreeChat
)

type sessionTimerState struct {
	mode         sessionTimerMode
	cycleID      int64
	cycleNumber  int
	startedAt    time.Time
	elapsed      time.Duration
	displayed    time.Duration
	persistedSec int64
	running      bool
	pendingCycle bool // /hero-new has started, but SQLite has not got the row yet.
	suppressed   bool // archive/free-chat reset wins over a stale refresh result.
}

type aiTimerState struct {
	startedAt time.Time
	displayed time.Duration
	running   bool
}

func timerTickCmd(generation uint64) tea.Cmd {
	return tea.Tick(time.Second, func(at time.Time) tea.Msg {
		return timerTickMsg{at: at, generation: generation}
	})
}

func (m model) hasTimerWork() bool {
	return m.sessionTimer.running || m.aiTimer.running
}

func (m *model) ensureTimerLoop() tea.Cmd {
	if !m.hasTimerWork() || m.timerLoopStarted {
		return nil
	}
	m.timerLoopStarted = true
	m.timerGeneration++
	return timerTickCmd(m.timerGeneration)
}

func (m *model) invalidateTimerLoop() {
	m.timerLoopStarted = false
	m.timerGeneration++
}

func (t sessionTimerState) elapsedAt(at time.Time) time.Duration {
	d := t.elapsed
	if t.running && !t.startedAt.IsZero() {
		d += at.Sub(t.startedAt)
	}
	if d < 0 {
		return 0
	}
	return d.Truncate(time.Second)
}

func (t aiTimerState) elapsedAt(at time.Time) time.Duration {
	if !t.running || t.startedAt.IsZero() {
		return t.displayed.Truncate(time.Second)
	}
	d := at.Sub(t.startedAt)
	if d < 0 {
		return 0
	}
	return d.Truncate(time.Second)
}

func (m model) saveSessionDurationCmd(seconds int64) tea.Cmd {
	if m.svc == nil || m.sessionTimer.cycleID <= 0 || seconds < 0 {
		return nil
	}
	svc := m.svc
	cycleID := m.sessionTimer.cycleID
	return func() tea.Msg {
		if err := svc.UpdateCycleSessionDuration(cycleID, seconds); err != nil {
			// The in-memory counter remains authoritative for this TUI process;
			// the next refresh can recover if the store becomes available again.
			slog.Debug("tui persist session duration failed", "cycle", cycleID, "error", err)
		}
		return nil
	}
}

func combineTimerCmds(cmds ...tea.Cmd) tea.Cmd {
	filtered := make([]tea.Cmd, 0, len(cmds))
	for _, cmd := range cmds {
		if cmd != nil {
			filtered = append(filtered, cmd)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return tea.Batch(filtered...)
	}
}

func maxTimerSeconds(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (m model) startCycleSessionTimer(at time.Time) model {
	if at.IsZero() {
		at = time.Now()
	}
	m.sessionTimer = sessionTimerState{
		mode:         sessionTimerCycle,
		startedAt:    at,
		running:      true,
		pendingCycle: true,
	}
	return m
}

func (m model) startFreeChatSessionTimer(at time.Time) model {
	if at.IsZero() {
		at = time.Now()
	}
	m.sessionTimer = sessionTimerState{
		mode:      sessionTimerFreeChat,
		startedAt: at,
		running:   true,
	}
	return m
}

func (m model) startAITimer(at time.Time) model {
	if at.IsZero() {
		at = time.Now()
	}
	m.aiTimer = aiTimerState{startedAt: at, running: true}
	return m
}

func (m model) stopAITimer(at time.Time) model {
	if !m.aiTimer.running {
		return m
	}
	m.aiTimer.displayed = m.aiTimer.elapsedAt(at)
	m.aiTimer.startedAt = time.Time{}
	m.aiTimer.running = false
	if !m.hasTimerWork() {
		m.invalidateTimerLoop()
	}
	return m
}

func (m model) resetAITimer() model {
	m.aiTimer = aiTimerState{}
	if !m.hasTimerWork() {
		m.invalidateTimerLoop()
	}
	return m
}

func (m model) stopSessionTimer(at time.Time) (model, tea.Cmd) {
	if !m.sessionTimer.running {
		return m, nil
	}
	m.sessionTimer.displayed = m.sessionTimer.elapsedAt(at)
	m.sessionTimer.elapsed = m.sessionTimer.displayed
	m.sessionTimer.startedAt = time.Time{}
	m.sessionTimer.running = false
	m.sessionTimer.pendingCycle = false
	var saveCmd tea.Cmd
	seconds := int64(m.sessionTimer.displayed / time.Second)
	if seconds > m.sessionTimer.persistedSec {
		m.sessionTimer.persistedSec = seconds
		saveCmd = m.saveSessionDurationCmd(seconds)
	}
	if !m.hasTimerWork() {
		m.invalidateTimerLoop()
	}
	return m, saveCmd
}

func (m model) resetSessionTimer() model {
	m.sessionTimer = sessionTimerState{suppressed: true}
	if !m.hasTimerWork() {
		m.invalidateTimerLoop()
	}
	return m
}

func (m model) handleTimerTick(msg timerTickMsg) (model, tea.Cmd) {
	if msg.generation != 0 && msg.generation != m.timerGeneration {
		return m, nil
	}
	at := msg.at
	if at.IsZero() {
		at = time.Now()
	}
	if m.sessionTimer.running {
		m.sessionTimer.displayed = m.sessionTimer.elapsedAt(at)
	}
	if m.aiTimer.running {
		m.aiTimer.displayed = m.aiTimer.elapsedAt(at)
	}
	var saveCmd tea.Cmd
	if m.sessionTimer.running {
		seconds := int64(m.sessionTimer.displayed / time.Second)
		if seconds > m.sessionTimer.persistedSec {
			m.sessionTimer.persistedSec = seconds
			saveCmd = m.saveSessionDurationCmd(seconds)
		}
	}
	if !m.hasTimerWork() {
		m.invalidateTimerLoop()
		return m, saveCmd
	}
	return m, combineTimerCmds(saveCmd, timerTickCmd(m.timerGeneration))
}

func parseCycleTimerTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// cycleTimerState returns the persisted active duration. The timestamp
// fallback keeps cycles created before the timer migration readable.
func cycleTimerState(c *store.Cycle, at time.Time) (elapsed time.Duration, started time.Time, running bool) {
	if c == nil {
		return 0, time.Time{}, false
	}
	if c.SessionDurationSeconds > 0 {
		elapsed = time.Duration(c.SessionDurationSeconds) * time.Second
		if c.Status == store.CycleStatusActive {
			return elapsed, at, true
		}
		return elapsed, time.Time{}, false
	}
	start, hasStart := parseCycleTimerTime(c.StartedAt)
	if c.Status == store.CycleStatusActive {
		// Active timers resume from the saved seconds and start a new in-TUI
		// segment. This deliberately excludes time while the TUI was closed.
		return 0, at, true
	}
	if !hasStart {
		return 0, time.Time{}, false
	}
	end, hasEnd := parseCycleTimerTime(c.CompletedAt)
	if !hasEnd || end.Before(start) {
		end = at
	}
	elapsed = end.Sub(start).Truncate(time.Second)
	if elapsed < 0 {
		elapsed = 0
	}
	return elapsed, time.Time{}, false
}

func (m model) syncSessionTimer(c *store.Cycle, at time.Time) (model, tea.Cmd) {
	if at.IsZero() {
		at = time.Now()
	}

	if c == nil {
		// A free-chat timer is independent of project refreshes. A cycle timer,
		// however, has no owner after archive and must return to zero.
		if m.sessionTimer.mode == sessionTimerCycle && !m.sessionTimer.pendingCycle {
			m = m.resetSessionTimer()
		}
		return m, m.ensureTimerLoop()
	}

	// Do not resurrect an old completed cycle after the user explicitly
	// archived/reset the session. A newly resumed active cycle is allowed.
	if m.sessionTimer.suppressed && c.Status != store.CycleStatusActive {
		return m, m.ensureTimerLoop()
	}
	// A normal TUI without an active cycle can have a free-chat conversation;
	// an old completed row must not replace that in-memory timer.
	if m.sessionTimer.mode == sessionTimerFreeChat && c.Status != store.CycleStatusActive {
		return m, m.ensureTimerLoop()
	}

	var saveCmd tea.Cmd
	if m.sessionTimer.pendingCycle {
		// While /hero-new is still running, refreshes can legitimately return
		// the previous completed cycle. Do not attach that row's frozen time to
		// the new pending cycle; only the newly-created active row can bind it.
		if c.Status != store.CycleStatusActive {
			return m, m.ensureTimerLoop()
		}
		carried := m.sessionTimer.elapsedAt(at)
		cycleElapsed, _, _ := cycleTimerState(c, at)
		if cycleElapsed > carried {
			carried = cycleElapsed
		}
		persistedSec := maxTimerSeconds(c.SessionDurationSeconds, 0)
		carriedSec := int64(carried / time.Second)
		if carriedSec > persistedSec {
			persistedSec = carriedSec
		}
		m.sessionTimer = sessionTimerState{
			mode:         sessionTimerCycle,
			cycleID:      c.ID,
			cycleNumber:  c.Number,
			startedAt:    at,
			elapsed:      carried,
			displayed:    carried,
			persistedSec: persistedSec,
			running:      c.Status == store.CycleStatusActive,
		}
		if carriedSec > c.SessionDurationSeconds {
			saveCmd = m.saveSessionDurationCmd(carriedSec)
		}
		if !m.sessionTimer.running {
			m.sessionTimer.elapsed, _, _ = cycleTimerState(c, at)
			m.sessionTimer.displayed = m.sessionTimer.elapsed
		}
	} else if m.sessionTimer.mode == sessionTimerCycle && m.sessionTimer.cycleID == c.ID {
		if c.Status == store.CycleStatusActive {
			if !m.sessionTimer.running {
				elapsed, started, _ := cycleTimerState(c, at)
				if elapsed < m.sessionTimer.elapsed {
					elapsed = m.sessionTimer.elapsed
				}
				m.sessionTimer.elapsed = elapsed
				m.sessionTimer.startedAt = started
				if m.sessionTimer.startedAt.IsZero() {
					m.sessionTimer.startedAt = at
				}
				m.sessionTimer.running = true
				m.sessionTimer.persistedSec = maxTimerSeconds(m.sessionTimer.persistedSec, int64(elapsed/time.Second))
				if m.sessionTimer.persistedSec > c.SessionDurationSeconds {
					saveCmd = m.saveSessionDurationCmd(m.sessionTimer.persistedSec)
				}
			}
			m.sessionTimer.displayed = m.sessionTimer.elapsedAt(at)
		} else {
			elapsed, _, _ := cycleTimerState(c, at)
			if elapsed < m.sessionTimer.elapsed {
				elapsed = m.sessionTimer.elapsed
			}
			if elapsed < m.sessionTimer.displayed {
				elapsed = m.sessionTimer.displayed
			}
			m.sessionTimer.elapsed = elapsed
			m.sessionTimer.displayed = elapsed
			m.sessionTimer.startedAt = time.Time{}
			m.sessionTimer.running = false
			m.sessionTimer.persistedSec = maxTimerSeconds(m.sessionTimer.persistedSec, int64(elapsed/time.Second))
			if m.sessionTimer.persistedSec > c.SessionDurationSeconds {
				saveCmd = m.saveSessionDurationCmd(m.sessionTimer.persistedSec)
			}
		}
	} else {
		elapsed, started, running := cycleTimerState(c, at)
		m.sessionTimer = sessionTimerState{
			mode:         sessionTimerCycle,
			cycleID:      c.ID,
			cycleNumber:  c.Number,
			startedAt:    started,
			elapsed:      elapsed,
			displayed:    elapsed,
			persistedSec: maxTimerSeconds(c.SessionDurationSeconds, 0),
			running:      running,
		}
		if running {
			m.sessionTimer.displayed = m.sessionTimer.elapsedAt(at)
		}
	}
	m.sessionTimer.suppressed = false
	return m, combineTimerCmds(saveCmd, m.ensureTimerLoop())
}

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int64(d / time.Second)
	hours := totalSeconds / 3600
	minutes := (totalSeconds / 60) % 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
