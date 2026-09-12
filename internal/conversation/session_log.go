package conversation

import "strings"

// C16 diagnostic logs must not include session IDs, native IDs, prompts, or provider payloads (PRD-C16-001 §5).

func logAttrKind(kind string) []any {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return nil
	}
	return []any{"kind", kind}
}

func logAttrEventType(eventType string) []any {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return nil
	}
	return []any{"event_type", eventType}
}

func logAttrImported(count int) []any {
	return []any{"imported", count}
}
