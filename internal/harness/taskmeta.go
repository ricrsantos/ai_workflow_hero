package harness

import "strings"

// HeroAgentNames is the identity set for named Hero agents in Task metadata.
var HeroAgentNames = []string{
	"orchestration_agent",
	"discover_agent",
	"planning_agent",
	"context_agent",
	"backend_agent",
	"frontend_agent",
	"generic_agent",
	"qa_agent",
	"judge_agent",
	"browser_ui_agent",
	"end2end_qa_agent",
}

// HeroAgentFromLabel returns a known Hero agent id embedded in s, or "".
func HeroAgentFromLabel(s string) string {
	key := strings.ToLower(strings.TrimSpace(s))
	key = strings.TrimPrefix(key, "task ")
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, " ", "_")
	if key == "" {
		return ""
	}
	for _, known := range HeroAgentNames {
		if key == known || strings.Contains(key, known) {
			return known
		}
	}
	return ""
}

// IsGenericTaskType reports nested fan-out types that are not named Hero agents.
func IsGenericTaskType(s string) bool {
	key := strings.ToLower(strings.TrimSpace(s))
	key = strings.ReplaceAll(key, "-", "_")
	switch key {
	case "generalpurpose", "general_purpose", "explore", "shell", "bash",
		"best_of_n_runner", "bestofn", "task":
		return true
	default:
		return false
	}
}

// IsTaskToolName reports harness tool names that launch a nested agent Task.
func IsTaskToolName(s string) bool {
	key := strings.ToLower(strings.TrimSpace(s))
	key = strings.ReplaceAll(key, "-", "_")
	switch key {
	case "task", "task_tool", "tasktool", "collab", "collabtoolcall", "collab_tool_call":
		return true
	default:
		return strings.Contains(key, "task") && !strings.Contains(key, "status")
	}
}
