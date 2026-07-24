// Package envhygiene provides soft secrets hygiene helpers for Hero projects:
// ensure `.env.example` + `.gitignore` patterns, and detect tracked secret files.
package envhygiene

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// MarkerBegin / MarkerEnd delimit the Hero-managed secrets block in .gitignore.
	MarkerBegin = "# BEGIN Hero secrets hygiene"
	MarkerEnd   = "# END Hero secrets hygiene"

	EnvExamplePath = ".env.example"
	GitignorePath  = ".gitignore"

	templateEnvExample     = "templates/env.example"
	templateGitignoreBlock = "templates/gitignore-secrets"
)

// SensitiveTrackedPrefixes are path prefixes that should never be committed.
var SensitiveTrackedPrefixes = []string{
	".env",
}

// SensitiveTrackedExact are exact relative paths that should never be committed.
var SensitiveTrackedExact = []string{
	"credentials.json",
	"secrets.json",
}

// SensitiveTrackedSuffixes are file suffixes that should never be committed.
var SensitiveTrackedSuffixes = []string{
	".pem",
}

// EnsureProjectRoot creates `.env.example` if missing and ensures `.gitignore`
// contains the Hero secrets hygiene block. Existing files are never overwritten;
// missing patterns are appended as a marked block.
func EnsureProjectRoot(projectDir string, assetsFS fs.FS) error {
	if err := ensureEnvExample(projectDir, assetsFS); err != nil {
		return err
	}
	return ensureGitignore(projectDir, assetsFS)
}

func ensureEnvExample(projectDir string, assetsFS fs.FS) error {
	dst := filepath.Join(projectDir, EnvExamplePath)
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", dst, err)
	}

	data, err := fs.ReadFile(assetsFS, templateEnvExample)
	if err != nil {
		return fmt.Errorf("read asset %s: %w", templateEnvExample, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}

func ensureGitignore(projectDir string, assetsFS fs.FS) error {
	path := filepath.Join(projectDir, GitignorePath)
	block, err := fs.ReadFile(assetsFS, templateGitignoreBlock)
	if err != nil {
		return fmt.Errorf("read asset %s: %w", templateGitignoreBlock, err)
	}
	blockStr := strings.TrimSpace(string(block)) + "\n"

	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(path, []byte(blockStr), 0o644)
		}
		return fmt.Errorf("read %s: %w", path, err)
	}

	content := string(existing)
	if strings.Contains(content, MarkerBegin) {
		return nil
	}
	// Also skip append if `.env` is already ignored without the Hero marker.
	if GitignoreIgnoresEnv(content) {
		return nil
	}

	sep := "\n"
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		sep = "\n\n"
	} else if len(content) > 0 {
		sep = "\n"
	} else {
		sep = ""
	}
	return os.WriteFile(path, []byte(content+sep+blockStr), 0o644)
}

// GitignoreIgnoresEnv reports whether content already ignores `.env` files.
func GitignoreIgnoresEnv(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		switch trimmed {
		case ".env", ".env.*", "*.env", "**/.env", "/.env":
			return true
		}
	}
	return false
}

// HasEnvExample reports whether `.env.example` exists in projectDir.
func HasEnvExample(projectDir string) bool {
	_, err := os.Stat(filepath.Join(projectDir, EnvExamplePath))
	return err == nil
}

// HasGitignore reports whether `.gitignore` exists in projectDir.
func HasGitignore(projectDir string) bool {
	_, err := os.Stat(filepath.Join(projectDir, GitignorePath))
	return err == nil
}

// IsSensitivePath reports whether a repo-relative path looks like a secret file.
func IsSensitivePath(rel string) bool {
	rel = filepath.ToSlash(rel)
	base := filepath.Base(rel)

	if base == EnvExamplePath || strings.HasSuffix(rel, "/"+EnvExamplePath) {
		return false
	}

	for _, exact := range SensitiveTrackedExact {
		if rel == exact || base == exact {
			return true
		}
	}
	for _, suf := range SensitiveTrackedSuffixes {
		if strings.HasSuffix(strings.ToLower(base), suf) {
			return true
		}
	}
	for _, prefix := range SensitiveTrackedPrefixes {
		if base == prefix || strings.HasPrefix(base, prefix+".") || strings.HasPrefix(base, prefix+"-") {
			return true
		}
	}
	return false
}

// TrackedSensitiveFiles returns tracked files that look like secrets (warn-only).
// If git is unavailable or the directory is not a repo, returns nil, nil.
func TrackedSensitiveFiles(projectDir string) ([]string, error) {
	cmd := exec.Command("git", "-C", projectDir, "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}
	var found []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f == "" {
			continue
		}
		if IsSensitivePath(f) {
			found = append(found, f)
		}
	}
	return found, nil
}
