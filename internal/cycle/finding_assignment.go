package cycle

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// FindingAssignmentMarkdown is the scheduler-owned find-* block shown to
// Implementation. Residual and repro.source are never truncated. The repro
// section is rendered for the mode the finding actually locked, so a
// command-mode or evidence-only finding never tells the agent to write a Go
// test it cannot write.
func FindingAssignmentMarkdown(f store.Finding, occs []store.FindingOccurrence, policy findingrepro.Policy) string {
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
	writeFindingReproSection(&b, f, occs, policy)
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

func writeFindingReproSection(b *strings.Builder, f store.Finding, occs []store.FindingOccurrence, policy findingrepro.Policy) {
	mode, pkg, test, source := store.LatestFindingReproWithMode(f, occs)
	switch {
	case mode == findingrepro.ModeEvidence:
		fmt.Fprintf(b, "  Repro: evidence-only — this failure has no deterministic re-run, so the scheduler cannot gate it with a command.\n")
		if evidence := findingEvidenceList(f.EvidenceJSON); len(evidence) > 0 {
			fmt.Fprintf(b, "    Evidence: %s\n", strings.Join(evidence, ", "))
		}
		fmt.Fprintf(b, "    Reproduce from that evidence, fix Residual, and state in your report how you verified the fix. Do not report this ID in tasks_completed while Residual is still true.\n")
	case mode == findingrepro.ModeCommand && pkg != "" && test != "":
		argv, err := policy.CommandArgv(pkg, test)
		rendered := strings.Join(argv, " ")
		fmt.Fprintf(b, "  Repro:\n")
		fmt.Fprintf(b, "    Mode: command\n")
		fmt.Fprintf(b, "    Target: %s\n", pkg)
		fmt.Fprintf(b, "    Test: %s\n", test)
		if err != nil || rendered == "" {
			fmt.Fprintf(b, "    The project repro command is not configured (verification.repro.command in workflow-config.yml); Hero cannot lock done automatically until it is.\n")
		} else {
			fmt.Fprintf(b, "    Land this test first (it MUST fail on current code). Then fix until `%s` passes. Do not report this ID in tasks_completed if Residual is still true or that command fails. The validation agent does not write the test file — you land it.\n", rendered)
		}
		writeFindingReproSource(b, source)
	case pkg == "" || test == "":
		fmt.Fprintf(b, "  Repro: missing — the next validation report must include repro.package, repro.test, and repro.source. Until then Hero cannot lock done automatically.\n")
	default:
		fmt.Fprintf(b, "  Repro:\n")
		fmt.Fprintf(b, "    Mode: go_test\n")
		fmt.Fprintf(b, "    Package: %s\n", pkg)
		fmt.Fprintf(b, "    Test: %s\n", test)
		fmt.Fprintf(b, "    Land this test first (it MUST fail on current code). Then fix until `go test %s -count=1 -run ^%s$` passes. Do not report this ID in tasks_completed if Residual is still true or that command fails. The validation agent does not write the test file — you land repro.source.\n", pkg, test)
		writeFindingReproSource(b, source)
	}
}

func writeFindingReproSource(b *strings.Builder, source string) {
	if strings.TrimSpace(source) == "" {
		return
	}
	fmt.Fprintf(b, "    Source:\n")
	for _, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		fmt.Fprintf(b, "      %s\n", line)
	}
}

func findingEvidenceList(evidenceJSON string) []string {
	if strings.TrimSpace(evidenceJSON) == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(evidenceJSON), &out); err != nil {
		return nil
	}
	return out
}
