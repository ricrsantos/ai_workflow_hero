package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

type streamOutcome struct {
	done bool
	err  error
}

// turnStreamState tracks text already streamed for a single Execute turn so
// item/completed can emit only the authoritative suffix (OpenCode pattern).
type turnStreamState struct {
	emittedText       map[string]string
	authoritativeText map[string]string
	agentTextOrder    []string
	lastAgentKey      string
}

func newTurnStreamState() *turnStreamState {
	return &turnStreamState{
		emittedText:       make(map[string]string),
		authoritativeText: make(map[string]string),
	}
}

func (st *turnStreamState) noteTextDelta(key, delta string) {
	if st == nil || delta == "" {
		return
	}
	st.noteAgentTextKey(key)
	st.emittedText[key] += delta
	st.lastAgentKey = key
}

// noteAgentTextKey preserves app-server agent-message order. A completed item
// is authoritative for its own key; this order lets Execute rebuild a complete
// answer even if live deltas lost a span in the middle of the message.
func (st *turnStreamState) noteAgentTextKey(key string) {
	if st == nil || key == "" {
		return
	}
	if st.emittedText == nil {
		st.emittedText = make(map[string]string)
	}
	if st.authoritativeText == nil {
		st.authoritativeText = make(map[string]string)
	}
	if _, exists := st.emittedText[key]; !exists {
		st.agentTextOrder = append(st.agentTextOrder, key)
		st.emittedText[key] = ""
	}
}

func (st *turnStreamState) noteAuthoritativeText(key, text string) {
	if st == nil || key == "" || text == "" {
		return
	}
	st.noteAgentTextKey(key)
	st.authoritativeText[key] = text
}

func (st *turnStreamState) onlyAgentTextKey() (string, bool) {
	if st == nil || len(st.agentTextOrder) != 1 {
		return "", false
	}
	return st.agentTextOrder[0], true
}

// output returns the final agent-message snapshots in wire order. fallback
// retains output from non-agent text events such as plan-only turns.
func (st *turnStreamState) output(fallback string) string {
	if st == nil || len(st.agentTextOrder) == 0 {
		return fallback
	}

	var out strings.Builder
	lastEndedNewline := false
	for _, key := range st.agentTextOrder {
		text := st.authoritativeText[key]
		if text == "" {
			text = st.emittedText[key]
		}
		if text == "" {
			continue
		}
		if out.Len() > 0 && !lastEndedNewline && !strings.HasPrefix(text, "\n") {
			out.WriteByte('\n')
		}
		out.WriteString(text)
		lastEndedNewline = strings.HasSuffix(text, "\n")
	}
	if out.Len() == 0 {
		return fallback
	}
	return out.String()
}

// ensureItemSeparator inserts a newline when Codex starts a new agentMessage
// item. Deltas are concatenated per itemId; successive items have no trailing
// newline, so the TUI would otherwise glue "....→ next status" onto one line.
func (st *turnStreamState) ensureItemSeparator(key, next, harnessType, sessionID string, emit func(harness.StreamDelta)) {
	if st == nil || key == "" || next == "" {
		return
	}
	if st.lastAgentKey == "" || st.lastAgentKey == key {
		return
	}
	prev := ""
	if st.emittedText != nil {
		prev = st.emittedText[st.lastAgentKey]
	}
	if prev == "" || strings.HasSuffix(prev, "\n") || strings.HasPrefix(next, "\n") {
		return
	}
	emit(harness.StreamDelta{
		Kind:        harness.StreamKindText,
		Text:        "\n",
		HarnessType: harnessType,
		SessionID:   sessionID,
	})
}

func (st *turnStreamState) hasAnyAgentText() bool {
	if st == nil || st.emittedText == nil {
		return false
	}
	for _, v := range st.emittedText {
		if v != "" {
			return true
		}
	}
	return false
}

// canRepairFromLastAgentMessage is true when turn/completed's lastAgentMessage
// extends (or is a prefix of) streamed text. A divergent summary is usually
// only the last item — appending it glues a duplicate paragraph onto the
// transcript.
func (st *turnStreamState) canRepairFromLastAgentMessage(key, full string) bool {
	if strings.TrimSpace(full) == "" {
		return false
	}
	if st == nil || st.emittedText == nil {
		return true
	}
	emitted := st.emittedText[key]
	if emitted == "" {
		return !st.hasAnyAgentText()
	}
	return full == emitted || strings.HasPrefix(full, emitted) || strings.HasPrefix(emitted, full)
}

func (st *turnStreamState) emitAuthoritativeText(key, full, harnessType, sessionID string, emit func(harness.StreamDelta)) {
	if full == "" {
		return
	}
	if st == nil {
		emit(harness.StreamDelta{
			Kind:        harness.StreamKindText,
			Text:        full,
			HarnessType: harnessType,
			SessionID:   sessionID,
			Phase:       harness.StreamPhaseCompleted,
		})
		return
	}
	if st.emittedText == nil {
		st.emittedText = make(map[string]string)
	}
	st.noteAuthoritativeText(key, full)
	emitted := st.emittedText[key]
	if full == emitted {
		st.lastAgentKey = key
		return
	}
	text := full
	switch {
	case strings.HasPrefix(full, emitted):
		text = full[len(emitted):]
		if text == "" {
			st.lastAgentKey = key
			return
		}
	case strings.HasPrefix(emitted, full):
		st.lastAgentKey = key
		return
	default:
		// A completed item can repair a dropped span in the middle of a live
		// stream. It cannot be appended safely: that would duplicate the whole
		// message. Keep the final snapshot for ExecutionResult.Output instead.
		st.lastAgentKey = key
		return
	}
	st.ensureItemSeparator(key, text, harnessType, sessionID, emit)
	emit(harness.StreamDelta{
		Kind:        harness.StreamKindText,
		Text:        text,
		HarnessType: harnessType,
		SessionID:   sessionID,
		Phase:       harness.StreamPhaseCompleted,
	})
	st.emittedText[key] = full
	st.lastAgentKey = key
}

func agentTextKey(sessionID, itemID string) string {
	if itemID == "" {
		return sessionID + ":agent"
	}
	return sessionID + ":agent:" + itemID
}

// resolveAgentTextKey prefers the item id key; falls back when deltas lacked itemId.
func resolveAgentTextKey(st *turnStreamState, sessionID, itemID string) string {
	key := agentTextKey(sessionID, itemID)
	if st == nil || st.emittedText == nil || itemID == "" {
		return key
	}
	if st.emittedText[key] != "" {
		return key
	}
	fallback := agentTextKey(sessionID, "")
	if st.emittedText[fallback] != "" {
		return fallback
	}
	return key
}

// resolveRepairTextKey picks the stream key whose emitted prefix matches full,
// so turn/completed summaries repair without duplicating prior deltas.
func (st *turnStreamState) resolveRepairTextKey(sessionID, full string) string {
	fallback := agentTextKey(sessionID, "")
	if st == nil || st.emittedText == nil || full == "" {
		return fallback
	}
	prefix := sessionID + ":agent"
	var bestKey string
	bestLen := -1
	for key, emitted := range st.emittedText {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if emitted == "" {
			continue
		}
		if full == emitted || strings.HasPrefix(full, emitted) {
			if len(emitted) > bestLen {
				bestKey = key
				bestLen = len(emitted)
			}
		}
	}
	if bestKey != "" {
		return bestKey
	}
	return fallback
}

// handleNotification maps Codex app-server notifications to StreamDelta.
func (a *Adapter) handleNotification(ctx context.Context, method string, raw json.RawMessage, sessionID string, req harness.ExecuteRequest, textBuf *strings.Builder, st *turnStreamState) streamOutcome {
	_ = ctx
	emit := func(d harness.StreamDelta) {
		if d.SessionID == "" {
			d.SessionID = sessionID
		}
		// Buffer text before notifying the TUI so ExecutionResult.Output stays
		// complete even when OnStreamDelta blocks or the consumer lags.
		if d.Kind == harness.StreamKindText && d.Text != "" && textBuf != nil {
			textBuf.WriteString(d.Text)
		}
		if req.OnStreamDelta != nil {
			req.OnStreamDelta(d)
		}
	}

	var params map[string]any
	_ = json.Unmarshal(raw, &params)
	threadID := stringField(params, "threadId")
	if threadID == "" {
		if turn, ok := params["turn"].(map[string]any); ok {
			threadID = stringField(turn, "threadId")
		}
	}
	if threadID != "" && threadID != sessionID {
		return streamOutcome{}
	}

	switch method {
	case "turn/started":
		if turn, ok := params["turn"].(map[string]any); ok {
			if id := stringField(turn, "id"); id != "" {
				a.mu.Lock()
				a.activeTurn[sessionID] = id
				a.mu.Unlock()
			}
		}
		emit(harness.SessionDelta(harness.SessionStateRunning, "turn started", method, sessionID))
		return streamOutcome{}

	case "turn/completed":
		status := ""
		var turnErr error
		var turn map[string]any
		if t, ok := params["turn"].(map[string]any); ok {
			turn = t
			status = strings.ToLower(stringField(turn, "status"))
			if errObj, ok := turn["error"].(map[string]any); ok {
				msg := stringField(errObj, "message")
				if msg != "" {
					turnErr = fmt.Errorf("%s", msg)
					if isAuthMessage(msg) {
						turnErr = &AuthError{Err: turnErr}
					}
				}
			}
		}
		// Newer Codex builds may include the final agent message on completion
		// when live deltas were dropped under app-server backpressure.
		if status == "" || status == "completed" || status == "success" {
			if msg := lastAgentMessageFromTurn(turn, params); msg != "" {
				if key, only := st.onlyAgentTextKey(); only {
					// With one agent item the completion snapshot is unambiguous,
					// including when a live delta lost bytes in the middle.
					st.emitAuthoritativeText(key, msg, method, sessionID, emit)
				} else {
					key := st.resolveRepairTextKey(sessionID, msg)
					if st.canRepairFromLastAgentMessage(key, msg) {
						st.emitAuthoritativeText(key, msg, method, sessionID, emit)
					}
				}
			}
		}
		switch status {
		case "interrupted":
			emit(harness.SessionDelta(harness.SessionStateIdle, "turn interrupted", method, sessionID))
			return streamOutcome{done: true}
		case "failed":
			if turnErr == nil {
				turnErr = fmt.Errorf("codex turn failed")
			}
			emit(harness.SessionDelta(harness.SessionStateFailed, turnErr.Error(), method, sessionID))
			return streamOutcome{done: true, err: turnErr}
		default:
			emit(harness.SessionDelta(harness.SessionStateIdle, "turn completed", method, sessionID))
			return streamOutcome{done: true}
		}

	case "item/agentMessage/delta":
		delta := stringFieldRaw(params, "delta", "text")
		if delta != "" {
			key := agentTextKey(sessionID, stringField(params, "itemId"))
			st.ensureItemSeparator(key, delta, method, sessionID, emit)
			st.noteTextDelta(key, delta)
			emit(harness.StreamDelta{Kind: harness.StreamKindText, Text: delta, HarnessType: method, SessionID: sessionID})
		}
		return streamOutcome{}

	case "item/plan/delta":
		delta := stringFieldRaw(params, "delta", "text")
		if delta != "" {
			emit(harness.StreamDelta{Kind: harness.StreamKindText, Text: delta, HarnessType: method, SessionID: sessionID})
		}
		return streamOutcome{}

	case "item/reasoning/summaryTextDelta", "item/reasoning/textDelta":
		// Buffer-only: live reasoning deltas often omit inter-token spaces.
		// Emit StreamKindThinking once from item/completed (OpenCode pattern).
		return streamOutcome{}

	case "item/reasoning/summaryPartAdded":
		if req.Debug {
			emit(harness.ActivityDelta(method, "reasoning summary section", sessionID))
		}
		return streamOutcome{}

	case "item/commandExecution/outputDelta":
		delta := stringFieldRaw(params, "delta", "text")
		if delta != "" {
			emit(harness.StreamDelta{Kind: harness.StreamKindTool, Text: delta, HarnessType: method, SessionID: sessionID})
		}
		return streamOutcome{}

	case "item/started", "item/completed":
		return a.handleItemLifecycle(method, params, sessionID, req, st, emit)

	case "turn/plan/updated":
		emit(harness.ActivityDelta(method, "plan updated", sessionID))
		return streamOutcome{}

	case "turn/diff/updated":
		emit(harness.ActivityDelta(method, "diff updated", sessionID))
		return streamOutcome{}

	case "thread/started", "thread/status/changed", "thread/archived", "thread/unarchived", "thread/closed", "thread/deleted", "thread/name/updated":
		emit(harness.SessionDelta(harness.SessionStateIdle, method, method, sessionID))
		return streamOutcome{}

	case "thread/tokenUsage/updated":
		a.mapTokenUsage(params, sessionID, req.Debug, emit)
		return streamOutcome{}

	case "account/rateLimits/updated":
		a.mu.Lock()
		if a.usageUSDUnsetBySession == nil {
			a.usageUSDUnsetBySession = make(map[string]bool)
		}
		a.usageUSDUnsetBySession[sessionID] = true
		a.mu.Unlock()
		if req.Debug {
			emit(harness.ActivityDelta(method, "rate limits updated", sessionID))
		}
		return streamOutcome{}

	case "warning", "configWarning":
		msg := stringField(params, "message", "summary")
		if msg == "" {
			msg = "codex warning"
		}
		emit(harness.StreamDelta{Kind: harness.StreamKindWarning, Text: msg, HarnessType: method, SessionID: sessionID})
		return streamOutcome{}

	case "error":
		msg := ""
		if errObj, ok := params["error"].(map[string]any); ok {
			msg = stringField(errObj, "message")
		}
		if msg == "" {
			msg = stringField(params, "message")
		}
		if msg == "" {
			msg = "codex error"
		}
		emit(harness.StreamDelta{Kind: harness.StreamKindWarning, Text: msg, HarnessType: method, SessionID: sessionID})
		if isAuthMessage(msg) {
			return streamOutcome{done: true, err: &AuthError{Err: fmt.Errorf("%s", msg)}}
		}
		return streamOutcome{}

	case "serverRequest/resolved", "account/updated", "account/login/completed", "skills/changed":
		if req.Debug {
			emit(harness.ActivityDelta(method, method, sessionID))
		}
		return streamOutcome{}

	default:
		// UI-C06-001 §5 / design D11: yellow warning only in --debug; never dump raw JSON-RPC.
		a.log().Debug("unrecognized codex app-server event", "method", method, "payload_bytes", len(raw))
		if req.Debug {
			emit(harness.StreamDelta{
				Kind:        harness.StreamKindWarning,
				Text:        fmt.Sprintf("WARNING: unrecognized Codex app-server event %q", method),
				HarnessType: method,
				SessionID:   sessionID,
			})
		}
		return streamOutcome{}
	}
}

func (a *Adapter) handleItemLifecycle(method string, params map[string]any, sessionID string, req harness.ExecuteRequest, st *turnStreamState, emit func(harness.StreamDelta)) streamOutcome {
	item, _ := params["item"].(map[string]any)
	if item == nil {
		if req.Debug {
			emit(harness.ActivityDelta(method, method, sessionID))
		}
		return streamOutcome{}
	}
	itemType := stringField(item, "type")
	switch itemType {
	case "agentMessage":
		if method == "item/completed" {
			text := stringFieldRaw(item, "text")
			if text == "" {
				text = stringField(item, "text")
			}
			key := resolveAgentTextKey(st, sessionID, stringField(item, "id"))
			st.emitAuthoritativeText(key, text, method, sessionID, emit)
		} else if req.Debug {
			emit(harness.ActivityDelta(method, "agent message", sessionID))
		}
	case "reasoning":
		// Thinking only from the completed snapshot (spaces preserved).
		if method != "item/completed" {
			return streamOutcome{}
		}
		text := firstNonEmpty(stringFieldRaw(item, "summary"), stringFieldRaw(item, "content"),
			stringField(item, "summary"), stringField(item, "content"))
		if text != "" {
			emit(harness.StreamDelta{
				Kind:        harness.StreamKindThinking,
				Text:        text,
				HarnessType: method,
				SessionID:   sessionID,
			})
		}
	case "commandExecution", "mcpToolCall", "dynamicToolCall", "webSearch":
		summary := itemType
		if cmd := stringField(item, "command", "tool", "query"); cmd != "" {
			summary = itemType + ": " + cmd
		}
		emit(harness.StreamDelta{Kind: harness.StreamKindTool, Text: summary, HarnessType: method, SessionID: sessionID})
	case "collabToolCall":
		agentName := firstNonEmpty(
			harness.HeroAgentFromLabel(stringField(item, "agent", "name", "title", "description")),
			stringField(item, "agent", "name", "title"),
			"task",
		)
		if harness.HeroAgentFromLabel(agentName) != "" {
			agentName = harness.HeroAgentFromLabel(agentName)
		}
		model := stringField(item, "model")
		callID := firstNonEmpty(stringField(item, "id", "itemId", "callId"), stringField(params, "itemId"))
		if callID == "" {
			callID = "collab:" + agentName
		}
		phase := harness.StreamPhaseStarted
		label := "Task " + agentName
		if method == "item/completed" {
			phase = harness.StreamPhaseCompleted
			label += " (completed)"
		}
		emit(harness.StreamDelta{
			Kind:        harness.StreamKindTool,
			Text:        label,
			AgentName:   agentName,
			Model:       model,
			CallID:      callID,
			Phase:       phase,
			HarnessType: method,
			SessionID:   sessionID,
		})
	case "fileChange":
		emit(harness.ActivityDelta(method, "file change", sessionID))
	case "plan":
		if text := stringFieldRaw(item, "text"); text != "" {
			emit(harness.StreamDelta{Kind: harness.StreamKindText, Text: text, HarnessType: method, SessionID: sessionID})
		} else if text := stringField(item, "text"); text != "" {
			emit(harness.StreamDelta{Kind: harness.StreamKindText, Text: text, HarnessType: method, SessionID: sessionID})
		} else {
			emit(harness.ActivityDelta(method, "plan", sessionID))
		}
	case "contextCompaction", "enteredReviewMode", "exitedReviewMode", "imageView", "userMessage":
		if req.Debug {
			emit(harness.ActivityDelta(method, itemType, sessionID))
		}
	default:
		if itemType == "" {
			emit(harness.WarningDelta(adapterName, method, sessionID, fmt.Sprintf("%v", item)))
		} else if req.Debug {
			emit(harness.ActivityDelta(method, itemType, sessionID))
		}
	}
	return streamOutcome{}
}

func (a *Adapter) mapTokenUsage(params map[string]any, sessionID string, debug bool, emit func(harness.StreamDelta)) {
	usage := harness.Usage{}
	hasUSD := false
	extract := func(m map[string]any) {
		if m == nil {
			return
		}
		if in := int64Field(m, "inputTokens", "input_tokens", "promptTokens"); in > 0 {
			usage.InputTokens = in
		}
		if out := int64Field(m, "outputTokens", "output_tokens", "completionTokens"); out > 0 {
			usage.OutputTokens = out
		}
		if total := int64Field(m, "totalTokens", "tokensUsed", "total"); total > 0 && usage.InputTokens == 0 && usage.OutputTokens == 0 {
			usage.InputTokens = total
		}
		for _, k := range []string{"usd", "costUsd", "cost_usd", "amountUsd", "price"} {
			if _, ok := m[k]; ok {
				hasUSD = true
			}
		}
	}
	extractContainer := func(m map[string]any) {
		if m == nil {
			return
		}
		// Codex app-server v2 sends a cumulative snapshot with `last` for
		// the current turn and `total` for the whole thread. Execute reports
		// one turn, so never add the cumulative `total` to the TUI counter.
		if last, ok := m["last"].(map[string]any); ok {
			extract(last)
			return
		}
		extract(m)
	}
	if u, ok := params["usage"].(map[string]any); ok {
		extractContainer(u)
	}
	if u, ok := params["tokenUsage"].(map[string]any); ok {
		extractContainer(u)
	}
	// Only read top-level token fields when nested usage objects were absent.
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		extract(params)
	} else {
		for _, k := range []string{"usd", "costUsd", "cost_usd", "amountUsd", "price"} {
			if _, ok := params[k]; ok {
				hasUSD = true
			}
		}
	}

	a.mu.Lock()
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if a.usageBySession == nil {
			a.usageBySession = make(map[string]harness.Usage)
		}
		a.usageBySession[sessionID] = usage
	}
	if !hasUSD {
		if a.usageUSDUnsetBySession == nil {
			a.usageUSDUnsetBySession = make(map[string]bool)
		}
		a.usageUSDUnsetBySession[sessionID] = true
	}
	a.mu.Unlock()

	if debug {
		emit(harness.ActivityDelta("thread/tokenUsage/updated",
			fmt.Sprintf("tokens in=%d out=%d", usage.InputTokens, usage.OutputTokens), sessionID))
	}
}

// handleServerRequest answers Codex approval prompts via OnPermissionRequest.
func (a *Adapter) handleServerRequest(ctx context.Context, id json.RawMessage, method string, raw json.RawMessage, sessionID string, req harness.ExecuteRequest) {
	a.mu.Lock()
	rpc := a.rpc
	a.mu.Unlock()
	if rpc == nil {
		return
	}

	var params map[string]any
	_ = json.Unmarshal(raw, &params)

	switch method {
	case "item/commandExecution/requestApproval",
		"item/fileChange/requestApproval",
		"item/permissions/requestApproval":
		a.replyApproval(ctx, rpc, id, method, params, sessionID, req)
	case "tool/requestUserInput":
		// Treat as a permission-style gate: allow → accept first option path via decline/cancel if no handler.
		a.replyApproval(ctx, rpc, id, method, params, sessionID, req)
	default:
		if req.OnStreamDelta != nil {
			req.OnStreamDelta(harness.WarningDelta(adapterName, method, sessionID, string(raw)))
		}
		_ = rpc.Respond(id, map[string]any{"decision": "decline"})
	}
}

func (a *Adapter) replyApproval(ctx context.Context, rpc *rpcConn, id json.RawMessage, method string, params map[string]any, sessionID string, req harness.ExecuteRequest) {
	title := method
	desc := stringField(params, "reason", "command")
	if cwd := stringField(params, "cwd"); cwd != "" && desc != "" {
		desc = desc + "\n" + cwd
	} else if cwd := stringField(params, "cwd"); cwd != "" {
		desc = cwd
	}
	permID := stringField(params, "itemId", "requestId")
	if permID == "" {
		permID = string(id)
	}

	// workspaceWrite+writableRoots is attached in turnStartParams. It makes a
	// file-change approval project-scoped, while commands, MCP permissions, and
	// any network-capable operation still reach the TUI for a human decision.
	if harness.NormalizePermissionProfile(req.PermissionProfile) == harness.PermissionProfileAutoProject && method == "item/fileChange/requestApproval" {
		_ = rpc.Respond(id, approvalDecision(method, true))
		return
	}
	if req.OnStreamDelta != nil {
		req.OnStreamDelta(harness.StreamDelta{
			Kind:        harness.StreamKindPermission,
			Text:        "Allow? [y/N]",
			HarnessType: method,
			SessionID:   sessionID,
			Metadata:    map[string]string{"permission_id": permID},
		})
	}

	approved := false
	if req.OnPermissionRequest != nil {
		resp, err := req.OnPermissionRequest(ctx, harness.PermissionRequest{
			ID:          permID,
			Title:       title,
			Description: desc,
			HarnessType: method,
			SessionID:   sessionID,
		})
		if err != nil {
			a.log().Error("codex permission handler failed", "error", err, "method", method)
			_ = rpc.Respond(id, approvalDecision(method, false))
			return
		}
		approved = resp.Approved
	} else {
		if req.OnStreamDelta != nil {
			req.OnStreamDelta(harness.StreamDelta{
				Kind:        harness.StreamKindWarning,
				Text:        fmt.Sprintf("codex permission required (%s) but no OnPermissionRequest handler", title),
				HarnessType: method,
				SessionID:   sessionID,
			})
		}
	}
	_ = rpc.Respond(id, approvalDecision(method, approved))
}

func approvalDecision(method string, approved bool) any {
	if method == "item/permissions/requestApproval" {
		if approved {
			return map[string]any{"permissions": []any{}, "scope": "turn"}
		}
		return map[string]any{"permissions": []any{}}
	}
	if approved {
		return map[string]any{"decision": "accept"}
	}
	return map[string]any{"decision": "decline"}
}

// lastAgentMessageFromTurn extracts a completion-summary agent message when
// Codex includes it on turn/completed (repairs dropped live deltas).
func lastAgentMessageFromTurn(turn, params map[string]any) string {
	candidates := []map[string]any{turn, params}
	for _, m := range candidates {
		if m == nil {
			continue
		}
		if s := stringFieldRaw(m, "lastAgentMessage", "last_agent_message"); s != "" {
			return s
		}
		if summary, ok := m["summary"].(map[string]any); ok {
			if s := stringFieldRaw(summary, "lastAgentMessage", "last_agent_message", "text"); s != "" {
				return s
			}
		}
		if items, ok := m["items"].([]any); ok {
			for i := len(items) - 1; i >= 0; i-- {
				item, _ := items[i].(map[string]any)
				if item == nil {
					continue
				}
				if stringField(item, "type") != "agentMessage" {
					continue
				}
				if s := stringFieldRaw(item, "text"); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func stringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					return s
				}
			case fmt.Stringer:
				if s := strings.TrimSpace(t.String()); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// stringFieldRaw preserves whitespace-only deltas (e.g. inter-token spaces).
func stringFieldRaw(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if t != "" {
					return t
				}
			case fmt.Stringer:
				if s := t.String(); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func int64Field(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return int64(t)
			case int64:
				return t
			case int:
				return int64(t)
			case json.Number:
				n, _ := t.Int64()
				return n
			}
		}
	}
	return 0
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
