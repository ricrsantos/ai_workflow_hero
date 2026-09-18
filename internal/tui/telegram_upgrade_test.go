package tui

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/upgrade"
)

func TestParseTelegramHeroUpgrade(t *testing.T) {
	cases := []struct {
		text         string
		matched, val bool
	}{
		{"/hero-upgrade", true, true},
		{" /HERO-UPGRADE ", true, true},
		{"/hero-upgrade --yes", true, false},
		{"/hero-upgrade now", true, false},
		{"/hero-update", false, false},
		{"hero-upgrade", false, false},
		{"", false, false},
	}
	for _, tc := range cases {
		matched, valid := parseTelegramHeroUpgrade(tc.text)
		if matched != tc.matched || valid != tc.val {
			t.Fatalf("parseTelegramHeroUpgrade(%q)=(%v,%v) want (%v,%v)",
				tc.text, matched, valid, tc.matched, tc.val)
		}
	}
}

func heroUpgradeTestModel(projectDir, version string) model {
	m := NewTestModel(&cycle.Service{ProjectDir: projectDir})
	m.version = version
	m.telegram = &telegramState{
		connected: true,
		recordOutbound: func(text string) {
			// recordOutbound is replaced per-test where assertions need it.
		},
	}
	return m
}

func TestTelegramHeroUpgradeRejectsArgs(t *testing.T) {
	var outbound []string
	m := heroUpgradeTestModel(t.TempDir(), "3.4.0")
	m.telegram.recordOutbound = func(text string) { outbound = append(outbound, text) }

	next, cmd := m.handleTelegramInbound(telegramInboundMsg{text: "/hero-upgrade --yes", address: "proj"})
	m = next
	if cmd != nil {
		_ = cmd()
	}
	if len(outbound) != 1 || !strings.Contains(outbound[0], "Usage: /hero-upgrade") {
		t.Fatalf("outbound=%v", outbound)
	}
}

func TestTelegramHeroUpgradeRequiresProjectDir(t *testing.T) {
	var outbound []string
	m := NewTestModel(nil)
	m.version = "3.4.0"
	m.telegram = &telegramState{connected: true,
		recordOutbound: func(text string) { outbound = append(outbound, text) }}

	_, cmd := m.handleTelegramInbound(telegramInboundMsg{text: "/hero-upgrade", address: "proj"})
	if cmd != nil {
		_ = cmd()
	}
	if len(outbound) != 1 || !strings.Contains(outbound[0], "project directory is not set") {
		t.Fatalf("outbound=%v", outbound)
	}
}

func TestTelegramHeroUpgradeRequiresVersion(t *testing.T) {
	var outbound []string
	m := heroUpgradeTestModel(t.TempDir(), "")
	m.telegram.recordOutbound = func(text string) { outbound = append(outbound, text) }

	_, cmd := m.handleTelegramInbound(telegramInboundMsg{text: "/hero-upgrade", address: "proj"})
	if cmd != nil {
		_ = cmd()
	}
	if len(outbound) != 1 || !strings.Contains(outbound[0], "version is not set") {
		t.Fatalf("outbound=%v", outbound)
	}
}

func TestTelegramHeroUpgradeRefusesWhileStreaming(t *testing.T) {
	var outbound []string
	m := heroUpgradeTestModel(t.TempDir(), "3.4.0")
	m.streaming = true
	m.telegram.recordOutbound = func(text string) { outbound = append(outbound, text) }

	next, cmd := m.handleTelegramInbound(telegramInboundMsg{text: "/hero-upgrade", address: "proj"})
	_ = next
	if cmd != nil {
		_ = cmd()
	}
	if len(outbound) != 1 || !strings.Contains(outbound[0], "/interrupt") {
		t.Fatalf("outbound=%v", outbound)
	}
}

func TestTelegramHeroUpgradeBusy(t *testing.T) {
	var outbound []string
	m := heroUpgradeTestModel(t.TempDir(), "3.4.0")
	m.heroUpgradeBusy = true
	m.telegram.recordOutbound = func(text string) { outbound = append(outbound, text) }

	_, cmd := m.handleTelegramInbound(telegramInboundMsg{text: "/hero-upgrade", address: "proj"})
	if cmd != nil {
		_ = cmd()
	}
	if len(outbound) != 1 || !strings.Contains(outbound[0], "already running") {
		t.Fatalf("outbound=%v", outbound)
	}
}

func TestTelegramHeroUpgradeSuccessReportsCLIOutput(t *testing.T) {
	old := upgradeRunFunc
	upgradeRunFunc = func(opts upgrade.Options, stdout, stderr io.Writer) (upgrade.Result, error) {
		if opts.Version != "3.4.0" {
			t.Fatalf("version=%q", opts.Version)
		}
		_, _ = io.WriteString(stdout, "Updated: .cursor/commands/hero-start.md")
		return upgrade.Result{Updated: []string{".cursor/commands/hero-start.md"}}, nil
	}
	t.Cleanup(func() { upgradeRunFunc = old })

	var outbound []string
	m := heroUpgradeTestModel(t.TempDir(), "3.4.0")
	m.telegram.recordOutbound = func(text string) { outbound = append(outbound, text) }

	next, cmd := m.handleTelegramInbound(telegramInboundMsg{text: "/hero-upgrade", address: "proj"})
	m = next
	if !m.heroUpgradeBusy {
		t.Fatal("upgrade must mark the model busy while the worker runs")
	}
	// First outbound is the "started" notice recorded synchronously.
	if len(outbound) != 1 || !strings.Contains(outbound[0], "Hero upgrade started") {
		t.Fatalf("started outbound=%v", outbound)
	}
	// Run the background worker and apply its result message.
	msg := RunCmdForTest(cmd)
	res, ok := msg.(telegramHeroUpgradeResultMsg)
	if !ok {
		t.Fatalf("msg type %T", msg)
	}
	updated, cmd := m.Update(res)
	m = updated.(model)
	if cmd != nil {
		_ = cmd()
	}
	if m.heroUpgradeBusy {
		t.Fatal("busy flag must be cleared after the result")
	}
	if len(outbound) != 2 {
		t.Fatalf("outbound=%v", outbound)
	}
	got := outbound[1]
	for _, want := range []string{"Hero upgraded to version 3.4.0.", "Updated: 1 file(s).", "Updated: .cursor/commands/hero-start.md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("success outbound missing %q: %q", want, got)
		}
	}
}

func TestTelegramHeroUpgradeFailureReportsError(t *testing.T) {
	old := upgradeRunFunc
	upgradeRunFunc = func(opts upgrade.Options, stdout, stderr io.Writer) (upgrade.Result, error) {
		_, _ = io.WriteString(stderr, "partial output")
		return upgrade.Result{}, errors.New("boom")
	}
	t.Cleanup(func() { upgradeRunFunc = old })

	m := heroUpgradeTestModel(t.TempDir(), "3.4.0")
	m.telegram.recordOutbound = func(text string) {}

	next, cmd := m.handleTelegramInbound(telegramInboundMsg{text: "/hero-upgrade", address: "proj"})
	m = next
	msg := RunCmdForTest(cmd)
	res, ok := msg.(telegramHeroUpgradeResultMsg)
	if !ok {
		t.Fatalf("msg type %T", msg)
	}
	var outbound []string
	m.telegram.recordOutbound = func(text string) { outbound = append(outbound, text) }
	updated, cmd := m.Update(res)
	m = updated.(model)
	if cmd != nil {
		_ = cmd()
	}
	if len(outbound) != 1 {
		t.Fatalf("outbound=%v", outbound)
	}
	if !strings.Contains(outbound[0], "Hero upgrade failed: boom") || !strings.Contains(outbound[0], "partial output") {
		t.Fatalf("failure outbound=%q", outbound[0])
	}
}

func TestTelegramHeroUpgradeDoesNotStartHarnessTurn(t *testing.T) {
	old := upgradeRunFunc
	upgradeRunFunc = func(opts upgrade.Options, stdout, stderr io.Writer) (upgrade.Result, error) {
		return upgrade.Result{}, nil
	}
	t.Cleanup(func() { upgradeRunFunc = old })

	m := heroUpgradeTestModel(t.TempDir(), "3.4.0")
	next, _ := m.handleTelegramInbound(telegramInboundMsg{text: "/hero-upgrade", address: "proj"})
	if next.streaming {
		t.Fatal("/hero-upgrade must not start a harness turn")
	}
}
