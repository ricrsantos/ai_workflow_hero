package cycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ricrsantos/ai_workflow_hero/internal/ideadocs"
)

const ideaReadme = "README.md"

type ideaArchiveMove struct {
	source      string
	destination string
}

type ideaArchivePlan struct {
	moves []ideaArchiveMove
}

// prepareIdeaArchive validates the active idea entries before any of them are
// moved. The plan keeps archive retries safe when a destination already exists.
func prepareIdeaArchive(projectDir string) (ideaArchivePlan, error) {
	ideaRoot := filepath.Join(projectDir, filepath.FromSlash(ideadocs.DirName))
	entries, err := os.ReadDir(ideaRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ideaArchivePlan{}, nil
		}
		return ideaArchivePlan{}, fmt.Errorf("list idea notes: %w", err)
	}

	archiveRoot := filepath.Join(ideaRoot, ideadocs.ExcludeArchive)
	plan := ideaArchivePlan{}
	for _, entry := range entries {
		switch entry.Name() {
		case ideadocs.ExcludeArchive, ideadocs.ExcludeTobe, ideaReadme:
			continue
		}
		plan.moves = append(plan.moves, ideaArchiveMove{
			source:      filepath.Join(ideaRoot, entry.Name()),
			destination: filepath.Join(archiveRoot, entry.Name()),
		})
	}
	if len(plan.moves) == 0 {
		return plan, nil
	}

	if err := os.MkdirAll(archiveRoot, 0o755); err != nil {
		return ideaArchivePlan{}, fmt.Errorf("create idea archive directory: %w", err)
	}
	for _, move := range plan.moves {
		if _, err := os.Lstat(move.destination); err == nil {
			return ideaArchivePlan{}, fmt.Errorf("archive idea notes: destination already exists: %s", move.destination)
		} else if !errors.Is(err, os.ErrNotExist) {
			return ideaArchivePlan{}, fmt.Errorf("check idea archive destination %s: %w", move.destination, err)
		}
	}
	return plan, nil
}

func (p ideaArchivePlan) apply() error {
	for i, move := range p.moves {
		if err := os.Rename(move.source, move.destination); err != nil {
			moveErr := fmt.Errorf("move idea entry %s to archive: %w", move.source, err)
			if rollbackErr := rollbackIdeaMoves(p.moves[:i]); rollbackErr != nil {
				return errors.Join(moveErr, fmt.Errorf("restore moved idea entries: %w", rollbackErr))
			}
			return moveErr
		}
	}
	return nil
}

func (p ideaArchivePlan) rollback() error {
	return rollbackIdeaMoves(p.moves)
}

func rollbackIdeaMoves(moves []ideaArchiveMove) error {
	var rollbackErr error
	for i := len(moves) - 1; i >= 0; i-- {
		move := moves[i]
		if _, err := os.Lstat(move.destination); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("inspect moved idea entry %s: %w", move.destination, err))
			continue
		}
		if _, err := os.Lstat(move.source); err == nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore destination %s: source already exists", move.destination))
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("check idea source %s: %w", move.source, err))
			continue
		}
		if err := os.Rename(move.destination, move.source); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore idea entry %s: %w", move.destination, err))
		}
	}
	return rollbackErr
}
