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
	Type         string `json:"type"`
	Subtype      string `json:"subtype"`
	IsError      bool   `json:"is_error"`
	DurationMS   int64  `json:"duration_ms"`
	Result       string `json:"result"`
	SessionID    string `json:"session_id"`
	RequestID    string `json:"request_id"`
	Usage        *cliUsageJSON `json:"usage"`
}

type cliUsageJSON struct {
	InputTokens       int64 `json:"inputTokens"`
	OutputTokens      int64 `json:"outputTokens"`
	InputTokensSnake  int64 `json:"input_tokens"`
	OutputTokensSnake int64 `json:"output_tokens"`
}

type cliStreamEvent struct {
	Type       string          `json:"type"`
	Subtype    string          `json:"subtype"`
	SessionID  string          `json:"session_id"`
	Result     string          `json:"result"`
	IsError    bool            `json:"is_error"`
	DurationMS int64           `json:"duration_ms"`
	Usage      *cliUsageJSON   `json:"usage"`
	Message    *cliMessage     `json:"message"`
	Text       string          `json:"text"` // thinking delta
	ToolCall   json.RawMessage `json:"tool_call"`
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
// included in ExecutionResult.Output.
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
	)

	emit := func(kind harness.StreamKind, text string) {
		if onDelta == nil || text == "" {
			return
		}
		onDelta(harness.StreamDelta{Kind: kind, Text: text})
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
		case "thinking":
			if ev.Subtype == "completed" {
				continue
			}
			text := ev.Text
			if text == "" {
				text = extractContentText(ev, "thinking", "reasoning")
			}
			emit(harness.StreamKindThinking, text)
		case "tool_call":
			label := formatToolCall(ev.ToolCall)
			if label == "" {
				continue
			}
			switch ev.Subtype {
			case "", "started":
				emit(harness.StreamKindTool, label)
			case "completed":
				if isTaskToolLabel(label) {
					emit(harness.StreamKindTool, label+" (completed)")
				}
			}
		case "assistant":
			if ev.TimestampMS != nil {
				sawPartial = true
			}
			if thinking := extractContentText(ev, "thinking", "reasoning"); thinking != "" && shouldEmitDelta(ev, sawPartial) {
				emit(harness.StreamKindThinking, thinking)
			}
			text := extractAssistantText(ev)
			if text == "" {
				continue
			}
			if !shouldEmitDelta(ev, sawPartial) {
				continue
			}
			emit(harness.StreamKindText, text)
			assistant.WriteString(text)
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
