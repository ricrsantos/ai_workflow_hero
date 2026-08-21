// Package codex implements HarnessAdapter via Hero-managed `codex app-server`
// over stdio JSON-RPC (ADR-044, ADR-045; PRD-C06-001 §4.2–4.4).
package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const adapterName = "codex"

// ClientIdentity identifies Hero to the Codex app-server (Compliance Logs).
const (
	ClientName    = "hero_tui"
	ClientTitle   = "AI Workflow Hero"
	ClientVersion = "2.5.0"
)

// LookPathFunc resolves CLI binaries (tests inject).
type LookPathFunc func(string) (string, error)

// ProcessRunner starts and stops child processes with stdio pipes (design D3).
type ProcessRunner interface {
	Start(ctx context.Context, dir, name string, args ...string) (StdioHandle, error)
}

// StdioHandle is a started child process with stdin/stdout for JSON-RPC.
type StdioHandle interface {
	PID() int
	Stdin() WriteCloser
	Stdout() ReadCloser
	Wait() error
	Kill() error
}

// WriteCloser / ReadCloser aliases keep the injectable surface small.
type WriteCloser interface {
	Write([]byte) (int, error)
	Close() error
}

type ReadCloser interface {
	Read([]byte) (int, error)
	Close() error
}

// Adapter implements harness.HarnessAdapter via codex app-server (ADR-044).
type Adapter struct {
	ProjectDir string
	Store      *store.Store
	Logger     *slog.Logger
	LookPath   LookPathFunc
	Runner     ProcessRunner
	// ClientInfo overrides initialize clientInfo (tests inject).
	ClientInfo map[string]string

	mu            sync.Mutex
	startMu       sync.Mutex
	appPID        int
	appHandle     StdioHandle
	rpc           *rpcConn
	sessions      map[string]*sessionState
	cancels       map[string]context.CancelFunc
	activeTurn    map[string]string // threadID → turnID
	lastUsage     harness.Usage
	usageUSDUnset bool
}

type sessionState struct {
	session harness.Session
	status  harness.ExecutionStatus
}

// Compile-time contract checks (task 4.1).
var (
	_ harness.HarnessAdapter = (*Adapter)(nil)
	_ harness.ModelLister    = (*Adapter)(nil)
)

// NewAdapter returns a Codex harness adapter for projectDir.
func NewAdapter(projectDir string, st *store.Store) *Adapter {
	return &Adapter{
		ProjectDir: projectDir,
		Store:      st,
		Logger:     slog.Default(),
		LookPath:   exec.LookPath,
		Runner:     ExecRunner{},
		sessions:   make(map[string]*sessionState),
		cancels:    make(map[string]context.CancelFunc),
		activeTurn: make(map[string]string),
	}
}

func (a *Adapter) log() *slog.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return slog.Default()
}

// Name implements harness.HarnessAdapter.
func (a *Adapter) Name() string { return adapterName }

// IsAvailable implements harness.HarnessAdapter.
func (a *Adapter) IsAvailable(ctx context.Context) error {
	if _, err := a.cliPath(); err != nil {
		return fmt.Errorf("codex CLI not on PATH: %w", err)
	}
	return nil
}

func (a *Adapter) cliPath() (string, error) {
	look := a.LookPath
	if look == nil {
		look = exec.LookPath
	}
	return look("codex")
}

// CreateSession implements harness.HarnessAdapter (Codex thread).
func (a *Adapter) CreateSession(ctx context.Context, req harness.SessionRequest) (*harness.Session, error) {
	if err := a.ensureAppServer(ctx); err != nil {
		return nil, err
	}
	params := map[string]any{}
	if cwd := strings.TrimSpace(req.ProjectDir); cwd != "" {
		params["cwd"] = cwd
	} else if a.ProjectDir != "" {
		params["cwd"] = a.ProjectDir
	}
	if name := strings.TrimSpace(req.StageName); name != "" {
		params["name"] = name
	}
	var result struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := a.rpcCall(ctx, "thread/start", params, &result); err != nil {
		return nil, mapRPCError(err)
	}
	id := strings.TrimSpace(result.Thread.ID)
	if id == "" {
		return nil, fmt.Errorf("codex thread/start: empty thread id")
	}
	out := &harness.Session{
		ID:         id,
		ProjectDir: req.ProjectDir,
		StageName:  req.StageName,
		AgentName:  req.AgentName,
		CreatedAt:  time.Now().UTC(),
	}
	a.mu.Lock()
	a.sessions[id] = &sessionState{
		session: *out,
		status:  harness.ExecutionStatus{SessionID: id, State: harness.StatusIdle},
	}
	a.mu.Unlock()
	a.log().Info("codex thread created", "thread_id", id)
	return out, nil
}

// ResumeSession implements harness.HarnessAdapter.
func (a *Adapter) ResumeSession(ctx context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}
	if err := a.ensureAppServer(ctx); err != nil {
		return err
	}
	params := map[string]any{"threadId": sessionID}
	var result struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := a.rpcCall(ctx, "thread/resume", params, &result); err != nil {
		return mapRPCError(err)
	}
	a.mu.Lock()
	if _, ok := a.sessions[sessionID]; !ok {
		a.sessions[sessionID] = &sessionState{
			session: harness.Session{ID: sessionID, CreatedAt: time.Now().UTC()},
			status:  harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusIdle},
		}
	}
	a.mu.Unlock()
	return nil
}

// Execute implements harness.HarnessAdapter.
func (a *Adapter) Execute(ctx context.Context, req harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	req = harness.NormalizeExecuteRequest(req)
	if err := a.ensureAppServer(ctx); err != nil {
		return nil, err
	}
	if err := a.requireAuth(ctx); err != nil {
		return nil, err
	}

	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sess, err := a.CreateSession(ctx, harness.SessionRequest{
			ProjectDir: req.ProjectDir,
			StageName:  req.StageName,
			AgentName:  req.AgentName,
		})
		if err != nil {
			return nil, err
		}
		sessionID = sess.ID
	} else {
		// Load thread into memory when resuming across app-server restarts.
		// Already-loaded threads may error; turn/start still proceeds.
		if err := a.ResumeSession(ctx, sessionID); err != nil {
			a.log().Debug("codex thread resume skipped", "thread_id", sessionID, "error", err)
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	a.mu.Lock()
	a.cancels[sessionID] = cancel
	a.lastUsage = harness.Usage{}
	a.usageUSDUnset = false
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.cancels, sessionID)
		a.mu.Unlock()
		cancel()
	}()

	start := time.Now()
	params := turnStartParams(sessionID, req, a.ProjectDir)
	a.setStatus(sessionID, harness.StatusRunning, "")

	var buf strings.Builder
	streamDone := make(chan streamOutcome, 1)
	a.rpc.SetHandlers(
		func(method string, raw json.RawMessage) {
			out := a.handleNotification(runCtx, method, raw, sessionID, req, &buf)
			if out.done || out.err != nil {
				select {
				case streamDone <- out:
				default:
				}
			}
		},
		func(id json.RawMessage, method string, raw json.RawMessage) {
			a.handleServerRequest(runCtx, id, method, raw, sessionID, req)
		},
	)
	defer a.rpc.SetHandlers(nil, nil)

	var turnResult struct {
		Turn struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"turn"`
	}
	if err := a.rpcCall(runCtx, "turn/start", params, &turnResult); err != nil {
		a.setStatus(sessionID, harness.StatusFailed, err.Error())
		return nil, mapPropertyRejection(err, strings.TrimSpace(req.Model), req.Properties)
	}
	turnID := strings.TrimSpace(turnResult.Turn.ID)
	if turnID != "" {
		a.mu.Lock()
		a.activeTurn[sessionID] = turnID
		a.mu.Unlock()
	}
	if req.OnStreamDelta != nil {
		req.OnStreamDelta(harness.SessionDelta(harness.SessionStateRunning, "turn started", "turn/started", sessionID))
	}

	// Some servers finish synchronously in the turn/start result.
	switch strings.ToLower(strings.TrimSpace(turnResult.Turn.Status)) {
	case "completed", "interrupted":
		// already done
	case "failed":
		errMsg := "codex turn failed"
		if turnResult.Turn.Error != nil && turnResult.Turn.Error.Message != "" {
			errMsg = turnResult.Turn.Error.Message
		}
		a.setStatus(sessionID, harness.StatusFailed, errMsg)
		return nil, mapPropertyRejection(fmt.Errorf("%s", errMsg), strings.TrimSpace(req.Model), req.Properties)
	default:
		// Wait for turn/completed (or cancel / context).
		select {
		case <-runCtx.Done():
			_ = a.interruptTurn(context.Background(), sessionID)
			a.setStatus(sessionID, harness.StatusCancelled, "cancelled")
			return nil, runCtx.Err()
		case out := <-streamDone:
			if out.err != nil {
				a.setStatus(sessionID, harness.StatusFailed, out.err.Error())
				return nil, out.err
			}
		}
	}

	a.mu.Lock()
	usage := a.lastUsage
	usdUnset := a.usageUSDUnset
	delete(a.activeTurn, sessionID)
	a.mu.Unlock()

	if usdUnset && (usage.InputTokens > 0 || usage.OutputTokens > 0) && req.OnStreamDelta != nil {
		req.OnStreamDelta(harness.StreamDelta{
			Kind:        harness.StreamKindWarning,
			Text:        "Codex usage has no USD rates; cost left unset/zero",
			HarnessType: "thread/tokenUsage/updated",
			SessionID:   sessionID,
		})
	}

	out := buf.String()
	a.setStatus(sessionID, harness.StatusCompleted, "")
	return &harness.ExecutionResult{
		SessionID:  sessionID,
		Output:     out,
		Summary:    truncate(out, 200),
		Usage:      usage,
		Duration:   time.Since(start),
		StreamDone: true,
	}, nil
}

// Cancel implements harness.HarnessAdapter.
func (a *Adapter) Cancel(ctx context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	a.mu.Lock()
	if c, ok := a.cancels[sessionID]; ok {
		c()
	}
	a.mu.Unlock()
	if sessionID == "" {
		return nil
	}
	return a.interruptTurn(ctx, sessionID)
}

func (a *Adapter) interruptTurn(ctx context.Context, sessionID string) error {
	a.mu.Lock()
	turnID := a.activeTurn[sessionID]
	rpc := a.rpc
	a.mu.Unlock()
	if rpc == nil || turnID == "" {
		return nil
	}
	params := map[string]any{
		"threadId": sessionID,
		"turnId":   turnID,
	}
	if err := a.rpcCall(ctx, "turn/interrupt", params, &struct{}{}); err != nil {
		a.log().Debug("codex turn/interrupt", "error", err)
	}
	a.setStatus(sessionID, harness.StatusCancelled, "interrupted")
	return nil
}

// Status implements harness.HarnessAdapter.
func (a *Adapter) Status(ctx context.Context, sessionID string) (*harness.ExecutionStatus, error) {
	_ = ctx
	a.mu.Lock()
	defer a.mu.Unlock()
	if st, ok := a.sessions[sessionID]; ok {
		s := st.status
		return &s, nil
	}
	return &harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusIdle}, nil
}

// Dispatch implements harness.HarnessAdapter.
func (a *Adapter) Dispatch(ctx context.Context, req harness.DispatchRequest) (harness.DispatchResult, error) {
	_, err := a.Execute(ctx, harness.ExecuteRequest{
		ProjectDir: req.ProjectDir,
		Prompt:     req.Prompt,
		Model:      req.Model,
		Mode:       req.Mode,
		Stream:     false,
	})
	if err != nil {
		return harness.DispatchResult{Message: err.Error()}, err
	}
	return harness.DispatchResult{Dispatched: true}, nil
}

// ListModels implements harness.ModelLister via model/list (native Codex ids).
func (a *Adapter) ListModels(ctx context.Context) ([]string, error) {
	if err := a.ensureAppServer(ctx); err != nil {
		return nil, err
	}
	var models []string
	cursor := ""
	for {
		params := map[string]any{
			"limit":         100,
			"includeHidden": false,
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var result struct {
			Data []struct {
				ID    string `json:"id"`
				Model string `json:"model"`
			} `json:"data"`
			NextCursor *string `json:"nextCursor"`
		}
		if err := a.rpcCall(ctx, "model/list", params, &result); err != nil {
			return nil, mapRPCError(err)
		}
		for _, m := range result.Data {
			id := strings.TrimSpace(m.ID)
			if id == "" {
				id = strings.TrimSpace(m.Model)
			}
			if id != "" {
				models = append(models, id)
			}
		}
		if result.NextCursor == nil || strings.TrimSpace(*result.NextCursor) == "" {
			break
		}
		cursor = strings.TrimSpace(*result.NextCursor)
	}
	return models, nil
}

// AppServerResetDelay is the pause between stopping and restarting codex app-server
// so .codex/agents definition changes are loaded (OpenCode ServeResetDelay analog).
const AppServerResetDelay = 2 * time.Second

var appServerResetDelay = AppServerResetDelay

// ResetAppServer stops managed app-server, waits AppServerResetDelay, and starts a
// fresh child so synced agent definitions take effect (design D9).
func (a *Adapter) ResetAppServer(ctx context.Context) error {
	if err := a.StopAppServer(ctx); err != nil {
		return err
	}
	delay := appServerResetDelay
	if delay <= 0 {
		delay = AppServerResetDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	return a.ensureAppServer(ctx)
}

// StopAppServer stops the Hero-managed app-server child and clears Codex registry rows.
func (a *Adapter) StopAppServer(ctx context.Context) error {
	return a.stopAppServerState(ctx)
}

// HasManagedAppServer reports whether Hero owns a Codex app-server child.
func (a *Adapter) HasManagedAppServer() bool {
	a.mu.Lock()
	pid := a.appPID
	a.mu.Unlock()
	if pid > 0 {
		return true
	}
	if a.Store == nil {
		return false
	}
	entries, err := a.Store.ListCodexServeRegistry(a.ProjectDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.PID > 0 {
			return true
		}
	}
	return false
}

func (a *Adapter) setStatus(sessionID, state, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st, ok := a.sessions[sessionID]
	if !ok {
		st = &sessionState{session: harness.Session{ID: sessionID}}
		a.sessions[sessionID] = st
	}
	st.status = harness.ExecutionStatus{SessionID: sessionID, State: state, Message: message}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func turnStartParams(threadID string, req harness.ExecuteRequest, projectDir string) map[string]any {
	params := map[string]any{
		"threadId": threadID,
		"input": []map[string]string{
			{"type": "text", "text": req.Prompt},
		},
	}
	cwd := strings.TrimSpace(req.ProjectDir)
	if cwd == "" {
		cwd = strings.TrimSpace(projectDir)
	}
	if cwd != "" {
		params["cwd"] = cwd
	}
	if model := strings.TrimSpace(req.Model); model != "" {
		params["model"] = model
	}
	// C5: adapter-owned native mapping (ADR-045).
	for k, v := range nativePropertyOptions(req.Properties) {
		params[k] = v
	}
	if mode := strings.TrimSpace(strings.ToLower(req.Mode)); mode == harness.ModePlan || mode == "plan" {
		params["collaborationMode"] = map[string]any{
			"mode": "plan",
		}
	}
	return params
}
