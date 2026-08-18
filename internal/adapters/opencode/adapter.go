package opencode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
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

var serveListenLine = regexp.MustCompile(`listening on (https?://[^\s]+)`)

type execHandle struct {
	cmd      *exec.Cmd
	baseURL  string
	port     int
	urlErr   error
	urlReady chan struct{}
}

func (ExecRunner) Start(ctx context.Context, dir, name string, args ...string) (ProcessHandle, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	h := &execHandle{cmd: cmd, urlReady: make(chan struct{})}
	go h.scanServeOutput(io.MultiReader(stdout, stderr))
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *execHandle) scanServeOutput(r io.Reader) {
	defer close(h.urlReady)
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		m := serveListenLine.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		u, err := url.Parse(m[1])
		if err != nil {
			h.urlErr = fmt.Errorf("parse opencode serve url: %w", err)
			return
		}
		port, _ := strconv.Atoi(u.Port())
		h.baseURL = strings.TrimRight(u.String(), "/")
		h.port = port
		return
	}
	if err := scanner.Err(); err != nil && h.urlErr == nil {
		h.urlErr = err
	}
	if h.baseURL == "" && h.urlErr == nil {
		h.urlErr = fmt.Errorf("opencode serve exited without listening URL")
	}
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

	mu           sync.Mutex
	serveStartMu sync.Mutex
	baseURL      string
	servePID     int
	servePort    int
	sessions     map[string]*sessionState
	cancels      map[string]context.CancelFunc
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
	if err := httpOK(resp); err != nil {
		return nil, err
	}
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
	if modelObj := modelPayload(model); modelObj != nil {
		payload["model"] = modelObj
	}
	// C5: normalized fs/th/ef values map to native OpenCode option keys here,
	// inside the adapter (ADR-041); the TUI never builds provider payloads.
	if opts := nativePropertyOptions(req.Properties); opts != nil {
		payload["options"] = opts
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, rejectionFromBody(resp, model, req.Properties)
	}
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, rejectionFromBody(resp, strings.TrimSpace(req.Model), req.Properties)
	}
	resp.Body.Close()

	var buf strings.Builder
	partTexts := make(map[string]string)
	assistantMsgID := ""
	events, err := a.subscribeEvents(ctx)
	if err != nil {
		return nil, err
	}
	defer events.Close()

	err = readSSEEvents(ctx, events, func(evt map[string]any) error {
		var text string
		var done bool
		text, done, assistantMsgID = parseEventDelta(evt, sessionID, partTexts, assistantMsgID)
		if text != "" && req.OnStreamDelta != nil {
			req.OnStreamDelta(harness.StreamDelta{Kind: harness.StreamKindText, Text: text})
			buf.WriteString(text)
		}
		if done {
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, err
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
	if err := httpOK(resp); err != nil {
		return nil, err
	}
	var data providersResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	var models []string
	for _, p := range data.Providers {
		for modelID, meta := range p.Models {
			id := strings.TrimSpace(modelID)
			if mid := strings.TrimSpace(meta.ID); mid != "" {
				id = mid
			}
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

// StopServe stops managed serve children and clears registry rows.
func (a *Adapter) StopServe(ctx context.Context) error {
	a.mu.Lock()
	pid := a.servePID
	a.baseURL = ""
	a.servePID = 0
	a.servePort = 0
	a.mu.Unlock()

	if pid > 0 {
		killProcess(pid)
	}

	if a.Store != nil {
		entries, err := a.Store.ListServeRegistry()
		if err == nil {
			for _, e := range entries {
				if e.Harness != adapterName || e.PID <= 0 {
					continue
				}
				if e.PID != pid {
					killProcess(e.PID)
				}
			}
		}
		_ = a.Store.ClearServeRegistry()
	}
	return nil
}

// KillProcess terminates a child process by PID (cross-platform).
func KillProcess(pid int) {
	killProcess(pid)
}

func killProcess(pid int) {
	if pid <= 0 {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Kill()
}

// ServeURLAlive reports whether an opencode serve base URL responds.
func ServeURLAlive(ctx context.Context, baseURL string) bool {
	return serveURLAlive(ctx, baseURL, http.DefaultClient)
}

func serveURLAlive(ctx context.Context, baseURL string, client HTTPDoer) bool {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false
	}
	if client == nil {
		client = http.DefaultClient
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/config/providers", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

func (a *Adapter) ensureServe(ctx context.Context) error {
	a.mu.Lock()
	if a.baseURL != "" {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	a.serveStartMu.Lock()
	defer a.serveStartMu.Unlock()

	a.mu.Lock()
	if a.baseURL != "" {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	if a.adoptFromRegistry(ctx) {
		return nil
	}

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

func (a *Adapter) adoptFromRegistry(ctx context.Context) bool {
	if a.Store == nil {
		return false
	}
	entries, err := a.Store.ListServeRegistry()
	if err != nil {
		return false
	}
	var latest *store.ServeRegistryEntry
	for i := range entries {
		e := &entries[i]
		if e.Harness == adapterName && strings.TrimSpace(e.URL) != "" {
			latest = e
		}
	}
	if latest == nil {
		return false
	}
	if !serveURLAlive(ctx, latest.URL, a.HTTP) {
		_ = a.Store.DeleteServeRegistry(latest.ID)
		return false
	}

	a.mu.Lock()
	a.baseURL = latest.URL
	a.servePID = latest.PID
	a.servePort = latest.Port
	a.mu.Unlock()
	a.log().Info("adopted existing opencode serve", "pid", latest.PID, "url", latest.URL)
	return true
}

func defaultServeURLResolver(handle ProcessHandle) (string, int, error) {
	eh, ok := handle.(*execHandle)
	if !ok {
		return "", 0, fmt.Errorf("resolve opencode serve url: unexpected process handle")
	}
	select {
	case <-eh.urlReady:
	case <-time.After(30 * time.Second):
		return "", 0, fmt.Errorf("timeout waiting for opencode serve to listen")
	}
	if eh.urlErr != nil {
		return "", 0, eh.urlErr
	}
	if eh.baseURL == "" {
		return "", 0, fmt.Errorf("opencode serve did not publish a listen URL")
	}
	return eh.baseURL, eh.port, nil
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
	if err := httpOK(resp); err != nil {
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
	ID     string               `json:"id"`
	Models map[string]modelMeta `json:"models"`
}

type modelMeta struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
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

func parseEventDelta(evt map[string]any, sessionID string, partTexts map[string]string, assistantMsgID string) (text string, done bool, nextAssistantMsgID string) {
	nextAssistantMsgID = assistantMsgID
	evtType, _ := evt["type"].(string)
	props, _ := evt["properties"].(map[string]any)
	if sid, _ := props["sessionID"].(string); sid != "" && sid != sessionID {
		return "", false, nextAssistantMsgID
	}

	switch evtType {
	case "session.idle":
		return "", true, nextAssistantMsgID
	case "session.status":
		if st, ok := props["status"].(map[string]any); ok {
			if t, _ := st["type"].(string); t == "idle" {
				return "", true, nextAssistantMsgID
			}
		}
	case "message.updated":
		if info, ok := props["info"].(map[string]any); ok {
			if role, _ := info["role"].(string); role == "assistant" {
				if id, _ := info["id"].(string); id != "" {
					nextAssistantMsgID = id
				}
			}
		}
	case "message.part.updated":
		part, ok := props["part"].(map[string]any)
		if !ok {
			return "", false, nextAssistantMsgID
		}
		if pt, _ := part["type"].(string); pt != "text" {
			return "", false, nextAssistantMsgID
		}
		msgID, _ := part["messageID"].(string)
		// User parts arrive before message.updated(assistant); never stream until bound.
		if nextAssistantMsgID == "" {
			return "", false, nextAssistantMsgID
		}
		if msgID != "" && msgID != nextAssistantMsgID {
			return "", false, nextAssistantMsgID
		}
		full, _ := part["text"].(string)
		partID, _ := part["id"].(string)
		prev := partTexts[partID]
		if len(full) <= len(prev) {
			return "", false, nextAssistantMsgID
		}
		delta := full[len(prev):]
		partTexts[partID] = full
		return delta, false, nextAssistantMsgID
	case "message.done":
		if sid, _ := evt["sessionID"].(string); sid == "" || sid == sessionID {
			return "", true, nextAssistantMsgID
		}
	}
	return "", false, nextAssistantMsgID
}

func modelPayload(slug string) map[string]string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil
	}
	if i := strings.Index(slug, "/"); i > 0 {
		return map[string]string{
			"providerID": slug[:i],
			"modelID":    slug[i+1:],
		}
	}
	return map[string]string{"modelID": slug}
}

func readSSEEvents(ctx context.Context, body io.Reader, handler func(map[string]any) error) error {
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
			continue
		}
		if err := handler(evt); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func httpOK(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return fmt.Errorf("opencode api %s: %s", resp.Request.URL.Path, resp.Status)
	}
	return fmt.Errorf("opencode api %s: %s — %s", resp.Request.URL.Path, resp.Status, msg)
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
