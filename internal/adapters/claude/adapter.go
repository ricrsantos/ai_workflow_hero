// Package claude implements the supervised Claude Code CLI harness.
package claude

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"gopkg.in/yaml.v3"
)

const (
	adapterName = "claude"
	cliName     = "claude"
	cancelGrace = 2 * time.Second
)

// LookPathFunc locates the Claude CLI. Tests inject it so no host installation
// is needed.
type LookPathFunc func(string) (string, error)

// ProcessLauncher starts exactly one Claude turn. It is deliberately separate
// from Launcher, which is only used for version and flag probes.
type ProcessLauncher interface {
	Start(context.Context, string, Invocation) (Process, error)
}

// Process is the owned, execution-scoped Claude child. Interrupt and Kill act
// on its process group, never on a shared daemon or another harness process.
type Process interface {
	Stdout() io.Reader
	Stderr() io.Reader
	Wait() error
	Interrupt() error
	Kill() error
	PID() int
}

// Clock is the small time seam needed for deterministic session timestamps and
// health snapshots.
type Clock interface{ Now() time.Time }

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now().UTC() }

// Adapter implements harness.HarnessAdapter and harness.HealthChecker.
// All externally observable dependencies are injectable; in particular,
// availability and session operations never call ProcessLauncher.Start.
type Adapter struct {
	ProjectDir      string
	Logger          *slog.Logger
	LookPath        LookPathFunc
	ProbeLauncher   Launcher
	ProcessLauncher ProcessLauncher
	Clock           Clock
	TokenSource     func() (string, error)
	// PermissionBridgeStarter creates the constrained, execution-scoped ask
	// bridge. It is injected by tests; production uses the local stdio bridge.
	PermissionBridgeStarter PermissionBridgeStarter
	// CatalogFS and ProjectReadFile keep offline model discovery deterministic
	// in tests and avoid touching the Claude CLI or credentials.
	CatalogFS       fs.FS
	ProjectReadFile func(string) ([]byte, error)

	mu       sync.Mutex
	sessions map[string]*sessionState
	active   map[string]*runningTurn
}

type sessionState struct {
	session harness.Session
	status  harness.ExecutionStatus
	updated time.Time
}

type runningTurn struct {
	process        Process
	status         harness.ExecutionStatus
	lastEventAt    time.Time
	lastActivityAt time.Time
	cancelled      bool
}

// NewAdapter returns a production adapter. It starts no process until Execute.
func NewAdapter(projectDir string) *Adapter {
	return &Adapter{
		ProjectDir:              projectDir,
		Logger:                  slog.Default(),
		LookPath:                exec.LookPath,
		ProcessLauncher:         execProcessLauncher{},
		Clock:                   wallClock{},
		TokenSource:             randomToken,
		PermissionBridgeStarter: defaultPermissionBridgeStarter{},
		CatalogFS:               assets.FS,
		ProjectReadFile:         os.ReadFile,
		sessions:                make(map[string]*sessionState),
		active:                  make(map[string]*runningTurn),
	}
}

func (a *Adapter) log() *slog.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return slog.Default()
}

func (a *Adapter) now() time.Time {
	if a.Clock != nil {
		return a.Clock.Now().UTC()
	}
	return time.Now().UTC()
}

// Name implements harness.HarnessAdapter.
func (a *Adapter) Name() string { return adapterName }

// IsAvailable checks PATH, version, and advertised headless flags only. It
// never starts a prompt/session and never performs authentication.
func (a *Adapter) IsAvailable(ctx context.Context) error {
	path, err := a.cliPath()
	if err != nil {
		return err
	}
	launcher := a.ProbeLauncher
	if launcher == nil {
		launcher = commandProbeLauncher{path: path}
	}
	if err := (ProtocolGate{Launcher: launcher, Logger: a.log()}).Verify(ctx); err != nil {
		return fmt.Errorf("Claude Code is unavailable: %w", err)
	}
	return nil
}

func (a *Adapter) cliPath() (string, error) {
	look := a.LookPath
	if look == nil {
		look = exec.LookPath
	}
	path, err := look(cliName)
	if err != nil || strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("Claude Code CLI is not on PATH; install Claude Code %s or newer, then retry", MinimumCLIVersion)
	}
	return path, nil
}

// CreateSession creates only a local Hero binding. Claude provides its native
// id from stream-json after Execute starts; a placeholder must not be resumed.
func (a *Adapter) CreateSession(_ context.Context, req harness.SessionRequest) (*harness.Session, error) {
	dir := strings.TrimSpace(req.ProjectDir)
	if dir == "" {
		dir = a.ProjectDir
	}
	s := &harness.Session{ProjectDir: dir, StageName: req.StageName, AgentName: req.AgentName, CreatedAt: a.now()}
	key := pendingSessionKey(req.StageName, req.AgentName)
	a.mu.Lock()
	a.sessions[key] = &sessionState{session: *s, status: harness.ExecutionStatus{State: harness.StatusIdle, Message: "Claude session binding created"}, updated: a.now()}
	a.mu.Unlock()
	a.log().Info("claude session binding created", "stage", req.StageName, "agent", req.AgentName)
	return s, nil
}

// ResumeSession arms a known Claude native session for the following Execute.
// Cross-harness protection is enforced by the store/TUI binding before this
// method is called; this adapter never guesses another harness's IDs.
func (a *Adapter) ResumeSession(_ context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("Claude session id is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if st, ok := a.sessions[sessionID]; ok {
		st.status = harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusIdle, Message: "Claude resume armed"}
		st.updated = a.now()
	} else {
		a.sessions[sessionID] = &sessionState{session: harness.Session{ID: sessionID, ProjectDir: a.ProjectDir, CreatedAt: a.now()}, status: harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusIdle, Message: "Claude resume armed"}, updated: a.now()}
	}
	a.log().Info("claude session resume armed", "session_id", sessionID)
	return nil
}

// Execute starts one supervised Claude CLI child and incrementally normalizes
// its NDJSON stream. A resume failure is returned as-is: the original prompt
// is never resent into a new Claude session.
func (a *Adapter) Execute(ctx context.Context, req harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	req = harness.NormalizeExecuteRequest(req)
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, errors.New("Claude execute prompt is required")
	}
	path, err := a.cliPath()
	if err != nil {
		return nil, err
	}
	if err := a.IsAvailable(ctx); err != nil {
		return nil, err
	}
	if err := a.validateProfile(req); err != nil {
		return nil, err
	}
	if harness.NormalizePermissionProfile(req.PermissionProfile) == harness.PermissionProfileAsk {
		if err := a.verifyLiveAskTransport(ctx); err != nil {
			return nil, err
		}
	}
	dir := strings.TrimSpace(req.ProjectDir)
	if dir == "" {
		dir = a.ProjectDir
	}
	trackID := strings.TrimSpace(req.SessionID)
	if isForeignClaudeSessionID(trackID) {
		return nil, fmt.Errorf("Claude cannot resume a %s session id", foreignHarnessName(trackID))
	}
	if trackID == "" {
		trackID = pendingSessionKey(req.StageName, req.AgentName)
	}
	invocation, err := a.command(dir, req)
	if err != nil {
		return nil, err
	}
	var bridge PermissionBridgeHandle
	var bridgeDone <-chan error
	var bridgeConfigPath string
	if harness.NormalizePermissionProfile(req.PermissionProfile) == harness.PermissionProfileAsk {
		token, tokenErr := a.TokenSource()
		if tokenErr != nil || strings.TrimSpace(token) == "" {
			if tokenErr == nil {
				tokenErr = errors.New("empty permission token")
			}
			return nil, fmt.Errorf("start Claude permission bridge: %w", tokenErr)
		}
		starter := a.PermissionBridgeStarter
		if starter == nil {
			starter = defaultPermissionBridgeStarter{}
		}
		bridge, err = starter.StartPermissionBridge(ctx, PermissionBridgeConfig{
			Token: token, SessionID: trackID, Logger: a.log(), Request: req.OnPermissionRequest,
		})
		if err != nil {
			return nil, fmt.Errorf("start Claude permission bridge: %w", err)
		}
		defer func() {
			if closeErr := bridge.Close(); closeErr != nil {
				a.log().Error("claude permission bridge cleanup failed", "error", closeErr)
			}
		}()
		if live, ok := bridge.(permissionMCPConfigProvider); ok {
			bridgeConfigPath, err = writePermissionMCPConfig(live.permissionMCPConfig())
			if err != nil {
				return nil, fmt.Errorf("start Claude permission bridge: %w", err)
			}
			defer os.Remove(bridgeConfigPath)
			// strict-mcp-config prevents user/project MCP configuration from being
			// exposed to this supervised turn. The file contains only the private
			// Hero approval helper and is removed on every exit path.
			invocation.Args = append(invocation.Args, "--mcp-config", bridgeConfigPath, "--strict-mcp-config")
		}
	}
	invocation.Args = append(invocation.Args, prompt)
	launcher := a.ProcessLauncher
	if launcher == nil {
		launcher = execProcessLauncher{}
	}
	started := a.now()
	process, err := launcher.Start(ctx, path, invocation)
	if err != nil {
		return nil, classifyStartError(err)
	}
	a.setRunning(trackID, process)
	defer a.clearRunning(trackID, process)
	stopContextWatch := make(chan struct{})
	defer close(stopContextWatch)
	go func() {
		select {
		case <-ctx.Done():
			// Context cancellation has the same SIGINT-first lifecycle as an
			// explicit TUI cancel. It never relies on CommandContext killing only
			// the parent and leaving Claude child tools behind.
			_ = a.Cancel(context.Background(), trackID)
		case <-stopContextWatch:
		}
	}()
	a.log().Info("claude turn started", "pid", process.PID(), "stage", req.StageName, "resume", req.SessionID != "")
	if bridge != nil {
		done := make(chan error, 1)
		bridgeDone = done
		go func() { done <- bridge.Run(ctx) }()
	}

	assembler := newResultAssembler()
	stderrDone := make(chan string, 1)
	go func() { stderrDone <- readRedacted(process.Stderr()) }()
	streamErr := a.consume(ctx, process.Stdout(), req, trackID, assembler)
	waitErr := process.Wait()
	stderr := <-stderrDone
	if streamErr != nil {
		a.setFailed(trackID, streamErr.Error())
		return nil, streamErr
	}
	if waitErr != nil {
		err := classifyExecutionError(waitErr, stderr)
		a.setFailed(trackID, err.Error())
		return nil, err
	}
	if bridgeDone != nil {
		select {
		case bridgeErr := <-bridgeDone:
			if bridgeErr != nil && !errors.Is(bridgeErr, io.EOF) && !errors.Is(bridgeErr, context.Canceled) {
				a.setFailed(trackID, bridgeErr.Error())
				return nil, fmt.Errorf("Claude permission bridge: %w", bridgeErr)
			}
		default:
		}
	}
	result, err := assembler.result(a.now().Sub(started))
	if err != nil {
		a.setFailed(trackID, err.Error())
		return nil, err
	}
	if result.SessionID != "" {
		a.setCompleted(result.SessionID)
	} else {
		a.setCompleted(trackID)
	}
	a.log().Info("claude turn completed", "session_id", result.SessionID, "duration", result.Duration)
	return result, nil
}

func isForeignClaudeSessionID(sessionID string) bool {
	sessionID = strings.ToLower(strings.TrimSpace(sessionID))
	return strings.HasPrefix(sessionID, "thr_") || strings.HasPrefix(sessionID, "thread_") ||
		strings.HasPrefix(sessionID, "opencode:") || strings.HasPrefix(sessionID, "cursor:")
}

func foreignHarnessName(sessionID string) string {
	sessionID = strings.ToLower(strings.TrimSpace(sessionID))
	switch {
	case strings.HasPrefix(sessionID, "thr_"), strings.HasPrefix(sessionID, "thread_"):
		return "Codex"
	case strings.HasPrefix(sessionID, "opencode:"):
		return "OpenCode"
	default:
		return "Cursor"
	}
}

func (a *Adapter) validateProfile(req harness.ExecuteRequest) error {
	switch harness.NormalizePermissionProfile(req.PermissionProfile) {
	case harness.PermissionProfileAsk:
		if a.TokenSource == nil {
			return fmt.Errorf("%w: configure the Claude permission bridge before using ask", ErrAskUnsupported)
		}
		if req.OnPermissionRequest == nil {
			return fmt.Errorf("%w: Claude ask mode requires a permission decision callback", ErrAskUnsupported)
		}
		return nil
	case harness.PermissionProfileAutoProject, harness.PermissionProfileAutoAll:
		return nil
	default:
		return fmt.Errorf("%w: unsupported Claude permission profile", ErrAskUnsupported)
	}
}

func (a *Adapter) verifyLiveAskTransport(ctx context.Context) error {
	launcher := a.ProbeLauncher
	if launcher == nil {
		path, err := a.cliPath()
		if err != nil {
			return err
		}
		launcher = commandProbeLauncher{path: path}
	}
	help, err := launcher.Run(ctx, Invocation{Args: []string{"--help"}})
	if err != nil {
		return fmt.Errorf("%w: probe Claude ask transport: %v", ErrAskUnsupported, err)
	}
	if help.ExitCode != 0 {
		return fmt.Errorf("%w: Claude --help exited with status %d", ErrAskUnsupported, help.ExitCode)
	}
	if err := ValidateLiveAskTransportFlags(help.Stdout); err != nil {
		return fmt.Errorf("%w: %v", ErrAskUnsupported, err)
	}
	return nil
}

func (a *Adapter) command(dir string, req harness.ExecuteRequest) (Invocation, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return Invocation{}, errors.New("Claude model id is required")
	}
	invocation := StreamInvocation(dir, model)
	if sessionID := strings.TrimSpace(req.SessionID); sessionID != "" {
		invocation.Args = append(invocation.Args, "--resume", sessionID)
	}
	switch req.PermissionProfile {
	case harness.PermissionProfileAsk:
		invocation.Args = append(invocation.Args, "--permission-prompt-tool", PermissionPromptToolName)
	case harness.PermissionProfileAutoProject:
		invocation.Args = append(invocation.Args, "--permission-mode", "acceptEdits")
	case harness.PermissionProfileAutoAll:
		invocation.Args = append(invocation.Args, "--dangerously-skip-permissions")
	}
	if effort := strings.TrimSpace(req.Properties["ef"]); effort != "" {
		invocation.Args = append(invocation.Args, "--effort", effort)
	}
	return invocation, nil
}

func (a *Adapter) consume(ctx context.Context, r io.Reader, req harness.ExecuteRequest, trackID string, assembler *resultAssembler) error {
	if r == nil {
		return errors.New("Claude process did not provide stdout")
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxNDJSONLineBytes)
	for line := 1; scanner.Scan(); line++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw := append([]byte(nil), scanner.Bytes()...)
		if len(strings.TrimSpace(string(raw))) == 0 {
			continue
		}
		event, err := decodeRawEvent(line, raw)
		if err != nil {
			return err
		}
		deltas, err := assembler.consume(event)
		if err != nil {
			return err
		}
		for _, delta := range deltas {
			if delta.SessionID == "" {
				delta.SessionID = assembler.sessionID
			}
			a.recordDelta(trackID, delta)
			if delta.Kind == harness.StreamKindSession && delta.SessionID != "" {
				a.bindNativeSession(trackID, delta.SessionID, req)
			}
			if err := forwardClaudeGate(ctx, req, delta); err != nil {
				return err
			}
			if req.Stream && req.OnStreamDelta != nil {
				req.OnStreamDelta(delta)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Claude stream: %w", err)
	}
	return nil
}

func forwardClaudeGate(ctx context.Context, req harness.ExecuteRequest, delta harness.StreamDelta) error {
	requestID := strings.TrimSpace(delta.Metadata["request_id"])
	switch delta.Kind {
	case harness.StreamKindPermission:
		if req.OnPermissionRequest == nil {
			return fmt.Errorf("Claude permission request %q has no decision callback", requestID)
		}
		_, err := req.OnPermissionRequest(ctx, harness.PermissionRequest{ID: requestID, Title: "Claude permission", Description: delta.Text, HarnessType: delta.HarnessType, SessionID: delta.SessionID})
		if err != nil {
			return fmt.Errorf("Claude permission decision: %w", err)
		}
	case harness.StreamKindQuestion:
		if req.OnQuestionRequest == nil {
			return fmt.Errorf("Claude question request %q has no response callback", requestID)
		}
		_, err := req.OnQuestionRequest(ctx, harness.QuestionRequest{ID: requestID, HarnessType: delta.HarnessType, SessionID: delta.SessionID, Questions: []harness.QuestionItem{{Question: delta.Text, Custom: true}}})
		if err != nil {
			return fmt.Errorf("Claude question response: %w", err)
		}
	}
	return nil
}

// Cancel sends SIGINT first, then schedules a bounded kill fallback. It is
// intentionally idempotent so UI and context cancellation can race safely.
func (a *Adapter) Cancel(_ context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	a.mu.Lock()
	turn, ok := a.active[sessionID]
	if !ok && sessionID == "" && len(a.active) == 1 {
		for _, turn = range a.active {
			ok = true
		}
	}
	if !ok || turn == nil {
		a.mu.Unlock()
		return nil
	}
	if turn.cancelled {
		a.mu.Unlock()
		return nil
	}
	turn.cancelled = true
	turn.status.State = harness.StatusCancelled
	turn.status.Message = "Claude cancellation requested"
	process := turn.process
	a.mu.Unlock()
	if err := process.Interrupt(); err != nil {
		return fmt.Errorf("interrupt Claude process group: %w", err)
	}
	a.log().Info("claude cancellation requested", "pid", process.PID())
	go func() {
		time.Sleep(cancelGrace)
		a.mu.Lock()
		stillActive := false
		for _, active := range a.active {
			if active.process == process {
				stillActive = true
				break
			}
		}
		a.mu.Unlock()
		if stillActive {
			_ = process.Kill()
			a.log().Info("claude cancellation escalated", "pid", process.PID())
		}
	}()
	return nil
}

// Status implements harness.HarnessAdapter without probing or starting Claude.
func (a *Adapter) Status(_ context.Context, sessionID string) (*harness.ExecutionStatus, error) {
	sessionID = strings.TrimSpace(sessionID)
	a.mu.Lock()
	defer a.mu.Unlock()
	if turn, ok := a.active[sessionID]; ok {
		status := turn.status
		status.SessionID = sessionID
		return &status, nil
	}
	if st, ok := a.sessions[sessionID]; ok {
		status := st.status
		status.SessionID = sessionID
		return &status, nil
	}
	return &harness.ExecutionStatus{SessionID: sessionID, State: harness.StatusIdle, Message: "no active Claude execution"}, nil
}

// Dispatch delegates to Execute, keeping the legacy path daemon-free.
func (a *Adapter) Dispatch(ctx context.Context, req harness.DispatchRequest) (harness.DispatchResult, error) {
	result, err := a.Execute(ctx, harness.ExecuteRequest{ProjectDir: req.ProjectDir, Prompt: req.Prompt, StageName: req.StageName, Model: req.Model, Mode: req.Mode, Stream: true, PermissionProfile: harness.PermissionProfileAsk})
	if err != nil {
		return harness.DispatchResult{}, err
	}
	return harness.DispatchResult{Dispatched: true, Message: result.Summary}, nil
}

// ListModels reads only the embedded/project-local Claude catalog. It never
// invokes the CLI, starts a session, or requires authentication, which keeps
// boot and picker discovery deterministic.
func (a *Adapter) ListModels(_ context.Context) ([]string, error) {
	models := make(map[string]struct{})
	mergeClaudeCatalog := func(data []byte) {
		var catalog struct {
			Provider string         `yaml:"provider"`
			Models   map[string]any `yaml:"models"`
		}
		if yaml.Unmarshal(data, &catalog) != nil || !strings.EqualFold(strings.TrimSpace(catalog.Provider), adapterName) {
			return
		}
		for model := range catalog.Models {
			if model = strings.TrimSpace(model); model != "" {
				models[model] = struct{}{}
			}
		}
	}
	catalogFS := a.CatalogFS
	if catalogFS == nil {
		catalogFS = assets.FS
	}
	if data, err := fs.ReadFile(catalogFS, "models/claude.yml"); err == nil {
		mergeClaudeCatalog(data)
	}
	if strings.TrimSpace(a.ProjectDir) != "" {
		readFile := a.ProjectReadFile
		if readFile == nil {
			readFile = os.ReadFile
		}
		if data, err := readFile(filepath.Join(a.ProjectDir, ".workflow-hero", "models", "claude.yml")); err == nil {
			mergeClaudeCatalog(data)
		}
	}
	if len(models) == 0 {
		return nil, errors.New("Claude model catalog is unavailable")
	}
	out := make([]string, 0, len(models))
	for model := range models {
		out = append(out, model)
	}
	sort.Strings(out)
	return out, nil
}

// CheckHealth is observational. It never starts a turn or a probe process.
func (a *Adapter) CheckHealth(_ context.Context, sessionID string) (harness.HarnessHealth, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	turn, ok := a.active[strings.TrimSpace(sessionID)]
	if !ok && strings.TrimSpace(sessionID) == "" && len(a.active) == 1 {
		for _, turn = range a.active {
			ok = true
		}
	}
	if !ok || turn == nil {
		return harness.HarnessHealth{ProcessAlive: false, ServerAlive: false, SessionAlive: true, Details: "no active Claude process"}, nil
	}
	return harness.HarnessHealth{ProcessAlive: true, ServerAlive: true, SessionAlive: turn.status.State != harness.StatusFailed, LastEventAt: turn.lastEventAt, LastActivityAt: turn.lastActivityAt, Details: turn.status.Message}, nil
}

func (a *Adapter) setRunning(id string, process Process) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.active[id] = &runningTurn{process: process, status: harness.ExecutionStatus{SessionID: id, State: harness.StatusRunning, Message: "Claude process running"}}
}

func (a *Adapter) clearRunning(id string, process Process) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if current, ok := a.active[id]; ok && current.process == process {
		delete(a.active, id)
	}
	// The first init frame replaces a temporary execution key with the native
	// Claude session id. Remove either form after the child exits.
	for key, current := range a.active {
		if current.process == process {
			delete(a.active, key)
		}
	}
}

func (a *Adapter) recordDelta(id string, delta harness.StreamDelta) {
	a.mu.Lock()
	defer a.mu.Unlock()
	turn, ok := a.active[id]
	if !ok {
		return
	}
	now := a.now()
	turn.lastEventAt = now
	switch delta.Kind {
	case harness.StreamKindText, harness.StreamKindThinking, harness.StreamKindTool, harness.StreamKindActivity, harness.StreamKindPermission, harness.StreamKindQuestion:
		turn.lastActivityAt = now
	}
}

func (a *Adapter) bindNativeSession(trackID, nativeID string, req harness.ExecuteRequest) {
	nativeID = strings.TrimSpace(nativeID)
	if nativeID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	dir := req.ProjectDir
	if dir == "" {
		dir = a.ProjectDir
	}
	st, exists := a.sessions[nativeID]
	if !exists {
		st = &sessionState{session: harness.Session{ID: nativeID, ProjectDir: dir, StageName: req.StageName, AgentName: req.AgentName, CreatedAt: a.now()}}
		a.sessions[nativeID] = st
	}
	st.status = harness.ExecutionStatus{SessionID: nativeID, State: harness.StatusRunning, Message: "Claude native session bound"}
	st.updated = a.now()
	if turn, ok := a.active[trackID]; ok {
		a.active[nativeID] = turn
		delete(a.active, trackID)
	}
}

func (a *Adapter) setCompleted(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if st, ok := a.sessions[id]; ok {
		st.status = harness.ExecutionStatus{SessionID: id, State: harness.StatusCompleted, Message: "Claude turn completed"}
		st.updated = a.now()
	}
}

func (a *Adapter) setFailed(id, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if st, ok := a.sessions[id]; ok {
		st.status = harness.ExecutionStatus{SessionID: id, State: harness.StatusFailed, Message: message}
		st.updated = a.now()
	}
}

func pendingSessionKey(stage, agent string) string {
	return "pending:" + strings.TrimSpace(stage) + ":" + strings.TrimSpace(agent)
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate Claude permission token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

var (
	_ harness.HarnessAdapter = (*Adapter)(nil)
	_ harness.HealthChecker  = (*Adapter)(nil)
	_ harness.ModelLister    = (*Adapter)(nil)
)
