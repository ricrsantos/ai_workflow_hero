package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func TestFormatElapsedUsesContinuousClock(t *testing.T) {
	cases := map[time.Duration]string{
		0:                          "00:00:00",
		5 * time.Second:            "00:00:05",
		65 * time.Second:           "00:01:05",
		24*time.Hour + time.Second: "24:00:01",
		49*time.Hour + 2*time.Minute + 3*time.Second: "49:02:03",
	}
	for input, want := range cases {
		if got := formatElapsed(input); got != want {
			t.Fatalf("formatElapsed(%s)=%q want %q", input, got, want)
		}
	}
}

func TestSessionTimerStartsAtZeroUntilCycleContinuation(t *testing.T) {
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	cycle := &store.Cycle{
		ID:                     7,
		Number:                 3,
		Status:                 store.CycleStatusActive,
		StartedAt:              now.Add(-(26*time.Hour + 2*time.Second)).Format(time.RFC3339Nano),
		SessionDurationSeconds: int64(26*time.Hour/time.Second) + 2,
	}
	m := NewTestModel(nil)
	m, _ = m.syncSessionTimer(cycle, now)
	if got := formatElapsed(m.sessionTimer.displayed); got != "00:00:00" {
		t.Fatalf("startup session=%q want 00:00:00", got)
	}
	if m.sessionTimer.running {
		t.Fatal("startup refresh must not start the cycle session timer")
	}

	m = m.requestCycleSessionRestore()
	m, _ = m.syncSessionTimer(cycle, now)
	if got := formatElapsed(m.sessionTimer.displayed); got != "26:00:02" {
		t.Fatalf("loaded session=%q want 26:00:02", got)
	}
	if !m.sessionTimer.running {
		t.Fatal("active cycle session timer must run")
	}

	m, _ = m.handleTimerTick(timerTickMsg{at: now.Add(3 * time.Second), generation: m.timerGeneration})
	if got := formatElapsed(m.sessionTimer.displayed); got != "26:00:05" {
		t.Fatalf("ticked session=%q want 26:00:05", got)
	}

	cycle.Status = store.CycleStatusCompleted
	cycle.CompletedAt = now.Add(-(2 * time.Second)).Format(time.RFC3339Nano)
	m, _ = m.syncSessionTimer(cycle, now)
	if m.sessionTimer.running {
		t.Fatal("completed cycle session timer must stop")
	}
	if got := formatElapsed(m.sessionTimer.displayed); got != "26:00:05" {
		t.Fatalf("completed session=%q want 26:00:05", got)
	}
}

func TestFreeChatSessionTimerStartsOnceAndContinuesAcrossPrompts(t *testing.T) {
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	m := NewTestModel(nil)
	m.freeChatMode = true
	m = m.startExecuteTimers(now)
	if !m.sessionTimer.running || m.sessionTimer.mode != sessionTimerFreeChat {
		t.Fatalf("first free-chat prompt must start Session: %+v", m.sessionTimer)
	}
	if got := formatElapsed(m.sessionTimer.displayed); got != "00:00:00" {
		t.Fatalf("first free-chat prompt=%q want 00:00:00", got)
	}

	m.sessionTimer.elapsed = 8 * time.Second
	m.sessionTimer.displayed = 8 * time.Second
	startedAt := m.sessionTimer.startedAt
	m = m.startExecuteTimers(now.Add(20 * time.Second))
	if got := formatElapsed(m.sessionTimer.displayed); got != "00:00:08" {
		t.Fatalf("second free-chat prompt reset Session=%q", got)
	}
	if !m.sessionTimer.startedAt.Equal(startedAt) {
		t.Fatalf("second free-chat prompt restarted Session at %s, want %s", m.sessionTimer.startedAt, startedAt)
	}
}

func TestSessionTimerStartsAtHeroNewAndResetsForFreeChat(t *testing.T) {
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	m := NewTestModel(nil).startCycleSessionTimer(now)
	if !m.sessionTimer.running || !m.sessionTimer.pendingCycle {
		t.Fatal("hero-new must start a pending cycle session timer")
	}
	m, _ = m.handleTimerTick(timerTickMsg{at: now.Add(8 * time.Second), generation: m.timerGeneration})
	if got := formatElapsed(m.sessionTimer.displayed); got != "00:00:08" {
		t.Fatalf("pending session=%q want 00:00:08", got)
	}
	previous := &store.Cycle{
		ID:                     6,
		Status:                 store.CycleStatusCompleted,
		SessionDurationSeconds: 90,
	}
	m, _ = m.syncSessionTimer(previous, now.Add(8*time.Second))
	if m.sessionTimer.mode != sessionTimerCycle || m.sessionTimer.cycleID != 0 {
		t.Fatalf("pending timer bound to previous cycle: %+v", m.sessionTimer)
	}
	if got := formatElapsed(m.sessionTimer.displayed); got != "00:00:08" {
		t.Fatalf("previous cycle replaced pending session=%q", got)
	}

	m = m.startFreeChatSessionTimer(now.Add(20 * time.Second))
	if got := formatElapsed(m.sessionTimer.displayed); got != "00:00:00" {
		t.Fatalf("free-chat reset=%q want 00:00:00", got)
	}
	m = m.resetSessionTimer()
	if got := formatElapsed(m.sessionTimer.displayed); got != "00:00:00" {
		t.Fatalf("session reset=%q want 00:00:00", got)
	}

	cancelled := NewTestModel(nil).startCycleSessionTimer(now)
	cancelled.runtimeCommandName = "new"
	cancelled.streaming = true
	updated, _ := cancelled.Update(streamCancelDoneMsg{})
	cancelled = updated.(model)
	if cancelled.sessionTimer.running || cancelled.sessionTimer.pendingCycle {
		t.Fatalf("cancelled hero-new left Session running: %+v", cancelled.sessionTimer)
	}
}

func TestAITimerStopsAndRestartsForEachDemand(t *testing.T) {
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	m := NewTestModel(nil).startAITimer(now)
	m, _ = m.handleTimerTick(timerTickMsg{at: now.Add(65 * time.Second), generation: m.timerGeneration})
	if got := formatElapsed(m.aiTimer.displayed); got != "00:01:05" {
		t.Fatalf("AI timer=%q want 00:01:05", got)
	}
	m = m.stopAITimer(now.Add(70 * time.Second))
	if m.aiTimer.running {
		t.Fatal("AI timer must stop after response")
	}
	if got := formatElapsed(m.aiTimer.displayed); got != "00:01:10" {
		t.Fatalf("stopped AI timer=%q want 00:01:10", got)
	}
	m = m.startAITimer(now.Add(100 * time.Second))
	if got := formatElapsed(m.aiTimer.displayed); got != "00:00:00" {
		t.Fatalf("new AI demand=%q want 00:00:00", got)
	}
}

func TestNavbarTimerSubdivisionIsAtBottom(t *testing.T) {
	m := NewTestModel(nil)
	m.width = 100
	m.status.CycleNumber = 1
	m.sessionTimer.displayed = time.Hour + 2*time.Minute + 3*time.Second
	m.aiTimer.displayed = 4 * time.Second
	view := stripANSI(m.renderNavSidebar(18))
	lines := strings.Split(view, "\n")
	session := strings.Index(view, "Sessão 01:02:03")
	ai := strings.Index(view, "AI     00:00:04")
	if session < 0 || ai < 0 {
		t.Fatalf("navbar missing timers:\n%s", view)
	}
	if session >= ai {
		t.Fatalf("session timer must precede AI timer:\n%s", view)
	}
	rangeLine := -1
	sessionLine := -1
	configLine := -1
	separatorLine := -1
	for i, line := range lines {
		switch {
		case strings.Contains(line, "alt+1-6"):
			rangeLine = i
		case strings.Contains(line, "Sessão 01:02:03"):
			sessionLine = i
		case strings.Contains(line, "Config"):
			configLine = i
		}
		if strings.Contains(line, strings.Repeat("─", 10)) && sessionLine < 0 {
			separatorLine = i
		}
	}
	if rangeLine < 0 || sessionLine != rangeLine+2 || separatorLine != rangeLine+1 {
		t.Fatalf("shortcut must sit immediately above timer divider:\n%s", view)
	}
	if configLine < 0 || rangeLine <= configLine+1 {
		t.Fatalf("shortcut must be separated from the menu options:\n%s", view)
	}
}
