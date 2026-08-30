package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func TestSettingsIsLastWithoutCycleAndDebugIsDefault(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 28)
	plain := stripANSI(ViewForTest(m))
	if !strings.Contains(plain, "Settings") {
		t.Fatalf("Settings missing from navbar: %q", plain)
	}
	if strings.Index(plain, "Settings") < strings.Index(plain, "Events") {
		t.Fatalf("Settings must follow Events: %q", plain)
	}
	if got := m.settings.verbosity; got != install.ChatVerbosityDebug {
		t.Fatalf("default verbosity=%q want debug", got)
	}
}

func TestSettingsRendersProfilesAndKeyboardSelection(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 28)
	m, _ = m.openSettings()
	plain := stripANSI(ViewForTest(m))
	for _, label := range []string{"Compact", "Standard", "Detailed", "Debug"} {
		if !strings.Contains(plain, label) {
			t.Fatalf("missing %q from settings: %q", label, plain)
		}
	}
	next, _ := HandleTestKey(m, "up")
	if next.settings.cursor != len(verbosityOptions)-2 {
		t.Fatalf("cursor=%d want detailed index", next.settings.cursor)
	}
}

func TestChatVerbosityFiltersOnlyTranscriptDetails(t *testing.T) {
	m := NewTestModel(nil)
	m.settings.verbosity = install.ChatVerbosityCompact
	if !m.chatVerbosityShows(harness.StreamDelta{Kind: harness.StreamKindText}) {
		t.Fatal("compact must show assistant text")
	}
	if m.chatVerbosityShows(harness.StreamDelta{Kind: harness.StreamKindThinking}) {
		t.Fatal("compact must hide thinking")
	}
	m.settings.verbosity = install.ChatVerbosityStandard
	if !m.chatVerbosityShows(harness.StreamDelta{Kind: harness.StreamKindTool}) {
		t.Fatal("standard must show tools")
	}
	if m.chatVerbosityShows(harness.StreamDelta{Kind: harness.StreamKindActivity}) {
		t.Fatal("standard must hide activities")
	}
	m.settings.verbosity = install.ChatVerbosityDetailed
	if !m.chatVerbosityShows(harness.StreamDelta{Kind: harness.StreamKindThinking}) || !m.chatVerbosityShows(harness.StreamDelta{Kind: harness.StreamKindActivity}) {
		t.Fatal("detailed must show thinking and activities")
	}
}

func TestCompactVerbosityResetsResponseTimerForHiddenThinking(t *testing.T) {
	m := NewTestModel(nil)
	m.settings.verbosity = install.ChatVerbosityCompact
	started := time.Now().Add(-time.Minute)
	m.aiResponseTimer = aiTimerState{startedAt: started, displayed: time.Minute, running: true}
	m = m.appendStreamDelta(harness.StreamDelta{Kind: harness.StreamKindThinking, Text: "hidden"})
	if !m.aiResponseTimer.running || !m.aiResponseTimer.startedAt.After(started) || m.aiResponseTimer.displayed != 0 {
		t.Fatalf("hidden thinking did not reset AI rp: %+v", m.aiResponseTimer)
	}
}
