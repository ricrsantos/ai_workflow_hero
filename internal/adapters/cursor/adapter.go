package cursor

import (
	"context"
	"fmt"
	"io"
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

	// RetryMax is the number of Execute attempts (0 = default 3). Tests may set 1 to disable.
	RetryMax int
	// RetrySleep replaces time.Sleep between retriable Execute failures (tests inject a no-op).
	RetrySleep func(time.Duration)

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
	req = harness.NormalizeExecuteRequest(req)
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
	// Only resume when the caller passes SessionID explicitly (conversation).
	// Do not inject adapter.resumeID — that leaked chat/sync sessions into palette Dispatch.

	// C5: compose normalized properties into the native model slug (ADR-041).
	// Workflow-composed slugs that already carry the suffix are not double-suffixed.
	model := ComposeModelSlug(req.Model, req.Properties)

	format := "json"
	if req.Stream {
		format = "stream-json"
	}
	// --print is non-interactive: without --force, Auto-review rejects Shell (no TTY
	// to prompt), so the agent cannot run `hero` and starts searching parent dirs.
	// --workspace pins the agent to the consumer project (cmd.Dir alone is not enough
	// when Cursor walks up for a git/workspace root).
	// --sandbox disabled keeps the user's PATH (nvm/npm/openspec) in Shell; the
	// default sandbox often strips those dirs so `hero cycle archive` cannot find
	// openspec even when it is installed for the login shell.
	// --approve-mcps auto-approves MCP tool servers in headless mode (no TTY to prompt).
	args := []string{"--print", "--output-format", format, "--trust", "--force", "--approve-mcps", "--sandbox", "disabled"}
	if dir != "" {
		args = append(args, "--workspace", dir)
	}
	if req.Stream {
		// Character-level assistant deltas while the process runs.
		args = append(args, "--stream-partial-output")
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if mode := strings.TrimSpace(strings.ToLower(req.Mode)); mode == harness.ModePlan || mode == "plan" {
		args = append(args, "--mode", "plan")
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
		trackID = pendingExecKey(req.StageName)
	}
	a.setRunning(trackID, cancel)
	defer a.clearRunning(trackID, cancel)

	a.log().Info("cursor agent execute start", "format", format, "stage", req.StageName, "resume", sessionID != "", "stream", req.Stream)
	start := time.Now()
	runner := a.runner()

	var (
		parsed *harness.ExecutionResult
		res    RunResult
	)
	attempts := a.executeRetryMax()
	for attempt := 1; attempt <= attempts; attempt++ {
		parsed = nil
		res = RunResult{}
		if req.Stream {
			parsed, res, err = a.executeStreamLive(runCtx, runner, dir, spec.Path, fullArgs, req)
		} else {
			res, err = runner.Run(runCtx, dir, spec.Path, fullArgs)
			if err == nil {
				parsed, err = ParseJSONResult(res.Stdout)
			}
		}
		stdout, stderr := string(res.Stdout), string(res.Stderr)
		if IsAuthFailure(stdout, stderr) {
			a.setStatus(trackID, harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusFailed, Message: LoginHint})
			return nil, &AuthError{Detail: firstLine(stderr, stdout)}
		}
		if IsTrustFailure(stdout, stderr) {
			msg := firstLine(stderr, stdout)
			a.setStatus(trackID, harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusFailed, Message: TrustHint})
			return nil, fmt.Errorf("cursor agent workspace trust required (%s); %s", msg, TrustHint)
		}
		if err != nil {
			if runCtx.Err() != nil {
				a.setStatus(trackID, harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusCancelled, Message: "cancelled"})
				return nil, fmt.Errorf("cursor agent execute cancelled: %w", runCtx.Err())
			}
			if attempt < attempts && IsRetriableFailure(stdout, stderr, err) {
				a.log().Warn("cursor agent execute retriable failure", "attempt", attempt, "error", err, "stderr", firstLine(stderr, stdout))
				a.sleep(executeRetryBackoff(attempt))
				continue
			}
			// C5: a property-composed slug rejected by the CLI must fail
			// explicitly instead of stripping the property or retrying silently.
			if rejection := propertyRejectionForOutput(model, req.Properties, stdout, stderr, err); rejection != nil {
				a.setStatus(trackID, harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusFailed, Message: rejection.Error()})
				return nil, rejection
			}
			a.setStatus(trackID, harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusFailed, Message: firstLine(stderr, err.Error())})
			a.log().Error("cursor agent execute failed", "error", err, "stderr", firstLine(stderr, ""))
			return nil, fmt.Errorf("cursor agent execute failed: %w (%s)", err, firstLine(stderr, stdout))
		}
		break
	}
	elapsed := time.Since(start)
	if parsed == nil {
		a.setStatus(trackID, harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusFailed, Message: "empty parse result"})
		return nil, fmt.Errorf("cursor agent execute failed: empty parse result")
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

// executeStreamLive pipes CLI stdout into ParseStreamJSON while the process runs
// so OnStreamDelta fires before the agent exits.
func (a *Adapter) executeStreamLive(
	ctx context.Context,
	runner CommandRunner,
	dir, path string,
	args []string,
	req harness.ExecuteRequest,
) (*harness.ExecutionResult, RunResult, error) {
	pr, pw := io.Pipe()
	type parseOut struct {
		res *harness.ExecutionResult
		err error
	}
	parsedCh := make(chan parseOut, 1)
	opts := StreamParseOptions{
		OnDelta:             req.OnStreamDelta,
		OnPermissionRequest: req.OnPermissionRequest,
	}
	go func() {
		res, err := ParseStreamJSONWithOptions(ctx, pr, opts)
		parsedCh <- parseOut{res: res, err: err}
	}()

	var (
		runRes RunResult
		runErr error
	)
	if sr, ok := runner.(StreamingCommandRunner); ok {
		runRes, runErr = sr.RunStreaming(ctx, dir, path, args, pw)
	} else {
		// Fallback: buffer then feed the parser (deltas still fire, but only after exit).
		runRes, runErr = runner.Run(ctx, dir, path, args)
		if len(runRes.Stdout) > 0 {
			_, _ = pw.Write(runRes.Stdout)
		}
	}
	_ = pw.Close()
	out := <-parsedCh

	if runErr != nil {
		return out.res, runRes, runErr
	}
	if out.err != nil {
		return nil, runRes, out.err
	}
	if out.res == nil {
		return nil, runRes, fmt.Errorf("cursor agent stream-json: empty result")
	}
	if runRes.ExitCode != 0 && strings.TrimSpace(out.res.Output) == "" {
		detail := firstLine(string(runRes.Stderr), string(runRes.Stdout))
		if req.OnStreamDelta != nil {
			req.OnStreamDelta(harness.SessionDelta(harness.SessionStateFailed, detail, "process.exit", ""))
		}
		return nil, runRes, fmt.Errorf("cursor agent exited with code %d: %s", runRes.ExitCode, detail)
	}
	return out.res, runRes, nil
}

// Cancel implements harness.HarnessAdapter.
// An empty sessionID cancels the in-flight Execute (TUI /hero-start clears the
// session until the CLI returns a Cursor chat id).
func (a *Adapter) Cancel(_ context.Context, sessionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	cancel := a.lookupCancelLocked(sessionID)
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

// HasInFlight reports whether a Cursor agent CLI process is currently running.
func (a *Adapter) HasInFlight() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.activeCancels) > 0
}

func pendingExecKey(stageName string) string {
	return "pending:" + stageName
}

func (a *Adapter) lookupCancelLocked(sessionID string) context.CancelFunc {
	if c := a.activeCancels[sessionID]; c != nil {
		return c
	}
	if sessionID != "" {
		for k, c := range a.activeCancels {
			if strings.HasSuffix(k, sessionID) || strings.Contains(k, sessionID) {
				return c
			}
		}
		return nil
	}
	// No Cursor session id yet: cancel the pending Execute (usually one).
	if c := a.activeCancels[pendingExecKey("")]; c != nil {
		return c
	}
	for k, c := range a.activeCancels {
		if strings.HasPrefix(k, "pending:") {
			return c
		}
	}
	for _, c := range a.activeCancels {
		return c
	}
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
		Model:      req.Model,
		Mode:       req.Mode,
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
	msg := strings.TrimSpace(execRes.Output)
	if msg == "" {
		msg = strings.TrimSpace(execRes.Summary)
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
	// Do not arm resumeID here — callers that need resume pass SessionID explicitly.
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

const defaultExecuteRetries = 3

func (a *Adapter) executeRetryMax() int {
	if a != nil && a.RetryMax > 0 {
		return a.RetryMax
	}
	return defaultExecuteRetries
}

func (a *Adapter) sleep(d time.Duration) {
	if a != nil && a.RetrySleep != nil {
		a.RetrySleep(d)
		return
	}
	time.Sleep(d)
}

func executeRetryBackoff(attempt int) time.Duration {
	d := time.Duration(attempt) * time.Second
	if d > 4*time.Second {
		return 4 * time.Second
	}
	return d
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
