package cycle

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

var archiveDirPattern = regexp.MustCompile(`^C(\d+)-(\d{4}-\d{2}-\d{2})-(.+)$`)

// CyclesView is the aggregated hero-cycles listing.
type CyclesView struct {
	Total  int          `json:"total"`
	Cycles []CycleEntry `json:"cycles"`
}

// CycleEntry is one cycle with per-etapa metrics.
type CycleEntry struct {
	Number       int              `json:"number"`
	Title        string           `json:"title"`
	Status       string           `json:"status"`
	ArchivedDate string           `json:"archivedDate,omitempty"`
	Source       string           `json:"source"` // sqlite | archive
	Stages       []CycleStageRow  `json:"stages"`
	TotalIn      int64            `json:"totalInputTokens"`
	TotalOut     int64            `json:"totalOutputTokens"`
	TotalCost    float64          `json:"totalCostUSD"`
	TotalTokens  int64            `json:"totalTokens"`
}

// CycleStageRow is one etapa row in the cycles listing.
type CycleStageRow struct {
	Name         string  `json:"name"`
	Status       string  `json:"status,omitempty"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CostUSD      float64 `json:"costUSD"`
	DurationMS   int64   `json:"durationMS"`
}

type stageMetricsAgg struct {
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
	DurationMS   int64
}

// Cycles aggregates cycles and metrics from SQLite and archive folders.
func (s *Service) Cycles() (CyclesView, error) {
	if s == nil || s.Store == nil {
		return CyclesView{}, fmt.Errorf("cycle service not initialized")
	}

	dbCycles, err := s.Store.ListCycles()
	if err != nil {
		return CyclesView{}, err
	}

	known := make(map[int]struct{}, len(dbCycles))
	entries := make([]CycleEntry, 0, len(dbCycles))

	for _, c := range dbCycles {
		known[c.Number] = struct{}{}
		entry, err := s.cycleEntryFromSQLite(c)
		if err != nil {
			return CyclesView{}, err
		}
		entries = append(entries, entry)
	}

	archiveEntries, err := s.archiveOnlyCycles(known)
	if err != nil {
		return CyclesView{}, err
	}
	entries = append(entries, archiveEntries...)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Number < entries[j].Number
	})

	slog.Debug("aggregated cycles listing", "sqlite", len(dbCycles), "archive_only", len(archiveEntries), "total", len(entries))
	return CyclesView{Total: len(entries), Cycles: entries}, nil
}

func (s *Service) cycleEntryFromSQLite(c store.Cycle) (CycleEntry, error) {
	stages, err := s.Store.ListStages(c.ID)
	if err != nil {
		return CycleEntry{}, err
	}
	metrics, err := s.Store.ListMetrics(c.ID)
	if err != nil {
		return CycleEntry{}, err
	}

	byStage := aggregateMetrics(metrics)
	rows := make([]CycleStageRow, 0, len(stages))
	for _, st := range stages {
		agg := byStage[st.Name]
		rows = append(rows, CycleStageRow{
			Name:         displayStageName(st.Name),
			Status:       formatStageStatus(st.Status),
			InputTokens:  agg.InputTokens,
			OutputTokens: agg.OutputTokens,
			CostUSD:      agg.CostUSD,
			DurationMS:   agg.DurationMS,
		})
		delete(byStage, st.Name)
	}
	for name, agg := range byStage {
		rows = append(rows, CycleStageRow{
			Name:         displayStageName(name),
			InputTokens:  agg.InputTokens,
			OutputTokens: agg.OutputTokens,
			CostUSD:      agg.CostUSD,
			DurationMS:   agg.DurationMS,
		})
	}

	entry := CycleEntry{
		Number:       c.Number,
		Title:        c.Title,
		Status:       c.Status,
		ArchivedDate: archivedDateFromCompleted(c.CompletedAt),
		Source:       "sqlite",
		Stages:       rows,
	}
	finalizeCycleTotals(&entry)
	return entry, nil
}

func (s *Service) archiveOnlyCycles(known map[int]struct{}) ([]CycleEntry, error) {
	archiveRoot := filepath.Join(s.ProjectDir, cursoradapter.HeroCyclesDir, "archive")
	ents, err := os.ReadDir(archiveRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read archive dir: %w", err)
	}

	var out []CycleEntry
	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		number, date, ok := parseArchiveDirName(ent.Name())
		if !ok {
			slog.Debug("skipping unrecognized archive folder", "name", ent.Name())
			continue
		}
		if _, exists := known[number]; exists {
			continue
		}

		dir := filepath.Join(archiveRoot, ent.Name())
		entry, ok, err := s.cycleEntryFromArchiveDir(dir, number, date)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, entry)
		}
	}
	return out, nil
}

func (s *Service) cycleEntryFromArchiveDir(dir string, number int, archivedDate string) (CycleEntry, bool, error) {
	metricsPath := filepath.Join(dir, "metrics.md")
	data, err := os.ReadFile(metricsPath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("archive folder has no metrics.md", "dir", dir)
			return CycleEntry{}, false, nil
		}
		return CycleEntry{}, false, fmt.Errorf("read %s: %w", metricsPath, err)
	}

	content := string(data)
	meta := store.ParseLegacyMetricsMeta(content)
	title := meta.Title
	if title == "" {
		title = titleFromArchiveSlug(filepath.Base(dir))
	}
	if number <= 0 && meta.Number > 0 {
		number = meta.Number
	}

	rows := make([]CycleStageRow, 0)
	for _, m := range store.ParseLegacyMetrics(content) {
		rows = append(rows, CycleStageRow{
			Name:         strings.TrimSpace(m.Stage),
			InputTokens:  m.InputTokens,
			OutputTokens: m.OutputTokens,
			CostUSD:      m.CostUSD,
			DurationMS:   m.DurationMS,
		})
	}

	entry := CycleEntry{
		Number:       number,
		Title:        title,
		Status:       store.CycleStatusArchived,
		ArchivedDate: archivedDate,
		Source:       "archive",
		Stages:       rows,
	}
	finalizeCycleTotals(&entry)
	slog.Info("included archive-only cycle", "number", number, "dir", dir)
	return entry, true, nil
}

func aggregateMetrics(metrics []store.Metric) map[string]stageMetricsAgg {
	out := make(map[string]stageMetricsAgg)
	for _, m := range metrics {
		agg := out[m.StageName]
		agg.InputTokens += m.InputTokens
		agg.OutputTokens += m.OutputTokens
		agg.CostUSD += m.CostUSD
		agg.DurationMS += m.DurationMS
		out[m.StageName] = agg
	}
	return out
}

func finalizeCycleTotals(entry *CycleEntry) {
	for _, row := range entry.Stages {
		entry.TotalIn += row.InputTokens
		entry.TotalOut += row.OutputTokens
		entry.TotalCost += row.CostUSD
	}
	entry.TotalTokens = entry.TotalIn + entry.TotalOut
}

func parseArchiveDirName(name string) (number int, date string, ok bool) {
	m := archiveDirPattern.FindStringSubmatch(name)
	if len(m) != 4 {
		return 0, "", false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0, "", false
	}
	return n, m[2], true
}

func archivedDateFromCompleted(completedAt string) string {
	if completedAt == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, completedAt); err == nil {
		return t.Format("2006-01-02")
	}
	if len(completedAt) >= 10 {
		return completedAt[:10]
	}
	return ""
}

func titleFromArchiveSlug(folder string) string {
	m := archiveDirPattern.FindStringSubmatch(folder)
	if len(m) != 4 {
		return folder
	}
	slug := strings.ReplaceAll(m[3], "-", " ")
	return strings.TrimSpace(slug)
}

func formatStageStatus(status string) string {
	switch status {
	case store.StageRunning:
		return "running"
	case store.StageWaiting:
		return "waiting"
	case store.StagePendingApproval:
		return "pending approval"
	case store.StageCompleted:
		return "completed"
	case store.StageEscalated:
		return "escalated"
	case store.StageFailed:
		return "failed"
	case store.StageSkipped:
		return "skipped"
	default:
		return strings.ToLower(status)
	}
}

func formatCycleStatusBadge(entry CycleEntry) string {
	switch entry.Status {
	case store.CycleStatusActive:
		return "active"
	case store.CycleStatusArchived:
		if entry.ArchivedDate != "" {
			return fmt.Sprintf("archived %s", entry.ArchivedDate)
		}
		return "archived"
	case store.CycleStatusCompleted:
		return "completed"
	case store.CycleStatusCancelled:
		return "cancelled"
	default:
		if entry.Status == "" {
			return "unknown"
		}
		return entry.Status
	}
}

// FormatCycles writes the hero-cycles listing per UI-C03-001 §5.
func FormatCycles(w io.Writer, view CyclesView) {
	output.Progressf(w, "Cycles (%d total)", view.Total)
	fmt.Fprintln(w)

	for _, c := range view.Cycles {
		fmt.Fprintf(w, "C%d — %s [%s]\n", c.Number, c.Title, formatCycleStatusBadge(c))
		for _, row := range c.Stages {
			fmt.Fprint(w, formatCycleStageLine(row))
		}
		if len(c.Stages) > 0 {
			fmt.Fprintf(w, "  Total: %s tokens  %s\n", formatTokenCount(c.TotalTokens), formatCostUSD(c.TotalCost))
		}
		fmt.Fprintln(w)
	}
}

func formatCycleStageLine(row CycleStageRow) string {
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(padRight(row.Name, 13))
	if row.Status != "" {
		b.WriteString(padRight(row.Status, 10))
	} else if row.InputTokens > 0 || row.OutputTokens > 0 || row.CostUSD > 0 || row.DurationMS > 0 {
		b.WriteString(padRight("", 10))
	}
	if row.InputTokens > 0 || row.OutputTokens > 0 {
		b.WriteString(fmt.Sprintf("in: %s  out: %s  ", formatTokenCount(row.InputTokens), formatTokenCount(row.OutputTokens)))
	}
	if row.CostUSD > 0 {
		b.WriteString(formatCostUSD(row.CostUSD))
		b.WriteString("  ")
	}
	if dur := formatDuration(row.DurationMS); dur != "" {
		b.WriteString(dur)
	}
	b.WriteString("\n")
	return b.String()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s + " "
	}
	return s + strings.Repeat(" ", width-len(s))
}

func formatTokenCount(n int64) string {
	if n >= 1000 {
		if n%1000 == 0 {
			return fmt.Sprintf("%dk", n/1000)
		}
		whole := n / 1000
		frac := (n % 1000) / 100
		if frac == 0 {
			return fmt.Sprintf("%dk", whole)
		}
		return fmt.Sprintf("%d.%dk", whole, frac)
	}
	return fmt.Sprintf("%d", n)
}

func formatCostUSD(cost float64) string {
	if cost == 0 {
		return "~$0.00"
	}
	if cost < 0.01 {
		return fmt.Sprintf("~$%.4f", cost)
	}
	return fmt.Sprintf("~$%.2f", cost)
}

func formatDuration(ms int64) string {
	if ms <= 0 {
		return ""
	}
	if ms >= 60_000 {
		minutes := ms / 60_000
		if ms%60_000 >= 30_000 {
			minutes++
		}
		return fmt.Sprintf("%dm", minutes)
	}
	if ms >= 1000 {
		seconds := ms / 1000
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dms", ms)
}
