package cycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// ArchiveOptions controls OpenSpec coupling for hero cycle archive (ADR-023).
type ArchiveOptions struct {
	Force          bool
	SkipOpenspec   bool   // alias of Force when OpenSpec archive fails
	OpenspecChange string // override for this invocation
}

// OpenspecRunner runs `openspec archive <name> -y`. Injectable for tests.
type OpenspecRunner func(ctx context.Context, name string) error

// ErrMultipleOpenspecChanges is returned when more than one active OpenSpec change
// exists and no name is stored or overridden.
var ErrMultipleOpenspecChanges = errors.New("multiple active OpenSpec changes")

// ErrOpenspecArchiveFailed is returned when OpenSpec archive fails and force is not set.
var ErrOpenspecArchiveFailed = errors.New("openspec archive failed")

// OpenspecExec hooks LookPath and command execution for the default runner.
type OpenspecExec struct {
	LookPath func(string) (string, error)
	Run      func(ctx context.Context, binary string, args ...string) error
}

func (s *Service) openspecRunner() OpenspecRunner {
	if s != nil && s.OpenspecRunner != nil {
		return s.OpenspecRunner
	}
	execHooks := OpenspecExec{}
	if s != nil && s.OpenspecExec.LookPath != nil {
		execHooks.LookPath = s.OpenspecExec.LookPath
	}
	if s != nil && s.OpenspecExec.Run != nil {
		execHooks.Run = s.OpenspecExec.Run
	}
	return defaultOpenspecRunner(execHooks)
}

func defaultOpenspecRunner(hooks OpenspecExec) OpenspecRunner {
	lookPath := hooks.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	run := hooks.Run
	if run == nil {
		run = func(ctx context.Context, binary string, args ...string) error {
			cmd := exec.CommandContext(ctx, binary, args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}
	return func(ctx context.Context, name string) error {
		binary, err := lookPath("openspec")
		if err != nil {
			return fmt.Errorf("openspec binary not found on PATH: %w", err)
		}
		slog.Info("running openspec archive", "change", name)
		if err := run(ctx, binary, "archive", name, "-y"); err != nil {
			return fmt.Errorf("openspec archive %s -y: %w", name, err)
		}
		return nil
	}
}

// ManualOpenspecArchiveCommand returns the manual recovery command for a change name.
func ManualOpenspecArchiveCommand(name string) string {
	return fmt.Sprintf("openspec archive %s -y", name)
}

// ManualOpenspecArchiveInstructions returns user-facing recovery text.
func ManualOpenspecArchiveInstructions(name string) string {
	return fmt.Sprintf(
		"OpenSpec archive did not complete. Run manually:\n  %s",
		ManualOpenspecArchiveCommand(name),
	)
}

func (s *Service) resolveOpenspecChangeName(c *store.Cycle, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if c.OpenspecChange != "" {
		return c.OpenspecChange, nil
	}
	active, err := listActiveOpenspecChanges(s.ProjectDir)
	if err != nil {
		return "", err
	}
	switch len(active) {
	case 0:
		return "", nil
	case 1:
		return active[0], nil
	default:
		sort.Strings(active)
		return "", fmt.Errorf(
			"%w (%s): set one with `hero cycle openspec-change <name>` or pass --openspec-change",
			ErrMultipleOpenspecChanges,
			strings.Join(active, ", "),
		)
	}
}

func listActiveOpenspecChanges(projectDir string) ([]string, error) {
	root := filepath.Join(projectDir, "openspec", "changes")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list openspec changes: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "archive" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}
