package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// CapabilityCacheRow is a project-scoped normalized capability cache row (schema v5).
type CapabilityCacheRow struct {
	Harness        string
	Model          string
	PropertiesJSON string
	RetrievedAt    string
}

// ModelListCacheRow is a project-scoped model list cache row (schema v5).
type ModelListCacheRow struct {
	Harness     string
	ModelsJSON  string
	RefreshedAt string
}

// UpsertModelList replaces the cached model list for a harness. Successful API
// responses are authoritative: the row is replaced atomically.
func (s *Store) UpsertModelList(harness string, models []string, refreshedAt string) error {
	harness = strings.TrimSpace(harness)
	if harness == "" {
		return fmt.Errorf("harness is required")
	}
	data, err := json.Marshal(models)
	if err != nil {
		return fmt.Errorf("marshal model list: %w", err)
	}
	if _, err := s.db.Exec(`
INSERT INTO model_list_cache(harness, models_json, refreshed_at) VALUES(?, ?, ?)
ON CONFLICT(harness) DO UPDATE SET models_json = excluded.models_json, refreshed_at = excluded.refreshed_at`,
		harness, string(data), refreshedAt); err != nil {
		return fmt.Errorf("upsert model list cache: %w", err)
	}
	s.log.Info("model list cache updated", "harness", harness, "models", len(models))
	return nil
}

// ModelList reads the cached model list for a harness regardless of age.
// Returns (nil, "", nil) when no row exists.
func (s *Store) ModelList(harness string) ([]string, string, error) {
	harness = strings.TrimSpace(harness)
	var raw, refreshedAt string
	err := s.db.QueryRow(`
SELECT models_json, refreshed_at FROM model_list_cache WHERE harness = ?`, harness).
		Scan(&raw, &refreshedAt)
	if err == sql.ErrNoRows {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	var models []string
	if err := json.Unmarshal([]byte(raw), &models); err != nil {
		return nil, "", fmt.Errorf("unmarshal cached model list: %w", err)
	}
	return models, refreshedAt, nil
}

// UpsertCapabilities replaces the cached normalized capabilities for one
// harness/model pair (API-authoritative replacement semantics; ADR-039).
func (s *Store) UpsertCapabilities(row CapabilityCacheRow) error {
	row.Harness = strings.TrimSpace(row.Harness)
	row.Model = strings.TrimSpace(row.Model)
	if row.Harness == "" || row.Model == "" {
		return fmt.Errorf("harness and model are required")
	}
	if _, err := s.db.Exec(`
INSERT INTO model_capability_cache(harness, model, properties_json, retrieved_at) VALUES(?, ?, ?, ?)
ON CONFLICT(harness, model) DO UPDATE SET
  properties_json = excluded.properties_json,
  retrieved_at = excluded.retrieved_at`,
		row.Harness, row.Model, row.PropertiesJSON, row.RetrievedAt); err != nil {
		return fmt.Errorf("upsert capability cache: %w", err)
	}
	s.log.Debug("capability cache upserted", "harness", row.Harness, "model", row.Model)
	return nil
}

// Capabilities reads the cached row for a harness/model regardless of age.
// Returns (CapabilityCacheRow{}, false, nil) when no row exists; the persisted
// timestamp is retained so callers can report staleness.
func (s *Store) Capabilities(harness, model string) (CapabilityCacheRow, bool, error) {
	harness = strings.TrimSpace(harness)
	model = strings.TrimSpace(model)
	var row CapabilityCacheRow
	err := s.db.QueryRow(`
SELECT harness, model, properties_json, retrieved_at
FROM model_capability_cache WHERE harness = ? AND model = ?`, harness, model).
		Scan(&row.Harness, &row.Model, &row.PropertiesJSON, &row.RetrievedAt)
	if err == sql.ErrNoRows {
		return CapabilityCacheRow{}, false, nil
	}
	if err != nil {
		return CapabilityCacheRow{}, false, err
	}
	return row, true, nil
}

// ListCapabilities returns every cached capability row for one harness.
func (s *Store) ListCapabilities(harness string) ([]CapabilityCacheRow, error) {
	harness = strings.TrimSpace(harness)
	rows, err := s.db.Query(`
SELECT harness, model, properties_json, retrieved_at
FROM model_capability_cache WHERE harness = ? ORDER BY model ASC`, harness)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CapabilityCacheRow
	for rows.Next() {
		var r CapabilityCacheRow
		if err := rows.Scan(&r.Harness, &r.Model, &r.PropertiesJSON, &r.RetrievedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BeginRefresh marks a background refresh as pending for a harness and returns
// the new generation number (C5 refresh coordination).
func (s *Store) BeginRefresh(harness string) (int64, error) {
	harness = strings.TrimSpace(harness)
	if harness == "" {
		return 0, fmt.Errorf("harness is required")
	}
	if _, err := s.db.Exec(`
INSERT INTO model_refresh_state(harness, generation, pending, updated_at)
VALUES(?, 1, 1, ?)
ON CONFLICT(harness) DO UPDATE SET
  generation = generation + 1,
  pending = 1,
  updated_at = excluded.updated_at`,
		harness, nowRFC3339()); err != nil {
		return 0, fmt.Errorf("begin refresh: %w", err)
	}
	var gen int64
	if err := s.db.QueryRow(`SELECT generation FROM model_refresh_state WHERE harness = ?`, harness).Scan(&gen); err != nil {
		return 0, err
	}
	return gen, nil
}

// CompleteRefresh marks a refresh generation as done (no-op for stale generations).
func (s *Store) CompleteRefresh(harness string, generation int64) error {
	harness = strings.TrimSpace(harness)
	if _, err := s.db.Exec(`
UPDATE model_refresh_state SET pending = 0, updated_at = ?
WHERE harness = ? AND generation = ?`,
		nowRFC3339(), harness, generation); err != nil {
		return fmt.Errorf("complete refresh: %w", err)
	}
	return nil
}

// RefreshState reports the pending refresh generation for a harness.
func (s *Store) RefreshState(harness string) (generation int64, pending bool, err error) {
	harness = strings.TrimSpace(harness)
	err = s.db.QueryRow(`
SELECT generation, pending FROM model_refresh_state WHERE harness = ?`, harness).
		Scan(&generation, &pending)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return generation, pending, nil
}
