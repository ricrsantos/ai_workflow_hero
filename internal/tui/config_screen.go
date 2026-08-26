package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

// Config screen key bindings are kept together so screen-specific controls do
// not leak raw key-string checks across the TUI.
var configKeys = struct {
	Save      key.Binding
	SaveStart key.Binding
	Retry     key.Binding
	Reload    key.Binding
	Next      key.Binding
	Previous  key.Binding
	Toggle    key.Binding
	Edit      key.Binding
	Cancel    key.Binding
	Leave     key.Binding
	Discard   key.Binding
}{
	Save:      key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save")),
	SaveStart: key.NewBinding(key.WithKeys("ctrl+enter"), key.WithHelp("ctrl+enter", "save and start")),
	Retry:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "retry failed stage")),
	Reload:    key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "reload")),
	Next:      key.NewBinding(key.WithKeys("tab", "down"), key.WithHelp("tab", "next field")),
	Previous:  key.NewBinding(key.WithKeys("shift+tab", "up"), key.WithHelp("shift+tab", "previous field")),
	Toggle:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
	Edit:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
	Cancel:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	Leave:     key.NewBinding(key.WithKeys("esc", "alt+n", "alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+q"), key.WithHelp("esc", "leave")),
	Discard:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "discard")),
}

type configScreen struct {
	loading         bool
	saving          bool
	doc             *workflowconfig.Document
	baseline        workflowconfig.ManagedConfig
	draft           workflowconfig.ManagedConfig
	err             string
	fieldErrors     map[string]string
	harnessWarnings []string
	message         string
	dirty           bool
	retryAllowed    map[string]bool
	focus           int
	editing         bool
	editBuffer      string
	editCursor      int
	leaveDialog     bool
	leaveScreen     screen
	leaveQuit       bool
}

type configLoadedMsg struct {
	doc             *workflowconfig.Document
	harnessWarnings []string
	err             error
}

type configSavedMsg struct {
	doc     *workflowconfig.Document
	retries map[string]bool
	start   bool
	err     error
}

type configRetryMsg struct {
	stage string
	err   error
}

func (m model) openConfig() (model, tea.Cmd) {
	if m.freeChatMode || !m.hasActiveCycle() {
		return m, nil
	}
	m.chatInputFocused = false
	m.screen = screenConfig
	if m.config.doc != nil || m.config.loading {
		return m, nil
	}
	m.config.loading = true
	m.config.err = ""
	m.config.message = ""
	return m, m.configLoadCmd()
}

func (m model) configLoadCmd() tea.Cmd {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	return func() tea.Msg {
		doc, err := workflowconfig.LoadCurrentDocument(projectDir)
		if err != nil {
			return configLoadedMsg{err: err}
		}
		return configLoadedMsg{
			doc:             doc,
			harnessWarnings: configHarnessWarnings(m.svc, projectDir),
		}
	}
}

func configHarnessWarnings(svc *cycle.Service, projectDir string) []string {
	if svc == nil || svc.Registry == nil {
		return nil
	}
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		return nil
	}
	var warnings []string
	for _, id := range install.ListEnabledHarnesses(hero) {
		adapter, err := svc.Registry.Adapter(id)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s harness unavailable: %s", harnessDisplayName(id), harnessUnavailableReason(err)))
			continue
		}
		if err := adapter.IsAvailable(context.Background()); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s harness unavailable: %s", harnessDisplayName(id), harnessUnavailableReason(err)))
		}
	}
	return warnings
}

func (m model) handleConfigMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case configLoadedMsg:
		m.config.loading = false
		if msg.err != nil {
			m.config.err = msg.err.Error()
			slog.Error("config document load failed", "error", msg.err)
			return m, nil
		}
		m.config.doc = msg.doc
		m.config.baseline = msg.doc.Config
		m.config.draft = msg.doc.Config
		m.config.harnessWarnings = msg.harnessWarnings
		m.config.dirty = false
		m.config.err = ""
		m.config.fieldErrors = nil
		return m, nil
	case configSavedMsg:
		m.config.saving = false
		if msg.err != nil {
			m.config.err = msg.err.Error()
			m.config.fieldErrors = configFieldErrors(msg.err)
			slog.Error("config save failed", "error", msg.err)
			return m, nil
		}
		m.config.doc = msg.doc
		m.config.baseline = msg.doc.Config
		m.config.draft = msg.doc.Config
		m.config.retryAllowed = msg.retries
		m.config.dirty = false
		m.config.err = ""
		m.config.fieldErrors = nil
		m.config.message = fmt.Sprintf("✓ Configuration saved and synchronized for cycle C%d.", m.status.CycleNumber)
		slog.Info("cycle configuration saved", "cycle", m.status.CycleNumber)
		if msg.start {
			return m.beginHeroStart()
		}
		if m.config.leaveDialog {
			m.config.leaveDialog = false
			return m.completeConfigLeave()
		}
		return m, m.refreshCmd()
	case configRetryMsg:
		if msg.err != nil {
			m.config.err = msg.err.Error()
			slog.Error("failed stage retry failed", "stage", msg.stage, "error", msg.err)
			return m, nil
		}
		delete(m.config.retryAllowed, msg.stage)
		m.config.message = fmt.Sprintf("✓ %s retry queued.", configStageLabel(msg.stage))
		slog.Info("failed stage retry queued", "stage", msg.stage)
		return m, m.refreshCmd()
	}
	return m, nil
}

func (m model) configReadOnly() bool {
	return m.actionBusy || m.streaming || m.heroStartBootstrapping || m.heroStartPreparing
}

type configField struct {
	path  string
	label string
	kind  string // text, number, bool, harness, model, property
	stage string
	agent string
}

func (m model) configFields() []configField {
	fields := []configField{
		{"title", "Title", "text", "", ""},
		{"objective", "Objective", "text", "", ""},
		{"workflow_config.user_preferred_language", "Chat language", "text", "", ""},
		{"scope.backend", "Backend", "bool", "", ""},
		{"scope.frontend", "Frontend", "bool", "", ""},
		{"scope.native", "Native", "bool", "", ""},
		{"scope.script", "Script", "bool", "", ""},
		{"scope.infrastructure", "Infrastructure", "bool", "", ""},
	}
	requiredAgents := m.config.draft.RequiredAgentNames()
	for _, name := range []string{"research", "planning", "implementation", "qa", "judge", "browser_ui_validation", "qa_end_to_end"} {
		stage, ok := m.config.draft.Stages[name]
		if !ok {
			continue
		}
		fields = append(fields, configField{"stages." + name + ".enabled", configStageLabel(name) + " enabled", "bool", name, ""})
		if !stage.Enabled {
			continue
		}
		fields = append(fields,
			configField{"stages." + name + ".purpose", configStageLabel(name) + " purpose", "text", name, ""},
			configField{"stages." + name + ".max_iterations", configStageLabel(name) + " iterations", "number", name, ""},
			configField{"stages." + name + ".timeout_minutes", configStageLabel(name) + " timeout", "number", name, ""},
			configField{"stages." + name + ".require_human_approval", configStageLabel(name) + " approval", "bool", name, ""},
		)
		if name == "browser_ui_validation" && m.config.draft.Scope.Frontend {
			fields = append(fields,
				configField{"stages." + name + ".visual_validation.enabled", "Visual validation", "bool", name, ""},
				configField{"stages." + name + ".visual_validation.reference_dir", "Reference directory", "text", name, ""},
			)
		}
		if name == "qa_end_to_end" && m.config.draft.Scope.Frontend {
			fields = append(fields, configField{"stages." + name + ".use_playwright", "Use Playwright", "bool", name, ""})
		}
		for _, agent := range requiredAgents {
			if configAgentStage(agent) != name {
				continue
			}
			if agent == "browser_ui_agent" && !m.config.draft.Scope.Frontend {
				continue
			}
			fields = append(fields, m.agentFields(agent, name)...)
		}
	}
	for _, name := range requiredAgents {
		if configAgentStage(name) == "" {
			fields = append(fields, m.agentFields(name, "")...)
		}
	}
	fields = append(fields, m.agentFields("fallback_model", "")...)
	return fields
}

func (m model) agentFields(name, stage string) []configField {
	prefix := "agents." + name
	if name == "fallback_model" {
		prefix = name
	}
	fields := []configField{
		{prefix + ".harness", configAgentLabel(name) + " harness", "harness", stage, name},
		{prefix + ".model", configAgentLabel(name) + " model", "model", stage, name},
	}
	agent := m.config.draft.FallbackModel
	if name != "fallback_model" {
		agent = m.config.draft.Agents[name]
	}
	for _, property := range []struct{ key, suffix string }{
		{harness.PropertyFast, "enable_fast_model"},
		{harness.PropertyThink, "thinking"},
		{harness.PropertyEffort, "reasoning_effort"},
	} {
		if m.configPropertyVisible(agent, property.key) {
			fields = append(fields, configField{prefix + "." + property.suffix, configAgentLabel(name) + " " + property.key, "property", stage, name})
		}
	}
	if name != "fallback_model" {
		fields = append(fields, configField{prefix + ".subagent.same_of_agent", configAgentLabel(name) + " subagent same as agent", "bool", stage, name + ":subagent"})
	}
	if name != "fallback_model" && !agent.Subagent.SameOfAgent {
		fields = append(fields,
			configField{prefix + ".subagent.model", configAgentLabel(name) + " subagent model", "model", stage, name + ":subagent"},
		)
		subagent := agent
		subagent.Model = agent.Subagent.Model
		subagent.EnableFastModel = agent.Subagent.EnableFastModel
		subagent.Thinking = agent.Subagent.Thinking
		subagent.ReasoningEffort = agent.Subagent.ReasoningEffort
		for _, property := range []struct{ key, suffix string }{
			{harness.PropertyFast, "enable_fast_model"},
			{harness.PropertyThink, "thinking"},
			{harness.PropertyEffort, "reasoning_effort"},
		} {
			if m.configPropertyVisible(subagent, property.key) {
				fields = append(fields, configField{prefix + ".subagent." + property.suffix, configAgentLabel(name) + " subagent " + property.key, "property", stage, name + ":subagent"})
			}
		}
	}
	return fields
}

func configAgentLabel(name string) string {
	return configStageLabel(name)
}

func configAgentStage(name string) string {
	switch name {
	case "discover_agent":
		return "research"
	case "planning_agent":
		return "planning"
	case "backend_agent", "frontend_agent", "generic_agent":
		return "implementation"
	case "qa_agent":
		return "qa"
	case "judge_agent":
		return "judge"
	case "browser_ui_agent":
		return "browser_ui_validation"
	case "end2end_qa_agent":
		return "qa_end_to_end"
	default:
		return ""
	}
}

func (m model) configPropertyVisible(agent workflowconfig.AgentModelConfig, property string) bool {
	if m.propsSvc == nil {
		return true
	}
	snap := m.propsSvc.Snapshot(agent.Harness, agent.Model)
	if snap.Source == modelprops.SourceUnknown {
		return true
	}
	return snap.Property(property).Available
}

func (m model) configCapabilityWarning(field configField) string {
	if m.propsSvc == nil || field.kind != "model" || field.agent == "" {
		return ""
	}
	isSubagent := strings.HasSuffix(field.agent, ":subagent")
	agentName := strings.TrimSuffix(field.agent, ":subagent")
	agent := m.config.draft.FallbackModel
	if agentName != "fallback_model" {
		agent = m.config.draft.Agents[agentName]
	}
	model := agent.Model
	if isSubagent {
		model = agent.Subagent.Model
	}
	if m.propsSvc.Snapshot(agent.Harness, model).Source != modelprops.SourceUnknown {
		return ""
	}
	return "⚠ Missing capability data; configured properties are preserved."
}

func (m model) handleConfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.config.leaveDialog {
		if m.config.saving {
			return m, nil
		}
		if key.Matches(msg, configKeys.Save) || key.Matches(msg, configKeys.Edit) {
			return m.beginConfigSave(false)
		}
		if key.Matches(msg, configKeys.Discard) {
			m.config.draft = m.config.baseline
			m.config.dirty = false
			m.config.leaveDialog = false
			m.config.message = "Configuration changes discarded."
			return m.completeConfigLeave()
		}
		if key.Matches(msg, configKeys.Cancel) {
			m.config.leaveDialog = false
			return m, nil
		}
		return m, nil
	}
	if m.config.editing {
		return m.handleConfigEditKey(msg)
	}
	if key.Matches(msg, configKeys.Reload) {
		if m.config.saving {
			return m, nil
		}
		m.config = configScreen{loading: true}
		return m, m.configLoadCmd()
	}
	if key.Matches(msg, configKeys.Save) {
		return m.beginConfigSave(false)
	}
	if key.Matches(msg, configKeys.SaveStart) {
		return m.beginConfigSave(true)
	}
	if key.Matches(msg, configKeys.Retry) {
		for stage := range m.config.retryAllowed {
			return m.beginConfigRetry(stage)
		}
	}
	if m.config.dirty && key.Matches(msg, configKeys.Leave) {
		m.config.leaveDialog = true
		m.config.leaveScreen, m.config.leaveQuit = configLeaveTarget(msg)
		return m, nil
	}
	if key.Matches(msg, configKeys.Leave) {
		m.config.leaveScreen, m.config.leaveQuit = configLeaveTarget(msg)
		return m.completeConfigLeave()
	}
	fields := m.configFields()
	if key.Matches(msg, configKeys.Next) && len(fields) > 0 {
		m.config.focus = (m.config.focus + 1) % len(fields)
		return m.configEnsureFocusVisible(), nil
	}
	if key.Matches(msg, configKeys.Previous) && len(fields) > 0 {
		m.config.focus = (m.config.focus - 1 + len(fields)) % len(fields)
		return m.configEnsureFocusVisible(), nil
	}
	if len(fields) > 0 && (key.Matches(msg, configKeys.Toggle) || key.Matches(msg, configKeys.Edit)) {
		field := fields[m.config.focus%len(fields)]
		if m.configReadOnly() || m.stageProtected(field.stage) {
			m.config.message = "Editing is available when execution/preflight finishes."
			return m, nil
		}
		if field.kind == "bool" {
			m = m.toggleConfigField(field)
			return m.configEnsureFocusVisible(), nil
		}
		if field.kind == "harness" || field.kind == "model" || field.kind == "property" {
			m = m.cycleConfigChoice(field)
			return m, nil
		}
		m.config.editing = true
		m.config.editBuffer = m.configFieldValue(field)
		m.config.editCursor = runeLen(m.config.editBuffer)
		return m.configEnsureFocusVisible(), nil
	}
	return m.handleKey(msg)
}

func configLeaveTarget(msg tea.KeyMsg) (screen, bool) {
	switch msg.String() {
	case "ctrl+c", "alt+q":
		return screenConfig, true
	case "alt+n":
		return screenEvents, false
	case "alt+1", "ctrl+1":
		return screenConversation, false
	case "alt+2", "ctrl+2":
		return screenConfig, false
	case "alt+3", "ctrl+3":
		return screenStatus, false
	case "alt+4", "ctrl+4":
		return screenArtifacts, false
	case "alt+5", "ctrl+5":
		return screenCosts, false
	default:
		return screenConversation, false
	}
}

func (m model) completeConfigLeave() (tea.Model, tea.Cmd) {
	if m.config.leaveQuit {
		m.config.leaveQuit = false
		return m, tea.Quit
	}
	target := m.config.leaveScreen
	m.config.leaveScreen = screenConfig
	switch target {
	case screenConversation:
		return m.enterConversation()
	case screenConfig:
		return m, nil
	default:
		return m.goListScreen(target)
	}
}

func (m model) handleConfigEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, configKeys.Cancel) {
		m.config.editing = false
		m.config.editBuffer = ""
		m.config.editCursor = 0
		return m, nil
	}
	if key.Matches(msg, configKeys.Edit) {
		fields := m.configFields()
		if len(fields) > 0 {
			m = m.setConfigField(fields[m.config.focus%len(fields)], m.config.editBuffer)
		}
		m.config.editing = false
		m.config.editBuffer = ""
		m.config.editCursor = 0
		return m, nil
	}
	switch msg.String() {
	case "left":
		if m.config.editCursor > 0 {
			m.config.editCursor--
		}
	case "right":
		if m.config.editCursor < runeLen(m.config.editBuffer) {
			m.config.editCursor++
		}
	case "home":
		m.config.editCursor = 0
	case "end":
		m.config.editCursor = runeLen(m.config.editBuffer)
	case "backspace":
		if m.config.editCursor > 0 {
			runes := []rune(m.config.editBuffer)
			m.config.editBuffer = string(append(runes[:m.config.editCursor-1], runes[m.config.editCursor:]...))
			m.config.editCursor--
		}
	case "delete":
		runes := []rune(m.config.editBuffer)
		if m.config.editCursor < len(runes) {
			m.config.editBuffer = string(append(runes[:m.config.editCursor], runes[m.config.editCursor+1:]...))
		}
	default:
		if len(msg.Runes) > 0 && !msg.Alt {
			runes := []rune(m.config.editBuffer)
			cursor := m.config.editCursor
			if cursor < 0 {
				cursor = 0
			}
			if cursor > len(runes) {
				cursor = len(runes)
			}
			out := make([]rune, 0, len(runes)+len(msg.Runes))
			out = append(out, runes[:cursor]...)
			out = append(out, msg.Runes...)
			out = append(out, runes[cursor:]...)
			m.config.editBuffer = string(out)
			m.config.editCursor = cursor + len(msg.Runes)
		}
	}
	return m.configEnsureFocusVisible(), nil
}

func (m model) stageProtected(stage string) bool {
	if stage == "" {
		return false
	}
	for _, row := range m.status.Stages {
		if strings.EqualFold(strings.ReplaceAll(row.Name, " ", "_"), stage) {
			return row.Status == "Completed"
		}
	}
	return false
}

func (m model) configFieldValue(field configField) string {
	c := m.config.draft
	switch field.path {
	case "title":
		return c.Title
	case "objective":
		return c.Objective
	case "workflow_config.user_preferred_language":
		return c.WorkflowConfig.UserPreferredLanguage
	}
	if field.stage != "" && field.agent == "" {
		stage := c.Stages[field.stage]
		switch {
		case strings.HasSuffix(field.path, ".purpose"):
			return stage.Purpose
		case strings.HasSuffix(field.path, ".max_iterations"):
			return fmt.Sprintf("%d", stage.MaxIterations)
		case strings.HasSuffix(field.path, ".timeout_minutes"):
			return fmt.Sprintf("%d", stage.TimeoutMinutes)
		case strings.HasSuffix(field.path, ".visual_validation.reference_dir"):
			return stage.VisualValidation.ReferenceDir
		}
	}
	if field.agent != "" {
		isSubagent := strings.HasSuffix(field.agent, ":subagent")
		agentName := strings.TrimSuffix(field.agent, ":subagent")
		agent := c.FallbackModel
		if agentName != "fallback_model" {
			agent = c.Agents[agentName]
		}
		if isSubagent {
			switch {
			case strings.HasSuffix(field.path, ".same_of_agent"):
				return fmt.Sprintf("%t", agent.Subagent.SameOfAgent)
			case strings.HasSuffix(field.path, ".model"):
				return agent.Subagent.Model
			case strings.HasSuffix(field.path, ".thinking"):
				return agent.Subagent.Thinking
			case strings.HasSuffix(field.path, ".reasoning_effort"):
				return agent.Subagent.ReasoningEffort
			}
		}
		switch {
		case strings.HasSuffix(field.path, ".harness"):
			return agent.Harness
		case strings.HasSuffix(field.path, ".model"):
			return agent.Model
		case strings.HasSuffix(field.path, ".thinking"):
			return agent.Thinking
		case strings.HasSuffix(field.path, ".reasoning_effort"):
			return agent.ReasoningEffort
		}
	}
	return ""
}

func (m model) setConfigField(field configField, value string) model {
	value = strings.TrimSpace(value)
	c := m.config.draft
	switch field.path {
	case "title":
		c.Title = value
	case "objective":
		c.Objective = value
	case "workflow_config.user_preferred_language":
		c.WorkflowConfig.UserPreferredLanguage = value
	default:
		if field.stage != "" && field.agent == "" {
			stage := c.Stages[field.stage]
			switch {
			case strings.HasSuffix(field.path, ".purpose"):
				stage.Purpose = value
			case strings.HasSuffix(field.path, ".max_iterations"):
				_, _ = fmt.Sscanf(value, "%d", &stage.MaxIterations)
			case strings.HasSuffix(field.path, ".timeout_minutes"):
				_, _ = fmt.Sscanf(value, "%d", &stage.TimeoutMinutes)
			case strings.HasSuffix(field.path, ".visual_validation.reference_dir"):
				stage.VisualValidation.ReferenceDir = value
			}
			c.Stages[field.stage] = stage
		} else if field.agent != "" {
			isSubagent := strings.HasSuffix(field.agent, ":subagent")
			agentName := strings.TrimSuffix(field.agent, ":subagent")
			agent := c.FallbackModel
			if agentName != "fallback_model" {
				agent = c.Agents[agentName]
			}
			if isSubagent && strings.HasSuffix(field.path, ".model") {
				agent.Subagent.Model = value
			} else if isSubagent && strings.HasSuffix(field.path, ".thinking") {
				agent.Subagent.Thinking = value
			} else if isSubagent && strings.HasSuffix(field.path, ".reasoning_effort") {
				agent.Subagent.ReasoningEffort = value
			} else if strings.HasSuffix(field.path, ".harness") {
				agent.Harness = value
			} else if strings.HasSuffix(field.path, ".model") {
				agent.Model = value
			} else if strings.HasSuffix(field.path, ".thinking") {
				agent.Thinking = value
			} else if strings.HasSuffix(field.path, ".reasoning_effort") {
				agent.ReasoningEffort = value
			}
			if agentName == "fallback_model" {
				c.FallbackModel = agent
			} else {
				c.Agents[agentName] = agent
			}
		}
	}
	m.config.draft = c
	m.config.dirty = true
	return m
}

func (m model) toggleConfigField(field configField) model {
	c := m.config.draft
	switch field.path {
	case "scope.backend":
		c.Scope.Backend = !c.Scope.Backend
	case "scope.frontend":
		c.Scope.Frontend = !c.Scope.Frontend
	case "scope.native":
		c.Scope.Native = !c.Scope.Native
	case "scope.script":
		c.Scope.Script = !c.Scope.Script
	case "scope.infrastructure":
		c.Scope.Infrastructure = !c.Scope.Infrastructure
	default:
		if field.stage != "" && field.agent == "" {
			stage := c.Stages[field.stage]
			switch {
			case strings.HasSuffix(field.path, ".enabled"):
				stage.Enabled = !stage.Enabled
			case strings.HasSuffix(field.path, ".require_human_approval"):
				stage.RequireHumanApproval = !stage.RequireHumanApproval
			case strings.HasSuffix(field.path, ".visual_validation.enabled"):
				stage.VisualValidation.Enabled = !stage.VisualValidation.Enabled
			case strings.HasSuffix(field.path, ".use_playwright"):
				stage.UsePlaywright = !stage.UsePlaywright
			}
			c.Stages[field.stage] = stage
		} else if field.agent != "" {
			isSubagent := strings.HasSuffix(field.agent, ":subagent")
			agentName := strings.TrimSuffix(field.agent, ":subagent")
			agent := c.FallbackModel
			if agentName != "fallback_model" {
				agent = c.Agents[agentName]
			}
			switch {
			case isSubagent && strings.HasSuffix(field.path, ".same_of_agent"):
				agent.Subagent.SameOfAgent = !agent.Subagent.SameOfAgent
			case isSubagent && strings.HasSuffix(field.path, ".enable_fast_model"):
				agent.Subagent.EnableFastModel = !agent.Subagent.EnableFastModel
			case strings.HasSuffix(field.path, ".enable_fast_model"):
				agent.EnableFastModel = !agent.EnableFastModel
			default:
				return m
			}
			if agentName == "fallback_model" {
				c.FallbackModel = agent
			} else {
				c.Agents[agentName] = agent
			}
		}
	}
	m.config.draft = c
	m.config.dirty = true
	return m
}

func (m model) cycleConfigChoice(field configField) model {
	c := m.config.draft
	isSubagent := strings.HasSuffix(field.agent, ":subagent")
	agentName := strings.TrimSuffix(field.agent, ":subagent")
	agent := c.FallbackModel
	if agentName != "fallback_model" {
		agent = c.Agents[agentName]
	}
	switch field.kind {
	case "harness":
		choices := m.enabledHarnessIDs()
		if len(choices) > 0 {
			agent.Harness = nextChoice(agent.Harness, choices)
		}
	case "model":
		var choices []string
		for _, option := range m.modelOptions {
			if strings.EqualFold(option.Harness, agent.Harness) {
				choices = append(choices, option.Model)
			}
		}
		if len(choices) > 0 {
			if isSubagent {
				agent.Subagent.Model = nextChoice(agent.Subagent.Model, choices)
			} else {
				agent.Model = nextChoice(agent.Model, choices)
			}
		} else {
			m.config.message = "⚠ Model catalog unavailable; existing YAML model retained."
			return m
		}
	case "property":
		if isSubagent && strings.HasSuffix(field.path, ".thinking") {
			agent.Subagent.Thinking = nextChoice(agent.Subagent.Thinking, []string{"na", "true", "false"})
		} else if isSubagent && strings.HasSuffix(field.path, ".reasoning_effort") {
			agent.Subagent.ReasoningEffort = nextChoice(agent.Subagent.ReasoningEffort, []string{"na", "low", "medium", "high"})
		} else if strings.HasSuffix(field.path, ".thinking") {
			agent.Thinking = nextChoice(agent.Thinking, []string{"na", "true", "false"})
		} else if strings.HasSuffix(field.path, ".reasoning_effort") {
			agent.ReasoningEffort = nextChoice(agent.ReasoningEffort, []string{"na", "low", "medium", "high"})
		} else {
			return m.toggleConfigField(field)
		}
	}
	if agentName == "fallback_model" {
		c.FallbackModel = agent
	} else {
		c.Agents[agentName] = agent
	}
	m.config.draft = c
	m.config.dirty = true
	return m
}

func nextChoice(current string, choices []string) string {
	for i, choice := range choices {
		if strings.EqualFold(choice, current) {
			return choices[(i+1)%len(choices)]
		}
	}
	return choices[0]
}

func (m model) beginConfigSave(start bool) (model, tea.Cmd) {
	if m.config.loading || m.config.saving || m.config.doc == nil {
		return m, nil
	}
	if m.configReadOnly() {
		m.config.message = "Editing is available when execution/preflight finishes."
		return m, nil
	}
	if err := m.config.draft.Validate(m.configValidationOptions()); err != nil {
		m.config.err = err.Error()
		m.config.fieldErrors = configFieldErrors(err)
		return m, nil
	}
	m.config.saving = true
	m.config.err = ""
	m.config.fieldErrors = nil
	return m, m.configSaveCmd(start)
}

func (m model) configSaveCmd(start bool) tea.Cmd {
	doc := m.config.doc
	draft := m.config.draft
	before := m.config.baseline
	svc := m.svc
	return func() tea.Msg {
		if doc == nil || svc == nil {
			return configSavedMsg{err: fmt.Errorf("configuration service unavailable")}
		}
		hero, err := install.LoadHeroJSON(svc.ProjectDir)
		if err != nil {
			return configSavedMsg{err: fmt.Errorf("read enabled harnesses: %w", err)}
		}
		opts := m.configValidationOptions()
		opts.ValidateEnabledHarnesses = true
		opts.EnabledHarnesses = install.ListEnabledHarnesses(hero)
		if err := doc.Write(draft, opts); err != nil {
			return configSavedMsg{err: err}
		}
		if err := svc.SyncCycleConfig(); err != nil {
			return configSavedMsg{err: err}
		}
		updated, err := workflowconfig.LoadDocument(doc.Path())
		if err != nil {
			return configSavedMsg{err: err}
		}
		retries := make(map[string]bool)
		for _, name := range failedStageNames(svc) {
			if configChangedStage(workflowconfig.ManagedDiff(before, draft), name) {
				retries[name] = true
			}
		}
		if start {
			// The caller receives a successful save first; start is routed through
			// the existing TUI handler rather than duplicating preflight here.
			return configSavedMsg{doc: updated, retries: retries, start: true}
		}
		return configSavedMsg{doc: updated, retries: retries}
	}
}

func (m model) configValidationOptions() workflowconfig.ValidationOptions {
	opts := workflowconfig.ValidationOptions{}
	opts.ModelKnown = func(harnessID, modelID string) (bool, bool) {
		known := false
		for _, option := range m.modelOptions {
			if strings.EqualFold(option.Harness, harnessID) {
				known = true
				if strings.EqualFold(option.Model, modelID) {
					return true, true
				}
			}
		}
		return known, false
	}
	opts.PropertyCapability = func(harnessID, modelID, property string) (bool, bool, []string) {
		if m.propsSvc == nil {
			return false, false, nil
		}
		snapshot := m.propsSvc.Snapshot(harnessID, modelID)
		if snapshot.Source == modelprops.SourceUnknown {
			return false, false, nil
		}
		capability := snapshot.Property(property)
		return true, capability.Available, capability.AcceptedValues
	}
	return opts
}

func configChangedStage(paths []string, stage string) bool {
	prefix := "stages." + stage + "."
	for _, path := range paths {
		if strings.HasPrefix(path, prefix) || path == "stages."+stage {
			return true
		}
		if strings.HasPrefix(path, "agents.") {
			parts := strings.Split(path, ".")
			if len(parts) > 1 && configAgentStage(parts[1]) == stage {
				return true
			}
		}
	}
	return false
}

func configFieldErrors(err error) map[string]string {
	if err == nil {
		return nil
	}
	message := err.Error()
	fields := map[string]string{}
	for _, prefix := range []string{
		"title", "objective", "workflow_config.user_preferred_language", "scope.frontend",
		"stages.", "agents.", "fallback_model",
	} {
		if idx := strings.Index(message, prefix); idx >= 0 {
			path := message[idx:]
			if end := strings.IndexAny(path, " :"); end >= 0 {
				path = path[:end]
			}
			fields[path] = message
			return fields
		}
	}
	fields["form"] = message
	return fields
}

func (m model) configEnsureFocusVisible() model {
	if m.screen != screenConfig || m.frameContentHeight() <= 0 {
		return m
	}
	lines := strings.Split(m.renderConfig(), "\n")
	for index, line := range lines {
		if strings.Contains(line, "▸ ") {
			height := m.frameContentHeight()
			if index < m.contentOffset {
				m.contentOffset = index
			} else if index >= m.contentOffset+height {
				m.contentOffset = index - height + 1
			}
			return m.clampContentOffset()
		}
	}
	return m
}

func failedStageNames(svc *cycle.Service) []string {
	if svc == nil || svc.Store == nil {
		return nil
	}
	c, err := svc.Store.GetActiveCycle()
	if err != nil {
		return nil
	}
	stages, err := svc.Store.ListStages(c.ID)
	if err != nil {
		return nil
	}
	var names []string
	for _, stage := range stages {
		if stage.Status == "Failed" {
			names = append(names, stage.Name)
		}
	}
	return names
}

func (m model) beginConfigRetry(stage string) (model, tea.Cmd) {
	if m.configReadOnly() || !m.config.retryAllowed[stage] {
		return m, nil
	}
	svc := m.svc
	return m, func() tea.Msg {
		if svc == nil {
			return configRetryMsg{stage: stage, err: fmt.Errorf("cycle service unavailable")}
		}
		return configRetryMsg{stage: stage, err: svc.RetryFailedStage(stage)}
	}
}

func (m model) renderConfig() string {
	if m.width < 50 || m.height < 12 {
		return errorStyle.Render("window too small\nResize the terminal to edit cycle configuration.")
	}
	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("Config · Cycle C%d", m.status.CycleNumber)))
	b.WriteByte('\n')
	if m.config.loading {
		b.WriteString(infoStyle.Render("→ Loading cycle configuration…"))
		return b.String()
	}
	if m.config.err != "" && m.config.doc == nil {
		b.WriteString(errorStyle.Render("✗ " + m.config.err))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("Correct the workflow-config.yml file manually, then reload Config."))
		return b.String()
	}
	if m.config.doc == nil {
		b.WriteString(mutedStyle.Render("No configuration loaded."))
		return b.String()
	}
	if m.config.dirty {
		b.WriteString(warnStyle.Render("⚠ Unsaved configuration changes"))
		b.WriteByte('\n')
	}
	if m.config.saving {
		b.WriteString(infoStyle.Render("→ Saving configuration…"))
		b.WriteByte('\n')
	} else if m.config.err != "" {
		b.WriteString(errorStyle.Render("✗ " + m.config.err))
		b.WriteByte('\n')
	} else if m.configReadOnly() {
		b.WriteString(mutedStyle.Render("Editing is available when execution/preflight finishes."))
		b.WriteByte('\n')
	} else if m.config.message != "" {
		b.WriteString(successStyle.Render(m.config.message))
		b.WriteByte('\n')
	}
	for _, warning := range m.config.harnessWarnings {
		b.WriteString(warnStyle.Render("⚠ " + warning))
		b.WriteByte('\n')
	}
	if m.config.leaveDialog {
		b.WriteString(m.renderConfigLeaveDialog())
		return b.String()
	}
	b.WriteByte('\n')
	fields := m.configFields()
	focus := 0
	if len(fields) > 0 {
		focus = m.config.focus % len(fields)
	}
	lastSection := ""
	for i, field := range fields {
		section := configFieldSection(field)
		if section != lastSection {
			if lastSection != "" {
				b.WriteByte('\n')
			}
			b.WriteString(headerStyle.Render(section))
			b.WriteByte('\n')
			lastSection = section
		}
		value := m.configFieldValue(field)
		if field.kind == "bool" || field.kind == "property" && strings.HasSuffix(field.path, ".enable_fast_model") {
			value = "[ ]"
			if configBoolValue(m.config.draft, field) {
				value = "[x]"
			}
		}
		line := m.renderConfigField(field.label, value, i == focus, m.stageProtected(field.stage))
		if field.stage != "" && strings.HasSuffix(field.path, ".enabled") && !configBoolValue(m.config.draft, field) {
			line += "  " + mutedStyle.Render("configuration retained")
		}
		if message, ok := m.config.fieldErrors[field.path]; ok {
			line += "\n" + errorStyle.Render("    ✗ "+message)
		}
		if warning := m.configCapabilityWarning(field); warning != "" {
			line += "\n" + warnStyle.Render("    "+warning)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if len(m.config.retryAllowed) > 0 {
		b.WriteByte('\n')
		for stage := range m.config.retryAllowed {
			b.WriteString(successStyle.Render("  Retry failed stage: [r] " + configStageLabel(stage)))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func configFieldSection(field configField) string {
	switch {
	case strings.HasPrefix(field.path, "title"), strings.HasPrefix(field.path, "objective"), strings.HasPrefix(field.path, "workflow_config."):
		return "Identity"
	case strings.HasPrefix(field.path, "scope."):
		return "Scope"
	case field.stage != "":
		return configStageLabel(field.stage)
	default:
		return "Shared / Advanced"
	}
}

func (m model) renderConfigLeaveDialog() string {
	var b strings.Builder
	b.WriteString(warnStyle.Render("Unsaved configuration changes"))
	b.WriteByte('\n')
	if m.config.saving {
		b.WriteString(infoStyle.Render("→ Saving configuration…"))
		return b.String()
	}
	if m.config.err != "" {
		b.WriteString(errorStyle.Render("✗ " + m.config.err))
		b.WriteByte('\n')
	}
	b.WriteString("[enter] Save  [d] Discard  [esc] Cancel")
	return b.String()
}

// configValueWidth is the width available for wrapping a config field's value.
func (m model) configValueWidth() int {
	w := m.contentWidth() - 2
	if w < 20 {
		w = 20
	}
	return w
}

// renderConfigField renders a single label/value row. The selection treatment
// is deliberately limited to the label, leaving values easy to scan. During
// editing, the current buffer and caret render in place with a subtle value
// background, so the user never needs to look for input below the viewport.
func (m model) renderConfigField(label, value string, focused, protected bool) string {
	prefix := label + ": "
	valueWidth := m.configValueWidth() - lipgloss.Width("  "+prefix)
	if valueWidth < 8 {
		valueWidth = 8
	}
	disabled := protected || m.configReadOnly() || m.config.saving
	editing := focused && m.config.editing
	if editing {
		value = configValueWithCaret(m.config.editBuffer, m.config.editCursor)
	}
	valueLines := wrapOutputLine(value, valueWidth)
	if len(valueLines) == 0 {
		valueLines = []string{""}
	}
	cursor := "  "
	if focused {
		cursor = "▸ "
	}
	labelStyle := configLabelStyle
	valueStyle := configValueStyle
	if disabled {
		labelStyle = configDisabledLabelStyle
		valueStyle = configDisabledValueStyle
	}
	if focused {
		if disabled {
			labelStyle = configDisabledSelectedLabelStyle
		} else {
			labelStyle = configSelectedLabelStyle
		}
	}
	indent := strings.Repeat(" ", lipgloss.Width(prefix))
	var b strings.Builder
	for i, vl := range valueLines {
		if i == 0 {
			b.WriteString(cursor)
			b.WriteString(labelStyle.Render(prefix))
		} else {
			b.WriteString("  ")
			b.WriteString(labelStyle.Render(indent))
		}
		if editing {
			b.WriteString(renderConfigEditingValue(vl, valueStyle))
		} else {
			b.WriteString(valueStyle.Render(vl))
		}
		if i < len(valueLines)-1 {
			b.WriteByte('\n')
		}
	}
	line := b.String()
	if protected {
		line += "  " + mutedStyle.Render("completed stage is protected")
	}
	return line
}

const configCaretMarker = "\x00"

func configValueWithCaret(value string, cursor int) string {
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	return string(runes[:cursor]) + configCaretMarker + string(runes[cursor:])
}

func renderConfigEditingValue(value string, fallback lipgloss.Style) string {
	parts := strings.SplitN(value, configCaretMarker, 2)
	if len(parts) != 2 {
		return fallback.Render(value)
	}
	return configEditingValueStyle.Render(parts[0]) + configEditingCaretStyle.Render(" ") + configEditingValueStyle.Render(parts[1])
}

func configBoolValue(c workflowconfig.ManagedConfig, field configField) bool {
	switch field.path {
	case "scope.backend":
		return c.Scope.Backend
	case "scope.frontend":
		return c.Scope.Frontend
	case "scope.native":
		return c.Scope.Native
	case "scope.script":
		return c.Scope.Script
	case "scope.infrastructure":
		return c.Scope.Infrastructure
	}
	if field.stage != "" && field.agent == "" {
		stage := c.Stages[field.stage]
		switch {
		case strings.HasSuffix(field.path, ".enabled"):
			return stage.Enabled
		case strings.HasSuffix(field.path, ".require_human_approval"):
			return stage.RequireHumanApproval
		case strings.HasSuffix(field.path, ".visual_validation.enabled"):
			return stage.VisualValidation.Enabled
		case strings.HasSuffix(field.path, ".use_playwright"):
			return stage.UsePlaywright
		}
	}
	if field.agent != "" {
		isSubagent := strings.HasSuffix(field.agent, ":subagent")
		agentName := strings.TrimSuffix(field.agent, ":subagent")
		agent := c.FallbackModel
		if agentName != "fallback_model" {
			agent = c.Agents[agentName]
		}
		if isSubagent && strings.HasSuffix(field.path, ".same_of_agent") {
			return agent.Subagent.SameOfAgent
		}
		if isSubagent && strings.HasSuffix(field.path, ".enable_fast_model") {
			return agent.Subagent.EnableFastModel
		}
		if strings.HasSuffix(field.path, ".enable_fast_model") {
			if agentName == "fallback_model" {
				return c.FallbackModel.EnableFastModel
			}
			return c.Agents[agentName].EnableFastModel
		}
	}
	return false
}

func configStageLabel(name string) string {
	parts := strings.Split(name, "_")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}
