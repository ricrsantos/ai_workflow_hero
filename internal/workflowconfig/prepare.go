package workflowconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"gopkg.in/yaml.v3"
)

const workflowConfigFileName = "workflow-config.yml"

var importedTopLevelKeys = []string{
	"workflow_config",
	"fallback_model",
	"stages",
	"agents",
}

// PreparationResult describes the active workflow configuration after
// EnsureCurrent returns. SourcePath is empty when the template was used
// without a previous archived cycle.
type PreparationResult struct {
	Path       string
	SourcePath string
	Created    bool
}

// EnsureCurrent validates the active workflow configuration and creates it
// deterministically when it is missing. New files start from the installed
// template and deep-merge the highest archived cycle's workflow_config,
// fallback_model, stages, and agents sections. Existing files are preserved.
func EnsureCurrent(projectDir string) (PreparationResult, error) {
	if strings.TrimSpace(projectDir) == "" {
		return PreparationResult{}, fmt.Errorf("project directory is required")
	}

	currentPath := filepath.Join(projectDir, cursoradapter.HeroCurrentCycleDir, workflowConfigFileName)
	if info, err := os.Stat(currentPath); err == nil {
		if !info.Mode().IsRegular() {
			return PreparationResult{}, fmt.Errorf("active workflow-config.yml is not a regular file: %s", currentPath)
		}
		if _, err := LoadDocument(currentPath); err != nil {
			return PreparationResult{}, fmt.Errorf("validate active workflow-config.yml: %w", err)
		}
		return PreparationResult{Path: currentPath}, nil
	} else if !os.IsNotExist(err) {
		return PreparationResult{}, fmt.Errorf("stat active workflow-config.yml: %w", err)
	}

	currentDir := filepath.Dir(currentPath)
	if err := os.MkdirAll(currentDir, 0o755); err != nil {
		return PreparationResult{}, fmt.Errorf("create active cycle directory: %w", err)
	}

	templatePath := filepath.Join(projectDir, cursoradapter.HeroTemplatesDir, workflowConfigFileName)
	templateRoot, err := loadMappingRoot(templatePath)
	if err != nil {
		return PreparationResult{}, fmt.Errorf("load workflow-config template: %w", err)
	}

	sourcePath, err := previousArchivedConfigPath(projectDir)
	if err != nil {
		return PreparationResult{}, err
	}
	if sourcePath != "" {
		sourceRoot, err := loadMappingRoot(sourcePath)
		if err != nil {
			return PreparationResult{}, fmt.Errorf("load previous workflow-config.yml: %w", err)
		}
		for _, key := range importedTopLevelKeys {
			mergeMappingKey(templateRoot, sourceRoot, key)
		}
	}

	document := yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{templateRoot}}
	encoded, err := yaml.Marshal(&document)
	if err != nil {
		return PreparationResult{}, fmt.Errorf("encode prepared workflow-config.yml: %w", err)
	}
	if err := validateEncodedDocument(encoded); err != nil {
		return PreparationResult{}, fmt.Errorf("validate prepared workflow-config.yml: %w", err)
	}
	if err := writeAtomic(currentPath, encoded); err != nil {
		return PreparationResult{}, fmt.Errorf("write prepared workflow-config.yml: %w", err)
	}
	if _, err := LoadDocument(currentPath); err != nil {
		return PreparationResult{}, fmt.Errorf("validate prepared workflow-config.yml: %w", err)
	}

	return PreparationResult{
		Path:       currentPath,
		SourcePath: sourcePath,
		Created:    true,
	}, nil
}

func loadMappingRoot(path string) (*yaml.Node, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parse %s: root must be a mapping", path)
	}
	return document.Content[0], nil
}

func validateEncodedDocument(raw []byte) error {
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return fmt.Errorf("parse encoded YAML: %w", err)
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("root must be a mapping")
	}
	var cfg ManagedConfig
	if err := document.Content[0].Decode(&cfg); err != nil {
		return fmt.Errorf("decode encoded YAML: %w", err)
	}
	return nil
}

type archivedConfigCandidate struct {
	name string
	num  int
	path string
}

func previousArchivedConfigPath(projectDir string) (string, error) {
	archiveRoot := filepath.Join(projectDir, cursoradapter.HeroCyclesDir, "archive")
	entries, err := os.ReadDir(archiveRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("list archived cycles: %w", err)
	}

	candidates := make([]archivedConfigCandidate, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		num, ok := archivedCycleNumber(entry.Name())
		if !ok {
			continue
		}
		path := filepath.Join(archiveRoot, entry.Name(), workflowConfigFileName)
		info, statErr := os.Stat(path)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return "", fmt.Errorf("stat archived workflow-config.yml: %w", statErr)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		candidates = append(candidates, archivedConfigCandidate{name: entry.Name(), num: num, path: path})
	}
	if len(candidates) == 0 {
		return "", nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].num != candidates[j].num {
			return candidates[i].num > candidates[j].num
		}
		return candidates[i].name > candidates[j].name
	})
	return candidates[0].path, nil
}

func archivedCycleNumber(name string) (int, bool) {
	if len(name) < 4 || name[0] != 'C' {
		return 0, false
	}
	dash := strings.IndexByte(name[1:], '-')
	if dash < 1 {
		return 0, false
	}
	digits := name[1 : dash+1]
	for _, r := range digits {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	num, err := strconv.Atoi(digits)
	return num, err == nil && num > 0
}

func mergeMappingKey(target, source *yaml.Node, key string) {
	sourceValue := mappingValue(source, key)
	if sourceValue == nil {
		return
	}
	targetValue := mappingValue(target, key)
	if targetValue == nil {
		target.Content = append(target.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			cloneYAMLNode(sourceValue),
		)
		return
	}
	mergeYAMLNode(targetValue, sourceValue)
}

func mergeYAMLNode(target, source *yaml.Node) {
	if target.Kind != yaml.MappingNode || source.Kind != yaml.MappingNode {
		replacement := cloneYAMLNode(source)
		*target = *replacement
		return
	}
	for i := 0; i+1 < len(source.Content); i += 2 {
		key := source.Content[i].Value
		sourceValue := source.Content[i+1]
		targetValue := mappingValue(target, key)
		if targetValue == nil {
			target.Content = append(target.Content, cloneYAMLNode(source.Content[i]), cloneYAMLNode(sourceValue))
			continue
		}
		mergeYAMLNode(targetValue, sourceValue)
	}
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := *node
	if node.Alias != nil {
		clone.Alias = cloneYAMLNode(node.Alias)
	}
	if len(node.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			clone.Content[i] = cloneYAMLNode(child)
		}
	}
	return &clone
}
