package upgrade

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
)

var genericModelLine = regexp.MustCompile(`(?m)^generic_model:\s*["']?([^"'\r\n#]+?)["']?\s*(#.*)?$`)

const workflowConfigMigrationHint = `Replace generic_model with:

fallback_model:
  model: <model-id>
  reasoning_effort: medium
  enable_fast_model: false
  thinking: na`

// migrateLegacyWorkflowConfigs renames legacy generic_model to fallback_model in
// per-cycle workflow-config.yml files (and the templates copy if still present).
func migrateLegacyWorkflowConfigs(projectDir string, stderr io.Writer) ([]string, error) {
	var migrated []string

	roots := []string{
		filepath.Join(projectDir, cursoradapter.HeroCyclesDir),
		filepath.Join(projectDir, cursoradapter.HeroTemplatesDir),
	}

	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return migrated, err
		}

		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != "workflow-config.yml" {
				return nil
			}

			rel, err := filepath.Rel(projectDir, path)
			if err != nil {
				return err
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", rel, err)
			}

			content := string(data)
			if !strings.Contains(content, "generic_model:") {
				return nil
			}

			if strings.Contains(content, "fallback_model:") {
				output.Warningf(stderr, "%s still uses generic_model and already defines fallback_model; merge manually.", rel)
				output.Warning(stderr, workflowConfigMigrationHint)
				return nil
			}

			updated, ok := migrateGenericModelYAML(content)
			if !ok {
				output.Warningf(stderr, "%s uses generic_model in an unsupported format; update manually.", rel)
				output.Warning(stderr, workflowConfigMigrationHint)
				return nil
			}

			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", rel, err)
			}

			migrated = append(migrated, rel)
			return nil
		})
		if err != nil {
			return migrated, err
		}
	}

	return migrated, nil
}

func migrateGenericModelYAML(content string) (string, bool) {
	match := genericModelLine.FindStringSubmatch(content)
	if match == nil {
		return content, false
	}

	modelID := strings.TrimSpace(match[1])
	replacement := fmt.Sprintf(`fallback_model:
  model: %s
  reasoning_effort: medium
  enable_fast_model: false
  thinking: na`, modelID)

	return genericModelLine.ReplaceAllString(content, replacement), true
}
