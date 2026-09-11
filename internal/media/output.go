package media

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// MaterializeAsset copies a harness-produced image into this session's
// private asset store and replaces its path/metadata with the immutable
// session reference. Metadata-only failure assets are returned unchanged.
func (s *Store) MaterializeAsset(ctx context.Context, asset harness.Asset) (harness.Asset, error) {
	if s == nil {
		return harness.Asset{}, errors.New("materializing output asset: store is nil")
	}
	if strings.TrimSpace(asset.Path) == "" {
		return asset, nil
	}
	attachment, err := s.Materialize(ctx, MaterializeRequest{
		SourcePath:        asset.Path,
		OriginalName:      asset.Name,
		TurnID:            asset.TurnID,
		Origin:            string(asset.Source),
		Workspace:         s.workspace,
		AllowExternalPath: true,
	})
	if err != nil {
		return harness.Asset{}, err
	}
	asset.Attachment = attachment
	if asset.SessionID == "" {
		asset.SessionID = s.sessionID
	}
	asset.Saved = true
	return asset, nil
}

// MaterializeBytes stores provider output bytes through the same validation,
// deduplication, manifest, and permission path as file-backed assets. The
// temporary source is private and removed before the method returns.
func (s *Store) MaterializeBytes(ctx context.Context, data []byte, name, turnID, origin string) (harness.Attachment, error) {
	if s == nil {
		return harness.Attachment{}, errors.New("materializing output bytes: store is nil")
	}
	file, err := os.CreateTemp("", "hero-output-*")
	if err != nil {
		return harness.Attachment{}, err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return harness.Attachment{}, err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return harness.Attachment{}, err
	}
	if err := file.Close(); err != nil {
		return harness.Attachment{}, err
	}
	return s.Materialize(ctx, MaterializeRequest{
		SourcePath:        path,
		OriginalName:      name,
		TurnID:            turnID,
		Origin:            origin,
		Workspace:         s.workspace,
		AllowExternalPath: true,
	})
}
