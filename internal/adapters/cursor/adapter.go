package cursor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const adapterName = "cursor"

// AgentCLI is the Cursor Agent CLI binary name searched on PATH.
const AgentCLI = "cursor-agent"

// Adapter implements harness.HarnessAdapter for Cursor projects.
type Adapter struct {
	ProjectDir string
	Logger     *slog.Logger
	LookPath   func(string) (string, error)
	// Pusher attempts IDE push when the agent CLI is available. When nil, dispatch
	// falls back to chat guidance even if the CLI is present (V1 has no stable push API).
	Pusher func(ctx context.Context, agentPath string, req harness.DispatchRequest) (harness.DispatchResult, error)
	// VerifyAgent checks that agentPath responds without invoking a prompt (no LLM).
	VerifyAgent func(ctx context.Context, agentPath string) error
}

// NewAdapter returns a Cursor harness adapter for projectDir.
func NewAdapter(projectDir string) *Adapter {
	return &Adapter{
		ProjectDir: projectDir,
		Logger:     slog.Default(),
		LookPath:   exec.LookPath,
	}
}

// Name implements harness.HarnessAdapter.
func (a *Adapter) Name() string {
	return adapterName
}

// SupportsChat implements harness.HarnessAdapter.
func (a *Adapter) SupportsChat() bool {
	return a.chatReady()
}

// Dispatch implements harness.HarnessAdapter.
func (a *Adapter) Dispatch(ctx context.Context, req harness.DispatchRequest) (harness.DispatchResult, error) {
	fallback := dispatchFallbackMessage(req)
	if !a.chatReady() {
		a.log().Debug("cursor chat assets missing", "project", a.ProjectDir, "stage", req.StageName, "custom_prompt", isCustomCommandPrompt(req))
		return harness.DispatchResult{Dispatched: false, Message: fallback}, nil
	}

	lookPath := a.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	agentPath, err := lookPath(AgentCLI)
	if err != nil {
		a.log().Debug("cursor agent cli not on PATH", "binary", AgentCLI)
		return harness.DispatchResult{Dispatched: false, Message: fallback}, nil
	}

	verify := a.VerifyAgent
	if verify == nil {
		verify = defaultVerifyAgent
	}
	if err := verify(ctx, agentPath); err != nil {
		a.log().Info("cursor agent cli unavailable", "error", err)
		return harness.DispatchResult{Dispatched: false, Message: fallback}, nil
	}

	if a.Pusher != nil {
		res, err := a.Pusher(ctx, agentPath, req)
		if err != nil {
			a.log().Error("cursor dispatch failed", "stage", req.StageName, "error", err)
			return harness.DispatchResult{}, err
		}
		return res, nil
	}

	a.log().Debug("cursor push API unavailable; chat fallback", "stage", req.StageName, "custom_prompt", isCustomCommandPrompt(req))
	return harness.DispatchResult{Dispatched: false, Message: fallback}, nil
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

func defaultVerifyAgent(ctx context.Context, agentPath string) error {
	cmd := exec.CommandContext(ctx, agentPath, "--version")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// dispatchFallbackMessage returns an actionable unavailable message for stage dispatch
// or imported custom commands (expanded markdown Prompt with empty StageName). Design D3.
func dispatchFallbackMessage(req harness.DispatchRequest) string {
	if req.StageName != "" {
		return fmt.Sprintf("Dispatch unavailable for stage %s; continue via Cursor chat (/hero:start).", req.StageName)
	}
	if isCustomCommandPrompt(req) {
		return "Dispatch unavailable; run the same command in Cursor chat."
	}
	return "Dispatch unavailable; continue via Cursor chat (/hero:start)."
}

func isCustomCommandPrompt(req harness.DispatchRequest) bool {
	return strings.TrimSpace(req.Prompt) != ""
}

// Compile-time interface check.
var _ harness.HarnessAdapter = (*Adapter)(nil)
