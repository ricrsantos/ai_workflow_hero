package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	agentLabelHARN = "HARN"
	agentsBoxWidth = 26
)

type liveAgent struct {
	CallID string
	Name   string
	Label  string
	Model  string
}

var agentShortLabels = map[string]string{
	"orchestration_agent": "ORCH",
	"orchestrator":        "ORCH",
	"backend_agent":       "BACK",
	"frontend_agent":      "FRNT",
	"generic_agent":       "GEN",
	"qa_agent":            "QA",
	"judge_agent":         "JUDG",
	"planning_agent":      "PLAN",
	"discover_agent":      "DISC",
	"context_agent":       "CTX",
	"browser_ui_agent":    "BUI",
	"end2end_qa_agent":    "E2E",
}

func normalizeAgentKey(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.TrimPrefix(s, "task ")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func resolveAgentKey(name string) string {
	key := normalizeAgentKey(name)
	if key == "" {
		return ""
	}
	if _, ok := agentShortLabels[key]; ok {
		return key
	}
	for known := range agentShortLabels {
		if strings.Contains(key, known) {
			return known
		}
	}
	return key
}

func isKnownHeroAgent(name string) bool {
	key := resolveAgentKey(name)
	if key == "" {
		return false
	}
	_, ok := agentShortLabels[key]
	return ok
}

func agentShortLabel(name string) string {
	key := resolveAgentKey(name)
	if key == "" {
		return agentLabelHARN
	}
	if label, ok := agentShortLabels[key]; ok {
		return label
	}
	return agentLabelHARN
}

func formatAgentHeader(name, model string) string {
	display := agentShortLabel(name)
	model = strings.TrimSpace(model)
	if model == "not set" {
		model = ""
	}
	if model != "" {
		return fmt.Sprintf("[%s - %s]", display, model)
	}
	return fmt.Sprintf("[%s]", display)
}

func wrapAgentLabels(labels []string, width int) string {
	if width < 4 {
		width = 4
	}
	if len(labels) == 0 {
		return ""
	}
	var lines []string
	var cur string
	for _, lab := range labels {
		lab = clipLabel(lab, 4)
		next := lab
		if cur != "" {
			next = cur + " | " + lab
		}
		if utf8.RuneCountInString(next) <= width {
			cur = next
			continue
		}
		if cur != "" {
			lines = append(lines, cur)
		}
		cur = lab
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return strings.Join(lines, "\n")
}

func clipLabel(s string, max int) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return agentLabelHARN
	}
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max])
	}
	return s
}
