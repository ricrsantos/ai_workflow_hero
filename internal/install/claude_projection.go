package install

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	claudeDirName = ".claude"

	// ClaudeContextMarkerBegin and ClaudeContextMarkerEnd delimit the only part
	// of the root CLAUDE.md file that Hero owns.
	ClaudeContextMarkerBegin = "<!-- BEGIN AI WORKFLOW HERO MANAGED CONTEXT -->"
	ClaudeContextMarkerEnd   = "<!-- END AI WORKFLOW HERO MANAGED CONTEXT -->"

	claudeContextBlock = ClaudeContextMarkerBegin + "\n" +
		"@AGENTS.md\n" +
		"\n" +
		"## AI Workflow Hero\n" +
		"\n" +
		"Use the project workflow provided by `.claude/skills/workflow-hero/`.\n" +
		"Keep operational cycle state in `.workflow-hero/`; project knowledge belongs in `context/current-state.md` and `context/context-log.md`.\n" +
		ClaudeContextMarkerEnd + "\n"
)

// ClaudeContextDecision controls how Hero handles the root CLAUDE.md file.
// Context management is deliberately explicit because CLAUDE.md may contain
// instructions which are unrelated to Hero.
type ClaudeContextDecision string

const (
	// ClaudeContextInsertOrUpdate creates the managed block or replaces the
	// existing managed block while preserving all unmarked text.
	ClaudeContextInsertOrUpdate ClaudeContextDecision = "insert-or-update"
	// ClaudeContextLeaveUnchanged leaves an existing or missing CLAUDE.md alone.
	ClaudeContextLeaveUnchanged ClaudeContextDecision = "leave-unchanged"
)

// ClaudeContextResult describes a context-file operation.
type ClaudeContextResult struct {
	Changed bool
	Created bool
	Removed bool
	Content string
}

// ClaudePaths holds project-relative Claude projection paths.
type ClaudePaths struct {
	Root     string
	Agents   string
	Commands string
	Skills   string
}

// ClaudePathsFor returns projection directory paths under projectDir.
func ClaudePathsFor(projectDir string) ClaudePaths {
	root := filepath.Join(projectDir, claudeDirName)
	return ClaudePaths{
		Root:     root,
		Agents:   filepath.Join(root, "agents"),
		Commands: filepath.Join(root, "commands"),
		Skills:   filepath.Join(root, "skills"),
	}
}

// ClaudeOwnedPaths returns the projection directories managed by Hero. The
// root .claude directory and user files inside these directories are not
// themselves owned; RemoveClaudeProjection removes only known asset paths.
func ClaudeOwnedPaths(projectDir string) []string {
	p := ClaudePathsFor(projectDir)
	return []string{
		p.Agents,
		p.Commands,
		filepath.Join(p.Skills, "workflow-hero"),
		filepath.Join(p.Skills, "grilling"),
	}
}

// ClaudeAssetGroups returns embed.FS source to destination pairs for the
// Claude projection. Root AGENTS.md is intentionally not an asset group.
func ClaudeAssetGroups(projectDir string) []struct{ Src, Dst string } {
	p := ClaudePathsFor(projectDir)
	return []struct{ Src, Dst string }{
		{"claude/agents", p.Agents},
		{"claude/commands", p.Commands},
		{"claude/skills/workflow-hero", filepath.Join(p.Skills, "workflow-hero")},
		{"claude/skills/grilling", filepath.Join(p.Skills, "grilling")},
	}
}

// ProvisionClaude writes the embedded Claude projection and records each
// projected file in checksums. It never creates or copies root AGENTS.md.
func ProvisionClaude(projectDir string, assetsFS fs.FS, checksums Checksums) error {
	if assetsFS == nil {
		return errors.New("Claude assets filesystem is required")
	}
	if checksums == nil {
		return errors.New("Claude projection checksums map is required")
	}

	paths := ClaudePathsFor(projectDir)
	for _, dir := range []string{paths.Root, paths.Agents, paths.Commands, paths.Skills} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	for _, group := range ClaudeAssetGroups(projectDir) {
		if err := copyAssetDir(assetsFS, group.Src, group.Dst, projectDir, checksums); err != nil {
			return fmt.Errorf("copy Claude %s: %w", group.Src, err)
		}
	}
	slog.Info("claude projection provisioned", "path", paths.Root)
	return nil
}

// ApplyClaudeContext creates or updates Hero's marked CLAUDE.md block. A
// leave-unchanged decision is a no-op, including when AGENTS.md is absent.
// Insert/update requires AGENTS.md and rejects symlinked or malformed files so
// an atomic operation can never silently replace user instructions.
func ApplyClaudeContext(projectDir string, decision ClaudeContextDecision) (ClaudeContextResult, error) {
	decision = ClaudeContextDecision(strings.ToLower(strings.TrimSpace(string(decision))))
	if decision == "" {
		decision = ClaudeContextLeaveUnchanged
	}
	if decision == ClaudeContextLeaveUnchanged {
		return readClaudeContextResult(projectDir)
	}
	if decision != ClaudeContextInsertOrUpdate {
		return ClaudeContextResult{}, fmt.Errorf("unsupported Claude context decision %q", decision)
	}
	if err := requireClaudeAgentsFile(projectDir); err != nil {
		return ClaudeContextResult{}, err
	}

	path := filepath.Join(projectDir, "CLAUDE.md")
	data, mode, exists, err := readClaudeContextFile(path)
	if err != nil {
		return ClaudeContextResult{}, err
	}
	content := string(data)
	location, err := locateClaudeContextBlock(content)
	if err != nil {
		return ClaudeContextResult{}, err
	}
	var updated string
	created := !exists
	if location.found {
		updated = content[:location.start] + claudeContextBlock + content[location.end:]
	} else {
		updated = appendClaudeContextBlock(content)
	}
	if updated == content && exists {
		return ClaudeContextResult{Content: content}, nil
	}
	if err := atomicWriteClaudeContext(path, []byte(updated), mode, exists); err != nil {
		return ClaudeContextResult{}, err
	}
	slog.Info("claude managed context updated", "path", path, "created", created)
	return ClaudeContextResult{Changed: true, Created: created, Content: updated}, nil
}

// EnsureClaudeContext is a concise alias for ApplyClaudeContext used by
// lifecycle callers that have already resolved the explicit decision.
func EnsureClaudeContext(projectDir string, decision ClaudeContextDecision) (ClaudeContextResult, error) {
	return ApplyClaudeContext(projectDir, decision)
}

// UpdateClaudeContext refreshes an existing valid managed block during an
// upgrade. It does not insert a block into a file that never opted into Hero
// context management, and it does not create CLAUDE.md when it is absent.
func UpdateClaudeContext(projectDir string) (ClaudeContextResult, error) {
	path := filepath.Join(projectDir, "CLAUDE.md")
	data, mode, exists, err := readClaudeContextFile(path)
	if err != nil {
		return ClaudeContextResult{}, err
	}
	if !exists {
		return ClaudeContextResult{}, nil
	}
	content := string(data)
	location, err := locateClaudeContextBlock(content)
	if err != nil {
		return ClaudeContextResult{}, err
	}
	if !location.found {
		return ClaudeContextResult{Content: content}, nil
	}
	updated := content[:location.start] + claudeContextBlock + content[location.end:]
	if updated == content {
		return ClaudeContextResult{Content: content}, nil
	}
	if err := requireClaudeAgentsFile(projectDir); err != nil {
		return ClaudeContextResult{}, err
	}
	if err := atomicWriteClaudeContext(path, []byte(updated), mode, true); err != nil {
		return ClaudeContextResult{}, err
	}
	slog.Info("claude managed context refreshed", "path", path)
	return ClaudeContextResult{Changed: true, Content: updated}, nil
}

// RemoveClaudeManagedContext removes only the marked block from CLAUDE.md.
// Unmarked content, including a wholly user-owned file, is preserved. A file
// containing duplicate or unmatched markers is left untouched and returns an
// actionable error because ownership cannot be determined safely.
func RemoveClaudeManagedContext(projectDir string) (ClaudeContextResult, error) {
	path := filepath.Join(projectDir, "CLAUDE.md")
	data, mode, exists, err := readClaudeContextFile(path)
	if err != nil {
		return ClaudeContextResult{}, err
	}
	if !exists {
		return ClaudeContextResult{}, nil
	}
	content := string(data)
	location, err := locateClaudeContextBlock(content)
	if err != nil {
		return ClaudeContextResult{}, err
	}
	if !location.found {
		return ClaudeContextResult{Content: content}, nil
	}

	prefix := content[:location.start]
	suffix := content[location.end:]
	// ApplyClaudeContext inserts one separator line before a new block. Treat
	// that generated line as part of the managed block on removal, while
	// retaining all other user bytes exactly.
	if strings.HasSuffix(prefix, "\n\n") {
		prefix = prefix[:len(prefix)-1]
	}
	updated := prefix + suffix
	if strings.TrimSpace(updated) == "" {
		if err := os.Remove(path); err != nil {
			return ClaudeContextResult{}, fmt.Errorf("remove empty managed %s: %w", path, err)
		}
		slog.Info("claude managed context removed", "path", path, "file_removed", true)
		return ClaudeContextResult{Changed: true, Removed: true}, nil
	}
	if err := atomicWriteClaudeContext(path, []byte(updated), mode, true); err != nil {
		return ClaudeContextResult{}, err
	}
	slog.Info("claude managed context removed", "path", path, "file_removed", false)
	return ClaudeContextResult{Changed: true, Content: updated}, nil
}

// RemoveClaudeProjection removes Hero-owned Claude asset files and empty
// parent directories while retaining user files under .claude. Checksums are
// used as a forward-compatible manifest; the built-in names cover projects
// created before checksums were introduced for Claude.
func RemoveClaudeProjection(projectDir string) error {
	paths := ClaudePathsFor(projectDir)
	owned := make(map[string]struct{})
	for _, name := range claudeOwnedAssetFiles {
		owned[filepath.Clean(filepath.Join(paths.Root, name))] = struct{}{}
	}
	if checksums, err := LoadChecksums(projectDir); err == nil {
		prefix := filepath.Clean(claudeDirName) + string(filepath.Separator)
		for rel := range checksums {
			clean := filepath.Clean(filepath.FromSlash(rel))
			if strings.HasPrefix(clean, prefix) {
				owned[filepath.Join(projectDir, clean)] = struct{}{}
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load Claude projection checksums: %w", err)
	}

	for path := range owned {
		if err := removeOwnedFile(path); err != nil {
			return err
		}
	}
	for _, dir := range []string{
		filepath.Join(paths.Skills, "workflow-hero"),
		filepath.Join(paths.Skills, "grilling"),
		paths.Skills,
		paths.Agents,
		paths.Commands,
		paths.Root,
	} {
		if err := removeIfEmpty(dir); err != nil {
			return err
		}
	}
	slog.Info("claude projection removed", "path", paths.Root)
	return nil
}

// claudeContextLocation uses byte offsets so all unmarked content can be
// copied exactly as it appeared on disk.
type claudeContextLocation struct {
	start int
	end   int
	found bool
}

func locateClaudeContextBlock(content string) (claudeContextLocation, error) {
	beginCount := strings.Count(content, ClaudeContextMarkerBegin)
	endCount := strings.Count(content, ClaudeContextMarkerEnd)
	if beginCount == 0 && endCount == 0 {
		return claudeContextLocation{}, nil
	}
	if beginCount != 1 || endCount != 1 {
		return claudeContextLocation{}, fmt.Errorf("CLAUDE.md has duplicate Hero context markers; resolve them manually before changing the managed block")
	}
	begin := strings.Index(content, ClaudeContextMarkerBegin)
	endMarker := strings.Index(content, ClaudeContextMarkerEnd)
	if begin > endMarker {
		return claudeContextLocation{}, fmt.Errorf("CLAUDE.md has malformed Hero context markers: end marker precedes begin marker")
	}
	if !markerAtLineBoundary(content, begin, len(ClaudeContextMarkerBegin)) || !markerAtLineBoundary(content, endMarker, len(ClaudeContextMarkerEnd)) {
		return claudeContextLocation{}, fmt.Errorf("CLAUDE.md has malformed Hero context markers: markers must occupy complete lines")
	}
	end := endMarker + len(ClaudeContextMarkerEnd)
	if end < len(content) && content[end] == '\r' {
		end++
	}
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return claudeContextLocation{start: begin, end: end, found: true}, nil
}

func markerAtLineBoundary(content string, offset, length int) bool {
	lineStart := offset == 0 || content[offset-1] == '\n'
	lineEnd := offset+length == len(content) || content[offset+length] == '\n' || content[offset+length] == '\r'
	return lineStart && lineEnd
}

func appendClaudeContextBlock(content string) string {
	if content == "" {
		return claudeContextBlock
	}
	separator := "\n\n"
	if strings.HasSuffix(content, "\n\n") || strings.HasSuffix(content, "\r\n\r\n") {
		separator = ""
	} else if strings.HasSuffix(content, "\n") || strings.HasSuffix(content, "\r\n") {
		separator = "\n"
	}
	return content + separator + claudeContextBlock
}

func requireClaudeAgentsFile(projectDir string) error {
	path := filepath.Join(projectDir, "AGENTS.md")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("managed Claude context requires AGENTS.md; create project instructions and retry")
		}
		return fmt.Errorf("check AGENTS.md: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("managed Claude context requires a regular AGENTS.md file")
	}
	return nil
}

func readClaudeContextResult(projectDir string) (ClaudeContextResult, error) {
	path := filepath.Join(projectDir, "CLAUDE.md")
	data, _, exists, err := readClaudeContextFile(path)
	if err != nil {
		return ClaudeContextResult{}, err
	}
	if !exists {
		return ClaudeContextResult{}, nil
	}
	if _, err := locateClaudeContextBlock(string(data)); err != nil {
		return ClaudeContextResult{}, err
	}
	return ClaudeContextResult{Content: string(data)}, nil
}

func readClaudeContextFile(path string) ([]byte, os.FileMode, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0o644, false, nil
		}
		return nil, 0, false, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, 0, false, fmt.Errorf("refusing to modify symlinked %s", path)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, false, fmt.Errorf("refusing to modify non-regular %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false, fmt.Errorf("read %s: %w", path, err)
	}
	return data, info.Mode().Perm(), true, nil
}

func atomicWriteClaudeContext(path string, data []byte, mode os.FileMode, existing bool) error {
	if mode == 0 {
		mode = 0o644
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".CLAUDE.md.tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary %s: %w", path, err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if err := tmp.Chmod(mode); err != nil {
		cleanup()
		return fmt.Errorf("set mode for temporary %s: %w", path, err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temporary %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temporary %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("commit %s: %w", path, err)
	}
	if existing {
		slog.Debug("claude context atomic replacement committed", "path", path)
	}
	return nil
}

func removeOwnedFile(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("remove Claude-owned path %s: %w", path, err)
}

func removeIfEmpty(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Claude projection directory %s: %w", path, err)
	}
	if len(entries) != 0 {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove empty Claude projection directory %s: %w", path, err)
	}
	return nil
}

// This manifest also permits uninstalling a projection created by a previous
// binary whose checksums file is missing. New files should be added here when
// the embedded Claude asset family grows.
var claudeOwnedAssetFiles = []string{
	"agents/backend_agent.md",
	"agents/browser_ui_agent.md",
	"agents/context_agent.md",
	"agents/discover_agent.md",
	"agents/end2end_qa_agent.md",
	"agents/frontend_agent.md",
	"agents/generic_agent.md",
	"agents/judge_agent.md",
	"agents/orchestration_agent.md",
	"agents/planning_agent.md",
	"agents/qa_agent.md",
	"commands/hero-approve.md",
	"commands/hero-archive.md",
	"commands/hero-back.md",
	"commands/hero-cancel.md",
	"commands/hero-continue.md",
	"commands/hero-cycles.md",
	"commands/hero-finish.md",
	"commands/hero-help.md",
	"commands/hero-model.md",
	"commands/hero-new.md",
	"commands/hero-reject.md",
	"commands/hero-resume.md",
	"commands/hero-start.md",
	"commands/hero-status.md",
	"commands/hero-sync.md",
	"commands/hero-todos.md",
	"skills/workflow-hero/SKILL.md",
	"skills/grilling/SKILL.md",
}
