// Package media owns validation and session-scoped storage for image assets.
//
// The package stores only immutable references in callers: image bytes stay in
// a private session directory and are never part of a harness or TUI model.
package media

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const (
	// DefaultMaxAttachmentsPerTurn is the maximum number of image attachments
	// accepted for one turn unless a caller supplies a stricter limit.
	DefaultMaxAttachmentsPerTurn = 5
	// DefaultMaxFileBytes is the maximum encoded image size accepted by Hero.
	DefaultMaxFileBytes int64 = 20 * 1024 * 1024
	// DefaultMaxWidth and DefaultMaxHeight bound decoded image dimensions.
	DefaultMaxWidth  = 4096
	DefaultMaxHeight = 4096
	// DefaultMaxPixelsPerTurn bounds decoded pixels for one turn.
	DefaultMaxPixelsPerTurn int64 = 100 * 1000 * 1000
	// DefaultRetention is the lifetime of an inactive session directory.
	DefaultRetention = 7 * 24 * time.Hour
)

const manifestVersion = 1

var (
	// ErrUnsupportedFormat indicates that the content is not one of the
	// supported PNG, JPEG, GIF, or WebP image formats.
	ErrUnsupportedFormat = errors.New("unsupported image format")
	// ErrInvalidImage indicates a recognized format with an invalid header.
	ErrInvalidImage = errors.New("invalid image")
	// ErrFileTooLarge indicates that the encoded image exceeds its byte limit.
	ErrFileTooLarge = errors.New("image file exceeds the maximum size")
	// ErrDimensionsExceeded indicates that one image exceeds its dimensions.
	ErrDimensionsExceeded = errors.New("image dimensions exceed the maximum")
	// ErrPixelLimitExceeded indicates that one image exceeds its pixel budget.
	ErrPixelLimitExceeded = errors.New("image pixels exceed the maximum")
	// ErrTooManyAttachments indicates that a turn exceeds its attachment count.
	ErrTooManyAttachments = errors.New("too many image attachments for the turn")
	// ErrTurnPixelLimitExceeded indicates that a turn exceeds its total pixel
	// budget.
	ErrTurnPixelLimitExceeded = errors.New("turn image pixels exceed the maximum")
	// ErrExternalPath indicates that a caller has not granted permission for a
	// path outside the configured workspace.
	ErrExternalPath = errors.New("external image path requires permission")
	// ErrInvalidSessionID indicates a session id that cannot be used as a
	// directory component.
	ErrInvalidSessionID = errors.New("invalid media session id")
	// ErrInvalidDataHome indicates a relative XDG data home value.
	ErrInvalidDataHome = errors.New("XDG data home must be an absolute path")
	// ErrSourceChanged indicates that the source changed between validation and
	// materialization.
	ErrSourceChanged = errors.New("image source changed during materialization")
)

// Limits contains the common image and turn limits. Zero values are replaced
// with the corresponding secure default.
type Limits struct {
	MaxAttachmentsPerTurn int
	MaxFileBytes          int64
	MaxWidth              int
	MaxHeight             int
	MaxPixelsPerTurn      int64
}

// DefaultLimits returns the limits defined by PRD-C14-001.
func DefaultLimits() Limits {
	return Limits{
		MaxAttachmentsPerTurn: DefaultMaxAttachmentsPerTurn,
		MaxFileBytes:          DefaultMaxFileBytes,
		MaxWidth:              DefaultMaxWidth,
		MaxHeight:             DefaultMaxHeight,
		MaxPixelsPerTurn:      DefaultMaxPixelsPerTurn,
	}
}

func normalizeLimits(limits Limits) Limits {
	defaults := DefaultLimits()
	if limits.MaxAttachmentsPerTurn <= 0 {
		limits.MaxAttachmentsPerTurn = defaults.MaxAttachmentsPerTurn
	}
	if limits.MaxFileBytes <= 0 {
		limits.MaxFileBytes = defaults.MaxFileBytes
	}
	if limits.MaxWidth <= 0 {
		limits.MaxWidth = defaults.MaxWidth
	}
	if limits.MaxHeight <= 0 {
		limits.MaxHeight = defaults.MaxHeight
	}
	if limits.MaxPixelsPerTurn <= 0 {
		limits.MaxPixelsPerTurn = defaults.MaxPixelsPerTurn
	}
	return limits
}

// ValidationOptions controls path and image validation. The declared MIME
// type of an attachment is deliberately not part of these options: the
// detected file header is authoritative.
type ValidationOptions struct {
	Limits
	Workspace         string
	AllowExternalPath bool
}

func (o ValidationOptions) normalized() ValidationOptions {
	o.Limits = normalizeLimits(o.Limits)
	return o
}

// MaterializeRequest describes one source image. OriginalName is metadata
// only and is sanitized before it is placed in the manifest. SourcePath and
// Path are aliases; SourcePath takes precedence.
type MaterializeRequest struct {
	SourcePath   string
	Path         string
	OriginalName string
	Name         string
	MIMEType     string
	TurnID       string
	Origin       string
	Workspace    string
	// AllowExternalPath must be true (or the store option must be true) when
	// the canonical source path is outside Workspace.
	AllowExternalPath bool
}

func (r MaterializeRequest) sourcePath() string {
	if strings.TrimSpace(r.SourcePath) != "" {
		return r.SourcePath
	}
	return r.Path
}

func (r MaterializeRequest) originalName() string {
	if strings.TrimSpace(r.OriginalName) != "" {
		return r.OriginalName
	}
	return r.Name
}

// Manifest is the private on-disk index for one session. It is intentionally
// separate from SQLite so image bytes and their paths remain session-scoped.
type Manifest struct {
	Version   int             `json:"version"`
	SessionID string          `json:"session_id"`
	Assets    []ManifestEntry `json:"assets"`
	// Entries is an in-memory compatibility view of Assets. It is omitted from
	// the JSON document so the manifest has one canonical collection.
	Entries []ManifestEntry `json:"-"`
}

// ManifestEntry correlates a generated asset id with its session, turn,
// origin, detected MIME type, content hash, and materialized path.
type ManifestEntry struct {
	AssetID   string    `json:"asset_id"`
	SessionID string    `json:"session_id"`
	TurnID    string    `json:"turn_id,omitempty"`
	Origin    string    `json:"origin"`
	MIMEType  string    `json:"mime_type"`
	Path      string    `json:"path"`
	Name      string    `json:"name,omitempty"`
	Size      int64     `json:"size"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	SHA256    string    `json:"sha256"`
	CreatedAt time.Time `json:"created_at"`
}

// StoreOptions configures a session asset store. An empty DataHome honors
// XDG_DATA_HOME and otherwise falls back to ~/.local/share.
type StoreOptions struct {
	DataHome           string
	SessionID          string
	Workspace          string
	AllowExternalPaths bool
	Limits             Limits
	Retention          time.Duration
	Now                func() time.Time
	Logger             *slog.Logger
	// RegisteredSessionIDs protects durable Hero session dirs during cleanup.
	RegisteredSessionIDs RegisteredSessionIDs
	ListRegistered       func() (RegisteredSessionIDs, error)
}

// Store materializes validated images for one session. A Store is safe for
// concurrent callers in one process; manifest updates are serialized and
// written atomically.
type Store struct {
	mu sync.Mutex

	dataHome             string
	sessionsRoot         string
	sessionDir           string
	assetsDir            string
	manifestPath         string
	sessionID            string
	workspace            string
	allowExternalPaths   bool
	limits               Limits
	retention            time.Duration
	now                  func() time.Time
	logger               *slog.Logger
	registeredSessionIDs RegisteredSessionIDs
	listRegistered       func() (RegisteredSessionIDs, error)
	manifest             Manifest
}

// NewStore creates a session store. The variadic options preserve a concise
// NewStore("session-id") call while allowing callers to configure XDG data
// home and policy explicitly.
func NewStore(sessionID string, options ...StoreOptions) (*Store, error) {
	if len(options) > 1 {
		return nil, fmt.Errorf("creating media store: expected at most one options value")
	}
	var opts StoreOptions
	if len(options) == 1 {
		opts = options[0]
	}
	if strings.TrimSpace(opts.SessionID) == "" {
		opts.SessionID = sessionID
	}
	return New(opts)
}

// New creates a session store from options. SessionID is required and is
// treated strictly as one directory component to prevent path traversal.
func New(options StoreOptions) (*Store, error) {
	sessionID, err := cleanSessionID(options.SessionID)
	if err != nil {
		return nil, err
	}
	dataHome, err := resolveDataHome(options.DataHome)
	if err != nil {
		return nil, err
	}
	retention := options.Retention
	if retention == 0 {
		retention = DefaultRetention
	}
	if retention < 0 {
		return nil, fmt.Errorf("creating media store: retention must not be negative")
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}

	store := &Store{
		dataHome:             dataHome,
		sessionsRoot:         filepath.Join(dataHome, "hero", "sessions"),
		sessionDir:           filepath.Join(dataHome, "hero", "sessions", sessionID),
		assetsDir:            filepath.Join(dataHome, "hero", "sessions", sessionID, "assets"),
		manifestPath:         filepath.Join(dataHome, "hero", "sessions", sessionID, "manifest.json"),
		sessionID:            sessionID,
		workspace:            strings.TrimSpace(options.Workspace),
		allowExternalPaths:   options.AllowExternalPaths,
		limits:               normalizeLimits(options.Limits),
		retention:            retention,
		now:                  now,
		logger:               options.Logger,
		registeredSessionIDs: options.RegisteredSessionIDs,
		listRegistered:       options.ListRegistered,
		manifest:             Manifest{Version: manifestVersion, SessionID: sessionID},
	}
	if err := store.ensureLayout(); err != nil {
		return nil, err
	}
	if err := store.loadManifest(); err != nil {
		return nil, err
	}
	return store, nil
}

// SessionID returns the opaque id used for this store.
func (s *Store) SessionID() string { return s.sessionID }

// DataHome returns the resolved XDG data home used by this store.
func (s *Store) DataHome() string { return s.dataHome }

// SessionsDir returns the root containing session directories.
func (s *Store) SessionsDir() string { return s.sessionsRoot }

// SessionDir returns this session's private directory.
func (s *Store) SessionDir() string { return s.sessionDir }

// AssetsDir returns this session's private asset directory.
func (s *Store) AssetsDir() string { return s.assetsDir }

// ManifestPath returns the private JSON manifest path.
func (s *Store) ManifestPath() string { return s.manifestPath }

// Manifest returns a copy of the current manifest.
func (s *Store) Manifest() Manifest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneManifest(s.manifest)
}

// ReadManifest reloads and returns the on-disk manifest. It is useful for
// consumers that need to inspect the retained session after a restart.
func (s *Store) ReadManifest() (Manifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadManifestLocked(); err != nil {
		return Manifest{}, err
	}
	return cloneManifest(s.manifest), nil
}

// Materialize validates and copies one image into the session store, returning
// an immutable harness attachment reference. The source extension and declared
// MIME type are ignored in favor of magic-byte/header detection.
func (s *Store) Materialize(ctx context.Context, request MaterializeRequest) (harness.Attachment, error) {
	ctx = nonNilContext(ctx)
	path := strings.TrimSpace(request.sourcePath())
	if path == "" {
		return harness.Attachment{}, fmt.Errorf("materializing image: source path is required")
	}
	workspace := strings.TrimSpace(request.Workspace)
	if workspace == "" {
		workspace = s.workspace
	}
	validation, err := ValidateImage(ctx, path, ValidationOptions{
		Limits:            s.limits,
		Workspace:         workspace,
		AllowExternalPath: request.AllowExternalPath || s.allowExternalPaths,
	})
	if err != nil {
		return harness.Attachment{}, err
	}
	if validation.ExternalPathWarning && !request.AllowExternalPath && !s.allowExternalPaths {
		return harness.Attachment{}, ErrExternalPath
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return harness.Attachment{}, err
	}
	if err := s.checkTurnLimitsLocked(validation, strings.TrimSpace(request.TurnID)); err != nil {
		return harness.Attachment{}, err
	}

	materializedPath, created, err := s.findOrWriteBlobLocked(ctx, validation)
	if err != nil {
		return harness.Attachment{}, err
	}

	assetID, err := newUUID()
	if err != nil {
		if created {
			_ = os.Remove(materializedPath)
		}
		return harness.Attachment{}, fmt.Errorf("materializing image: generating asset id: %w", err)
	}
	entry := ManifestEntry{
		AssetID:   assetID,
		SessionID: s.sessionID,
		TurnID:    strings.TrimSpace(request.TurnID),
		Origin:    normalizeOrigin(request.Origin),
		MIMEType:  validation.MIMEType,
		Path:      materializedPath,
		Name:      sanitizeName(request.originalName(), validation.CanonicalPath),
		Size:      validation.Size,
		Width:     validation.Width,
		Height:    validation.Height,
		SHA256:    validation.SHA256,
		CreatedAt: s.now().UTC(),
	}
	s.manifest.Assets = append(s.manifest.Assets, entry)
	s.manifest.Entries = append(s.manifest.Entries, entry)
	if err := s.saveManifestLocked(); err != nil {
		s.manifest.Assets = s.manifest.Assets[:len(s.manifest.Assets)-1]
		s.manifest.Entries = s.manifest.Entries[:len(s.manifest.Entries)-1]
		if created {
			_ = os.Remove(materializedPath)
		}
		return harness.Attachment{}, err
	}

	return harness.Attachment{
		ID:          assetID,
		Kind:        harness.MediaKindImage,
		Name:        entry.Name,
		MIMEType:    entry.MIMEType,
		Path:        entry.Path,
		Size:        entry.Size,
		Width:       entry.Width,
		Height:      entry.Height,
		ContentHash: entry.SHA256,
	}, nil
}

// MaterializeAttachment is a descriptive alias for Materialize.
func (s *Store) MaterializeAttachment(ctx context.Context, request MaterializeRequest) (harness.Attachment, error) {
	return s.Materialize(ctx, request)
}

// MaterializeTurn validates and materializes a batch of attachments. Limits
// are evaluated per turn before any request is written, so a rejected batch
// cannot exceed the configured count or pixel budget.
func (s *Store) MaterializeTurn(ctx context.Context, requests []MaterializeRequest) ([]harness.Attachment, error) {
	ctx = nonNilContext(ctx)
	if len(requests) == 0 {
		return nil, nil
	}
	byTurn := make(map[string]int)
	pixelsByTurn := make(map[string]int64)
	for _, request := range requests {
		turnID := strings.TrimSpace(request.TurnID)
		byTurn[turnID]++
		if byTurn[turnID] > s.limits.MaxAttachmentsPerTurn {
			return nil, ErrTooManyAttachments
		}
	}
	for _, request := range requests {
		validation, err := ValidateImage(ctx, request.sourcePath(), ValidationOptions{
			Limits:    s.limits,
			Workspace: firstNonEmpty(request.Workspace, s.workspace),
		})
		if err != nil {
			return nil, err
		}
		if validation.ExternalPathWarning && !request.AllowExternalPath && !s.allowExternalPaths {
			return nil, ErrExternalPath
		}
		turnID := strings.TrimSpace(request.TurnID)
		pixelsByTurn[turnID] += validation.Pixels
		if pixelsByTurn[turnID] > s.limits.MaxPixelsPerTurn {
			return nil, ErrTurnPixelLimitExceeded
		}
	}

	attachments := make([]harness.Attachment, 0, len(requests))
	for _, request := range requests {
		attachment, err := s.Materialize(ctx, request)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

func (s *Store) checkTurnLimitsLocked(validation ValidationResult, turnID string) error {
	count := 0
	var pixels int64
	for _, entry := range s.manifest.Assets {
		if strings.TrimSpace(entry.TurnID) != turnID {
			continue
		}
		count++
		pixels += safePixelCount(entry.Width, entry.Height)
	}
	if count+1 > s.limits.MaxAttachmentsPerTurn {
		return ErrTooManyAttachments
	}
	if validation.Pixels > s.limits.MaxPixelsPerTurn || pixels > s.limits.MaxPixelsPerTurn-validation.Pixels {
		return ErrTurnPixelLimitExceeded
	}
	return nil
}

func (s *Store) findOrWriteBlobLocked(ctx context.Context, validation ValidationResult) (string, bool, error) {
	for _, entry := range s.manifest.Assets {
		if !strings.EqualFold(entry.SHA256, validation.SHA256) || entry.Size != validation.Size {
			continue
		}
		path, ok := s.safeManifestPath(entry.Path)
		if !ok {
			continue
		}
		valid, err := verifyFileHash(ctx, path, validation.SHA256, validation.Size)
		if err != nil {
			return "", false, err
		}
		if valid {
			if err := os.Chmod(path, 0o600); err != nil {
				return "", false, wrapFilesystemError("securing image asset", err)
			}
			return path, false, nil
		}
	}

	for attempt := 0; attempt < 8; attempt++ {
		assetID, err := newUUID()
		if err != nil {
			return "", false, fmt.Errorf("materializing image: generating file id: %w", err)
		}
		path := filepath.Join(s.assetsDir, assetID)
		created, err := copyValidatedFile(ctx, validation.CanonicalPath, path, validation)
		if err != nil {
			return "", false, err
		}
		if created {
			return path, true, nil
		}
	}
	return "", false, fmt.Errorf("materializing image: could not allocate a unique asset path")
}

func copyValidatedFile(ctx context.Context, sourcePath, destinationPath string, validation ValidationResult) (bool, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return false, wrapFilesystemError("opening image source", err)
	}
	defer source.Close()

	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil
		}
		return false, wrapFilesystemError("creating image asset", err)
	}
	removeDestination := true
	defer func() {
		if removeDestination {
			_ = os.Remove(destinationPath)
		}
	}()

	hasher := sha256.New()
	buffer := make([]byte, 32*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			_ = destination.Close()
			return false, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			written += int64(read)
			if written > validation.Size {
				_ = destination.Close()
				return false, ErrFileTooLarge
			}
			if _, err := hasher.Write(buffer[:read]); err != nil {
				_ = destination.Close()
				return false, fmt.Errorf("materializing image: hashing source: %w", err)
			}
			if _, err := destination.Write(buffer[:read]); err != nil {
				_ = destination.Close()
				return false, wrapFilesystemError("writing image asset", err)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			_ = destination.Close()
			return false, wrapFilesystemError("reading image source", readErr)
		}
	}
	if written != validation.Size || !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), validation.SHA256) {
		_ = destination.Close()
		return false, ErrSourceChanged
	}
	if err := destination.Sync(); err != nil {
		_ = destination.Close()
		return false, wrapFilesystemError("syncing image asset", err)
	}
	if err := destination.Close(); err != nil {
		return false, wrapFilesystemError("closing image asset", err)
	}
	if err := os.Chmod(destinationPath, 0o600); err != nil {
		return false, wrapFilesystemError("securing image asset", err)
	}
	removeDestination = false
	return true, nil
}

func verifyFileHash(ctx context.Context, path, expected string, expectedSize int64) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, wrapFilesystemError("opening stored image asset", err)
	}
	defer file.Close()

	hasher := sha256.New()
	buffer := make([]byte, 32*1024)
	var size int64
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			size += int64(read)
			if size > expectedSize {
				return false, nil
			}
			if _, err := hasher.Write(buffer[:read]); err != nil {
				return false, fmt.Errorf("verifying stored image asset: hashing: %w", err)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return false, wrapFilesystemError("reading stored image asset", readErr)
		}
	}
	return size == expectedSize && strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), expected), nil
}

func (s *Store) ensureLayout() error {
	for _, path := range []string{
		filepath.Join(s.dataHome, "hero"),
		s.sessionsRoot,
		s.sessionDir,
		s.assetsDir,
	} {
		if err := ensurePrivateDir(path); err != nil {
			return err
		}
	}
	return nil
}

func ensurePrivateDir(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return wrapFilesystemError("creating media directory", err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return wrapFilesystemError("checking media directory", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("media directory is not a private directory")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return wrapFilesystemError("securing media directory", err)
	}
	return nil
}

func (s *Store) loadManifest() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadManifestLocked()
}

func (s *Store) loadManifestLocked() error {
	info, err := os.Lstat(s.manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		s.manifest = Manifest{Version: manifestVersion, SessionID: s.sessionID}
		return nil
	}
	if err != nil {
		return wrapFilesystemError("checking media manifest", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("media manifest is not a regular file")
	}
	if err := os.Chmod(s.manifestPath, 0o600); err != nil {
		return wrapFilesystemError("securing media manifest", err)
	}
	file, err := os.Open(s.manifestPath)
	if err != nil {
		return wrapFilesystemError("opening media manifest", err)
	}
	defer file.Close()
	const maxManifestBytes = 8 * 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(file, maxManifestBytes+1))
	if err != nil {
		return wrapFilesystemError("reading media manifest", err)
	}
	if len(data) > maxManifestBytes {
		return fmt.Errorf("media manifest exceeds the maximum size")
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("decoding media manifest: %w", err)
	}
	if manifest.SessionID != "" && manifest.SessionID != s.sessionID {
		return fmt.Errorf("media manifest belongs to a different session")
	}
	manifest.Version = manifestVersion
	manifest.SessionID = s.sessionID
	manifest.Entries = append([]ManifestEntry(nil), manifest.Assets...)
	s.manifest = manifest
	return nil
}

func (s *Store) saveManifestLocked() error {
	temporary, err := os.CreateTemp(s.sessionDir, ".manifest-*")
	if err != nil {
		return wrapFilesystemError("creating media manifest", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return wrapFilesystemError("securing media manifest", err)
	}
	data, err := json.MarshalIndent(s.manifest, "", "  ")
	if err != nil {
		_ = temporary.Close()
		return fmt.Errorf("encoding media manifest: %w", err)
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		_ = temporary.Close()
		return wrapFilesystemError("writing media manifest", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return wrapFilesystemError("syncing media manifest", err)
	}
	if err := temporary.Close(); err != nil {
		return wrapFilesystemError("closing media manifest", err)
	}
	if err := os.Rename(temporaryPath, s.manifestPath); err != nil {
		return wrapFilesystemError("installing media manifest", err)
	}
	removeTemporary = false
	if err := os.Chmod(s.manifestPath, 0o600); err != nil {
		return wrapFilesystemError("securing media manifest", err)
	}
	return nil
}

func (s *Store) safeManifestPath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.sessionDir, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(s.assetsDir, abs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", false
	}
	info, err := os.Lstat(abs)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", false
	}
	return abs, true
}

func cloneManifest(manifest Manifest) Manifest {
	if len(manifest.Assets) == 0 && len(manifest.Entries) > 0 {
		manifest.Assets = append([]ManifestEntry(nil), manifest.Entries...)
	}
	manifest.Assets = append([]ManifestEntry(nil), manifest.Assets...)
	manifest.Entries = append([]ManifestEntry(nil), manifest.Assets...)
	return manifest
}

func cleanSessionID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." || filepath.IsAbs(value) || strings.ContainsRune(value, '\x00') || strings.ContainsAny(value, `/\\`) {
		return "", ErrInvalidSessionID
	}
	return value, nil
}

func resolveDataHome(override string) (string, error) {
	dataHome := strings.TrimSpace(override)
	if dataHome == "" {
		dataHome = strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	}
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving media data home: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	if !filepath.IsAbs(dataHome) {
		return "", ErrInvalidDataHome
	}
	return filepath.Clean(dataHome), nil
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(rand.Reader, value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

func sanitizeName(name, fallbackPath string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(fallbackPath)
	}
	name = strings.NewReplacer("\\", "/").Replace(name)
	name = filepath.Base(name)
	if name == "." || name == ".." || name == string(filepath.Separator) || name == "" {
		name = "image"
	}
	var builder strings.Builder
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r < 0x20 || r == 0x7f:
			builder.WriteByte('_')
		default:
			builder.WriteRune(r)
		}
	}
	name = strings.TrimSpace(builder.String())
	if name == "" || name == "." || name == ".." {
		return "image"
	}
	if len([]rune(name)) > 255 {
		name = string([]rune(name)[:255])
	}
	return name
}

func normalizeOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return "user"
	}
	return origin
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func safePixelCount(width, height int) int64 {
	if width <= 0 || height <= 0 {
		return 0
	}
	return int64(width) * int64(height)
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// safeOperationError preserves errors.Is/errors.As behavior without copying
// paths from os.PathError into an error message that may later be logged.
type safeOperationError struct {
	op  string
	err error
}

func (e *safeOperationError) Error() string {
	return e.op + ": " + safeUnderlyingError(e.err)
}

func (e *safeOperationError) Unwrap() error { return e.err }

func wrapFilesystemError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &safeOperationError{op: operation, err: err}
}

func safeUnderlyingError(err error) string {
	if err == nil {
		return "operation failed"
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return linkErr.Err.Error()
	}
	return "operation failed"
}
