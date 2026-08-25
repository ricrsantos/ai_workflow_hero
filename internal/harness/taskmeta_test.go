package harness

import "testing"

func TestHeroAgentFromLabel(t *testing.T) {
	if got := HeroAgentFromLabel("Task qa_agent"); got != "qa_agent" {
		t.Fatalf("got %q", got)
	}
	if got := HeroAgentFromLabel("context_agent"); got != "context_agent" {
		t.Fatalf("got %q", got)
	}
	if HeroAgentFromLabel("explore") != "" {
		t.Fatal("generic Task types are not Hero agents")
	}
}

func TestIsGenericTaskType(t *testing.T) {
	for _, name := range []string{"explore", "generalPurpose", "general_purpose", "shell", "bash", "task"} {
		if !IsGenericTaskType(name) {
			t.Fatalf("%q should be generic", name)
		}
	}
	if IsGenericTaskType("planning_agent") || IsGenericTaskType("context_agent") {
		t.Fatal("named Hero agents are not generic Tasks")
	}
}

func TestIsTaskToolName(t *testing.T) {
	if !IsTaskToolName("task") || !IsTaskToolName("collabToolCall") {
		t.Fatal("expected Task tool names")
	}
	if IsTaskToolName("read") || IsTaskToolName("status") {
		t.Fatal("ordinary tools are not Task launches")
	}
}
