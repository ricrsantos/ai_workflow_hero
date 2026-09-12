package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

var escalatedActionLines = map[string]string{
	"hero-continue": "/hero-continue       grant more iterations",
	"hero-add-todo": "/hero-add-todo       choose findings to defer",
	"hero-cancel":   "/hero-cancel         cancel and roll back",
	"hero-finish":   "/hero-finish         emergency finish without creating ToDos",
}

func (m model) appendStatusHandoffSections(b *strings.Builder) {
	if m.status.CycleNumber == 0 && len(m.status.Stages) == 0 {
		return
	}
	m.writeStatusLoopBacks(b)
	m.writeStatusFindings(b)
	m.writeStatusTodosSummary(b)
	m.writeStatusDisposition(b)
	m.writeStatusEscalatedCTAs(b)
}

func (m model) writeStatusLoopBacks(b *strings.Builder) {
	if len(m.status.LoopBacks) == 0 {
		return
	}
	b.WriteByte('\n')
	b.WriteString(headerStyle.Render("Loop-backs"))
	b.WriteByte('\n')
	for _, lb := range m.status.LoopBacks {
		from := statusSourceStageLabel(lb.From)
		to := statusSourceStageLabel(lb.To)
		if to == "" {
			to = "Implementation"
		}
		ids := strings.Join(lb.FindingIDs, ", ")
		if ids == "" {
			ids = "—"
		}
		line := fmt.Sprintf("%s → %s · round %d · %s · %s",
			from, to, lb.Round, formatEventTimeLocal(lb.OccurredAt), ids)
		b.WriteString(mutedStyle.Render(line))
		b.WriteByte('\n')
	}
}

func (m model) writeStatusFindings(b *strings.Builder) {
	b.WriteByte('\n')
	block := m.status.Findings
	if block == nil || len(block.Items) == 0 {
		b.WriteString(mutedStyle.Render("Findings  none"))
		b.WriteByte('\n')
		return
	}
	counts := block.Counts
	header := fmt.Sprintf("Findings  open %d · reopened %d · done %d · ToDo %d",
		counts.Open, counts.Reopened, counts.Done, counts.DeferredTodo)
	b.WriteString(headerStyle.Render(header))
	b.WriteByte('\n')

	idW, srcW, ownerW, stateW := statusFindingColumnWidths(block.Items)
	headerRow := fmt.Sprintf(" %s %s %s %s %s %s",
		padRight("ID", idW),
		padRight("Source", srcW),
		padRight("Owner", ownerW),
		padRight("State", stateW),
		padRight("R", 3),
		"Issue",
	)
	b.WriteString(mutedStyle.Render(headerRow))
	b.WriteByte('\n')

	for i, row := range block.Items {
		state := statusFindingStateLabel(row.Status)
		issue := truncateStatusIssue(row.Issue, 48)
		line := fmt.Sprintf(" %s %s %s %s %s %s",
			padRight(row.ID, idW),
			padRight(statusSourceStageLabel(row.SourceStage), srcW),
			padRight(findingOwnerLabel(row.Owner), ownerW),
			padRight(state, stateW),
			padRight(fmt.Sprintf("%d", row.Round), 3),
			issue,
		)
		if i == m.statusFindingFocus {
			b.WriteString(infoStyle.Render(line))
			b.WriteByte('\n')
			m.writeStatusFindingDetail(b, row)
		} else {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
}

func (m model) writeStatusFindingDetail(b *strings.Builder, row cycle.StatusFindingRow) {
	b.WriteString(mutedStyle.Render("  status: " + row.Status))
	b.WriteByte('\n')
	if full := strings.TrimSpace(row.Issue); full != "" {
		b.WriteString(mutedStyle.Render("  issue: " + full))
		b.WriteByte('\n')
	}
	if m.svc == nil || m.svc.Store == nil {
		return
	}
	c, err := m.svc.Store.GetCurrentCycle()
	if err != nil {
		return
	}
	if c.Number != m.status.CycleNumber {
		return
	}
	f, err := m.svc.Store.GetFinding(c.ID, row.ID)
	if err != nil {
		return
	}
	if ac := strings.TrimSpace(f.AcceptanceCriteria); ac != "" {
		b.WriteString(mutedStyle.Render("  acceptance: " + ac))
		b.WriteByte('\n')
	}
	if occs, err := m.svc.Store.ListFindingOccurrences(c.ID, row.ID); err == nil && len(occs) > 0 {
		parts := make([]string, 0, len(occs))
		for _, o := range occs {
			parts = append(parts, fmt.Sprintf("%s(r%d)", o.Kind, o.Round))
		}
		b.WriteString(mutedStyle.Render("  occurrences: " + strings.Join(parts, ", ")))
		b.WriteByte('\n')
	}
	if evidence := formatFindingEvidenceSummary(f.EvidenceJSON); evidence != "" {
		b.WriteString(mutedStyle.Render("  evidence: " + evidence))
		b.WriteByte('\n')
	}
}

func formatFindingEvidenceSummary(evidenceJSON string) string {
	evidenceJSON = strings.TrimSpace(evidenceJSON)
	if evidenceJSON == "" || evidenceJSON == "[]" {
		return ""
	}
	var items []string
	if err := json.Unmarshal([]byte(evidenceJSON), &items); err != nil {
		return ""
	}
	if len(items) == 0 {
		return ""
	}
	return fmt.Sprintf("%d item(s)", len(items))
}

func (m model) writeStatusTodosSummary(b *strings.Builder) {
	if m.status.Todos == nil {
		return
	}
	t := m.status.Todos
	if t.Pending == 0 && t.Adopted == 0 && t.DeferredFromCycle == 0 {
		return
	}
	b.WriteByte('\n')
	line := fmt.Sprintf("ToDos  pending %d · adopted %d · deferred this cycle %d",
		t.Pending, t.Adopted, t.DeferredFromCycle)
	b.WriteString(headerStyle.Render(line))
	b.WriteByte('\n')
}

func (m model) writeStatusDisposition(b *strings.Builder) {
	if m.status.CompletionDisposition == nil {
		return
	}
	disp := strings.TrimSpace(*m.status.CompletionDisposition)
	if disp == "" {
		return
	}
	b.WriteByte('\n')
	b.WriteString(successStyle.Render("✓ Cycle C" + fmt.Sprintf("%d", m.status.CycleNumber) + " completed with deferred ToDos"))
	b.WriteByte('\n')
	b.WriteString(mutedStyle.Render("  disposition: " + disp))
	b.WriteByte('\n')
}

func (m model) writeStatusEscalatedCTAs(b *strings.Builder) {
	if len(m.status.AvailableActions) == 0 {
		return
	}
	b.WriteByte('\n')
	b.WriteString(headerStyle.Render("Escalated"))
	b.WriteByte('\n')
	for _, action := range m.status.AvailableActions {
		line := escalatedActionLines[action]
		if line == "" {
			line = "/" + action
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
}

func statusFindingStateLabel(status string) string {
	if status == store.FindingStatusDeferredTodo {
		return "ToDo"
	}
	return status
}

func statusFindingColumnWidths(items []cycle.StatusFindingRow) (idW, srcW, ownerW, stateW int) {
	idW, srcW, ownerW, stateW = len("ID"), len("Source"), len("Owner"), len("State")
	for _, row := range items {
		idW = max(idW, len(row.ID))
		srcW = max(srcW, len(statusSourceStageLabel(row.SourceStage)))
		ownerW = max(ownerW, len(findingOwnerLabel(row.Owner)))
		stateW = max(stateW, len(statusFindingStateLabel(row.Status)))
	}
	return idW + 1, srcW + 1, ownerW + 1, stateW + 1
}

func truncateStatusIssue(issue string, max int) string {
	issue = strings.TrimSpace(issue)
	if max <= 0 || len(issue) <= max {
		return issue
	}
	return issue[:max-3] + "..."
}

func (m model) statusFindingCount() int {
	if m.status.Findings == nil {
		return 0
	}
	return len(m.status.Findings.Items)
}

func (m model) handleStatusFindingKey(msg string) (model, bool) {
	if m.screen != screenStatus {
		return m, false
	}
	n := m.statusFindingCount()
	switch msg {
	case "esc":
		if m.statusFindingFocus >= 0 {
			m.statusFindingFocus = -1
			return m, true
		}
	case "enter":
		if n == 0 {
			return m, true
		}
		if m.statusFindingFocus < 0 {
			m.statusFindingFocus = 0
		} else {
			m.statusFindingFocus = (m.statusFindingFocus + 1) % n
		}
		return m, true
	case "up":
		if m.statusFindingFocus >= 0 && n > 0 {
			m.statusFindingFocus--
			if m.statusFindingFocus < 0 {
				m.statusFindingFocus = n - 1
			}
			return m, true
		}
	case "down":
		if m.statusFindingFocus >= 0 && n > 0 {
			m.statusFindingFocus = (m.statusFindingFocus + 1) % n
			return m, true
		}
	}
	return m, false
}
