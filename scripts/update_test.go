package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func updateScriptPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(filepath.Dir(scriptPath(t)), name)
}

func TestBuildUpdateScript_OnlyBuildsCurrentTargetHero(t *testing.T) {
	path := updateScriptPath(t, "build_update.sh")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		t.Fatalf("build_update.sh must be an executable file: %v", info.Mode())
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"GOOS=\"${GOOS:-$(go env GOOS)}\"",
		"GOARCH=\"${GOARCH:-$(go env GOARCH)}\"",
		"HERO_UPDATE_OUTPUT",
		"./cmd/hero",
		"-trimpath",
		"-X main.version=",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("build_update.sh missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"./cmd/hero-telegram-daemon",
		"hero-telegram-daemon_${VERSION}",
		"rm -rf \"${DIST}\"",
		"manifest.json",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("build_update.sh must not contain %q", forbidden)
		}
	}
	assertShellSyntax(t, path)
}

func TestHeroUpdateScriptsHaveShellSyntax(t *testing.T) {
	for _, name := range []string{"hero-update.sh", "install_update_dev.sh", "uninstall_update_dev.sh"} {
		path := updateScriptPath(t, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.IsDir() || info.Mode()&0o111 == 0 {
			t.Fatalf("%s must be an executable file: %v", name, info.Mode())
		}
		assertShellSyntax(t, path)
	}
}

func TestUninstallUpdateDevRemovesOnlyUpdaterArtifacts(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	configDir := filepath.Join(root, "config")
	unitDir := filepath.Join(configDir, "systemd", "user")
	for _, path := range []string{
		installDir,
		filepath.Join(unitDir, "timers.target.wants"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	for _, path := range []string{
		filepath.Join(installDir, "hero-update.sh"),
		filepath.Join(installDir, "needs-update.txt"),
		filepath.Join(installDir, "hero-update.lock"),
		filepath.Join(unitDir, "hero-update.service"),
		filepath.Join(unitDir, "hero-update.timer"),
		filepath.Join(unitDir, "timers.target.wants", "hero-update.timer"),
	} {
		if err := os.WriteFile(path, []byte("owned\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{filepath.Join(installDir, "hero"), filepath.Join(installDir, "hero.previous")} {
		if err := os.WriteFile(path, []byte("keep\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	fakeBin := filepath.Join(root, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeSystemctl := filepath.Join(fakeBin, "systemctl")
	if err := os.WriteFile(fakeSystemctl, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", updateScriptPath(t, "uninstall_update_dev.sh"))
	cmd.Env = append(os.Environ(),
		"HERO_UPDATE_INSTALL_DIR="+installDir,
		"XDG_CONFIG_HOME="+configDir,
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("uninstall script failed: %v\n%s", err, output)
	}

	for _, path := range []string{
		filepath.Join(installDir, "hero-update.sh"),
		filepath.Join(installDir, "needs-update.txt"),
		filepath.Join(installDir, "hero-update.lock"),
		filepath.Join(unitDir, "hero-update.service"),
		filepath.Join(unitDir, "hero-update.timer"),
		filepath.Join(unitDir, "timers.target.wants", "hero-update.timer"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("updater artifact still exists at %s: %v", path, err)
		}
	}
	for _, path := range []string{filepath.Join(installDir, "hero"), filepath.Join(installDir, "hero.previous")} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("Hero file was removed at %s: %v", path, err)
		}
	}
}

func TestHeroUpdateSystemdUnitsContract(t *testing.T) {
	service, err := os.ReadFile(filepath.Join(filepath.Dir(scriptPath(t)), "systemd", "hero-update.service.in"))
	if err != nil {
		t.Fatal(err)
	}
	timer, err := os.ReadFile(filepath.Join(filepath.Dir(scriptPath(t)), "systemd", "hero-update.timer"))
	if err != nil {
		t.Fatal(err)
	}
	serviceText := string(service)
	for _, want := range []string{
		"Type=oneshot",
		"@HERO_UPDATE_SOURCE@",
		"@HERO_UPDATE_INSTALL_DIR@/hero-update.sh",
		"TimeoutStartSec=30min",
	} {
		if !strings.Contains(serviceText, want) {
			t.Errorf("service template missing %q", want)
		}
	}
	timerText := string(timer)
	for _, want := range []string{"OnBootSec=2min", "OnUnitActiveSec=5min", "Persistent=true", "hero-update.service"} {
		if !strings.Contains(timerText, want) {
			t.Errorf("timer missing %q", want)
		}
	}
}

func assertShellSyntax(t *testing.T, path string) {
	t.Helper()
	if out, err := exec.Command("bash", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("bash -n %s: %v\n%s", path, err, out)
	}
}
