package testaccess

import (
	"context"
	"errors"
	"os"
	"syscall"
)

// RemoveForArchive removes only project-root .env.hero under the existing
// cooperative ownership lease. Callers must first complete archive prerequisites.
// It never reads values or creates a recovery copy of credentials.
func (s *SafeStore) RemoveForArchive(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOwnership(); err != nil {
		return err
	}
	info, err := s.root.Lstat(credentialFile)
	if errors.Is(err, os.ErrNotExist) {
		if err := s.syncArchiveCleanup(); err != nil {
			return err
		}
		s.log.Debug("archive credential cleanup already complete")
		return nil
	}
	if err != nil || !safeOwnedFile(info) {
		return errors.New("archive pending: unsafe project-root .env.hero; correct its owner, permissions and path, then run /hero-archive again")
	}
	if err := s.gitCheck(ctx, false); err != nil {
		return err
	}
	file, err := s.root.OpenFile(credentialFile, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return errors.New("archive pending: cannot safely open project-root .env.hero; correct file access and run /hero-archive again")
	}
	held, statErr := file.Stat()
	closeErr := file.Close()
	if statErr != nil || closeErr != nil || !os.SameFile(info, held) || !safeOwnedFile(held) {
		return errors.New("archive pending: credential file ownership changed; retry /hero-archive after correcting local access")
	}
	current, err := s.root.Lstat(credentialFile)
	if err != nil || !safeOwnedFile(current) || !os.SameFile(info, current) {
		return errors.New("archive pending: credential file changed; retry /hero-archive after correcting local access")
	}
	if err := ctx.Err(); err != nil {
		return errors.New("archive pending: credential cleanup interrupted; run /hero-archive again")
	}
	if err := s.root.Remove(credentialFile); err != nil {
		s.log.Error("archive credential removal failed")
		return errors.New("archive pending: could not remove project-root .env.hero; resolve file access and run /hero-archive again")
	}
	if err := s.syncArchiveCleanup(); err != nil {
		return err
	}
	s.log.Info("archive project-root credentials removed")
	return nil
}

func (s *SafeStore) syncArchiveCleanup() error {
	dir, err := s.root.Open(".")
	if err != nil {
		return errors.New("archive pending: cannot confirm credential cleanup; correct root access and run /hero-archive again")
	}
	syncErr := dir.Sync()
	closeErr := dir.Close()
	if syncErr != nil || closeErr != nil {
		s.log.Error("archive credential cleanup durability check failed")
		return errors.New("archive pending: could not confirm credential cleanup; run /hero-archive again")
	}
	return nil
}
