package store

import (
	"path/filepath"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func TestMigrateLegacySessionBindingsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hero.db")
	ts := "2026-09-11T12:00:00Z"

	s, err := openCapped(path, 11)
	if err != nil {
		t.Fatal(err)
	}
	cycleID, err := s.CreateCycle(Cycle{
		Number:             3,
		Title:              "legacy-bindings",
		Status:             CycleStatusActive,
		StartedAt:          ts,
		ConfigSnapshotJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetOrchestrationSession(cycleID, "native-orch-1", "cursor"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStages([]Stage{
		{CycleID: cycleID, Name: "research", Status: StageWaiting, MaxIterations: 1, SortOrder: 0},
		{CycleID: cycleID, Name: "qa", Status: StageWaiting, MaxIterations: 1, SortOrder: 1},
		{CycleID: cycleID, Name: "implementation", Status: StageWaiting, MaxIterations: 1, SortOrder: 2},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStageSessionBinding(cycleID, "research", "cursor", "native-research"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStageSessionBinding(cycleID, "qa", "cursor", "native-qa"); err != nil {
		t.Fatal(err)
	}
	// Empty binding must be skipped.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	sessions, err := s2.ListSessions(ListSessionsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 3 {
		t.Fatalf("sessions=%d want 3 (orch + research + qa)", len(sessions))
	}
	byKey := map[string]Session{}
	for _, sess := range sessions {
		if !sess.LegacySourceKey.Valid {
			t.Fatalf("missing legacy_source_key on %+v", sess)
		}
		byKey[sess.LegacySourceKey.String] = sess
		if sess.TranscriptState != TranscriptUnavailableLegacy {
			t.Fatalf("transcript_state=%q want unavailable_legacy", sess.TranscriptState)
		}
	}
	orch := byKey[legacyOrchestrationSourceKey(cycleID)]
	if orch.Kind != SessionKindOrchestration || orch.Title != "C3 · Orchestration · ORCH" ||
		orch.NativeSessionID != "native-orch-1" || orch.HarnessID != "cursor" {
		t.Fatalf("orchestration session: %+v", orch)
	}
	research := byKey[legacyStageSourceKey(cycleID, "research")]
	if research.Kind != SessionKindResearch || research.Title != "C3 · Research · DISC" {
		t.Fatalf("research session: %+v", research)
	}
	qa := byKey[legacyStageSourceKey(cycleID, "qa")]
	if qa.Kind != SessionKindStageAgent || qa.Title != "C3 · QA · QA" {
		t.Fatalf("qa session: %+v", qa)
	}

	var eventCount int
	if err := s2.db.QueryRow(`SELECT COUNT(*) FROM session_events`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 0 {
		t.Fatalf("session_events=%d want 0", eventCount)
	}

	s3, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s3.Close()
	sessions2, err := s3.ListSessions(ListSessionsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions2) != 3 {
		t.Fatalf("second open sessions=%d want 3", len(sessions2))
	}
}

func TestLegacyStageDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		stage string
		want  string
	}{
		{"known_qa", "qa", "QA"},
		{"accented", "café_etape", "Café Etape"},
		{"cjk", "日本語_review", "日本語 Review"},
		{"emoji", "🎨_design", "🎨 Design"},
		{"combining_nfd", "cafe\u0301_room", "Café Room"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := legacyStageDisplayName(tc.stage)
			if norm.NFC.String(got) != norm.NFC.String(tc.want) {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("invalid UTF-8: %q", got)
			}
		})
	}
}
