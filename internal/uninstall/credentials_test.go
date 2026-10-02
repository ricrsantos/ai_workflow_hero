package uninstall_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/uninstall"
)

func TestLifecyclePreserveUninstallDisclosesCredentials(t *testing.T) {
	dir := makeInstalledDir(t)
	path := filepath.Join(dir, ".env.hero")
	if err := os.WriteFile(path, []byte("SYNTHETIC_UNINSTALL_SENTINEL"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := uninstall.Run(uninstall.Options{ProjectDir: dir}, &output, &output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "SYNTHETIC_UNINSTALL_SENTINEL" {
		t.Fatal("uninstall removed credentials")
	}
	if !strings.Contains(output.String(), ".env.hero credentials are retained") || strings.Contains(output.String(), "SYNTHETIC_UNINSTALL_SENTINEL") {
		t.Fatal("missing safe retention disclosure")
	}
}
