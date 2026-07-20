package variables

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
)

// Variable is a key/value pair from the project config.
type Variable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Variables is the full variable set.
type Variables struct {
	Items []Variable `json:"variables"`
}

// Options holds variables command options.
type Options struct {
	ProjectDir string
}

// Run reads project.json and hero.json and returns key fields.
func Run(opts Options) (Variables, error) {
	var vars Variables

	// Read hero.json.
	heroData, err := os.ReadFile(filepath.Join(opts.ProjectDir, cursoradapter.HeroJSONPath))
	if err != nil && !os.IsNotExist(err) {
		return vars, fmt.Errorf("read hero.json: %w", err)
	}
	var heroJSON install.HeroJSON
	if err == nil {
		if jerr := json.Unmarshal(heroData, &heroJSON); jerr != nil {
			return vars, fmt.Errorf("parse hero.json: %w", jerr)
		}
		vars.Items = append(vars.Items,
			Variable{"cli.version", heroJSON.CLI.Version},
			Variable{"cli.installedAt", heroJSON.CLI.InstalledAt},
			Variable{"assets.version", heroJSON.Assets.Version},
			Variable{"assets.installedAt", heroJSON.Assets.InstalledAt},
		)
	}

	// Read project.json.
	projectData, err := os.ReadFile(filepath.Join(opts.ProjectDir, cursoradapter.ProjectJSONPath))
	if err != nil && !os.IsNotExist(err) {
		return vars, fmt.Errorf("read project.json: %w", err)
	}
	var projectJSON install.ProjectJSON
	if err == nil {
		if jerr := json.Unmarshal(projectData, &projectJSON); jerr != nil {
			return vars, fmt.Errorf("parse project.json: %w", jerr)
		}
		vars.Items = append(vars.Items,
			Variable{"project.name", projectJSON.Name},
			Variable{"project.summary", projectJSON.Summary},
			Variable{"project.createdAt", projectJSON.CreatedAt},
			Variable{"project.workflow.name", projectJSON.Workflow.Name},
			Variable{"project.workflow.phase", projectJSON.Workflow.Phase},
			Variable{"project.workflow.cycle", fmt.Sprintf("%d", projectJSON.Workflow.Cycle)},
		)
	}

	return vars, nil
}

// PrintTable writes a human-readable table of variables to w.
func PrintTable(w io.Writer, vars Variables) {
	if len(vars.Items) == 0 {
		output.Progressf(w, "No variables found. Is Hero installed in this project?")
		return
	}
	headers := []string{"Key", "Value"}
	var rows [][]string
	for _, v := range vars.Items {
		rows = append(rows, []string{v.Key, v.Value})
	}
	output.Table(w, headers, rows)
}
