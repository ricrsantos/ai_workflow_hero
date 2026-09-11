package claude

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// maxUnknownEventWarnings keeps a malformed or future CLI stream from flooding
// the TUI/log while still making the first occurrences actionable.
const maxUnknownEventWarnings = 3

// resultAssembler keeps only the observable state needed to repair a final
// result after a partial stream. Raw payloads are never retained or emitted.
type resultAssembler struct {
	sessionID       string
	model           string
	properties      map[string]string
	partial         strings.Builder
	final           string
	usage           harness.Usage
	lastCall        harness.Usage
	sawTool         bool
	sawAssistantUse bool
	completed       bool
	unknowns        int
	debug           bool
	toolPaths       []string
	toolPathSet     map[string]struct{}
}

func newResultAssembler(debug bool) *resultAssembler {
	return &resultAssembler{
		properties:  make(map[string]string),
		debug:       debug,
		toolPathSet: make(map[string]struct{}),
	}
}

func (a *resultAssembler) noteToolPaths(paths []string) {
	if a == nil {
		return
	}
	if a.toolPathSet == nil {
		a.toolPathSet = make(map[string]struct{})
	}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, exists := a.toolPathSet[path]; exists {
			continue
		}
		a.toolPathSet[path] = struct{}{}
		a.toolPaths = append(a.toolPaths, path)
	}
}

func decodeRawEvent(line int, raw []byte) (RawEvent, error) {
	var envelope struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return RawEvent{}, fmt.Errorf("decode Claude NDJSON line %d: %w", line, err)
	}
	if strings.TrimSpace(envelope.Type) == "" {
		return RawEvent{}, fmt.Errorf("decode Claude NDJSON line %d: event type is required", line)
	}
	return RawEvent{Line: line, Type: envelope.Type, Subtype: envelope.Subtype, Raw: raw}, nil
}

func (a *resultAssembler) consume(event RawEvent) ([]harness.StreamDelta, error) {
	var payload map[string]any
	if err := json.Unmarshal(event.Raw, &payload); err != nil {
		return nil, fmt.Errorf("decode Claude NDJSON line %d: %w", event.Line, err)
	}
	sessionID := stringAt(payload, "session_id")
	if sessionID != "" && a.sessionID == "" {
		a.sessionID = sessionID
	}
	switch event.Type {
	case "system":
		return a.system(event, payload)
	case "assistant":
		return a.assistant(event, payload), nil
	case "user":
		return a.user(event, payload), nil
	case "permission":
		if delta, ok := permissionDelta(payload, a.sessionID); ok {
			return []harness.StreamDelta{delta}, nil
		}
		return []harness.StreamDelta{harness.WarningDelta(adapterName, event.Type, a.sessionID, "redacted malformed permission event")}, nil
	case "question":
		if delta, ok := questionDelta(payload, a.sessionID); ok {
			return []harness.StreamDelta{delta}, nil
		}
		return []harness.StreamDelta{harness.WarningDelta(adapterName, event.Type, a.sessionID, "redacted malformed question event")}, nil
	case "result":
		return a.consumeResult(event, payload)
	case "stream_event":
		return a.streamEvent(payload), nil
	case "tool_progress":
		return a.toolProgress(payload), nil
	case "tool_use_summary":
		return []harness.StreamDelta{harness.ActivityDelta("tool_use_summary", "Claude tool summary", a.sessionID)}, nil
	case "auth_status":
		return a.authStatus(payload), nil
	case "rate_limit_event":
		return a.rateLimit(payload), nil
	case "control_request":
		return a.controlRequest(payload), nil
	case "conversation_reset":
		return []harness.StreamDelta{harness.ActivityDelta("conversation_reset", "Claude conversation reset", a.sessionID)}, nil
	case "mirror_error":
		return []harness.StreamDelta{{Kind: harness.StreamKindWarning, Text: "Claude session mirror error", HarnessType: "mirror_error", SessionID: a.sessionID}}, nil
	case "prompt_suggestion", "control_response", "control_cancel_request", "keep_alive", "notification", "memory_recall":
		return a.debugOnlyActivity(event.Type, debugOnlySummary(event.Type)), nil
	default:
		return a.unknownWarning(event.Type, "redacted unknown event"), nil
	}
}

func (a *resultAssembler) system(event RawEvent, p map[string]any) ([]harness.StreamDelta, error) {
	switch event.Subtype {
	case "init", "resume":
		if model := stringAt(p, "model"); model != "" {
			a.model = model
		}
		a.properties = effectiveProperties(p, a.properties)
		if a.sessionID == "" {
			return nil, fmt.Errorf("Claude %s event is missing session_id", event.Subtype)
		}
		return []harness.StreamDelta{harness.SessionDelta(harness.SessionStateRunning, "Claude session ready", "system."+event.Subtype, a.sessionID)}, nil
	case "usage":
		a.noteCallUsage(p, false)
		return []harness.StreamDelta{harness.ActivityDelta("system.usage", "Claude usage updated", a.sessionID)}, nil
	case "subagent_started":
		name := firstNonEmpty(stringAt(p, "agent_type"), stringAt(p, "agent_id"), "subagent")
		return []harness.StreamDelta{{Kind: harness.StreamKindTool, Text: "Claude subagent " + name, AgentName: stringAt(p, "agent_id"), Model: a.model, CallID: stringAt(p, "agent_id"), Phase: harness.StreamPhaseStarted, HarnessType: "system.subagent_started", SessionID: a.sessionID}}, nil
	case "subagent_completed":
		return []harness.StreamDelta{{Kind: harness.StreamKindTool, Text: "Claude subagent completed", AgentName: stringAt(p, "agent_id"), CallID: stringAt(p, "agent_id"), Phase: harness.StreamPhaseCompleted, HarnessType: "system.subagent_completed", SessionID: a.sessionID}}, nil
	case "task_started":
		return a.taskStarted(p), nil
	case "task_updated":
		return a.taskUpdated(p), nil
	case "task_notification":
		return a.taskNotification(p), nil
	case "task_progress":
		return []harness.StreamDelta{harness.ActivityDelta("system.task_progress", taskProgressActivity(p), a.sessionID)}, nil
	case "retry", "api_retry":
		return []harness.StreamDelta{harness.ActivityDelta("system."+event.Subtype, systemActivity("retry", p), a.sessionID)}, nil
	case "hook_started", "hook_completed", "hook_progress", "hook_response":
		return []harness.StreamDelta{harness.ActivityDelta("system."+event.Subtype, systemActivity(event.Subtype, p), a.sessionID)}, nil
	case "plugin_loaded":
		return []harness.StreamDelta{harness.ActivityDelta("system.plugin_loaded", systemActivity(event.Subtype, p), a.sessionID)}, nil
	case "local_command_output":
		return a.localCommandOutput(p), nil
	case "informational":
		return a.informational(p), nil
	case "permission_denied":
		return a.permissionDenied(p), nil
	case "elicitation_complete":
		return []harness.StreamDelta{harness.ActivityDelta("system.elicitation_complete", "Claude elicitation completed", a.sessionID)}, nil
	case "worker_shutting_down":
		return a.workerShuttingDown(p), nil
	case "status", "compact_boundary", "plugin_install", "thinking_tokens", "session_state_changed", "files_persisted", "commands_changed", "background_tasks_changed":
		return a.debugOnlyActivity("system."+event.Subtype, debugOnlySystemSummary(event.Subtype, p)), nil
	case "stderr":
		return []harness.StreamDelta{{Kind: harness.StreamKindWarning, Text: "Claude CLI emitted a diagnostic (details redacted)", HarnessType: "system.stderr", SessionID: a.sessionID}}, nil
	default:
		return a.unknownWarning("system."+event.Subtype, "redacted unknown system event"), nil
	}
}

func (a *resultAssembler) assistant(event RawEvent, p map[string]any) []harness.StreamDelta {
	content := contentFrom(p)
	var out []harness.StreamDelta
	for _, item := range content {
		typ := stringAt(item, "type")
		switch typ {
		case "text":
			text := stringAt(item, "text")
			if text != "" {
				a.partial.WriteString(text)
				out = append(out, harness.StreamDelta{Kind: harness.StreamKindText, Text: text, HarnessType: "assistant.text", SessionID: a.sessionID})
			}
		case "thinking":
			text := firstNonEmpty(stringAt(item, "thinking"), stringAt(item, "text"))
			if text != "" {
				out = append(out, harness.StreamDelta{Kind: harness.StreamKindThinking, Text: text, HarnessType: "assistant.thinking", SessionID: a.sessionID})
			}
		case "tool_use":
			a.sawTool = true
			name := firstNonEmpty(stringAt(item, "name"), "tool")
			out = append(out, harness.StreamDelta{Kind: harness.StreamKindTool, Text: "Claude tool " + name, CallID: stringAt(item, "id"), HarnessType: "assistant.tool_use", SessionID: a.sessionID})
		default:
			if typ == "" {
				out = append(out, a.unknownWarning("assistant.content", "redacted unknown content block")...)
				continue
			}
			out = append(out, harness.StreamDelta{Kind: harness.StreamKindText, Text: "Claude " + typ, HarnessType: "assistant." + typ, SessionID: a.sessionID})
		}
	}
	if message, ok := p["message"].(map[string]any); ok {
		if usage, ok := message["usage"].(map[string]any); ok {
			a.noteCallUsage(usage, true)
		}
	}
	if usage, ok := p["usage"].(map[string]any); ok {
		a.noteCallUsage(usage, true)
	}
	return out
}

func (a *resultAssembler) streamEvent(p map[string]any) []harness.StreamDelta {
	if !a.debug {
		return nil
	}
	event, _ := p["event"].(map[string]any)
	if event == nil {
		return a.debugOnlyActivity("stream_event", "Claude stream event")
	}
	harnessType := "stream_event"
	if eventType := stringAt(event, "type"); eventType != "" {
		harnessType = "stream_event." + eventType
	}
	if delta, ok := event["delta"].(map[string]any); ok {
		if deltaType := stringAt(delta, "type"); deltaType != "" {
			harnessType = "stream_event." + deltaType
		}
	}
	label := strings.TrimPrefix(harnessType, "stream_event.")
	if label == "" {
		label = "event"
	}
	return a.debugOnlyActivity(harnessType, "Claude stream "+label)
}

func (a *resultAssembler) debugOnlyActivity(harnessType, summary string) []harness.StreamDelta {
	if !a.debug {
		return nil
	}
	return []harness.StreamDelta{harness.ActivityDelta(harnessType, summary, a.sessionID)}
}

func (a *resultAssembler) unknownWarning(eventType, detail string) []harness.StreamDelta {
	if !a.debug {
		return nil
	}
	a.unknowns++
	if a.unknowns <= maxUnknownEventWarnings {
		return []harness.StreamDelta{harness.WarningDelta(adapterName, eventType, a.sessionID, detail)}
	}
	if a.unknowns == maxUnknownEventWarnings+1 {
		return []harness.StreamDelta{{
			Kind:        harness.StreamKindWarning,
			Text:        "Claude emitted additional unknown events; further warnings suppressed",
			HarnessType: "claude.unknown.suppressed",
			SessionID:   a.sessionID,
		}}
	}
	return nil
}

func (a *resultAssembler) user(_ RawEvent, p map[string]any) []harness.StreamDelta {
	if delta, ok := permissionDelta(p, a.sessionID); ok {
		return []harness.StreamDelta{delta}
	}
	if delta, ok := questionDelta(p, a.sessionID); ok {
		return []harness.StreamDelta{delta}
	}
	for _, item := range contentFrom(p) {
		if stringAt(item, "type") == "tool_result" {
			return []harness.StreamDelta{{Kind: harness.StreamKindActivity, Text: "Claude tool completed", CallID: stringAt(item, "tool_use_id"), HarnessType: "user.tool_result", SessionID: a.sessionID}}
		}
	}
	return []harness.StreamDelta{harness.ActivityDelta("user.message", "Claude activity", a.sessionID)}
}

func (a *resultAssembler) consumeResult(event RawEvent, p map[string]any) ([]harness.StreamDelta, error) {
	if isResultError(event.Subtype) {
		return nil, fmt.Errorf("%s", resultErrorMessage(event.Subtype, p))
	}
	if event.Subtype != "success" && event.Subtype != "" {
		return nil, fmt.Errorf("Claude terminal result subtype %q is unsupported", event.Subtype)
	}
	if text := stringAt(p, "result"); text != "" {
		a.final = text
	}
	if sid := stringAt(p, "session_id"); sid != "" {
		if a.sessionID != "" && sid != a.sessionID {
			return nil, errorsNew("Claude terminal result session_id differs from stream session")
		}
		a.sessionID = sid
	}
	if a.sessionID == "" {
		return nil, errorsNew("Claude terminal result is missing session_id")
	}
	if usage, ok := p["usage"].(map[string]any); ok {
		a.usage = usageFrom(usage, harness.Usage{})
	}
	a.usage.ContextTokens = claudeOccupancy(a.lastCall, a.sawTool, a.usage)
	a.completed = true
	return []harness.StreamDelta{harness.SessionDelta(harness.SessionStateIdle, "Claude stream completed", "result.success", a.sessionID)}, nil
}

func (a *resultAssembler) result(duration time.Duration) (*harness.ExecutionResult, error) {
	if !a.completed {
		return nil, errorsNew("Claude stream ended without a terminal result")
	}
	output := a.final
	if output == "" {
		output = a.partial.String()
	}
	return &harness.ExecutionResult{
		SessionID:           a.sessionID,
		Output:              output,
		Summary:             output,
		Usage:               a.usage,
		Duration:            duration,
		StreamDone:          true,
		NativeModel:         a.model,
		EffectiveProperties: cloneProperties(a.properties),
	}, nil
}

func permissionDelta(p map[string]any, sessionID string) (harness.StreamDelta, bool) {
	kind := strings.ToLower(firstNonEmpty(stringAt(p, "type"), stringAt(p, "subtype")))
	if kind != "permission" && kind != "permission_request" && kind != "permission.asked" {
		return harness.StreamDelta{}, false
	}
	id := firstNonEmpty(stringAt(p, "request_id"), stringAt(p, "id"))
	tool := firstNonEmpty(stringAt(p, "tool_name"), stringAt(p, "tool"), "Claude tool")
	if id == "" {
		return harness.WarningDelta(adapterName, "permission", sessionID, "redacted malformed permission request"), true
	}
	return harness.StreamDelta{Kind: harness.StreamKindPermission, Text: "Claude requests permission for " + tool, HarnessType: "claude.permission", SessionID: sessionID, Metadata: map[string]string{"request_id": id, "tool_name": tool}}, true
}

func questionDelta(p map[string]any, sessionID string) (harness.StreamDelta, bool) {
	kind := strings.ToLower(firstNonEmpty(stringAt(p, "type"), stringAt(p, "subtype")))
	if kind != "question" && kind != "question_request" && kind != "question.asked" {
		return harness.StreamDelta{}, false
	}
	id := firstNonEmpty(stringAt(p, "request_id"), stringAt(p, "id"))
	question := firstNonEmpty(stringAt(p, "question"), "Claude needs an answer")
	if id == "" {
		return harness.WarningDelta(adapterName, "question", sessionID, "redacted malformed question request"), true
	}
	return harness.StreamDelta{Kind: harness.StreamKindQuestion, Text: question, HarnessType: "claude.question", SessionID: sessionID, Metadata: map[string]string{"request_id": id}}, true
}

func effectiveProperties(p map[string]any, prior map[string]string) map[string]string {
	result := cloneProperties(prior)
	if result == nil {
		result = make(map[string]string)
	}
	for _, key := range []string{"effort", "ef"} {
		if value := stringAt(p, key); value != "" {
			result["ef"] = value
		}
	}
	return result
}

func cloneProperties(properties map[string]string) map[string]string {
	if len(properties) == 0 {
		return nil
	}
	clone := make(map[string]string, len(properties))
	for key, value := range properties {
		clone[key] = value
	}
	return clone
}

func contentFrom(p map[string]any) []map[string]any {
	message, _ := p["message"].(map[string]any)
	if message == nil {
		return nil
	}
	raw, _ := message["content"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (a *resultAssembler) noteCallUsage(p map[string]any, fromAssistant bool) {
	if a == nil || p == nil {
		return
	}
	call := usageFrom(p, harness.Usage{}).WithCallOccupancy()
	if call.ContextTokens <= 0 && !call.HasBilledCounts() {
		return
	}
	if fromAssistant {
		a.sawAssistantUse = true
		a.lastCall = call
		return
	}
	if !a.sawAssistantUse {
		a.lastCall = call
	}
}

func claudeOccupancy(lastCall harness.Usage, sawTool bool, billed harness.Usage) int64 {
	if lastCall.ContextTokens > 0 {
		return lastCall.ContextTokens
	}
	if !sawTool {
		return billed.CallOccupancy()
	}
	return 0
}

func usageFrom(p map[string]any, prior harness.Usage) harness.Usage {
	if n, ok := firstInt64At(p, "input_tokens", "inputTokens"); ok {
		prior.InputTokens = n
	}
	if n, ok := firstInt64At(p, "output_tokens", "outputTokens"); ok {
		prior.OutputTokens = n
	}
	if n, ok := firstInt64At(p, "cache_read_input_tokens", "cacheReadTokens", "cache_read_tokens"); ok {
		prior.CacheReadTokens = n
	}
	if n, ok := firstInt64At(p, "cache_creation_input_tokens", "cacheWriteTokens", "cache_write_tokens", "cacheCreationTokens"); ok {
		prior.CacheWriteTokens = n
	}
	return prior
}

func firstInt64At(p map[string]any, keys ...string) (int64, bool) {
	for _, key := range keys {
		if n, ok := int64At(p, key); ok {
			return n, true
		}
	}
	return 0, false
}

func systemActivity(subtype string, p map[string]any) string {
	switch subtype {
	case "retry":
		return "Claude retry " + firstNonEmpty(stringAt(p, "attempt"), "started")
	case "hook_started", "hook_completed", "hook_progress", "hook_response":
		return "Claude hook " + firstNonEmpty(stringAt(p, "hook_name"), stringAt(p, "hook"), subtype)
	case "plugin_loaded":
		return "Claude plugin loaded"
	default:
		return "Claude activity"
	}
}

func systemStatusActivity(p map[string]any) string {
	if status := firstNonEmpty(stringAt(p, "status"), stringAt(p, "message")); status != "" {
		return "Claude status: " + status
	}
	return "Claude status update"
}

func stringAt(p map[string]any, key string) string {
	v, ok := p[key]
	if !ok {
		return ""
	}
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		return fmt.Sprintf("%g", value)
	default:
		return ""
	}
}

func int64At(p map[string]any, key string) (int64, bool) {
	v, ok := p[key].(float64)
	return int64(v), ok
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func errorsNew(message string) error { return fmt.Errorf("%s", message) }
