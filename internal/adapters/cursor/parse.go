package cursor

import (
	"bufio"
	"bytes"
	"context"
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
	InputTokens              int64 `json:"inputTokens"`
	OutputTokens             int64 `json:"outputTokens"`
	InputTokensSnake         int64 `json:"input_tokens"`
	OutputTokensSnake        int64 `json:"output_tokens"`
	CacheReadTokens          int64 `json:"cacheReadTokens"`
	CacheReadTokensSnake     int64 `json:"cache_read_tokens"`
	CacheWriteTokens         int64 `json:"cacheWriteTokens"`
	CacheWriteTokensSnake    int64 `json:"cache_write_tokens"`
	CacheCreationTokens      int64 `json:"cacheCreationTokens"`
	CacheCreationTokensSnake int64 `json:"cache_creation_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
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
	ToolResult       json.RawMessage `json:"tool_result"` // top-level tool_result events
	Event            json.RawMessage `json:"event"`       // stream_event SSE wrapper
	CallID           string          `json:"call_id"`
	AgentID          string          `json:"agent_id"`
	ParentToolCallID string          `json:"parent_tool_call_id"`
	// system/api_retry fields
	Attempt      int    `json:"attempt"`
	MaxRetries   int    `json:"max_retries"`
	RetryDelayMS int64  `json:"retry_delay_ms"`
	ErrorStatus  int    `json:"error_status"`
	ErrorMsg     string `json:"error"` // top-level error event message and api_retry code
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
	cacheRead := u.CacheReadTokens
	if cacheRead == 0 {
		cacheRead = u.CacheReadTokensSnake
	}
	cacheWrite := u.CacheWriteTokens
	if cacheWrite == 0 {
		cacheWrite = u.CacheWriteTokensSnake
	}
	if cacheWrite == 0 {
		cacheWrite = u.CacheCreationTokens
	}
	if cacheWrite == 0 {
		cacheWrite = u.CacheCreationTokensSnake
	}
	if cacheWrite == 0 {
		cacheWrite = u.CacheCreationInputTokens
	}
	return harness.Usage{
		InputTokens:      in,
		OutputTokens:     out,
		CacheReadTokens:  cacheRead,
		CacheWriteTokens: cacheWrite,
	}
}

// cursorOccupancy prefers the last per-call usage event. A billed result
// snapshot is occupancy only when the run had a single model call (no tools).
func cursorOccupancy(lastCall harness.Usage, sawToolCall bool, billed harness.Usage) int64 {
	if lastCall.ContextTokens > 0 {
		return lastCall.ContextTokens
	}
	if !sawToolCall {
		return billed.CallOccupancy()
	}
	return 0
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

// StreamParseOptions configures ParseStreamJSONWithOptions.
type StreamParseOptions struct {
	OnDelta func(harness.StreamDelta)
	// OnRaw receives one bounded, already-trimmed NDJSON line. It is intended
	// for turn-scoped structured path extraction; callers must not retain raw
	// provider payloads.
	OnRaw func([]byte)
	// OnPermissionRequest is invoked for permission_request NDJSON events. Unlike
	// OpenCode, Cursor headless mode resolves permissions via --force and
	// --approve-mcps; stream-json has no reply channel, so approval in the TUI
	// does not unblock the CLI process.
	OnPermissionRequest func(context.Context, harness.PermissionRequest) (harness.PermissionResponse, error)
}

// ParseStreamJSON reads NDJSON stream-json events and builds an ExecutionResult.
// When onDelta is non-nil, thinking, tool activity, and assistant text are forwarded
// (partial assistant deltas preferred). Thinking/tools are display-only and not
// included in ExecutionResult.Output. Nested Task assistant text is attributed to
// the open Task when the CLI forwards it (or when exactly one Task is in flight).
func ParseStreamJSON(r io.Reader, onDelta func(harness.StreamDelta)) (*harness.ExecutionResult, error) {
	return ParseStreamJSONWithOptions(context.Background(), r, StreamParseOptions{OnDelta: onDelta})
}

// ParseStreamJSONWithOptions is like ParseStreamJSON with permission callbacks.
func ParseStreamJSONWithOptions(ctx context.Context, r io.Reader, opts StreamParseOptions) (*harness.ExecutionResult, error) {
	sc := bufio.NewScanner(r)
	// Cursor stream lines can be large (tool payloads); raise limit.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 10*1024*1024)

	var (
		sessionID      string
		result         *harness.ExecutionResult
		assistant      strings.Builder
		sawPartial     bool
		sawSubstantive bool
		sawToolCall    bool
		lastCallUsage  harness.Usage
		streamError    string // captured from top-level "error" events
		state          = newStreamParseState()
	)

	emit := func(d harness.StreamDelta) {
		if isSubstantiveStreamDelta(d) {
			sawSubstantive = true
		}
		if opts.OnDelta == nil {
			return
		}
		if d.Text == "" && d.Phase == "" {
			return
		}
		if d.Kind == harness.StreamKindText && d.CallID != "" && d.Text != "" {
			state.emittedText[d.CallID] = true
		}
		opts.OnDelta(d)
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
		if opts.OnRaw != nil {
			opts.OnRaw(append([]byte(nil), line...))
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
		case "user":
			// Echo of the user message; no user-visible delta.
		case "system":
			if ev.Subtype == "api_retry" {
				handleCursorAPIRetry(ev, sessionID, emit)
			}
			// Other system subtypes (init, etc.) are lifecycle-only; no delta.
		case "permission_request", "permission":
			if err := handleCursorPermission(ctx, line, ev.SessionID, opts, emit); err != nil {
				return nil, err
			}
		case "permission_decision":
			// Informational; CLI already resolved the request.
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
				sawToolCall = true
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
				if toolCallDenied(ev.ToolCall) {
					emit(harness.StreamDelta{
						Kind:        harness.StreamKindWarning,
						Text:        formatToolPermissionDenied(formatToolCall(ev.ToolCall)),
						HarnessType: "tool_call.permission_denied",
						SessionID:   sessionID,
					})
				}
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
			usage := ev.Usage.toHarness()
			usage.ContextTokens = cursorOccupancy(lastCallUsage, sawToolCall, usage)
			result = &harness.ExecutionResult{
				SessionID:  sessionID,
				Output:     out,
				Summary:    summarize(out),
				Usage:      usage,
				Duration:   time.Duration(ev.DurationMS) * time.Millisecond,
				StreamDone: true,
			}
			if result.SessionID == "" {
				result.SessionID = ev.SessionID
			}
		case "error":
			// Fatal stream error emitted before or instead of a "result" event.
			msg := strings.TrimSpace(ev.ErrorMsg)
			if msg == "" {
				msg = strings.TrimSpace(ev.Result)
			}
			if msg == "" {
				msg = "cursor agent stream error"
			}
			streamError = msg
			// Emit immediately so the TUI status bar updates before the function returns.
			emit(harness.WarningDelta("cursor", "error", sessionID, msg))
		case "tool_result":
			// Tool result emitted as a separate top-level event (alternative to tool_call/completed).
			callID := strings.TrimSpace(ev.CallID)
			if content := extractTaskResultContent(ev.ToolResult); content != "" && !state.emittedText[callID] {
				name, model, parentID := state.attrFromEvent(ev)
				if callID == "" {
					callID = parentID
				}
				emitAttr(harness.StreamKindTool, content, name, model, callID, "")
			}
		case "usage":
			// Per-model-call usage (Detailed+ via StreamKindActivity). The last
			// event is window occupancy; result.usage stays the billed run sum.
			if ev.Usage != nil {
				u := ev.Usage.toHarness().WithCallOccupancy()
				lastCallUsage = u
				summary := fmt.Sprintf("Usage: %d in / %d out tokens", u.InputTokens, u.OutputTokens)
				emit(harness.StreamDelta{
					Kind:        harness.StreamKindActivity,
					Text:        summary,
					HarnessType: "usage",
					SessionID:   sessionID,
				})
				// Cache breakdown is hero-debug-only (too noisy for Detailed).
				if u.CacheReadTokens > 0 || u.CacheWriteTokens > 0 {
					detail := fmt.Sprintf("Usage cache: read=%d write=%d tokens", u.CacheReadTokens, u.CacheWriteTokens)
					emit(harness.StreamDelta{
						Kind:        harness.StreamKindActivity,
						Text:        detail,
						HarnessType: "usage.cache",
						SessionID:   sessionID,
						Metadata:    map[string]string{"hero_debug_only": "true"},
					})
				}
			}
		case "stream_event":
			// SSE wrapper events emitted with --stream-partial-output.
			handleCursorStreamEvent(ev, sessionID, emit)
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
		// A top-level "error" event with no subsequent "result" is a fatal failure.
		if streamError != "" {
			return nil, fmt.Errorf("cursor agent stream error: %s", streamError)
		}
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
	if err := emptyStreamResultError(result, sawSubstantive); err != nil {
		return nil, err
	}
	return result, nil
}

func isSubstantiveStreamDelta(d harness.StreamDelta) bool {
	switch d.Kind {
	case harness.StreamKindText, harness.StreamKindThinking:
		return strings.TrimSpace(d.Text) != ""
	case harness.StreamKindTool:
		return strings.TrimSpace(d.Text) != "" || d.Phase != ""
	default:
		return false
	}
}

func emptyStreamResultError(result *harness.ExecutionResult, sawSubstantive bool) error {
	if result == nil {
		return fmt.Errorf("cursor agent returned empty response")
	}
	if sawSubstantive {
		return nil
	}
	if strings.TrimSpace(result.Output) == "" {
		return fmt.Errorf("cursor agent returned empty response")
	}
	return nil
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
	return harness.HeroAgentFromLabel(s)
}

func isGenericTaskType(s string) bool {
	return harness.IsGenericTaskType(s)
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

func handleCursorPermission(ctx context.Context, line []byte, sessionID string, opts StreamParseOptions, emit func(harness.StreamDelta)) error {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil
	}
	title := cursorStringProp(raw, "name", "tool", "permission", "action")
	if title == "" {
		title = "tool"
	}
	desc := cursorStringProp(raw, "reason", "sideEffect", "message")
	if desc == "" {
		desc = strings.TrimSpace(string(line))
	}
	id, _ := raw["id"].(string)
	evtType, _ := raw["type"].(string)

	if opts.OnPermissionRequest == nil {
		emit(harness.WarningDelta("cursor", evtType, sessionID, desc))
		return fmt.Errorf("cursor permission required (%s) but no OnPermissionRequest handler", title)
	}

	emit(harness.StreamDelta{
		Kind:        harness.StreamKindPermission,
		Text:        title + ": " + desc,
		HarnessType: evtType,
		SessionID:   sessionID,
		Metadata:    map[string]string{"permission_id": id},
	})

	if _, err := opts.OnPermissionRequest(ctx, harness.PermissionRequest{
		ID:          id,
		Title:       title,
		Description: desc,
		HarnessType: evtType,
		SessionID:   sessionID,
	}); err != nil {
		return err
	}
	return nil
}

func cursorStringProp(props map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := props[k].(string)
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func toolCallDenied(raw json.RawMessage) bool {
	s := strings.ToLower(string(raw))
	return strings.Contains(s, "user rejected") ||
		strings.Contains(s, "permission denied") ||
		strings.Contains(s, `"permissiongranted":false`) ||
		strings.Contains(s, `"permission_granted":false`)
}

func formatToolPermissionDenied(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "tool"
	}
	return fmt.Sprintf("Cursor denied %s (permission not granted). Hero runs with --force and --approve-mcps; if this persists, check .cursor/permissions.json or your team MCP allowlist.", label)
}

// handleCursorAPIRetry emits retry progress deltas for system/api_retry events.
//
//   - Standard+: simple "Retrying (N/M)" as StreamKindTool
//   - Detailed+: delay and error code as StreamKindActivity
//   - hero debug: raw payload as StreamKindActivity with hero_debug_only
func handleCursorAPIRetry(ev cliStreamEvent, sessionID string, emit func(harness.StreamDelta)) {
	reason := formatRetryReason(ev.ErrorMsg)
	msg := fmt.Sprintf("Retrying (%d/%d)%s", ev.Attempt, ev.MaxRetries, reason)
	emit(harness.StreamDelta{
		Kind:        harness.StreamKindTool,
		Text:        msg,
		HarnessType: "system.api_retry",
		SessionID:   sessionID,
	})
	if ev.RetryDelayMS > 0 || ev.ErrorStatus != 0 {
		detail := fmt.Sprintf("Retry delay: %dms, status: %d, error: %s", ev.RetryDelayMS, ev.ErrorStatus, ev.ErrorMsg)
		emit(harness.StreamDelta{
			Kind:        harness.StreamKindActivity,
			Text:        detail,
			HarnessType: "system.api_retry.detail",
			SessionID:   sessionID,
		})
	}
}

// formatRetryReason returns a human-readable suffix for known api_retry error codes.
func formatRetryReason(code string) string {
	switch code {
	case "rate_limit":
		return " (rate limited)"
	case "server_error":
		return " (server error)"
	case "billing_error":
		return " (billing error)"
	case "authentication_failed":
		return " (auth failed)"
	case "max_output_tokens":
		return " (max output tokens)"
	default:
		if code != "" {
			return fmt.Sprintf(" (%s)", code)
		}
		return ""
	}
}

// handleCursorStreamEvent processes stream_event SSE wrapper events.
//
//   - message_delta: Detailed+ (StreamKindActivity)
//   - all others: hero debug only (hero_debug_only metadata)
func handleCursorStreamEvent(ev cliStreamEvent, sessionID string, emit func(harness.StreamDelta)) {
	if len(ev.Event) == 0 {
		return
	}
	var inner struct {
		Type  string `json:"type"`
		Delta struct {
			Type       string `json:"type"`
			Text       string `json:"text"`
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
		Usage *cliUsageJSON `json:"usage"`
	}
	if err := json.Unmarshal(ev.Event, &inner); err != nil || inner.Type == "" {
		return
	}
	harnessType := "stream_event." + inner.Type
	switch inner.Type {
	case "message_delta":
		// Stop reason and final usage — Detailed+ via StreamKindActivity.
		text := fmt.Sprintf("stream: stop_reason=%s", inner.Delta.StopReason)
		if inner.Usage != nil {
			u := inner.Usage.toHarness()
			text = fmt.Sprintf("stream: stop_reason=%s out=%d tokens", inner.Delta.StopReason, u.OutputTokens)
		}
		emit(harness.StreamDelta{
			Kind:        harness.StreamKindActivity,
			Text:        text,
			HarnessType: harnessType,
			SessionID:   sessionID,
		})
	default:
		// Protocol-level events (message_start/stop, content_block_start/stop/delta):
		// hero debug only.
		text := inner.Type
		if inner.Delta.Text != "" {
			text = inner.Type + ": " + truncateStreamText(inner.Delta.Text, 120)
		}
		emit(harness.StreamDelta{
			Kind:        harness.StreamKindActivity,
			Text:        text,
			HarnessType: harnessType,
			SessionID:   sessionID,
			Metadata:    map[string]string{"hero_debug_only": "true"},
		})
	}
}

func truncateStreamText(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
