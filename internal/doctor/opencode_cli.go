package doctor

import (
	"os/exec"

	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

func addOpenCodeCLIChecks(projectDir string, addCheck func(name, status, message string)) {
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		return
	}
	if !install.IsHarnessEnabled(hero, "opencode") {
		return
	}
	if _, err := exec.LookPath("opencode"); err != nil {
		addCheck("opencode-cli", "warn", "opencode CLI not on PATH — OpenCode harness will be unavailable until installed")
		return
	}
	addCheck("opencode-cli", "ok", "opencode CLI available on PATH")
}
