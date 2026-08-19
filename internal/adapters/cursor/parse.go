package cursor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// cliResultJSON is the Cursor Agent CLI terminal JSON / stream-json result event.
type cliResultJSON struct {
	Type       string        `json:"type"`
	Subtype    string        `json:"subtype"`
	IsError    bool          `json:"is_error"`
	DurationMS int64         `json:"duration_ms"`
	Result     string        `json:"result"`
	SessionID  string        `json:"session_id"`
	RequestID  string        `json:"request_id"`
	Usage      *cliUsageJSON `json:"usage"`
}

type cliUsageJSON struct {
	InputTokens       int64 `json:"inputTokens"`
	OutputTokens      int64 `json:"outputTokens"`
	InputTokensSnake  int64 `json:"input_tokens"`
	OutputTokensSnake int64 `json:"output_tokens"`
}

type cliStreamEvent struct {
	Type             string          `json:"type"`
	Subtype          string          `json:"subtype"`
	SessionID        string          `json:"session_id"`
	Result           string          `json:"result"`
	IsError          bool            `json:"is_error"`
	DurationMS       int64           `json:"duration_ms"`
	Usage            *cliUsageJSON   `json:"usage"`
	Message          *cliMessage     `json:"message"`
	Text             string          `json:"text"` // thinking delta
	ToolCall         json.RawMessage `json:"tool_call"`
	CallID           string          `json:"call_id"`
	AgentID          string          `json:"agent_id"`
	ParentToolCallID string          `json:"parent_tool_call_id"`
	// Partial-stream filters (docs): skip when model_call_id set or timestamp absent on final flush.
	TimestampMS *int64  `json:"timestamp_ms"`
	ModelCallID *string `json:"model_call_id"`
}

type cliMessage struct {
	Role    string           `json:"role"`
	Content []cliContentPart `json:"content"`
}

type cliContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type taskInfo struct {
	Name  string
	Model string
}

type streamParseState struct {
	openTasks   map[string]taskInfo
	taskOrder   []string
	emittedText map[string]bool
}

func newStreamParseState() *streamParseState {
	return &streamParseState{
		openTasks:   make(map[string]taskInfo),
		emittedText: make(map[string]bool),
	}
}

func (s *streamParseState) openTask(id string, info taskInfo) {
	if id == "" {
		return
	}
	if _, exists := s.openTasks[id]; !exists {
		s.taskOrder = append(s.taskOrder, id)
	}
	s.openTasks[id] = info
}

func (s *streamParseState) closeTask(id string) taskInfo {
	info := s.openTasks[id]
	delete(s.openTasks, id)
	out := s.taskOrder[:0]
	for _, existing := range s.taskOrder {
		if existing != id {
			out = append(out, existing)
		}
	}
	s.taskOrder = out
	return info
}

func (s *streamParseState) soleOpenTask() (id string, info taskInfo, ok bool) {
	if len(s.taskOrder) != 1 {
		return "", taskInfo{}, false
	}
	id = s.taskOrder[0]
	return id, s.openTasks[id], true
}

func (s *streamParseState) attrFromEvent(ev cliStreamEvent) (name, model, callID string) {
	if parent := strings.TrimSpace(ev.ParentToolCallID); parent != "" {
		if info, ok := s.openTasks[parent]; ok {
			return info.Name, info.Model, parent
		}
	}
	if id, info, ok := s.soleOpenTask(); ok {
		return info.Name, info.Model, id
	}
	return "", "", ""
}

func (u *cliUsageJSON) toHarness() harness.Usage {
	if u == nil {
		return harness.Usage{}
	}
	in := u.InputTokens
	if in == 0 {
		in = u.InputTokensSnake
	}
	out := u.OutputTokens
	if out == 0 {
		out = u.OutputTokensSnake
	}
	return harness.Usage{InputTokens: in, OutputTokens: out}
}

// ParseJSONResult maps a single JSON object from --output-format json.
func ParseJSONResult(data []byte) (*harness.ExecutionResult, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("empty cursor agent JSON output")
	}
	var raw cliResultJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse cursor agent JSON: %w", err)
	}
	if raw.IsError || raw.Subtype == "error" {
		return nil, fmt.Errorf("cursor agent returned error result: %s", strings.TrimSpace(raw.Result))
	}
	out := strings.TrimSpace(raw.Result)
	return &harness.ExecutionResult{
		SessionID:  raw.SessionID,
		Output:     out,
		Summary:    summarize(out),
		Usage:      raw.Usage.toHarness(),
		Duration:   time.Duration(raw.DurationMS) * time.Millisecond,
		StreamDone: true,
	}, nil
}

// ParseStreamJSON reads NDJSON stream-json events and builds an ExecutionResult.
// When onDelta is non-nil, thinking, tool activity, and assistant text are forwarded
// (partial assistant deltas preferred). Thinking/tools are display-only and not
// included in ExecutionResult.Output. Nested Task assistant text is attributed to
// the open Task when the CLI forwards it (or when exactly one Task is in flight).
func ParseStreamJSON(r io.Reader, onDelta func(harness.StreamDelta)) (*harness.ExecutionResult, error) {
	sc := bufio.NewScanner(r)
	// Cursor stream lines can be large (tool payloads); raise limit.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 10*1024*1024)

	var (
		sessionID  string
		result     *harness.ExecutionResult
		assistant  strings.Builder
		sawPartial bool
		state      = newStreamParseState()
	)

	emit := func(d harness.StreamDelta) {
		if onDelta == nil {
			return
		}
		if d.Text == "" && d.Phase == "" {
			return
		}
		if d.Kind == harness.StreamKindText && d.CallID != "" && d.Text != "" {
			state.emittedText[d.CallID] = true
		}
		onDelta(d)
	}

	emitAttr := func(kind harness.StreamKind, text, name, model, callID, phase string) {
		if text == "" && phase == "" {
			return
		}
		emit(harness.StreamDelta{
			Kind:      kind,
			Text:      text,
			AgentName: name,
			Model:     model,
			CallID:    callID,
			Phase:     phase,
		})
	}

	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev cliStreamEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			// Skip non-JSON noise lines.
			continue
		}
		if ev.SessionID != "" {
			sessionID = ev.SessionID
		}
		switch ev.Type {
		case "system", "user":
			// Stream lifecycle events; no user-visible delta.
		case "thinking":
			if ev.Subtype == "completed" {
				continue
			}
			text := ev.Text
			if text == "" {
				text = extractContentText(ev, "thinking", "reasoning")
			}
			name, model, callID := state.attrFromEvent(ev)
			emitAttr(harness.StreamKindThinking, text, name, model, callID, "")
		case "tool_call":
			label := formatToolCall(ev.ToolCall)
			info, resultContent, isTask := extractTaskMeta(ev.ToolCall)
			if !isTask {
				isTask = isTaskToolLabel(label)
			}
			if info.Name == "" && isTask {
				info.Name = taskNameFromLabel(label)
			}
			callID := strings.TrimSpace(ev.CallID)
			switch ev.Subtype {
			case "", "started":
				if isTask {
					if callID == "" {
						callID = "task:" + info.Name
					}
					if ev.AgentID != "" && info.Name == "" {
						info.Name = ev.AgentID
					}
					state.openTask(callID, info)
					emitAttr(harness.StreamKindTool, label, info.Name, info.Model, callID, harness.StreamPhaseStarted)
					break
				}
				if label == "" {
					break
				}
				name, model, parentID := state.attrFromEvent(ev)
				emitAttr(harness.StreamKindTool, label, name, model, parentID, "")
			case "completed":
				if isTask {
					if callID == "" {
						callID = "task:" + info.Name
					}
					closed := state.closeTask(callID)
					if info.Name == "" {
						info.Name = closed.Name
					}
					if info.Model == "" {
						info.Model = closed.Model
					}
					if resultContent != "" && !state.emittedText[callID] {
						emitAttr(harness.StreamKindText, resultContent, info.Name, info.Model, callID, "")
					}
					if label != "" {
						emitAttr(harness.StreamKindTool, label+" (completed)", info.Name, info.Model, callID, harness.StreamPhaseCompleted)
					} else {
						emitAttr(harness.StreamKindTool, "", info.Name, info.Model, callID, harness.StreamPhaseCompleted)
					}
					break
				}
			}
		case "assistant":
			if ev.TimestampMS != nil {
				sawPartial = true
			}
			name, model, callID := state.attrFromEvent(ev)
			if thinking := extractContentText(ev, "thinking", "reasoning"); thinking != "" && shouldEmitDelta(ev, sawPartial) {
				emitAttr(harness.StreamKindThinking, thinking, name, model, callID, "")
			}
			text := extractAssistantText(ev)
			if text == "" {
				continue
			}
			if !shouldEmitDelta(ev, sawPartial) {
				continue
			}
			emitAttr(harness.StreamKindText, text, name, model, callID, "")
			if callID == "" {
				assistant.WriteString(text)
			}
		case "result":
			if ev.IsError || ev.Subtype == "error" {
				return nil, fmt.Errorf("cursor agent stream error: %s", strings.TrimSpace(ev.Result))
			}
			out := strings.TrimSpace(ev.Result)
			if out == "" {
				out = strings.TrimSpace(assistant.String())
			}
			result = &harness.ExecutionResult{
				SessionID:  sessionID,
				Output:     out,
				Summary:    summarize(out),
				Usage:      ev.Usage.toHarness(),
				Duration:   time.Duration(ev.DurationMS) * time.Millisecond,
				StreamDone: true,
			}
			if result.SessionID == "" {
				result.SessionID = ev.SessionID
			}
		default:
			if ev.Type != "" {
				emit(harness.WarningDelta("cursor", ev.Type, sessionID, string(line)))
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read stream-json: %w", err)
	}
	if result == nil {
		out := strings.TrimSpace(assistant.String())
		if out == "" && sessionID == "" {
			return nil, fmt.Errorf("stream-json ended without result event")
		}
		result = &harness.ExecutionResult{
			SessionID:  sessionID,
			Output:     out,
			Summary:    summarize(out),
			StreamDone: true,
		}
	}
	return result, nil
}

func extractAssistantText(ev cliStreamEvent) string {
	return extractContentText(ev, "text", "")
}

func extractContentText(ev cliStreamEvent, types ...string) string {
	if ev.Message == nil {
		return ""
	}
	want := make(map[string]struct{}, len(types))
	for _, t := range types {
		want[t] = struct{}{}
	}
	var b strings.Builder
	for _, p := range ev.Message.Content {
		if _, ok := want[p.Type]; ok {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func formatToolCall(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	if fn, ok := obj["function"]; ok {
		var f struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}
		if json.Unmarshal(fn, &f) == nil && strings.TrimSpace(f.Name) != "" {
			if summary := toolArgSummaryFromJSON(f.Arguments); summary != "" {
				return f.Name + " " + summary
			}
			return f.Name
		}
	}
	preferred := []string{
		"taskToolCall", "readToolCall", "writeToolCall", "editToolCall", "grepToolCall",
		"globToolCall", "shellToolCall", "deleteToolCall", "searchToolCall",
	}
	for _, key := range preferred {
		if val, ok := obj[key]; ok {
			return formatNamedTool(key, val)
		}
	}
	for key, val := range obj {
		if strings.HasSuffix(key, "ToolCall") {
			return formatNamedTool(key, val)
		}
	}
	return ""
}

func extractTaskMeta(raw json.RawMessage) (info taskInfo, resultContent string, ok bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return taskInfo{}, "", false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return taskInfo{}, "", false
	}
	if val, found := obj["taskToolCall"]; found {
		return taskWrapMeta(val)
	}
	if fn, found := obj["function"]; found {
		var f struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}
		if json.Unmarshal(fn, &f) == nil && isTaskToolLabel(f.Name) {
			return taskInfoFromArgs(parseArgsMap(f.Arguments)), "", true
		}
	}
	return taskInfo{}, "", false
}

func taskWrapMeta(val json.RawMessage) (taskInfo, string, bool) {
	var wrap struct {
		Args   map[string]any  `json:"args"`
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(val, &wrap) != nil {
		return taskInfo{}, "", true
	}
	return taskInfoFromArgs(wrap.Args), extractTaskResultContent(wrap.Result), true
}

func taskInfoFromArgs(args map[string]any) taskInfo {
	if args == nil {
		return taskInfo{}
	}
	return taskInfo{
		Name:  heroAgentFromTaskArgs(args),
		Model: firstArgString(args, "model"),
	}
}

// heroTaskAgentNames is the Cursor Task identity for named Hero agents.
// Prefer these over generic subagent_type values such as generalPurpose.
var heroTaskAgentNames = []string{
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

func heroAgentFromTaskArgs(args map[string]any) string {
	candidates := []string{
		firstArgString(args, "subagent_type"),
		firstArgString(args, "name"),
		firstArgString(args, "description"),
		firstArgString(args, "prompt"),
	}
	for _, c := range candidates {
		if name := extractHeroAgentName(c); name != "" {
			return name
		}
	}
	for _, c := range candidates {
		if c != "" && !isGenericTaskType(c) {
			return c
		}
	}
	return firstArgString(args, "subagent_type", "name", "description")
}

func extractHeroAgentName(s string) string {
	key := strings.ToLower(strings.TrimSpace(s))
	key = strings.TrimPrefix(key, "task ")
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, " ", "_")
	if key == "" {
		return ""
	}
	for _, known := range heroTaskAgentNames {
		if key == known || strings.Contains(key, known) {
			return known
		}
	}
	return ""
}

func isGenericTaskType(s string) bool {
	key := strings.ToLower(strings.TrimSpace(s))
	key = strings.ReplaceAll(key, "-", "_")
	switch key {
	case "generalpurpose", "general_purpose", "explore", "shell", "bash",
		"best_of_n_runner", "bestofn":
		return true
	default:
		return false
	}
}

func firstArgString(args map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := args[k]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s != "" {
			return s
		}
	}
	return ""
}

func parseArgsMap(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var args map[string]any
	if json.Unmarshal([]byte(raw), &args) != nil {
		return nil
	}
	return args
}

func extractTaskResultContent(result json.RawMessage) string {
	if len(bytes.TrimSpace(result)) == 0 {
		return ""
	}
	var asStr string
	if json.Unmarshal(result, &asStr) == nil {
		return strings.TrimSpace(asStr)
	}
	var wrap struct {
		Success struct {
			Content string `json:"content"`
		} `json:"success"`
		Content string `json:"content"`
	}
	if json.Unmarshal(result, &wrap) == nil {
		if s := strings.TrimSpace(wrap.Success.Content); s != "" {
			return s
		}
		return strings.TrimSpace(wrap.Content)
	}
	return ""
}

func taskNameFromLabel(label string) string {
	label = strings.TrimSpace(label)
	lower := strings.ToLower(label)
	if strings.HasPrefix(lower, "task ") {
		return strings.TrimSpace(label[5:])
	}
	if strings.EqualFold(label, "task") {
		return ""
	}
	return label
}

func formatNamedTool(key string, val json.RawMessage) string {
	name := humanizeToolName(strings.TrimSuffix(key, "ToolCall"))
	var wrap struct {
		Args map[string]any `json:"args"`
	}
	if json.Unmarshal(val, &wrap) != nil || wrap.Args == nil {
		return name
	}
	if summary := toolArgSummary(wrap.Args); summary != "" {
		return name + " " + summary
	}
	return name
}

func humanizeToolName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Tool"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func isTaskToolLabel(label string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(label)), "task")
}

func toolArgSummary(args map[string]any) string {
	for _, k := range []string{"path", "file_path", "query", "pattern", "glob", "command", "url", "name", "description"} {
		v, ok := args[k]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s != "" {
			return s
		}
	}
	return ""
}

func toolArgSummaryFromJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var args map[string]any
	if json.Unmarshal([]byte(raw), &args) != nil {
		return ""
	}
	return toolArgSummary(args)
}

// shouldEmitDelta follows Cursor stream-partial-output guidance:
// use events with timestamp_ms present and model_call_id absent; skip duplicate
// flushes; emit complete assistant messages when not in partial mode.
func shouldEmitDelta(ev cliStreamEvent, sawPartial bool) bool {
	if ev.ModelCallID != nil && *ev.ModelCallID != "" {
		return false
	}
	if ev.TimestampMS != nil {
		return true
	}
	// Final flush in partial mode (no timestamp) — skip duplicate text.
	if sawPartial {
		return false
	}
	return true
}

func summarize(out string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	if idx := strings.IndexByte(out, '\n'); idx >= 0 {
		line := strings.TrimSpace(out[:idx])
		if len(line) > 120 {
			return line[:117] + "..."
		}
		return line
	}
	if len(out) > 120 {
		return out[:117] + "..."
	}
	return out
}
