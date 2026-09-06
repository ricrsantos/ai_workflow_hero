package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

type telegramConfigWizardStep string

const (
	telegramConfigLoading        telegramConfigWizardStep = "loading"
	telegramConfigTitle          telegramConfigWizardStep = "title"
	telegramConfigObjective      telegramConfigWizardStep = "objective"
	telegramConfigLanguage       telegramConfigWizardStep = "language"
	telegramConfigScope          telegramConfigWizardStep = "scope"
	telegramConfigStages         telegramConfigWizardStep = "stages"
	telegramConfigModelsQuestion telegramConfigWizardStep = "models-question"
	telegramConfigModelChoice    telegramConfigWizardStep = "model-choice"
	telegramConfigSummary        telegramConfigWizardStep = "summary"
	telegramConfigSaving         telegramConfigWizardStep = "saving"
)

var telegramConfigStageOrder = []string{
	"research",
	"planning",
	"implementation",
	"qa",
	"judge",
	"browser_ui_validation",
	"qa_end_to_end",
}

var telegramConfigScopeOrder = []string{
	"backend",
	"frontend",
	"native",
	"script",
	"infrastructure",
}

// telegramConfigWizard owns one uncommitted cycle-configuration draft. It is
// deliberately kept at the TUI edge: the daemon routes Telegram frames, while
// this project-local process owns YAML, cycle synchronization, and validation.
type telegramConfigWizard struct {
	address     string
	cycleNumber int
	step        telegramConfigWizardStep

	doc      *workflowconfig.Document
	baseline workflowconfig.ManagedConfig
	draft    workflowconfig.ManagedConfig

	modelTargets []string
	modelIndex   int
	saving       bool
}

type telegramConfigLoadedMsg struct {
	address     string
	cycleNumber int
	doc         *workflowconfig.Document
	err         error
}

type telegramConfigSavedMsg struct {
	address     string
	cycleNumber int
	doc         *workflowconfig.Document
	err         error
}

type telegramConfigShowMsg struct {
	address     string
	cycleNumber int
	config      workflowconfig.ManagedConfig
	err         error
}

// startTelegramConfig starts the explicit /hero-config wizard. Loading is
// asynchronous so a filesystem or SQLite delay never blocks Bubble Tea's
// Update loop.
func (m model) startTelegramConfig(address string) (model, tea.Cmd) {
	if m.telegram == nil || m.svc == nil || m.freeChatMode {
		return m, m.telegramOutboundCmd("A configuração de ciclo está disponível apenas em um projeto com ciclo ativo.")
	}
	if !m.hasActiveCycle() {
		return m, m.telegramOutboundCmd("Nenhum ciclo ativo. Execute /hero-new primeiro.")
	}
	if m.streaming || m.actionBusy || m.heroStartBootstrapping || m.heroStartPreparing {
		return m, m.telegramOutboundCmd("O Hero está executando uma tarefa. Aguarde a conclusão antes de configurar o ciclo.")
	}
	if m.config.dirty {
		return m, m.telegramOutboundCmd("Há alterações não salvas na tela Config local. Salve ou descarte-as antes de usar o Telegram.")
	}
	if m.telegram.configWizard != nil {
		if m.telegram.configWizard.address == address {
			return m, m.telegramOutboundCmd("A configuração já está em andamento. Responda à pergunta atual ou use /hero-config-show.")
		}
		return m, m.telegramOutboundCmd("Já existe uma configuração remota em andamento para esta instância.")
	}

	// Starting cycle configuration explicitly cancels a free-chat /model draft;
	// no partial free-chat choice is committed by that cancellation.
	m.telegram.modelSelection = nil
	m.telegram.configWizard = &telegramConfigWizard{
		address:     address,
		cycleNumber: m.status.CycleNumber,
		step:        telegramConfigLoading,
	}
	return m, tea.Batch(
		m.telegramOutboundCmd("Carregando a configuração do ciclo…"),
		m.telegramConfigLoadCmd(address, m.status.CycleNumber),
	)
}

func (m model) telegramConfigLoadCmd(address string, cycleNumber int) tea.Cmd {
	projectDir := ""
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
	}
	return func() tea.Msg {
		doc, err := workflowconfig.LoadCurrentDocument(projectDir)
		return telegramConfigLoadedMsg{
			address:     address,
			cycleNumber: cycleNumber,
			doc:         doc,
			err:         err,
		}
	}
}

func (m model) handleTelegramConfigLoaded(msg telegramConfigLoadedMsg) (model, tea.Cmd) {
	w := m.telegramConfigForAddress(msg.address)
	if w == nil || w.step != telegramConfigLoading {
		return m, nil
	}
	if msg.err != nil || msg.doc == nil {
		m.telegram.configWizard = nil
		if msg.err != nil {
			return m, m.telegramOutboundCmd("Não foi possível carregar workflow-config.yml: " + msg.err.Error())
		}
		return m, m.telegramOutboundCmd("Não foi possível carregar workflow-config.yml: documento vazio.")
	}
	w.doc = msg.doc
	w.cycleNumber = msg.cycleNumber
	w.baseline = msg.doc.Config
	w.draft = msg.doc.Config
	w.step = telegramConfigTitle
	return m, m.telegramOutboundCmd(m.telegramConfigPrompt())
}

func (m model) telegramConfigForAddress(address string) *telegramConfigWizard {
	if m.telegram == nil || m.telegram.configWizard == nil {
		return nil
	}
	if m.telegram.configWizard.address != address {
		return nil
	}
	return m.telegram.configWizard
}

// handleTelegramConfigCommand handles Telegram-only cycle configuration
// commands before normal slash dispatch. It returns handled=true when the
// command belongs to this feature, including malformed subcommands.
func (m model) handleTelegramConfigCommand(text, address string) (model, tea.Cmd, bool) {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	if lower == "/hero-config-show" {
		if w := m.telegramConfigForAddress(address); w != nil && w.doc != nil {
			return m, m.telegramOutboundCmd(formatTelegramConfig(w.cycleNumber, w.draft, true)), true
		}
		next, cmd := m.startTelegramConfigShow(address)
		return next, cmd, true
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 || !strings.EqualFold(parts[0], "/hero-config") {
		return m, nil, false
	}
	arg := ""
	if len(trimmed) > len(parts[0]) {
		arg = strings.TrimSpace(trimmed[len(parts[0]):])
	}
	switch strings.ToLower(arg) {
	case "", "start", "iniciar":
		next, cmd := m.startTelegramConfig(address)
		return next, cmd, true
	case "show", "status", "ver":
		if w := m.telegramConfigForAddress(address); w != nil && w.doc != nil {
			return m, m.telegramOutboundCmd(formatTelegramConfig(w.cycleNumber, w.draft, true)), true
		}
		next, cmd := m.startTelegramConfigShow(address)
		return next, cmd, true
	case "cancel", "cancelar":
		if m.telegram != nil && m.telegram.configWizard != nil && m.telegram.configWizard.address == address {
			m.telegram.configWizard = nil
			m.telegram.modelSelection = nil
			return m, m.telegramOutboundCmd("Configuração do ciclo cancelada. Nenhuma alteração foi salva."), true
		}
		return m, m.telegramOutboundCmd("Não há uma configuração de ciclo em andamento."), true
	case "save", "salvar":
		w := m.telegramConfigForAddress(address)
		if w == nil {
			return m, m.telegramOutboundCmd("Nenhuma configuração em andamento. Execute /hero-config."), true
		}
		if w.step != telegramConfigSummary {
			return m, m.telegramOutboundCmd("Ainda há perguntas pendentes. Responda à pergunta atual antes de salvar."), true
		}
		next, cmd := m.beginTelegramConfigSave()
		return next, cmd, true
	default:
		return m, m.telegramOutboundCmd("Uso: /hero-config, /hero-config-show ou /hero-config cancel."), true
	}
}

func (m model) startTelegramConfigShow(address string) (model, tea.Cmd) {
	if m.telegram == nil || m.svc == nil {
		return m, nil
	}
	if !m.hasActiveCycle() {
		return m, m.telegramOutboundCmd("Nenhum ciclo ativo. Execute /hero-new primeiro.")
	}
	projectDir := m.svc.ProjectDir
	cycleNumber := m.status.CycleNumber
	return m, func() tea.Msg {
		doc, err := workflowconfig.LoadCurrentDocument(projectDir)
		if err != nil {
			return telegramConfigShowMsg{address: address, cycleNumber: cycleNumber, err: err}
		}
		return telegramConfigShowMsg{
			address:     address,
			cycleNumber: cycleNumber,
			config:      doc.Config,
		}
	}
}

func (m model) handleTelegramConfigShow(msg telegramConfigShowMsg) (model, tea.Cmd) {
	if m.telegram == nil {
		return m, nil
	}
	if msg.err != nil {
		return m, m.telegramOutboundCmd("Não foi possível ler a configuração: " + msg.err.Error())
	}
	return m, m.telegramOutboundCmd(formatTelegramConfig(msg.cycleNumber, msg.config, false))
}

func (m model) handleTelegramConfigInput(address, text string) (model, tea.Cmd) {
	w := m.telegramConfigForAddress(address)
	if w == nil {
		return m, nil
	}
	if w.saving || w.step == telegramConfigSaving {
		return m, m.telegramOutboundCmd("Salvando a configuração. Aguarde…")
	}
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "/") {
		return m, m.telegramOutboundCmd("Finalize o wizard atual ou use /hero-config cancel para cancelá-lo.")
	}

	switch w.step {
	case telegramConfigTitle:
		if telegramConfigKeep(trimmed) {
			if strings.TrimSpace(w.draft.Title) == "" {
				return m, m.telegramConfigInvalid("O título atual está vazio; informe um título.")
			}
		} else {
			if trimmed == "" {
				return m, m.telegramConfigInvalid("O título não pode ficar vazio.")
			}
			w.draft.Title = trimmed
		}
		w.step = telegramConfigObjective
		return m, m.telegramOutboundCmd(m.telegramConfigPrompt())

	case telegramConfigObjective:
		if telegramConfigKeep(trimmed) {
			if strings.TrimSpace(w.draft.Objective) == "" {
				return m, m.telegramConfigInvalid("O objetivo atual está vazio; informe um objetivo.")
			}
		} else {
			if trimmed == "" {
				return m, m.telegramConfigInvalid("O objetivo não pode ficar vazio.")
			}
			w.draft.Objective = trimmed
		}
		w.step = telegramConfigLanguage
		return m, m.telegramOutboundCmd(m.telegramConfigPrompt())

	case telegramConfigLanguage:
		if telegramConfigKeep(trimmed) {
			if strings.TrimSpace(w.draft.WorkflowConfig.UserPreferredLanguage) == "" {
				return m, m.telegramConfigInvalid("O idioma atual está vazio; informe um idioma.")
			}
		} else {
			if trimmed == "" {
				return m, m.telegramConfigInvalid("Informe um idioma ou responda 'manter'.")
			}
			w.draft.WorkflowConfig.UserPreferredLanguage = trimmed
		}
		w.step = telegramConfigScope
		return m, m.telegramOutboundCmd(m.telegramConfigPrompt())

	case telegramConfigScope:
		if !telegramConfigKeep(trimmed) {
			selected, err := parseTelegramConfigNumberSet(trimmed, len(telegramConfigScopeOrder))
			if err != nil {
				return m, m.telegramConfigInvalid("Escopo inválido. Use números separados por vírgula, por exemplo: 1,2.")
			}
			w.draft.Scope = workflowconfig.Scope{}
			for _, index := range selected {
				switch telegramConfigScopeOrder[index-1] {
				case "backend":
					w.draft.Scope.Backend = true
				case "frontend":
					w.draft.Scope.Frontend = true
				case "native":
					w.draft.Scope.Native = true
				case "script":
					w.draft.Scope.Script = true
				case "infrastructure":
					w.draft.Scope.Infrastructure = true
				}
			}
		}
		w.step = telegramConfigStages
		return m, m.telegramOutboundCmd(m.telegramConfigPrompt())

	case telegramConfigStages:
		if !telegramConfigKeep(trimmed) {
			selected, err := parseTelegramConfigNumberSet(trimmed, len(telegramConfigStageOrder))
			if err != nil {
				return m, m.telegramConfigInvalid("Stages inválidos. Use os números separados por vírgula exibidos acima.")
			}
			selectedSet := make(map[int]bool, len(selected))
			for _, index := range selected {
				selectedSet[index] = true
			}
			for index, name := range telegramConfigStageOrder {
				stage, ok := w.draft.Stages[name]
				if !ok {
					continue
				}
				stage.Enabled = selectedSet[index+1]
				w.draft.Stages[name] = stage
			}
		}
		w.step = telegramConfigModelsQuestion
		return m, m.telegramOutboundCmd(m.telegramConfigPrompt())

	case telegramConfigModelsQuestion:
		if telegramConfigYes(trimmed) {
			return m.beginTelegramConfigModelReview()
		}
		if telegramConfigNo(trimmed) {
			w.step = telegramConfigSummary
			return m, m.telegramOutboundCmd(m.telegramConfigPrompt())
		}
		return m, m.telegramConfigInvalid("Responda 1 para revisar os modelos ou 2 para manter os atuais.")

	case telegramConfigModelChoice:
		if telegramConfigYes(trimmed) {
			return m.startTelegramCycleModelSelection(address, w.modelTargets[w.modelIndex])
		}
		if telegramConfigNo(trimmed) {
			w.modelIndex++
			return m.telegramConfigModelPrompt()
		}
		return m, m.telegramConfigInvalid("Responda 1 para escolher outro modelo ou 2 para manter o atual.")

	case telegramConfigSummary:
		switch trimmed {
		case "1":
			return m.beginTelegramConfigSave()
		case "2":
			return m.beginTelegramConfigModelReview()
		case "3":
			m.telegram.configWizard = nil
			return m, m.telegramOutboundCmd("Configuração cancelada. Nenhuma alteração foi salva.")
		default:
			return m, m.telegramConfigInvalid("Responda 1 para salvar, 2 para revisar modelos ou 3 para cancelar.")
		}
	}
	return m, nil
}

func (m model) telegramConfigInvalid(message string) tea.Cmd {
	return m.telegramOutboundCmd(message + "\n\n" + m.telegramConfigPrompt())
}

func (m model) telegramConfigPrompt() string {
	w := m.telegram.configWizard
	if w == nil {
		return ""
	}
	switch w.step {
	case telegramConfigTitle:
		return fmt.Sprintf("Configuração do ciclo C%d\n\nTítulo atual: %s\n\nEnvie o novo título ou responda 'manter'.", w.cycleNumber, telegramConfigValue(w.draft.Title))
	case telegramConfigObjective:
		return fmt.Sprintf("Objetivo atual: %s\n\nEnvie o novo objetivo ou responda 'manter'.", telegramConfigValue(w.draft.Objective))
	case telegramConfigLanguage:
		return fmt.Sprintf("Idioma atual do chat: %s\n\nEnvie o idioma, por exemplo PT-BR, ou responda 'manter'.", telegramConfigValue(w.draft.WorkflowConfig.UserPreferredLanguage))
	case telegramConfigScope:
		return telegramConfigScopePrompt(w.draft.Scope)
	case telegramConfigStages:
		return telegramConfigStagesPrompt(w.draft)
	case telegramConfigModelsQuestion:
		return "Deseja revisar os harnesses e modelos dos agentes do ciclo?\n\n1 - Escolher pelo wizard remoto\n2 - Manter os modelos atuais"
	case telegramConfigModelChoice:
		return m.telegramConfigModelText()
	case telegramConfigSummary:
		return formatTelegramConfig(w.cycleNumber, w.draft, true) + "\n\n1 - Salvar configuração\n2 - Revisar modelos\n3 - Cancelar"
	case telegramConfigSaving:
		return "Salvando a configuração…"
	default:
		return ""
	}
}

func telegramConfigScopePrompt(scope workflowconfig.Scope) string {
	var b strings.Builder
	b.WriteString("Quais escopos este ciclo deve cobrir?\n\n")
	for i, name := range telegramConfigScopeOrder {
		marker := " "
		if telegramConfigScopeEnabled(scope, name) {
			marker = "✓"
		}
		fmt.Fprintf(&b, "%d - [%s] %s\n", i+1, marker, configStageLabel(name))
	}
	b.WriteString("\nEnvie os números separados por vírgula, por exemplo 1,2, ou responda 'manter'.")
	return b.String()
}

func telegramConfigStagesPrompt(cfg workflowconfig.ManagedConfig) string {
	var b strings.Builder
	b.WriteString("Quais stages devem ficar habilitados?\n\n")
	for i, name := range telegramConfigStageOrder {
		stage, ok := cfg.Stages[name]
		if !ok {
			continue
		}
		marker := " "
		if stage.Enabled {
			marker = "✓"
		}
		fmt.Fprintf(&b, "%d - [%s] %s\n", i+1, marker, configStageLabel(name))
	}
	b.WriteString("\nEnvie os números separados por vírgula ou responda 'manter'. Os budgets e aprovações atuais serão preservados.")
	return b.String()
}

func (m model) beginTelegramConfigModelReview() (model, tea.Cmd) {
	w := m.telegram.configWizard
	if w == nil {
		return m, nil
	}
	w.modelTargets = telegramConfigModelTargets(w.draft)
	w.modelIndex = 0
	if len(w.modelTargets) == 0 {
		w.step = telegramConfigSummary
		return m, m.telegramOutboundCmd(m.telegramConfigPrompt())
	}
	w.step = telegramConfigModelChoice
	return m.telegramConfigModelPrompt()
}

func (m model) telegramConfigModelPrompt() (model, tea.Cmd) {
	w := m.telegram.configWizard
	if w == nil {
		return m, nil
	}
	for w.modelIndex < len(w.modelTargets) {
		target := w.modelTargets[w.modelIndex]
		agent, ok := telegramConfigReviewAgent(w, target)
		if !ok {
			w.modelIndex++
			continue
		}
		if strings.TrimSpace(agent.Harness) == "" || strings.TrimSpace(agent.Model) == "" {
			return m.startTelegramCycleModelSelection(w.address, target)
		}
		return m, m.telegramOutboundCmd(telegramConfigModelTextFor(target, agent))
	}
	w.step = telegramConfigSummary
	return m, m.telegramOutboundCmd(m.telegramConfigPrompt())
}

func (m model) telegramConfigModelText() string {
	w := m.telegram.configWizard
	if w == nil || w.modelIndex >= len(w.modelTargets) {
		return ""
	}
	agent, ok := telegramConfigAgent(w.draft, w.modelTargets[w.modelIndex])
	if !ok {
		return ""
	}
	return telegramConfigModelTextFor(w.modelTargets[w.modelIndex], agent)
}

func telegramConfigReviewAgent(w *telegramConfigWizard, target string) (workflowconfig.AgentModelConfig, bool) {
	if w == nil {
		return workflowconfig.AgentModelConfig{}, false
	}
	if target == "fallback_model" {
		return w.draft.FallbackModel, true
	}
	if agent, ok := w.draft.Agents[target]; ok {
		return agent, true
	}
	if w.draft.Agents == nil {
		w.draft.Agents = make(map[string]workflowconfig.AgentModelConfig)
	}
	agent := workflowconfig.AgentModelConfig{Subagent: workflowconfig.SubagentConfig{SameOfAgent: true}}
	w.draft.Agents[target] = agent
	return agent, true
}

func telegramConfigModelTextFor(target string, agent workflowconfig.AgentModelConfig) string {
	if strings.TrimSpace(agent.Harness) == "" || strings.TrimSpace(agent.Model) == "" {
		return fmt.Sprintf("Modelo de %s ainda não configurado.\n\n1 - Escolher modelo\n2 - Manter o atual",
			configStageLabel(target))
	}
	return fmt.Sprintf(
		"Modelo de %s\nAtual: %s\n\n1 - Escolher outro modelo\n2 - Manter o atual",
		configStageLabel(target), telegramConfigAgentPair(agent),
	)
}

func telegramConfigModelTargets(cfg workflowconfig.ManagedConfig) []string {
	targets := append([]string(nil), cfg.RequiredAgentNames()...)
	targets = append(targets, "fallback_model")
	seen := make(map[string]bool, len(targets))
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

func telegramConfigAgent(cfg workflowconfig.ManagedConfig, name string) (workflowconfig.AgentModelConfig, bool) {
	if name == "fallback_model" {
		return cfg.FallbackModel, true
	}
	agent, ok := cfg.Agents[name]
	return agent, ok
}

func telegramConfigScopeEnabled(scope workflowconfig.Scope, name string) bool {
	switch name {
	case "backend":
		return scope.Backend
	case "frontend":
		return scope.Frontend
	case "native":
		return scope.Native
	case "script":
		return scope.Script
	case "infrastructure":
		return scope.Infrastructure
	default:
		return false
	}
}

func telegramConfigValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(vazio)"
	}
	return value
}

func telegramConfigKeep(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "manter", "keep", "atual", "current", "-":
		return true
	default:
		return false
	}
}

func telegramConfigYes(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "sim", "s", "yes", "y":
		return true
	default:
		return false
	}
}

func telegramConfigNo(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "2", "não", "nao", "n", "no":
		return true
	default:
		return false
	}
}

func parseTelegramConfigNumberSet(text string, max int) ([]int, error) {
	text = strings.NewReplacer(",", " ", ";", " ").Replace(text)
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty selection")
	}
	seen := make(map[int]bool, len(fields))
	selected := make([]int, 0, len(fields))
	for _, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil || value < 1 || value > max || seen[value] {
			return nil, fmt.Errorf("invalid selection %q", field)
		}
		seen[value] = true
		selected = append(selected, value)
	}
	return selected, nil
}

// formatTelegramConfig returns a compact, non-secret summary suitable for
// Telegram. It intentionally excludes raw YAML comments and unknown fields.
func formatTelegramConfig(cycleNumber int, cfg workflowconfig.ManagedConfig, draft bool) string {
	var b strings.Builder
	if draft {
		fmt.Fprintf(&b, "Configuração do ciclo C%d (rascunho)\n", cycleNumber)
	} else {
		fmt.Fprintf(&b, "Configuração do ciclo C%d\n", cycleNumber)
	}
	fmt.Fprintf(&b, "\nTítulo: %s\nObjetivo: %s\nIdioma: %s\nEscopo: %s\n",
		telegramConfigValue(cfg.Title),
		telegramConfigValue(cfg.Objective),
		telegramConfigValue(cfg.WorkflowConfig.UserPreferredLanguage),
		telegramConfigScopeSummary(cfg.Scope),
	)

	b.WriteString("\nStages:\n")
	for _, name := range telegramConfigStageOrder {
		stage, ok := cfg.Stages[name]
		if !ok {
			continue
		}
		state := "desativado"
		if stage.Enabled {
			approval := "sem aprovação"
			if stage.RequireHumanApproval {
				approval = "com aprovação"
			}
			state = fmt.Sprintf("ativo · %d iterações · %d min · %s", stage.MaxIterations, stage.TimeoutMinutes, approval)
		}
		fmt.Fprintf(&b, "- %s: %s\n", configStageLabel(name), state)
	}

	b.WriteString("\nModelos:\n")
	for _, name := range telegramConfigDisplayAgentNames(cfg) {
		agent, ok := telegramConfigAgent(cfg, name)
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "- %s: %s\n", configStageLabel(name), telegramConfigAgentPair(agent))
	}
	return strings.TrimSpace(b.String())
}

func telegramConfigScopeSummary(scope workflowconfig.Scope) string {
	values := make([]string, 0, len(telegramConfigScopeOrder))
	for _, name := range telegramConfigScopeOrder {
		if telegramConfigScopeEnabled(scope, name) {
			values = append(values, name)
		}
	}
	if len(values) == 0 {
		return "nenhum"
	}
	return strings.Join(values, ", ")
}

func telegramConfigDisplayAgentNames(cfg workflowconfig.ManagedConfig) []string {
	order := []string{
		"orchestration_agent", "context_agent", "discover_agent", "planning_agent",
		"backend_agent", "frontend_agent", "generic_agent", "qa_agent", "judge_agent",
		"browser_ui_agent", "end2end_qa_agent", "fallback_model",
	}
	names := make([]string, 0, len(order))
	for _, name := range order {
		if name == "fallback_model" {
			names = append(names, name)
			continue
		}
		if _, ok := cfg.Agents[name]; ok {
			names = append(names, name)
		}
	}
	return names
}

func telegramConfigAgentPair(agent workflowconfig.AgentModelConfig) string {
	harnessID := strings.TrimSpace(agent.Harness)
	modelID := strings.TrimSpace(agent.Model)
	if harnessID == "" && modelID == "" {
		return "(não configurado)"
	}
	pair := harnessID + " · " + modelID
	properties := make([]string, 0, 3)
	if agent.EnableFastModel {
		properties = append(properties, "fs=true")
	}
	if value := strings.TrimSpace(agent.Thinking); value != "" && !strings.EqualFold(value, "na") {
		properties = append(properties, "th="+value)
	}
	if value := strings.TrimSpace(agent.ReasoningEffort); value != "" && !strings.EqualFold(value, "na") {
		properties = append(properties, "ef="+value)
	}
	if len(properties) > 0 {
		pair += " [" + strings.Join(properties, ", ") + "]"
	}
	return pair
}

func telegramConfigAgentProperties(agent workflowconfig.AgentModelConfig) map[string]string {
	return workflowconfig.EffectiveProperties(agent)
}

func telegramConfigApplyProperties(agent workflowconfig.AgentModelConfig, properties map[string]string) workflowconfig.AgentModelConfig {
	if value, ok := properties[harness.PropertyFast]; ok {
		agent.EnableFastModel = strings.EqualFold(strings.TrimSpace(value), "true")
	}
	if value := strings.TrimSpace(properties[harness.PropertyThink]); value != "" {
		agent.Thinking = value
	} else {
		agent.Thinking = "na"
	}
	if value := strings.TrimSpace(properties[harness.PropertyEffort]); value != "" {
		agent.ReasoningEffort = value
	} else {
		agent.ReasoningEffort = "na"
	}
	return agent
}

func (m model) beginTelegramConfigSave() (model, tea.Cmd) {
	w := m.telegram.configWizard
	if w == nil || w.doc == nil {
		return m, m.telegramOutboundCmd("Nenhuma configuração carregada. Execute /hero-config novamente.")
	}
	if m.streaming || m.actionBusy || m.heroStartBootstrapping || m.heroStartPreparing {
		return m, m.telegramOutboundCmd("O Hero está ocupado. Aguarde a conclusão antes de salvar.")
	}
	if m.config.dirty {
		return m, m.telegramOutboundCmd("Há alterações não salvas na tela Config local. Salve ou descarte-as antes de continuar.")
	}
	if err := w.draft.Validate(m.configValidationOptions()); err != nil {
		return m, m.telegramOutboundCmd("Configuração inválida: " + err.Error() + "\n\n" + m.telegramConfigPrompt())
	}
	w.saving = true
	w.step = telegramConfigSaving
	doc := w.doc
	draft := w.draft
	baseline := w.baseline
	address := w.address
	cycleNumber := w.cycleNumber
	return m, func() tea.Msg {
		updated, _, err := m.persistWorkflowConfigDraft(doc, draft, baseline)
		return telegramConfigSavedMsg{
			address:     address,
			cycleNumber: cycleNumber,
			doc:         updated,
			err:         err,
		}
	}
}

func (m model) handleTelegramConfigSaved(msg telegramConfigSavedMsg) (model, tea.Cmd) {
	w := m.telegramConfigForAddress(msg.address)
	if w == nil || !w.saving {
		return m, nil
	}
	if msg.err != nil {
		w.saving = false
		w.step = telegramConfigSummary
		return m, m.telegramOutboundCmd("Não foi possível salvar a configuração: " + msg.err.Error() + "\n\n" + m.telegramConfigPrompt())
	}
	if msg.doc != nil && m.config.doc != nil && !m.config.dirty {
		m.config.doc = msg.doc
		m.config.baseline = msg.doc.Config
		m.config.draft = msg.doc.Config
		m.config.dirty = false
	}
	m.telegram.configWizard = nil
	message := fmt.Sprintf("✓ Configuração do ciclo C%d salva e validada.\n\nUse /hero-start para iniciar o ciclo.", msg.cycleNumber)
	return m, tea.Batch(m.telegramOutboundCmd(message), m.refreshCmd())
}
