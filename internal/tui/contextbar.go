package tui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"gopkg.in/yaml.v3"
)

const (
	contextBarCells      = 12
	contextBarWarnPct    = 80
	contextBarDangerPct  = 95
	contextBarFillChar   = "█"
	contextBarEmptyChar  = "░"
	contextLabelMinWidth = 11
)

var contextEffortSuffixes = []string{"-thinking", "-high", "-fast", "-medium", "-low", "-max"}

type contextWindowCatalog map[string]int64

var ansiStatusEscapeRE = regexp.MustCompile(`\x1b\[[0-9;]*[[:alpha:]]`)

type modelPricingFile struct {
	Models map[string]modelPricingEntry `yaml:"models"`
}

type modelPricingEntry struct {
	ContextWindow int64 `yaml:"context_window"`
}

func loadContextWindowCatalog(projectDir string) contextWindowCatalog {
	cat := make(contextWindowCatalog)
	loadContextWindowsFromFS(assets.FS, "models", cat)
	if strings.TrimSpace(projectDir) != "" {
		loadContextWindowsFromDir(filepath.Join(projectDir, cursoradapter.HeroModelsDir), cat)
	}
	return cat
}

func loadContextWindowsFromDir(dir string, cat contextWindowCatalog) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		mergeContextWindows(cat, data)
	}
}

func loadContextWindowsFromFS(fsys fs.FS, dir string, cat contextWindowCatalog) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".yml") {
			continue
		}
		data, err := fs.ReadFile(fsys, dir+"/"+e.Name())
		if err != nil {
			continue
		}
		mergeContextWindows(cat, data)
	}
}

func mergeContextWindows(cat contextWindowCatalog, data []byte) {
	var file modelPricingFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return
	}
	for slug, entry := range file.Models {
		slug = strings.TrimSpace(slug)
		if slug == "" || entry.ContextWindow <= 0 {
			continue
		}
		cat[slug] = entry.ContextWindow
	}
}

func (c contextWindowCatalog) lookup(slug string) int64 {
	slug = strings.TrimSpace(slug)
	if slug == "" || c == nil {
		return 0
	}
	if max, ok := c[slug]; ok && max > 0 {
		return max
	}
	base := slug
	for range contextEffortSuffixes {
		removed := false
		for _, suffix := range contextEffortSuffixes {
			if !strings.HasSuffix(base, suffix) {
				continue
			}
			base = strings.TrimSuffix(base, suffix)
			if max, ok := c[base]; ok && max > 0 {
				return max
			}
			removed = true
			break
		}
		if !removed {
			break
		}
	}
	return 0
}

func formatTokenCount(n int64) string {
	if n < 0 {
		n = 0
	}
	if n >= 1_000_000 {
		if n%1_000_000 == 0 {
			return fmt.Sprintf("%d.0M", n/1_000_000)
		}
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1000 {
		k := (n + 500) / 1000
		return fmt.Sprintf("%dk", k)
	}
	return fmt.Sprintf("%d", n)
}

func renderContextBar(used, max int64) string {
	if max <= 0 {
		return ""
	}
	if used < 0 {
		used = 0
	}
	filled := int((used*int64(contextBarCells) + max/2) / max)
	if used > 0 && filled == 0 {
		filled = 1
	}
	if filled > contextBarCells {
		filled = contextBarCells
	}
	empty := contextBarCells - filled

	pct := (used * 100) / max
	fillStyle := successStyle
	switch {
	case pct >= contextBarDangerPct:
		fillStyle = errorStyle
	case pct >= contextBarWarnPct:
		fillStyle = warnStyle
	}

	bar := fillStyle.Render(strings.Repeat(contextBarFillChar, filled)) +
		mutedStyle.Render(strings.Repeat(contextBarEmptyChar, empty))
	label := formatTokenCount(used) + "/" + formatTokenCount(max)
	if w := lipgloss.Width(label); w < contextLabelMinWidth {
		label += strings.Repeat(" ", contextLabelMinWidth-w)
	}
	return bar + "  " + mutedStyle.Render(label)
}

func (m model) conversationContextSlug() string {
	if n := len(m.liveAgents); n > 0 {
		if slug := strings.TrimSpace(m.liveAgents[n-1].Model); slug != "" {
			return slug
		}
	}
	turn := m.latestAgentTurn()
	for i := len(turn) - 1; i >= 0; i-- {
		if slug := strings.TrimSpace(turn[i].modelSlug); slug != "" {
			return slug
		}
	}
	return m.conversationModelSlug()
}

func (m model) contextWindowMax() int64 {
	return m.contextWindows.lookup(m.conversationContextSlug())
}

// renderScrollHintLine renders the status row below the green response pane:
// scroll hint on the left, C5 property labels in the middle, context bar on the
// right (UI-C05-001 §4). Narrow terminals wrap at rune-safe boundaries instead
// of hiding any component.
func (m model) renderScrollHintLine() string {
	return strings.Join(m.renderChatStatusLines(m.width), "\n")
}

// renderChatStatusLines returns the status row as one or more rune-safe lines.
func (m model) renderChatStatusLines(width int) []string {
	if width < 1 {
		width = 1
	}
	leftText := "↑↓ scroll response"
	if m.streaming {
		leftText = "↑↓ scroll · ctrl+c interrupt"
	}
	segments := []statusSegment{
		{text: leftText, style: mutedStyle},
	}
	if labels := m.renderPropertyStatusSegments(); len(labels) > 0 {
		segments = append(segments, labels...)
	}
	if bar := renderContextBar(m.contextUsedTokens, m.contextWindowMax()); bar != "" {
		segments = append(segments, statusSegment{text: bar, plain: true})
	}
	return packStatusSegments(segments, width)
}

// renderPropertyLabels renders the stable [fs-…] [th-…] [ef-…] labels with the
// C5 color semantics: green for validated configured values (fs only when
// validated "true"), gray for na/unavailable/unvalidated (UI-C05-001 §4).
func (m model) renderPropertyLabels() string {
	segments := m.renderPropertyStatusSegments()
	labels := make([]string, 0, len(segments))
	for _, segment := range segments {
		labels = append(labels, segment.text)
	}
	return strings.Join(labels, " ")
}

// renderPropertyStatusSegments returns one independently styled segment per
// property. Keeping labels separate lets the responsive packer wrap between
// labels rather than slicing ANSI escape sequences in a colored label group.
func (m model) renderPropertyStatusSegments() []statusSegment {
	if !m.hasPropertyStatusModel() {
		return nil
	}
	values, validated := m.effectiveDisplayProperties()
	segments := make([]statusSegment, 0, len(harness.PropertyKeys()))
	for _, key := range harness.PropertyKeys() {
		value := values[key]
		if value == "" {
			value = "na"
		}
		label := "[" + key + "-" + value + "]"
		style := mutedStyle
		if validated[key] && value != "na" {
			if key != harness.PropertyFast || value == "true" {
				style = successStyle
			}
		}
		segments = append(segments, statusSegment{text: style.Render(label), plain: true})
	}
	return segments
}

func (m model) hasPropertyStatusModel() bool {
	return strings.TrimSpace(m.chatModelSlug) != "" ||
		strings.TrimSpace(m.runtimeModelSlug) != "" ||
		strings.TrimSpace(m.freechatSnapshot.ModelID) != "" ||
		len(m.freechatProps) > 0 || m.workflowAgentActive()
}

// statusSegment is one independently styled, rune-safe status-line segment.
type statusSegment struct {
	text  string
	style lipgloss.Style
	plain bool // true when no style is applied
}

// packStatusSegments lays segments out left-to-right, wrapping at rune-safe
// boundaries when the line is too narrow. Components are never dropped.
func packStatusSegments(segments []statusSegment, width int) []string {
	rendered := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg.plain {
			rendered = append(rendered, seg.text)
			continue
		}
		rendered = append(rendered, seg.style.Render(seg.text))
	}
	if len(rendered) == 0 {
		return []string{""}
	}

	var lines []string
	var current []string
	currentWidth := 0
	flush := func() {
		if len(current) > 0 {
			lines = append(lines, strings.Join(current, "  "))
			current = nil
			currentWidth = 0
		}
	}
	for _, seg := range rendered {
		w := lipgloss.Width(seg)
		gap := 2
		if len(current) == 0 {
			gap = 0
		}
		if len(current) > 0 && currentWidth+gap+w > width {
			flush()
			gap = 0
		}
		if w > width {
			// A single oversized segment wraps hard at rune-safe boundaries.
			flush()
			// lipgloss styles can add SGR bytes. Strip only for the hard-wrap
			// fallback so the rune index never counts escape bytes as content;
			// normal-width segments retain their original styling.
			for _, line := range wrapOutputLine(stripStatusANSI(seg), width) {
				lines = append(lines, line)
			}
			continue
		}
		current = append(current, seg)
		currentWidth += gap + w
	}
	flush()
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	return lines
}

func stripStatusANSI(s string) string {
	return ansiStatusEscapeRE.ReplaceAllString(s, "")
}
