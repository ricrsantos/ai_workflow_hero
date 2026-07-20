package install

import (
	"os"
	"path/filepath"
)

// isGitRepo reports whether dir (or any parent) contains a .git directory.
func isGitRepo(dir string) bool {
	return hasDotGit(dir)
}

// hasDotGit reports whether dir directly contains a .git entry.
func hasDotGit(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}
