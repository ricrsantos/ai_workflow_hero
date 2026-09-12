package cycle

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/reports"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const (
	implementationBackendAgent  = "backend_agent"
	implementationFrontendAgent = "frontend_agent"
	implementationGenericAgent  = "generic_agent"
)

var (
	implementationTaskHeaderRE = regexp.MustCompile(`^([ \t]*)([-*+]|[0-9]+[.)])[ \t]+(\[[ xX]\])[ \t]+(.*)$`)
	implementationTaskIDRE     = regexp.MustCompile(`\[task-[^\]\s]+\]`)
	implementationAgentTagRE   = regexp.MustCompile(`\[agent:([^\]\s]+)\]`)
)

// implementationTaskBlock is the smallest unit that can be delegated safely.
// Block retains the task's continuation text so acceptance and verification
// criteria are not lost when a scheduler builds an agent prompt.
type implementationTaskBlock struct {
	ID            string
	IDToken       string
	Owner         string
	Checkbox      string
	Header        string
	Block         string
	Line          int
	EndLine       int
	Pending       bool
	InvalidReason string

	headerStart   int
	checkboxStart int
	checkboxEnd   int
}

// implementationTaskPlan is fail-closed: callers must check Valid before
// using ByAgent. Unassigned tasks are retained for diagnostics and recovery.
type implementationTaskPlan struct {
	Tasks      []implementationTaskBlock
	ByAgent    map[string][]implementationTaskBlock
	Unassigned []implementationTaskBlock
	Invalid    []implementationTaskBlock
	Errors     []string
	Valid      bool
}

type implementationTaskLine struct {
	text   string
	start  int
	line   int
	fenced bool
}

// partitionImplementationTasks assigns pending tasks to the active
// implementation agents. Explicit owner tags are mandatory for a multi-agent
// run; the single-agent fallback preserves legacy unowned tasks.
func partitionImplementationTasks(raw string, activeAgents []string) implementationTaskPlan {
	plan := implementationTaskPlan{ByAgent: make(map[string][]implementationTaskBlock)}
	active, activeErrors := normalizeImplementationAgents(activeAgents)
	plan.Errors = append(plan.Errors, activeErrors...)
	plan.Errors = append(plan.Errors, implementationTaskSyntaxErrors(raw)...)

	allTasks := parseImplementationTaskBlocks(raw)
	allCounts := make(map[string]int, len(allTasks))
	for _, task := range allTasks {
		if task.ID != "" {
			allCounts[task.ID]++
		}
	}
	reportedDuplicates := make(map[string]struct{})
	for _, task := range allTasks {
		if task.ID == "" || allCounts[task.ID] <= 1 {
			continue
		}
		if _, ok := reportedDuplicates[task.ID]; ok {
			continue
		}
		reportedDuplicates[task.ID] = struct{}{}
		plan.Errors = append(plan.Errors, fmt.Sprintf("task ID %q is duplicated", task.ID))
	}
	for _, task := range allTasks {
		if task.Pending {
			plan.Tasks = append(plan.Tasks, task)
		}
	}
	if len(plan.Tasks) == 0 {
		plan.Valid = len(plan.Errors) == 0
		return plan
	}

	for index := range plan.Tasks {
		task := plan.Tasks[index]
		if task.ID == "" {
			task.InvalidReason = "task ID is required"
			plan.Invalid = append(plan.Invalid, task)
			plan.Errors = append(plan.Errors, fmt.Sprintf("line %d has a pending task without a task ID", task.Line))
			continue
		}
		if allCounts[task.ID] > 1 {
			task.InvalidReason = "task ID is duplicated"
			plan.Invalid = append(plan.Invalid, task)
			continue
		}

		owner, ownerErr := implementationTaskOwner(task.Header)
		if ownerErr != "" {
			task.InvalidReason = ownerErr
			plan.Unassigned = append(plan.Unassigned, task)
			plan.Errors = append(plan.Errors, fmt.Sprintf("task %q: %s", task.ID, ownerErr))
			continue
		}
		if owner == "" {
			if len(active) == 1 {
				owner = active[0]
			} else {
				task.InvalidReason = "task has no owner for a multi-agent assignment"
				plan.Unassigned = append(plan.Unassigned, task)
				plan.Errors = append(plan.Errors, fmt.Sprintf("task %q has no owner; explicit agent ownership is required", task.ID))
				continue
			}
		}
		task.Owner = owner
		plan.Tasks[index].Owner = owner
		if !containsImplementationAgent(active, owner) {
			task.InvalidReason = "task owner is not active"
			plan.Unassigned = append(plan.Unassigned, task)
			plan.Errors = append(plan.Errors, fmt.Sprintf("task %q owner %q is not active", task.ID, owner))
			continue
		}
		plan.ByAgent[owner] = append(plan.ByAgent[owner], task)
	}
	plan.Valid = len(plan.Errors) == 0
	return plan
}

func normalizeImplementationAgents(activeAgents []string) ([]string, []string) {
	seen := make(map[string]struct{}, len(activeAgents))
	active := make([]string, 0, len(activeAgents))
	var errs []string
	for _, raw := range activeAgents {
		agent := strings.TrimSpace(raw)
		if !isCanonicalImplementationAgent(agent) {
			errs = append(errs, fmt.Sprintf("unsupported active implementation agent %q", agent))
			continue
		}
		if _, ok := seen[agent]; ok {
			errs = append(errs, fmt.Sprintf("active implementation agent %q is duplicated", agent))
			continue
		}
		seen[agent] = struct{}{}
		active = append(active, agent)
	}
	return active, errs
}

func isCanonicalImplementationAgent(agent string) bool {
	switch agent {
	case implementationBackendAgent, implementationFrontendAgent, implementationGenericAgent:
		return true
	default:
		return false
	}
}

func containsImplementationAgent(agents []string, wanted string) bool {
	for _, agent := range agents {
		if agent == wanted {
			return true
		}
	}
	return false
}

func implementationTaskOwner(block string) (string, string) {
	matches := implementationAgentTagRE.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		return "", ""
	}
	if len(matches) != 1 {
		return "", "multiple agent owner tags are not allowed"
	}
	owner := matches[0][1]
	if !isCanonicalImplementationAgent(owner) {
		return "", fmt.Sprintf("unknown task owner %q", owner)
	}
	return owner, ""
}

func parseImplementationTaskBlocks(raw string) []implementationTaskBlock {
	lines := implementationTaskLines(raw)
	if len(lines) == 0 {
		return nil
	}

	// A nested checklist can be part of a task's acceptance criteria. The
	// shallowest checkbox indentation is therefore the task-list indentation.
	minIndent := -1
	for _, line := range lines {
		if line.fenced {
			continue
		}
		parsed, ok := parseImplementationTaskHeader(line.text)
		if !ok {
			continue
		}
		indent := len(parsed.indent)
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent < 0 {
		return nil
	}

	starts := make([]implementationTaskBlock, 0)
	for _, line := range lines {
		if line.fenced {
			continue
		}
		header, ok := parseImplementationTaskHeader(line.text)
		if !ok || len(header.indent) != minIndent {
			continue
		}
		ids := implementationTaskIDRE.FindAllString(header.rest, -1)
		id := ""
		if len(ids) == 1 {
			id = strings.TrimSuffix(strings.TrimPrefix(ids[0], "["), "]")
		}
		starts = append(starts, implementationTaskBlock{
			ID:            id,
			IDToken:       firstImplementationTaskID(ids),
			Checkbox:      header.checkbox,
			Header:        line.text,
			Pending:       header.checkbox == "[ ]",
			Line:          line.line,
			headerStart:   line.start,
			checkboxStart: line.start + header.checkboxStart,
			checkboxEnd:   line.start + header.checkboxEnd,
		})
	}

	for i := range starts {
		end := len(raw)
		endLine := lines[len(lines)-1].line
		if i+1 < len(starts) {
			end = starts[i+1].headerStart
			endLine = starts[i+1].Line - 1
		}
		// Do not make a task block consume a following top-level heading or
		// prose section, whether or not another checkbox follows it.
		for _, line := range lines {
			if line.fenced || line.start <= starts[i].headerStart || line.start >= end || strings.TrimSpace(line.text) == "" {
				continue
			}
			if isImplementationBlockBoundary(line.text) {
				end = line.start
				endLine = line.line - 1
				break
			}
		}
		starts[i].EndLine = endLine
		starts[i].Block = raw[starts[i].headerStart:end]
	}
	return starts
}

type implementationTaskHeader struct {
	indent        string
	checkbox      string
	checkboxStart int
	checkboxEnd   int
	rest          string
}

func parseImplementationTaskHeader(line string) (implementationTaskHeader, bool) {
	line = strings.TrimSuffix(line, "\r")
	matches := implementationTaskHeaderRE.FindStringSubmatchIndex(line)
	if matches == nil {
		return implementationTaskHeader{}, false
	}
	return implementationTaskHeader{
		indent:        line[matches[2]:matches[3]],
		checkbox:      line[matches[6]:matches[7]],
		checkboxStart: matches[6],
		checkboxEnd:   matches[7],
		rest:          line[matches[8]:matches[9]],
	}, true
}

func implementationTaskLines(raw string) []implementationTaskLine {
	if raw == "" {
		return nil
	}
	lines := make([]implementationTaskLine, 0, strings.Count(raw, "\n")+1)
	start := 0
	lineNumber := 1
	inFence := false
	var fenceChar byte
	fenceLength := 0
	for start < len(raw) {
		end := strings.IndexByte(raw[start:], '\n')
		if end < 0 {
			end = len(raw)
		} else {
			end += start + 1
		}
		textEnd := end
		if textEnd > start && raw[textEnd-1] == '\n' {
			textEnd--
		}
		text := raw[start:textEnd]
		fenced := inFence
		if marker, length, closes, ok := implementationFenceDelimiter(text); ok {
			fenced = true
			if !inFence {
				inFence = true
				fenceChar = marker
				fenceLength = length
			} else if closes && marker == fenceChar && length >= fenceLength {
				inFence = false
				fenceChar = 0
				fenceLength = 0
			}
		}
		lines = append(lines, implementationTaskLine{text: text, start: start, line: lineNumber, fenced: fenced})
		start = end
		lineNumber++
	}
	return lines
}

// implementationFenceDelimiter identifies a Markdown fenced-code delimiter.
// The caller decides whether a delimiter opens or closes a fence: outside a
// fence every valid delimiter opens one, while inside a fence only a delimiter
// with no info string can close it.
func implementationFenceDelimiter(line string) (marker byte, length int, closes bool, ok bool) {
	line = strings.TrimSuffix(line, "\r")
	indent := 0
	for indent < len(line) && indent < 3 && line[indent] == ' ' {
		indent++
	}
	if indent < len(line) && line[indent] == ' ' {
		return 0, 0, false, false
	}
	if indent >= len(line) || (line[indent] != '`' && line[indent] != '~') {
		return 0, 0, false, false
	}
	marker = line[indent]
	end := indent
	for end < len(line) && line[end] == marker {
		end++
	}
	if end-indent < 3 {
		return 0, 0, false, false
	}
	closes = strings.TrimSpace(line[end:]) == ""
	return marker, end - indent, closes, true
}

// implementationTaskSyntaxErrors makes unsupported checklist-looking lines
// fail closed instead of silently producing an empty assignment plan.
func implementationTaskSyntaxErrors(raw string) []string {
	var errs []string
	for _, line := range implementationTaskLines(raw) {
		if line.fenced || !isImplementationTaskCandidate(line.text) {
			continue
		}
		if _, ok := parseImplementationTaskHeader(line.text); ok {
			continue
		}
		errs = append(errs, fmt.Sprintf("line %d has unsupported task checkbox syntax", line.line))
	}
	return errs
}

func isImplementationTaskCandidate(line string) bool {
	line = strings.TrimSuffix(line, "\r")
	position := 0
	for position < len(line) && (line[position] == ' ' || line[position] == '\t') {
		position++
	}
	// A blockquote task is not part of the OpenSpec task syntax. Detect it so
	// the caller can reject it explicitly instead of accepting zero tasks.
	for position < len(line) && line[position] == '>' {
		position++
		for position < len(line) && (line[position] == ' ' || line[position] == '\t') {
			position++
		}
	}
	if position >= len(line) {
		return false
	}
	if line[position] == '-' || line[position] == '*' || line[position] == '+' {
		position++
	} else {
		start := position
		for position < len(line) && line[position] >= '0' && line[position] <= '9' {
			position++
		}
		if position == start || position >= len(line) || (line[position] != '.' && line[position] != ')') {
			return false
		}
		position++
	}
	if position >= len(line) || (line[position] != ' ' && line[position] != '\t') {
		return false
	}
	for position < len(line) && (line[position] == ' ' || line[position] == '\t') {
		position++
	}
	return position < len(line) && line[position] == '['
}

func isImplementationBlockBoundary(line string) bool {
	line = strings.TrimSuffix(line, "\r")
	indent := 0
	for indent < len(line) && line[indent] == ' ' {
		indent++
	}
	if indent > 3 || indent == len(line) {
		return false
	}
	return line[indent] == '#'
}

func firstImplementationTaskID(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// mergeImplementationAssignment unions unchecked owned task-* blocks with
// actionable findings per active agent. Task order follows OpenSpec; findings
// follow stable store order (created_at, id). A finding owner outside the
// active Implementation scope fails the whole plan before Execute.
func mergeImplementationAssignment(plan implementationTaskPlan, findings []store.Finding, activeAgents []string) (map[string][]implementationTaskBlock, []string) {
	byAgent := make(map[string][]implementationTaskBlock, len(activeAgents))
	for _, agent := range activeAgents {
		byAgent[agent] = append([]implementationTaskBlock(nil), plan.ByAgent[agent]...)
	}
	activeSet := make(map[string]struct{}, len(activeAgents))
	for _, agent := range activeAgents {
		activeSet[agent] = struct{}{}
	}
	var errs []string
	for _, finding := range findings {
		owner := strings.TrimSpace(finding.Owner)
		if !isCanonicalImplementationAgent(owner) {
			errs = append(errs, fmt.Sprintf("finding %q has unknown owner %q", finding.ID, owner))
			continue
		}
		if _, ok := activeSet[owner]; !ok {
			slog.Error("implementation assignment rejected finding owner outside active scope",
				"finding_id", finding.ID, "owner", owner, "active_agents", activeAgents)
			errs = append(errs, fmt.Sprintf("finding %q owner %q is not in active implementation scope", finding.ID, owner))
			continue
		}
		byAgent[owner] = append(byAgent[owner], implementationFindingBlock(finding))
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return byAgent, nil
}

func implementationFindingBlock(finding store.Finding) implementationTaskBlock {
	var b strings.Builder
	fmt.Fprintf(&b, "- [ ] %s · finding · %s\n", finding.ID, finding.SourceStage)
	if file := strings.TrimSpace(finding.File); file != "" {
		fmt.Fprintf(&b, "  File: %s\n", file)
	}
	if req := strings.TrimSpace(finding.Requirement); req != "" {
		fmt.Fprintf(&b, "  Requirement: %s\n", req)
	}
	fmt.Fprintf(&b, "  Issue: %s\n", strings.TrimSpace(finding.Issue))
	fmt.Fprintf(&b, "  Acceptance: %s\n", strings.TrimSpace(finding.AcceptanceCriteria))
	return implementationTaskBlock{
		ID:      finding.ID,
		Owner:   finding.Owner,
		Block:   b.String(),
		Pending: true,
	}
}

func implementationAssignmentIDs(items []implementationTaskBlock) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if id := strings.TrimSpace(item.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func implementationOpenSpecTaskIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if strings.HasPrefix(id, "task-") {
			out = append(out, id)
		}
	}
	return out
}

func normalizeImplementationAssignmentID(raw string) (string, error) {
	id, derr := reports.NormalizeAssignmentID(raw)
	if derr != nil {
		return "", fmt.Errorf("%s", derr.Error())
	}
	return id, nil
}

func validateImplementationAssignmentUnion(completed, remaining, assignment []string) error {
	if derr := reports.ValidateAssignmentUnion(completed, remaining, assignment); derr != nil {
		return fmt.Errorf("%s", derr.Error())
	}
	return nil
}

func buildImplementationStageDispatch(checklist implementationChecklist, activeAgents []string, findings []store.Finding) ([]string, map[string][]implementationTaskBlock, []string, string) {
	if !checklist.Linked {
		return nil, nil, nil, "active cycle has no linked OpenSpec tasks.md"
	}
	if !checklist.Ready {
		return nil, nil, nil, "linked OpenSpec tasks.md could not be read"
	}
	plan := partitionImplementationTasks(checklist.Raw, activeAgents)
	if !plan.Valid {
		if len(plan.Errors) == 0 {
			return nil, nil, nil, "implementation task ownership plan is invalid"
		}
		return nil, nil, nil, "implementation task ownership plan is invalid: " + strings.Join(plan.Errors, "; ")
	}
	byAgent, mergeErrors := mergeImplementationAssignment(plan, findings, activeAgents)
	if len(mergeErrors) > 0 {
		return nil, nil, nil, "implementation assignment union is invalid: " + strings.Join(mergeErrors, "; ")
	}
	hasWork := false
	for _, agent := range activeAgents {
		if len(byAgent[agent]) > 0 {
			hasWork = true
			break
		}
	}
	if !hasWork {
		assignments := make(map[string][]implementationTaskBlock, len(activeAgents))
		for _, agent := range activeAgents {
			assignments[agent] = []implementationTaskBlock{}
		}
		slog.Info("implementation verification wave scheduled", "active_agents", activeAgents)
		return append([]string(nil), activeAgents...), assignments, append([]string(nil), activeAgents...), ""
	}
	runAgents := make([]string, 0, len(activeAgents))
	assignments := make(map[string][]implementationTaskBlock, len(activeAgents))
	for _, agent := range activeAgents {
		items := append([]implementationTaskBlock(nil), byAgent[agent]...)
		if len(items) == 0 {
			continue
		}
		assignments[agent] = items
		runAgents = append(runAgents, agent)
	}
	if len(runAgents) == 0 {
		return nil, nil, nil, "implementation assignment has actionable work but no active owner"
	}
	return runAgents, assignments, append([]string(nil), runAgents...), ""
}

// markImplementationTasksComplete updates all requested task checkboxes in a
// single atomic replacement. Validation occurs before creating the temporary
// file, so a missing or ambiguous ID cannot partially mutate tasks.md.
func markImplementationTasksComplete(path string, taskIDs []string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("tasks path is required")
	}
	if len(taskIDs) == 0 {
		return errors.New("at least one task ID is required")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat tasks file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("tasks path must not be a symlink")
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("tasks path is not a regular file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read tasks file: %w", err)
	}
	if syntaxErrors := implementationTaskSyntaxErrors(string(raw)); len(syntaxErrors) > 0 {
		return fmt.Errorf("tasks file contains unsupported checklist syntax: %s", strings.Join(syntaxErrors, "; "))
	}

	wanted, err := normalizeImplementationTaskIDs(taskIDs)
	if err != nil {
		return err
	}
	tasks := parseImplementationTaskBlocks(string(raw))
	byID := make(map[string][]implementationTaskBlock)
	for _, task := range tasks {
		if task.ID != "" {
			byID[task.ID] = append(byID[task.ID], task)
		}
	}
	for _, id := range wanted {
		matches := byID[id]
		if len(matches) == 0 {
			return fmt.Errorf("task ID %q does not correspond to a parseable task", id)
		}
		if len(matches) > 1 {
			return fmt.Errorf("task ID %q is duplicated", id)
		}
		if !matches[0].Pending {
			return fmt.Errorf("task ID %q is already complete", id)
		}
	}

	updated := append([]byte(nil), raw...)
	positions := make([]implementationTaskBlock, 0, len(wanted))
	for _, id := range wanted {
		positions = append(positions, byID[id][0])
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i].checkboxStart < positions[j].checkboxStart })
	for i := len(positions) - 1; i >= 0; i-- {
		position := positions[i]
		copy(updated[position.checkboxStart:position.checkboxEnd], "[x]")
	}
	if bytes.Equal(updated, raw) {
		return nil
	}
	if err := writeImplementationTasksAtomically(path, updated, info.Mode(), raw); err != nil {
		return fmt.Errorf("write tasks file: %w", err)
	}
	return nil
}

func normalizeImplementationTaskIDs(taskIDs []string) ([]string, error) {
	wanted := make([]string, 0, len(taskIDs))
	seen := make(map[string]struct{}, len(taskIDs))
	for _, raw := range taskIDs {
		id := strings.TrimSpace(raw)
		if strings.HasPrefix(id, "[") && strings.HasSuffix(id, "]") {
			id = strings.TrimSuffix(strings.TrimPrefix(id, "["), "]")
		}
		if !strings.HasPrefix(id, "task-") || strings.TrimSpace(strings.TrimPrefix(id, "task-")) == "" || strings.ContainsAny(id, "[] \t\r\n") {
			return nil, fmt.Errorf("invalid task ID %q", raw)
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("requested task ID %q is duplicated", id)
		}
		seen[id] = struct{}{}
		wanted = append(wanted, id)
	}
	return wanted, nil
}

func writeImplementationTasksAtomically(path string, content []byte, mode os.FileMode, expected ...[]byte) error {
	if len(expected) > 1 {
		return errors.New("at most one expected tasks snapshot is allowed")
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".hero-*")
	if err != nil {
		return fmt.Errorf("create temporary tasks file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode.Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("preserve tasks file mode: %w", err)
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary tasks file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary tasks file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary tasks file: %w", err)
	}
	if len(expected) == 1 {
		if err := verifyImplementationTasksSnapshot(path, expected[0], mode); err != nil {
			return err
		}
	} else if err := verifyImplementationTasksTarget(path, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace tasks file: %w", err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open tasks directory for sync: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return fmt.Errorf("sync tasks directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close tasks directory: %w", err)
	}
	return nil
}

func verifyImplementationTasksSnapshot(path string, expected []byte, mode os.FileMode) error {
	if err := verifyImplementationTasksTarget(path, mode); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("verify tasks file before replace: %w", err)
	}
	if !bytes.Equal(current, expected) {
		return errors.New("tasks file changed concurrently")
	}
	return nil
}

func verifyImplementationTasksTarget(path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("verify tasks file before replace: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("tasks path became a symlink during update")
	}
	if !info.Mode().IsRegular() {
		return errors.New("tasks path is no longer a regular file")
	}
	if info.Mode().Perm() != mode.Perm() {
		return errors.New("tasks file changed concurrently")
	}
	return nil
}
