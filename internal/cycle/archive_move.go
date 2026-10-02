package cycle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

func (s *Service) verifyArchiveProjectOwnership() error {
	if s.Store == nil {
		return errors.New("archive pending: project store is unavailable")
	}
	var dbPath string
	if err := s.Store.DB().QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&dbPath); err != nil {
		return errors.New("archive pending: cannot verify project store ownership")
	}
	actual, err := filepath.EvalSymlinks(dbPath)
	if err != nil {
		return errors.New("archive pending: cannot verify project store ownership")
	}
	expected, err := filepath.EvalSymlinks(filepath.Join(s.ProjectDir, store.RelativeDBPath))
	if err != nil || expected != actual {
		return errors.New("archive pending: cycle store belongs to another project; open the owning project before retrying")
	}
	return nil
}

// Identity survives rename and distinguishes our interrupted move from an
// unrelated destination collision. No credential content enters this intent.
type archiveMoveIdentity struct {
	Destination string `json:"destination"`
	Device      uint64 `json:"device"`
	Inode       uint64 `json:"inode"`
}

func archiveMovePayload(destination string, info os.FileInfo) (string, error) {
	if info == nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("unsafe archive move directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("archive directory identity unavailable")
	}
	data, err := json.Marshal(archiveMoveIdentity{destination, uint64(stat.Dev), uint64(stat.Ino)})
	return string(data), err
}

func (s *Service) recordArchiveMove(cycleID int64, destination string, info os.FileInfo) error {
	payload, err := archiveMovePayload(destination, info)
	if err != nil {
		return err
	}
	_, err = s.Store.AppendEvent(store.Event{CycleID: cycleID, Type: "archive_move_intent", PayloadJSON: payload})
	return err
}

func (s *Service) matchesArchiveMove(cycleID int64, destination string, info os.FileInfo) (bool, error) {
	payload, err := archiveMovePayload(destination, info)
	if err != nil {
		return false, err
	}
	var exists bool
	err = s.Store.DB().QueryRow(`SELECT EXISTS(SELECT 1 FROM events WHERE cycle_id = ? AND type = 'archive_move_intent' AND payload_json = ?)`, cycleID, payload).Scan(&exists)
	return exists, err
}
