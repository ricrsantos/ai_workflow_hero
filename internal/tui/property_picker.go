package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// handlePropertyPickerKey processes keys while the C5 property screen is open
// (UI-C05-001 §3): Space changes the focused row; Enter saves the complete draft;
// Escape cancels everything.
func (m model) handlePropertyPickerKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.cancelPropertyDraft()
	case "up":
		idx := m.propsRowIndex()
		if idx > 0 {
			m.paletteIndex = idx - 1
		}
		return m, nil
	case "down":
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
			if next, ok := toggleBooleanValue(cap.AcceptedValues, m.propsDraft[key]); ok {
				m.propsDraft[key] = next
				m.propsEdited[key] = true
			}
			return m, nil
		}
		if next, ok := cyclePropertyValue(cap.AcceptedValues, m.propsDraft[key]); ok {
			m.propsDraft[key] = next
			m.propsEdited[key] = true
		}
		return m, nil
	case "enter":
		return m.commitPropertyDraft()
	}
	return m, nil
}

func toggleBooleanValue(accepted []string, current string) (string, bool) {
	values := make([]string, 0, len(accepted))
	for _, value := range accepted {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	if len(values) != 2 {
		return "", false
	}
	for _, value := range values {
		if value == current {
			for _, other := range values {
				if other != current {
					return other, true
				}
			}
		}
	}
	return values[0], true
}

func cyclePropertyValue(accepted []string, current string) (string, bool) {
	values := make([]string, 0, len(accepted))
	for _, value := range accepted {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return "", false
	}
	if len(values) == 1 {
		return values[0], true
	}
	current = strings.TrimSpace(current)
	for i, value := range values {
		if value == current {
			return values[(i+1)%len(values)], true
		}
	}
	return values[0], true
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
	b.WriteString(mutedStyle.Render("↑↓ navigate · space change · enter save · esc cancel"))
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
		case !ok || !cap.Available:
			if value != "" && value != "na" {
				line = fmt.Sprintf("%s: %s", friendlyPropertyName(key), value)
			} else {
				line = fmt.Sprintf("%s: %s", friendlyPropertyName(key), "na")
			}
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
		if i == m.propsRowIndex() {
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
	return b.String()
}
