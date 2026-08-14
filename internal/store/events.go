package store

import (
	"fmt"
	"strings"
)

// AppendEvent inserts an append-only event. Historical rows are never updated.
func (s *Store) AppendEvent(e Event) (int64, error) {
	ts := e.TS
	if ts == "" {
		ts = nowRFC3339()
	}
	payload := e.PayloadJSON
	if payload == "" {
		payload = "{}"
	}
	res, err := s.db.Exec(
		`INSERT INTO events(cycle_id, ts, type, payload_json) VALUES(?, ?, ?, ?)`,
		e.CycleID, ts, e.Type, payload,
	)
	if err != nil {
		return 0, fmt.Errorf("append event: %w", err)
	}
	return res.LastInsertId()
}

// ListEvents returns recent events for a cycle, newest last (chronological).
// If eventType is non-empty, filters by type. limit <= 0 means default 50.
func (s *Store) ListEvents(cycleID int64, eventType string, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 50
	}

	var (
		rows interface {
			Next() bool
			Scan(dest ...any) error
			Close() error
			Err() error
		}
		err error
	)

	if eventType == "" {
		rows, err = s.db.Query(`
SELECT id, cycle_id, ts, type, payload_json FROM events
WHERE cycle_id = ?
ORDER BY ts ASC, id ASC
LIMIT ?`, cycleID, limit)
	} else {
		rows, err = s.db.Query(`
SELECT id, cycle_id, ts, type, payload_json FROM events
WHERE cycle_id = ? AND type = ?
ORDER BY ts ASC, id ASC
LIMIT ?`, cycleID, eventType, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.CycleID, &e.TS, &e.Type, &e.PayloadJSON); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListEventsByTypes returns matching events for a cycle in chronological order.
func (s *Store) ListEventsByTypes(cycleID int64, types []string) ([]Event, error) {
	if len(types) == 0 {
		return s.ListEvents(cycleID, "", 0)
	}
	placeholders := make([]string, len(types))
	args := make([]any, 0, 1+len(types))
	args = append(args, cycleID)
	for i, t := range types {
		placeholders[i] = "?"
		args = append(args, t)
	}
	q := fmt.Sprintf(`
SELECT id, cycle_id, ts, type, payload_json FROM events
WHERE cycle_id = ? AND type IN (%s)
ORDER BY ts ASC, id ASC`, strings.Join(placeholders, ","))
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.CycleID, &e.TS, &e.Type, &e.PayloadJSON); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpsertMetric inserts or replaces a metrics row keyed by cycle+stage+agent.
func (s *Store) UpsertMetric(m Metric) error {
	_, err := s.db.Exec(`
INSERT INTO metrics(cycle_id, stage_name, model, agent, input_tokens, output_tokens, cost_usd, duration_ms)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(cycle_id, stage_name, agent) DO UPDATE SET
  model = excluded.model,
  input_tokens = excluded.input_tokens,
  output_tokens = excluded.output_tokens,
  cost_usd = excluded.cost_usd,
  duration_ms = excluded.duration_ms`,
		m.CycleID, m.StageName, m.Model, m.Agent,
		m.InputTokens, m.OutputTokens, m.CostUSD, m.DurationMS,
	)
	if err != nil {
		return fmt.Errorf("upsert metric: %w", err)
	}
	return nil
}

// ListMetrics returns all metrics for a cycle.
func (s *Store) ListMetrics(cycleID int64) ([]Metric, error) {
	rows, err := s.db.Query(`
SELECT id, cycle_id, stage_name, model, agent, input_tokens, output_tokens, cost_usd, duration_ms
FROM metrics WHERE cycle_id = ? ORDER BY id ASC`, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(
			&m.ID, &m.CycleID, &m.StageName, &m.Model, &m.Agent,
			&m.InputTokens, &m.OutputTokens, &m.CostUSD, &m.DurationMS,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddArtifact inserts artifact metadata.
func (s *Store) AddArtifact(a Artifact) (int64, error) {
	created := a.CreatedAt
	if created == "" {
		created = nowRFC3339()
	}
	res, err := s.db.Exec(
		`INSERT INTO artifacts(cycle_id, path, kind, label, created_at) VALUES(?, ?, ?, ?, ?)`,
		a.CycleID, a.Path, a.Kind, a.Label, created,
	)
	if err != nil {
		return 0, fmt.Errorf("insert artifact: %w", err)
	}
	return res.LastInsertId()
}

// ListArtifacts returns artifact metadata for a cycle.
func (s *Store) ListArtifacts(cycleID int64) ([]Artifact, error) {
	rows, err := s.db.Query(`
SELECT id, cycle_id, path, kind, label, created_at
FROM artifacts WHERE cycle_id = ? ORDER BY id ASC`, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Artifact
	for rows.Next() {
		var a Artifact
		if err := rows.Scan(&a.ID, &a.CycleID, &a.Path, &a.Kind, &a.Label, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AddConversation inserts a conversation entry.
func (s *Store) AddConversation(c ConversationEntry) (int64, error) {
	ts := c.TS
	if ts == "" {
		ts = nowRFC3339()
	}
	res, err := s.db.Exec(
		`INSERT INTO conversation(cycle_id, ts, role, kind, body) VALUES(?, ?, ?, ?, ?)`,
		c.CycleID, ts, c.Role, c.Kind, c.Body,
	)
	if err != nil {
		return 0, fmt.Errorf("insert conversation: %w", err)
	}
	return res.LastInsertId()
}

// ListConversation returns conversation entries for a cycle.
func (s *Store) ListConversation(cycleID int64) ([]ConversationEntry, error) {
	rows, err := s.db.Query(`
SELECT id, cycle_id, ts, role, kind, body
FROM conversation WHERE cycle_id = ? ORDER BY ts ASC, id ASC`, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ConversationEntry
	for rows.Next() {
		var c ConversationEntry
		if err := rows.Scan(&c.ID, &c.CycleID, &c.TS, &c.Role, &c.Kind, &c.Body); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
