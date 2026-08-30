package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

// configModelPickerMax is the slash-overlay-sized window for a harness catalog.
// The visible row count still shrinks with the Config pane height.
const configModelPickerMax = 8

func (m model) openConfigModelPicker(field configField) model {
	agent := m.configAgentForField(field)
	current := m.configFieldValue(field)
	choices := m.configModelChoices(agent.Harness, current)
	if len(choices) == 0 {
		m.config.message = "⚠ Model catalog unavailable; existing YAML model retained."
		return m
	}
	m.config.modelPicker = true
	m.config.pickerField = field
	m.config.pickerItems = choices
	m.config.pickerIndex = configChoiceIndex(choices, current)
	m.config.pickerOffset = 0
	m.config.message = ""
	return m.ensureConfigModelPickerVisible()
}

func (m model) closeConfigModelPicker() model {
	m.config.modelPicker = false
	m.config.pickerField = configField{}
	m.config.pickerItems = nil
	m.config.pickerIndex = 0
	m.config.pickerOffset = 0
	return m.configEnsureFocusVisible()
}

func (m model) handleConfigModelPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, configKeys.Cancel):
		return m.closeConfigModelPicker(), nil
	case key.Matches(msg, configKeys.Previous):
		if m.config.pickerIndex > 0 {
			m.config.pickerIndex--
		}
		return m.ensureConfigModelPickerVisible(), nil
	case key.Matches(msg, configKeys.Next):
		if m.config.pickerIndex < len(m.config.pickerItems)-1 {
			m.config.pickerIndex++
		}
		return m.ensureConfigModelPickerVisible(), nil
	case key.Matches(msg, configKeys.Edit):
		return m.applyConfigModelPicker(), nil
	}
	return m, nil
}

func (m model) applyConfigModelPicker() model {
	if len(m.config.pickerItems) == 0 {
		return m.closeConfigModelPicker()
	}
	idx := m.config.pickerIndex
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.config.pickerItems) {
		idx = len(m.config.pickerItems) - 1
	}
	value := m.config.pickerItems[idx]
	field := m.config.pickerField
	current := m.configFieldValue(field)
	m = m.closeConfigModelPicker()
	if strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(value)) {
		return m
	}
	return m.setConfigAgentModel(field, value)
}

func (m model) setConfigAgentModel(field configField, value string) model {
	value = strings.TrimSpace(value)
	if value == "" || field.agent == "" {
		return m
	}
	c := m.config.draft
	isSubagent := strings.HasSuffix(field.agent, ":subagent")
	agentName := strings.TrimSuffix(field.agent, ":subagent")
	agent := c.FallbackModel
	if agentName != "fallback_model" {
		agent = c.Agents[agentName]
	}
	if isSubagent {
		agent.Subagent.Model = value
	} else {
		agent.Model = value
	}
	agent = m.normalizeConfigAgentProperties(agent)
	if agentName == "fallback_model" {
		c.FallbackModel = agent
	} else {
		c.Agents[agentName] = agent
	}
	m.config.draft = c
	m.config.dirty = true
	return m
}

func (m model) configAgentForField(field configField) workflowconfig.AgentModelConfig {
	agentName := strings.TrimSuffix(field.agent, ":subagent")
	if agentName == "fallback_model" {
		return m.config.draft.FallbackModel
	}
	return m.config.draft.Agents[agentName]
}

func configChoiceIndex(choices []string, current string) int {
	current = strings.TrimSpace(current)
	for i, choice := range choices {
		if strings.EqualFold(choice, current) {
			return i
		}
	}
	return 0
}

func (m model) configModelPickerVisibleRows() int {
	chrome := 4 + configPickerBoxStyle.GetVerticalFrameSize()
	max := m.frameContentHeight() - chrome
	if max > configModelPickerMax {
		max = configModelPickerMax
	}
	if max < 3 {
		max = 3
	}
	if n := len(m.config.pickerItems); n > 0 && max > n {
		return n
	}
	return max
}

func (m model) ensureConfigModelPickerVisible() model {
	n := len(m.config.pickerItems)
	if n == 0 {
		m.config.pickerIndex = 0
		m.config.pickerOffset = 0
		return m
	}
	if m.config.pickerIndex < 0 {
		m.config.pickerIndex = 0
	}
	if m.config.pickerIndex >= n {
		m.config.pickerIndex = n - 1
	}
	visible := m.configModelPickerVisibleRows()
	if m.config.pickerIndex < m.config.pickerOffset {
		m.config.pickerOffset = m.config.pickerIndex
	}
	if m.config.pickerIndex >= m.config.pickerOffset+visible {
		m.config.pickerOffset = m.config.pickerIndex - visible + 1
	}
	maxOff := n - visible
	if maxOff < 0 {
		maxOff = 0
	}
	if m.config.pickerOffset < 0 {
		m.config.pickerOffset = 0
	}
	if m.config.pickerOffset > maxOff {
		m.config.pickerOffset = maxOff
	}
	return m
}

func (m model) renderConfigModelPicker() string {
	items := m.config.pickerItems
	if len(items) == 0 {
		return mutedStyle.Render("No matching models.")
	}
	m = m.ensureConfigModelPickerVisible()
	idx := m.config.pickerIndex
	visible := m.configModelPickerVisibleRows()
	start := m.config.pickerOffset
	end := start + visible
	if end > len(items) {
		end = len(items)
	}

	inner := m.configPickerInnerWidth()
	current := m.configFieldValue(m.config.pickerField)
	harnessName := harnessDisplayName(m.configAgentForField(m.config.pickerField).Harness)
	title := strings.TrimSpace(m.config.pickerField.label)
	if title == "" {
		title = "Select model"
	}
	if harnessName != "" {
		title += " · " + harnessName
	}

	var list strings.Builder
	if start > 0 {
		list.WriteString(mutedStyle.Render(truncateDisplayWidth(fmt.Sprintf("  ▲ %d more above", start), inner)))
		list.WriteByte('\n')
	}
	for i := start; i < end; i++ {
		line := items[i]
		if strings.EqualFold(line, current) {
			line += "  current"
		}
		marker := "  "
		if i == idx {
			marker = "▸ "
		}
		row := truncateDisplayWidth(marker+line, inner)
		if i == idx {
			list.WriteString(selectedStyle.Render(row))
		} else {
			list.WriteString(mutedStyle.Render(row))
		}
		if i < end-1 || end < len(items) {
			list.WriteByte('\n')
		}
	}
	if end < len(items) {
		list.WriteString(mutedStyle.Render(truncateDisplayWidth(fmt.Sprintf("  ▼ %d more below", len(items)-end), inner)))
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(truncateDisplayWidth(title, m.configPickerWidth())))
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render(fmt.Sprintf("%d–%d of %d", start+1, end, len(items))))
	b.WriteByte('\n')
	b.WriteString(configPickerBoxStyle.Width(inner).Render(strings.TrimRight(list.String(), "\n")))
	return b.String()
}

func (m model) configPickerWidth() int {
	width := m.contentWidth()
	frame := configPickerBoxStyle.GetHorizontalFrameSize()
	if width-frame < 24 {
		return 24 + frame
	}
	return width
}

func (m model) configPickerInnerWidth() int {
	inner := m.configPickerWidth() - configPickerBoxStyle.GetHorizontalFrameSize()
	if inner < 8 {
		return 8
	}
	return inner
}
