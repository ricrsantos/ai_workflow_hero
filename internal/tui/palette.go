package tui

import (
	"log/slog"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

type paletteAction int

const (
	actionGoScreen paletteAction = iota
	actionNew
	actionNewChat
	actionStart
	actionSync
	actionStatus
	actionApprove
	actionReject
	actionContinue
	actionBack
	actionCancel
	actionFinish
	actionArchive
	actionResume
	actionCycles
	actionTodos
	actionHelp
	actionImportCommand
	actionRefresh
	actionQuit
	actionModel
	actionSelectModel
	actionPickModelHarness
	actionHarness
	actionToggleHarness
	actionApplyHarness
	actionHarnessReset
	actionSelectHarnessReset
	actionConfigUpdate
)

// TUI slash labels (chat-oriented commands use non-/hero names).
const (
	slashModel   = "/model"
	slashHarness = "/harness"
	slashRefresh = "/hero-refresh"
)

type paletteItem struct {
	label        string
	hint         string
	action       paletteAction
	screen       screen
	commandPath  string
	commandLabel string
	modelSlug    string
	harnessID    string
}

func defaultHeroPaletteItems() []paletteItem {
	heroSlashes := []paletteItem{
		{label: "/new-chat", hint: "clear session", action: actionNewChat},
		{label: slashModel, hint: "select default model", action: actionModel},
		{label: "/hero-new", hint: "create cycle", action: actionNew},
		{label: "/hero-start", hint: "start workflow", action: actionStart},
		{label: "/hero-approve", hint: "pending approval", action: actionApprove},
		{label: "/hero-continue", hint: "grant extra iterations", action: actionContinue},
		{label: "/hero-reject", hint: "send back", action: actionReject},
		{label: "/hero-cancel", hint: "abort active cycle", action: actionCancel},
		{label: "/hero-back", hint: "reopen planning", action: actionBack},
		{label: "/hero-resume", hint: "reactivate cycle", action: actionResume},
		{label: "/hero-finish", hint: "complete cycle", action: actionFinish},
		{label: "/hero-archive", hint: "archive cycle", action: actionArchive},
		{label: "/hero-status", hint: "cycle status", action: actionStatus},
		{label: "/hero-cycles", hint: "list cycles", action: actionCycles},
		{label: "/hero-todos", hint: "pending items", action: actionTodos},
		{label: "/hero-sync", hint: "sync project", action: actionSync},
		{label: "/hero-config-update", hint: "reload model from config", action: actionConfigUpdate},
		{label: slashHarness, hint: "manage harnesses", action: actionHarness},
		{label: "/harness-reset", hint: "restart harness connection", action: actionHarnessReset},
		{label: "/hero-help", hint: "workflow guide", action: actionHelp},
		{label: slashRefresh, hint: "reload from store", action: actionRefresh},
	}
	goTo := []paletteItem{
		{label: "Go to - Chat", hint: "conversation", action: actionGoScreen, screen: screenConversation},
		{label: "Go to - Status", hint: "cycle overview", action: actionGoScreen, screen: screenStatus},
		{label: "Go to - Artifacts", hint: "linked files", action: actionGoScreen, screen: screenArtifacts},
		{label: "Go to - Costs", hint: "token metrics", action: actionGoScreen, screen: screenCosts},
		{label: "Go to - Events", hint: "event log", action: actionGoScreen, screen: screenEvents},
	}
	rest := []paletteItem{
		{label: "Quit", hint: "exit TUI", action: actionQuit},
	}
	items := make([]paletteItem, 0, len(heroSlashes)+len(goTo)+len(rest))
	items = append(items, heroSlashes...)
	items = append(items, goTo...)
	items = append(items, rest...)
	return items
}

func buildPaletteItems(projectDir string) []paletteItem {
	return buildPaletteItemsWithHome(projectDir, "")
}

func buildPaletteItemsWithHome(projectDir, userHome string) []paletteItem {
	items := defaultHeroPaletteItems()
	if projectDir == "" {
		return items
	}
	imported, err := cursor.DiscoverCommands(projectDir, userHome)
	if err != nil {
		slog.Debug("tui command discovery failed", "error", err)
		return items
	}
	for _, cmd := range imported {
		hint := sourceHint(cmd.Source)
		items = append(items, paletteItem{
			label:        cmd.Label,
			hint:         hint,
			action:       actionImportCommand,
			commandPath:  cmd.Path,
			commandLabel: cmd.Label,
		})
	}
	return items
}

func filterFreeChatPaletteItems(items []paletteItem) []paletteItem {
	out := make([]paletteItem, 0, len(items))
	for _, item := range items {
		if !isFreeChatPaletteItem(item) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func isFreeChatPaletteItem(item paletteItem) bool {
	label := strings.TrimSpace(item.label)
	if strings.HasPrefix(label, "/hero") {
		return false
	}
	if strings.HasPrefix(label, "Go to -") {
		return false
	}
	return true
}

func sourceHint(source cursor.CommandSource) string {
	switch source {
	case cursor.CommandSourceUser:
		return "harness command · user (~/.cursor/commands)"
	default:
		return "harness command · project"
	}
}

func (m model) filteredPaletteItems() []paletteItem {
	filter := strings.ToLower(strings.TrimSpace(m.paletteFilter))
	if filter == "" {
		return m.paletteItems
	}
	var out []paletteItem
	for _, item := range m.paletteItems {
		if strings.Contains(strings.ToLower(item.label), filter) ||
			strings.Contains(strings.ToLower(item.hint), filter) {
			out = append(out, item)
		}
	}
	return out
}
