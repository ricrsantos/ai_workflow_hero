package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestFindingFingerprintCanonicalization(t *testing.T) {
	cycleID := int64(42)
	base := FindingFingerprint(cycleID, FindingSourceQA, FindingOwnerGeneric,
		"./internal/store/findings.go", "req", "ac one")
	altFile := FindingFingerprint(cycleID, FindingSourceQA, FindingOwnerGeneric,
		"internal/store/findings.go", "req", "ac one")
	if base != altFile {
		t.Fatalf("file canonicalization mismatch")
	}
	ws := FindingFingerprint(cycleID, FindingSourceQA, FindingOwnerGeneric,
		"internal/store/findings.go", "req", "ac\t\none")
	if ws != altFile {
		t.Fatalf("acceptance whitespace normalization mismatch")
	}
	diffAC := FindingFingerprint(cycleID, FindingSourceQA, FindingOwnerGeneric,
		"internal/store/findings.go", "req", "ac two")
	if base == diffAC {
		t.Fatal("fingerprint must differ when acceptance changes")
	}
}

func TestPersistFindingCreateRediscoverReopen(t *testing.T) {
	s, cycleID := openTestStoreWithCycle(t)
	base := FindingInput{
		CycleID:            cycleID,
		SourceStage:        FindingSourceQA,
		Owner:              FindingOwnerGeneric,
		File:               "internal/store/findings.go",
		Requirement:        "PRD-C15 §5",
		Issue:              "first issue",
		AcceptanceCriteria: "tests pass",
	}

	res, err := s.PersistFinding(base)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res.Finding.ID != "find-qa-1" || res.OccurrenceKind != OccurrenceCreated || !res.Actionable {
		t.Fatalf("create result = %+v", res)
	}

	in := base
	in.Issue = "same fingerprint different issue"
	res, err = s.PersistFinding(in)
	if err != nil {
		t.Fatalf("rediscover: %v", err)
	}
	if res.Finding.ID != "find-qa-1" || res.OccurrenceKind != OccurrenceRediscovered || !res.Actionable {
		t.Fatalf("rediscover result = %+v", res)
	}
	all, err := s.ListFindingsByCycle(cycleID)
	if err != nil || len(all) != 1 {
		t.Fatalf("findings after rediscover = %+v err=%v", all, err)
	}

	if err := s.InTx(func(tx *sql.Tx) error {
		return s.MarkFindingDoneTx(tx, cycleID, "find-qa-1", "done issue", "tests pass", "[]")
	}); err != nil {
		t.Fatalf("mark done: %v", err)
	}
	in.Issue = "regression after done"
	res, err = s.PersistFinding(in)
	if err != nil {
		t.Fatalf("reopen fingerprint: %v", err)
	}
	if res.Finding.Status != FindingStatusReopened || res.Finding.Round != 2 || !res.Actionable {
		t.Fatalf("reopen result = %+v", res)
	}
	kind, n, err := lastOccurrence(s, cycleID, "find-qa-1")
	if err != nil || kind != OccurrenceReopened || n < 3 {
		t.Fatalf("occurrences kind=%q count=%d err=%v", kind, n, err)
	}
}

func TestPersistFindingReopenViaReopenID(t *testing.T) {
	s, cycleID := openTestStoreWithCycle(t)
	in := FindingInput{
		CycleID:            cycleID,
		SourceStage:        FindingSourceJudge,
		Owner:              FindingOwnerBackend,
		File:               "internal/engine/engine.go",
		Requirement:        "ADR-084",
		Issue:              "judge gap",
		AcceptanceCriteria: "atomic tx",
	}
	res, err := s.PersistFinding(in)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	if err := s.InTx(func(tx *sql.Tx) error {
		return s.MarkFindingDoneTx(tx, cycleID, res.Finding.ID, in.Issue, in.AcceptanceCriteria, "[]")
	}); err != nil {
		t.Fatalf("mark done: %v", err)
	}
	in.ReopenID = res.Finding.ID
	in.Issue = "reopen by id"
	res, err = s.PersistFinding(in)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if res.Finding.ID != "find-judge-1" || res.Finding.Round != 2 || res.OccurrenceKind != OccurrenceReopened {
		t.Fatalf("result = %+v", res)
	}
}

func TestPersistFindingDeferredRecurrence(t *testing.T) {
	s, cycleID := openTestStoreWithCycle(t)
	in := FindingInput{
		CycleID:            cycleID,
		SourceStage:        FindingSourceQA,
		Owner:              FindingOwnerFrontend,
		File:               "internal/tui/screens.go",
		Requirement:        "UI-C15",
		Issue:              "defer me",
		AcceptanceCriteria: "show findings",
	}
	res, err := s.PersistFinding(in)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	if err := s.InTx(func(tx *sql.Tx) error {
		return s.SetFindingDeferredTodoTx(tx, cycleID, res.Finding.ID, in.Issue, in.AcceptanceCriteria, "[]")
	}); err != nil {
		t.Fatalf("defer: %v", err)
	}
	in.Issue = "deferred again"
	res, err = s.PersistFinding(in)
	if err != nil {
		t.Fatalf("recurrence: %v", err)
	}
	if res.OccurrenceKind != OccurrenceDeferredRecurrence || res.Actionable || res.Finding.Status != FindingStatusDeferredTodo {
		t.Fatalf("result = %+v", res)
	}
	kind, _, err := lastOccurrence(s, cycleID, res.Finding.ID)
	if err != nil || kind != OccurrenceDeferredRecurrence {
		t.Fatalf("last kind=%q err=%v", kind, err)
	}
}

func TestPersistFindingRejectsUnsafeContent(t *testing.T) {
	s, cycleID := openTestStoreWithCycle(t)
	token := "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"

	tests := []struct {
		name string
		in   FindingInput
	}{
		{
			name: "secret in issue",
			in: FindingInput{
				CycleID: cycleID, SourceStage: FindingSourceQA, Owner: FindingOwnerGeneric,
				File: "internal/x.go", Issue: "leak " + token, AcceptanceCriteria: "safe",
			},
		},
		{
			name: "data url in acceptance",
			in: FindingInput{
				CycleID: cycleID, SourceStage: FindingSourceQA, Owner: FindingOwnerGeneric,
				File: "internal/x.go", Issue: "bad evidence", AcceptanceCriteria: "data:image/png;base64,abc",
			},
		},
		{
			name: "sensitive evidence path",
			in: FindingInput{
				CycleID: cycleID, SourceStage: FindingSourceQA, Owner: FindingOwnerGeneric,
				File: "internal/x.go", Issue: "bad path", AcceptanceCriteria: "safe",
				Evidence: []string{".env"},
			},
		},
		{
			name: "traversal evidence path",
			in: FindingInput{
				CycleID: cycleID, SourceStage: FindingSourceQA, Owner: FindingOwnerGeneric,
				File: "internal/x.go", Issue: "bad path", AcceptanceCriteria: "safe",
				Evidence: []string{"../secret.txt"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.PersistFinding(tc.in)
			if !errors.Is(err, ErrInvalidFindingContent) {
				t.Fatalf("err = %v want ErrInvalidFindingContent", err)
			}
		})
	}
}

func TestFindingIDAllocationPerNamespace(t *testing.T) {
	s, cycleID := openTestStoreWithCycle(t)
	stages := []struct {
		stage FindingInput
		want  string
	}{
		{FindingInput{CycleID: cycleID, SourceStage: FindingSourceQA, Owner: FindingOwnerGeneric, File: "a.go", Issue: "i", AcceptanceCriteria: "ac"}, "find-qa-1"},
		{FindingInput{CycleID: cycleID, SourceStage: FindingSourceQA, Owner: FindingOwnerGeneric, File: "b.go", Issue: "i2", AcceptanceCriteria: "ac2"}, "find-qa-2"},
		{FindingInput{CycleID: cycleID, SourceStage: FindingSourceJudge, Owner: FindingOwnerBackend, File: "c.go", Issue: "j", AcceptanceCriteria: "ac"}, "find-judge-1"},
		{FindingInput{CycleID: cycleID, SourceStage: FindingSourceBrowserUI, Owner: FindingOwnerFrontend, File: "d.go", Issue: "b", AcceptanceCriteria: "ac"}, "find-bui-1"},
		{FindingInput{CycleID: cycleID, SourceStage: FindingSourceQAEndToEnd, Owner: FindingOwnerGeneric, File: "e.go", Issue: "e", AcceptanceCriteria: "ac"}, "find-e2e-1"},
	}
	for _, tc := range stages {
		res, err := s.PersistFinding(tc.stage)
		if err != nil {
			t.Fatalf("%s: %v", tc.want, err)
		}
		if res.Finding.ID != tc.want {
			t.Fatalf("got id %q want %q", res.Finding.ID, tc.want)
		}
	}
}

func TestListActionableFindingsByOwner(t *testing.T) {
	s, cycleID := openTestStoreWithCycle(t)
	_, err := s.PersistFinding(FindingInput{
		CycleID: cycleID, SourceStage: FindingSourceQA, Owner: FindingOwnerGeneric,
		File: "a.go", Issue: "one", AcceptanceCriteria: "ac",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.PersistFinding(FindingInput{
		CycleID: cycleID, SourceStage: FindingSourceQA, Owner: FindingOwnerBackend,
		File: "b.go", Issue: "two", AcceptanceCriteria: "ac",
	})
	if err != nil {
		t.Fatal(err)
	}
	gen, err := s.ListActionableFindings(cycleID, FindingOwnerGeneric)
	if err != nil || len(gen) != 1 || gen[0].ID != "find-qa-1" {
		t.Fatalf("generic actionable = %+v err=%v", gen, err)
	}
	all, err := s.ListActionableFindings(cycleID, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("all actionable = %+v err=%v", all, err)
	}
	reopened, err := s.ListFindingsByStatus(cycleID, FindingStatusOpen, FindingStatusReopened)
	if err != nil || len(reopened) != 2 {
		t.Fatalf("status query = %+v err=%v", reopened, err)
	}
}

func TestPersistFindingReopenedRediscovery(t *testing.T) {
	s, cycleID := openTestStoreWithCycle(t)
	in := FindingInput{
		CycleID: cycleID, SourceStage: FindingSourceQA, Owner: FindingOwnerGeneric,
		File: "pkg.go", Issue: "first", AcceptanceCriteria: "fix",
	}
	res, err := s.PersistFinding(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.InTx(func(tx *sql.Tx) error {
		return s.MarkFindingDoneTx(tx, cycleID, res.Finding.ID, in.Issue, in.AcceptanceCriteria, "[]")
	}); err != nil {
		t.Fatal(err)
	}
	_, err = s.PersistFinding(in)
	if err != nil {
		t.Fatal(err)
	}
	in.Issue = "still broken"
	res, err = s.PersistFinding(in)
	if err != nil {
		t.Fatal(err)
	}
	if res.OccurrenceKind != OccurrenceRediscovered || res.Finding.Status != FindingStatusReopened {
		t.Fatalf("result = %+v", res)
	}
}

func openTestStoreWithCycle(t *testing.T) (*Store, int64) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hero.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	id, err := s.CreateCycle(Cycle{Number: 1, Title: "findings", Status: CycleStatusActive, StartedAt: nowRFC3339()})
	if err != nil {
		t.Fatalf("CreateCycle: %v", err)
	}
	return s, id
}

func lastOccurrence(s *Store, cycleID int64, findingID string) (kind string, count int, err error) {
	rows, err := s.DB().Query(`
SELECT kind FROM finding_occurrences WHERE cycle_id = ? AND finding_id = ? ORDER BY sequence ASC`, cycleID, findingID)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()
	var kinds []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return "", 0, err
		}
		kinds = append(kinds, k)
	}
	if len(kinds) == 0 {
		return "", 0, errors.New("no occurrences")
	}
	return kinds[len(kinds)-1], len(kinds), rows.Err()
}
