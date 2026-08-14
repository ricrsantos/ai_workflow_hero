package cycle_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/spf13/cobra"
)

func finishCycleForArchive(t *testing.T, svc *cycle.Service) {
	t.Helper()
	if err := svc.StartStage("research"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseStage("research", "done", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Finish(""); err != nil {
		t.Fatal(err)
	}
}

func mkdirOpenspecChange(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, "openspec", "changes", name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveWithOptionsZeroOpenspecChanges(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	var openspecCalled bool
	svc.OpenspecRunner = func(ctx context.Context, name string) error {
		openspecCalled = true
		return nil
	}

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	res, err := svc.ArchiveWithOptions(cycle.ArchiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if openspecCalled {
		t.Fatal("expected no openspec call when no active changes")
	}
	if res.CycleNumber != 1 || res.ArchiveDir == "" {
		t.Fatalf("archive result: %+v", res)
	}
}

func TestArchiveWithOptionsStoredNameRunsOpenspec(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	var gotName string
	svc.OpenspecRunner = func(ctx context.Context, name string) error {
		gotName = name
		return nil
	}

	mkdirOpenspecChange(t, dir, "slash-parity-tui-harness")

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetOpenspecChange("slash-parity-tui-harness"); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	res, err := svc.ArchiveWithOptions(cycle.ArchiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "slash-parity-tui-harness" {
		t.Fatalf("openspec name=%q", gotName)
	}
	if res.OpenspecChange != "slash-parity-tui-harness" {
		t.Fatalf("result openspec change=%q", res.OpenspecChange)
	}
}

func TestArchiveWithOptionsSingleActiveChangeAutoDetect(t *testing.T) {
	dir := setupProject(t)
	mkdirOpenspecChange(t, dir, "hero-1-0")

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	var gotName string
	svc.OpenspecRunner = func(ctx context.Context, name string) error {
		gotName = name
		return nil
	}

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	if _, err := svc.ArchiveWithOptions(cycle.ArchiveOptions{}); err != nil {
		t.Fatal(err)
	}
	if gotName != "hero-1-0" {
		t.Fatalf("auto-detected name=%q want hero-1-0", gotName)
	}
}

func TestArchiveWithOptionsMultipleChangesFailsClosed(t *testing.T) {
	dir := setupProject(t)
	mkdirOpenspecChange(t, dir, "change-a")
	mkdirOpenspecChange(t, dir, "change-b")

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	svc.OpenspecRunner = func(ctx context.Context, name string) error {
		t.Fatal("openspec should not run when name is ambiguous")
		return nil
	}

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	_, err = svc.ArchiveWithOptions(cycle.ArchiveOptions{})
	if !errors.Is(err, cycle.ErrMultipleOpenspecChanges) {
		t.Fatalf("err=%v want ErrMultipleOpenspecChanges", err)
	}

	cycles, err := svc.Store.ListCycles()
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) == 0 || cycles[len(cycles)-1].Status == store.CycleStatusArchived {
		t.Fatal("cycle should remain unarchived")
	}
}

func TestArchiveWithOptionsOverrideMultipleChanges(t *testing.T) {
	dir := setupProject(t)
	mkdirOpenspecChange(t, dir, "change-a")
	mkdirOpenspecChange(t, dir, "change-b")

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	var gotName string
	svc.OpenspecRunner = func(ctx context.Context, name string) error {
		gotName = name
		return nil
	}

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	if _, err := svc.ArchiveWithOptions(cycle.ArchiveOptions{OpenspecChange: "change-b"}); err != nil {
		t.Fatal(err)
	}
	if gotName != "change-b" {
		t.Fatalf("override name=%q", gotName)
	}
}

func TestArchiveWithOptionsOpenspecFailureBlocksHero(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	svc.OpenspecRunner = func(ctx context.Context, name string) error {
		return errors.New("exit status 1")
	}

	mkdirOpenspecChange(t, dir, "my-change")

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetOpenspecChange("my-change"); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	_, err = svc.ArchiveWithOptions(cycle.ArchiveOptions{})
	if !errors.Is(err, cycle.ErrOpenspecArchiveFailed) {
		t.Fatalf("err=%v want ErrOpenspecArchiveFailed", err)
	}
	if !strings.Contains(err.Error(), cycle.ManualOpenspecArchiveCommand("my-change")) {
		t.Fatalf("missing manual command in err: %v", err)
	}

	cycles, err := svc.Store.ListCycles()
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) == 0 || cycles[len(cycles)-1].Status == store.CycleStatusArchived {
		t.Fatal("cycle should remain unarchived after openspec failure")
	}
}

func TestArchiveWithOptionsForceAfterOpenspecFailure(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	svc.OpenspecRunner = func(ctx context.Context, name string) error {
		return errors.New("boom")
	}

	mkdirOpenspecChange(t, dir, "forced-change")

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetOpenspecChange("forced-change"); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	res, err := svc.ArchiveWithOptions(cycle.ArchiveOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OpenspecForced {
		t.Fatal("expected OpenspecForced")
	}
	if res.ArchiveDir == "" {
		t.Fatal("expected hero archive dir")
	}

	cycles, err := svc.Store.ListCycles()
	if err != nil {
		t.Fatal(err)
	}
	if cycles[len(cycles)-1].Status != store.CycleStatusArchived {
		t.Fatal("cycle should be archived after force")
	}
}

func TestArchiveWithOptionsSkipOpenspecAlias(t *testing.T) {
	dir := setupProject(t)
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	svc.OpenspecRunner = func(ctx context.Context, name string) error {
		return errors.New("boom")
	}

	mkdirOpenspecChange(t, dir, "skip-me")

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetOpenspecChange("skip-me"); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	res, err := svc.ArchiveWithOptions(cycle.ArchiveOptions{SkipOpenspec: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OpenspecForced {
		t.Fatal("expected OpenspecForced with --skip-openspec alias")
	}
}

func TestDefaultOpenspecRunnerUsesLookPathAndExec(t *testing.T) {
	dir := setupProject(t)
	var looked string
	var ran []string

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	svc.OpenspecExec = cycle.OpenspecExec{
		LookPath: func(name string) (string, error) {
			looked = name
			return "/fake/openspec", nil
		},
		Run: func(ctx context.Context, binary string, args ...string) error {
			ran = append([]string{binary}, args...)
			return nil
		},
	}

	mkdirOpenspecChange(t, dir, "wired")

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetOpenspecChange("wired"); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	if _, err := svc.ArchiveWithOptions(cycle.ArchiveOptions{}); err != nil {
		t.Fatal(err)
	}
	if looked != "openspec" {
		t.Fatalf("LookPath=%q want openspec", looked)
	}
	if len(ran) != 4 || ran[0] != "/fake/openspec" || ran[1] != "archive" || ran[2] != "wired" || ran[3] != "-y" {
		t.Fatalf("run args=%v", ran)
	}
}

func TestCycleArchiveCommandFlagsDocumented(t *testing.T) {
	var archiveCmd *cobra.Command
	for _, cmd := range cycle.NewCommands() {
		if cmd.Use == "cycle" {
			for _, sub := range cmd.Commands() {
				if sub.Use == "archive" {
					archiveCmd = sub
					break
				}
			}
		}
	}
	if archiveCmd == nil {
		t.Fatal("cycle archive command not found")
	}
	for _, name := range []string{"force", "skip-openspec", "openspec-change"} {
		if archiveCmd.Flags().Lookup(name) == nil {
			t.Fatalf("missing flag %q", name)
		}
	}
}

func TestListActiveOpenspecChangesIgnoresArchiveDir(t *testing.T) {
	dir := setupProject(t)
	mkdirOpenspecChange(t, dir, "live")
	if err := os.MkdirAll(filepath.Join(dir, "openspec", "changes", "archive", "old"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	var gotName string
	svc.OpenspecRunner = func(ctx context.Context, name string) error {
		gotName = name
		return nil
	}

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	if _, err := svc.ArchiveWithOptions(cycle.ArchiveOptions{}); err != nil {
		t.Fatal(err)
	}
	if gotName != "live" {
		t.Fatalf("name=%q want live", gotName)
	}
}

func TestArchiveSkipsOpenspecWhenStoredChangeAlreadyArchived(t *testing.T) {
	dir := setupProject(t)
	if err := os.MkdirAll(filepath.Join(dir, "openspec", "changes", "archive", "2026-08-13-kanban-task-manager-mvp"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	var openspecCalled bool
	svc.OpenspecRunner = func(ctx context.Context, name string) error {
		openspecCalled = true
		return errors.New("openspec binary not found on PATH")
	}

	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetOpenspecChange("kanban-task-manager-mvp"); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	res, err := svc.ArchiveWithOptions(cycle.ArchiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if openspecCalled {
		t.Fatal("openspec CLI must be skipped when the change dir is already archived")
	}
	if res.ArchiveDir == "" {
		t.Fatal("expected Hero archive to proceed")
	}
	if res.OpenspecForced {
		t.Fatal("skip of already-archived change is not a force path")
	}
}

func TestDefaultOpenspecRunnerFindsBinaryOutsidePATH(t *testing.T) {
	dir := setupProject(t)
	binDir := t.TempDir()
	fake := filepath.Join(binDir, "openspec")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var ran []string
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	svc.OpenspecExec = cycle.OpenspecExec{
		LookPath: func(string) (string, error) {
			return "", exec.ErrNotFound
		},
		ExtraDirs: func() []string { return []string{binDir} },
		Run: func(ctx context.Context, binary string, args ...string) error {
			ran = append([]string{binary}, args...)
			return nil
		},
	}

	mkdirOpenspecChange(t, dir, "from-nvm")
	if _, err := svc.NewCycle("", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetOpenspecChange("from-nvm"); err != nil {
		t.Fatal(err)
	}
	finishCycleForArchive(t, svc)

	if _, err := svc.ArchiveWithOptions(cycle.ArchiveOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 4 || ran[0] != fake || ran[2] != "from-nvm" {
		t.Fatalf("run args=%v want binary %s archive from-nvm -y", ran, fake)
	}
}
