package tui

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

type telegramModelSelectionStage string

const (
	telegramModelSelectHarness  telegramModelSelectionStage = "harness"
	telegramModelLoadingModels  telegramModelSelectionStage = "loading-models"
	telegramModelSelectModel    telegramModelSelectionStage = "model"
	telegramModelSelectProperty telegramModelSelectionStage = "property"
)

const telegramModelsPerPage = 35

// telegramModelSelection holds non-secret, per-instance state for the remote
// /model wizard. It uses the same persisted free-chat pair as the TUI picker.
type telegramModelSelection struct {
	address        string
	configAgent    string // empty for the free-chat /model picker
	configSubagent bool
	stage          telegramModelSelectionStage
	harnesses      []string
	models         []string
	modelPage      int
	harnessID      string
	modelSlug      string
	snapshot       modelprops.Snapshot
	keys           []string
	keyIndex       int
	properties     map[string]string
}

type telegramModelListMsg struct {
	address   string
	harnessID string
	models    []string
	err       error
}

func (m model) startTelegramModelSelection(address string) (model, tea.Cmd) {
	return m.startTelegramModelSelectionFor(address, "", false)
}

// startTelegramCycleModelSelection reuses the Telegram /model wizard for one
// workflow-config agent. The only difference from free chat is where the
// completed pair is committed: the active cycle draft instead of hero.json.
func (m model) startTelegramCycleModelSelection(address, agentName string) (model, tea.Cmd) {
	if m.telegram == nil || m.telegram.configWizard == nil {
		return m, nil
	}
	return m.startTelegramModelSelectionFor(address, agentName, false)
}

func (m model) startTelegramCycleSubagentModelSelection(address, agentName string) (model, tea.Cmd) {
	if m.telegram == nil || m.telegram.configWizard == nil {
		return m, nil
	}
	return m.startTelegramModelSelectionFor(address, agentName, true)
}

func (m model) startTelegramModelSelectionFor(address, configAgent string, configSubagent bool) (model, tea.Cmd) {
	if m.telegram == nil {
		return m, nil
	}
	harnesses := m.enabledHarnessIDs()
	if len(harnesses) == 0 {
		m.telegram.modelSelection = nil
		return m, m.telegramOutboundCmd("No harness is enabled. Use /harness in the local TUI first.")
	}
	if configSubagent {
		wizard := m.telegramConfigForAddress(address)
		if wizard == nil {
			return m, m.telegramOutboundCmd("A configuração remota expirou. Execute /hero-config novamente.")
		}
		agent, ok := telegramConfigAgent(wizard.draft, configAgent)
		if !ok || strings.TrimSpace(agent.Harness) == "" {
			return m, m.telegramOutboundCmd("O harness do agente principal precisa ser configurado antes do subagent.")
		}
		parentHarness := strings.TrimSpace(strings.ToLower(agent.Harness))
		if !m.harnessEnabled(parentHarness) {
			return m, m.telegramOutboundCmd("O harness do agente principal não está habilitado. Configure-o primeiro no /harness.")
		}
		harnesses = []string{parentHarness}
	}
	m = m.ensurePropsSvcForTelegram()
	m.telegram.modelSelection = &telegramModelSelection{
		address:        address,
		configAgent:    configAgent,
		configSubagent: configSubagent,
		stage:          telegramModelSelectHarness,
		harnesses:      append([]string(nil), harnesses...),
	}
	title := "Escolha o Harness:"
	if configAgent != "" {
		if configSubagent {
			title = "Escolha o Harness do subagent de " + configStageLabel(configAgent) + ":"
		} else {
			title = "Escolha o Harness para " + configStageLabel(configAgent) + ":"
		}
	}
	var refresh tea.Cmd
	if m.propsSvc != nil && !m.propsRefreshBusy {
		m.propsRefreshBusy = true
		refresh = m.startModelPropsRefresh(harnesses)
	}
	outbound := m.telegramOutboundCmd(telegramNumberedOptions(title, displayHarnesses(harnesses)))
	if refresh != nil {
		return m, tea.Batch(refresh, outbound)
	}
	return m, outbound
}

func (m model) ensurePropsSvcForTelegram() model {
	if m.propsSvc == nil && m.svc != nil {
		m.propsSvc = modelprops.NewService(m.svc.ProjectDir, m.svc.Store, m.svc.Registry, assets.FS)
	}
	if m.propsSvc != nil && m.svc != nil {
		m.propsSvc.Store = m.svc.Store
		m.propsSvc.Registry = m.svc.Registry
	}
	return m
}

func displayHarnesses(ids []string) []string {
	labels := make([]string, 0, len(ids))
	for _, id := range ids {
		labels = append(labels, harnessDisplayName(id))
	}
	return labels
}

func telegramNumberedOptions(title string, options []string) string {
	var b strings.Builder
	b.WriteString(title)
	for i, option := range options {
		fmt.Fprintf(&b, "\n%d - %s", i+1, option)
	}
	b.WriteString("\n\nResponda somente com o número desejado.")
	return b.String()
}

func (m model) handleTelegramModelSelection(address, text string) (model, tea.Cmd) {
	selection := m.telegram.modelSelection
	if selection == nil || selection.address != address {
		return m, nil
	}
	if selection.stage == telegramModelLoadingModels {
		return m, m.telegramOutboundCmd("Estou carregando os modelos. Aguarde a lista de opções.")
	}
	trimmed := strings.TrimSpace(text)
	choice, err := strconv.Atoi(trimmed)
	if err != nil {
		return m, m.telegramOutboundCmd("Opção inválida. Responda somente com um dos números exibidos.")
	}

	switch selection.stage {
	case telegramModelSelectHarness:
		if choice < 1 || choice > len(selection.harnesses) {
			return m, m.telegramOutboundCmd("Opção inválida. Responda somente com um dos números exibidos.")
		}
		selection.harnessID = selection.harnesses[choice-1]
		selection.modelPage = 0
		selection.stage = telegramModelLoadingModels
		return m, tea.Batch(
			m.telegramOutboundCmd("Carregando modelos de "+harnessDisplayName(selection.harnessID)+"…"),
			m.telegramModelListCmd(selection.address, selection.harnessID),
		)

	case telegramModelSelectModel:
		pageModels, _, hasNext := telegramModelPageSlice(selection)
		if choice == 0 {
			if selection.modelPage > 0 {
				selection.modelPage--
				return m, m.telegramModelPromptCmd()
			}
			if hasNext {
				selection.modelPage++
				return m, m.telegramModelPromptCmd()
			}
			return m, m.telegramOutboundCmd("Opção inválida. Responda somente com um dos números exibidos.")
		}
		if hasNext && selection.modelPage > 0 && choice == len(pageModels)+1 {
			selection.modelPage++
			return m, m.telegramModelPromptCmd()
		}
		if choice < 1 || choice > len(pageModels) {
			return m, m.telegramOutboundCmd("Opção inválida. Responda somente com um dos números exibidos.")
		}
		selection.modelSlug = pageModels[choice-1]
		return m.finishTelegramModelSelection()

	case telegramModelSelectProperty:
		if choice < 1 {
			return m, m.telegramOutboundCmd("Opção inválida. Responda somente com um dos números exibidos.")
		}
		key := selection.keys[selection.keyIndex]
		cap := selection.snapshot.Property(key)
		if choice > len(cap.AcceptedValues) {
			return m, m.telegramOutboundCmd("Opção inválida. Responda somente com um dos números exibidos.")
		}
		selection.properties[key] = cap.AcceptedValues[choice-1]
		selection.keyIndex++
		return m.promptTelegramModelProperty()
	}
	return m, nil
}

func (m model) telegramModelListCmd(address, harnessID string) tea.Cmd {
	return func() tea.Msg {
		models, err := listModelsForHarnessFn(context.Background(), m, harnessID)
		return telegramModelListMsg{address: address, harnessID: harnessID, models: models, err: err}
	}
}

func (m model) telegramModelPromptCmd() tea.Cmd {
	return func() tea.Msg { return telegramModelPromptMsg{} }
}

type telegramModelPromptMsg struct{}

func (m model) handleTelegramModelPrompt() (model, tea.Cmd) {
	if m.telegram == nil || m.telegram.modelSelection == nil {
		return m, nil
	}
	selection := m.telegram.modelSelection
	if selection.stage != telegramModelSelectModel || len(selection.models) == 0 {
		return m, nil
	}
	return m, m.telegramOutboundCmd(telegramModelPageText(selection))
}

func (m model) handleTelegramModelList(msg telegramModelListMsg) (model, tea.Cmd) {
	if m.telegram == nil || m.telegram.modelSelection == nil {
		return m, nil
	}
	selection := m.telegram.modelSelection
	if selection.address != msg.address || selection.stage != telegramModelLoadingModels || selection.harnessID != msg.harnessID {
		return m, nil
	}
	m = m.ensurePropsSvcForTelegram()
	current := m.telegramModelCurrentSlug(selection)
	var warning string
	if msg.err == nil {
		// A successful adapter response is authoritative. Do not merge the
		// embedded catalog here: it can contain stale model ids that the
		// selected harness no longer exposes.
		selection.models = liveModelChoices(msg.models)
	} else {
		warning = "Não foi possível atualizar a lista live; usando modelos locais conhecidos.\n\n"
		selection.models = m.modelChoicesForHarness(selection.harnessID, current, nil)
	}
	if len(selection.models) == 0 {
		selection.stage = telegramModelSelectHarness
		if msg.err != nil {
			return m, m.telegramOutboundCmd("Não foi possível listar os modelos de " + harnessDisplayName(msg.harnessID) + ". Escolha outro harness ou tente /model novamente.")
		}
		return m, m.telegramOutboundCmd("Nenhum modelo disponível para " + harnessDisplayName(msg.harnessID) + ". Escolha outro harness ou tente /model novamente.")
	}
	selection.modelPage = 0
	selection.stage = telegramModelSelectModel
	return m, m.telegramOutboundCmd(warning + telegramModelPageText(selection))
}

func (m model) telegramModelCurrentSlug(selection *telegramModelSelection) string {
	if selection == nil {
		return ""
	}
	if selection.configAgent == "" {
		return ""
	}
	wizard := m.telegramConfigForAddress(selection.address)
	if wizard == nil {
		return ""
	}
	agent, ok := telegramConfigReviewAgent(wizard, selection.configAgent)
	if !ok {
		return ""
	}
	if selection.configSubagent {
		if agent.Subagent.SameOfAgent {
			return strings.TrimSpace(agent.Model)
		}
		return strings.TrimSpace(agent.Subagent.Model)
	}
	return strings.TrimSpace(agent.Model)
}

func telegramModelPageSlice(selection *telegramModelSelection) (pageModels []string, hasPrev, hasNext bool) {
	if selection == nil || len(selection.models) == 0 {
		return nil, false, false
	}
	totalPages := telegramModelTotalPages(len(selection.models))
	if selection.modelPage < 0 {
		selection.modelPage = 0
	}
	if selection.modelPage >= totalPages {
		selection.modelPage = totalPages - 1
	}
	start := selection.modelPage * telegramModelsPerPage
	end := start + telegramModelsPerPage
	if end > len(selection.models) {
		end = len(selection.models)
	}
	pageModels = append([]string(nil), selection.models[start:end]...)
	hasPrev = selection.modelPage > 0
	hasNext = selection.modelPage < totalPages-1
	return pageModels, hasPrev, hasNext
}

func telegramModelTotalPages(modelCount int) int {
	if modelCount <= 0 {
		return 0
	}
	return int(math.Ceil(float64(modelCount) / float64(telegramModelsPerPage)))
}

func telegramModelPageText(selection *telegramModelSelection) string {
	pageModels, hasPrev, hasNext := telegramModelPageSlice(selection)
	totalPages := telegramModelTotalPages(len(selection.models))
	title := "Escolha o modelo:"
	if totalPages > 1 {
		title = fmt.Sprintf("Escolha o modelo (página %d/%d):", selection.modelPage+1, totalPages)
	}
	var b strings.Builder
	b.WriteString(title)
	if hasPrev {
		b.WriteString("\n0 - Página anterior")
	}
	for i, slug := range pageModels {
		fmt.Fprintf(&b, "\n%d - %s", i+1, slug)
	}
	if hasNext {
		if hasPrev {
			fmt.Fprintf(&b, "\n%d - Próxima página", len(pageModels)+1)
		} else {
			b.WriteString("\n0 - Próxima página")
		}
	}
	b.WriteString("\n\nResponda somente com o número desejado.")
	return b.String()
}

func (m model) finishTelegramModelSelection() (model, tea.Cmd) {
	selection := m.telegram.modelSelection
	m = m.ensurePropsSvcForTelegram()
	if m.propsSvc == nil || m.svc == nil {
		m.telegram.modelSelection = nil
		return m, m.telegramOutboundCmd("Não foi possível carregar as propriedades do modelo.")
	}
	selection.snapshot = m.propsSvc.Snapshot(selection.harnessID, selection.modelSlug)
	var saved map[string]string
	if selection.configAgent != "" {
		wizard := m.telegramConfigForAddress(selection.address)
		if wizard == nil {
			m.telegram.modelSelection = nil
			return m, m.telegramOutboundCmd("A configuração remota expirou. Execute /hero-config novamente.")
		}
		agent, ok := telegramConfigAgent(wizard.draft, selection.configAgent)
		if !ok {
			m.telegram.modelSelection = nil
			return m, m.telegramOutboundCmd("O agente selecionado não existe na configuração atual.")
		}
		if selection.configSubagent {
			saved = telegramConfigSubagentProperties(agent)
		} else {
			saved = telegramConfigAgentProperties(agent)
		}
	} else {
		hero, err := install.LoadHeroJSON(m.svc.ProjectDir)
		if err != nil {
			m.telegram.modelSelection = nil
			return m, m.telegramOutboundCmd("Não foi possível carregar a configuração do Hero: " + err.Error())
		}
		saved = install.EffectivePairProperties(hero, selection.harnessID, selection.modelSlug)
	}
	selection.properties, _ = modelprops.EffectiveValues(selection.snapshot, saved)
	selection.properties = mergeLockedPropertyDraft(selection.snapshot, selection.properties)
	selection.keys = selection.snapshot.SelectableKeys()
	selection.keyIndex = 0
	if len(selection.keys) == 0 {
		return m.commitTelegramModelSelection()
	}
	selection.stage = telegramModelSelectProperty
	return m.promptTelegramModelProperty()
}

func (m model) promptTelegramModelProperty() (model, tea.Cmd) {
	selection := m.telegram.modelSelection
	if selection == nil || selection.keyIndex >= len(selection.keys) {
		return m.commitTelegramModelSelection()
	}
	key := selection.keys[selection.keyIndex]
	cap := selection.snapshot.Property(key)
	return m, m.telegramOutboundCmd(telegramNumberedOptions(friendlyPropertyName(key)+":", cap.AcceptedValues))
}

func (m model) commitTelegramModelSelection() (model, tea.Cmd) {
	selection := m.telegram.modelSelection
	if selection == nil || m.svc == nil {
		return m, nil
	}
	if selection.configAgent != "" {
		wizard := m.telegramConfigForAddress(selection.address)
		if wizard == nil {
			m.telegram.modelSelection = nil
			return m, m.telegramOutboundCmd("A configuração remota expirou. Execute /hero-config novamente.")
		}
		agent, ok := telegramConfigAgent(wizard.draft, selection.configAgent)
		if !ok {
			m.telegram.modelSelection = nil
			return m, m.telegramOutboundCmd("O agente selecionado não existe na configuração atual.")
		}
		if selection.configSubagent {
			if !strings.EqualFold(strings.TrimSpace(agent.Harness), strings.TrimSpace(selection.harnessID)) {
				m.telegram.modelSelection = nil
				return m, m.telegramOutboundCmd("O subagent deve usar o mesmo harness do agente principal.")
			}
			agent.Subagent.SameOfAgent = false
			agent.Subagent.Model = selection.modelSlug
			agent.Subagent = telegramConfigApplySubagentProperties(agent.Subagent, selection.properties)
		} else {
			agent.Harness = selection.harnessID
			agent.Model = selection.modelSlug
			agent = telegramConfigApplyProperties(agent, selection.properties)
			// Selecting a new parent agent/model invalidates the previous
			// dedicated subagent choice as a default. The subagent inherits the
			// newly selected parent model until the user explicitly chooses a
			// dedicated model in the next subagent step. Its harness is implicit
			// and therefore always remains the parent's harness.
			if selection.configAgent != "fallback_model" {
				agent.Subagent.SameOfAgent = true
			}
		}
		if selection.configAgent == "fallback_model" {
			wizard.draft.FallbackModel = agent
		} else {
			if wizard.draft.Agents == nil {
				wizard.draft.Agents = make(map[string]workflowconfig.AgentModelConfig)
			}
			wizard.draft.Agents[selection.configAgent] = agent
		}
		m.telegram.modelSelection = nil
		wizard.modelIndex++
		next, cmd := m.telegramConfigModelPrompt()
		label := "Modelo de " + configStageLabel(selection.configAgent)
		if selection.configSubagent {
			label = "Modelo do subagent de " + configStageLabel(selection.configAgent)
		}
		return next, combineTimerCmds(
			m.telegramOutboundCmd(label+" salvo."),
			cmd,
		)
	}
	if err := install.CommitModelSelection(m.svc.ProjectDir, selection.harnessID, selection.modelSlug, selection.properties); err != nil {
		m.telegram.modelSelection = nil
		return m, m.telegramOutboundCmd("Não foi possível salvar o modelo: " + err.Error())
	}
	m.chatHarnessID = selection.harnessID
	m.chatModelSlug = selection.modelSlug
	m = m.loadFreechatProps()
	m.telegram.modelSelection = nil
	return m, m.telegramOutboundCmd(fmt.Sprintf("Modelo selecionado: %s · %s", selection.modelSlug, harnessDisplayName(selection.harnessID)))
}
