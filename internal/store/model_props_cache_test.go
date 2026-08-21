package store

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func newStoreAt(t *testing.T, dir string) *Store {
	t.Helper()
	st, err := Open(filepath.Join(dir, DBFileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestSchemaV5MigrationFromV4(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DBFileName)

	// Build a v4 database with operational data.
	v4, err := openCapped(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v4.db.Exec(`INSERT INTO cycles(number, title, objective, status)
VALUES(1, 'c4 cycle', 'keep me', 'completed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := v4.db.Exec(`INSERT INTO harness_serve_registry(harness, pid, port, url, created_at)
VALUES('opencode', 42, 4096, 'http://127.0.0.1:4096', '2026-08-15T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := v4.Close(); err != nil {
		t.Fatal(err)
	}

	st := newStoreAt(t, dir)
	v, err := st.SchemaVersion()
	if err != nil || v != currentSchemaVersion {
		t.Fatalf("version=%d err=%v want %d", v, err, currentSchemaVersion)
	}
	var title string
	if err := st.db.QueryRow(`SELECT title FROM cycles WHERE number = 1`).Scan(&title); err != nil {
		t.Fatalf("v4 cycle data must remain readable: %v", err)
	}
	if title != "c4 cycle" {
		t.Fatalf("title=%q", title)
	}
	entries, err := st.ListServeRegistry()
	if err != nil || len(entries) != 1 || entries[0].Harness != "opencode" {
		t.Fatalf("v4 serve registry must remain readable: %v %+v", err, entries)
	}
}

func TestSchemaV6MigrationFromV5(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DBFileName)

	v5, err := openCapped(path, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v5.db.Exec(`INSERT INTO harness_serve_registry(harness, pid, port, url, created_at)
VALUES('opencode', 42, 4096, 'http://127.0.0.1:4096', '2026-08-15T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := v5.Close(); err != nil {
		t.Fatal(err)
	}

	st := newStoreAt(t, dir)
	v, err := st.SchemaVersion()
	if err != nil || v != currentSchemaVersion {
		t.Fatalf("version=%d err=%v want %d", v, err, currentSchemaVersion)
	}
	entries, err := st.ListServeRegistry()
	if err != nil || len(entries) != 1 {
		t.Fatalf("serve registry: %v %+v", err, entries)
	}
	if entries[0].ProjectPath != "" {
		t.Fatalf("project_path=%q want empty default", entries[0].ProjectPath)
	}
}

func TestCapabilityCacheRoundTripPreservesTimestamp(t *testing.T) {
	st := newStoreAt(t, t.TempDir())
	props := map[string]any{
		"fs": map[string]any{"available": true, "values": []string{"true", "false"}, "default": "false"},
		"th": map[string]any{"available": true, "values": []string{"off", "max"}, "default": "off"},
	}
	raw, err := json.Marshal(props)
	if err != nil {
		t.Fatal(err)
	}
	ts := "2026-08-17T10:00:00Z"
	if err := st.UpsertCapabilities(CapabilityCacheRow{
		Harness: "opencode", Model: "opencode-go/deepseek-v4-pro",
		PropertiesJSON: string(raw), RetrievedAt: ts,
	}); err != nil {
		t.Fatal(err)
	}
	row, ok, err := st.Capabilities("opencode", "opencode-go/deepseek-v4-pro")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if row.RetrievedAt != ts || row.PropertiesJSON != string(raw) {
		t.Fatalf("round trip mismatch: %+v", row)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(row.PropertiesJSON), &got); err != nil {
		t.Fatal(err)
	}
	if got["th"] == nil || got["fs"] == nil {
		t.Fatalf("normalized json lost keys: %v", got)
	}
}

func TestCapabilityCacheAPIReplacementSemantics(t *testing.T) {
	st := newStoreAt(t, t.TempDir())
	oldProps := `{"ef":{"available":true,"values":["low","high"],"default":"low"}}`
	newProps := `{"ef":{"available":true,"values":["medium"],"default":"medium"}}`
	if err := st.UpsertCapabilities(CapabilityCacheRow{
		Harness: "opencode", Model: "m1",
		PropertiesJSON: oldProps, RetrievedAt: "2026-08-16T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertCapabilities(CapabilityCacheRow{
		Harness: "opencode", Model: "m1",
		PropertiesJSON: newProps, RetrievedAt: "2026-08-17T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	row, ok, err := st.Capabilities("opencode", "m1")
	if err != nil || !ok {
		t.Fatal(err)
	}
	if row.PropertiesJSON != newProps {
		t.Fatalf("successful refresh must replace cached values: %s", row.PropertiesJSON)
	}
	if row.RetrievedAt != "2026-08-17T00:00:00Z" {
		t.Fatalf("timestamp must be replaced: %s", row.RetrievedAt)
	}
	rows, err := st.ListCapabilities("opencode")
	if err != nil || len(rows) != 1 {
		t.Fatalf("list: %v %+v", err, rows)
	}
}

func TestCacheStaleReadAfterFailure(t *testing.T) {
	st := newStoreAt(t, t.TempDir())
	props := `{"th":{"available":true,"values":["off"],"default":"off"}}`
	if err := st.UpsertCapabilities(CapabilityCacheRow{
		Harness: "opencode", Model: "m1",
		PropertiesJSON: props, RetrievedAt: "2026-08-15T09:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	// A failed refresh never deletes the old row; the original timestamp is retained.
	row, ok, err := st.Capabilities("opencode", "m1")
	if err != nil || !ok {
		t.Fatal(err)
	}
	if row.RetrievedAt != "2026-08-15T09:00:00Z" {
		t.Fatalf("stale row must retain its original timestamp: %s", row.RetrievedAt)
	}
	if !strings.Contains(row.PropertiesJSON, `"off"`) {
		t.Fatalf("stale data must remain readable: %s", row.PropertiesJSON)
	}
}

func TestCacheCrossProjectIsolation(t *testing.T) {
	stA := newStoreAt(t, t.TempDir())
	stB := newStoreAt(t, t.TempDir())
	propsA := `{"ef":{"available":true,"values":["high"],"default":"high"}}`
	propsB := `{"ef":{"available":true,"values":["low"],"default":"low"}}`
	if err := stA.UpsertCapabilities(CapabilityCacheRow{Harness: "opencode", Model: "same/model", PropertiesJSON: propsA, RetrievedAt: "2026-08-17T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := stB.UpsertCapabilities(CapabilityCacheRow{Harness: "opencode", Model: "same/model", PropertiesJSON: propsB, RetrievedAt: "2026-08-17T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	rowA, okA, _ := stA.Capabilities("opencode", "same/model")
	rowB, okB, _ := stB.Capabilities("opencode", "same/model")
	if !okA || !okB {
		t.Fatal("both projects must have rows")
	}
	if rowA.PropertiesJSON != propsA || rowB.PropertiesJSON != propsB {
		t.Fatalf("cross-project leakage: A=%s B=%s", rowA.PropertiesJSON, rowB.PropertiesJSON)
	}
	if _, ok, _ := stA.Capabilities("opencode", "absent/model"); ok {
		t.Fatal("absent row must report ok=false")
	}
}

func TestModelListCacheUpsertAndRead(t *testing.T) {
	st := newStoreAt(t, t.TempDir())
	if err := st.UpsertModelList("cursor", []string{"composer-2.5", "grok-4.6"}, "2026-08-17T10:00:00Z"); err != nil {
		t.Fatal(err)
	}
	models, ts, err := st.ModelList("cursor")
	if err != nil || len(models) != 2 || models[1] != "grok-4.6" {
		t.Fatalf("models=%v ts=%s err=%v", models, ts, err)
	}
	if err := st.UpsertModelList("cursor", []string{"composer-2.5"}, "2026-08-17T11:00:00Z"); err != nil {
		t.Fatal(err)
	}
	models, ts, _ = st.ModelList("cursor")
	if len(models) != 1 || ts != "2026-08-17T11:00:00Z" {
		t.Fatalf("replacement failed: %v %s", models, ts)
	}
	if _, _, err := st.ModelList("opencode"); err != nil {
		t.Fatalf("missing row must be a clean miss: %v", err)
	}
}

func TestRefreshStateGenerationMarkers(t *testing.T) {
	st := newStoreAt(t, t.TempDir())
	gen, err := st.BeginRefresh("opencode")
	if err != nil || gen != 1 {
		t.Fatalf("gen=%d err=%v", gen, err)
	}
	g2, pending, err := st.RefreshState("opencode")
	if err != nil || g2 != 1 || !pending {
		t.Fatalf("state gen=%d pending=%v err=%v", g2, pending, err)
	}
	if err := st.CompleteRefresh("opencode", gen); err != nil {
		t.Fatal(err)
	}
	_, pending, err = st.RefreshState("opencode")
	if err != nil || pending {
		t.Fatalf("pending=%v err=%v after completion", pending, err)
	}
	// A stale completion (older generation) must not clear a newer pending run.
	gen2, err := st.BeginRefresh("opencode")
	if err != nil || gen2 != 2 {
		t.Fatalf("gen2=%d err=%v", gen2, err)
	}
	if err := st.CompleteRefresh("opencode", gen); err != nil {
		t.Fatal(err)
	}
	_, pending, err = st.RefreshState("opencode")
	if err != nil || !pending {
		t.Fatalf("stale completion must not clear pending run: pending=%v err=%v", pending, err)
	}
	if _, pending, err := st.RefreshState("cursor"); err != nil || pending {
		t.Fatalf("untouched harness must not be pending: pending=%v err=%v", pending, err)
	}
}
