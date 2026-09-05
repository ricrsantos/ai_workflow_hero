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
	m = SetHeight(m, 40)
	m, _ = m.openSettings()
	plain := stripANSI(ViewForTest(m))
	for _, label := range []string{"Compact", "Standard", "Detailed", "Debug"} {
		if !strings.Contains(plain, label) {
			t.Fatalf("missing %q from settings: %q", label, plain)
		}
	}
	if !strings.Contains(plain, "CHAT VERBOSITY") || !strings.Contains(plain, "TELEGRAM PLUGIN") {
		t.Fatalf("missing section headers: %q", plain)
	}
	compactIdx := strings.Index(plain, "Compact")
	compactDescIdx := strings.Index(plain, "Responses, errors, and required approvals.")
	if compactIdx < 0 || compactDescIdx < compactIdx || strings.Count(plain[compactIdx:compactDescIdx], "\n") != 0 {
		t.Fatalf("Compact description must share the radio line: %q", plain)
	}
	if strings.Contains(plain, "○") || strings.Contains(plain, "•") {
		t.Fatalf("verbosity radios must not use circle glyphs: %q", plain)
	}
	if !strings.Contains(plain, "> Debug") {
		t.Fatalf("applied profile must use navbar caret: %q", plain)
	}
	if !strings.Contains(plain, "| Copy command |") {
		t.Fatalf("copy action must be a piped button: %q", plain)
	}
	next, _ := HandleTestKey(m, "up")
	if next.settings.cursor != len(verbosityOptions)-2 {
		t.Fatalf("cursor=%d want detailed index", next.settings.cursor)
	}
}

func TestSettingsEnterAppliesFocusedVerbosityNotCursorIndex(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 40)
	m, _ = m.openSettings()
	// Debug (index 3) → copy command → Compact (index 0).
	m, _ = HandleTestKey(m, "down")
	m, _ = HandleTestKey(m, "down")
	if m.settings.cursor != 0 {
		t.Fatalf("cursor=%d want compact", m.settings.cursor)
	}
	next, _ := HandleTestKey(m, "enter")
	if next.settings.verbosity != install.ChatVerbosityCompact {
		t.Fatalf("verbosity=%q want compact", next.settings.verbosity)
	}
}

func TestSettingsCopyCommandDoesNotChangeVerbosity(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 40)
	m, _ = m.openSettings()
	before := m.settings.verbosity
	m, _ = HandleTestKey(m, "down")
	if got := m.settingsRows()[m.settings.cursor].kind; got != rowTelegramCopyCommand {
		t.Fatalf("cursor kind=%d want copy command", got)
	}
	next, cmd := HandleTestKey(m, "enter")
	if next.settings.verbosity != before {
		t.Fatalf("verbosity changed to %q", next.settings.verbosity)
	}
	if cmd == nil {
		t.Fatal("copy command must return clipboard cmd")
	}
	if !strings.Contains(next.statusText, telegramInstallCommand) {
		t.Fatalf("status=%q", next.statusText)
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
