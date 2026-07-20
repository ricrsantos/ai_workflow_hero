package variables_test

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/variables"
)

func makeInstalledDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	var sb strings.Builder
	if err := install.Run(install.Options{
		ProjectDir: dir,
		Name:       "Var Test Project",
		Summary:    "Testing variables",
		Tools:      []string{"cursor"},
		Version:    "1.2.3",
		GitInit:    false,
		AssetsFS:   assets.FS,
	}, &sb, &sb); err != nil {
		t.Fatalf("install: %v", err)
	}
	return dir
}

func TestVariables_KeyFields(t *testing.T) {
	dir := makeInstalledDir(t)

	vars, err := variables.Run(variables.Options{ProjectDir: dir})
	if err != nil {
		t.Fatalf("variables.Run: %v", err)
	}

	findVar := func(key string) string {
		for _, v := range vars.Items {
			if v.Key == key {
				return v.Value
			}
		}
		return ""
	}

	if v := findVar("cli.version"); v != "1.2.3" {
		t.Errorf("cli.version = %q, want %q", v, "1.2.3")
	}
	if v := findVar("project.name"); v != "Var Test Project" {
		t.Errorf("project.name = %q, want %q", v, "Var Test Project")
	}
	if v := findVar("project.summary"); v != "Testing variables" {
		t.Errorf("project.summary = %q, want %q", v, "Testing variables")
	}
	if v := findVar("project.workflow.cycle"); v != "0" {
		t.Errorf("project.workflow.cycle = %q, want %q", v, "0")
	}
}

func TestVariables_JSONRoundtrip(t *testing.T) {
	dir := makeInstalledDir(t)

	vars, _ := variables.Run(variables.Options{ProjectDir: dir})

	data, err := json.Marshal(vars)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded variables.Variables
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(decoded.Items) != len(vars.Items) {
		t.Errorf("item count mismatch: got %d, want %d", len(decoded.Items), len(vars.Items))
	}
}

func TestVariables_TableOutput(t *testing.T) {
	dir := makeInstalledDir(t)

	vars, _ := variables.Run(variables.Options{ProjectDir: dir})

	var sb strings.Builder
	variables.PrintTable(&sb, vars)
	out := sb.String()

	if !strings.Contains(out, "cli.version") {
		t.Errorf("table missing cli.version: %s", out)
	}
	if !strings.Contains(out, "project.name") {
		t.Errorf("table missing project.name: %s", out)
	}
}
