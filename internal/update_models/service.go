package update_models

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/assetconflict"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

const defaultBaseURL = "https://raw.githubusercontent.com/ricrsantos/ai_workflow_hero/main/assets/models"

// ModelFile describes a model pricing file to fetch.
type ModelFile struct {
	Filename string
	URL      string
}

// Options holds update-models configuration.
type Options struct {
	ProjectDir string
	BaseURL    string
	HTTPClient *http.Client
}

// ModelNames is the canonical list of model pricing files.
var ModelNames = []string{
	"openai.yml",
	"anthropic.yml",
	"google.yml",
	"cursor.yml",
	"moonshot.yml",
	"zhipu.yml",
	"xai.yml",
}

// Run fetches updated model pricing files and writes them to .workflow-hero/models/.
func Run(opts Options, stdout, stderr io.Writer) error {
	if opts.BaseURL == "" {
		opts.BaseURL = defaultBaseURL
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}

	modelsDir := filepath.Join(opts.ProjectDir, cursoradapter.HeroModelsDir)
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		return fmt.Errorf("create models directory: %w", err)
	}

	checksums, err := install.LoadChecksums(opts.ProjectDir)
	if err != nil {
		return fmt.Errorf("load checksums: %w", err)
	}

	baseURL := strings.TrimRight(opts.BaseURL, "/")
	now := time.Now()

	var errs []string
	for _, name := range ModelNames {
		url := baseURL + "/" + name
		data, err := fetchURL(opts.HTTPClient, url)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			fmt.Fprintf(stderr, "[WARN] Failed to fetch %s: %v\n", name, err)
			continue
		}

		dest := filepath.Join(modelsDir, name)
		relKey, _ := filepath.Rel(opts.ProjectDir, dest)
		newHash := assetconflict.SHA256Hex(data)

		existingData, readErr := os.ReadFile(dest)
		if readErr == nil {
			originalHash := checksums[relKey]
			if assetconflict.IsCustomized(existingData, originalHash) && assetconflict.SHA256Hex(existingData) != newHash {
				if _, err := assetconflict.Replace(dest, existingData, data, relKey, stderr, now); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", name, err))
					continue
				}
				checksums[relKey] = newHash
				fmt.Fprintf(stdout, "[OK] Updated %s\n", name)
				continue
			}
		}

		if err := os.WriteFile(dest, data, 0o644); err != nil {
			errs = append(errs, fmt.Sprintf("%s: write error: %v", name, err))
			continue
		}
		checksums[relKey] = newHash
		fmt.Fprintf(stdout, "[OK] Updated %s\n", name)
	}

	if err := install.WriteChecksums(opts.ProjectDir, checksums); err != nil {
		return fmt.Errorf("write checksums: %w", err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("some model files could not be updated: %s", strings.Join(errs, "; "))
	}
	return nil
}

func fetchURL(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return data, nil
}
