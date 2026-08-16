package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const adapterName = "opencode"

// LookPathFunc resolves CLI binaries (tests inject).
type LookPathFunc func(string) (string, error)

// ProcessRunner starts and stops child processes (design D6).
type ProcessRunner interface {
	Start(ctx context.Context, dir, name string, args ...string) (ProcessHandle, error)
}

// ProcessHandle is a started child process.
type ProcessHandle interface {
	PID() int
	Wait() error
	Kill() error
}

// HTTPDoer performs HTTP requests (tests inject).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ExecRunner is the default ProcessRunner using os/exec.
type ExecRunner struct{}

type execHandle struct {
	cmd *exec.Cmd
}

func (ExecRunner) Start(ctx context.Context, dir, name string, args ...string) (ProcessHandle, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &execHandle{cmd: cmd}, nil
}

func (h *execHandle) PID() int    { return h.cmd.Process.Pid }
func (h *execHandle) Wait() error { return h.cmd.Wait() }
func (h *execHandle) Kill() error { return h.cmd.Process.Kill() }

// ServeURLResolver returns the base URL after starting opencode serve (tests inject).
type ServeURLResolver func(handle ProcessHandle) (baseURL string, port int, err error)

// Adapter implements harness.HarnessAdapter via opencode serve HTTP API (ADR-035).
type Adapter struct {
	ProjectDir      string
	Store           *store.Store
	Logger          *slog.Logger
	LookPath        LookPathFunc
	Runner          ProcessRunner
	HTTP            HTTPDoer
	ResolveServeURL ServeURLResolver

	mu        sync.Mutex
	baseURL   string
	servePID  int
	servePort int
	sessions  map[string]*sessionState
	cancels   map[string]context.CancelFunc
}

type sessionState struct {
	session harness.Session
	status  harness.ExecutionStatus
}

// NewAdapter returns an OpenCode harness adapter for projectDir.
func NewAdapter(projectDir string, st *store.Store) *Adapter {
	return &Adapter{
		ProjectDir: projectDir,
		Store:      st,
		Logger:     slog.Default(),
		LookPath:   exec.LookPath,
		Runner:     ExecRunner{},
		HTTP:       http.DefaultClient,
		sessions:   make(map[string]*sessionState),
		cancels:    make(map[string]context.CancelFunc),
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
		return fmt.Errorf("opencode CLI not on PATH: %w", err)
	}
	return nil
}

func (a *Adapter) cliPath() (string, error) {
	look := a.LookPath
	if look == nil {
		look = exec.LookPath
	}
	return look("opencode")
}

// CreateSession implements harness.HarnessAdapter.
func (a *Adapter) CreateSession(ctx context.Context, req harness.SessionRequest) (*harness.Session, error) {
	if err := a.ensureServe(ctx); err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]string{"title": req.StageName})
	resp, err := a.post(ctx, "/session", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var sess opencodeSession
	if err := json.NewDecoder(resp.Body).Decode(&sess); err != nil {
		return nil, err
	}
	out := &harness.Session{
		ID:         sess.ID,
		ProjectDir: req.ProjectDir,
		StageName:  req.StageName,
		AgentName:  req.AgentName,
		CreatedAt:  time.Now().UTC(),
	}
	a.mu.Lock()
	a.sessions[sess.ID] = &sessionState{session: *out}
	a.mu.Unlock()
	return out, nil
}

// ResumeSession implements harness.HarnessAdapter.
func (a *Adapter) ResumeSession(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id required")
	}
	if err := a.ensureServe(ctx); err != nil {
		return err
	}
	resp, err := a.get(ctx, "/session/"+sessionID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("session %q not found", sessionID)
	}
	return nil
}

// Execute implements harness.HarnessAdapter.
func (a *Adapter) Execute(ctx context.Context, req harness.ExecuteRequest) (*harness.ExecutionResult, error) {
	if err := a.ensureServe(ctx); err != nil {
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
	}
	runCtx, cancel := context.WithCancel(ctx)
	a.mu.Lock()
	a.cancels[sessionID] = cancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.cancels, sessionID)
		a.mu.Unlock()
		cancel()
	}()

	start := time.Now()
	model := strings.TrimSpace(req.Model)
	parts := []map[string]string{{"type": "text", "text": req.Prompt}}
	payload := map[string]any{
		"parts": parts,
	}
	if model != "" {
		payload["model"] = model
	}
	if req.AgentName != "" {
		payload["agent"] = req.AgentName
	}
	body, _ := json.Marshal(payload)

	if req.Stream && req.OnStreamDelta != nil {
		return a.executeStream(runCtx, sessionID, body, req, start)
	}
	resp, err := a.post(runCtx, "/session/"+sessionID+"/message", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var msgResp messageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		return nil, err
	}
	text := extractText(msgResp.Parts)
	return &harness.ExecutionResult{
		SessionID:  sessionID,
		Output:     text,
		Summary:    truncate(text, 200),
		Duration:   time.Since(start),
		StreamDone: true,
	}, nil
}

func (a *Adapter) executeStream(ctx context.Context, sessionID string, body []byte, req harness.ExecuteRequest, start time.Time) (*harness.ExecutionResult, error) {
	resp, err := a.post(ctx, "/session/"+sessionID+"/prompt_async", body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	var buf strings.Builder
	events, err := a.subscribeEvents(ctx)
	if err != nil {
		return nil, err
	}
	defer events.Close()
	dec := json.NewDecoder(events)
	for dec.More() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var evt map[string]any
		if err := dec.Decode(&evt); err != nil {
			if err == io.EOF {
				break
			}
			a.log().Debug("opencode event decode", "error", err)
			continue
		}
		text, done := parseEventDelta(evt, sessionID)
		if text != "" && req.OnStreamDelta != nil {
			req.OnStreamDelta(harness.StreamDelta{Kind: harness.StreamKindText, Text: text})
			buf.WriteString(text)
		}
		if done {
			break
		}
	}
	out := buf.String()
	return &harness.ExecutionResult{
		SessionID:  sessionID,
		Output:     out,
		Summary:    truncate(out, 200),
		Duration:   time.Since(start),
		StreamDone: true,
	}, nil
}

// Cancel implements harness.HarnessAdapter.
func (a *Adapter) Cancel(ctx context.Context, sessionID string) error {
	a.mu.Lock()
	if c, ok := a.cancels[sessionID]; ok {
		c()
	}
	a.mu.Unlock()
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if err := a.ensureServe(ctx); err != nil {
		return err
	}
	resp, err := a.post(ctx, "/session/"+sessionID+"/abort", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Status implements harness.HarnessAdapter.
func (a *Adapter) Status(ctx context.Context, sessionID string) (*harness.ExecutionStatus, error) {
	a.mu.Lock()
	if st, ok := a.sessions[sessionID]; ok {
		s := st.status
		a.mu.Unlock()
		return &s, nil
	}
	a.mu.Unlock()
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

// ListModels implements harness.ModelLister via /config/providers.
func (a *Adapter) ListModels(ctx context.Context) ([]string, error) {
	if err := a.ensureServe(ctx); err != nil {
		return nil, err
	}
	resp, err := a.get(ctx, "/config/providers")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data providersResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	var models []string
	for _, p := range data.Providers {
		for _, m := range p.Models {
			id := strings.TrimSpace(m.ID)
			if id == "" {
				continue
			}
			if p.ID != "" && !strings.Contains(id, "/") {
				id = p.ID + "/" + id
			}
			models = append(models, id)
		}
	}
	return models, nil
}

// StopServe stops the managed serve child and clears registry rows.
func (a *Adapter) StopServe(ctx context.Context) error {
	a.mu.Lock()
	pid := a.servePID
	a.baseURL = ""
	a.servePID = 0
	a.servePort = 0
	a.mu.Unlock()
	if pid > 0 {
		proc, _ := exec.Command("kill", fmt.Sprintf("%d", pid)).Output()
		_ = proc
	}
	if a.Store != nil {
		_ = a.Store.ClearServeRegistry()
	}
	return nil
}

func (a *Adapter) ensureServe(ctx context.Context) error {
	a.mu.Lock()
	if a.baseURL != "" {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	cli, err := a.cliPath()
	if err != nil {
		return err
	}
	handle, err := a.Runner.Start(ctx, a.ProjectDir, cli, "serve", "--port", "0", "--hostname", "127.0.0.1")
	if err != nil {
		return fmt.Errorf("start opencode serve: %w", err)
	}
	resolve := a.ResolveServeURL
	if resolve == nil {
		resolve = defaultServeURLResolver
	}
	url, port, err := resolve(handle)
	if err != nil {
		_ = handle.Kill()
		return fmt.Errorf("resolve opencode serve url: %w", err)
	}

	a.mu.Lock()
	a.baseURL = url
	a.servePID = handle.PID()
	a.servePort = port
	a.mu.Unlock()

	if a.Store != nil {
		_, _ = a.Store.InsertServeRegistry(store.ServeRegistryEntry{
			Harness:   adapterName,
			PID:       handle.PID(),
			Port:      port,
			URL:       url,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}
	a.log().Info("opencode serve started", "pid", handle.PID(), "url", url)
	return nil
}

func defaultServeURLResolver(handle ProcessHandle) (string, int, error) {
	_ = handle
	return "http://127.0.0.1:4096", 4096, nil
}

func (a *Adapter) get(ctx context.Context, path string) (*http.Response, error) {
	return a.do(ctx, http.MethodGet, path, nil)
}

func (a *Adapter) post(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return a.do(ctx, http.MethodPost, path, body)
}

func (a *Adapter) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	a.mu.Lock()
	base := a.baseURL
	a.mu.Unlock()
	if base == "" {
		return nil, fmt.Errorf("opencode serve not running")
	}
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := a.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func (a *Adapter) subscribeEvents(ctx context.Context) (io.ReadCloser, error) {
	a.mu.Lock()
	base := a.baseURL
	a.mu.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/event", nil)
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
	return resp.Body, nil
}

type opencodeSession struct {
	ID string `json:"id"`
}

type messageResponse struct {
	Parts []part `json:"parts"`
}

type part struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type providersResponse struct {
	Providers []providerEntry `json:"providers"`
}

type providerEntry struct {
	ID     string       `json:"id"`
	Models []modelEntry `json:"models"`
}

type modelEntry struct {
	ID string `json:"id"`
}

func extractText(parts []part) string {
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "text" && p.Text != "" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func parseEventDelta(evt map[string]any, sessionID string) (text string, done bool) {
	if t, _ := evt["type"].(string); t == "session.idle" || t == "message.done" {
		if sid, _ := evt["sessionID"].(string); sid == "" || sid == sessionID {
			return "", true
		}
	}
	if p, ok := evt["part"].(map[string]any); ok {
		if pt, _ := p["type"].(string); pt == "text" {
			if s, _ := p["text"].(string); s != "" {
				return s, false
			}
		}
	}
	return "", false
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

var _ harness.HarnessAdapter = (*Adapter)(nil)
var _ harness.ModelLister = (*Adapter)(nil)
