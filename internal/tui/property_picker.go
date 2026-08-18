package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// handlePropertyPickerKey processes keys while the C5 property screen is open
// (UI-C05-001 §3): Space toggles booleans, Enter opens/confirms multi-value
// lists, main Enter saves the complete draft, Escape cancels everything.
func (m model) handlePropertyPickerKey(msg tea.KeyMsg) (model, tea.Cmd) {
	if m.propsValueList {
		return m.handlePropertyValueListKey(msg)
	}
	switch msg.String() {
	case "esc":
		return m.cancelPropertyDraft()
	case "up", "ctrl+p":
		idx := m.propsRowIndex()
		if idx > 0 {
			m.paletteIndex = idx - 1
		}
		return m, nil
	case "down", "ctrl+n":
		idx := m.propsRowIndex()
		if idx < len(harness.PropertyKeys())-1 {
			m.paletteIndex = idx + 1
		}
		return m, nil
	case " ", "space":
		key := harness.PropertyKeys()[m.propsRowIndex()]
		cap, ok := m.propsSnapshot.Properties[key]
		if !ok || !cap.Available {
			return m, nil // disabled rows stay inert
		}
		if key == harness.PropertyFast {
			// Boolean toggle for fast mode (UI-C05-001 §3: space toggle).
			if m.propsDraft[key] == "true" {
				m.propsDraft[key] = "false"
			} else {
				m.propsDraft[key] = "true"
			}
			m.propsEdited[key] = true
		}
		return m, nil
	case "enter":
		key := harness.PropertyKeys()[m.propsRowIndex()]
		cap, ok := m.propsSnapshot.Properties[key]
		if !ok || !cap.Available {
			return m, nil // disabled rows stay inert
		}
		if key == harness.PropertyFast || len(cap.AcceptedValues) <= 1 {
			// Booleans and single-value properties have nothing to list, so the
			// main Enter saves the complete draft (ENTER to save; ADR-042).
			return m.commitPropertyDraft()
		}
		if m.propsEdited[key] {
			// A multi-value row whose value was already chosen this session:
			// Enter commits the complete draft instead of re-opening its list.
			return m.commitPropertyDraft()
		}
		// Open the secondary multi-value list.
		m.propsValueList = true
		m.propsValueKey = key
		m.propsValueIndex = 0
		for i, v := range cap.AcceptedValues {
			if v == m.propsDraft[key] {
				m.propsValueIndex = i
				break
			}
		}
		return m, nil
	}
	return m, nil
}

// handlePropertyValueListKey processes the secondary multi-value list.
func (m model) handlePropertyValueListKey(msg tea.KeyMsg) (model, tea.Cmd) {
	cap, ok := m.propsSnapshot.Properties[m.propsValueKey]
	if !ok {
		m.propsValueList = false
		return m, nil
	}
	switch msg.String() {
	case "esc":
		// Escape from the secondary list cancels the complete selection.
		return m.cancelPropertyDraft()
	case "up", "ctrl+p":
		if m.propsValueIndex > 0 {
			m.propsValueIndex--
		}
		return m, nil
	case "down", "ctrl+n":
		if m.propsValueIndex < len(cap.AcceptedValues)-1 {
			m.propsValueIndex++
		}
		return m, nil
	case "enter":
		if m.propsValueIndex >= 0 && m.propsValueIndex < len(cap.AcceptedValues) {
			m.propsDraft[m.propsValueKey] = cap.AcceptedValues[m.propsValueIndex]
			m.propsEdited[m.propsValueKey] = true
		}
		m.propsValueList = false
		m.propsValueKey = ""
		m.propsValueIndex = 0
		return m, nil
	}
	return m, nil
}

// propsRowIndex clamps the palette cursor to the three fixed property rows.
func (m model) propsRowIndex() int {
	idx := m.paletteIndex
	n := len(harness.PropertyKeys())
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return idx
}

// renderPropertyPicker renders the main property screen (UI-C05-001 §3).
func (m model) renderPropertyPicker() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(propertyPickerHeader(m.propsDraftHarness)))
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render("↑↓ navigate · space toggle · enter select · ENTER to save · esc cancel"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	for i, key := range harness.PropertyKeys() {
		cap, ok := m.propsSnapshot.Properties[key]
		value := m.propsDraft[key]
		if value == "" {
			value = "na"
		}
		var line string
		switch {
		case ok && cap.Available && key == harness.PropertyFast:
			line = fmt.Sprintf("%s: %s", friendlyPropertyName(key), value)
		case ok && cap.Available:
			line = fmt.Sprintf("%s: %s", friendlyPropertyName(key), value)
		default:
			line = fmt.Sprintf("%s: %s", friendlyPropertyName(key), "na")
		}
		hasStyle := false
		var style lipgloss.Style
		if !ok || !cap.Available {
			style = mutedStyle // disabled rows stay visible in gray
			hasStyle = true
		}
		prefix := "  "
		if i == m.propsRowIndex() && !m.propsValueList {
			prefix = "▸ "
			if !hasStyle {
				style = selectedStyle
			}
		}
		if prefix == "▸ " || hasStyle {
			b.WriteString(style.Render(prefix + line))
		} else {
			b.WriteString(prefix + line)
		}
		b.WriteByte('\n')
	}

	// Secondary multi-value list (th/ef and future multi-value keys).
	if m.propsValueList {
		b.WriteByte('\n')
		cap := m.propsSnapshot.Properties[m.propsValueKey]
		b.WriteString(infoStyle.Render(fmt.Sprintf("  %s values:", friendlyPropertyName(m.propsValueKey))))
		b.WriteByte('\n')
		for i, v := range cap.AcceptedValues {
			prefix := "    "
			if i == m.propsValueIndex {
				prefix = "  ▸ "
			}
			b.WriteString(prefix + v)
			b.WriteByte('\n')
		}
		b.WriteString(mutedStyle.Render("    ↑↓ choose · enter confirm · esc cancel"))
		b.WriteByte('\n')
	}
	return b.String()
}
