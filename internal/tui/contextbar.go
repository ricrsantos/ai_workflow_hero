package tui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/assets"
	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
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

var contextEffortSuffixes = []string{"-high", "-fast", "-medium", "-low", "-max"}

type contextWindowCatalog map[string]int64

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
	for _, suffix := range contextEffortSuffixes {
		if strings.HasSuffix(slug, suffix) {
			base := strings.TrimSuffix(slug, suffix)
			if max, ok := c[base]; ok && max > 0 {
				return max
			}
			return 0
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

func (m model) renderScrollHintLine() string {
	leftText := "↑↓ scroll response"
	if m.streaming {
		leftText = "↑↓ scroll · ctrl+c interrupt"
	}
	left := mutedStyle.Render(leftText)
	right := renderContextBar(m.contextUsedTokens, m.contextWindowMax())
	w := m.width
	if w < 20 {
		w = 20
	}
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	if right == "" {
		return left
	}
	gap := w - leftW - rightW
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}
