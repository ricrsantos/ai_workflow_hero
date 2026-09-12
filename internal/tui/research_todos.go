package tui

import (
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

type researchTodoSnapshot struct {
	CycleNumber int
	Objective   string
	Pending     []store.Todo
}

func (m model) researchTodoSnapshot() (researchTodoSnapshot, error) {
	var snap researchTodoSnapshot
	if m.svc == nil || m.svc.Store == nil {
		return snap, nil
	}
	c, err := m.svc.Store.GetActiveCycle()
	if err != nil {
		return snap, err
	}
	snap.CycleNumber = c.Number
	snap.Objective = strings.TrimSpace(c.Objective)
	pending, err := m.svc.ListPendingTodos()
	if err != nil {
		return snap, err
	}
	snap.Pending = pending
	return snap, nil
}

// researchPendingTodoPromptSection injects deterministic pending-ToDo context after
// idea notes and before grilling (PRD-C15-001 §9.3; UI-C15-001 §9).
func researchPendingTodoPromptSection(snap researchTodoSnapshot) string {
	var b strings.Builder
	b.WriteString("## Research pending ToDo adoption (mandatory before grilling)\n\n")
	b.WriteString("Hero pre-loaded pending project ToDos from hero.db. Follow UI-C15-001 §9:\n\n")
	b.WriteString("1. Emit: → Checking pending project ToDos\n")
	if len(snap.Pending) == 0 {
		b.WriteString("2. No items below — emit: ✓ No pending project ToDos\n")
		b.WriteString("3. Continue with ordinary requirements grilling.\n")
		return strings.TrimRight(b.String(), "\n")
	}
	b.WriteString("2. Present the list below in the user's chat language (one focused question at a time).\n")
	b.WriteString("3. Identify objective matches separately from other candidates; recommend leaving unrelated items out.\n")
	b.WriteString("4. After the user confirms selection, persist each ID with:\n")
	b.WriteString("   hero adopt-todo <id>... [--note \"...\"]\n")
	b.WriteString("5. On success emit: ✓ Adopted by C")
	b.WriteString(fmt.Sprintf("%d", snap.CycleNumber))
	b.WriteString(": <id> and → The item remains visible until this cycle validates it.\n")
	b.WriteString("6. If adoption fails, explain the error, offer retry, and do NOT finalize requirements or close Research until adoption succeeds or the user declines all items.\n")
	b.WriteString("7. Never claim adoption that is absent from hero.db state.\n")
	b.WriteString("8. Include adopted items in requirements handed to Planning.\n\n")
	if obj := snap.Objective; obj != "" {
		b.WriteString("Cycle objective (for matching):\n")
		b.WriteString(obj)
		b.WriteString("\n\n")
	}
	implied, other := partitionTodosByObjective(snap.Pending, snap.Objective)
	b.WriteString("Pending project ToDos (authoritative query):\n\n")
	if len(implied) > 0 {
		b.WriteString("Already implied by this cycle objective\n")
		for _, t := range implied {
			writeTodoLine(&b, t)
		}
		b.WriteByte('\n')
	}
	if len(other) > 0 {
		b.WriteString("Other candidates\n")
		for _, t := range other {
			writeTodoLine(&b, t)
		}
		b.WriteByte('\n')
	}
	b.WriteString("Ask: Do you want to add any other pending ToDo to this cycle?\n")
	b.WriteString("Recommendation: leave unrelated items out to keep the cycle focused.\n")
	return strings.TrimRight(b.String(), "\n")
}

func writeTodoLine(b *strings.Builder, t store.Todo) {
	b.WriteString("  ")
	b.WriteString(t.ID)
	b.WriteString(" · ")
	b.WriteString(strings.TrimSpace(t.Summary))
	if ac := strings.TrimSpace(t.AcceptanceCriteria); ac != "" {
		b.WriteString(" · ")
		b.WriteString(ac)
	}
	b.WriteByte('\n')
}

func partitionTodosByObjective(pending []store.Todo, objective string) (implied, other []store.Todo) {
	for _, t := range pending {
		if todoImpliedByObjective(t, objective) {
			implied = append(implied, t)
		} else {
			other = append(other, t)
		}
	}
	return implied, other
}

func todoImpliedByObjective(t store.Todo, objective string) bool {
	obj := strings.ToLower(strings.TrimSpace(objective))
	if obj == "" {
		return false
	}
	id := strings.ToLower(strings.TrimSpace(t.ID))
	if id != "" && strings.Contains(obj, id) {
		return true
	}
	summary := strings.ToLower(strings.TrimSpace(t.Summary))
	if summary != "" && strings.Contains(obj, summary) {
		return true
	}
	for _, tok := range strings.Fields(summary) {
		if len(tok) < 5 {
			continue
		}
		if strings.Contains(obj, tok) {
			return true
		}
	}
	return false
}

// researchPendingTodoSectionForService builds the adoption block for tests.
func researchPendingTodoSectionForService(svc *cycle.Service) (string, error) {
	m := model{svc: svc}
	snap, err := m.researchTodoSnapshot()
	if err != nil {
		return "", err
	}
	return researchPendingTodoPromptSection(snap), nil
}
