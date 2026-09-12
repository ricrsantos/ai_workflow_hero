package conversation

import (
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"
)

const (
	titleImageConversation    = "Image conversation"
	titleMiddleDot            = " · "
	freeChatTitleMaxGraphemes = 48
)

// TitleFreeChat derives a deterministic Free Chat title from the first turn text.
// Attachment-only turns should pass attachmentOnly=true (or empty text after normalization).
func TitleFreeChat(firstText string, attachmentOnly bool) string {
	if attachmentOnly {
		return titleImageConversation
	}
	normalized := normalizeFreeChatTitleText(firstText)
	if normalized == "" {
		return titleImageConversation
	}
	return truncateGraphemes(normalized, freeChatTitleMaxGraphemes)
}

// TitleOrchestration returns the cycle-aware orchestration session title (D4).
func TitleOrchestration(cycleNumber int) string {
	return formatCycleTitle(cycleNumber, "Orchestration", "ORCH")
}

// TitleResearch returns the cycle-aware research session title (D4).
func TitleResearch(cycleNumber int) string {
	return formatCycleTitle(cycleNumber, "Research", "DISC")
}

// TitleStageAgent returns the cycle-aware named stage-agent title (D4).
func TitleStageAgent(cycleNumber int, stageName, agentName string) string {
	stage := StageDisplayName(stageName)
	label := AgentShortLabel(agentName)
	return formatCycleTitle(cycleNumber, stage, label)
}

func formatCycleTitle(cycleNumber int, stageLabel, agentLabel string) string {
	n := cycleNumber
	if n < 0 {
		n = 0
	}
	return "C" + itoaNonNeg(n) + titleMiddleDot + stageLabel + titleMiddleDot + agentLabel
}

func itoaNonNeg(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func normalizeFreeChatTitleText(text string) string {
	s := norm.NFC.String(strings.TrimSpace(text))
	return collapseUnicodeSpace(s)
}

func collapseUnicodeSpace(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(b.String())
}

func truncateGraphemes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if uniseg.GraphemeClusterCount(s) <= max {
		return s
	}
	var n int
	var out strings.Builder
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		if n >= max {
			break
		}
		out.WriteString(gr.Str())
		n++
	}
	return out.String()
}

// StageDisplayName maps internal stage_name to English UI labels (D4).
func StageDisplayName(stageName string) string {
	key := strings.ToLower(strings.TrimSpace(stageName))
	if key == "" {
		return "Stage"
	}
	if label, ok := stageDisplayNames[key]; ok {
		return label
	}
	parts := strings.Split(key, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = capitalizeFirstGrapheme(p)
	}
	return strings.Join(parts, " ")
}

// capitalizeFirstGrapheme uppercases the first grapheme cluster; the remainder is unchanged.
func capitalizeFirstGrapheme(s string) string {
	if s == "" {
		return ""
	}
	gr := uniseg.NewGraphemes(s)
	if !gr.Next() {
		return s
	}
	first := gr.Str()
	return strings.ToUpper(first) + s[len(first):]
}

var stageDisplayNames = map[string]string{
	"research":              "Research",
	"planning":              "Planning",
	"implementation":        "Implementation",
	"qa":                    "QA",
	"judge":                 "Judge",
	"browser_ui_validation": "Browser UI Validation",
	"qa_end_to_end":         "QA End-to-End",
}

// AgentShortLabel maps agent_name to the four-letter History label (D4).
func AgentShortLabel(agentName string) string {
	key := normalizeAgentKey(agentName)
	if key == "" {
		return "HARN"
	}
	if label, ok := agentShortLabels[key]; ok {
		return label
	}
	return "TASK"
}

func normalizeAgentKey(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.TrimPrefix(s, "task ")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
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
