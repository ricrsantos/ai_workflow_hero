package doctor

import (
	"os/exec"

	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func addCodexCLIChecks(projectDir string, addCheck func(name, status, message string)) {
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		return
	}
	if !install.IsHarnessEnabled(hero, "codex") {
		return
	}
	if _, err := exec.LookPath("codex"); err != nil {
		addCheck("codex-cli", "warn", "Codex CLI not on PATH — Codex harness will be unavailable until installed")
		return
	}
	addCheck("codex-cli", "ok", "codex CLI available on PATH")
}
