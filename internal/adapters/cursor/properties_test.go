package cursor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestComposeModelSlugFastOnly(t *testing.T) {
	got := ComposeModelSlug("composer-2.5", map[string]string{"fs": "true"})
	if got != "composer-2.5-fast" {
		t.Fatalf("got %q", got)
	}
	if got := ComposeModelSlug("composer-2.5", map[string]string{"fs": "false"}); got != "composer-2.5" {
		t.Fatalf("fs=false must not suffix: %q", got)
	}
	if got := ComposeModelSlug("composer-2.5", nil); got != "composer-2.5" {
		t.Fatalf("no props must not suffix: %q", got)
	}
}

func TestComposeModelSlugNeverDoubleSuffixed(t *testing.T) {
	// Workflow-composed slugs already carry the suffix (workflowconfig.ResolveModelSlug).
	got := ComposeModelSlug("grok-4.6-fast", map[string]string{"fs": "true"})
	if got != "grok-4.6-fast" {
		t.Fatalf("double fast suffix: %q", got)
	}
	got = ComposeModelSlug("grok-4.6-high-thinking", map[string]string{"ef": "high", "th": "true"})
	if got != "grok-4.6-high-thinking" {
		t.Fatalf("double thinking/effort suffix: %q", got)
	}
	got = ComposeModelSlug("grok-4.6", map[string]string{"ef": "high", "th": "true", "fs": "true"})
	if got != "grok-4.6-fast-thinking-high" {
		t.Fatalf("combined composition: %q", got)
	}
}

func TestComposeModelSlugDynamicValues(t *testing.T) {
	got := ComposeModelSlug("composer-2.5", map[string]string{"th": "max"})
	if got != "composer-2.5-thinking-max" {
		t.Fatalf("dynamic thinking value: %q", got)
	}
	if got := ComposeModelSlug("composer-2.5", map[string]string{"th": "off"}); got != "composer-2.5" {
		t.Fatalf("th=off must not suffix: %q", got)
	}
	if got := ComposeModelSlug("composer-2.5", map[string]string{"ef": "na"}); got != "composer-2.5" {
		t.Fatalf("ef=na must not suffix: %q", got)
	}
	if got := ComposeModelSlug("composer-2.5", map[string]string{"future_key": "x"}); got != "composer-2.5" {
		t.Fatalf("future keys must not affect the slug: %q", got)
	}
	if got := ComposeModelSlug("", map[string]string{"fs": "true"}); got != "" {
		t.Fatalf("empty model stays empty: %q", got)
	}
}

// rejectingRunner fails every run with a model-rejection style output.
type rejectingRunner struct {
	attempts int
	model    string
}

func (r *rejectingRunner) Run(_ context.Context, _ string, _ string, args []string) (RunResult, error) {
	r.attempts++
	model := r.model
	for i, a := range args {
		if a == "--model" && i+1 < len(args) {
			model = args[i+1]
		}
	}
	return RunResult{
		Stdout: []byte(`{"result":"failed"}`),
		Stderr: []byte(fmt.Sprintf("Error: unknown model %q (not in the catalog)", model)),
	}, errors.New("agent exited 1")
}

func TestExecuteRejectsComposedPropertyExplicitly(t *testing.T) {
	adapter := NewAdapter(t.TempDir())
	adapter.LookPath = func(string) (string, error) { return "/fake/cursor-agent", nil }
	adapter.RetryMax = 1
	adapter.VerifyAgent = func(context.Context, string) error { return nil }
	adapter.Runner = &rejectingRunner{model: "composer-2.5-fast"}

	_, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		ProjectDir: adapter.ProjectDir,
		Prompt:     "hi",
		Model:      "composer-2.5",
		Properties: map[string]string{"fs": "true"},
	})
	if err == nil {
		t.Fatal("execute must fail")
	}
	if !harness.IsPropertyRejection(err) {
		t.Fatalf("error must be a property rejection, got: %v", err)
	}
	var pre *harness.PropertyRejectionError
	if !errors.As(err, &pre) {
		t.Fatal("unwrap must expose PropertyRejectionError")
	}
	if pre.Property != "fs" || pre.Model != "composer-2.5-fast" || pre.Harness != "cursor" {
		t.Fatalf("rejection fields: %+v", pre)
	}
	// No silent retry without the property: the runner saw exactly one attempt.
	if adapter.Runner.(*rejectingRunner).attempts != 1 {
		t.Fatalf("attempts=%d want 1 (no retry after rejection)", adapter.Runner.(*rejectingRunner).attempts)
	}
}

func TestExecuteWithoutPropertiesKeepsExistingErrorBehavior(t *testing.T) {
	adapter := NewAdapter(t.TempDir())
	adapter.LookPath = func(string) (string, error) { return "/fake/cursor-agent", nil }
	adapter.RetryMax = 1
	adapter.Runner = &rejectingRunner{model: "composer-2.5"}

	_, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		ProjectDir: adapter.ProjectDir,
		Prompt:     "hi",
		Model:      "composer-2.5",
	})
	if err == nil {
		t.Fatal("execute must fail")
	}
	if harness.IsPropertyRejection(err) {
		t.Fatalf("no properties set: rejection must stay a plain execute error, got %v", err)
	}
	if !strings.Contains(err.Error(), "cursor agent execute failed") {
		t.Fatalf("C4 error shape must remain: %v", err)
	}
}

func TestPropertyRejectionForOutputIgnoresUnrelatedFailures(t *testing.T) {
	err := errors.New("network down")
	if got := propertyRejectionForOutput("composer-2.5-fast", map[string]string{"fs": "true"}, "", "connection refused", err); got != nil {
		t.Fatalf("unrelated failure must not become a rejection: %v", got)
	}
	// Model mention without rejection wording is not a property rejection either.
	if got := propertyRejectionForOutput("composer-2.5-fast", map[string]string{"fs": "true"}, "using composer-2.5-fast", "timeout", err); got != nil {
		t.Fatalf("timeout must not become a rejection: %v", got)
	}
}
