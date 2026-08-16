package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/envhygiene"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// CheckResult represents the outcome of a single doctor check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warn", "fail"
	Message string `json:"message"`
}

// Report is the full doctor report.
type Report struct {
	Checks []CheckResult `json:"checks"`
	OK     bool          `json:"ok"`
}

// Options holds doctor configuration.
type Options struct {
	ProjectDir    string
	BinaryVersion string
	// CursorCLIProbe overrides the default Cursor Agent CLI availability check (tests).
	CursorCLIProbe CursorCLIProbe
}

// Run performs all doctor checks and returns a report.
func Run(opts Options) Report {
	var report Report
	report.OK = true

	addCheck := func(name, status, message string) {
		report.Checks = append(report.Checks, CheckResult{
			Name:    name,
			Status:  status,
			Message: message,
		})
		if status == "fail" {
			report.OK = false
		}
	}

	// 1. Git repository check.
	if hasDotGit(opts.ProjectDir) {
		addCheck("git-repo", "ok", "project is a git repository")
	} else {
		addCheck("git-repo", "fail", "not a git repository — run `git init` or `hero install --git-init`")
	}

	// 2. Operational store (hero.db) — auto-create when Hero is installed.
	heroInstalled := false
	if _, err := os.Stat(filepath.Join(opts.ProjectDir, cursoradapter.HeroJSONPath)); err == nil {
		heroInstalled = true
	}
	dbPath := filepath.Join(opts.ProjectDir, store.RelativeDBPath)
	if heroInstalled {
		s, err := store.OpenProject(opts.ProjectDir)
		if err != nil {
			addCheck("operational-store", "fail", fmt.Sprintf("cannot open/create %s: %v", store.RelativeDBPath, err))
		} else {
			_ = s.Close()
			if _, err := os.Stat(dbPath); err != nil {
				addCheck("operational-store", "fail", fmt.Sprintf("missing %s after open", store.RelativeDBPath))
			} else {
				addCheck("operational-store", "ok", store.RelativeDBPath)
			}
		}
	}

	// 3. Hero config files.
	requiredFiles := []string{
		cursoradapter.HeroJSONPath,
		cursoradapter.ProjectJSONPath,
		cursoradapter.DocumentsJSONPath,
		cursoradapter.ChecksumsJSONPath,
		cursoradapter.MetricsSummaryPath,
		cursoradapter.WorkflowHelpPath,
	}
	for _, f := range requiredFiles {
		full := filepath.Join(opts.ProjectDir, f)
		if _, err := os.Stat(full); err != nil {
			addCheck("file:"+f, "fail", fmt.Sprintf("missing: %s", f))
		} else {
			addCheck("file:"+f, "ok", f)
		}
	}

	// 4. Command files (ADR-011 inventory).
	for _, f := range cursoradapter.RequiredCommandFiles() {
		full := filepath.Join(opts.ProjectDir, f)
		if _, err := os.Stat(full); err != nil {
			addCheck("cmd:"+f, "fail", fmt.Sprintf("missing command file: %s", f))
		} else {
			addCheck("cmd:"+f, "ok", f)
		}
	}

	// 5. Agent files (ADR-011 inventory).
	for _, f := range cursoradapter.RequiredAgentFiles() {
		full := filepath.Join(opts.ProjectDir, f)
		if _, err := os.Stat(full); err != nil {
			addCheck("agent:"+f, "fail", fmt.Sprintf("missing agent file: %s", f))
		} else {
			addCheck("agent:"+f, "ok", f)
		}
	}

	// 6. Skill files.
	skillFiles := []string{
		filepath.Join(cursoradapter.WorkflowHeroSkillDir, "SKILL.md"),
		filepath.Join(cursoradapter.GrillingSkillDir, "SKILL.md"),
	}
	for _, f := range skillFiles {
		full := filepath.Join(opts.ProjectDir, f)
		if _, err := os.Stat(full); err != nil {
			addCheck("skill:"+f, "fail", fmt.Sprintf("missing skill: %s", f))
		} else {
			addCheck("skill:"+f, "ok", f)
		}
	}

	// 7. hero.json version vs binary version.
	var configuredTools []string
	heroPath := filepath.Join(opts.ProjectDir, cursoradapter.HeroJSONPath)
	heroData, err := os.ReadFile(heroPath)
	if err == nil {
		var heroJSON install.HeroJSON
		if jsonErr := json.Unmarshal(heroData, &heroJSON); jsonErr != nil {
			addCheck("hero-json-parse", "fail", "hero.json is not valid JSON: "+jsonErr.Error())
		} else {
			configuredTools = heroJSON.CLI.Tools
			if heroJSON.CLI.Version != opts.BinaryVersion {
				addCheck("version-match", "warn", fmt.Sprintf(
					"installed version %q differs from binary version %q — run `hero upgrade`",
					heroJSON.CLI.Version, opts.BinaryVersion,
				))
			} else {
				addCheck("version-match", "ok", fmt.Sprintf("version %s", opts.BinaryVersion))
			}
		}
	}

	// 8. Parse config JSON files.
	jsonFiles := []string{
		cursoradapter.ProjectJSONPath,
		cursoradapter.DocumentsJSONPath,
		cursoradapter.ChecksumsJSONPath,
	}
	for _, f := range jsonFiles {
		full := filepath.Join(opts.ProjectDir, f)
		data, err := os.ReadFile(full)
		if err != nil {
			continue // already caught above
		}
		var v interface{}
		if err := json.Unmarshal(data, &v); err != nil {
			addCheck("json:"+f, "fail", fmt.Sprintf("%s is not valid JSON: %v", f, err))
		}
	}

	// 9. Soft secrets hygiene (warn only — never fails the report).
	if !envhygiene.HasEnvExample(opts.ProjectDir) {
		addCheck("secrets-env-example", "warn", "missing .env.example — commit placeholders only; keep real secrets in local .env")
	} else {
		addCheck("secrets-env-example", "ok", ".env.example present")
	}

	giPath := filepath.Join(opts.ProjectDir, envhygiene.GitignorePath)
	if giData, err := os.ReadFile(giPath); err != nil {
		addCheck("secrets-gitignore", "warn", "missing .gitignore — add patterns so .env and secrets are not committed")
	} else if !envhygiene.GitignoreIgnoresEnv(string(giData)) {
		addCheck("secrets-gitignore", "warn", ".gitignore does not ignore .env — add .env (and keep .env.example committed)")
	} else {
		addCheck("secrets-gitignore", "ok", ".gitignore ignores .env")
	}

	if tracked, err := envhygiene.TrackedSensitiveFiles(opts.ProjectDir); err == nil && len(tracked) > 0 {
		addCheck("secrets-tracked", "warn", fmt.Sprintf(
			"sensitive files tracked by git: %s — untrack them (git rm --cached) and keep values local",
			strings.Join(tracked, ", "),
		))
	} else {
		addCheck("secrets-tracked", "ok", "no sensitive files tracked by git")
	}

	// 10. Harness marker detection (warn-only; ADR-022; UI-C02-001 §5).
	if len(configuredTools) > 0 {
		addHarnessMarkerChecks(opts.ProjectDir, configuredTools, addCheck)
	}

	// 11. Cursor Agent CLI diagnostics (warn-only; PRD-C03-001 §4.10; complementary to TUI boot).
	if heroInstalled {
		addCursorCLIChecks(context.Background(), opts.ProjectDir, opts.CursorCLIProbe, addCheck)
		addOpenCodeCLIChecks(opts.ProjectDir, addCheck)
	}

	return report
}

func addHarnessMarkerChecks(projectDir string, configuredTools []string, addCheck func(name, status, message string)) {
	res, err := harness.DetectMarkers(projectDir, configuredTools)
	if err != nil {
		slog.Error("harness marker detection failed", "error", err)
		addCheck("harness-markers", "warn", "could not scan harness markers: "+err.Error())
		return
	}

	configured := map[string]bool{}
	for _, t := range configuredTools {
		configured[t] = true
	}

	if len(res.UnsupportedPresent) == 0 {
		addCheck("harness-markers", "ok", "no unsupported harness markers detected")
		return
	}

	for _, m := range res.UnsupportedPresent {
		addCheck(
			"harness-marker:"+m.ToolID,
			"warn",
			harness.UnsupportedMarkerMessage(m, configured[m.ToolID]),
		)
	}
}

// PrintTable writes a human-readable table of check results to w.
func PrintTable(w io.Writer, r Report) {
	fmt.Fprintln(w, "+----------------------------+--------+------------------------------------------------------+")
	fmt.Fprintln(w, "| Check                      | Status | Message                                              |")
	fmt.Fprintln(w, "+----------------------------+--------+------------------------------------------------------+")
	for _, c := range r.Checks {
		status := c.Status
		name := truncate(c.Name, 26)
		msg := truncate(c.Message, 52)
		fmt.Fprintf(w, "| %-26s | %-6s | %-52s |\n", name, status, msg)
	}
	fmt.Fprintln(w, "+----------------------------+--------+------------------------------------------------------+")
	if r.OK {
		fmt.Fprintln(w, "All checks passed.")
	} else {
		fmt.Fprintln(w, "Some checks failed. See above for details.")
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func hasDotGit(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}
