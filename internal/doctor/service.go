package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/envhygiene"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
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
	ProjectDir     string
	BinaryVersion  string
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

	// 2. Hero config files.
	requiredFiles := []string{
		cursoradapter.HeroJSONPath,
		cursoradapter.ProjectJSONPath,
		cursoradapter.DocumentsJSONPath,
		cursoradapter.ChecksumsJSONPath,
		cursoradapter.MetricsSummaryPath,
	}
	for _, f := range requiredFiles {
		full := filepath.Join(opts.ProjectDir, f)
		if _, err := os.Stat(full); err != nil {
			addCheck("file:"+f, "fail", fmt.Sprintf("missing: %s", f))
		} else {
			addCheck("file:"+f, "ok", f)
		}
	}

	// 3. Command files (ADR-011 inventory).
	for _, f := range cursoradapter.RequiredCommandFiles() {
		full := filepath.Join(opts.ProjectDir, f)
		if _, err := os.Stat(full); err != nil {
			addCheck("cmd:"+f, "fail", fmt.Sprintf("missing command file: %s", f))
		} else {
			addCheck("cmd:"+f, "ok", f)
		}
	}

	// 4. Agent files (ADR-011 inventory).
	for _, f := range cursoradapter.RequiredAgentFiles() {
		full := filepath.Join(opts.ProjectDir, f)
		if _, err := os.Stat(full); err != nil {
			addCheck("agent:"+f, "fail", fmt.Sprintf("missing agent file: %s", f))
		} else {
			addCheck("agent:"+f, "ok", f)
		}
	}

	// 5. Skill files.
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

	// 6. hero.json version vs binary version.
	heroPath := filepath.Join(opts.ProjectDir, cursoradapter.HeroJSONPath)
	heroData, err := os.ReadFile(heroPath)
	if err == nil {
		var heroJSON install.HeroJSON
		if jsonErr := json.Unmarshal(heroData, &heroJSON); jsonErr != nil {
			addCheck("hero-json-parse", "fail", "hero.json is not valid JSON: "+jsonErr.Error())
		} else {
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

	// 7. Parse config JSON files.
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

	// 8. Soft secrets hygiene (warn only — never fails the report).
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

	return report
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
