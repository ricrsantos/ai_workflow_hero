package tui

import (
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
	if got := m.telegramStatusText(now); got != "idle" {
		t.Fatalf("idle status=%q", got)
	}
}

func TestTelegramStatusCommandAndAutoReportSendStatus(t *testing.T) {
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
	if len(outbound) != 1 || outbound[0] != "idle" {
		t.Fatalf("/status outbound=%v", outbound)
	}

	cmd = next.maybeTelegramAutoReport(now)
	if cmd != nil {
		_ = cmd()
	}
	if len(outbound) != 2 || outbound[1] != "idle" {
		t.Fatalf("auto-report outbound=%v", outbound)
	}
	if !next.telegram.nextAutoReportAt.After(now) {
		t.Fatal("auto report schedule was not advanced")
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
	for _, want := range []string{"Not configured", "Installed · v2.9.2", "Connected", "Project ID", "Pair"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q: %q", want, plain)
		}
	}
	// Down from Debug lands on Project ID, then Auto report, then Pair — never a status badge.
	m, _ = HandleTestKey(m, "down")
	if got := m.settingsRows()[m.settings.cursor].kind; got != rowTelegramAbbrev {
		t.Fatalf("after debug, cursor kind=%d want Project ID", got)
	}
	m, _ = HandleTestKey(m, "down")
	if got := m.settingsRows()[m.settings.cursor].kind; got != rowTelegramAutoReport {
		t.Fatalf("after project ID, cursor kind=%d want Auto report", got)
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
	next, _ = m.Update(telegramInboundMsg{text: "1", address: "proj"})
	m = next.(model)
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
	got := m.settingsRows()[m.settings.cursor]
	if got.kind != rowTelegramAction || got.action != "retry" {
		t.Fatalf("cursor=%+v want Retry", got)
	}
	return m
}
