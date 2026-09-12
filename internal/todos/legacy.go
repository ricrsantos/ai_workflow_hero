package todos

import (
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// NormalizeLegacyLine collapses whitespace for stable legacy line identity.
func NormalizeLegacyLine(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

// StructuredLine is a parsed Hero-managed projection list item.
type StructuredLine struct {
	ID      string
	Status  string
	Origin  string
	Summary string
}

// IsStructuredProjectedLine reports whether text matches the Hero projection format.
func IsStructuredProjectedLine(text string) bool {
	_, ok := ParseStructuredProjectedLine(text)
	return ok
}

// ParseStructuredProjectedLine parses `- `id` · status · origin · summary`.
func ParseStructuredProjectedLine(text string) (StructuredLine, bool) {
	trimmed := strings.TrimSpace(text)
	if m := listItemRE.FindStringSubmatch(trimmed); m != nil {
		trimmed = strings.TrimSpace(m[2])
	}
	const sep = " · "
	parts := strings.Split(trimmed, sep)
	if len(parts) != 4 {
		return StructuredLine{}, false
	}
	idPart := strings.TrimSpace(parts[0])
	if !strings.HasPrefix(idPart, "`") || !strings.HasSuffix(idPart, "`") {
		return StructuredLine{}, false
	}
	id := strings.Trim(idPart, "`")
	if id == "" {
		return StructuredLine{}, false
	}
	return StructuredLine{
		ID:      id,
		Status:  strings.TrimSpace(parts[1]),
		Origin:  strings.TrimSpace(parts[2]),
		Summary: strings.TrimSpace(parts[3]),
	}, true
}

// MatchLegacyPendingLine finds an unmatched pending list item by normalized content.
func MatchLegacyPendingLine(items []Item, normalized string) (Item, bool) {
	want := NormalizeLegacyLine(normalized)
	if want == "" {
		return Item{}, false
	}
	for _, item := range items {
		if IsStructuredProjectedLine(item.Text) {
			continue
		}
		if NormalizeLegacyLine(item.Text) == want {
			return item, true
		}
	}
	return Item{}, false
}

// PromoteSelectedLegacyLine allocates todo-N for one selected legacy Pending line.
// It does not import other unmatched lines into SQLite.
func PromoteSelectedLegacyLine(st *store.Store, projectDir, selectedLineText string) (string, error) {
	if st == nil {
		return "", fmt.Errorf("store is required")
	}
	if sl, ok := ParseStructuredProjectedLine(selectedLineText); ok {
		if strings.HasPrefix(sl.ID, "todo-") || strings.HasPrefix(sl.ID, "find-") {
			return sl.ID, nil
		}
	}
	norm := NormalizeLegacyLine(selectedLineText)
	if norm == "" {
		return "", fmt.Errorf("legacy line text is empty")
	}
	items, err := ReadProject(projectDir)
	if err != nil {
		return "", err
	}
	if _, ok := MatchLegacyPendingLine(items, norm); !ok {
		return "", fmt.Errorf("selected legacy line not found in pending projection")
	}
	projected, err := st.ListTodosForProjection()
	if err != nil {
		return "", err
	}
	for _, t := range projected {
		if t.OriginType != store.TodoOriginLegacy {
			continue
		}
		if NormalizeLegacyLine(t.Summary) == norm {
			return t.ID, nil
		}
	}
	return st.CreateLegacyTodo(store.CreateLegacyTodoParams{Summary: norm})
}
