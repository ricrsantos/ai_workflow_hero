package claude

import (
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func (a *resultAssembler) toolProgress(p map[string]any) []harness.StreamDelta {
	name := firstNonEmpty(stringAt(p, "tool_name"), "tool")
	return []harness.StreamDelta{{
		Kind:        harness.StreamKindTool,
		Text:        "Claude tool " + name + " running",
		CallID:      stringAt(p, "tool_use_id"),
		HarnessType: "tool_progress",
		SessionID:   a.sessionID,
	}}
}

func (a *resultAssembler) authStatus(p map[string]any) []harness.StreamDelta {
	if err := stringAt(p, "error"); err != "" {
		return []harness.StreamDelta{{
			Kind:        harness.StreamKindWarning,
			Text:        "Claude authentication failed",
			HarnessType: "auth_status",
			SessionID:   a.sessionID,
		}}
	}
	return a.debugOnlyActivity("auth_status", "Claude authentication status")
}

func (a *resultAssembler) rateLimit(p map[string]any) []harness.StreamDelta {
	info, _ := p["rate_limit_info"].(map[string]any)
	status := strings.ToLower(stringAt(info, "status"))
	switch status {
	case "rejected":
		return []harness.StreamDelta{{Kind: harness.StreamKindWarning, Text: "Claude rate limit rejected the turn", HarnessType: "rate_limit_event", SessionID: a.sessionID}}
	case "allowed_warning":
		return []harness.StreamDelta{{Kind: harness.StreamKindWarning, Text: "Claude rate limit warning", HarnessType: "rate_limit_event", SessionID: a.sessionID}}
	default:
		return a.debugOnlyActivity("rate_limit_event", "Claude rate limit updated")
	}
}

func (a *resultAssembler) controlRequest(p map[string]any) []harness.StreamDelta {
	request, _ := p["request"].(map[string]any)
	subtype := firstNonEmpty(stringAt(request, "subtype"), stringAt(p, "subtype"))
	switch subtype {
	case "can_use_tool":
		tool := firstNonEmpty(stringAt(request, "tool_name"), stringAt(p, "tool_name"), "tool")
		return []harness.StreamDelta{{
			Kind:        harness.StreamKindWarning,
			Text:        "Claude requested permission for " + tool + " via the control protocol",
			HarnessType: "control_request.can_use_tool",
			SessionID:   a.sessionID,
		}}
	case "elicitation":
		return []harness.StreamDelta{{
			Kind:        harness.StreamKindWarning,
			Text:        "Claude requested MCP elicitation via the control protocol",
			HarnessType: "control_request.elicitation",
			SessionID:   a.sessionID,
		}}
	default:
		return a.debugOnlyActivity("control_request", "Claude control request")
	}
}

func (a *resultAssembler) taskStarted(p map[string]any) []harness.StreamDelta {
	name := firstNonEmpty(stringAt(p, "description"), stringAt(p, "task_type"), "task")
	id := firstNonEmpty(stringAt(p, "task_id"), stringAt(p, "tool_use_id"))
	return []harness.StreamDelta{{
		Kind:        harness.StreamKindTool,
		Text:        "Claude task " + name,
		AgentName:   stringAt(p, "task_id"),
		CallID:      id,
		Phase:       harness.StreamPhaseStarted,
		HarnessType: "system.task_started",
		SessionID:   a.sessionID,
	}}
}

func (a *resultAssembler) taskUpdated(p map[string]any) []harness.StreamDelta {
	id := stringAt(p, "task_id")
	patch, _ := p["patch"].(map[string]any)
	status := firstNonEmpty(stringAt(patch, "status"), "updated")
	return []harness.StreamDelta{{
		Kind:        harness.StreamKindTool,
		Text:        "Claude task " + status,
		AgentName:   id,
		CallID:      id,
		HarnessType: "system.task_updated",
		SessionID:   a.sessionID,
	}}
}

func (a *resultAssembler) taskNotification(p map[string]any) []harness.StreamDelta {
	status := firstNonEmpty(stringAt(p, "status"), "completed")
	id := firstNonEmpty(stringAt(p, "task_id"), stringAt(p, "tool_use_id"))
	return []harness.StreamDelta{{
		Kind:        harness.StreamKindTool,
		Text:        "Claude task " + status,
		AgentName:   stringAt(p, "task_id"),
		CallID:      id,
		Phase:       harness.StreamPhaseCompleted,
		HarnessType: "system.task_notification",
		SessionID:   a.sessionID,
	}}
}

func (a *resultAssembler) localCommandOutput(p map[string]any) []harness.StreamDelta {
	content := stringAt(p, "content")
	if content == "" {
		return nil
	}
	return []harness.StreamDelta{{Kind: harness.StreamKindText, Text: content, HarnessType: "system.local_command_output", SessionID: a.sessionID}}
}

func (a *resultAssembler) informational(p map[string]any) []harness.StreamDelta {
	level := strings.ToLower(stringAt(p, "level"))
	content := firstNonEmpty(shortText(stringAt(p, "content"), 200), "Claude status")
	if level == "warning" {
		return []harness.StreamDelta{{Kind: harness.StreamKindWarning, Text: content, HarnessType: "system.informational", SessionID: a.sessionID}}
	}
	return []harness.StreamDelta{harness.ActivityDelta("system.informational", content, a.sessionID)}
}

func (a *resultAssembler) permissionDenied(p map[string]any) []harness.StreamDelta {
	tool := firstNonEmpty(stringAt(p, "tool_name"), "tool")
	return []harness.StreamDelta{{
		Kind:        harness.StreamKindWarning,
		Text:        "Claude denied permission for " + tool,
		HarnessType: "system.permission_denied",
		SessionID:   a.sessionID,
	}}
}

func (a *resultAssembler) workerShuttingDown(p map[string]any) []harness.StreamDelta {
	reason := firstNonEmpty(stringAt(p, "reason"), "host_exit")
	return []harness.StreamDelta{{
		Kind:        harness.StreamKindWarning,
		Text:        "Claude worker shutting down (" + reason + ")",
		HarnessType: "system.worker_shutting_down",
		SessionID:   a.sessionID,
	}}
}

func taskProgressActivity(p map[string]any) string {
	name := firstNonEmpty(stringAt(p, "description"), stringAt(p, "subagent_type"), "task")
	return "Claude task progress: " + name
}

func debugOnlySummary(eventType string) string {
	switch eventType {
	case "prompt_suggestion":
		return "Claude prompt suggestion"
	case "control_response":
		return "Claude control response"
	case "control_cancel_request":
		return "Claude control cancel"
	case "keep_alive":
		return "Claude keep-alive"
	case "notification":
		return "Claude notification"
	case "memory_recall":
		return "Claude memory recall"
	default:
		return "Claude " + eventType
	}
}

func debugOnlySystemSummary(subtype string, p map[string]any) string {
	switch subtype {
	case "status":
		return systemStatusActivity(p)
	case "compact_boundary":
		return "Claude compact boundary"
	case "plugin_install":
		return "Claude plugin install " + firstNonEmpty(stringAt(p, "status"), "updated")
	case "thinking_tokens":
		return "Claude thinking tokens"
	case "session_state_changed":
		return "Claude session state changed"
	case "files_persisted":
		return "Claude files persisted"
	case "commands_changed":
		return "Claude commands changed"
	case "background_tasks_changed":
		return "Claude background tasks changed"
	default:
		return "Claude " + subtype
	}
}

func isResultError(subtype string) bool {
	switch subtype {
	case "error", "error_max_turns", "error_during_execution", "error_max_budget_usd", "error_max_structured_output_retries":
		return true
	default:
		return false
	}
}

func resultErrorMessage(subtype string, p map[string]any) string {
	if message := firstNonEmpty(stringAt(p, "error"), firstErrorFromArray(p)); message != "" {
		return message
	}
	switch subtype {
	case "error_max_turns":
		return "Claude reached the maximum number of turns"
	case "error_during_execution":
		return "Claude reported an execution error"
	case "error_max_budget_usd":
		return "Claude reached the maximum budget"
	case "error_max_structured_output_retries":
		return "Claude failed structured output after retries"
	default:
		return "Claude reported an execution error"
	}
}

func firstErrorFromArray(p map[string]any) string {
	raw, ok := p["errors"].([]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			continue
		}
		if text = strings.TrimSpace(text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "; ")
}

func shortText(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
