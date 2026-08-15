package tui

import "testing"

func TestAgentShortLabel(t *testing.T) {
	cases := map[string]string{
		"orchestration_agent": "ORCH",
		"backend_agent":       "BACK",
		"frontend_agent":      "FRNT",
		"generic_agent":       "GEN",
		"qa_agent":            "QA",
		"judge_agent":         "JUDG",
		"planning_agent":      "PLAN",
		"discover_agent":      "DISC",
		"context_agent":       "CTX",
		"browser_ui_agent":    "BUI",
		"end2end_qa_agent":    "E2E",
		"":                    "HARN",
		"explore":             "HARN",
		"bash":                "HARN",
		"Task qa_agent":       "QA",
	}
	for name, want := range cases {
		if got := agentShortLabel(name); got != want {
			t.Errorf("agentShortLabel(%q)=%q want %q", name, got, want)
		}
	}
}

func TestFormatAgentHeader(t *testing.T) {
	if got := formatAgentHeader("orchestration_agent", "composer-2.5"); got != "[ORCH - composer-2.5]" {
		t.Fatalf("parent orch=%q", got)
	}
	if got := formatAgentHeader("qa_agent", "composer-2.5"); got != "[QA - composer-2.5]" {
		t.Fatalf("sub qa=%q", got)
	}
	if got := formatAgentHeader("", "grok-4.6"); got != "[HARN - grok-4.6]" {
		t.Fatalf("freechat=%q", got)
	}
	if got := formatAgentHeader("explore", "composer-2.5"); got != "[HARN - composer-2.5]" {
		t.Fatalf("harness-native=%q", got)
	}
	if got := formatAgentHeader("backend_agent", ""); got != "[BACK]" {
		t.Fatalf("no model=%q", got)
	}
}

func TestWrapAgentLabels(t *testing.T) {
	got := wrapAgentLabels([]string{"ORCH", "BACK"}, 24)
	if got != "ORCH | BACK" {
		t.Fatalf("got %q", got)
	}
	if wrapAgentLabels(nil, 24) != "" {
		t.Fatal("expected empty wrap")
	}
}

func TestIsKnownHeroAgent(t *testing.T) {
	if !isKnownHeroAgent("planning_agent") || !isKnownHeroAgent("Task qa_agent") {
		t.Fatal("expected named Hero agents")
	}
	if isKnownHeroAgent("") || isKnownHeroAgent("explore") || isKnownHeroAgent("generalPurpose") {
		t.Fatal("generic/empty names are not Hero agents")
	}
}
