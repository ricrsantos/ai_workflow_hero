package cursor_test

import (
	"context"
	"strings"
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestParseModelsOutput(t *testing.T) {
	raw := []byte(`Available models

auto - Auto (default)
composer-2.5 - Composer 2.5 (current)
composer-2.5-fast - Composer 2.5 Fast
cursor-grok-4.5-high - Cursor Grok 4.5

`)
	got := cursoradapter.ParseModelsOutput(raw)
	want := []string{"auto", "composer-2.5", "composer-2.5-fast", "cursor-grok-4.5-high"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestParseModelsOutput_DedupAndBare(t *testing.T) {
	raw := []byte("composer-2.5\ncomposer-2.5 - Composer\ngpt-5.5-high\n")
	got := cursoradapter.ParseModelsOutput(raw)
	if len(got) != 2 || got[0] != "composer-2.5" || got[1] != "gpt-5.5-high" {
		t.Fatalf("got %v", got)
	}
}

func TestParseModelsOutput_Empty(t *testing.T) {
	if got := cursoradapter.ParseModelsOutput(nil); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if got := cursoradapter.ParseModelsOutput([]byte("Available models\n\n")); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestListModels_UsesModelsSubcommand(t *testing.T) {
	dir := withCursorAssets(t)
	stdout := []byte("Available models\n\ncomposer-2.5 - Composer 2.5\nauto - Auto\n")
	var gotArgs []string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{{
		matchArgs: func(args []string) bool {
			return len(args) > 0 && args[0] == "models"
		},
		result:  cursoradapter.RunResult{Stdout: stdout},
		capture: &gotArgs,
	}}}

	models, err := adapter.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "composer-2.5" || models[1] != "auto" {
		t.Fatalf("models=%v", models)
	}
	if len(gotArgs) == 0 || gotArgs[0] != "models" {
		t.Fatalf("args=%v", gotArgs)
	}
}

func TestListModels_FallsBackToListModelsFlag(t *testing.T) {
	dir := withCursorAssets(t)
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{
		{
			matchArgs: func(args []string) bool {
				return len(args) > 0 && args[0] == "models"
			},
			result: cursoradapter.RunResult{Stdout: []byte("Available models\n")},
		},
		{
			matchArgs: func(args []string) bool {
				return containsArg(args, "--list-models")
			},
			result: cursoradapter.RunResult{Stdout: []byte("composer-2.5 - Composer\n")},
		},
	}}

	models, err := adapter.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0] != "composer-2.5" {
		t.Fatalf("models=%v", models)
	}
}

func TestListModels_ErrorOnEmpty(t *testing.T) {
	dir := withCursorAssets(t)
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/bin/cursor-agent", nil }
	adapter.Runner = &fakeRunner{t: t, handlers: []fakeCall{
		{matchArgs: func(args []string) bool { return len(args) > 0 && args[0] == "models" }, result: cursoradapter.RunResult{Stdout: []byte("")}},
		{matchArgs: func(args []string) bool { return containsArg(args, "--list-models") }, result: cursoradapter.RunResult{Stdout: []byte("")}},
	}}
	_, err := adapter.ListModels(context.Background())
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err=%v", err)
	}
}

var _ harness.ModelLister = (*cursoradapter.Adapter)(nil)
