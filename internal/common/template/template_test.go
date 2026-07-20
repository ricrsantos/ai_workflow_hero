package template_test

import (
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/template"
)

func TestRender_SimpleSubstitution(t *testing.T) {
	src := "Project: {{project.name}}"
	data := template.Data{
		"project": {"name": "Hero"},
	}
	got, err := template.Render(src, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Project: Hero" {
		t.Errorf("got %q, want %q", got, "Project: Hero")
	}
}

func TestRender_NestedPath(t *testing.T) {
	src := "Name: {{project.name}}, Summary: {{project.summary}}"
	data := template.Data{
		"project": {
			"name":    "Hero",
			"summary": "AI workflow tool",
		},
	}
	got, err := template.Render(src, data)
	if err != nil {
		t.Fatal(err)
	}
	want := "Name: Hero, Summary: AI workflow tool"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRender_UnknownKeyLeftUnchanged(t *testing.T) {
	src := "{{unknown.key}} stays"
	got, err := template.Render(src, template.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got != src {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestRender_NoLoopSupport(t *testing.T) {
	// Loop markers must be left as-is (ADR-006 — no loop engine).
	src := "{{#items}}{{item.name}}{{/items}}"
	got, err := template.Render(src, template.Data{})
	if err != nil {
		t.Fatal(err)
	}
	// The loop markers must remain intact in output.
	if !strings.Contains(got, "{{#items}}") {
		t.Errorf("loop marker was consumed; output: %q", got)
	}
	if !strings.Contains(got, "{{/items}}") {
		t.Errorf("closing loop marker was consumed; output: %q", got)
	}
}

func TestRender_MultipleNamespaces(t *testing.T) {
	src := "{{project.name}} v{{cli.version}}"
	data := template.Data{
		"project": {"name": "Hero"},
		"cli":     {"version": "1.0.0"},
	}
	got, err := template.Render(src, data)
	if err != nil {
		t.Fatal(err)
	}
	want := "Hero v1.0.0"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRender_EmptyString(t *testing.T) {
	got, err := template.Render("", template.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestRender_PlainTextNoPlaceholders(t *testing.T) {
	src := "Hello, world!"
	got, err := template.Render(src, template.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got != src {
		t.Errorf("got %q, want %q", got, src)
	}
}
