package update_models_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/assetconflict"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/update_models"
)

// sampleYAML is a minimal valid model pricing YAML.
const sampleYAML = `provider: test
version: 1
last_updated: 2026-07-14
currency: usd
unit: per_1m_tokens

models:
  test-model:
    input: 1.00
    output: 2.00
`

func makeTestServer(t *testing.T, files map[string]string, failFiles []string) *httptest.Server {
	t.Helper()
	failSet := make(map[string]bool)
	for _, f := range failFiles {
		failSet[f] = true
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		if failSet[name] {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if content, ok := files[name]; ok {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(content))
		} else {
			_, _ = w.Write([]byte(sampleYAML))
		}
	}))
}

func TestUpdateModels_Rewrites_AllFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, cursoradapter.HeroModelsDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	srv := makeTestServer(t, nil, nil)
	defer srv.Close()

	var stdout, stderr strings.Builder
	err := update_models.Run(update_models.Options{
		ProjectDir: dir,
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for _, name := range update_models.ModelNames {
		path := filepath.Join(dir, cursoradapter.HeroModelsDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("model file %s not written: %v", name, err)
			continue
		}
		if !strings.Contains(string(data), "provider: test") {
			t.Errorf("model file %s content unexpected: %s", name, string(data))
		}
	}
}

func TestUpdateModels_PartialFailure_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, cursoradapter.HeroModelsDir), 0o755)

	// Make anthropic.yml fail.
	srv := makeTestServer(t, nil, []string{"anthropic.yml"})
	defer srv.Close()

	var stdout, stderr strings.Builder
	err := update_models.Run(update_models.Options{
		ProjectDir: dir,
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}, &stdout, &stderr)

	if err == nil {
		t.Fatal("expected error when one file fails to fetch")
	}
	if !strings.Contains(err.Error(), "anthropic.yml") {
		t.Errorf("error should mention anthropic.yml: %v", err)
	}

	// Other files should still have been written.
	for _, name := range update_models.ModelNames {
		if name == "anthropic.yml" {
			continue
		}
		path := filepath.Join(dir, cursoradapter.HeroModelsDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to be written despite anthropic.yml failure", name)
		}
	}
}

func TestUpdateModels_NetworkFailure_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, cursoradapter.HeroModelsDir), 0o755)

	var stdout, stderr strings.Builder
	err := update_models.Run(update_models.Options{
		ProjectDir: dir,
		BaseURL:    "http://127.0.0.1:1", // unreachable
		HTTPClient: http.DefaultClient,
	}, &stdout, &stderr)

	if err == nil {
		t.Fatal("expected error on network failure")
	}
}

func TestUpdateModels_CustomizedFileIsReplacedWithBackup(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := install.Run(install.Options{
		ProjectDir: dir,
		Name:       "Test",
		Summary:    "test",
		Tools:      []string{"cursor"},
		Version:    "1.0.0",
		GitInit:    false,
		AssetsFS:   assets.FS,
	}, new(strings.Builder), new(strings.Builder)); err != nil {
		t.Fatalf("install: %v", err)
	}

	targetRel := filepath.Join(cursoradapter.HeroModelsDir, "openai.yml")
	targetAbs := filepath.Join(dir, targetRel)
	original, err := os.ReadFile(targetAbs)
	if err != nil {
		t.Fatal(err)
	}
	customized := append(original, []byte("\n# local tweak\n")...)
	if err := os.WriteFile(targetAbs, customized, 0o644); err != nil {
		t.Fatal(err)
	}

	newContent := sampleYAML + "models:\n  test-model:\n    input: 9.99\n"
	srv := makeTestServer(t, map[string]string{"openai.yml": newContent}, nil)
	defer srv.Close()

	var stdout, stderr strings.Builder
	if err := update_models.Run(update_models.Options{
		ProjectDir: dir,
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}, &stdout, &stderr); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	written, err := os.ReadFile(targetAbs)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != newContent {
		t.Fatalf("model file not replaced; got %q", written)
	}

	entries, err := os.ReadDir(filepath.Dir(targetAbs))
	if err != nil {
		t.Fatal(err)
	}
	var backupFound bool
	for _, e := range entries {
		if strings.Contains(e.Name(), ".conflict") && strings.HasPrefix(e.Name(), "openai.yml_") {
			backupFound = true
			backupData, _ := os.ReadFile(filepath.Join(filepath.Dir(targetAbs), e.Name()))
			if !strings.Contains(string(backupData), "local tweak") {
				t.Error("backup should contain customization")
			}
		}
	}
	if !backupFound {
		t.Error("conflict backup file not found")
	}
	if !strings.Contains(stderr.String(), "replaced with the new file due to conflicts") {
		t.Fatalf("expected conflict warning, got: %s", stderr.String())
	}

	checksumsData, err := os.ReadFile(filepath.Join(dir, cursoradapter.ChecksumsJSONPath))
	if err != nil {
		t.Fatal(err)
	}
	var checksums install.Checksums
	if err := json.Unmarshal(checksumsData, &checksums); err != nil {
		t.Fatal(err)
	}
	if checksums[targetRel] != assetconflict.SHA256Hex([]byte(newContent)) {
		t.Fatal("checksums.json not updated after conflict replacement")
	}
}
