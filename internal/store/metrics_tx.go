package store

import (
	"database/sql"
	"fmt"
)

// UpsertMetricTx inserts or replaces a metrics row inside an open transaction.
func (s *Store) UpsertMetricTx(tx *sql.Tx, m Metric) error {
	_, err := tx.Exec(`
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
