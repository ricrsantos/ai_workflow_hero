package tui

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

func TestTelegramOriginLabel(t *testing.T) {
	if got, ok := telegramOriginLabel(convMessage{role: convRoleUser, origin: "telegram:ai_workflow_2"}); !ok || got != "← [Telegram · ai_workflow_2]" {
		t.Fatalf("user label=%q ok=%v", got, ok)
	}
	if got, ok := telegramOriginLabel(convMessage{role: convRoleAgent, origin: "telegram:ai_workflow_2"}); !ok || got != "→ [Telegram · ai_workflow_2]" {
		t.Fatalf("agent label=%q ok=%v", got, ok)
	}
	if _, ok := telegramOriginLabel(convMessage{role: convRoleUser}); ok {
		t.Fatal("local user message must not carry a Telegram label")
	}
	if _, ok := telegramOriginLabel(convMessage{role: convRoleAgent, origin: "telegram:"}); !ok {
		t.Fatal("agent with empty address should still be labelled")
	}
}

func TestFormatTelegramEventFiltersToLifecycleOnly(t *testing.T) {
	cases := []struct {
		kind    conversation.EventKind
		nonZero bool
	}{
		{conversation.EventCycleStarted, true},
		{conversation.EventCycleFinished, true},
		{conversation.EventStageStarted, true},
		{conversation.EventStageFinished, true},
		{conversation.EventApprovalRequired, true},
		{conversation.EventError, true},
		{conversation.EventFinalResult, true},
	}
	for _, c := range cases {
		got := formatTelegramEvent(conversation.Event{Kind: c.kind, CycleID: 1, StageName: "qa", Message: "x"})
		if (got != "") != c.nonZero {
			t.Errorf("kind %s: got=%q nonZero=%v", c.kind, got, c.nonZero)
		}
	}
	// Unknown kinds (e.g. stream/tool) produce no outbound text.
	if got := formatTelegramEvent(conversation.Event{Kind: "tool_call", Message: "noise"}); got != "" {
		t.Errorf("tool event must not notify, got %q", got)
	}
}

func TestProjectAbbrevNormalization(t *testing.T) {
	if got := projectAbbrev("/home/u/AI Workflow Hero!"); got != "aiworkflowhero" {
		t.Fatalf("abbrev=%q", got)
	}
	if got := normalizeTelegramAbbrev(""); got != "proj" {
		t.Fatalf("empty abbrev default=%q", got)
	}
	if got := normalizeTelegramAbbrev("My-Proj_2"); got != "my-proj_2" {
		t.Fatalf("abbrev=%q", got)
	}
}

func TestLoadTelegramAbbrevPrefersHeroJSON(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"cli":{},"assets":{},"telegram":{"project_abbrev":"aiwkhero"}}`
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadTelegramAbbrev(dir); got != "aiwkhero" {
		t.Fatalf("abbrev=%q want aiwkhero (not directory name %q)", got, projectAbbrev(dir))
	}
}

func TestLoadTelegramAbbrevFallsBackToDirectoryName(t *testing.T) {
	dir := t.TempDir()
	if got := loadTelegramAbbrev(dir); got != projectAbbrev(dir) {
		t.Fatalf("abbrev=%q want directory fallback %q", got, projectAbbrev(dir))
	}
}

func TestLoadTelegramAlwaysSend(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"cli":{},"assets":{},"telegram":{"always_send":true}}`
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if !loadTelegramAlwaysSend(dir) {
		t.Fatal("always_send=false want true")
	}
}

func TestCommitTelegramAbbrevWritesHeroJSON(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".workflow-hero", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "hero.json"), []byte(`{"cli":{},"assets":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := model{
		svc:      &cycle.Service{ProjectDir: dir},
		telegram: &telegramState{installed: true, abbrev: projectAbbrev(dir)},
	}
	m.settings.editingAbbrev = true
	m.settings.abbrevDraft = "aiwkhero"
	m, cmd := m.commitTelegramAbbrev()
	if m.telegram.abbrev != "aiwkhero" {
		t.Fatalf("in-memory abbrev=%q", m.telegram.abbrev)
	}
	if cmd == nil {
		t.Fatal("expected persist command")
	}
	msg := cmd()
	saved, ok := msg.(telegramAbbrevSavedMsg)
	if !ok {
		t.Fatalf("msg type %T", msg)
	}
	if saved.err != nil {
		t.Fatal(saved.err)
	}
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hero.Telegram.ProjectAbbrev != "aiwkhero" {
		t.Fatalf("hero.json project_abbrev=%q", hero.Telegram.ProjectAbbrev)
	}
	if got := loadTelegramAbbrev(dir); got != "aiwkhero" {
		t.Fatalf("reload=%q", got)
	}
}

func TestTelegramStatusText(t *testing.T) {
	now := time.Now()
	m := NewTestModel(nil)
	m.status = cycle.StatusView{
		CycleNumber: 7,
		Title:       "Telegram status",
		Objective:   "Verify remote status",
		Status:      "active",
		Stages:      []cycle.StatusStage{{Name: "Implementation", Status: "Running", Iteration: "1/3"}},
	}
	m.sessionTimer = sessionTimerState{startedAt: now.Add(-2 * time.Minute), running: true}
	m.aiTimer = aiTimerState{startedAt: now.Add(-time.Minute), running: true}
	m.aiResponseTimer = aiTimerState{startedAt: now.Add(-30 * time.Second), running: true}
	m.contextUsedTokens = 100000
	m.chatModelSlug = "test-model"
	m.contextWindows = contextWindowCatalog{"test-model": 250000, "worker-model": 250000}
	m.liveAgents = []liveAgent{
		{Name: "orchestration_agent", Model: "orchestrator-model", Harness: "cursor"},
		{Name: "generic_agent", Model: "worker-model", Harness: "opencode"},
	}

	got := m.telegramStatusText(now)
	for _, want := range []string{
		"Cycle C7: Telegram status",
		"Current stage: Implementation (Running, iteration 1/3)",
		"Agents:\n- orchestration_agent: orchestrator-model\n- generic_agent: worker-model",
		"Session: 00:02:00",
		"AI wk: 00:01:00",
		"AI rp: 00:00:30",
		"Context: 100k/250k",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "Objective:") {
		t.Fatalf("cycle status must not include the objective summary: %q", got)
	}

	m.status = cycle.StatusView{}
	m.streaming = true
	m.liveAgents = []liveAgent{{Model: "free-chat-model", Harness: "cursor"}}
	got = m.telegramStatusText(now)
	if !strings.HasPrefix(got, "Waiting for harness\nAgents:\n- harness: free-chat-model\n") {
		t.Fatalf("free-chat status=%q", got)
	}
	m.liveAgents = nil
	m.chatModelSlug = "fallback-free-model"
	got = m.telegramStatusText(now)
	if !strings.Contains(got, "- harness: fallback-free-model") {
		t.Fatalf("free-chat fallback status=%q", got)
	}
	m.streaming = false
	m.chatModelSlug = "test-model"
	got = m.telegramStatusText(now)
	for _, want := range []string{
		"idle",
		"Model: test-model",
		"Session: 00:02:00",
		"AI wk: 00:01:00",
		"AI rp: 00:00:30",
		"Context: 100k/250k",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("idle status missing %q: %q", want, got)
		}
	}
}

func TestTelegramIdleStatusUsesSelectedModelContextWindow(t *testing.T) {
	now := time.Now()
	m := NewTestModel(nil)
	m.chatModelSlug = "selected-model"
	m.contextUsedTokens = 100000
	m.contextWindows = contextWindowCatalog{
		"selected-model":       250000,
		"previous-agent-model": 500000,
	}
	m.transcript = []convMessage{{
		role:      convRoleAgent,
		content:   "previous response",
		modelSlug: "previous-agent-model",
	}}

	got := m.telegramStatusText(now)
	if !strings.Contains(got, "Model: selected-model") {
		t.Fatalf("idle status model=%q", got)
	}
	if !strings.Contains(got, "Context: 100k/250k") {
		t.Fatalf("idle status must use selected model window=%q", got)
	}
}

func TestTelegramStatusCommandAllowsIdleButAutoReportSkipsIt(t *testing.T) {
	now := time.Now()
	var outbound []string
	m := NewTestModel(nil)
	m.telegram = &telegramState{
		installed:         true,
		connected:         true,
		paired:            true,
		autoReportMinutes: 1,
		nextAutoReportAt:  now.Add(-time.Second),
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}

	next, cmd := m.handleTelegramInbound(telegramInboundMsg{text: telegramStatusCommand, isCommand: true, address: "proj"})
	if cmd != nil {
		_ = cmd()
	}
	if len(outbound) != 1 || !strings.Contains(outbound[0], "idle\nModel: not set") {
		t.Fatalf("/status outbound=%v", outbound)
	}

	cmd = next.maybeTelegramAutoReport(now)
	if cmd != nil {
		_ = cmd()
	}
	if len(outbound) != 1 {
		t.Fatalf("auto-report outbound=%v", outbound)
	}
	if !next.telegram.nextAutoReportAt.After(now) {
		t.Fatal("auto report schedule was not advanced")
	}
}

func TestTelegramInterruptCancelsActiveConversation(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	outbound := []string{}
	streamOut := make(chan tea.Msg, 1)
	relay := newConversationStreamRelay("ex-1", streamOut)
	m.streaming = true
	m.harnessSessionID = "telegram-session"
	m.agentMsgIndex = 0
	m.transcript = []convMessage{{role: convRoleAgent, content: "partial"}}
	m.executes = map[string]convExecute{
		"ex-1": {
			ID:            "ex-1",
			HarnessID:     "cursor",
			SessionID:     "telegram-session",
			AgentMsgIndex: 0,
			relay:         relay,
		},
	}
	m.telegram = &telegramState{
		connected: true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}

	next, cmd := m.handleTelegramInbound(telegramInboundMsg{
		text:    telegramInterruptCommand,
		address: "proj",
	})
	if !next.streaming {
		t.Fatal("interrupt request must leave cancellation in flight until the command completes")
	}
	if len(outbound) != 1 || outbound[0] != "Interrupt requested." {
		t.Fatalf("interrupt outbound=%v", outbound)
	}
	if cmd == nil {
		t.Fatal("interrupt must schedule harness cancellation")
	}

	msg := cmd()
	updated, _ := next.Update(msg)
	final := updated.(model)
	if final.streaming {
		t.Fatal("interrupt must stop the active conversation")
	}
	if !h.CancelCalled() {
		t.Fatal("interrupt must call the active harness Cancel")
	}
	if !final.transcript[0].interrupted {
		t.Fatal("interrupt must mark the active response as interrupted")
	}
}

func TestTelegramInterruptWithoutActiveProcessDoesNotStartHarnessTurn(t *testing.T) {
	outbound := []string{}
	m := NewTestModel(nil)
	m.telegram = &telegramState{
		connected: true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}

	next, cmd := m.handleTelegramInbound(telegramInboundMsg{text: telegramInterruptCommand, address: "proj"})
	if cmd != nil {
		_ = cmd()
	}
	if next.streaming {
		t.Fatal("interrupt without an active process must not start streaming")
	}
	if len(outbound) != 1 || outbound[0] != "No process is running." {
		t.Fatalf("idle interrupt outbound=%v", outbound)
	}
}

func TestTelegramHelpReturnsCommandCatalog(t *testing.T) {
	outbound := []string{}
	m := NewTestModel(nil)
	m.telegram = &telegramState{
		connected: true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}

	next, cmd := m.handleTelegramInbound(telegramInboundMsg{text: "/help", address: "proj"})
	if cmd != nil {
		_ = cmd()
	}
	if next.streaming {
		t.Fatal("/help must not start a harness turn")
	}
	if len(outbound) != 1 || !strings.Contains(outbound[0], "Hero Telegram commands") {
		t.Fatalf("/help outbound=%v", outbound)
	}
	if !strings.Contains(outbound[0], "/status") || !strings.Contains(outbound[0], "/kill") {
		t.Fatalf("/help missing commands: %q", outbound[0])
	}
}

func TestTelegramAutoUpdateCommandIsExact(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"/auto-update", true},
		{" /AUTO-UPDATE ", true},
		{"/auto-update now", false},
		{"auto-update", false},
	} {
		if got := isTelegramAutoUpdateCommand(tc.text); got != tc.want {
			t.Fatalf("isTelegramAutoUpdateCommand(%q)=%v want %v", tc.text, got, tc.want)
		}
	}
}

func TestTelegramUpdateRestartEventRequestsTUIRestart(t *testing.T) {
	m := NewTestModel(nil)
	m.telegram = &telegramState{connected: true, paired: true}
	next, cmd := m.handleTelegramMsg(telegramEventMsg{eventType: ipc.EventUpdateRestart})
	updated := next.(model)
	if !updated.restartRequested {
		t.Fatal("update restart event must mark the model for restart")
	}
	if cmd == nil {
		t.Fatal("update restart event must quit the TUI")
	}
}

func TestIsTelegramKillCommand(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"/kill", true},
		{" /Kill ", true},
		{"/KILL", true},
		{"/interrupt", false},
		{"/kill now", false},
		{"kill", false},
	}
	for _, tc := range cases {
		if got := isTelegramKillCommand(tc.text); got != tc.want {
			t.Fatalf("isTelegramKillCommand(%q)=%v want %v", tc.text, got, tc.want)
		}
	}
}

func TestTelegramKillInboundForceKillsWithoutHarnessTurn(t *testing.T) {
	old := telegramForceKillProcess
	killed := false
	telegramForceKillProcess = func() { killed = true }
	t.Cleanup(func() { telegramForceKillProcess = old })

	outbound := []string{}
	m := NewTestModel(nil)
	m.streaming = true
	m.telegram = &telegramState{
		connected: true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}

	next, cmd := m.handleTelegramInbound(telegramInboundMsg{
		text:      telegramKillCommand,
		inboundID: "in-kill",
		address:   "proj",
	})
	if cmd != nil {
		t.Fatal("kill must not schedule Bubble Tea cmds that could hang")
	}
	if !killed {
		t.Fatal("kill must force-kill the TUI process")
	}
	if len(outbound) != 1 || outbound[0] != telegramKillOutboundText {
		t.Fatalf("kill outbound=%v", outbound)
	}
	if !next.streaming {
		t.Fatal("kill must not run the interrupt/cancel path")
	}
}

func TestTelegramClientApplyKillAcksAndOutboundsBeforeForceKill(t *testing.T) {
	old := telegramForceKillProcess
	killed := false
	telegramForceKillProcess = func() { killed = true }
	t.Cleanup(func() { telegramForceKillProcess = old })

	server, client := net.Pipe()
	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
	})

	recvDone := make(chan []ipc.Message, 1)
	recvErr := make(chan error, 1)
	go func() {
		pc := ipc.NewConn(server)
		var got []ipc.Message
		for i := 0; i < 2; i++ {
			m, err := pc.Recv()
			if err != nil {
				recvErr <- err
				return
			}
			got = append(got, m)
		}
		recvDone <- got
	}()

	c := &telegramClient{conn: ipc.NewConn(client)}
	c.applyTelegramKill("in-42")
	if !killed {
		t.Fatal("client kill must force-kill after sending frames")
	}

	select {
	case msgs := <-recvDone:
		if len(msgs) != 2 {
			t.Fatalf("frames=%d want 2", len(msgs))
		}
		if msgs[0].Type != ipc.TypeAckDelivery || msgs[0].AckID != "in-42" {
			t.Fatalf("ack frame=%+v", msgs[0])
		}
		if msgs[1].Type != ipc.TypeOutbound || msgs[1].OutboundText != telegramKillOutboundText {
			t.Fatalf("outbound frame=%+v", msgs[1])
		}
	case err := <-recvErr:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for kill frames")
	}
}

func TestTelegramAutoReportSendsNonIdleStatusOncePerInterval(t *testing.T) {
	now := time.Now()
	var outbound []string
	m := NewTestModel(nil)
	m.status = cycle.StatusView{CycleNumber: 3, Title: "Active cycle", Status: "active"}
	m.telegram = &telegramState{
		installed:         true,
		connected:         true,
		paired:            true,
		autoReportMinutes: 1,
		nextAutoReportAt:  now.Add(-time.Second),
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}

	cmd := m.maybeTelegramAutoReport(now)
	if cmd != nil {
		_ = cmd()
	}
	if len(outbound) != 1 || !strings.HasPrefix(outbound[0], "Cycle C3: Active cycle") {
		t.Fatalf("active auto-report outbound=%q", outbound)
	}

	if cmd := m.maybeTelegramAutoReport(now); cmd != nil {
		t.Fatal("same interval must not schedule a second auto report")
	}
}

func TestTelegramQueuedTurnSendsStatusOnceWhileStreaming(t *testing.T) {
	var outbound []string
	m := NewTestModel(nil)
	m.status = cycle.StatusView{CycleNumber: 3, Title: "Active cycle", Status: "active"}
	m.streaming = true
	m.telegram = &telegramState{
		connected: true,
		paired:    true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}

	inbound := telegramInboundMsg{text: "follow up", address: "proj"}
	next, _ := m.handleTelegramInbound(inbound)
	if len(outbound) != 1 || !strings.HasPrefix(outbound[0], "Cycle C3:") {
		t.Fatalf("queued status=%v", outbound)
	}
	if len(next.telegramPendingTurns) != 1 {
		t.Fatalf("queue=%d want 1", len(next.telegramPendingTurns))
	}

	for i := 0; i < 10; i++ {
		next, _ = next.handleTelegramInbound(inbound)
	}
	if len(outbound) != 1 {
		t.Fatalf("retry flood outbound=%d want 1", len(outbound))
	}
	if len(next.telegramPendingTurns) != 1 {
		t.Fatalf("dedup queue=%d want 1", len(next.telegramPendingTurns))
	}

	next, _ = next.handleTelegramInbound(telegramInboundMsg{text: "another", address: "proj"})
	if len(outbound) != 2 {
		t.Fatalf("second unique queued status=%d want 2", len(outbound))
	}
	if len(next.telegramPendingTurns) != 2 {
		t.Fatalf("queue=%d want 2", len(next.telegramPendingTurns))
	}
}

func TestTelegramQueuedTurnStartsAfterExecuteCompletes(t *testing.T) {
	m := withDefaultChatModel(NewTestModel(nil))
	m.status = cycle.StatusView{CycleNumber: 3, Title: "Active cycle", Status: "active"}
	m.streaming = true
	m.executes = map[string]convExecute{"ex-1": {ID: "ex-1"}}
	m.telegram = &telegramState{connected: true, paired: true}

	next, _ := m.handleTelegramInbound(telegramInboundMsg{text: "follow up", address: "proj"})
	if len(next.telegramPendingTurns) != 1 {
		t.Fatal("expected queued turn")
	}

	updated, _ := next.Update(executeDoneMsg{
		executeID: "ex-1",
		result:    &harness.ExecutionResult{Output: "done"},
	})
	final := updated.(model)
	if len(final.telegramPendingTurns) != 0 {
		t.Fatalf("queue not drained: %d", len(final.telegramPendingTurns))
	}
	if !final.streaming {
		t.Fatal("drained turn must start a harness execute")
	}
	found := false
	for _, msg := range final.transcript {
		if msg.role == convRoleUser && strings.Contains(msg.content, "follow up") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("drained turn missing from transcript: %+v", final.transcript)
	}
}

func TestTelegramAutoReportStaleTickDoesNotFlood(t *testing.T) {
	now := time.Now()
	var outbound []string
	m := NewTestModel(nil)
	m.status = cycle.StatusView{CycleNumber: 3, Title: "Active cycle", Status: "active"}
	m.timerGeneration = 1
	m.timerLoopStarted = true
	m.telegram = &telegramState{
		connected:         true,
		paired:            true,
		autoReportMinutes: 1,
		nextAutoReportAt:  now.Add(-2 * time.Minute),
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}

	stale := now.Add(-2 * time.Minute)
	m, _ = m.handleTimerTick(timerTickMsg{at: stale, generation: 1})
	if len(outbound) != 1 {
		t.Fatalf("stale tick outbound=%d want 1", len(outbound))
	}

	m, _ = m.handleTimerTick(timerTickMsg{at: stale.Add(time.Second), generation: 1})
	if len(outbound) != 1 {
		t.Fatalf("second stale tick flooded outbound=%d", len(outbound))
	}

	for i := 0; i < 20; i++ {
		m, _ = m.handleTimerTick(timerTickMsg{at: now, generation: 1})
	}
	if len(outbound) != 1 {
		t.Fatalf("same-generation burst flooded outbound=%d", len(outbound))
	}

	m, _ = m.handleTimerTick(timerTickMsg{at: now, generation: 0})
	if len(outbound) != 1 {
		t.Fatalf("generation 0 tick sent status outbound=%d", len(outbound))
	}
}

func TestSettingsRows_NotInstalledShowsGuidance(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 40)
	rows := m.settingsRows()
	if len(rows) != len(verbosityOptions)+1 {
		t.Fatalf("rows=%d", len(rows))
	}
	last := rows[len(rows)-1]
	if last.kind != rowTelegramCopyCommand {
		t.Fatalf("expected copy-command row, got %d", last.kind)
	}
	if !strings.Contains(last.desc, telegramInstallCommand) {
		t.Fatalf("guidance missing install command: %q", last.desc)
	}
	m, _ = m.openSettings()
	plain := stripANSI(ViewForTest(m))
	if !strings.Contains(plain, "Not installed") {
		t.Fatalf("missing not-installed badge: %q", plain)
	}
	if !strings.Contains(plain, telegramInstallCommand) {
		t.Fatalf("missing install command box: %q", plain)
	}
	if strings.Contains(plain, "› Telegram") || strings.Contains(plain, "> Telegram") {
		t.Fatalf("install guidance must not look like a verbosity radio: %q", plain)
	}
}

func TestSettingsRows_InstalledShowsControls(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 40)
	m.telegram = &telegramState{installed: true, pluginVersion: "2.9.2", protocolVersion: 1, connected: true, address: "ai_workflow_2", abbrev: "ai_workflow"}
	rows := m.settingsRows()
	kinds := map[settingsRowKind]bool{}
	for _, r := range rows {
		kinds[r.kind] = true
	}
	if kinds[rowTelegramCopyCommand] || !kinds[rowTelegramAbbrev] || !kinds[rowTelegramAction] {
		t.Fatalf("installed rows=%+v", kinds)
	}
	for _, r := range rows {
		for _, secret := range []string{"token", "chat_id"} {
			if strings.Contains(strings.ToLower(r.desc), secret) {
				t.Fatalf("row %q leaks secret word %q", r.desc, secret)
			}
		}
	}
	m, _ = m.openSettings()
	plain := stripANSI(ViewForTest(m))
	for _, want := range []string{"Not configured", "Installed · v2.9.2", "Connected", "Project ID", "Auto report: Disabled", "Always send: Disabled", "Pair"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q: %q", want, plain)
		}
	}
	// Down from Debug lands on Project ID, then Auto report, Always send, then Pair — never a status badge.
	m, _ = HandleTestKey(m, "down")
	if got := m.settingsRows()[m.settings.cursor].kind; got != rowTelegramAbbrev {
		t.Fatalf("after debug, cursor kind=%d want Project ID", got)
	}
	m, _ = HandleTestKey(m, "down")
	if got := m.settingsRows()[m.settings.cursor].kind; got != rowTelegramAutoReport {
		t.Fatalf("after project ID, cursor kind=%d want Auto report", got)
	}
	m, _ = HandleTestKey(m, "down")
	if got := m.settingsRows()[m.settings.cursor].kind; got != rowTelegramAlwaysSend {
		t.Fatalf("after auto report, cursor kind=%d want Always send", got)
	}
	m, _ = HandleTestKey(m, "down")
	if got := m.settingsRows()[m.settings.cursor]; got.kind != rowTelegramAction || got.action != "pair" {
		t.Fatalf("cursor=%+v want Pair", got)
	}
}

func TestPairEnterOpensInstructionModal(t *testing.T) {
	m := settingsFocusedOnPair(t, true)
	m, _ = HandleTestKey(m, "enter")
	if m.telegram == nil || !m.telegram.pairing {
		t.Fatal("Pair must open the pairing modal")
	}
	if m.telegram.pairState != "waiting" {
		t.Fatalf("pairState=%q want waiting so pairing starts immediately", m.telegram.pairState)
	}
	plain := stripANSI(ViewForTest(m))
	for _, want := range []string{
		"Pair Telegram",
		"Open the configured Telegram bot",
		"Waiting for a pairing code",
		"pairing will complete automatically",
		"Waiting for confirmation",
		"[Cancel]",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in pairing modal: %q", want, plain)
		}
	}
	if strings.Contains(plain, "Token:") {
		t.Fatalf("Pair must not start on the token form: %q", plain)
	}
}

func TestDisconnectedSettingsShowsRetryNotPair(t *testing.T) {
	m := settingsFocusedOnRetry(t)
	plain := stripANSI(ViewForTest(m))
	if !strings.Contains(plain, "Retry") {
		t.Fatalf("disconnected Settings must offer Retry: %q", plain)
	}
	if !strings.Contains(plain, "Start the daemon with Retry") {
		t.Fatalf("missing recovery guidance: %q", plain)
	}
	if strings.Contains(plain, "| Pair |") {
		t.Fatalf("Pair must wait until the daemon is connected: %q", plain)
	}
	m, _ = HandleTestKey(m, "enter")
	if !m.telegram.retrying {
		t.Fatal("Retry must mark the daemon as retrying")
	}
	if m.telegram.pairing {
		t.Fatal("Retry must not open the pairing modal")
	}
}

func TestTelegramConnectedEnablesPair(t *testing.T) {
	m := settingsFocusedOnRetry(t)
	next, _ := m.Update(telegramConnectedMsg{})
	m = next.(model)
	if !m.telegram.connected {
		t.Fatal("telegramConnectedMsg must set connected")
	}
	plain := stripANSI(ViewForTest(m))
	if !strings.Contains(plain, "Connected") || !strings.Contains(plain, "Pair") {
		t.Fatalf("connected Settings must show Pair: %q", plain)
	}
}

func TestTelegramTurnReplyTextOnlyForCompletedTelegramTurns(t *testing.T) {
	if got := telegramTurnReplyText("telegram:aiwkhero", "Estou no repositório", "", false); got != "" {
		t.Fatalf("incomplete turn must not reply: %q", got)
	}
	if got := telegramTurnReplyText("", "Estou no repositório", "", true); got != "" {
		t.Fatalf("local turn must not reply: %q", got)
	}
	if got := telegramTurnReplyText("telegram:aiwkhero", "Estou no repositório", "ignored", true); got != "Estou no repositório" {
		t.Fatalf("got %q", got)
	}
	if got := telegramTurnReplyText("telegram:aiwkhero", "", "boom", true); got != "boom" {
		t.Fatalf("error reply=%q", got)
	}
}

func TestExecuteDoneSendsTelegramConversationReply(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 40)
	var got []string
	m.telegram = &telegramState{
		installed: true,
		connected: true,
		recordOutbound: func(text string) {
			got = append(got, text)
		},
	}
	m.streaming = true
	m.agentMsgIndex = 1
	m.transcript = []convMessage{
		{role: convRoleUser, content: "em que projeto vc está?", origin: "telegram:aiwkhero"},
		{role: convRoleAgent, content: "", origin: "telegram:aiwkhero"},
	}
	m.executes = map[string]convExecute{
		"ex-1": {ID: "ex-1", Origin: "telegram:aiwkhero", AgentMsgIndex: 1},
	}
	next, _ := m.Update(executeDoneMsg{
		executeID: "ex-1",
		result:    &harness.ExecutionResult{Output: "Estou no repositório AI Workflow Hero"},
	})
	m = next.(model)
	if len(got) != 1 || got[0] != "Estou no repositório AI Workflow Hero" {
		t.Fatalf("outbound=%q", got)
	}
}

func TestExecuteDoneInConversationBatchSendsTelegramReply(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 40)
	var got []string
	m.telegram = &telegramState{
		installed: true,
		connected: true,
		recordOutbound: func(text string) {
			got = append(got, text)
		},
	}
	m.streaming = true
	m.agentMsgIndex = 1
	m.transcript = []convMessage{
		{role: convRoleUser, content: "em que projeto vc está?", origin: "telegram:aiwkhero"},
		{role: convRoleAgent, content: "", origin: "telegram:aiwkhero"},
	}
	m.executes = map[string]convExecute{
		"ex-1": {ID: "ex-1", Origin: "telegram:aiwkhero", AgentMsgIndex: 1},
	}
	next, cmd := m.Update(conversationBatchMsg{messages: []tea.Msg{
		executeDoneMsg{
			executeID: "ex-1",
			result:    &harness.ExecutionResult{Output: "Estou no repositório AI Workflow Hero"},
		},
	}})
	_ = next
	if len(got) != 1 || got[0] != "Estou no repositório AI Workflow Hero" {
		t.Fatalf("batch outbound=%q", got)
	}
	if cmd == nil {
		t.Fatal("batch must keep the Telegram outbound command")
	}
}

func TestExecuteDoneLocalTurnDoesNotSendTelegramReply(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 40)
	var got []string
	m.telegram = &telegramState{
		installed: true,
		connected: true,
		recordOutbound: func(text string) {
			got = append(got, text)
		},
	}
	m.streaming = true
	m.agentMsgIndex = 1
	m.transcript = []convMessage{
		{role: convRoleUser, content: "hello"},
		{role: convRoleAgent, content: ""},
	}
	m.executes = map[string]convExecute{
		"ex-1": {ID: "ex-1", AgentMsgIndex: 1},
	}
	next, _ := m.Update(executeDoneMsg{
		executeID: "ex-1",
		result:    &harness.ExecutionResult{Output: "local only"},
	})
	_ = next
	if len(got) != 0 {
		t.Fatalf("local turn leaked to Telegram: %q", got)
	}
}

func TestExecuteDoneLocalTurnSendsTelegramReplyWhenAlwaysSendEnabled(t *testing.T) {
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 40)
	var got []string
	m.telegram = &telegramState{
		installed:  true,
		connected:  true,
		alwaysSend: true,
		recordOutbound: func(text string) {
			got = append(got, text)
		},
	}
	m.streaming = true
	m.agentMsgIndex = 1
	m.transcript = []convMessage{
		{role: convRoleUser, content: "hello"},
		{role: convRoleAgent, content: ""},
	}
	m.executes = map[string]convExecute{
		"ex-1": {ID: "ex-1", AgentMsgIndex: 1},
	}
	next, _ := m.Update(executeDoneMsg{
		executeID: "ex-1",
		result:    &harness.ExecutionResult{Output: "local turn forwarded"},
	})
	_ = next
	if len(got) != 1 || got[0] != "local turn forwarded" {
		t.Fatalf("outbound=%q", got)
	}
}

func TestTelegramListenCmdDeliversBufferedFrames(t *testing.T) {
	m := NewTestModel(nil)
	ch := make(chan tea.Msg, 1)
	m.telegramMsgCh = ch
	ch <- telegramConnectedMsg{}
	cmd := m.telegramListenCmd()
	if cmd == nil {
		t.Fatal("listener cmd must be issued when the channel exists")
	}
	msg := cmd()
	if _, ok := msg.(telegramConnectedMsg); !ok {
		t.Fatalf("got %T, want telegramConnectedMsg", msg)
	}
}

func TestTelegramModelSelectionUsesNumberedRemoteWizard(t *testing.T) {
	m, dir := newPickerTestModel(t)
	var outbound []string
	m.telegram = &telegramState{
		installed: true,
		connected: true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}

	// /model from Telegram must not open the local palette.
	next, _ := m.Update(telegramInboundMsg{text: "/model", isCommand: true, address: "proj"})
	m = next.(model)
	if m.screen == screenPalette {
		t.Fatal("Telegram /model must not open the local TUI picker")
	}
	if len(outbound) != 1 || !strings.Contains(outbound[0], "Escolha o Harness:") || !strings.Contains(outbound[0], "1 - Cursor") {
		t.Fatalf("harness prompt=%q", outbound)
	}

	// Cursor, full/model, then fs=true, th=max, ef=high.
	prev := listModelsForHarnessFn
	listModelsForHarnessFn = func(_ context.Context, _ model, harnessID string) ([]string, error) {
		return []string{"full/model", "partial/model", "pricing-only"}, nil
	}
	t.Cleanup(func() { listModelsForHarnessFn = prev })

	next, cmd := m.Update(telegramInboundMsg{text: "1", address: "proj"})
	m = flushTeaCmds(next.(model), cmd)
	if !strings.Contains(outbound[len(outbound)-1], "Escolha o modelo:") {
		t.Fatalf("model prompt=%q", outbound[len(outbound)-1])
	}
	next, _ = m.Update(telegramInboundMsg{text: "1", address: "proj"})
	m = next.(model)
	if !strings.Contains(outbound[len(outbound)-1], "Fast Mode:") {
		t.Fatalf("fast prompt=%q", outbound[len(outbound)-1])
	}
	next, _ = m.Update(telegramInboundMsg{text: "1", address: "proj"})
	m = next.(model)
	if !strings.Contains(outbound[len(outbound)-1], "Thinking:") {
		t.Fatalf("thinking prompt=%q", outbound[len(outbound)-1])
	}
	next, _ = m.Update(telegramInboundMsg{text: "2", address: "proj"})
	m = next.(model)
	if !strings.Contains(outbound[len(outbound)-1], "Reasoning effort:") {
		t.Fatalf("effort prompt=%q", outbound[len(outbound)-1])
	}
	next, _ = m.Update(telegramInboundMsg{text: "3", address: "proj"})
	m = next.(model)
	if got := outbound[len(outbound)-1]; !strings.Contains(got, "Modelo selecionado: full/model · Cursor") {
		t.Fatalf("completion=%q", got)
	}
	if m.telegram.modelSelection != nil {
		t.Fatal("selection state must be cleared after save")
	}
	hero, err := install.LoadHeroJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	harnessID, modelID := install.GetFreechatDefault(hero)
	if harnessID != "cursor" || modelID != "full/model" {
		t.Fatalf("freechat pair=%s/%s", harnessID, modelID)
	}
	props := install.EffectivePairProperties(hero, "cursor", "full/model")
	if props["fs"] != "true" || props["th"] != "max" || props["ef"] != "high" {
		t.Fatalf("saved properties=%v", props)
	}
}

func TestPairingProgressShowsStartCode(t *testing.T) {
	m := settingsFocusedOnPair(t, true)
	m, _ = HandleTestKey(m, "enter")
	next, _ := m.Update(telegramEventMsg{eventType: "pairing_progress", data: "428391"})
	m = next.(model)
	plain := stripANSI(ViewForTest(m))
	if !strings.Contains(plain, "Send: /start 428391") {
		t.Fatalf("expected pairing code instructions, got %q", plain)
	}
	if !strings.Contains(plain, "Code expires in") {
		t.Fatalf("expected countdown, got %q", plain)
	}
}

func TestPairingEscCancelsEvenWithNavbarFocus(t *testing.T) {
	m := settingsFocusedOnPair(t, true)
	m, _ = HandleTestKey(m, "enter")
	m.shellFocus = shellFocusNavbar
	m, _ = HandleTestKey(m, "esc")
	if m.telegram.pairing {
		t.Fatal("esc must close the pairing modal even if the navbar had focus")
	}
	plain := stripANSI(ViewForTest(m))
	if strings.Contains(plain, "Pair Telegram") {
		t.Fatalf("modal still visible after esc: %q", plain)
	}
}

func TestPairingMissingTokenAsksWithoutEchoingSecret(t *testing.T) {
	m := settingsFocusedOnPair(t, true)
	m, _ = HandleTestKey(m, "enter")
	next, _ := m.Update(telegramEventMsg{eventType: "pairing_progress", data: "missing-token"})
	m = next.(model)
	plain := stripANSI(ViewForTest(m))
	if !strings.Contains(plain, "Token:") {
		t.Fatalf("missing-token should prompt for a masked token: %q", plain)
	}
	m, _ = HandleTestKey(m, "a")
	m, _ = HandleTestKey(m, "b")
	m, _ = HandleTestKey(m, "c")
	plain = stripANSI(ViewForTest(m))
	if strings.Contains(plain, "abc") {
		t.Fatalf("token value must not be rendered: %q", plain)
	}
}

func settingsFocusedOnPair(t *testing.T, connected bool) model {
	t.Helper()
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 40)
	m.telegram = &telegramState{
		installed:       true,
		pluginVersion:   "2.9.2",
		protocolVersion: 1,
		connected:       connected,
		address:         "ai_workflow_2",
		abbrev:          "ai_workflow",
	}
	m, _ = m.openSettings()
	m, _ = HandleTestKey(m, "down")
	m, _ = HandleTestKey(m, "down")
	m, _ = HandleTestKey(m, "down")
	m, _ = HandleTestKey(m, "down")
	got := m.settingsRows()[m.settings.cursor]
	if got.kind != rowTelegramAction || got.action != "pair" {
		t.Fatalf("cursor=%+v want Pair", got)
	}
	return m
}

func settingsFocusedOnRetry(t *testing.T) model {
	t.Helper()
	m := SetWidth(NewTestModel(nil), 100)
	m = SetHeight(m, 40)
	m.telegram = &telegramState{
		installed:       true,
		pluginVersion:   "2.9.2",
		protocolVersion: 1,
		connected:       false,
		abbrev:          "ai_workflow",
		daemonErr:       "ipc: dial: connection refused",
	}
	m, _ = m.openSettings()
	m, _ = HandleTestKey(m, "down")
	m, _ = HandleTestKey(m, "down")
	m, _ = HandleTestKey(m, "down")
	m, _ = HandleTestKey(m, "down")
	got := m.settingsRows()[m.settings.cursor]
	if got.kind != rowTelegramAction || got.action != "retry" {
		t.Fatalf("cursor=%+v want Retry", got)
	}
	return m
}

func flushTeaCmds(m model, cmd tea.Cmd) model {
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				m = flushTeaCmds(m, sub)
			}
			cmd = nil
			continue
		}
		var nextCmd tea.Cmd
		m, nextCmd = HandleTestMsg(m, msg)
		cmd = nextCmd
	}
	return m
}

func TestTelegramModelSelectionMergesCatalogAndLiveGrok(t *testing.T) {
	m, _ := newPickerTestModel(t)
	var outbound []string
	m.telegram = &telegramState{
		installed: true,
		connected: true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}
	m.propsSvc.Catalog = propsCatalog(map[string]map[string]modelprops.CatalogProperty{
		"cursor-grok-4.5": {
			"fs": {Available: true, Values: []string{"true", "false"}, Default: "false"},
		},
	})
	if m.svc != nil && m.svc.Store != nil {
		_ = m.svc.Store.UpsertModelList("cursor", []string{
			"composer-2.5",
			"cursor-grok-4.5-high",
			"cursor-grok-4.5-low",
		}, "2026-09-08T00:00:00Z")
	}

	prev := listModelsForHarnessFn
	listModelsForHarnessFn = func(_ context.Context, _ model, harnessID string) ([]string, error) {
		return []string{
			"composer-2.5",
			"cursor-grok-4.5-high",
			"cursor-grok-4.5-medium",
		}, nil
	}
	t.Cleanup(func() { listModelsForHarnessFn = prev })

	next, _ := m.Update(telegramInboundMsg{text: "/model", isCommand: true, address: "proj"})
	next, cmd := next.(model).Update(telegramInboundMsg{text: "1", address: "proj"})
	m = flushTeaCmds(next.(model), cmd)

	prompt := outbound[len(outbound)-1]
	for _, want := range []string{"cursor-grok-4.5", "cursor-grok-4.5-high", "cursor-grok-4.5-low", "cursor-grok-4.5-medium"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("merged model prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestTelegramModelSelectionAlwaysUsesLiveList(t *testing.T) {
	m, _ := newPickerTestModel(t)
	m.telegram = &telegramState{installed: true, connected: true}
	if m.svc != nil && m.svc.Store != nil {
		_ = m.svc.Store.UpsertModelList("cursor", []string{"stale-only/model"}, "2026-09-08T00:00:00Z")
	}

	var listed bool
	prev := listModelsForHarnessFn
	listModelsForHarnessFn = func(_ context.Context, _ model, harnessID string) ([]string, error) {
		listed = true
		return []string{"live/model"}, nil
	}
	t.Cleanup(func() { listModelsForHarnessFn = prev })

	next, _ := m.Update(telegramInboundMsg{text: "/model", isCommand: true, address: "proj"})
	next, cmd := next.(model).Update(telegramInboundMsg{text: "1", address: "proj"})
	_ = flushTeaCmds(next.(model), cmd)
	if !listed {
		t.Fatal("Telegram model selection must call live ListModels even when cache is populated")
	}
}

func TestTelegramModelSelectionPaginatesLongLists(t *testing.T) {
	m, _ := newPickerTestModel(t)
	var outbound []string
	m.telegram = &telegramState{
		installed: true,
		connected: true,
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}

	models := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		models = append(models, fmt.Sprintf("zz-model-%02d", i))
	}
	models[10] = "cursor-grok-4.5-high"

	prev := listModelsForHarnessFn
	listModelsForHarnessFn = func(_ context.Context, _ model, harnessID string) ([]string, error) {
		return models, nil
	}
	t.Cleanup(func() { listModelsForHarnessFn = prev })

	m.modelOptions = nil
	m.availableModels = nil

	next, _ := m.Update(telegramInboundMsg{text: "/model", isCommand: true, address: "proj"})
	next, cmd := next.(model).Update(telegramInboundMsg{text: "1", address: "proj"})
	m = flushTeaCmds(next.(model), cmd)

	prompt := outbound[len(outbound)-1]
	if !strings.Contains(prompt, "página 1/") {
		t.Fatalf("expected paginated header, got %q", prompt)
	}
	if !strings.Contains(prompt, "Próxima página") {
		t.Fatalf("expected next-page option, got %q", prompt)
	}

	next, _ = m.Update(telegramInboundMsg{text: "0", address: "proj"})
	m = next.(model)
	if m.telegram.modelSelection == nil || m.telegram.modelSelection.modelPage != 1 {
		t.Fatalf("page=%d want 1 after next navigation", m.telegram.modelSelection.modelPage)
	}
}

func TestTelegramModelSelectionStartsPropsRefresh(t *testing.T) {
	m, _ := newPickerTestModel(t)
	m.telegram = &telegramState{installed: true, connected: true}
	m.propsRefreshBusy = false

	next, _ := m.Update(telegramInboundMsg{text: "/model", isCommand: true, address: "proj"})
	if !next.(model).propsRefreshBusy {
		t.Fatal("Telegram /model must start background model props refresh")
	}
}
