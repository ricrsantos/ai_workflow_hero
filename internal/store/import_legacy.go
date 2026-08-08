package store

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// LegacyImportResult summarizes a one-shot markdown → store import.
type LegacyImportResult struct {
	Imported   bool
	CycleID    int64
	CycleNumber int
	Stages     int
	Metrics    int
}

// ImportLegacyCycle reads workflow.md (and optionally metrics.md) under
// cycleDir and populates the store. Used by hero upgrade from 0.9.x.
// If workflow.md is missing, returns Imported=false with no error.
func (s *Store) ImportLegacyCycle(cycleDir string, cycleNumber int, configSnapshotJSON string) (LegacyImportResult, error) {
	workflowPath := filepath.Join(cycleDir, "workflow.md")
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		if os.IsNotExist(err) {
			return LegacyImportResult{}, nil
		}
		return LegacyImportResult{}, fmt.Errorf("read workflow.md: %w", err)
	}

	parsed, err := parseLegacyWorkflow(string(data))
	if err != nil {
		return LegacyImportResult{}, err
	}
	if cycleNumber <= 0 {
		cycleNumber = parsed.Number
	}
	if cycleNumber <= 0 {
		cycleNumber = 1
	}
	if configSnapshotJSON == "" {
		configSnapshotJSON = "{}"
	}

	status := CycleStatusActive
	switch strings.ToLower(parsed.Status) {
	case "completed", "complete", "finished":
		status = CycleStatusCompleted
	case "cancelled", "canceled":
		status = CycleStatusCancelled
	}

	started := parsed.Started
	if started == "" {
		started = nowRFC3339()
	} else if _, err := time.Parse("2006-01-02", started); err == nil {
		started = started + "T00:00:00Z"
	}

	completed := parsed.Completed
	if completed != "" {
		if _, err := time.Parse("2006-01-02", completed); err == nil {
			completed = completed + "T00:00:00Z"
		}
	}

	id, err := s.CreateCycle(Cycle{
		Number:             cycleNumber,
		Title:              parsed.Title,
		Objective:          parsed.Objective,
		Status:             status,
		StartedAt:          started,
		CompletedAt:        completed,
		ConfigSnapshotJSON: configSnapshotJSON,
	})
	if err != nil {
		return LegacyImportResult{}, err
	}

	stages := make([]Stage, 0, len(parsed.Stages))
	for i, ls := range parsed.Stages {
		stages = append(stages, Stage{
			CycleID:              id,
			Name:                 normalizeStageName(ls.Name),
			Status:               mapLegacyStageStatus(ls.Status),
			Iteration:            ls.Iteration,
			MaxIterations:        ls.MaxIterations,
			ExtraIterations:      ls.ExtraIterations,
			RequireHumanApproval: ls.RequireApproval,
			SortOrder:            i,
		})
	}
	if err := s.CreateStages(stages); err != nil {
		return LegacyImportResult{}, err
	}

	metricsCount := 0
	metricsPath := filepath.Join(cycleDir, "metrics.md")
	if mdata, err := os.ReadFile(metricsPath); err == nil {
		for _, lm := range parseLegacyMetrics(string(mdata)) {
			if err := s.UpsertMetric(Metric{
				CycleID:      id,
				StageName:    normalizeStageName(lm.Stage),
				Model:        lm.Model,
				Agent:        lm.Agent,
				InputTokens:  lm.InputTokens,
				OutputTokens: lm.OutputTokens,
				CostUSD:      lm.CostUSD,
				DurationMS:   lm.DurationMS,
			}); err != nil {
				return LegacyImportResult{}, err
			}
			metricsCount++
		}
	}

	if _, err := s.AppendEvent(Event{
		CycleID:     id,
		Type:        EventLegacyImported,
		PayloadJSON: `{"source":"workflow.md"}`,
	}); err != nil {
		return LegacyImportResult{}, err
	}

	return LegacyImportResult{
		Imported:    true,
		CycleID:     id,
		CycleNumber: cycleNumber,
		Stages:      len(stages),
		Metrics:     metricsCount,
	}, nil
}

type legacyWorkflow struct {
	Number    int
	Title     string
	Objective string
	Status    string
	Started   string
	Completed string
	Stages    []legacyStage
}

type legacyStage struct {
	Name            string
	Status          string
	Iteration       int
	MaxIterations   int
	ExtraIterations int
	RequireApproval bool
}

type legacyMetric struct {
	Stage        string
	Agent        string
	Model        string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	DurationMS   int64
}

// LegacyMetricRow is one parsed metrics.md table row.
type LegacyMetricRow struct {
	Stage        string
	Agent        string
	Model        string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	DurationMS   int64
}

// LegacyMetricsMeta holds header fields parsed from metrics.md.
type LegacyMetricsMeta struct {
	Number int
	Title  string
}

// ParseLegacyMetrics parses stage metric rows from metrics.md content.
func ParseLegacyMetrics(content string) []LegacyMetricRow {
	raw := parseLegacyMetrics(content)
	out := make([]LegacyMetricRow, len(raw))
	for i, m := range raw {
		out[i] = LegacyMetricRow{
			Stage:        m.Stage,
			Agent:        m.Agent,
			Model:        m.Model,
			InputTokens:  m.InputTokens,
			OutputTokens: m.OutputTokens,
			CostUSD:      m.CostUSD,
			DurationMS:   m.DurationMS,
		}
	}
	return out
}

// ParseLegacyMetricsMeta extracts cycle number and title from metrics.md content.
func ParseLegacyMetricsMeta(content string) LegacyMetricsMeta {
	var meta LegacyMetricsMeta
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(trimmed, "# Metrics") {
			if idx := strings.LastIndex(trimmed, "C"); idx >= 0 {
				numStr := strings.TrimSpace(trimmed[idx+1:])
				if n, err := strconv.Atoi(numStr); err == nil {
					meta.Number = n
				}
			}
			continue
		}
		if v, ok := fieldValue(trimmed, "**Title**"); ok {
			meta.Title = v
		}
	}
	return meta
}

func parseLegacyWorkflow(content string) (legacyWorkflow, error) {
	var w legacyWorkflow
	scanner := bufio.NewScanner(strings.NewReader(content))

	inTable := false
	headerPassed := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# Workflow") {
			// "# Workflow — Cycle C1" or "Cycle C12"
			if idx := strings.LastIndex(trimmed, "C"); idx >= 0 {
				numStr := strings.TrimSpace(trimmed[idx+1:])
				if n, err := strconv.Atoi(numStr); err == nil {
					w.Number = n
				}
			}
			continue
		}
		if v, ok := fieldValue(trimmed, "**Title**"); ok {
			w.Title = v
			continue
		}
		if v, ok := fieldValue(trimmed, "**Objective**"); ok {
			w.Objective = v
			continue
		}
		if v, ok := fieldValue(trimmed, "**Status**"); ok {
			w.Status = v
			continue
		}
		if v, ok := fieldValue(trimmed, "**Started**"); ok {
			w.Started = v
			continue
		}
		if v, ok := fieldValue(trimmed, "**Completed**"); ok {
			w.Completed = v
			continue
		}

		if !inTable {
			if strings.HasPrefix(trimmed, "| Stage") {
				inTable = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "|---") || strings.HasPrefix(trimmed, "| ---") || strings.HasPrefix(trimmed, "|----") {
			headerPassed = true
			continue
		}
		if !headerPassed {
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		parts := splitTableRow(trimmed)
		if len(parts) < 4 {
			continue
		}
		iter, maxIter := parseIteration(parts[2])
		extra := 0
		if len(parts) >= 5 {
			extra = parseExtra(parts[4])
		}
		approval := strings.Contains(strings.ToLower(parts[3]), "required") ||
			strings.EqualFold(parts[3], "pending")
		w.Stages = append(w.Stages, legacyStage{
			Name:            parts[0],
			Status:          parts[1],
			Iteration:       iter,
			MaxIterations:   maxIter,
			ExtraIterations: extra,
			RequireApproval: approval,
		})
	}
	return w, nil
}

func parseLegacyMetrics(content string) []legacyMetric {
	var out []legacyMetric
	scanner := bufio.NewScanner(strings.NewReader(content))
	inTable := false
	headerPassed := false

	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if !inTable {
			if strings.HasPrefix(trimmed, "| Stage") {
				inTable = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "|---") || strings.HasPrefix(trimmed, "| ---") {
			headerPassed = true
			continue
		}
		if !headerPassed {
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		parts := splitTableRow(trimmed)
		if len(parts) < 6 {
			continue
		}
		if strings.Contains(parts[0], "Subtotal") || parts[0] == "" {
			continue
		}
		if isDash(parts[3]) && isDash(parts[4]) && isDash(parts[5]) {
			continue
		}
		out = append(out, legacyMetric{
			Stage:        parts[0],
			Agent:        dashToEmpty(parts[1]),
			Model:        dashToEmpty(parts[2]),
			InputTokens:  parseInt64Loose(parts[3]),
			OutputTokens: parseInt64Loose(parts[4]),
			CostUSD:      parseFloatLoose(parts[5]),
			DurationMS:   parseDurationMS(parts, 6),
		})
	}
	return out
}

func fieldValue(line, key string) (string, bool) {
	if !strings.HasPrefix(line, key) {
		return "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, key))
	rest = strings.TrimPrefix(rest, ":")
	return strings.TrimSpace(rest), true
}

func splitTableRow(line string) []string {
	parts := strings.Split(line, "|")
	var out []string
	for i, p := range parts {
		if i == 0 || i == len(parts)-1 {
			continue
		}
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func parseIteration(s string) (cur, max int) {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" || s == "-" {
		return 0, 1
	}
	parts := strings.Split(s, "/")
	if len(parts) == 1 {
		n, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		return n, n
	}
	cur, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	max, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	if max <= 0 {
		max = 1
	}
	return cur, max
}

func parseExtra(s string) int {
	s = strings.TrimSpace(strings.TrimPrefix(s, "+"))
	n, _ := strconv.Atoi(s)
	return n
}

func mapLegacyStageStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "waiting":
		return StageWaiting
	case "in progress", "running":
		return StageRunning
	case "pending approval", "pendingapproval", "pending":
		return StagePendingApproval
	case "completed", "complete", "done":
		return StageCompleted
	case "escalated":
		return StageEscalated
	case "failed":
		return StageFailed
	case "skipped", "skip":
		return StageSkipped
	default:
		return s
	}
}

// normalizeStageName maps display names to config keys.
func normalizeStageName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, " ", "_")
	n = strings.ReplaceAll(n, "-", "_")
	switch n {
	case "browser_ui_validation", "browser_ui", "browser_ui_validations":
		return "browser_ui_validation"
	case "qa_end_to_end", "qa_e2e", "end_to_end":
		return "qa_end_to_end"
	default:
		return n
	}
}

func isDash(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || s == "—" || s == "-" || s == "~$—" || strings.HasPrefix(s, "~$—")
}

func dashToEmpty(s string) string {
	if isDash(s) {
		return ""
	}
	return strings.TrimSpace(s)
}

func parseInt64Loose(s string) int64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if isDash(s) {
		return 0
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func parseFloatLoose(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "~")
	s = strings.TrimPrefix(s, "$")
	if isDash(s) {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseDurationMS(parts []string, idx int) int64 {
	if idx >= len(parts) {
		return 0
	}
	s := strings.TrimSpace(parts[idx])
	if isDash(s) {
		return 0
	}
	// Accept raw milliseconds or "12s" / "1m".
	if strings.HasSuffix(s, "ms") {
		n, _ := strconv.ParseInt(strings.TrimSuffix(s, "ms"), 10, 64)
		return n
	}
	if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ms") {
		n, _ := strconv.ParseFloat(strings.TrimSuffix(s, "s"), 64)
		return int64(n * 1000)
	}
	if strings.HasSuffix(s, "m") {
		n, _ := strconv.ParseFloat(strings.TrimSuffix(s, "m"), 64)
		return int64(n * 60_000)
	}
	return parseInt64Loose(s)
}
