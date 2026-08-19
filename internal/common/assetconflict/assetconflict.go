// Package assetconflict resolves Hero asset upgrade conflicts by backing up
// locally customized files before replacing them with updated content.
package assetconflict

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/output"
)

// SHA256Hex returns the SHA256 hex digest of data.
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// IsCustomized reports whether existingData differs from the originally installed checksum.
func IsCustomized(existingData []byte, originalChecksum string) bool {
	if originalChecksum == "" {
		return false
	}
	return SHA256Hex(existingData) != originalChecksum
}

// BackupPath returns a sibling backup path for a conflicting file.
// Pattern: {filename}_{timestamp}.conflict (e.g. backend_agent.md_20260819_154100.conflict).
func BackupPath(filePath string, now time.Time) string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)
	ts := now.UTC().Format("20060102_150405")
	return filepath.Join(dir, fmt.Sprintf("%s_%s.conflict", base, ts))
}

// Replace writes newData to dstPath after saving existingData to a timestamped .conflict backup.
// relKey is shown in the warning (typically path relative to the project root).
func Replace(dstPath string, existingData, newData []byte, relKey string, stderr io.Writer, now time.Time) (backupPath string, err error) {
	backupPath = BackupPath(dstPath, now)
	if err := os.WriteFile(backupPath, existingData, 0o644); err != nil {
		return "", fmt.Errorf("backup conflict copy %s: %w", backupPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}
	if err := os.WriteFile(dstPath, newData, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", dstPath, err)
	}
	output.Warningf(stderr, "%s was replaced with the new file due to conflicts (backup: %s).", relKey, backupPath)
	return backupPath, nil
}
