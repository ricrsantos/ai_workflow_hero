// Package scripts holds contract tests for release tooling.
// Go test files must live in a package directory; this file is placed next to
// release.sh but under a dedicated Go package name "scripts" so `go test ./scripts`
// validates the release script contract without executing a full cross-compile.
package scripts

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func scriptPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "release.sh")
}

func TestReleaseScript_ExistsAndExecutableContract(t *testing.T) {
	path := scriptPath(t)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("release.sh missing: %v", err)
	}
	if info.IsDir() {
		t.Fatal("release.sh is a directory")
	}
}

func TestReleaseScript_ArtifactNamingContract(t *testing.T) {
	data, err := os.ReadFile(scriptPath(t))
	if err != nil {
		t.Fatalf("read release.sh: %v", err)
	}
	src := string(data)

	required := []string{
		"linux/amd64",
		"linux/arm64",
		"darwin/amd64",
		"darwin/arm64",
		"hero_${VERSION}_${OS}_${ARCH}",
		"checksums.txt",
		`-X main.version=`,
		"./cmd/hero",
	}
	for _, want := range required {
		if !strings.Contains(src, want) {
			t.Errorf("release.sh missing required contract fragment %q", want)
		}
	}

	// V1 must not introduce Windows or CI automation in the release script.
	forbidden := []string{"windows", "GORELEASER", "github/workflows"}
	lower := strings.ToLower(src)
	for _, bad := range forbidden {
		if strings.Contains(lower, strings.ToLower(bad)) {
			t.Errorf("release.sh must not include out-of-scope fragment %q", bad)
		}
	}
}
