package opencode

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// streamState tracks incremental message reconstruction across SSE events.
type streamState struct {
	partTexts            map[string]string
	emittedText          map[string]string // text already sent to UI per stream key
	assistantMsgID       string
	agentName            string
	callID               string
	connected            bool
	lastTextPartID       string // last assistant text part that emitted output
	sessionNextReasoning bool   // v2 reasoning stream active; skip v1 reasoning parts
	usage                harness.Usage
}

func newStreamState() *streamState {
	return &streamState{
		partTexts:   make(map[string]string),
		emittedText: make(map[string]string),
	}
}

type streamHandler struct {
	adapter   *Adapter
	ctx       context.Context
	sessionID string
	req       harness.ExecuteRequest
	state     *streamState
	emit      func(harness.StreamDelta)
	textBuf   *strings.Builder
}

type streamOutcome struct {
	done bool
	err  error
}

// opencodeActivityEvents lists known non-streaming OpenCode events (EventManifest.Definitions).
var opencodeActivityEvents = map[string]struct{}{
	"catalog.updated":                {},
	"command.executed":               {},
	"file.edited":                    {},
	"file.watcher.updated":           {},
	"ide.installed":                  {},
	"installation.updated":           {},
	"installation.update-available":  {},
	"integration.updated":            {},
	"integration.connection.updated": {},
	"lsp.updated":                    {},
	"mcp.browser.open.failed":        {},
	"mcp.tools.changed":              {},
	"message.removed":                {},
	"message.part.removed":           {},
	"permission.replied":             {},
	"permission.v2.replied":          {},
	"plugin.added":                   {},
	"project.updated":                {},
	"project.directories.updated":    {},
	"pty.created":                    {},
	"pty.updated":                    {},
	"pty.exited":                     {},
	"pty.deleted":                    {},
	"question.asked":                 {},
	"question.replied":               {},
	"question.rejected":              {},
	"question.v2.replied":            {},
	"question.v2.rejected":           {},
	"reference.updated":              {},
	"session.created":                {},
	"session.updated":                {},
	"session.deleted":                {},
	"session.compacted":              {},
	"session.diff":                   {},
	"todo.updated":                   {},
	"tui.prompt.append":              {},
	"tui.command.execute":            {},
	"tui.toast.show":                 {},
	"tui.session.select":             {},
	"vcs.branch.updated":             {},
	"workspace.ready":                {},
	"workspace.status":               {},
	"workspace.failed":               {},
	"worktree.ready":                 {},
	"worktree.failed":                {},
	"global.disposed":                {},
	// Legacy events kept for older OpenCode builds.
	"lsp.client.diagnostics": {},
	"shell.env":              {},
}

// opencodeDebugOnlyActivities are hidden from chat unless ExecuteRequest.Debug is set.
var opencodeDebugOnlyActivities = map[string]struct{}{
	"session.updated":     {},
	"session.diff":        {},
	"plugin.added":        {},
	"reference.updated":   {},
	"integration.updated": {},
}

func (a *Adapter) processSSEEvent(ctx context.Context, evt map[string]any, sessionID string, state *streamState, req harness.ExecuteRequest, textBuf *strings.Builder) streamOutcome {
	h := &streamHandler{
		adapter:   a,
		ctx:       ctx,
		sessionID: sessionID,
		req:       req,
		state:     state,
		textBuf:   textBuf,
		emit: func(d harness.StreamDelta) {
			if req.OnStreamDelta == nil {
				return
			}
			if d.SessionID == "" {
				d.SessionID = sessionID
			}
			req.OnStreamDelta(d)
			if d.Kind == harness.StreamKindText && d.Text != "" && textBuf != nil {
				textBuf.WriteString(d.Text)
			}
		},
	}
	return h.handle(evt)
}

func (h *streamHandler) handle(evt map[string]any) streamOutcome {
	evtType := normalizeVersionedEventType(evtTypeString(evt))
	props := eventProperties(evt)
	if sid := eventSessionID(evt, props); sid != "" && sid != h.sessionID {
		return streamOutcome{}
	}

	switch evtType {
	case "server.connected":
		h.state.connected = true
		h.emit(harness.ActivityDelta(evtType, "OpenCode server connected", h.sessionID))
		return streamOutcome{}

	case "server.heartbeat":
		return streamOutcome{}

	case "server.instance.disposed":
		h.emit(harness.ActivityDelta(evtType, "OpenCode instance disposed", h.sessionID))
		return streamOutcome{done: true}

	case "sync":
		return h.handleSync(props)

	case "session.idle":
		h.flushPendingStreams()
		h.emit(harness.SessionDelta(harness.SessionStateIdle, "", evtType, h.sessionID))
		return streamOutcome{done: true}

	case "session.status":
		if st, ok := props["status"].(map[string]any); ok {
			stType, _ := st["type"].(string)
			if stType == "idle" {
				h.emit(harness.SessionDelta(harness.SessionStateIdle, "", evtType, h.sessionID))
				return streamOutcome{done: true}
			}
			h.emit(harness.SessionDelta(harness.SessionStateRunning, stType, evtType, h.sessionID))
		}
		return streamOutcome{}

	case "session.error":
		msg := stringProp(props, "error", "message")
		if msg == "" {
			msg = "session error"
		}
		h.emit(harness.SessionDelta(harness.SessionStateFailed, msg, evtType, h.sessionID))
		return streamOutcome{err: fmt.Errorf("opencode session error: %s", msg)}

	case "message.updated":
		h.handleMessageUpdated(props)
		return streamOutcome{}

	case "message.part.updated":
		return h.handlePartUpdated(props)

	case "message.part.delta":
		return h.handlePartDelta(props)

	case "message.done":
		if sid := eventSessionID(evt, props); sid == "" || sid == h.sessionID {
			h.flushPendingStreams()
			return streamOutcome{done: true}
		}
		return streamOutcome{}

	case "tool.execute.before":
		return h.emitToolPhase(props, harness.StreamPhaseStarted, evtType, "")

	case "tool.execute.after":
		return h.emitToolPhase(props, harness.StreamPhaseCompleted, evtType, " (completed)")

	case "permission.asked", "permission.v2.asked":
		if err := h.handlePermissionAsked(props, evtType); err != nil {
			return streamOutcome{err: err}
		}
		return streamOutcome{}

	case "question.asked", "question.v2.asked":
		if err := h.handleQuestionAsked(props, evtType); err != nil {
			return streamOutcome{err: err}
		}
		return streamOutcome{}
	}

	if strings.HasPrefix(evtType, "session.next.") {
		return h.handleSessionNext(evtType, props)
	}

	if _, ok := opencodeActivityEvents[evtType]; ok {
		h.emitActivityEvent(evtType, props)
		return streamOutcome{}
	}

	if evtType == "" {
		h.emitWarning(evtType, evt)
		return streamOutcome{}
	}

	h.emitWarning(evtType, evt)
	return streamOutcome{}
}

func (h *streamHandler) handleSync(props map[string]any) streamOutcome {
	se, ok := props["syncEvent"].(map[string]any)
	if !ok {
		return streamOutcome{}
	}
	innerType, _ := se["type"].(string)
	innerProps, _ := se["data"].(map[string]any)
	if innerType == "" {
		return streamOutcome{}
	}
	inner := map[string]any{"type": innerType, "properties": innerProps}
	if id, ok := se["id"].(string); ok {
		inner["id"] = id
	}
	return h.handle(inner)
}

func (h *streamHandler) emitActivityEvent(evtType string, props map[string]any) {
	if evtType == "catalog.updated" {
		if summary, ok := catalogUserSummary(props); ok {
			h.emit(harness.ActivityDelta(evtType, summary, h.sessionID))
			return
		}
		if !h.req.Debug {
			return
		}
	} else if _, debugOnly := opencodeDebugOnlyActivities[evtType]; debugOnly && !h.req.Debug {
		return
	}
	summary := activitySummary(evtType, props)
	if evtType == "session.diff" {
		summary = diffSummary(props)
	}
	h.emit(harness.ActivityDelta(evtType, summary, h.sessionID))
}

func catalogUserSummary(props map[string]any) (string, bool) {
	title := stringProp(props, "title", "name")
	items := catalogItemNames(props)
	if title == "" && len(items) == 0 {
		return "", false
	}
	if title == "" {
		title = "catalog"
	}
	if len(items) == 0 {
		return title, true
	}
	var b strings.Builder
	b.WriteString(title)
	for _, item := range items {
		b.WriteString("\n  - ")
		b.WriteString(item)
	}
	return b.String(), true
}

func catalogItemNames(props map[string]any) []string {
	for _, key := range []string{"items", "elements", "entries", "models", "catalog"} {
		if names := stringListFromProp(props, key); len(names) > 0 {
			return names
		}
	}
	return nil
}

func stringListFromProp(props map[string]any, key string) []string {
	raw, ok := props[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		return flattenStringList(v)
	case map[string]any:
		if nested, ok := v["items"].([]any); ok {
			return flattenStringList(nested)
		}
		if nested, ok := v["elements"].([]any); ok {
			return flattenStringList(nested)
		}
		var names []string
		for k := range v {
			names = append(names, k)
		}
		return names
	default:
		return nil
	}
}

func flattenStringList(items []any) []string {
	var out []string
	for _, item := range items {
		switch v := item.(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				out = append(out, s)
			}
		case map[string]any:
			if s := stringProp(v, "name", "title", "id", "slug"); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func (h *streamHandler) emittedStreamKey(key string) string {
	return "emitted:" + key
}

func (h *streamHandler) emitAuthoritativeText(key, full, evtType string) {
	if full == "" {
		return
	}
	if h.state.emittedText == nil {
		h.state.emittedText = make(map[string]string)
	}
	emKey := h.emittedStreamKey(key)
	emitted := h.state.emittedText[emKey]
	h.state.partTexts[key] = full
	if full == emitted {
		return
	}
	if strings.HasPrefix(full, emitted) {
		if suffix := full[len(emitted):]; suffix != "" {
			h.emitTextDelta(suffix, evtType)
		}
		h.state.emittedText[emKey] = full
		return
	}
	if strings.HasPrefix(emitted, full) {
		return
	}
	// Token deltas and authoritative snapshots can diverge — emit the full answer.
	h.emitTextDelta(full, evtType)
	h.state.emittedText[emKey] = full
}

func (h *streamHandler) emitAuthoritativeThinking(key, full, evtType string) {
	if full == "" {
		return
	}
	if h.state.emittedText == nil {
		h.state.emittedText = make(map[string]string)
	}
	emKey := h.emittedStreamKey(key)
	emitted := h.state.emittedText[emKey]
	h.state.partTexts[key] = full
	if full == emitted {
		return
	}
	if strings.HasPrefix(full, emitted) {
		if suffix := full[len(emitted):]; suffix != "" {
			h.emit(harness.StreamDelta{
				Kind:        harness.StreamKindThinking,
				Text:        suffix,
				AgentName:   h.state.agentName,
				CallID:      h.state.callID,
				HarnessType: evtType,
				SessionID:   h.sessionID,
			})
		}
		h.state.emittedText[emKey] = full
		return
	}
	if strings.HasPrefix(emitted, full) {
		return
	}
	h.emit(harness.StreamDelta{
		Kind:        harness.StreamKindThinking,
		Text:        full,
		AgentName:   h.state.agentName,
		CallID:      h.state.callID,
		HarnessType: evtType,
		SessionID:   h.sessionID,
	})
	h.state.emittedText[emKey] = full
}

func sessionNextReasoningKey(reasoningID string) string {
	if reasoningID == "" {
		return "reasoning:default"
	}
	return "reasoning:" + reasoningID
}

func (h *streamHandler) handleSessionNext(evtType string, props map[string]any) streamOutcome {
	switch evtType {
	case "session.next.agent.switched":
		if agent := stringProp(props, "agent"); agent != "" {
			h.state.agentName = agent
		}
		h.emit(harness.ActivityDelta(evtType, "agent: "+h.state.agentName, h.sessionID))

	case "session.next.text.started":
		h.noteAssistantMessageID(props)

	case "session.next.text.delta":
		h.noteAssistantMessageID(props)
		textID := stringProp(props, "textID")
		delta := stringPropRaw(props, "delta")
		if delta == "" {
			break
		}
		key := textStreamKey(textID)
		h.state.partTexts[key] = h.state.partTexts[key] + delta

	case "session.next.text.ended":
		h.noteAssistantMessageID(props)
		textID := stringProp(props, "textID")
		key := textStreamKey(textID)
		h.emitAuthoritativeText(key, stringPropRaw(props, "text"), evtType)

	case "session.next.reasoning.started":
		h.noteAssistantMessageID(props)
		h.state.sessionNextReasoning = true

	case "session.next.reasoning.delta":
		h.noteAssistantMessageID(props)
		h.state.sessionNextReasoning = true
		reasoningID := stringProp(props, "reasoningID")
		delta := stringPropRaw(props, "delta")
		if delta == "" {
			break
		}
		key := sessionNextReasoningKey(reasoningID)
		prev := h.state.partTexts[key]
		h.state.partTexts[key] = prev + delta

	case "session.next.reasoning.ended":
		h.noteAssistantMessageID(props)
		h.state.sessionNextReasoning = true
		reasoningID := stringProp(props, "reasoningID")
		key := sessionNextReasoningKey(reasoningID)
		h.emitAuthoritativeThinking(key, stringPropRaw(props, "text"), evtType)

	case "session.next.tool.input.started", "session.next.tool.called":
		return h.emitToolPhase(props, harness.StreamPhaseStarted, evtType, "")

	case "session.next.tool.success":
		return h.emitToolPhase(props, harness.StreamPhaseCompleted, evtType, " (completed)")

	case "session.next.tool.failed":
		return h.emitToolPhase(props, harness.StreamPhaseCompleted, evtType, " (failed)")

	case "session.next.shell.started":
		callID := stringProp(props, "callID")
		h.state.callID = callID
		cmd := stringProp(props, "command")
		if cmd == "" {
			cmd = "shell"
		}
		h.emit(harness.StreamDelta{
			Kind:        harness.StreamKindTool,
			Text:        "executing command: " + truncate(cmd, 80),
			AgentName:   h.state.agentName,
			CallID:      callID,
			Phase:       harness.StreamPhaseStarted,
			HarnessType: evtType,
			SessionID:   h.sessionID,
		})

	case "session.next.shell.ended":
		h.emit(harness.ActivityDelta(evtType, "shell ended", h.sessionID))

	default:
		h.emit(harness.ActivityDelta(evtType, activitySummary(evtType, props), h.sessionID))
	}
	return streamOutcome{}
}

func (h *streamHandler) emitToolPhase(props map[string]any, phase, evtType, suffix string) streamOutcome {
	callID := toolCallID(props)
	h.state.callID = callID
	toolName := toolNameFromProps(props)
	label := toolLabelByName(toolName)
	agentName := h.state.agentName
	model := ""
	if harness.IsTaskToolName(toolName) {
		taskName, taskModel := taskInfoFromProps(props)
		if taskName != "" {
			agentName = taskName
		}
		model = taskModel
		if taskName != "" {
			label = "Task " + taskName
		} else {
			label = "Task " + toolName
		}
	}
	if suffix != "" {
		label += suffix
	}
	h.emit(harness.StreamDelta{
		Kind:        harness.StreamKindTool,
		Text:        label,
		AgentName:   agentName,
		Model:       model,
		CallID:      callID,
		Phase:       phase,
		HarnessType: evtType,
		SessionID:   h.sessionID,
	})
	return streamOutcome{}
}

func toolNameFromProps(props map[string]any) string {
	name := stringProp(props, "tool", "name")
	if name == "" {
		if tool, ok := props["tool"].(map[string]any); ok {
			name = stringProp(tool, "name", "type", "permission")
		}
	}
	return name
}

func taskInfoFromProps(props map[string]any) (name, model string) {
	args := map[string]any{}
	for _, key := range []string{"input", "args", "arguments"} {
		if m, ok := props[key].(map[string]any); ok {
			args = m
			break
		}
	}
	if tool, ok := props["tool"].(map[string]any); ok {
		if m, ok := tool["input"].(map[string]any); ok && len(args) == 0 {
			args = m
		}
	}
	candidates := []string{
		stringProp(args, "subagent_type", "agent", "name", "description", "prompt"),
		stringProp(props, "subagent_type", "agent", "description"),
	}
	for _, c := range candidates {
		if n := harness.HeroAgentFromLabel(c); n != "" {
			name = n
			break
		}
		if c != "" && !harness.IsGenericTaskType(c) {
			name = c
			break
		}
		if harness.IsGenericTaskType(c) {
			name = c
			break
		}
	}
	model = stringProp(args, "model")
	if model == "" {
		model = stringProp(props, "model")
	}
	return name, model
}

func (h *streamHandler) emitPartTextGrowth(key, delta, evtType string) {
	if delta == "" {
		return
	}
	if h.state.emittedText == nil {
		h.state.emittedText = make(map[string]string)
	}
	emKey := h.emittedStreamKey(key)
	h.emitTextDelta(delta, evtType)
	h.state.emittedText[emKey] += delta
}

func (h *streamHandler) emitTextDelta(delta, evtType string) {
	if delta == "" {
		return
	}
	h.emit(harness.StreamDelta{
		Kind:        harness.StreamKindText,
		Text:        delta,
		AgentName:   h.state.agentName,
		CallID:      h.state.callID,
		HarnessType: evtType,
		SessionID:   h.sessionID,
	})
}

func textPartKey(partID, field string) string {
	return textStreamKey(partID)
}

func textStreamKey(id string) string {
	if id == "" {
		return "text:default"
	}
	return "text:" + id
}

func evtTypeString(evt map[string]any) string {
	t, _ := evt["type"].(string)
	return t
}

func normalizeVersionedEventType(t string) string {
	if t == "" {
		return t
	}
	i := strings.LastIndex(t, ".")
	if i <= 0 {
		return t
	}
	suffix := t[i+1:]
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return t
		}
	}
	if suffix == "" {
		return t
	}
	return t[:i]
}

func (h *streamHandler) flushPendingStreams() {
	for key, full := range h.state.partTexts {
		if full == "" {
			continue
		}
		switch {
		case strings.HasPrefix(key, "text:"):
			h.emitAuthoritativeText(key, full, "session.idle")
		case strings.HasPrefix(key, "reasoning:"):
			h.emitAuthoritativeThinking(key, full, "session.idle")
		}
	}
}

func (h *streamHandler) noteAssistantMessageID(props map[string]any) {
	id := stringProp(props, "assistantMessageID", "assistantMessageId")
	if id == "" {
		return
	}
	if id != h.state.assistantMsgID {
		h.state.sessionNextReasoning = false
		h.state.lastTextPartID = ""
	}
	h.state.assistantMsgID = id
}

// partBoundaryDelta inserts a space only when OpenCode starts a new text part after a
// previous one (separate part IDs), not between streaming chunks of the same part.
func (h *streamHandler) partBoundaryDelta(partID, prevInPart, delta string) string {
	if prevInPart != "" || partID == "" || h.state.lastTextPartID == "" || partID == h.state.lastTextPartID {
		return delta
	}
	return maybePartBoundarySpace(delta)
}

func maybePartBoundarySpace(delta string) string {
	if delta == "" {
		return delta
	}
	if strings.HasPrefix(delta, " ") || strings.HasPrefix(delta, "\n") || strings.HasPrefix(delta, "\t") {
		return delta
	}
	r, _ := utf8.DecodeRuneInString(delta)
	if r == utf8.RuneError || unicode.IsPunct(r) {
		return delta
	}
	return " " + delta
}

func (h *streamHandler) markTextPartEmitted(partID string) {
	if partID != "" {
		h.state.lastTextPartID = partID
	}
}

func (h *streamHandler) handleMessageUpdated(props map[string]any) {
	info, ok := props["info"].(map[string]any)
	if !ok {
		return
	}
	if role, _ := info["role"].(string); role == "assistant" {
		if id, _ := info["id"].(string); id != "" {
			h.noteAssistantMessageID(map[string]any{"assistantMessageID": id})
		}
	}
	if agent, _ := info["agent"].(string); agent != "" {
		h.state.agentName = agent
	}
	h.noteUsage(info)
}

func (h *streamHandler) handlePartDelta(props map[string]any) streamOutcome {
	delta := stringProp(props, "delta")
	if delta == "" {
		return streamOutcome{}
	}
	msgID := stringProp(props, "messageID")
	if h.state.assistantMsgID != "" && msgID != "" && msgID != h.state.assistantMsgID {
		return streamOutcome{}
	}
	// Live-only fragments; message.part.updated carries the growing full value.
	partID := stringProp(props, "partID")
	field := stringProp(props, "field")
	key := textPartKey(partID, field)
	prev := h.state.partTexts[key]
	h.state.partTexts[key] = prev + delta
	return streamOutcome{}
}

func (h *streamHandler) handlePartUpdated(props map[string]any) streamOutcome {
	part, ok := props["part"].(map[string]any)
	if !ok {
		return streamOutcome{}
	}
	pt, _ := part["type"].(string)
	msgID, _ := part["messageID"].(string)
	if h.state.assistantMsgID == "" {
		return streamOutcome{}
	}
	if msgID != "" && msgID != h.state.assistantMsgID {
		return streamOutcome{}
	}

	switch pt {
	case "step-start":
		if h.req.Debug {
			h.emit(harness.ActivityDelta("message.part.updated", "step started", h.sessionID))
		}
		return streamOutcome{}
	case "step-finish":
		h.noteUsage(part)
		if h.req.Debug {
			h.emit(harness.ActivityDelta("message.part.updated", "step finished", h.sessionID))
		}
		return streamOutcome{}
	case "tool":
		if h.req.Debug {
			label := toolLabelFromProps(part)
			if label == "" {
				label = "tool"
			}
			h.emit(harness.ActivityDelta("message.part.updated", "tool: "+label, h.sessionID))
		}
		return streamOutcome{}
	case "text", "":
		full, _ := part["text"].(string)
		partID, _ := part["id"].(string)
		key := textPartKey(partID, "text")
		if h.state.emittedText == nil {
			h.state.emittedText = make(map[string]string)
		}
		emKey := h.emittedStreamKey(key)
		emitted := h.state.emittedText[emKey]
		if full == emitted {
			h.state.partTexts[key] = full
			return streamOutcome{}
		}
		if strings.HasPrefix(full, emitted) {
			suffix := full[len(emitted):]
			if emitted == "" && h.state.lastTextPartID != "" && partID != h.state.lastTextPartID {
				suffix = maybePartBoundarySpace(suffix)
			}
			h.state.partTexts[key] = full
			h.markTextPartEmitted(partID)
			h.emitPartTextGrowth(key, suffix, "message.part.updated")
			return streamOutcome{}
		}
		if strings.HasPrefix(emitted, full) {
			h.state.partTexts[key] = full
			return streamOutcome{}
		}
		h.markTextPartEmitted(partID)
		h.emitAuthoritativeText(key, full, "message.part.updated")
	case "reasoning", "thinking":
		if h.state.sessionNextReasoning {
			full, _ := part["text"].(string)
			partID, _ := part["id"].(string)
			if partID != "" {
				h.state.partTexts[partID] = full
			}
			return streamOutcome{}
		}
		full, _ := part["text"].(string)
		partID, _ := part["id"].(string)
		prev := h.state.partTexts[partID]
		if len(full) <= len(prev) {
			return streamOutcome{}
		}
		delta := full[len(prev):]
		h.state.partTexts[partID] = full
		h.emit(harness.StreamDelta{
			Kind:        harness.StreamKindThinking,
			Text:        delta,
			AgentName:   h.state.agentName,
			CallID:      h.state.callID,
			HarnessType: "message.part.updated",
			SessionID:   h.sessionID,
		})
	default:
		h.emit(harness.ActivityDelta("message.part.updated", fmt.Sprintf("part type %s", pt), h.sessionID))
	}
	return streamOutcome{}
}

func (h *streamHandler) handlePermissionAsked(props map[string]any, evtType string) error {
	id, _ := props["id"].(string)
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("opencode permission event missing id")
	}
	perm := stringProp(props, "permission", "action")
	patterns := stringSliceProp(props, "patterns")
	if len(patterns) == 0 {
		patterns = stringSliceProp(props, "resources")
	}
	title := perm
	if title == "" {
		title = "permission"
	}
	desc := strings.Join(patterns, ", ")
	if desc == "" {
		if meta, ok := props["metadata"].(map[string]any); ok {
			desc = stringProp(meta, "filepath", "parentDir")
		}
	}
	if desc == "" {
		if raw, err := json.Marshal(props); err == nil {
			desc = truncate(string(raw), 200)
		}
	}

	if h.req.OnPermissionRequest == nil {
		h.emit(harness.WarningDelta(adapterName, evtType, h.sessionID, desc))
		return fmt.Errorf("opencode permission required (%s) but no OnPermissionRequest handler", title)
	}

	h.emit(harness.StreamDelta{
		Kind:        harness.StreamKindPermission,
		Text:        title + ": " + desc,
		HarnessType: evtType,
		SessionID:   h.sessionID,
		Metadata:    map[string]string{"permission_id": id},
	})

	resp, err := h.req.OnPermissionRequest(h.ctx, harness.PermissionRequest{
		ID:          id,
		Title:       title,
		Description: desc,
		HarnessType: evtType,
		SessionID:   h.sessionID,
	})
	if err != nil {
		return err
	}
	reply := "reject"
	if resp.Approved {
		reply = "once"
	}
	if err := h.adapter.replyPermission(h.ctx, h.sessionID, id, reply, h.req.ProjectDir); err != nil {
		return fmt.Errorf("opencode permission reply: %w", err)
	}
	return nil
}

func (h *streamHandler) handleQuestionAsked(props map[string]any, evtType string) error {
	id := stringProp(props, "id")
	if id == "" {
		return fmt.Errorf("opencode question event missing id")
	}
	questions := parseQuestionItems(props)
	if len(questions) == 0 {
		return fmt.Errorf("opencode question event missing questions")
	}
	req := harness.QuestionRequest{
		ID:          id,
		SessionID:   h.sessionID,
		HarnessType: evtType,
		Questions:   questions,
	}

	if h.req.OnQuestionRequest == nil {
		h.emit(harness.WarningDelta(adapterName, evtType, h.sessionID, formatQuestionPrompt(req, 0)))
		return fmt.Errorf("opencode question required but no OnQuestionRequest handler")
	}

	h.emit(harness.StreamDelta{
		Kind:        harness.StreamKindQuestion,
		Text:        formatQuestionPrompt(req, 0),
		HarnessType: evtType,
		SessionID:   h.sessionID,
		Metadata:    map[string]string{"question_id": id},
	})

	resp, err := h.req.OnQuestionRequest(h.ctx, req)
	if err != nil {
		return err
	}
	if resp.Rejected {
		if err := h.adapter.rejectQuestion(h.ctx, h.sessionID, id, h.req.ProjectDir); err != nil {
			return fmt.Errorf("opencode question reject: %w", err)
		}
		return nil
	}
	if len(resp.Answers) == 0 {
		return fmt.Errorf("opencode question response missing answers")
	}
	if err := h.adapter.replyQuestion(h.ctx, h.sessionID, id, resp.Answers, h.req.ProjectDir); err != nil {
		return fmt.Errorf("opencode question reply: %w", err)
	}
	return nil
}

func (h *streamHandler) emitWarning(evtType string, evt map[string]any) {
	raw, _ := json.Marshal(evt)
	h.emit(harness.WarningDelta(adapterName, evtType, h.sessionID, string(raw)))
}

func (a *Adapter) replyPermission(ctx context.Context, sessionID, requestID, reply, projectDir string) error {
	body, _ := json.Marshal(map[string]string{"reply": reply})
	path := "/permission/" + url.PathEscape(requestID) + "/reply"
	path = withDirectoryQuery(path, projectDir, a.ProjectDir)
	resp, err := a.post(ctx, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound && strings.TrimSpace(sessionID) != "" {
		resp.Body.Close()
		sessionPath := "/session/" + url.PathEscape(sessionID) + "/permission/" + url.PathEscape(requestID) + "/reply"
		sessionPath = withDirectoryQuery(sessionPath, projectDir, a.ProjectDir)
		resp, err = a.post(ctx, sessionPath, body)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}
	return httpOK(resp)
}

func withDirectoryQuery(path, projectDir, adapterDir string) string {
	dir := strings.TrimSpace(projectDir)
	if dir == "" {
		dir = strings.TrimSpace(adapterDir)
	}
	if dir == "" {
		return path
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + "directory=" + url.QueryEscape(dir)
}

func readSSEEvents(ctx context.Context, body io.Reader, handler func(map[string]any) error, onMalformed func(string)) error {
	scanner := bufio.NewScanner(body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		var evt map[string]any
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			if onMalformed != nil {
				onMalformed(payload)
			}
			slog.Warn("opencode sse malformed json", "error", err)
			continue
		}
		if err := handler(evt); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// parseEventDelta is retained for unit tests; production streaming uses processSSEEvent.
func parseEventDelta(evt map[string]any, sessionID string, state *streamState) (text string, done bool) {
	var captured strings.Builder
	out := (&Adapter{}).processSSEEvent(context.Background(), evt, sessionID, state, harness.ExecuteRequest{
		OnStreamDelta: func(d harness.StreamDelta) {
			if d.Kind == harness.StreamKindText {
				captured.WriteString(d.Text)
			}
		},
	}, nil)
	return captured.String(), out.done
}

func eventProperties(evt map[string]any) map[string]any {
	if props, ok := evt["properties"].(map[string]any); ok {
		return props
	}
	if data, ok := evt["data"].(map[string]any); ok {
		return data
	}
	return nil
}

func eventSessionID(evt, props map[string]any) string {
	if sid, _ := evt["sessionID"].(string); sid != "" {
		return sid
	}
	if props != nil {
		if sid, _ := props["sessionID"].(string); sid != "" {
			return sid
		}
	}
	return ""
}

func stringProp(props map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := props[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// noteUsage extracts token counts from OpenCode message.info or step-finish parts.
func (h *streamHandler) noteUsage(m map[string]any) {
	if h == nil || h.state == nil || m == nil {
		return
	}
	usage := extractOpenCodeUsage(m)
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return
	}
	h.state.usage = usage
}

func extractOpenCodeUsage(m map[string]any) harness.Usage {
	usage := harness.Usage{}
	if m == nil {
		return usage
	}
	extract := func(src map[string]any) {
		if src == nil {
			return
		}
		if in := int64Field(src, "input", "inputTokens", "input_tokens", "prompt", "promptTokens"); in > 0 {
			usage.InputTokens = in
		}
		if out := int64Field(src, "output", "outputTokens", "output_tokens", "completion", "completionTokens"); out > 0 {
			usage.OutputTokens = out
		}
		if total := int64Field(src, "total", "totalTokens", "tokensUsed"); total > 0 && usage.InputTokens == 0 && usage.OutputTokens == 0 {
			usage.InputTokens = total
		}
	}
	if tokens, ok := m["tokens"].(map[string]any); ok {
		extract(tokens)
	}
	if u, ok := m["usage"].(map[string]any); ok {
		extract(u)
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		extract(m)
	}
	return usage
}

func int64Field(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			if n > 0 {
				return int64(n)
			}
		case int64:
			if n > 0 {
				return n
			}
		case int:
			if n > 0 {
				return int64(n)
			}
		case json.Number:
			i, err := n.Int64()
			if err == nil && i > 0 {
				return i
			}
		}
	}
	return 0
}

// stringPropRaw preserves whitespace-only deltas (e.g. inter-token spaces).
func stringPropRaw(props map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := props[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func stringSliceProp(props map[string]any, key string) []string {
	raw, ok := props[key].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range raw {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func toolCallID(props map[string]any) string {
	if tool, ok := props["tool"].(map[string]any); ok {
		if id, _ := tool["callID"].(string); id != "" {
			return id
		}
	}
	if id, _ := props["callID"].(string); id != "" {
		return id
	}
	return ""
}

func toolLabelFromProps(props map[string]any) string {
	name := stringProp(props, "tool", "name")
	if name == "" {
		if tool, ok := props["tool"].(map[string]any); ok {
			name = stringProp(tool, "name", "type")
		}
	}
	return toolLabelByName(name)
}

func toolLabel(props map[string]any, phase string) string {
	name := stringProp(props, "tool", "name")
	if name == "" {
		if tool, ok := props["tool"].(map[string]any); ok {
			name = stringProp(tool, "name", "type", "permission")
		}
	}
	label := toolLabelByName(name)
	if phase == "after" && label == name {
		return name
	}
	return label
}

func toolLabelByName(name string) string {
	if name == "" {
		name = "tool"
	}
	switch strings.ToLower(name) {
	case "read", "read_file", "file_read":
		return "reading file"
	case "write", "edit", "file_edit", "edit_file":
		return "editing file"
	case "bash", "shell", "command":
		return "executing command"
	case "test", "run_test":
		return "running tests"
	default:
		return name
	}
}

func diffSummary(props map[string]any) string {
	if diff, ok := props["diff"].(string); ok && diff != "" {
		return "session diff: " + truncate(diff, 120)
	}
	return "session diff"
}

func activitySummary(evtType string, props map[string]any) string {
	if path := stringProp(props, "path", "file"); path != "" {
		return evtType + ": " + path
	}
	if msg := stringProp(props, "message", "text"); msg != "" {
		return evtType + ": " + truncate(msg, 120)
	}
	raw, err := json.Marshal(props)
	if err != nil {
		return evtType
	}
	return evtType + ": " + truncate(string(raw), 120)
}
