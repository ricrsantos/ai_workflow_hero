// Package userpath locates user-installed CLIs that live outside a stripped PATH
// (nvm, fnm, volta, asdf, npm prefix, ~/.local/bin). Cursor Agent sandbox shells
// often omit those directories even when the user's login shell has them.
package userpath

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExtraBinDirs returns directories that commonly hold node-based CLIs (openspec)
// and other user tools, even when the process PATH is minimal.
func ExtraBinDirs() []string {
	var dirs []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		dirs = append(dirs, p)
	}

	add(os.Getenv("NVM_BIN"))
	add(os.Getenv("FNM_MULTISHELL_PATH"))
	if volta := strings.TrimSpace(os.Getenv("VOLTA_HOME")); volta != "" {
		add(filepath.Join(volta, "bin"))
	}
	if prefix := strings.TrimSpace(os.Getenv("npm_config_prefix")); prefix != "" {
		add(filepath.Join(prefix, "bin"))
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		add(filepath.Join(home, ".local", "bin"))
		add(filepath.Join(home, ".npm-global", "bin"))
		add(filepath.Join(home, "bin"))
		add(filepath.Join(home, ".volta", "bin"))
		add(filepath.Join(home, ".asdf", "shims"))
		add(filepath.Join(home, ".fnm", "aliases", "default", "bin"))

		nvmDir := strings.TrimSpace(os.Getenv("NVM_DIR"))
		if nvmDir == "" {
			nvmDir = filepath.Join(home, ".nvm")
		}
		add(filepath.Join(nvmDir, "current", "bin"))
		for _, p := range globBins(filepath.Join(nvmDir, "versions", "node", "*", "bin")) {
			add(p)
		}
		for _, p := range globBins(filepath.Join(home, ".local", "share", "fnm", "node-versions", "*", "installation", "bin")) {
			add(p)
		}
		for _, p := range globBins(filepath.Join(home, ".fnm", "node-versions", "*", "installation", "bin")) {
			add(p)
		}
	}

	add("/usr/local/bin")
	add("/opt/homebrew/bin")
	return dedupeKeepOrder(dirs)
}

func globBins(pattern string) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	return matches
}

// LookPath finds name via lookPath first, then extra directories.
func LookPath(name string, lookPath func(string) (string, error), extra []string) (string, error) {
	if lookPath != nil {
		if p, err := lookPath(name); err == nil && strings.TrimSpace(p) != "" {
			return p, nil
		}
	}
	for _, dir := range extra {
		cand := filepath.Join(dir, name)
		if isExecutable(cand) {
			return cand, nil
		}
	}
	if lookPath != nil {
		return lookPath(name)
	}
	return "", os.ErrNotExist
}

func isExecutable(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return false
	}
	return fi.Mode()&0o111 != 0
}

// AugmentPATH prepends extraDirs to PATH in env (copy of os.Environ()-style pairs).
func AugmentPATH(env []string, extraDirs ...string) []string {
	if len(extraDirs) == 0 {
		return env
	}
	path := envValue(env, "PATH")
	parts := append([]string{}, extraDirs...)
	parts = append(parts, filepath.SplitList(path)...)
	parts = dedupeKeepOrder(parts)
	return setEnv(env, "PATH", strings.Join(parts, string(os.PathListSeparator)))
}

// EnvWithBinaryDir returns os.Environ() with the binary's directory and extra
// dirs prepended to PATH so shebang scripts (#!/usr/bin/env node) resolve.
func EnvWithBinaryDir(binary string, extra []string) []string {
	var prepend []string
	if dir := filepath.Dir(binary); dir != "" && dir != "." {
		prepend = append(prepend, dir)
	}
	prepend = append(prepend, extra...)
	return AugmentPATH(os.Environ(), prepend...)
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return env[i][len(prefix):]
		}
	}
	return ""
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			if !found {
				out = append(out, prefix+value)
				found = true
			}
			continue
		}
		out = append(out, e)
	}
	if !found {
		out = append(out, prefix+value)
	}
	return out
}

func dedupeKeepOrder(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
