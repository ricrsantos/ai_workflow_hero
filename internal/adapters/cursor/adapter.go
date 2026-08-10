package cursor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const adapterName = "cursor"

// Adapter implements harness.HarnessAdapter for Cursor projects via Agent CLI.
type Adapter struct {
	ProjectDir string
	Logger     *slog.Logger
	LookPath   LookPathFunc
	Runner     CommandRunner
	// Pusher attempts dispatch when the agent CLI is available. When nil, Dispatch
	// uses the default CLI Execute path (design D3).
	Pusher func(ctx context.Context, agentPath string, req harness.DispatchRequest) (harness.DispatchResult, error)
	// VerifyAgent checks that agentPath responds without invoking a prompt (no LLM).
	// Deprecated: prefer Runner; kept for existing tests that inject verification.
	VerifyAgent func(ctx context.Context, agentPath string) error

	mu            sync.Mutex
	resumeID      string
	sessions      map[string]*sessionState
	activeCancels map[string]context.CancelFunc
}

type sessionState struct {
	session   harness.Session
	status    harness.ExecutionStatus
	cancel    context.CancelFunc
	updatedAt time.Time
}

// NewAdapter returns a Cursor harness adapter for projectDir with default runner.
func NewAdapter(projectDir string) *Adapter {
	return &Adapter{
		ProjectDir:    projectDir,
		Logger:        slog.Default(),
		LookPath:      nil, // ResolveAgentCLI defaults to exec.LookPath
		Runner:        ExecCommandRunner{},
		sessions:      make(map[string]*sessionState),
		activeCancels: make(map[string]context.CancelFunc),
	}
}

// Name implements harness.HarnessAdapter.
func (a *Adapter) Name() string {
	return adapterName
}

// SupportsChat reports whether Cursor chat assets exist under ProjectDir.
// Kept as a concrete helper (not on HarnessAdapter; design D2).
func (a *Adapter) SupportsChat() bool {
	return a.chatReady()
}

// IsAvailable implements harness.HarnessAdapter (design D3).
func (a *Adapter) IsAvailable(ctx context.Context) error {
	spec, err := ResolveAgentCLI(a.LookPath)
	if err != nil {
		return err
	}
	runner := a.runner()

	verRes, err := runner.Run(ctx, a.ProjectDir, spec.Path, spec.BuildArgs("--version"))
	if err != nil {
		stdout, stderr := string(verRes.Stdout), string(verRes.Stderr)
		if IsAuthFailure(stdout, stderr) {
			return &AuthError{Detail: firstLine(stderr, stdout)}
		}
		return fmt.Errorf("cursor agent CLI not executable: %w", err)
	}
	a.log().Debug("cursor agent version ok", "version", strings.TrimSpace(string(verRes.Stdout)))

	// Lightweight auth probe: status/whoami when supported (no LLM).
	statusRes, statusErr := runner.Run(ctx, a.ProjectDir, spec.Path, spec.BuildArgs("status", "--format", "json"))
	combinedOut := string(statusRes.Stdout) + string(statusRes.Stderr)
	if IsAuthFailure(string(statusRes.Stdout), string(statusRes.Stderr)) {
		return &AuthError{Detail: firstLine(string(statusRes.Stderr), string(statusRes.Stdout))}
	}
	if statusErr != nil {
		// Older CLIs may lack `status`; treat unknown failures as unavailable only when auth-like.
		lower := strings.ToLower(combinedOut)
		if strings.Contains(lower, "unknown") || strings.Contains(lower, "unrecognized") {
			a.log().Debug("cursor agent status unsupported; version check sufficient")
			return nil
		}
		// Fall back to whoami.
		whoRes, whoErr := runner.Run(ctx, a.ProjectDir, spec.Path, spec.BuildArgs("whoami"))
		if IsAuthFailure(string(whoRes.Stdout), string(whoRes.Stderr)) {
			return &AuthError{Detail: firstLine(string(whoRes.Stderr), string(whoRes.Stdout))}
		}
		if whoErr != nil {
			a.log().Debug("cursor agent auth probe inconclusive", "error", whoErr)
		}
	}
	return nil
}

// CreateSession implements harness.HarnessAdapter.
func (a *Adapter) CreateSession(_ context.Context, req harness.SessionRequest) (*harness.Session, error) {
	dir := req.ProjectDir
	if dir == "" {
		dir = a.ProjectDir
	}
	sess := &harness.Session{
		ProjectDir: dir,
		StageName:  req.StageName,
		AgentName:  req.AgentName,
		CreatedAt:  time.Now().UTC(),
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	// Local placeholder until Execute returns Cursor session_id.
	key := placeholderSessionKey(sess)
	a.sessions[key] = &sessionState{
		session:   *sess,
		status:    harness.ExecutionStatus{State: harness.StatusIdle, Message: "session created"},
		updatedAt: time.Now().UTC(),
	}
	a.log().Info("cursor harness session created", "stage", req.StageName, "agent", req.AgentName)
	return sess, nil
}

// ResumeSession implements harness.HarnessAdapter.
func (a *Adapter) ResumeSession(_ context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resumeID = sessionID
	if st, ok := a.sessions[sessionID]; ok {
		st.status = harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusIdle, Message: "resume armed"}
		st.updatedAt = time.Now().UTC()
	} else {
		a.sessions[sessionID] = &sessionState{
			session:   harness.Session{ID: sessionID, ProjectDir: a.ProjectDir, CreatedAt: time.Now().UTC()},
			status:    harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusIdle, Message: "resume armed"},
			updatedAt: time.Now().UTC(),
		}
	}
	a.log().Info("cursor harness session resume armed", "session_id", sessionID)
	return nil
}

// Execute implements harness.HarnessAdapter.
func (a *Adapter) Execute(ctx context.Context, req harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("execute prompt is required")
	}
	spec, err := ResolveAgentCLI(a.LookPath)
	if err != nil {
		return nil, err
	}

	dir := req.ProjectDir
	if dir == "" {
		dir = a.ProjectDir
	}

	sessionID := strings.TrimSpace(req.SessionID)
	a.mu.Lock()
	if sessionID == "" {
		sessionID = a.resumeID
	}
	a.mu.Unlock()

	format := "json"
	if req.Stream {
		format = "stream-json"
	}
	args := []string{"--print", "--output-format", format}
	if model := strings.TrimSpace(req.Model); model != "" {
		args = append(args, "--model", model)
	}
	if sessionID != "" {
		args = append(args, "--resume="+sessionID)
	}
	// Prompt is positional (Cursor CLI: -p/--print is print mode, not prompt).
	args = append(args, prompt)
	fullArgs := spec.BuildArgs(args...)

	runCtx, cancel := context.WithCancel(ctx)
	trackID := sessionID
	if trackID == "" {
		trackID = "pending:" + req.StageName
	}
	a.setRunning(trackID, cancel)
	defer a.clearRunning(trackID, cancel)

	a.log().Info("cursor agent execute start", "format", format, "stage", req.StageName, "resume", sessionID != "")
	start := time.Now()
	runner := a.runner()
	res, err := runner.Run(runCtx, dir, spec.Path, fullArgs)
	elapsed := time.Since(start)

	stdout, stderr := string(res.Stdout), string(res.Stderr)
	if IsAuthFailure(stdout, stderr) {
		a.setStatus(trackID, harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusFailed, Message: LoginHint})
		return nil, &AuthError{Detail: firstLine(stderr, stdout)}
	}
	if err != nil {
		if runCtx.Err() != nil {
			a.setStatus(trackID, harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusCancelled, Message: "cancelled"})
			return nil, fmt.Errorf("cursor agent execute cancelled: %w", runCtx.Err())
		}
		a.setStatus(trackID, harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusFailed, Message: firstLine(stderr, err.Error())})
		a.log().Error("cursor agent execute failed", "error", err, "stderr", firstLine(stderr, ""))
		return nil, fmt.Errorf("cursor agent execute failed: %w (%s)", err, firstLine(stderr, stdout))
	}

	var parsed *harness.ExecutionResult
	if req.Stream {
		parsed, err = ParseStreamJSON(bytes.NewReader(res.Stdout), req.OnStreamDelta)
	} else {
		parsed, err = ParseJSONResult(res.Stdout)
	}
	if err != nil {
		a.setStatus(trackID, harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusFailed, Message: err.Error()})
		return nil, err
	}
	if parsed.Duration == 0 {
		parsed.Duration = elapsed
	}
	if parsed.SessionID == "" {
		parsed.SessionID = sessionID
	}
	a.rememberSession(parsed.SessionID, req, harness.StatusCompleted, "ok")
	a.log().Info("cursor agent execute done", "session_id", parsed.SessionID, "duration_ms", parsed.Duration.Milliseconds())
	return parsed, nil
}

// Cancel implements harness.HarnessAdapter.
func (a *Adapter) Cancel(_ context.Context, sessionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	cancel := a.activeCancels[sessionID]
	if cancel == nil && sessionID != "" {
		// Also try pending key variants.
		for k, c := range a.activeCancels {
			if strings.HasSuffix(k, sessionID) || strings.Contains(k, sessionID) {
				cancel = c
				break
			}
		}
	}
	if cancel == nil {
		return fmt.Errorf("no in-flight execution for session %q", sessionID)
	}
	cancel()
	if st, ok := a.sessions[sessionID]; ok {
		st.status = harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusCancelled, Message: "cancelled"}
		st.updatedAt = time.Now().UTC()
	}
	a.log().Info("cursor harness execution cancelled", "session_id", sessionID)
	return nil
}

// Status implements harness.HarnessAdapter.
func (a *Adapter) Status(_ context.Context, sessionID string) (*harness.ExecutionStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if st, ok := a.sessions[sessionID]; ok {
		cp := st.status
		return &cp, nil
	}
	if _, running := a.activeCancels[sessionID]; running {
		return &harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusRunning}, nil
	}
	return &harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusIdle, Message: "unknown session"}, nil
}

// Dispatch implements harness.HarnessAdapter as a thin wrapper over Execute (design D2/D3).
func (a *Adapter) Dispatch(ctx context.Context, req harness.DispatchRequest) (harness.DispatchResult, error) {
	fallback := dispatchFallbackMessage(req)
	if !a.chatReady() {
		a.log().Debug("cursor chat assets missing", "project", a.ProjectDir, "stage", req.StageName, "custom_prompt", isCustomCommandPrompt(req))
		return harness.DispatchResult{Dispatched: false, Message: fallback}, nil
	}

	spec, err := ResolveAgentCLI(a.LookPath)
	if err != nil {
		a.log().Debug("cursor agent cli not on PATH", "error", err)
		return harness.DispatchResult{Dispatched: false, Message: fallback}, nil
	}

	if a.VerifyAgent != nil {
		if err := a.VerifyAgent(ctx, spec.Path); err != nil {
			a.log().Info("cursor agent cli unavailable", "error", err)
			return harness.DispatchResult{Dispatched: false, Message: fallback}, nil
		}
	} else if err := a.IsAvailable(ctx); err != nil {
		msg := fallback
		if _, ok := err.(*AuthError); ok {
			msg = fmt.Sprintf("Dispatch unavailable; authenticate with `%s`.", LoginHint)
		}
		a.log().Info("cursor agent cli unavailable", "error", err)
		return harness.DispatchResult{Dispatched: false, Message: msg}, nil
	}

	if a.Pusher != nil {
		res, err := a.Pusher(ctx, spec.Path, req)
		if err != nil {
			a.log().Error("cursor dispatch failed", "stage", req.StageName, "error", err)
			return harness.DispatchResult{}, err
		}
		return res, nil
	}

	// Default Pusher path: Execute via CLI (remove nil-only chat fallback when CLI available).
	return a.defaultPush(ctx, req)
}

func (a *Adapter) defaultPush(ctx context.Context, req harness.DispatchRequest) (harness.DispatchResult, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = fmt.Sprintf("Continue Hero workflow for cycle stage %s.", req.StageName)
	}
	dir := req.ProjectDir
	if dir == "" {
		dir = a.ProjectDir
	}
	execRes, err := a.Execute(ctx, harness.ExecuteRequest{
		ProjectDir: dir,
		Prompt:     prompt,
		StageName:  req.StageName,
		Stream:     false,
	})
	if err != nil {
		if _, ok := err.(*AuthError); ok {
			return harness.DispatchResult{
				Dispatched: false,
				Message:    fmt.Sprintf("Dispatch unavailable; authenticate with `%s`.", LoginHint),
			}, nil
		}
		a.log().Error("cursor default push failed", "stage", req.StageName, "error", err)
		return harness.DispatchResult{}, err
	}
	msg := execRes.Summary
	if msg == "" {
		msg = execRes.Output
	}
	if len(msg) > 240 {
		msg = msg[:237] + "..."
	}
	a.log().Info("cursor dispatch via execute", "stage", req.StageName, "session_id", execRes.SessionID)
	return harness.DispatchResult{Dispatched: true, Message: msg}, nil
}

func (a *Adapter) chatReady() bool {
	if a.ProjectDir == "" {
		return false
	}
	startCmd := filepath.Join(a.ProjectDir, CommandsDir, "hero-start.md")
	if _, err := os.Stat(startCmd); err == nil {
		return true
	}
	cursorDir := filepath.Join(a.ProjectDir, CursorDir)
	fi, err := os.Stat(cursorDir)
	return err == nil && fi.IsDir()
}

func (a *Adapter) log() *slog.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return slog.Default()
}

func (a *Adapter) runner() CommandRunner {
	if a.Runner != nil {
		return a.Runner
	}
	return ExecCommandRunner{}
}

func (a *Adapter) setRunning(id string, cancel context.CancelFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.activeCancels[id] = cancel
	st, ok := a.sessions[id]
	if !ok {
		st = &sessionState{session: harness.Session{ID: id, ProjectDir: a.ProjectDir}}
		a.sessions[id] = st
	}
	st.status = harness.ExecutionStatus{SessionID: id, State: harness.StatusRunning}
	st.cancel = cancel
	st.updatedAt = time.Now().UTC()
}

func (a *Adapter) clearRunning(id string, _ context.CancelFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.activeCancels, id)
}

func (a *Adapter) setStatus(id string, status harness.ExecutionStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st, ok := a.sessions[id]
	if !ok {
		st = &sessionState{session: harness.Session{ID: id, ProjectDir: a.ProjectDir}}
		a.sessions[id] = st
	}
	st.status = status
	st.updatedAt = time.Now().UTC()
}

func (a *Adapter) rememberSession(sessionID string, req harness.ExecuteRequest, state, message string) {
	if sessionID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resumeID = sessionID
	a.sessions[sessionID] = &sessionState{
		session: harness.Session{
			ID:         sessionID,
			ProjectDir: req.ProjectDir,
			StageName:  req.StageName,
			AgentName:  req.AgentName,
			CreatedAt:  time.Now().UTC(),
		},
		status:    harness.ExecutionStatus{SessionID: sessionID, State: state, Message: message},
		updatedAt: time.Now().UTC(),
	}
}

func placeholderSessionKey(s *harness.Session) string {
	return fmt.Sprintf("local:%s:%s", s.StageName, s.CreatedAt.Format(time.RFC3339Nano))
}

func firstLine(primary, fallback string) string {
	s := strings.TrimSpace(primary)
	if s == "" {
		s = strings.TrimSpace(fallback)
	}
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// dispatchFallbackMessage returns an actionable unavailable message for stage dispatch
// or imported custom commands (expanded markdown Prompt with empty StageName). Design D3.
func dispatchFallbackMessage(req harness.DispatchRequest) string {
	if req.StageName != "" {
		return fmt.Sprintf("Dispatch unavailable for stage %s; continue via Cursor chat (/hero-start) or install/authenticate Cursor Agent CLI (`%s`).", req.StageName, LoginHint)
	}
	if isCustomCommandPrompt(req) {
		return "Dispatch unavailable; run the same command in Cursor chat."
	}
	return fmt.Sprintf("Dispatch unavailable; continue via Cursor chat (/hero-start) or run `%s`.", LoginHint)
}

func isCustomCommandPrompt(req harness.DispatchRequest) bool {
	return strings.TrimSpace(req.Prompt) != ""
}

// Compile-time interface check.
var _ harness.HarnessAdapter = (*Adapter)(nil)
