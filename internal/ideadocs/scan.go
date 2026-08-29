package ideadocs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// DirName is the project-relative directory for active design notes.
	DirName = "docs/idea"
	// ExcludeArchive is a top-level subdirectory ignored during discovery.
	ExcludeArchive = "archive"
	// ExcludeTobe is a top-level subdirectory ignored during discovery.
	ExcludeTobe = "tobe"
	// maxActiveFiles caps discovery output to keep prompts bounded.
	maxActiveFiles = 100
)

// ListActive returns project-relative paths to active idea files under docs/idea,
// excluding top-level archive/ and tobe/ subtrees. Missing docs/idea yields nil.
func ListActive(projectDir string) ([]string, error) {
	root := strings.TrimSpace(projectDir)
	if root == "" {
		return nil, nil
	}
	ideaRoot := filepath.Join(root, filepath.FromSlash(DirName))
	info, err := os.Stat(ideaRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	var paths []string
	walkErr := filepath.WalkDir(ideaRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			if path == ideaRoot {
				return nil
			}
			rel, relErr := filepath.Rel(ideaRoot, path)
			if relErr != nil {
				return relErr
			}
			top := strings.Split(filepath.ToSlash(rel), "/")[0]
			if top == ExcludeArchive || top == ExcludeTobe {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		relToProject, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		paths = append(paths, filepath.ToSlash(relToProject))
		if len(paths) >= maxActiveFiles {
			return errStopWalk
		}
		return nil
	})
	if walkErr != nil && walkErr != errStopWalk {
		return nil, walkErr
	}
	sort.Strings(paths)
	return paths, nil
}

var errStopWalk = errors.New("ideadocs: max files reached")

// PromptSection returns a markdown block listing active idea file paths for the
// discover session prompt. Empty when there are no paths.
func PromptSection(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Active idea notes (optional)\n\n")
	b.WriteString("Non-normative design notes found under docs/idea/ (excluding archive/ and tobe/). ")
	b.WriteString("Read each file at the start of Research before grilling; cycle PRD/ADR/UI supersede on conflict (ADR-019).\n\n")
	for _, p := range paths {
		b.WriteString("- ")
		b.WriteString(p)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
