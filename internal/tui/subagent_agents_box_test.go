package tui

import (
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func subagentStart(callID, name string) harness.StreamDelta {
	return harness.StreamDelta{Kind: harness.StreamKindTool, AgentName: name, CallID: callID, Phase: harness.StreamPhaseStarted, Subagent: true}
}

// Any harness-reported subagent appears, whatever its name; ordinary tool
// lifecycle never does, even when its text looks like a Task.
func TestAgentsBoxListsExactlyFlaggedSubagents(t *testing.T) {
	m := NewTestModel(nil)
	m.liveAgents = []liveAgent{{CallID: "ex-1", Name: "generic_agent", Label: "GEN"}}

	m = m.trackSubagentDelta(subagentStart("codex-agent:t1", "alpha"), "ex-1")
	m = m.trackSubagentDelta(subagentStart("toolu_1", "general-purpose"), "ex-1")
	m = m.trackSubagentDelta(subagentStart("call-q", "qa_agent"), "ex-1")
	m = m.trackSubagentDelta(harness.StreamDelta{Kind: harness.StreamKindTool, Text: "Task lookalike", AgentName: "planning_agent", CallID: "tool-9", Phase: harness.StreamPhaseStarted}, "ex-1")
	m = m.trackSubagentDelta(subagentStart("codex-agent:t1", "alpha"), "ex-1") // duplicate edge

	got := LiveAgentsForTest(m)
	if len(got) != 4 {
		t.Fatalf("agents=%+v want GEN + 3 subagents", got)
	}
	if got[1].Label != "TASK" || got[2].Label != "TASK" || got[3].Label != "QA" || got[1].Parent != "ex-1" {
		t.Fatalf("agents=%+v", got)
	}

	m = m.trackSubagentDelta(harness.StreamDelta{CallID: "toolu_1", Phase: harness.StreamPhaseCompleted, Subagent: true}, "ex-1")
	if got := LiveAgentsForTest(m); len(got) != 3 {
		t.Fatalf("completed subagent must leave: %+v", got)
	}
	// The parent Execute finishing removes its remaining subagents.
	m = m.removeLiveAgent("ex-1")
	if got := LiveAgentsForTest(m); len(got) != 0 {
		t.Fatalf("parent end must clear its subagents: %+v", got)
	}
}

// Validation stages reduce tool rows to progress markers; their subagents must
// still reach the Agents box, running on the parent's harness.
func TestAgentsBoxListsSubagentsOfValidationStage(t *testing.T) {
	m := NewTestModel(nil)
	m = EnterConversationForTest(m)
	m.streaming = true
	m.executes = map[string]convExecute{"ex-qa": {StageName: stageQA, HarnessID: "codex", AgentName: agentQA}}
	m.liveAgents = []liveAgent{{CallID: "ex-qa", Name: agentQA, Label: "QA"}}

	next, _ := m.Update(streamDeltaMsg{executeID: "ex-qa", delta: subagentStart("codex-agent:t1", "lint")})

	got := LiveAgentsForTest(next.(model))
	if len(got) != 2 || got[1].Harness != "codex" || got[1].Parent != "ex-qa" {
		t.Fatalf("agents=%+v want QA + lint subagent on codex", got)
	}
}
