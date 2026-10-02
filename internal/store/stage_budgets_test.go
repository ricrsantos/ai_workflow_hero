package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openStageBudgetStore(t *testing.T) (*Store, int64, time.Time) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "hero.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cycleID, err := s.CreateCycle(Cycle{Number: 1, Title: "budget test", Status: CycleStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"research", "planning", "implementation", "qa", "judge", "browser_ui_validation", "qa_end_to_end"}
	stages := make([]Stage, 0, len(names))
	for i, name := range names {
		stages = append(stages, Stage{CycleID: cycleID, Name: name, Status: StageRunning, TimeoutMinutes: 10, SortOrder: i})
	}
	if err := s.CreateStages(stages); err != nil {
		t.Fatal(err)
	}
	return s, cycleID, time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
}

func TestActiveBudgetCoversAllStagesAndParallelWorkCountsOnce(t *testing.T) {
	s, cycleID, start := openStageBudgetStore(t)
	names := []string{"research", "planning", "implementation", "qa", "judge", "browser_ui_validation", "qa_end_to_end"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			b, err := s.BeginStageBudget(cycleID, name, 10*time.Minute, start)
			if err != nil {
				t.Fatal(err)
			}
			checkpoint := start.Add(3 * time.Minute)
			b, err = s.CheckpointStageBudget(cycleID, name, b.Generation, checkpoint)
			if err != nil {
				t.Fatal(err)
			}
			if b.Consumed != 3*time.Minute {
				t.Fatalf("first sibling checkpoint consumed %s, want 3m", b.Consumed)
			}
			// Another concurrently running agent shares the same stage clock; a
			// checkpoint at the same instant must not charge the interval twice.
			b, err = s.CheckpointStageBudget(cycleID, name, b.Generation, checkpoint)
			if err != nil {
				t.Fatal(err)
			}
			if b.Consumed != 3*time.Minute {
				t.Fatalf("parallel sibling duplicated elapsed time: %s", b.Consumed)
			}
			if got := b.RemainingAt(checkpoint); got != 7*time.Minute {
				t.Fatalf("remaining=%s want 7m", got)
			}
		})
	}
}

func TestActiveBudgetPausesHumanWaitAndRestartUsesLastCheckpoint(t *testing.T) {
	s, cycleID, start := openStageBudgetStore(t)
	b, err := s.BeginStageBudget(cycleID, "qa", 10*time.Minute, start)
	if err != nil {
		t.Fatal(err)
	}
	firstCheckpoint := start.Add(3 * time.Minute)
	b, err = s.CheckpointStageBudget(cycleID, "qa", b.Generation, firstCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	pausedAt := start.Add(5 * time.Minute)
	b, err = s.PauseStageBudget(cycleID, "qa", b.Generation, StageBudgetWaiting, "human_question", pausedAt)
	if err != nil {
		t.Fatal(err)
	}
	if b.Consumed != 5*time.Minute || b.State != StageBudgetWaiting {
		t.Fatalf("paused budget = %+v", b)
	}
	restartAt := pausedAt.Add(24 * time.Hour)
	b, err = s.ResumeStageBudget(cycleID, "qa", b.Generation, restartAt, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := b.RemainingAt(restartAt); got != 5*time.Minute {
		t.Fatalf("offline/human wait charged: remaining=%s want 5m", got)
	}
	checkpointAt := restartAt.Add(2 * time.Minute)
	b, err = s.CheckpointStageBudget(cycleID, "qa", b.Generation, checkpointAt)
	if err != nil {
		t.Fatal(err)
	}
	recoveredAt := checkpointAt.Add(12 * time.Hour)
	interrupted, err := s.RecoverActiveStageBudgets(recoveredAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(interrupted) != 1 {
		t.Fatalf("recovered %d budgets, want 1", len(interrupted))
	}
	got := interrupted[0]
	if got.State != StageBudgetInterrupted || got.Consumed != 7*time.Minute || got.RemainingAt(recoveredAt) != 3*time.Minute {
		t.Fatalf("recovered budget counted downtime or lost checkpoint: %+v", got)
	}
	stage, err := s.GetStage(cycleID, "qa")
	if err != nil {
		t.Fatal(err)
	}
	if stage.Status != StageEscalated {
		t.Fatalf("restart stage status=%s want Escalated", stage.Status)
	}
}

func TestActiveBudgetExpiryRevokesGenerationAndIncreaseIsExplicit(t *testing.T) {
	s, cycleID, start := openStageBudgetStore(t)
	b, err := s.BeginStageBudget(cycleID, "browser_ui_validation", time.Minute, start)
	if err != nil {
		t.Fatal(err)
	}
	accepted, expired, err := s.AcceptStageBudget(cycleID, "browser_ui_validation", b.Generation, start.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if accepted || expired.State != StageBudgetExpired || expired.Generation == b.Generation {
		t.Fatalf("acceptance crossed deadline: accepted=%v budget=%+v", accepted, expired)
	}
	if _, _, err := s.AcceptStageBudget(cycleID, "browser_ui_validation", b.Generation, start.Add(time.Minute)); !errors.Is(err, ErrStageBudgetGeneration) {
		t.Fatalf("old generation error=%v, want stale generation", err)
	}
	if _, err := s.IncreaseStageBudget(cycleID, "browser_ui_validation", 2*time.Minute, start.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	increased, err := s.GetStageBudget(cycleID, "browser_ui_validation")
	if err != nil {
		t.Fatal(err)
	}
	if increased.Limit != 3*time.Minute || increased.Consumed != time.Minute || increased.RemainingAt(start.Add(time.Minute)) != 2*time.Minute {
		t.Fatalf("increase reset/changed elapsed time: %+v", increased)
	}
	if _, err := s.IncreaseStageBudget(cycleID, "browser_ui_validation", time.Minute, start.Add(time.Minute)); err == nil {
		t.Fatal("second increase with positive time remaining should be rejected")
	}
}

func TestActiveBudgetAcceptanceExpiryRaceIsSerialized(t *testing.T) {
	s, cycleID, start := openStageBudgetStore(t)
	b, err := s.BeginStageBudget(cycleID, "browser_ui_validation", time.Minute, start)
	if err != nil {
		t.Fatal(err)
	}
	deadline := start.Add(time.Minute)
	startRace := make(chan struct{})
	var wg sync.WaitGroup
	var accepted bool
	var acceptErr error
	var expired bool
	var expireErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-startRace
		accepted, _, acceptErr = s.AcceptStageBudget(cycleID, "browser_ui_validation", b.Generation, deadline)
	}()
	go func() {
		defer wg.Done()
		<-startRace
		_, expired, expireErr = s.ExpireStageBudget(cycleID, "browser_ui_validation", b.Generation, deadline)
	}()
	close(startRace)
	wg.Wait()
	if accepted {
		t.Fatal("completion won at the exact zero-balance deadline")
	}
	final, err := s.GetStageBudget(cycleID, "browser_ui_validation")
	if err != nil {
		t.Fatal(err)
	}
	if final.State != StageBudgetExpired || final.Generation <= b.Generation {
		t.Fatalf("expiry failed to revoke acceptance under race: %+v", final)
	}
	if acceptErr != nil && !errors.Is(acceptErr, ErrStageBudgetGeneration) {
		t.Fatalf("unexpected acceptance race error: %v", acceptErr)
	}
	if expireErr == nil && !expired {
		t.Fatal("expiry race returned success without an expired budget")
	}
	if expireErr != nil && !errors.Is(expireErr, ErrStageBudgetGeneration) {
		t.Fatalf("unexpected expiry race error: %v", expireErr)
	}
}

func TestPauseBlockedTxRevokesAcceptanceOrCommitsExpiry(t *testing.T) {
	s, cycleID, start := openStageBudgetStore(t)
	b, err := s.BeginStageBudget(cycleID, "implementation", 5*time.Minute, start)
	if err != nil {
		t.Fatal(err)
	}
	var blocked StageBudget
	err = s.InTx(func(tx *sql.Tx) error {
		var txErr error
		blocked, txErr = PauseStageBudgetBlockedTx(tx, cycleID, "implementation", start.Add(time.Minute), "coverage_missing")
		return txErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if blocked.State != StageBudgetBlocked || blocked.Generation == b.Generation || blocked.Consumed != time.Minute {
		t.Fatalf("blocked transition failed to checkpoint/revoke: %+v", blocked)
	}
	if _, _, err := s.AcceptStageBudget(cycleID, "implementation", b.Generation, start.Add(2*time.Minute)); !errors.Is(err, ErrStageBudgetGeneration) {
		t.Fatalf("stale result accepted after block: %v", err)
	}
}
