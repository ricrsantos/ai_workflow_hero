package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

type tuiRestartMsg struct{}

// restartHeroProcess is replaced by tests. Exec keeps the current terminal,
// argv, environment, and process identity while loading the newly installed
// Hero binary.
var restartHeroProcess = func() error {
	executable := os.Args[0]
	if resolved, err := exec.LookPath(executable); err == nil {
		executable = resolved
	} else if !filepath.IsAbs(executable) {
		absolute, absErr := filepath.Abs(executable)
		if absErr != nil {
			return err
		}
		executable = absolute
	}
	return syscall.Exec(executable, os.Args, os.Environ())
}
