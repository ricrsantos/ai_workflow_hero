package cycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/engine"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// Service is the shared façade used by CLI commands and the TUI.
type Service struct {
	ProjectDir string
	// WorkDir is the Execute workspace for free-chat (`hero chat`). Empty means
	// use ProjectDir (normal project TUI).
	WorkDir string
	Store   *store.Store
	Engine  *engine.Engine
	// Harness is optional; defaults to the Cursor adapter for hero run.
	Harness harness.HarnessAdapter
	// Registry resolves multi-harness adapters for TUI (Hero 2.0).
	Registry harnessmgr.Registry
	// OpenspecRunner runs `openspec archive <name> -y` before Hero archive; inject for tests.
	OpenspecRunner OpenspecRunner
	// OpenspecExec configures LookPath/exec for the default OpenspecRunner when OpenspecRunner is nil.
	OpenspecExec OpenspecExec
}

// ExecuteDir returns the workspace directory for harness Execute calls.
func (s *Service) ExecuteDir() string {
	if s == nil {
		return ""
	}
	if strings.TrimSpace(s.WorkDir) != "" {
		return s.WorkDir
	}
	return s.ProjectDir
}

// OpenService discovers the project root (or uses projectDir), ensures the
// operational SQLite store exists (creating/migrating and importing legacy
// cycle markdown when needed), and returns a ready Service.
func OpenService(projectDir string) (*Service, error) {
	if projectDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("could not determine current directory: %w", err)
		}
		projectDir = wd
	}
	root, err := FindProjectRoot(projectDir)
	if err != nil {
		return nil, err
	}
	st, _, err := EnsureOperationalStore(root)
	if err != nil {
		return nil, err
	}
	return &Service{
		ProjectDir: root,
		Store:      st,
		Engine:     engine.New(st),
		Registry:   harnessmgr.NewRegistry(root, st),
	}, nil
}

// Close closes the underlying store.
func (s *Service) Close() error {
	if s == nil || s.Store == nil {
		return nil
	}
	return s.Store.Close()
}

// FindProjectRoot walks up from start looking for .workflow-hero/.
func FindProjectRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		hero := filepath.Join(dir, cursoradapter.HeroDir)
		if fi, err := os.Stat(hero); err == nil && fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotInstalled
		}
		dir = parent
	}
}

// ErrNotInstalled is returned when no Hero project is found.
var ErrNotInstalled = errors.New("Hero is not installed in this project — run: hero install and select harnesses interactively, or enable them later in the TUI with /hero-harness")

// StatusView is the JSON/table shape for hero status.
type StatusView struct {
	CycleNumber    int           `json:"cycleNumber,omitempty"`
	Title          string        `json:"title,omitempty"`
	Objective      string        `json:"objective,omitempty"`
	Status         string        `json:"status,omitempty"`
	OpenspecChange string        `json:"openspec_change,omitempty"`
	Stages         []StatusStage `json:"stages"`
}

// StatusStage is one row in the status view.
type StatusStage struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	Iteration     string `json:"iteration"`
	HumanApproval string `json:"humanApproval"`
}

// Status returns the active cycle stage machine view.
func (s *Service) Status() (StatusView, error) {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		if errors.Is(err, store.ErrNoActiveCycle) {
			return StatusView{Stages: nil}, nil
		}
		return StatusView{}, err
	}
	stages, err := s.Store.ListStages(c.ID)
	if err != nil {
		return StatusView{}, err
	}
	view := StatusView{
		CycleNumber:    c.Number,
		Title:          c.Title,
		Objective:      c.Objective,
		Status:         c.Status,
		OpenspecChange: c.OpenspecChange,
	}
	for _, st := range stages {
		approval := "N/A"
		if st.RequireHumanApproval {
			approval = "Required"
			if st.Status == store.StagePendingApproval {
				approval = "Pending"
			} else if st.Status == store.StageCompleted {
				approval = "Approved"
			}
		} else if st.Status == store.StageCompleted {
			approval = "Auto"
		}
		view.Stages = append(view.Stages, StatusStage{
			Name:          displayStageName(st.Name),
			Status:        st.Status,
			Iteration:     fmt.Sprintf("%d/%d", st.Iteration, st.EffectiveMaxIterations()),
			HumanApproval: approval,
		})
	}
	return view, nil
}

// SessionCycle returns the cycle that owns the TUI session timer. Active is
// preferred; after /hero-finish or /hero-cancel the latest cycle is retained
// until /hero-archive resets the timer.
func (s *Service) SessionCycle() (*store.Cycle, error) {
	if s == nil || s.Store == nil {
		return nil, errors.New("cycle service unavailable")
	}
	active, err := s.Store.GetActiveCycle()
	if err == nil {
		return &active, nil
	}
	if !errors.Is(err, store.ErrNoActiveCycle) {
		return nil, err
	}
	cycles, err := s.Store.ListCycles()
	if err != nil {
		return nil, err
	}
	if len(cycles) == 0 || cycles[len(cycles)-1].Status == store.CycleStatusArchived {
		return nil, nil
	}
	return &cycles[len(cycles)-1], nil
}

// UpdateCycleSessionDuration persists the monotonic active-session counter.
func (s *Service) UpdateCycleSessionDuration(cycleID, seconds int64) error {
	if s == nil || s.Store == nil {
		return errors.New("cycle service unavailable")
	}
	return s.Store.UpdateCycleSessionDuration(cycleID, seconds)
}

// MetricsView is the metrics command output.
type MetricsView struct {
	CycleNumber int          `json:"cycleNumber"`
	Title       string       `json:"title"`
	Rows        []MetricsRow `json:"rows"`
	TotalIn     int64        `json:"totalInputTokens"`
	TotalOut    int64        `json:"totalOutputTokens"`
	TotalCost   float64      `json:"totalCostUSD"`
}

// MetricsRow is one metrics line.
type MetricsRow struct {
	Stage        string  `json:"stage"`
	Agent        string  `json:"agent"`
	Model        string  `json:"model"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CostUSD      float64 `json:"costUSD"`
	DurationMS   int64   `json:"durationMS"`
}

// Metrics returns per-stage metrics for the active cycle.
func (s *Service) Metrics() (MetricsView, error) {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return MetricsView{}, err
	}
	rows, err := s.Store.ListMetrics(c.ID)
	if err != nil {
		return MetricsView{}, err
	}
	view := MetricsView{CycleNumber: c.Number, Title: c.Title}
	for _, m := range rows {
		view.Rows = append(view.Rows, MetricsRow{
			Stage: m.StageName, Agent: m.Agent, Model: m.Model,
			InputTokens: m.InputTokens, OutputTokens: m.OutputTokens,
			CostUSD: m.CostUSD, DurationMS: m.DurationMS,
		})
		view.TotalIn += m.InputTokens
		view.TotalOut += m.OutputTokens
		view.TotalCost += m.CostUSD
	}
	return view, nil
}

// EventsView wraps event listing.
type EventsView struct {
	CycleNumber int           `json:"cycleNumber"`
	Events      []store.Event `json:"events"`
}

// Events lists recent events (optional type filter).
func (s *Service) Events(eventType string, limit int) (EventsView, error) {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return EventsView{}, err
	}
	ev, err := s.Store.ListEvents(c.ID, eventType, limit)
	if err != nil {
		return EventsView{}, err
	}
	return EventsView{CycleNumber: c.Number, Events: ev}, nil
}

// Approve applies approval with optional metrics JSON.
func (s *Service) Approve(summary, metricsJSON string) error {
	metrics, err := engine.ParseMetricsJSON(metricsJSON)
	if err != nil {
		return err
	}
	return mapBusy(s.Engine.Approve(lockHolder(), summary, metrics))
}

// Reject rejects the pending stage.
func (s *Service) Reject(reason string) error {
	return mapBusy(s.Engine.Reject(lockHolder(), reason))
}

// Cancel cancels the active cycle.
func (s *Service) Cancel(reason string) error {
	return mapBusy(s.Engine.Cancel(lockHolder(), reason))
}

// Finish finishes the active cycle.
func (s *Service) Finish(metricsJSON string) error {
	metrics, err := engine.ParseMetricsJSON(metricsJSON)
	if err != nil {
		return err
	}
	return mapBusy(s.Engine.Finish(lockHolder(), metrics))
}

// Continue grants extra iterations.
func (s *Service) Continue(extra int) error {
	return mapBusy(s.Engine.Continue(lockHolder(), extra))
}

// NewCycle creates a cycle from workflow-config.yml with title/objective from the file (or overrides).
func (s *Service) NewCycle(title, objective string) (engine.NewCycleResult, error) {
	return s.Engine.CreateCycleFromConfig(engine.NewCycleOptions{
		ProjectDir: s.ProjectDir,
		Title:      title,
		Objective:  objective,
	})
}

// PrepareCycle creates an active cycle with empty title/objective; stages come from workflow-config.yml.
// Called when /hero-new finishes preparing the config file.
func (s *Service) PrepareCycle() (engine.NewCycleResult, error) {
	res, err := s.Engine.CreateCycleFromConfig(engine.NewCycleOptions{
		ProjectDir: s.ProjectDir,
		DeferMeta:  true,
	})
	if err != nil {
		return res, err
	}
	if err := syncProjectWorkflowCycle(s.ProjectDir, res.Cycle.Number); err != nil {
		return res, err
	}
	return res, nil
}

// SyncCycleConfig updates the active cycle title/objective and still-open stage
// budgets from workflow-config.yml. Called by /hero-start before stage orchestration.
func (s *Service) SyncCycleConfig() error {
	return s.Engine.SyncCycleConfigFromWorkflow(s.ProjectDir)
}

// SetOpenspecChange persists the OpenSpec change directory name on the active cycle.
// Pass an empty name to clear (same as ClearOpenspecChange).
func (s *Service) SetOpenspecChange(name string) error {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	slog.Info("setting openspec_change", "cycle", c.Number, "name", name)
	return s.Store.SetOpenspecChange(c.ID, name)
}

// ClearOpenspecChange clears the OpenSpec change name on the active cycle.
func (s *Service) ClearOpenspecChange() error {
	return s.SetOpenspecChange("")
}

// ArchiveResult describes filesystem + store archive outcome.
type ArchiveResult struct {
	CycleNumber    int
	ArchiveDir     string
	OpenspecChange string // resolved change name (empty when OpenSpec step skipped)
	OpenspecForced bool   // true when Hero archived after OpenSpec failure with --force
}

// Archive moves the current cycle directory to archive/C<N>-<date>-<slug>
// using completed_at from the store, and marks the cycle archived.
// OpenSpec archive runs first when a change name is resolved (stored name,
// single active change, or --openspec-change override); see ArchiveWithOptions.
func (s *Service) Archive() (ArchiveResult, error) {
	return s.ArchiveWithOptions(ArchiveOptions{})
}

// ArchiveWithOptions archives the cycle after optional OpenSpec archive (ADR-023).
func (s *Service) ArchiveWithOptions(opts ArchiveOptions) (ArchiveResult, error) {
	c, err := s.resolveArchiveCycle()
	if err != nil {
		return ArchiveResult{}, err
	}

	name, err := s.resolveOpenspecChangeName(c, opts.OpenspecChange)
	if err != nil {
		return ArchiveResult{}, err
	}

	forced := opts.Force || opts.SkipOpenspec
	result := ArchiveResult{OpenspecChange: name}

	if name != "" && openspecChangeActive(s.ProjectDir, name) {
		if err := s.openspecRunner()(context.Background(), name); err != nil {
			if !forced {
				return ArchiveResult{}, fmt.Errorf("%w: %v\n\n%s", ErrOpenspecArchiveFailed, err, ManualOpenspecArchiveInstructions(name))
			}
			slog.Info("openspec archive failed; forcing hero archive", "change", name, "error", err)
			result.OpenspecForced = true
		}
	} else if name != "" {
		slog.Info("openspec change already archived or missing; skipping openspec CLI", "change", name)
	}

	heroResult, err := s.archiveHeroCycle(c)
	if err != nil {
		return ArchiveResult{}, err
	}
	result.CycleNumber = heroResult.CycleNumber
	result.ArchiveDir = heroResult.ArchiveDir
	return result, nil
}

func (s *Service) resolveArchiveCycle() (*store.Cycle, error) {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		// Also allow archiving a completed (non-active) latest cycle.
		cycles, listErr := s.Store.ListCycles()
		if listErr != nil || len(cycles) == 0 {
			return nil, err
		}
		c = cycles[len(cycles)-1]
		if c.Status == store.CycleStatusArchived {
			return nil, fmt.Errorf("cycle C%d is already archived", c.Number)
		}
	}
	return &c, nil
}

func (s *Service) archiveHeroCycle(c *store.Cycle) (ArchiveResult, error) {
	completed := c.CompletedAt
	if completed == "" {
		completed = time.Now().UTC().Format(time.RFC3339)
	}
	date := completed
	if t, err := time.Parse(time.RFC3339, completed); err == nil {
		date = t.Format("2006-01-02")
	} else if len(completed) >= 10 {
		date = completed[:10]
	}

	slug := slugify(c.Title)
	if slug == "" {
		slug = "cycle"
	}
	name := fmt.Sprintf("C%d-%s-%s", c.Number, date, slug)
	current := filepath.Join(s.ProjectDir, cursoradapter.HeroCurrentCycleDir)
	archiveRoot := filepath.Join(s.ProjectDir, cursoradapter.HeroCyclesDir, "archive")
	dest := filepath.Join(archiveRoot, name)

	if err := os.MkdirAll(archiveRoot, 0o755); err != nil {
		return ArchiveResult{}, err
	}
	if _, err := os.Stat(current); err == nil {
		if err := os.Rename(current, dest); err != nil {
			return ArchiveResult{}, fmt.Errorf("move current cycle: %w", err)
		}
		// Recreate empty current for next cycle.
		if err := os.MkdirAll(current, 0o755); err != nil {
			return ArchiveResult{}, err
		}
	}
	if err := s.Store.UpdateCycleStatus(c.ID, store.CycleStatusArchived, c.CompletedAt); err != nil {
		return ArchiveResult{}, err
	}
	slog.Info("hero cycle archived", "cycle", c.Number, "dir", dest)
	return ArchiveResult{CycleNumber: c.Number, ArchiveDir: dest}, nil
}

// Resume reactivates a cancelled/completed cycle by number (or latest non-archived).
func (s *Service) Resume(cycleNumber int) error {
	cycles, err := s.Store.ListCycles()
	if err != nil {
		return err
	}
	var target *store.Cycle
	for i := range cycles {
		c := &cycles[i]
		if cycleNumber > 0 {
			if c.Number == cycleNumber {
				target = c
				break
			}
			continue
		}
		if c.Status != store.CycleStatusArchived && c.Status != store.CycleStatusActive {
			target = c
		}
	}
	if target == nil && cycleNumber == 0 {
		for i := len(cycles) - 1; i >= 0; i-- {
			if cycles[i].Status != store.CycleStatusArchived {
				target = &cycles[i]
				break
			}
		}
	}
	if target == nil {
		return fmt.Errorf("no cycle to resume")
	}
	// Demote other active cycles.
	for _, c := range cycles {
		if c.Status == store.CycleStatusActive && c.ID != target.ID {
			_ = s.Store.UpdateCycleStatus(c.ID, store.CycleStatusArchived, c.CompletedAt)
		}
	}
	return s.Store.UpdateCycleStatus(target.ID, store.CycleStatusActive, "")
}

// StartStage starts a named stage on the active cycle.
func (s *Service) StartStage(name string) error {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	return s.Engine.StartStage(c.ID, name)
}

// RetryFailedStage explicitly requeues a failed stage after configuration has
// been saved and synchronized. It is intentionally a service API only; no
// free-form Cobra command is exposed.
func (s *Service) RetryFailedStage(name string) error {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	return s.Engine.RetryFailedStage(c.ID, name)
}

// CloseStage closes a running stage (used by Runtime / tests).
func (s *Service) CloseStage(name string, summary string, metricsJSON string, failed bool) error {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	metrics, err := engine.ParseMetricsJSON(metricsJSON)
	if err != nil {
		return err
	}
	return s.Engine.CloseStage(c.ID, name, engine.StageCloseInput{
		Summary: summary, Metrics: metrics, Failed: failed,
	})
}

// AccumulateStageHarnessMetrics adds turn token usage onto the active cycle's
// metrics row for stage+agent. No-op when there is no active cycle.
func (s *Service) AccumulateStageHarnessMetrics(stageName, agent, model string, usage harness.Usage, duration time.Duration) error {
	if s == nil || s.Engine == nil || s.Store == nil {
		return nil
	}
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return nil // freechat / no cycle — skip silently
	}
	ms := duration.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	return s.Engine.AccumulateStageMetrics(c.ID, stageName, agent, model, usage, ms)
}

// RunResult is the outcome of hero run dispatch.
type RunResult struct {
	Stage      string
	Dispatched bool
	Message    string
}

// RunOptions configures harness dispatch for hero run.
type RunOptions struct {
	Stage string
	Model string
	Mode  string
}

// Run dispatches stage execution via the harness adapter and records harness_invoked.
func (s *Service) Run(stage string) (RunResult, error) {
	return s.RunWith(RunOptions{Stage: stage})
}

// RunWith dispatches stage execution with optional harness model/mode.
func (s *Service) RunWith(opts RunOptions) (RunResult, error) {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return RunResult{}, err
	}
	stageName, err := s.resolveRunStage(c.ID, opts.Stage)
	if err != nil {
		return RunResult{}, err
	}

	adapter := s.Harness
	if adapter == nil {
		adapter = cursoradapter.NewAdapter(s.ProjectDir)
	}

	req := harness.DispatchRequest{
		ProjectDir: s.ProjectDir,
		CycleID:    c.ID,
		StageName:  stageName,
		Prompt:     runPrompt(c.Number, stageName),
		Model:      opts.Model,
		Mode:       opts.Mode,
	}
	result, err := adapter.Dispatch(context.Background(), req)
	if err != nil {
		return RunResult{}, err
	}
	if err := s.RecordHarnessInvoked(stageName, result.Message); err != nil {
		return RunResult{}, err
	}
	return RunResult{
		Stage:      stageName,
		Dispatched: result.Dispatched,
		Message:    result.Message,
	}, nil
}

func (s *Service) resolveRunStage(cycleID int64, stage string) (string, error) {
	if stage != "" {
		if _, err := s.Store.GetStage(cycleID, stage); err != nil {
			return "", fmt.Errorf("unknown stage %q", stage)
		}
		return stage, nil
	}
	stages, err := s.Store.ListStages(cycleID)
	if err != nil {
		return "", err
	}
	for _, prefer := range []string{store.StageRunning, store.StageEscalated, store.StagePendingApproval, store.StageWaiting} {
		for _, st := range stages {
			if st.Status == prefer {
				return st.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no stage available to dispatch")
}

func runPrompt(cycleNumber int, stage string) string {
	return fmt.Sprintf("Hero cycle C%d stage %q — continue in Cursor chat with /hero-start if not already running.", cycleNumber, stage)
}

// ActiveRunStage returns the name of the stage that should receive harness work
// (running, escalated, pending approval, or waiting — same precedence as Run).
func (s *Service) ActiveRunStage() (string, error) {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return "", err
	}
	return s.resolveRunStage(c.ID, "")
}

// ActiveStage returns the store row for ActiveRunStage.
func (s *Service) ActiveStage() (store.Stage, error) {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return store.Stage{}, err
	}
	name, err := s.resolveRunStage(c.ID, "")
	if err != nil {
		return store.Stage{}, err
	}
	return s.Store.GetStage(c.ID, name)
}

// StageHarnessSessionID returns the stored harness session id for a stage.
func (s *Service) StageHarnessSessionID(stageName string) (string, error) {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return "", err
	}
	st, err := s.Store.GetStage(c.ID, stageName)
	if err != nil {
		return "", err
	}
	return st.HarnessSessionID, nil
}

// SetStageHarnessSessionID persists a harness session id for the active cycle stage.
func (s *Service) SetStageHarnessSessionID(stageName, sessionID string) error {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	return s.Store.SetStageHarnessSessionID(c.ID, stageName, sessionID)
}

// SetStageHarnessID persists the harness adapter id used for a stage (multi-harness routing).
func (s *Service) SetStageHarnessID(stageName, harnessID string) error {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	return s.Store.SetStageHarnessID(c.ID, stageName, harnessID)
}

// StageHarnessID returns the harness id bound to a stage, or empty when unset.
func (s *Service) StageHarnessID(stageName string) (string, error) {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return "", err
	}
	return s.Store.StageHarnessID(c.ID, stageName)
}

// ConversationContext returns the active run stage and its harness session id.
func (s *Service) ConversationContext() (stageName, sessionID string, err error) {
	stageName, err = s.ActiveRunStage()
	if err != nil {
		return "", "", err
	}
	sessionID, err = s.StageHarnessSessionID(stageName)
	if err != nil {
		return "", "", err
	}
	return stageName, sessionID, nil
}

// RecordHarnessInvoked appends a harness_invoked event.
func (s *Service) RecordHarnessInvoked(stage, message string) error {
	c, err := s.Store.GetActiveCycle()
	if err != nil {
		return err
	}
	payload := fmt.Sprintf(`{"stage":%q,"message":%q}`, stage, message)
	_, err = s.Store.AppendEvent(store.Event{
		CycleID: c.ID, Type: store.EventHarnessInvoked, PayloadJSON: payload,
	})
	return err
}

func mapBusy(err error) error {
	if errors.Is(err, store.ErrBusy) {
		return fmt.Errorf("cycle is locked by another session; wait or clear the lock")
	}
	return err
}

func lockHolder() string {
	return fmt.Sprintf("cli-%d", os.Getpid())
}

func displayStageName(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = out[:40]
		out = strings.Trim(out, "-")
	}
	return out
}
