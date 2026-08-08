package todos

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// CurrentStateRelPath is the project-relative path to current-state.md.
	CurrentStateRelPath = "context/current-state.md"

	// HeaderText is the /hero-todos heading without a leading icon (UI-C03-001 §6).
	HeaderText = "Pending items (from context/current-state.md)"

	// SyncNoticeText is the sync guidance without a leading icon (UI-C03-001 §6).
	SyncNoticeText = "If docs/product or docs/architecture changed, run /hero-sync then /hero-todos to refresh."
)

// PendingSectionHeadings lists ## headings whose list items are pending work
// (design D8; PRD-C03-001 §4.7). hero-sync merges doc-derived items here.
var PendingSectionHeadings = []string{
	"Pending Features",
}

var (
	listItemRE    = regexp.MustCompile(`^(\s*[-*]|\s*\d+\.)\s+(.*)$`)
	placeholderRE = regexp.MustCompile(`^_\(.+_\)_$`)
)

// Item is one pending entry from a configured section of current-state.md.
type Item struct {
	Section string
	Text    string
}

// HeaderLine returns the /hero-todos progress heading with the → prefix.
func HeaderLine() string {
	return "→ " + HeaderText
}

// SyncNoticeLine returns the sync guidance with the ⚠ prefix.
func SyncNoticeLine() string {
	return "⚠ " + SyncNoticeText
}

// Parse extracts pending list items from current-state.md content.
func Parse(content []byte) ([]Item, error) {
	pending := pendingSectionSet()
	var items []Item
	var section string
	inPending := false

	for line := range strings.Lines(string(content)) {
		line = strings.TrimRight(line, "\r\n")
		if h, ok := parseHeading(line); ok {
			section = h
			inPending = pending[h]
			continue
		}
		if !inPending {
			continue
		}
		text, ok := parseListItem(line)
		if !ok || isPlaceholder(text) {
			continue
		}
		items = append(items, Item{Section: section, Text: text})
	}
	return items, nil
}

// ReadFile parses pending items from the given current-state.md path.
func ReadFile(path string) ([]Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return Parse(data)
}

// ReadProject parses context/current-state.md under projectDir.
func ReadProject(projectDir string) ([]Item, error) {
	return ReadFile(filepath.Join(projectDir, CurrentStateRelPath))
}

// FormatLines returns display lines for /hero-todos per UI-C03-001 §6.
// Callers may pass lines to output.Progress / output.Warning for TTY styling.
func FormatLines(items []Item) []string {
	lines := []string{HeaderLine(), ""}
	for _, item := range items {
		lines = append(lines, "• "+item.Text)
	}
	lines = append(lines, "", SyncNoticeLine())
	return lines
}

// Format joins FormatLines with newlines.
func Format(items []Item) string {
	return strings.Join(FormatLines(items), "\n")
}

func pendingSectionSet() map[string]bool {
	set := make(map[string]bool, len(PendingSectionHeadings))
	for _, h := range PendingSectionHeadings {
		set[h] = true
	}
	return set
}

func parseHeading(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "## ") {
		return "", false
	}
	title := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
	return title, title != ""
}

func parseListItem(line string) (string, bool) {
	m := listItemRE.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	text := strings.TrimSpace(m[2])
	if text == "" {
		return "", false
	}
	return text, true
}

func isPlaceholder(text string) bool {
	return placeholderRE.MatchString(strings.TrimSpace(text))
}
