//go:build unix

package codex

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func processCommandLine(pid int) (string, error) {
	if pid <= 0 {
		return "", fmt.Errorf("invalid pid")
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err == nil {
		return strings.ReplaceAll(string(data), "\x00", " "), nil
	}
	out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "args=").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
