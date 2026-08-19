package install

import (
	"encoding/json"
	"os"
	"path/filepath"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
)

// LoadChecksums reads .workflow-hero/config/checksums.json from projectDir.
// A missing file yields an empty map.
func LoadChecksums(projectDir string) (Checksums, error) {
	path := filepath.Join(projectDir, cursoradapter.ChecksumsJSONPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(Checksums), nil
		}
		return nil, err
	}
	var checksums Checksums
	if err := json.Unmarshal(data, &checksums); err != nil {
		return nil, err
	}
	if checksums == nil {
		checksums = make(Checksums)
	}
	return checksums, nil
}

// WriteChecksums writes checksums to .workflow-hero/config/checksums.json.
func WriteChecksums(projectDir string, checksums Checksums) error {
	path := filepath.Join(projectDir, cursoradapter.ChecksumsJSONPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(checksums, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}
