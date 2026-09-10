// Package autoupdate queues a deterministic local Hero development update.
//
// The package deliberately owns only the request side of the flow: it commits
// the current Hero source repository and atomically arms needs-update.txt. The
// systemd updater owns compilation, Hero/optional-plugin replacement, and TUI
// restart.
package autoupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// SourceEnv points at the Hero source repository used by the updater.
	SourceEnv = "HERO_UPDATE_SOURCE"
	// InstallDirEnv points at the directory containing the installed hero binary.
	InstallDirEnv = "HERO_UPDATE_INSTALL_DIR"
	// CommitMessageEnv optionally overrides the deterministic update commit message.
	CommitMessageEnv = "HERO_UPDATE_COMMIT_MESSAGE"

	DefaultInstallDir    = "/home/ricardo/installable/hero"
	DefaultCommitMessage = "chore: request Hero auto-update"
)

var ErrNoChanges = errors.New("no changes to commit")

// Config describes the source and local installation used by one update
// request. The defaults are intentionally development-oriented and can be
// overridden by environment variables for another machine.
type Config struct {
	SourceDir     string
	InstallDir    string
	StateFile     string
	BuildScript   string
	UpdaterScript string
	HeroBinary    string
	CommitMessage string
}

// ConfigForProject creates a configuration anchored at projectDir unless the
// development update environment explicitly selects another source repository.
func ConfigForProject(projectDir string) Config {
	source := strings.TrimSpace(os.Getenv(SourceEnv))
	if source == "" {
		source = projectDir
	}
	installDir := strings.TrimSpace(os.Getenv(InstallDirEnv))
	if installDir == "" {
		installDir = DefaultInstallDir
	}
	message := strings.TrimSpace(os.Getenv(CommitMessageEnv))
	if message == "" {
		message = DefaultCommitMessage
	}
	return Config{
		SourceDir:     filepath.Clean(source),
		InstallDir:    filepath.Clean(installDir),
		StateFile:     filepath.Join(installDir, "needs-update.txt"),
		BuildScript:   filepath.Join(source, "scripts", "build_update.sh"),
		UpdaterScript: filepath.Join(installDir, "hero-update.sh"),
		HeroBinary:    filepath.Join(installDir, "hero"),
		CommitMessage: message,
	}
}

// Result describes a successfully queued update.
type Result struct {
	Commit    string
	StateFile string
}

// Request commits the source repository and arms the updater flag. It never
// builds or restarts a process; those operations belong to systemd's service.
func Request(ctx context.Context, cfg Config) (Result, error) {
	cfg = withDefaults(cfg)
	if err := validateConfig(cfg); err != nil {
		return Result{}, err
	}

	rootRaw, err := runGit(ctx, cfg.SourceDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Result{}, fmt.Errorf("locate Hero git root: %w", err)
	}
	gitRoot := strings.TrimSpace(string(rootRaw))
	if gitRoot == "" {
		return Result{}, fmt.Errorf("locate Hero git root: empty path")
	}

	status, err := runGit(ctx, gitRoot, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return Result{}, fmt.Errorf("inspect Hero changes: %w", err)
	}
	if strings.TrimSpace(string(status)) == "" {
		return Result{}, ErrNoChanges
	}
	if path := sensitivePath(string(status)); path != "" {
		return Result{}, fmt.Errorf("refusing to commit sensitive file %q", path)
	}

	if _, err := runGit(ctx, gitRoot, "add", "-A", "--", "."); err != nil {
		return Result{}, fmt.Errorf("stage Hero changes: %w", err)
	}
	cached, err := runGit(ctx, gitRoot, "diff", "--cached", "--name-only", "--")
	if err != nil {
		return Result{}, fmt.Errorf("inspect staged Hero changes: %w", err)
	}
	if path := sensitivePath(string(cached)); path != "" {
		return Result{}, fmt.Errorf("refusing to commit sensitive file %q", path)
	}
	if strings.TrimSpace(string(cached)) == "" {
		return Result{}, ErrNoChanges
	}

	if _, err := runGit(ctx, gitRoot, "commit", "-m", cfg.CommitMessage); err != nil {
		return Result{}, fmt.Errorf("commit Hero changes: %w", err)
	}
	commit, err := runGit(ctx, gitRoot, "rev-parse", "--short", "HEAD")
	if err != nil {
		return Result{}, fmt.Errorf("read update commit: %w", err)
	}
	if err := writeFlag(cfg.StateFile, "true\n"); err != nil {
		return Result{}, fmt.Errorf("arm Hero updater: %w", err)
	}
	return Result{Commit: strings.TrimSpace(string(commit)), StateFile: cfg.StateFile}, nil
}

func withDefaults(cfg Config) Config {
	if strings.TrimSpace(cfg.InstallDir) == "" {
		cfg.InstallDir = DefaultInstallDir
	}
	if strings.TrimSpace(cfg.SourceDir) == "" {
		cfg.SourceDir = "."
	}
	cfg.SourceDir = filepath.Clean(cfg.SourceDir)
	cfg.InstallDir = filepath.Clean(cfg.InstallDir)
	if strings.TrimSpace(cfg.StateFile) == "" {
		cfg.StateFile = filepath.Join(cfg.InstallDir, "needs-update.txt")
	}
	if strings.TrimSpace(cfg.BuildScript) == "" {
		cfg.BuildScript = filepath.Join(cfg.SourceDir, "scripts", "build_update.sh")
	}
	if strings.TrimSpace(cfg.UpdaterScript) == "" {
		cfg.UpdaterScript = filepath.Join(cfg.InstallDir, "hero-update.sh")
	}
	if strings.TrimSpace(cfg.HeroBinary) == "" {
		cfg.HeroBinary = filepath.Join(cfg.InstallDir, "hero")
	}
	if strings.TrimSpace(cfg.CommitMessage) == "" {
		cfg.CommitMessage = DefaultCommitMessage
	}
	return cfg
}

func validateConfig(cfg Config) error {
	if info, err := os.Stat(cfg.SourceDir); err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a directory")
		}
		return fmt.Errorf("source directory %q: %w", cfg.SourceDir, err)
	}
	for label, path := range map[string]string{
		"build script":   cfg.BuildScript,
		"updater script": cfg.UpdaterScript,
		"Hero binary":    cfg.HeroBinary,
	} {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("%s %q: %w", label, path, err)
		}
		if info.IsDir() {
			return fmt.Errorf("%s %q is a directory", label, path)
		}
		if info.Mode()&0o111 == 0 {
			return fmt.Errorf("%s %q is not executable", label, path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(cfg.StateFile), 0o755); err != nil {
		return fmt.Errorf("create updater directory: %w", err)
	}
	return nil
}

func runGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, detail)
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

func sensitivePath(list string) string {
	for _, line := range strings.Split(list, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) >= 3 && line[2] == ' ' && isStatusCode(line[0]) && isStatusCode(line[1]) {
			line = strings.TrimSpace(line[2:])
		}
		if idx := strings.LastIndex(line, " -> "); idx >= 0 {
			line = strings.TrimSpace(line[idx+4:])
		}
		line = strings.Trim(line, "\"'")
		base := strings.ToLower(filepath.Base(line))
		switch {
		case base == ".env", strings.HasPrefix(base, ".env.") && base != ".env.example":
			return line
		case base == "credentials.json", base == "secrets.json":
			return line
		case strings.HasSuffix(base, ".pem"), strings.HasSuffix(base, ".key"):
			return line
		}
	}
	return ""
}

func isStatusCode(r byte) bool {
	return strings.ContainsRune(" MADRCUT?!", rune(r))
}

func writeFlag(path, value string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".needs-update-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(value); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
