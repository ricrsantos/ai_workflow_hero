package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/modelprops"
)

type telegramModelSelectionStage string

const (
	telegramModelSelectHarness  telegramModelSelectionStage = "harness"
	telegramModelLoadingModels  telegramModelSelectionStage = "loading-models"
	telegramModelSelectModel    telegramModelSelectionStage = "model"
	telegramModelSelectProperty telegramModelSelectionStage = "property"
)

// telegramModelSelection holds non-secret, per-instance state for the remote
// /model wizard. It uses the same persisted free-chat pair as the TUI picker.
type telegramModelSelection struct {
	address    string
	stage      telegramModelSelectionStage
	harnesses  []string
	models     []string
	harnessID  string
	modelSlug  string
	snapshot   modelprops.Snapshot
	keys       []string
	keyIndex   int
	properties map[string]string
}

type telegramModelListMsg struct {
	address   string
	harnessID string
	models    []string
	err       error
}

func (m model) startTelegramModelSelection(address string) (model, tea.Cmd) {
	if m.telegram == nil {
		return m, nil
	}
	harnesses := m.enabledHarnessIDs()
	if len(harnesses) == 0 {
		m.telegram.modelSelection = nil
		return m, m.telegramOutboundCmd("No harness is enabled. Use /harness in the local TUI first.")
	}
	m.telegram.modelSelection = &telegramModelSelection{
		address:   address,
		stage:     telegramModelSelectHarness,
		harnesses: append([]string(nil), harnesses...),
	}
	return m, m.telegramOutboundCmd(telegramNumberedOptions("Escolha o Harness:", displayHarnesses(harnesses)))
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
	choice, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || choice < 1 {
		return m, m.telegramOutboundCmd("Opção inválida. Responda somente com um dos números exibidos.")
	}

	switch selection.stage {
	case telegramModelSelectHarness:
		if choice > len(selection.harnesses) {
			return m, m.telegramOutboundCmd("Opção inválida. Responda somente com um dos números exibidos.")
		}
		selection.harnessID = selection.harnesses[choice-1]
		if models := m.modelsForHarness(selection.harnessID); len(models) > 0 {
			selection.models = append([]string(nil), models...)
			selection.stage = telegramModelSelectModel
			return m, m.telegramOutboundCmd(telegramNumberedOptions("Escolha o modelo:", selection.models))
		}
		selection.stage = telegramModelLoadingModels
		return m, m.telegramModelListCmd(selection.address, selection.harnessID)

	case telegramModelSelectModel:
		if choice > len(selection.models) {
			return m, m.telegramOutboundCmd("Opção inválida. Responda somente com um dos números exibidos.")
		}
		selection.modelSlug = selection.models[choice-1]
		return m.finishTelegramModelSelection()

	case telegramModelSelectProperty:
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

func (m model) handleTelegramModelList(msg telegramModelListMsg) (model, tea.Cmd) {
	if m.telegram == nil || m.telegram.modelSelection == nil {
		return m, nil
	}
	selection := m.telegram.modelSelection
	if selection.address != msg.address || selection.stage != telegramModelLoadingModels || selection.harnessID != msg.harnessID {
		return m, nil
	}
	if msg.err != nil || len(msg.models) == 0 {
		selection.stage = telegramModelSelectHarness
		if msg.err != nil {
			return m, m.telegramOutboundCmd("Não foi possível listar os modelos de " + harnessDisplayName(msg.harnessID) + ". Escolha outro harness ou tente /model novamente.")
		}
		return m, m.telegramOutboundCmd("Nenhum modelo disponível para " + harnessDisplayName(msg.harnessID) + ". Escolha outro harness ou tente /model novamente.")
	}
	selection.models = append([]string(nil), msg.models...)
	selection.stage = telegramModelSelectModel
	return m, m.telegramOutboundCmd(telegramNumberedOptions("Escolha o modelo:", selection.models))
}

func (m model) finishTelegramModelSelection() (model, tea.Cmd) {
	selection := m.telegram.modelSelection
	if m.propsSvc == nil && m.svc != nil {
		m.propsSvc = modelprops.NewService(m.svc.ProjectDir, m.svc.Store, m.svc.Registry, assets.FS)
	}
	if m.propsSvc == nil || m.svc == nil {
		m.telegram.modelSelection = nil
		return m, m.telegramOutboundCmd("Não foi possível carregar as propriedades do modelo.")
	}
	selection.snapshot = m.propsSvc.Snapshot(selection.harnessID, selection.modelSlug)
	hero, err := install.LoadHeroJSON(m.svc.ProjectDir)
	if err != nil {
		m.telegram.modelSelection = nil
		return m, m.telegramOutboundCmd("Não foi possível carregar a configuração do Hero: " + err.Error())
	}
	selection.properties, _ = modelprops.EffectiveValues(selection.snapshot, install.EffectivePairProperties(hero, selection.harnessID, selection.modelSlug))
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
