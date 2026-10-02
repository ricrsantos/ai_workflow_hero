package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The Planning SDD check (ADR-107) rejects task contracts that an
// implementation agent cannot satisfy honestly. Every rule here mirrors a
// stall seen in practice; the check stays deliberately narrow so it never
// second-guesses design choices, only contracts that cannot be met.

var (
	sddVerifyRE     = regexp.MustCompile(`(?i)\bverif(y|ication)\b[^:\n]{0,40}:`)
	sddUnresolvedRE = regexp.MustCompile(`(?i)\bTBD\b|\bto be (decided|defined|confirmed)\b|\bopen question\b|\bpending (user |explicit )?decision\b|\bneeds? (an? )?(explicit )?(user )?decision\b`)
	// Implementation agents must not edit current-state.md (C15 report
	// contract); the orchestrator owns it at Stage Close / finish. Only an edit
	// verb in the same sentence counts, so a task may still say the file is
	// out of its scope. A sentence ends at a period followed by whitespace, so
	// file names such as `TESTING.md` do not split it.
	sddCurrentStateEditRE = regexp.MustCompile(`(?i)\b(update|edit|write|rewrite|refresh|modify|record)\w*\b(?:[^.\n]|\.\S){0,160}?current-state\.md`)
)

// lintImplementationSDD returns the problems that would stall Implementation.
// extraDocs holds the other change documents (name → content) scanned only for
// unresolved-decision markers.
func lintImplementationSDD(raw string, activeAgents []string, extraDocs map[string]string) []string {
	var problems []string
	tasks := parseImplementationTaskBlocks(raw)
	if len(tasks) == 0 {
		return []string{"tasks.md has no checklist tasks"}
	}
	plan := partitionImplementationTasks(raw, activeAgents)
	problems = append(problems, plan.Errors...)
	problems = append(problems, sddDependencyCycles(tasks)...)
	for _, task := range tasks {
		if task.ID == "" {
			continue // already reported by the partition
		}
		if !sddVerifyRE.MatchString(task.Block) {
			problems = append(problems, fmt.Sprintf("task %q has no executable verification criterion (add a \"Verify:\" command)", task.ID))
		}
		if sddCurrentStateEditRE.MatchString(task.Block) {
			problems = append(problems, fmt.Sprintf("task %q edits context/current-state.md, which implementation agents must not edit; the orchestrator updates it at Stage Close", task.ID))
		}
	}
	problems = append(problems, sddUnresolved("tasks.md", raw)...)
	names := make([]string, 0, len(extraDocs))
	for name := range extraDocs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		problems = append(problems, sddUnresolved(name, extraDocs[name])...)
	}
	return dedupeStrings(problems)
}

// sddDependencyCycles finds cycles across every task, checked or not. The wave
// partition only notices a cycle once nothing else is ready; Planning should
// fail fast instead.
func sddDependencyCycles(tasks []implementationTaskBlock) []string {
	deps := make(map[string][]string, len(tasks))
	for _, task := range tasks {
		if task.ID != "" {
			deps[task.ID] = task.After
		}
	}
	const (
		unvisited = iota
		visiting
		done
	)
	state := make(map[string]int, len(deps))
	var problems []string
	var visit func(id string, path []string)
	visit = func(id string, path []string) {
		switch state[id] {
		case done:
			return
		case visiting:
			start := 0
			for i, p := range path {
				if p == id {
					start = i
					break
				}
			}
			problems = append(problems, "dependency cycle: "+strings.Join(append(append([]string(nil), path[start:]...), id), " -> "))
			return
		}
		state[id] = visiting
		for _, dep := range deps[id] {
			if _, known := deps[dep]; known {
				visit(dep, append(path, id))
			}
		}
		state[id] = done
	}
	ids := make([]string, 0, len(deps))
	for id := range deps {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		visit(id, nil)
	}
	return problems
}

func sddUnresolved(name, content string) []string {
	var problems []string
	for i, line := range strings.Split(content, "\n") {
		if match := sddUnresolvedRE.FindString(line); match != "" {
			problems = append(problems, fmt.Sprintf("%s line %d leaves a decision open (%q); resolve it in Planning", name, i+1, match))
		}
	}
	return problems
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// planningSDDProblems lints the cycle's linked OpenSpec change. An unlinked
// cycle is left to the Implementation gate, which already refuses it.
func (m model) planningSDDProblems() []string {
	checklist := m.implementationChecklist()
	if !checklist.Linked {
		return nil
	}
	if !checklist.Ready {
		return []string{"linked OpenSpec tasks.md is missing or unreadable"}
	}
	dir := filepath.Dir(checklist.Path)
	extra := map[string]string{}
	for _, name := range []string{"design.md", "proposal.md"} {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			extra[name] = string(data)
		}
	}
	return lintImplementationSDD(checklist.Raw, m.implementationAgentsFromScope(), extra)
}

func formatPlanningSDDRejected(problems []string) string {
	var b strings.Builder
	b.WriteString("⚠ Planning SDD rejected by Hero checks\n")
	for _, p := range problems {
		b.WriteString("→ " + p + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func planningSDDFeedbackSection(problems []string) string {
	return "\n## Previous SDD rejected — fix tasks.md and the change documents\n\n" +
		"Hero checked the linked OpenSpec change and refused to close Planning. " +
		"Edit the existing change in place to resolve every item, then emit your Output Format again.\n\n" +
		"- " + strings.Join(problems, "\n- ") + "\n"
}
