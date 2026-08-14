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

var agentTranscriptNames = map[string]string{
	"orchestration_agent": "Orchestrator",
	"orchestrator":        "Orchestrator",
	"backend_agent":       "Backend",
	"frontend_agent":      "Frontend",
	"generic_agent":       "Generic",
	"qa_agent":            "QA",
	"judge_agent":         "Judge",
	"planning_agent":      "Planning",
	"discover_agent":      "Discover",
	"context_agent":       "Context",
	"browser_ui_agent":    "Browser UI",
	"end2end_qa_agent":    "E2E QA",
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

func agentTranscriptName(name string) string {
	key := resolveAgentKey(name)
	if key == "" {
		return "Agent"
	}
	if display, ok := agentTranscriptNames[key]; ok {
		return display
	}
	return "Agent"
}

func formatAgentHeader(name, model string, isSubagent bool) string {
	display := agentTranscriptName(name)
	model = strings.TrimSpace(model)
	if model == "not set" {
		model = ""
	}
	if !isSubagent && agentShortLabel(name) == "ORCH" {
		return "[Orchestrator]"
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
