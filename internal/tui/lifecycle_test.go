package tui

import (
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
)

func TestLifecycleEventForUnknownCycleIsIgnored(t *testing.T) {
	svc := newTestServiceWithRunningResearch(t)
	cycleRow, err := svc.Store.GetActiveCycle()
	if err != nil {
		t.Fatal(err)
	}
	m := withDefaultChatModel(NewTestModel(svc))
	var outbound []string
	m.telegram = &telegramState{
		connected: true,
		paired:    true,
		address:   "hero_1",
		recordOutbound: func(text string) {
			outbound = append(outbound, text)
		},
	}

	unknown := conversation.Event{
		EventID:   5001,
		Kind:      conversation.EventStageStarted,
		CycleID:   cycleRow.ID + 1000,
		StageName: "research",
	}
	next, cmd := m.handleLifecycleEvent(unknown)
	if cmd != nil {
		t.Fatalf("unknown-cycle event produced cmd=%v", cmd)
	}
	if len(outbound) != 0 {
		t.Fatalf("unknown-cycle event reached Telegram: %q", outbound)
	}
	if len(next.pendingLifecycleEvents) != 0 {
		t.Fatalf("unknown-cycle event was queued: %+v", next.pendingLifecycleEvents)
	}

	known := conversation.Event{
		EventID:   5002,
		Kind:      conversation.EventStageStarted,
		CycleID:   cycleRow.ID,
		StageName: "research",
	}
	next, _ = next.handleLifecycleEvent(known)
	if len(outbound) != 1 || outbound[0] != "Stage started: research" {
		t.Fatalf("known-cycle event did not reach Telegram: %q", outbound)
	}
	if len(next.pendingLifecycleEvents) != 0 {
		t.Fatalf("known-cycle event was queued: %+v", next.pendingLifecycleEvents)
	}
}
