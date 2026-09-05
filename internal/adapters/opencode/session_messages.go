package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

type sessionMessage struct {
	Info  map[string]any `json:"info"`
	Parts []part         `json:"parts"`
}

type storedMessage struct {
	Info  map[string]any   `json:"info"`
	Parts []map[string]any `json:"parts"`
}

func (a *Adapter) fetchSessionMessages(ctx context.Context, sessionID, projectDir string) ([]storedMessage, error) {
	a.mu.Lock()
	base := a.baseURL
	a.mu.Unlock()
	if base == "" {
		return nil, fmt.Errorf("opencode serve not running")
	}
	path := withDirectoryQuery("/session/"+sessionID+"/message", projectDir, a.ProjectDir)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return nil, err
	}
	client := a.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if err := httpOK(resp); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	var messages []storedMessage
	if err := json.Unmarshal(body, &messages); err != nil {
		return nil, fmt.Errorf("decode session messages: %w", err)
	}
	return messages, nil
}

func textFromStoredParts(parts []map[string]any) string {
	var b strings.Builder
	for _, p := range parts {
		typ, _ := p["type"].(string)
		if !strings.EqualFold(typ, "text") {
			continue
		}
		text, _ := p["text"].(string)
		b.WriteString(text)
	}
	return b.String()
}

// fetchLatestAssistantOutput reads GET /session/{id}/message when SSE missed
// session.idle (e.g. subscribe-after-prompt race on fast completions).
func (a *Adapter) fetchLatestAssistantOutput(ctx context.Context, sessionID, projectDir string) (text string, usage harness.Usage, ok bool, err error) {
	messages, err := a.fetchSessionMessages(ctx, sessionID, projectDir)
	if err != nil {
		return "", harness.Usage{}, false, err
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		role, _ := msg.Info["role"].(string)
		if !strings.EqualFold(role, "assistant") {
			continue
		}
		out := textFromStoredParts(msg.Parts)
		if strings.TrimSpace(out) == "" {
			continue
		}
		return out, extractOpenCodeUsage(msg.Info), true, nil
	}
	return "", harness.Usage{}, false, nil
}

func (a *Adapter) tryRecoverCompletedSession(
	ctx context.Context,
	sessionID, projectDir string,
	req harness.ExecuteRequest,
	state *streamState,
	buf *strings.Builder,
) bool {
	text, usage, ok, err := a.fetchLatestAssistantOutput(ctx, sessionID, projectDir)
	if err != nil {
		a.log().Warn("opencode session message recovery failed", "error", err, "sessionID", sessionID)
		return false
	}
	if !ok || strings.TrimSpace(text) == "" {
		return false
	}
	if state != nil && !state.stepUsageSeen && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
		state.usage = usage
	}
	existing := ""
	if buf != nil {
		existing = buf.String()
	}
	var suffix string
	switch {
	case existing == "":
		suffix = text
	case strings.HasPrefix(text, existing):
		suffix = text[len(existing):]
	case !strings.Contains(existing, text):
		suffix = text
	}
	if suffix != "" && buf != nil {
		buf.WriteString(suffix)
	}
	if suffix != "" && req.OnStreamDelta != nil {
		req.OnStreamDelta(harness.StreamDelta{
			Kind:        harness.StreamKindText,
			Text:        suffix,
			HarnessType: "session.message.recovery",
			SessionID:   sessionID,
		})
	}
	a.log().Info("opencode recovered assistant output from session messages", "sessionID", sessionID)
	return true
}
