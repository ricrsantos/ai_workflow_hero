package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

type globalHealthResponse struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version"`
}

// CheckHealth implements harness.HealthChecker for the OpenCode serve harness.
func (a *Adapter) CheckHealth(ctx context.Context, sessionID string) (harness.HarnessHealth, error) {
	a.mu.Lock()
	pid := a.servePID
	baseURL := a.baseURL
	a.mu.Unlock()

	health := harness.HarnessHealth{
		ProcessAlive: processAlive(pid),
		ServerAlive:  false,
		SessionAlive: true,
		Details:      "opencode serve",
	}
	if baseURL == "" {
		health.ProcessAlive = false
		health.ServerAlive = false
		health.Details = "opencode serve not running"
		return health, nil
	}
	if !health.ProcessAlive && ServeURLAlive(ctx, baseURL) {
		health.ProcessAlive = true
	}

	serverOK, details := checkGlobalHealth(ctx, baseURL, a.HTTP)
	health.ServerAlive = serverOK
	if details != "" {
		health.Details = details
	}
	if !serverOK {
		health.ServerAlive = false
		if health.ProcessAlive {
			health.Details = "opencode serve process alive but health endpoint unreachable"
		}
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID != "" {
		if err := a.ResumeSession(ctx, sessionID); err != nil {
			health.SessionAlive = false
			if health.Details != "" {
				health.Details += "; "
			}
			health.Details += err.Error()
		}
	}
	return health, nil
}

func checkGlobalHealth(ctx context.Context, baseURL string, client HTTPDoer) (bool, string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false, ""
	}
	if client == nil {
		client = http.DefaultClient
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/global/health", nil)
	if err != nil {
		return ServeURLAlive(reqCtx, baseURL), ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ServeURLAlive(reqCtx, baseURL), ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ServeURLAlive(reqCtx, baseURL), fmt.Sprintf("health status %d", resp.StatusCode)
	}
	var body globalHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return true, "health endpoint responded"
	}
	if !body.Healthy {
		return false, "server reported unhealthy"
	}
	details := "healthy"
	if v := strings.TrimSpace(body.Version); v != "" {
		details = "healthy " + v
	}
	return true, details
}

func processAlive(pid int) bool {
	if pid <= 0 || processZombie(pid) {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
