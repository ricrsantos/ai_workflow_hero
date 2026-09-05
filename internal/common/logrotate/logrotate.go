// Package logrotate provides a size-bounded rotating file writer for Hero
// project and daemon logs (ADR-064; PRD-C09-001 §3.5). The active file rotates
// after MaxSize bytes and at most MaxFiles files (including the active one) are
// retained. Every write is passed through the shared redaction helper so
// credentials never reach disk.
package logrotate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
)

const (
	// DefaultMaxSize is the rotation threshold (10 MB).
	DefaultMaxSize = 10 * 1024 * 1024
	// DefaultMaxFiles is the total number of files retained, active included.
	DefaultMaxFiles = 10
)

// Writer is a rotating, redacting file writer. It is safe for concurrent use.
type Writer struct {
	// Filename is the active log path (e.g. ".workflow-hero/logs/tui.log").
	Filename string
	// MaxSize is the rotation threshold in bytes.
	MaxSize int64
	// MaxFiles is the total files retained, active included (backups + active).
	MaxFiles int
	// Redactor transforms each write before it reaches disk. When nil, no
	// redaction is applied.
	Redactor func(string) string

	mu   sync.Mutex
	file *os.File
	size int64

	open   func(name string, flag int, perm os.FileMode) (*os.File, error)
	rename func(oldpath, newpath string) error
	remove func(name string) error
}

// New returns a Writer for filename with the default 10 MB × 10 file policy and
// the shared token redaction applied.
func New(filename string) *Writer {
	return &Writer{
		Filename: filename,
		MaxSize:  DefaultMaxSize,
		MaxFiles: DefaultMaxFiles,
		Redactor: func(s string) string { return redact.Redact(s) },
		open:     os.OpenFile,
		rename:   os.Rename,
		remove:   os.Remove,
	}
}

func (w *Writer) maxSize() int64 {
	if w.MaxSize <= 0 {
		return DefaultMaxSize
	}
	return w.MaxSize
}

func (w *Writer) maxFiles() int {
	if w.MaxFiles <= 1 {
		return DefaultMaxFiles
	}
	return w.MaxFiles
}

// Write appends p to the active file, rotating first when the write would push
// the file over MaxSize. It reports the full length of p as consumed even when
// redaction changes the on-disk byte count.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.Filename == "" {
		return 0, fmt.Errorf("logrotate: empty filename")
	}
	if w.file == nil {
		if err := w.openActive(); err != nil {
			return 0, err
		}
	}

	data := p
	if w.Redactor != nil {
		data = []byte(w.Redactor(string(p)))
	}

	if w.size > 0 && w.size+int64(len(data)) > w.maxSize() {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	if _, err := w.file.Write(data); err != nil {
		return 0, err
	}
	w.size += int64(len(data))
	return len(p), nil
}

// Close closes the active file, if any.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	w.size = 0
	return err
}

func (w *Writer) openActive() error {
	if err := os.MkdirAll(filepath.Dir(w.Filename), 0o755); err != nil {
		return fmt.Errorf("logrotate: mkdir: %w", err)
	}
	f, err := w.open(w.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("logrotate: open: %w", err)
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("logrotate: stat: %w", err)
	}
	w.file = f
	w.size = fi.Size()
	return nil
}

// rotate closes the active file, shifts backups, and opens a fresh active file.
// Backups are named <file>.1 .. <file>.<MaxFiles-1>.
func (w *Writer) rotate() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
		w.size = 0
	}

	// Drop the oldest backup.
	_ = w.remove(w.Filename + "." + strconv.Itoa(w.maxFiles()-1))

	// Shift .(i-1) -> .i for i in [MaxFiles-1 .. 2].
	for i := w.maxFiles() - 1; i >= 2; i-- {
		src := w.Filename + "." + strconv.Itoa(i-1)
		dst := w.Filename + "." + strconv.Itoa(i)
		if err := w.rename(src, dst); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("logrotate: rename: %w", err)
		}
	}
	// Rotate the active file into .1.
	if err := w.rename(w.Filename, w.Filename+".1"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("logrotate: rename active: %w", err)
	}
	return w.openActive()
}

// MigrateLegacy moves a legacy single-file log to newPath once, when the legacy
// file exists and newPath does not (PRD-C09-001 §3.5 AC7). It returns nil when
// there is nothing to migrate.
func MigrateLegacy(legacyPath, newPath string) error {
	if _, err := os.Stat(legacyPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("logrotate: stat legacy: %w", err)
	}
	if _, err := os.Stat(newPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("logrotate: stat new: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return fmt.Errorf("logrotate: mkdir: %w", err)
	}
	if err := os.Rename(legacyPath, newPath); err != nil {
		return fmt.Errorf("logrotate: migrate legacy: %w", err)
	}
	return nil
}
