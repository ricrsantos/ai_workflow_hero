package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func updateScriptPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(filepath.Dir(scriptPath(t)), name)
}

func TestBuildUpdateScriptBuildsCurrentTargetHeroAndTelegramDaemon(t *testing.T) {
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
		"HERO_UPDATE_TELEGRAM_DAEMON_OUTPUT",
		"hero_update_${VERSION}_${GOOS}_${GOARCH}",
		"hero-telegram-daemon_update_${VERSION}_${GOOS}_${GOARCH}",
		"./cmd/hero",
		"./cmd/hero-telegram-daemon",
		"-trimpath",
		"-X main.version=",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("build_update.sh missing %q", want)
		}
	}
	for _, forbidden := range []string{
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

func TestHeroUpdateScriptSignalsInstalledHeroAndClearsState(t *testing.T) {
	if _, err := os.Stat("/proc"); err != nil {
		t.Skip("requires /proc executable discovery")
	}
	catPath, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("cat is unavailable")
	}
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true is unavailable")
	}

	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	sourceDir := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(sourceDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	homeDir := filepath.Join(root, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	heroPath := filepath.Join(installDir, "hero")
	catBinary, err := os.ReadFile(catPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(heroPath, catBinary, 0o755); err != nil {
		t.Fatal(err)
	}
	newHeroPath := writeFakeHero(t, root)
	buildScript := fakeBuildScript(newHeroPath, truePath)
	if err := os.WriteFile(filepath.Join(sourceDir, "scripts", "build_update.sh"), []byte(buildScript), 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(installDir, "needs-update.txt")
	if err := os.WriteFile(statePath, []byte("true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdin, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	tui := exec.Command(heroPath)
	tui.Stdin = stdin
	if err := tui.Start(); err != nil {
		_ = stdin.Close()
		_ = stdinWriter.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = stdinWriter.Close()
		if tui.Process != nil {
			_ = tui.Process.Kill()
		}
	})
	procExe := filepath.Join("/proc", strconv.Itoa(tui.Process.Pid), "exe")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got, err := os.Readlink(procExe); err == nil && got == heroPath {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got, err := os.Readlink(procExe); err != nil || got != heroPath {
		t.Fatalf("test TUI executable=%q err=%v want %q", got, err, heroPath)
	}

	updater := exec.Command("bash", updateScriptPath(t, "hero-update.sh"))
	updater.Env = append(os.Environ(),
		"HERO_UPDATE_SOURCE="+sourceDir,
		"HERO_UPDATE_INSTALL_DIR="+installDir,
		"HERO_UPDATE_BIN="+heroPath,
		"HERO_UPDATE_TELEGRAM_PLUGIN_DIR="+filepath.Join(homeDir, "plugins", "telegram"),
		"HOME="+homeDir,
	)
	output, err := updater.CombinedOutput()
	if err != nil {
		t.Fatalf("updater failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "Hero restart signal sent") {
		t.Fatalf("updater did not signal the TUI:\n%s", output)
	}

	waitDone := make(chan error, 1)
	go func() { waitDone <- tui.Wait() }()
	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Fatal("installed Hero TUI was not signaled")
	}
	flag, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(flag) != "false\n" {
		t.Fatalf("state=%q want false after successful installation", flag)
	}
	if _, err := os.Stat(filepath.Join(installDir, "hero.previous")); err != nil {
		t.Fatalf("previous binary was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".workflow-hero", "plugins", "telegram")); !os.IsNotExist(err) {
		t.Fatalf("normal Hero update unexpectedly installed Telegram plugin: %v", err)
	}
}

func TestHeroUpdateScriptUpdatesInstalledTelegramPlugin(t *testing.T) {
	if _, err := os.Stat("/proc"); err != nil {
		t.Skip("requires /proc executable discovery")
	}
	catPath, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("cat is unavailable")
	}
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true is unavailable")
	}

	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	sourceDir := filepath.Join(root, "source")
	homeDir := filepath.Join(root, "home")
	pluginDir := filepath.Join(homeDir, ".workflow-hero", "plugins", "telegram")
	for _, dir := range []string{installDir, filepath.Join(sourceDir, "scripts"), pluginDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	catBinary, err := os.ReadFile(catPath)
	if err != nil {
		t.Fatal(err)
	}
	heroPath := filepath.Join(installDir, "hero")
	if err := os.WriteFile(heroPath, catBinary, 0o755); err != nil {
		t.Fatal(err)
	}
	daemonPath := filepath.Join(pluginDir, "hero-telegram-daemon")
	if err := os.WriteFile(daemonPath, catBinary, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(pluginDir, "manifest.json")
	oldManifest := map[string]any{
		"name":             "telegram",
		"version":          "old",
		"protocol_version": 1,
		"daemon_path":      daemonPath,
		"installed_at":     "2026-01-01T00:00:00Z",
	}
	manifestData, err := json.Marshal(oldManifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(manifestData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	newHeroPath := writeFakeHero(t, root)
	if err := os.WriteFile(filepath.Join(sourceDir, "scripts", "build_update.sh"), []byte(fakeBuildScript(newHeroPath, truePath)), 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(installDir, "needs-update.txt")
	if err := os.WriteFile(statePath, []byte("true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tui := startBlockingProcess(t, heroPath)
	daemon := startBlockingProcess(t, daemonPath)
	waitForExecutable(t, tui.cmd, heroPath)
	waitForExecutable(t, daemon.cmd, daemonPath)

	updater := exec.Command("bash", updateScriptPath(t, "hero-update.sh"))
	updater.Env = append(os.Environ(),
		"HERO_UPDATE_SOURCE="+sourceDir,
		"HERO_UPDATE_INSTALL_DIR="+installDir,
		"HERO_UPDATE_BIN="+heroPath,
		"HERO_UPDATE_TELEGRAM_PLUGIN_DIR="+pluginDir,
		"HERO_UPDATE_TELEGRAM_DAEMON="+daemonPath,
		"HERO_UPDATE_TELEGRAM_MANIFEST="+manifestPath,
		"HOME="+homeDir,
	)
	output, err := updater.CombinedOutput()
	if err != nil {
		t.Fatalf("updater failed: %v\n%s", err, output)
	}
	outputText := string(output)
	if !strings.Contains(outputText, "Telegram plugin detected") {
		t.Fatalf("updater did not detect the installed Telegram plugin:\n%s", output)
	}
	if !strings.Contains(outputText, "Telegram daemon stop signal sent") {
		t.Fatalf("updater did not stop the old Telegram daemon:\n%s", output)
	}
	if !strings.Contains(outputText, "Hero restart signal sent") {
		t.Fatalf("updater did not restart the TUI:\n%s", output)
	}

	tui.wait(t, time.Second)
	daemon.wait(t, time.Second)
	assertUpdateState(t, statePath, "false\n")
	if _, err := os.Stat(filepath.Join(installDir, "hero.previous")); err != nil {
		t.Fatalf("previous binary was not preserved: %v", err)
	}
	if got, err := os.ReadFile(daemonPath); err != nil {
		t.Fatal(err)
	} else if want, err := os.ReadFile(truePath); err != nil {
		t.Fatal(err)
	} else if string(got) != string(want) {
		t.Fatal("installed Telegram daemon does not match the built daemon artifact")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var gotManifest map[string]any
	if err := json.Unmarshal(data, &gotManifest); err != nil {
		t.Fatal(err)
	}
	if gotManifest["version"] != "test-new" || gotManifest["daemon_path"] != daemonPath || gotManifest["protocol_version"] != float64(1) {
		t.Fatalf("updated manifest=%v", gotManifest)
	}
	assertNoUpdateTempFiles(t, installDir, pluginDir)
}

func TestHeroUpdateScriptFailureKeepsStateArmed(t *testing.T) {
	t.Run("build failure", func(t *testing.T) {
		root := t.TempDir()
		installDir := filepath.Join(root, "install")
		sourceDir := filepath.Join(root, "source")
		if err := os.MkdirAll(filepath.Join(sourceDir, "scripts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(installDir, 0o755); err != nil {
			t.Fatal(err)
		}
		heroPath := filepath.Join(installDir, "hero")
		if err := os.WriteFile(heroPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		buildPath := filepath.Join(sourceDir, "scripts", "build_update.sh")
		if err := os.WriteFile(buildPath, []byte("#!/usr/bin/env bash\nexit 42\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		statePath := filepath.Join(installDir, "needs-update.txt")
		if err := os.WriteFile(statePath, []byte("true\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		cmd := exec.Command("bash", updateScriptPath(t, "hero-update.sh"))
		cmd.Env = append(os.Environ(),
			"HERO_UPDATE_SOURCE="+sourceDir,
			"HERO_UPDATE_INSTALL_DIR="+installDir,
			"HERO_UPDATE_BIN="+heroPath,
		)
		if output, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("build failure unexpectedly succeeded:\n%s", output)
		}
		assertUpdateState(t, statePath, "true\n")
		assertNoUpdateTempFiles(t, installDir)
	})

	t.Run("installation failure", func(t *testing.T) {
		truePath, err := exec.LookPath("true")
		if err != nil {
			t.Skip("true is unavailable")
		}
		root := t.TempDir()
		installDir := filepath.Join(root, "install")
		sourceDir := filepath.Join(root, "source")
		if err := os.MkdirAll(filepath.Join(sourceDir, "scripts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(installDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// A directory passes the executable check but cannot be copied as the
		// existing Hero binary, so the install step fails after the build.
		heroPath := filepath.Join(installDir, "hero")
		if err := os.Mkdir(heroPath, 0o755); err != nil {
			t.Fatal(err)
		}
		newHeroPath := writeFakeHero(t, root)
		buildScript := fakeBuildScript(newHeroPath, truePath)
		if err := os.WriteFile(filepath.Join(sourceDir, "scripts", "build_update.sh"), []byte(buildScript), 0o755); err != nil {
			t.Fatal(err)
		}
		statePath := filepath.Join(installDir, "needs-update.txt")
		if err := os.WriteFile(statePath, []byte("true\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		cmd := exec.Command("bash", updateScriptPath(t, "hero-update.sh"))
		cmd.Env = append(os.Environ(),
			"HERO_UPDATE_SOURCE="+sourceDir,
			"HERO_UPDATE_INSTALL_DIR="+installDir,
			"HERO_UPDATE_BIN="+heroPath,
		)
		if output, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("installation failure unexpectedly succeeded:\n%s", output)
		}
		assertUpdateState(t, statePath, "true\n")
		assertNoUpdateTempFiles(t, installDir)
	})

	t.Run("manifest validation failure", func(t *testing.T) {
		truePath, err := exec.LookPath("true")
		if err != nil {
			t.Skip("true is unavailable")
		}
		root := t.TempDir()
		installDir := filepath.Join(root, "install")
		sourceDir := filepath.Join(root, "source")
		homeDir := filepath.Join(root, "home")
		pluginDir := filepath.Join(homeDir, ".workflow-hero", "plugins", "telegram")
		for _, dir := range []string{installDir, filepath.Join(sourceDir, "scripts"), pluginDir} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		heroPath := filepath.Join(installDir, "hero")
		oldHero := []byte("#!/bin/sh\nexit 0\n")
		if err := os.WriteFile(heroPath, oldHero, 0o755); err != nil {
			t.Fatal(err)
		}
		daemonPath := filepath.Join(pluginDir, "hero-telegram-daemon")
		oldDaemon := []byte("old daemon\n")
		if err := os.WriteFile(daemonPath, oldDaemon, 0o755); err != nil {
			t.Fatal(err)
		}
		manifestPath := filepath.Join(pluginDir, "manifest.json")
		oldManifest := []byte("{not valid json\n")
		if err := os.WriteFile(manifestPath, oldManifest, 0o644); err != nil {
			t.Fatal(err)
		}

		newHeroPath := writeFakeHero(t, root)
		if err := os.WriteFile(filepath.Join(sourceDir, "scripts", "build_update.sh"), []byte(fakeBuildScript(newHeroPath, truePath)), 0o755); err != nil {
			t.Fatal(err)
		}
		statePath := filepath.Join(installDir, "needs-update.txt")
		if err := os.WriteFile(statePath, []byte("true\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		cmd := exec.Command("bash", updateScriptPath(t, "hero-update.sh"))
		cmd.Env = append(os.Environ(),
			"HERO_UPDATE_SOURCE="+sourceDir,
			"HERO_UPDATE_INSTALL_DIR="+installDir,
			"HERO_UPDATE_BIN="+heroPath,
			"HERO_UPDATE_TELEGRAM_PLUGIN_DIR="+pluginDir,
			"HERO_UPDATE_TELEGRAM_DAEMON="+daemonPath,
			"HERO_UPDATE_TELEGRAM_MANIFEST="+manifestPath,
			"HOME="+homeDir,
		)
		if output, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("malformed manifest unexpectedly succeeded:\n%s", output)
		}
		assertUpdateState(t, statePath, "true\n")
		if got, err := os.ReadFile(heroPath); err != nil {
			t.Fatal(err)
		} else if string(got) != string(oldHero) {
			t.Fatal("Hero changed after manifest validation failure")
		}
		if got, err := os.ReadFile(daemonPath); err != nil {
			t.Fatal(err)
		} else if string(got) != string(oldDaemon) {
			t.Fatal("Telegram daemon changed after manifest validation failure")
		}
		if got, err := os.ReadFile(manifestPath); err != nil {
			t.Fatal(err)
		} else if string(got) != string(oldManifest) {
			t.Fatal("Telegram manifest changed after manifest validation failure")
		}
		assertNoUpdateTempFiles(t, installDir, pluginDir)
	})
}

func assertUpdateState(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("state=%q want %q", got, want)
	}
}

func assertNoUpdateTempFiles(t *testing.T, dirs ...string) {
	t.Helper()
	for _, dir := range dirs {
		for _, pattern := range []string{
			filepath.Join(dir, ".hero.next.*"),
			filepath.Join(dir, ".hero-telegram-daemon.next.*"),
			filepath.Join(dir, ".hero.previous.next.*"),
			filepath.Join(dir, ".hero.old.*"),
			filepath.Join(dir, ".hero.previous.old.*"),
			filepath.Join(dir, ".telegram-daemon.old.*"),
			filepath.Join(dir, ".telegram-manifest.old.*"),
			filepath.Join(dir, ".manifest.next.*"),
			filepath.Join(dir, "needs-update.txt.*"),
		} {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != 0 {
				t.Fatalf("temporary updater files remain: %v", matches)
			}
		}
	}
}

func writeFakeHero(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "new-hero")
	contents := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"${1:-}\" == version ]]; then\n  printf 'hero version test-new\\n'\nfi\n"
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeBuildScript(heroPath, daemonPath string) string {
	return "#!/usr/bin/env bash\nset -euo pipefail\ncp " + shellQuote(heroPath) + " \"${HERO_UPDATE_OUTPUT}\"\nchmod 0755 \"${HERO_UPDATE_OUTPUT}\"\ncp " + shellQuote(daemonPath) + " \"${HERO_UPDATE_TELEGRAM_DAEMON_OUTPUT}\"\nchmod 0755 \"${HERO_UPDATE_TELEGRAM_DAEMON_OUTPUT}\"\n"
}

type blockingProcess struct {
	cmd    *exec.Cmd
	stdin  *os.File
	writer *os.File
	waited bool
}

func startBlockingProcess(t *testing.T, path string) *blockingProcess {
	t.Helper()
	stdin, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &blockingProcess{cmd: exec.Command(path), stdin: stdin, writer: writer}
	process.cmd.Stdin = stdin
	if err := process.cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = writer.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = process.stdin.Close()
		_ = process.writer.Close()
		if !process.waited && process.cmd.Process != nil {
			_ = process.cmd.Process.Kill()
			_ = process.cmd.Wait()
			process.waited = true
		}
	})
	return process
}

func waitForExecutable(t *testing.T, cmd *exec.Cmd, want string) {
	t.Helper()
	procExe := filepath.Join("/proc", strconv.Itoa(cmd.Process.Pid), "exe")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got, err := os.Readlink(procExe); err == nil && got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, err := os.Readlink(procExe)
	t.Fatalf("process executable=%q err=%v want %q", got, err, want)
}

func (p *blockingProcess) wait(t *testing.T, timeout time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	select {
	case err := <-done:
		p.waited = true
		if err != nil {
			// The process is expected to terminate from the updater's signal.
		}
	case <-time.After(timeout):
		t.Fatalf("process %s was not stopped", p.cmd.Path)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
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
		"TimeoutStartSec=5min",
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
