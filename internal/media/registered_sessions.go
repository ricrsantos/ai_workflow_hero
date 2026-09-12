package media

import "strings"

// RegisteredSessionIDs is the set of Hero session IDs whose on-disk directories
// must not be removed by age-based startup cleanup (C16 / ADR-096).
type RegisteredSessionIDs map[string]struct{}

// RegisteredSessionIDSet builds a set from database session primary keys.
func RegisteredSessionIDSet(ids []string) RegisteredSessionIDs {
	set := make(RegisteredSessionIDs, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			set[id] = struct{}{}
		}
	}
	return set
}

// Contains reports whether id is a registered durable session directory name.
func (set RegisteredSessionIDs) Contains(id string) bool {
	if len(set) == 0 {
		return false
	}
	_, ok := set[strings.TrimSpace(id)]
	return ok
}
