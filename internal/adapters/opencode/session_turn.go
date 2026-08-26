package opencode

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type sessionTurn string

const (
	sessionTurnUnknown sessionTurn = ""
	sessionTurnIdle    sessionTurn = "idle"
	sessionTurnBusy    sessionTurn = "busy"
	sessionTurnGone    sessionTurn = "gone"
)

func (a *Adapter) probeSessionTurn(ctx context.Context, sessionID, projectDir string) sessionTurn {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return sessionTurnUnknown
	}
	a.mu.Lock()
	base := a.baseURL
	a.mu.Unlock()
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return sessionTurnUnknown
	}
	client := a.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	path := withDirectoryQuery("/session/"+sessionID, projectDir, a.ProjectDir)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return sessionTurnUnknown
	}
	resp, err := client.Do(req)
	if err != nil {
		return sessionTurnUnknown
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return sessionTurnGone
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return sessionTurnUnknown
	default:
		return parseSessionTurn(body)
	}
}

func parseSessionTurn(body []byte) sessionTurn {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return sessionTurnUnknown
	}
	switch strings.ToLower(statusType(raw)) {
	case "idle", "completed", "complete", "done":
		return sessionTurnIdle
	case "busy", "running", "retry", "processing", "busy_retry":
		return sessionTurnBusy
	}
	return sessionTurnUnknown
}

func statusType(raw map[string]any) string {
	if s, ok := raw["status"].(string); ok {
		return s
	}
	if st, ok := raw["status"].(map[string]any); ok {
		if t, ok := st["type"].(string); ok {
			return t
		}
	}
	if t, ok := raw["time"].(map[string]any); ok {
		if _, busy := t["busy"]; busy {
			return "busy"
		}
		if _, completed := t["completed"]; completed {
			return "idle"
		}
	}
	return ""
}
