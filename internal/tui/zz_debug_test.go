package tui

import "testing"

func TestZZDebugWorkflowProps(t *testing.T) {
	svc, h := newConversationTestService(t)
	m := NewTestModel(svc)
	m = m.withRuntimeAgent(agentOrchestration)
	t.Logf("runtimeAgentName=%q workflowProps=%v", m.runtimeAgentName, m.workflowProps)
	m.runtimeModelSlug = "composer-2.5"
	m = SetConversationInput(m, "start")
	m2, cmd := SubmitConversationForTest(m)
	t.Logf("after submit: runtimeAgentName=%q workflowProps=%v", m2.runtimeAgentName, m2.workflowProps)
	msg := RunCmdForTest(cmd)
	t.Logf("cmd msg: %T", msg)
	if done, ok := msg.(executeDoneMsg); ok {
		t.Logf("done err=%v props=%v", done.err, h.lastProps)
	} else {
		// stream delta / other — apply stream messages
		for i := 0; i < 50 && IsConversationStreaming(m2); i++ {
			m2, _ = HandleTestMsg(m2, ExecuteDoneResultForTest(nil, nil))
		}
		t.Logf("lastProps=%v", h.lastProps)
	}
}
