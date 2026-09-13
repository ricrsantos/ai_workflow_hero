package cycle

import (
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// FindingAssignmentMarkdown is the scheduler-owned find-* block shown to
// Implementation. Residual and repro.source are never truncated.
func FindingAssignmentMarkdown(f store.Finding, occs []store.FindingOccurrence) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- [ ] %s · finding · %s\n", f.ID, f.SourceStage)
	if file := strings.TrimSpace(f.File); file != "" {
		fmt.Fprintf(&b, "  File: %s\n", file)
	}
	if req := strings.TrimSpace(f.Requirement); req != "" {
		fmt.Fprintf(&b, "  Requirement: %s\n", req)
	}
	fmt.Fprintf(&b, "  Issue: %s\n", strings.TrimSpace(f.Issue))
	fmt.Fprintf(&b, "  Acceptance: %s\n", strings.TrimSpace(f.AcceptanceCriteria))
	writeLabeledMultiline(&b, "Residual", store.LatestFindingResidual(f, occs))
	fmt.Fprintf(&b, "  Contract: frozen — reopen this ID only for the same file, requirement, acceptance, and repro test; a different residual or different test is a new find-* ID. Fix Residual; Issue is identity only.\n")
	pkg, test, source := store.LatestFindingRepro(f, occs)
	if pkg == "" || test == "" {
		fmt.Fprintf(&b, "  Repro: missing — the next validation report must include repro.package, repro.test, and repro.source. Until then Hero cannot lock done via go test.\n")
	} else {
		fmt.Fprintf(&b, "  Repro:\n")
		fmt.Fprintf(&b, "    Package: %s\n", pkg)
		fmt.Fprintf(&b, "    Test: %s\n", test)
		fmt.Fprintf(&b, "    Land this test first (it MUST fail on current code). Then fix until `go test %s -count=1 -run ^%s$` passes. Do not report this ID in tasks_completed if Residual is still true or that command fails. QA does not write the test file — you land repro.source.\n", pkg, test)
		if strings.TrimSpace(source) != "" {
			fmt.Fprintf(&b, "    Source:\n")
			for _, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
				fmt.Fprintf(&b, "      %s\n", line)
			}
		}
	}
	if len(occs) > 0 {
		b.WriteString("  History:\n")
		for _, o := range occs {
			issue := strings.TrimSpace(o.Issue)
			if len(issue) > 160 {
				issue = issue[:157] + "..."
			}
			if issue == "" {
				fmt.Fprintf(&b, "    r%d %s\n", o.Round, o.Kind)
				continue
			}
			fmt.Fprintf(&b, "    r%d %s: %s\n", o.Round, o.Kind, issue)
		}
	}
	return b.String()
}

func writeLabeledMultiline(b *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		fmt.Fprintf(b, "  %s:\n", label)
		return
	}
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	fmt.Fprintf(b, "  %s: %s\n", label, lines[0])
	for _, line := range lines[1:] {
		fmt.Fprintf(b, "    %s\n", line)
	}
}
