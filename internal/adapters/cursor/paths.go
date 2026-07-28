// Package cursor defines Cursor-specific path constants and helpers for the Hero CLI.
package cursor

import "path/filepath"

const (
	// CursorDir is the Cursor configuration directory within a project.
	CursorDir = ".cursor"

	// CommandsDir is the Cursor commands directory.
	CommandsDir = ".cursor/commands"

	// AgentsDir is the Cursor agents directory.
	AgentsDir = ".cursor/agents"

	// SkillsDir is the Cursor skills directory.
	SkillsDir = ".cursor/skills"

	// WorkflowHeroSkillDir is the workflow-hero skill directory.
	WorkflowHeroSkillDir = ".cursor/skills/workflow-hero"

	// GrillingSkillDir is the grilling skill directory.
	GrillingSkillDir = ".cursor/skills/grilling"

	// HeroDir is the main Hero configuration directory.
	HeroDir = ".workflow-hero"

	// HeroConfigDir is the Hero configuration subdirectory.
	HeroConfigDir = ".workflow-hero/config"

	// HeroTemplatesDir is the Hero templates directory.
	HeroTemplatesDir = ".workflow-hero/templates"

	// HeroModelsDir is the Hero models directory.
	HeroModelsDir = ".workflow-hero/models"

	// HeroDocsDir is the Hero user documentation directory.
	HeroDocsDir = ".workflow-hero/docs"

	// WorkflowHelpPath is the installed end-user guide.
	WorkflowHelpPath = ".workflow-hero/docs/workflow-help.md"

	// HeroCyclesDir is the Hero cycles base directory.
	HeroCyclesDir = ".workflow-hero/cycles"

	// HeroCurrentCycleDir is the current active cycle directory.
	HeroCurrentCycleDir = ".workflow-hero/cycles/current"

	// HeroJSONPath is the Hero installation metadata file.
	HeroJSONPath = ".workflow-hero/config/hero.json"

	// ProjectJSONPath is the project identity file.
	ProjectJSONPath = ".workflow-hero/config/project.json"

	// DocumentsJSONPath is the documents registry file.
	DocumentsJSONPath = ".workflow-hero/config/documents.json"

	// ChecksumsJSONPath stores SHA256 checksums of installed assets.
	ChecksumsJSONPath = ".workflow-hero/config/checksums.json"

	// MetricsSummaryPath is the project-wide metrics summary.
	MetricsSummaryPath = ".workflow-hero/metrics-summary.md"

	// LockFilePath is the cycle lock file to prevent concurrent sessions.
	LockFilePath = ".workflow-hero/cycles/current/.lock"
)

// RequiredFiles returns all required Hero-managed files for doctor checks.
// These match the ADR-011 inventory.
func RequiredFiles() []string {
	return []string{
		HeroJSONPath,
		ProjectJSONPath,
		DocumentsJSONPath,
		ChecksumsJSONPath,
		MetricsSummaryPath,
		WorkflowHelpPath,
		filepath.Join(WorkflowHeroSkillDir, "SKILL.md"),
		filepath.Join(GrillingSkillDir, "SKILL.md"),
	}
}

// RequiredCommandFiles returns all required command asset files per ADR-011.
func RequiredCommandFiles() []string {
	commands := []string{
		"hero-init", "hero-start", "hero-approve", "hero-reject",
		"hero-cancel", "hero-finish", "hero-archive", "hero-resume",
		"hero-sync", "hero-status", "hero-help", "hero-continue", "hero-back",
	}
	files := make([]string, len(commands))
	for i, cmd := range commands {
		files[i] = filepath.Join(CommandsDir, cmd+".md")
	}
	return files
}

// RequiredAgentFiles returns all required agent asset files per ADR-011.
func RequiredAgentFiles() []string {
	agents := []string{
		"orchestration_agent", "discover_agent", "planning_agent", "context_agent",
		"backend_agent", "frontend_agent", "generic_agent",
		"qa_agent", "judge_agent", "browser_ui_agent", "end2end_qa_agent",
	}
	files := make([]string, len(agents))
	for i, agent := range agents {
		files[i] = filepath.Join(AgentsDir, agent+".md")
	}
	return files
}

// HeroOwnedPaths returns the paths that `hero uninstall` should remove.
func HeroOwnedPaths() []string {
	return []string{
		AgentsDir,
		WorkflowHeroSkillDir,
		GrillingSkillDir,
		HeroDir,
	}
}

// HeroOwnedCommandPattern is used to match hero command files for removal.
// Matches files of the form .cursor/commands/hero-*.md
const HeroOwnedCommandPattern = "hero-*.md"
