package testaccess

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const credentialFile = ".env.hero"

var (
	// ErrStaleDraft prevents overwriting a manual edit with an older Config draft.
	ErrStaleDraft = errors.New(".env.hero changed outside Config; Reload before saving")
	// ErrStoreBusy means another cooperating TUI owns test-access operations.
	ErrStoreBusy = errors.New("test access is owned by another TUI; close its editor or execution before retrying")
	// ErrTrackedCredentials requires operator action; Hero never untracks files.
	ErrTrackedCredentials = errors.New(".env.hero is tracked by Git; remove it from tracking and configure /.env.hero in .gitignore before retrying")
)

// Draft holds secret-bearing editor data. It must never be persisted in YAML,
// reports, events, conversation history, or diagnostic messages.
type Draft struct {
	Document Document `json:"-"`
	revision [32]byte
	exists   bool
	project  string
}

func (Draft) String() string   { return "test-access draft (redacted)" }
func (Draft) GoString() string { return "test-access draft (redacted)" }

// CredentialSnapshot pins one selected account for an attempt. Subsequent file
// edits affect only a new explicit Snapshot call, never the running attempt.
type CredentialSnapshot struct {
	account Account
}

// Account returns the selected account to the deterministic executor only.
func (s CredentialSnapshot) Account() Account { return s.account }
func (CredentialSnapshot) String() string     { return "test-access snapshot (redacted)" }
func (CredentialSnapshot) GoString() string   { return "test-access snapshot (redacted)" }

// SafeStore anchors operations to a project directory and holds an exclusive
// process lock until Close. The lock is cooperative, not a harness sandbox.
type SafeStore struct {
	mu               sync.Mutex
	root             *os.Root
	path             string
	identity         os.FileInfo
	metadata         *os.Root
	metadataIdentity os.FileInfo
	lock             *os.File
	log              *slog.Logger
}

// OpenSafeStore requires an existing Git project with Hero metadata. It never
// reads credentials until Load or Snapshot and never exposes Git output.
func OpenSafeStore(projectDir string, logger *slog.Logger) (*SafeStore, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if !filepath.IsAbs(projectDir) || filepath.Clean(projectDir) != projectDir || projectDir == string(filepath.Separator) {
		return nil, errors.New("test access requires an absolute, clean project-root path")
	}
	info, err := os.Lstat(projectDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("test-access project root must be a real directory")
	}
	canonical, err := filepath.EvalSymlinks(projectDir)
	if err != nil {
		return nil, errors.New("cannot resolve test-access project root")
	}
	root, err := os.OpenRoot(canonical)
	if err != nil {
		return nil, errors.New("cannot open test-access project root")
	}
	s := &SafeStore{root: root, path: canonical, identity: info, log: logger}
	metadata, err := root.Lstat(".workflow-hero")
	if err != nil || !metadata.IsDir() || metadata.Mode()&os.ModeSymlink != 0 {
		_ = root.Close()
		return nil, errors.New("test access requires real project-root .workflow-hero metadata")
	}
	metadataRoot, err := root.OpenRoot(".workflow-hero")
	if err != nil {
		_ = root.Close()
		return nil, errors.New("cannot anchor test-access ownership")
	}
	anchored, err := metadataRoot.Stat(".")
	if err != nil || !os.SameFile(metadata, anchored) {
		_ = metadataRoot.Close()
		_ = root.Close()
		return nil, errors.New("test-access metadata ownership changed")
	}
	lock, err := metadataRoot.OpenFile("testaccess.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		_ = metadataRoot.Close()
		_ = root.Close()
		return nil, errors.New("cannot acquire safe test-access ownership")
	}
	lockInfo, err := lock.Stat()
	if err != nil || !safeOwnedFile(lockInfo) {
		_ = lock.Close()
		_ = metadataRoot.Close()
		_ = root.Close()
		return nil, errors.New("unsafe test-access ownership file")
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		_ = metadataRoot.Close()
		_ = root.Close()
		return nil, ErrStoreBusy
	}
	s.lock = lock
	s.metadata = metadataRoot
	s.metadataIdentity = metadata
	if err := s.recoverTemporaryWrites(); err != nil {
		_ = s.Close()
		return nil, err
	}
	s.log.Debug("test-access ownership acquired")
	return s, nil
}

// Close releases ownership. The empty lock file remains to prevent inode races
// between cooperating owners; it contains no credentials or identifiers.
func (s *SafeStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nil {
		return nil
	}
	err := s.lock.Close()
	metadataErr := s.metadata.Close()
	rootErr := s.root.Close()
	s.root = nil
	return errors.Join(err, metadataErr, rootErr)
}

func safeOwnedFile(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Getuid()) && stat.Nlink == 1
}

// Recover only our exact nonce-shaped temporary names under exclusive
// ownership. Never follow a link or remove any arbitrary user file.
func (s *SafeStore) recoverTemporaryWrites() error {
	entries, err := os.ReadDir(s.path)
	if err != nil {
		return errors.New("cannot inspect interrupted test-access writes")
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".env.hero.tmp-") {
			continue
		}
		nonce := strings.TrimPrefix(name, ".env.hero.tmp-")
		if len(nonce) != 32 {
			continue
		}
		if _, err := hex.DecodeString(nonce); err != nil {
			continue
		}
		info, err := s.root.Lstat(name)
		if err != nil {
			return errors.New("cannot inspect interrupted test-access write")
		}
		safe := safeOwnedFile(info)
		if !safe && info.Mode().IsRegular() && info.Mode().Perm() == 0o600 && info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) == 0 {
			stat, ok := info.Sys().(*syscall.Stat_t)
			credential, credentialErr := s.root.Lstat(credentialFile)
			// A process may have died between create-only Link and unlink.
			safe = ok && stat.Uid == uint32(os.Getuid()) && stat.Nlink == 2 && credentialErr == nil && os.SameFile(info, credential)
		}
		if !safe {
			return errors.New("unsafe interrupted test-access temporary file; correct its ownership/path before retrying")
		}
		if err := s.root.Remove(name); err != nil {
			return errors.New("cannot remove interrupted test-access temporary file; remove the local temporary file before retrying")
		}
		s.log.Info("interrupted test-access temporary write removed")
	}
	return nil
}

func (s *SafeStore) checkOwnership() error {
	if s.root == nil {
		return errors.New("test-access store is closed")
	}
	info, err := os.Stat(s.path)
	if err != nil || !os.SameFile(info, s.identity) {
		return errors.New("test-access project ownership changed; reopen Config")
	}
	metadata, err := s.root.Lstat(".workflow-hero")
	if err != nil || !metadata.IsDir() || metadata.Mode()&os.ModeSymlink != 0 || !os.SameFile(metadata, s.metadataIdentity) {
		return errors.New("test-access metadata ownership changed; reopen Config")
	}
	info, err = s.metadata.Lstat("testaccess.lock")
	held, heldErr := s.lock.Stat()
	if err != nil || heldErr != nil || !safeOwnedFile(info) || !os.SameFile(info, held) {
		return ErrStoreBusy
	}
	return nil
}

func (s *SafeStore) gitCheck(ctx context.Context, requireIgnored bool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := func(args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "git", append([]string{"-C", s.path}, args...)...)
	}
	out, err := command("rev-parse", "--show-toplevel").Output()
	if err != nil || strings.TrimSpace(string(out)) != s.path {
		return errors.New("test access requires the Git project root")
	}
	err = command("ls-files", "--error-unmatch", "--", credentialFile).Run()
	if err == nil {
		return ErrTrackedCredentials
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		return errors.New("cannot verify .env.hero tracking status")
	}
	if requireIgnored {
		if err := command("check-ignore", "--quiet", "--no-index", "--", credentialFile).Run(); err != nil {
			return errors.New(".env.hero is not ignored by Git; add /.env.hero to project-root .gitignore before retrying")
		}
	}
	return nil
}

func (s *SafeStore) read() ([]byte, bool, error) {
	info, err := s.root.Lstat(credentialFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil || !safeOwnedFile(info) {
		return nil, false, errors.New("unsafe .env.hero; use a regular, owner-held, non-linked file with mode 0600")
	}
	f, err := s.root.OpenFile(credentialFile, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, false, errors.New("cannot safely open .env.hero")
	}
	defer func() { _ = f.Close() }()
	opened, err := f.Stat()
	if err != nil || !safeOwnedFile(opened) || !os.SameFile(info, opened) {
		return nil, false, errors.New(".env.hero ownership changed; Reload before retrying")
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, false, errors.New("cannot read .env.hero")
	}
	after, err := f.Stat()
	if err != nil || !safeOwnedFile(after) || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
		return nil, false, errors.New(".env.hero changed while reading; Reload before retrying")
	}
	return data, true, nil
}

func (s *SafeStore) load(ctx context.Context) (Draft, error) {
	if err := s.checkOwnership(); err != nil {
		return Draft{}, err
	}
	data, exists, err := s.read()
	if err != nil {
		return Draft{}, err
	}
	if err := s.gitCheck(ctx, exists); err != nil {
		return Draft{}, err
	}
	doc, err := ParseDotenv(data)
	if err != nil {
		return Draft{}, err
	}
	s.log.Debug("test-access draft loaded", "file_present", exists)
	return Draft{Document: doc, revision: sha256.Sum256(data), exists: exists, project: s.path}, nil
}

// Load validates manual files before exposing an editor draft. Missing files
// yield an empty draft; Save establishes the ignore rule before creating one.
func (s *SafeStore) Load(ctx context.Context) (Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft, err := s.load(ctx)
	if err != nil {
		s.log.Error("test-access load blocked")
	}
	return draft, err
}

// Snapshot rereads the latest validated file and retains only the selected
// usable account in private attempt memory.
func (s *SafeStore) Snapshot(ctx context.Context, userID string) (CredentialSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft, err := s.load(ctx)
	if err != nil {
		return CredentialSnapshot{}, err
	}
	account, err := draft.Document.SelectedAccount(userID)
	if err != nil {
		return CredentialSnapshot{}, err
	}
	s.log.Debug("test-access attempt snapshot created")
	return CredentialSnapshot{account: account}, nil
}

func (s *SafeStore) ensureIgnore() error {
	info, err := s.root.Lstat(".gitignore")
	var content []byte
	mode := os.FileMode(0o644)
	if err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("unsafe .gitignore; use a regular project-root file")
		}
		f, err := s.root.OpenFile(".gitignore", os.O_RDONLY|syscall.O_NOFOLLOW, 0)
		if err != nil {
			return errors.New("cannot safely read .gitignore")
		}
		content, err = io.ReadAll(f)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return errors.New("cannot read .gitignore")
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("cannot inspect .gitignore")
	}
	// Append after negations, so the root rule is effective and remains
	// idempotent even when an existing earlier rule has been overridden.
	lastRule, previousRule := "", ""
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			previousRule = lastRule
			lastRule = line
		}
	}
	if previousRule == "/.env.hero.tmp-*" && lastRule == "/.env.hero" {
		return nil
	}
	if len(content) > 0 && content[len(content)-1] != '\n' {
		content = append(content, '\n')
	}
	content = append(content, []byte("/.env.hero.tmp-*\n/.env.hero\n")...)
	return s.replace(".gitignore", content, mode, nil, false)
}

// Save refuses stale drafts and atomically replaces only the root credential
// file. It creates no backups and leaves no temporary plaintext on failure.
func (s *SafeStore) Save(ctx context.Context, draft Draft) (Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOwnership(); err != nil {
		return Draft{}, err
	}
	if err := s.gitCheck(ctx, false); err != nil {
		return Draft{}, err
	}
	if draft.project != s.path {
		return Draft{}, errors.New("test-access draft belongs to another project; Reload before saving")
	}
	data, exists, err := s.read()
	if err != nil {
		return Draft{}, err
	}
	if exists != draft.exists || sha256.Sum256(data) != draft.revision {
		return Draft{}, ErrStaleDraft
	}
	serialized, err := SerializeDotenv(draft.Document)
	if err != nil {
		return Draft{}, err
	}
	if err := s.ensureIgnore(); err != nil {
		return Draft{}, err
	}
	if err := s.gitCheck(ctx, true); err != nil {
		return Draft{}, err
	}
	// Recheck after Git/ignore work rather than overwriting an intervening edit.
	data, exists, err = s.read()
	if err != nil {
		return Draft{}, err
	}
	if exists != draft.exists || sha256.Sum256(data) != draft.revision {
		return Draft{}, ErrStaleDraft
	}
	if err := ctx.Err(); err != nil {
		return Draft{}, errors.New("test-access save cancelled")
	}
	precommit := func() error {
		if err := s.checkOwnership(); err != nil {
			return err
		}
		current, present, err := s.read()
		if err != nil {
			return err
		}
		if present != draft.exists || sha256.Sum256(current) != draft.revision {
			return ErrStaleDraft
		}
		if ctx.Err() != nil {
			return errors.New("test-access save cancelled")
		}
		return nil
	}
	if err := s.replace(credentialFile, serialized, 0o600, precommit, !draft.exists); err != nil {
		s.log.Error("test-access save failed")
		return Draft{}, err
	}
	s.log.Info("test-access file saved")
	return Draft{Document: draft.Document, revision: sha256.Sum256(serialized), exists: true, project: s.path}, nil
}

func (s *SafeStore) replace(target string, data []byte, mode os.FileMode, precommit func() error, createOnly bool) (resultErr error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return errors.New("cannot prepare atomic test-access write")
	}
	tmp := target + ".tmp-" + hex.EncodeToString(nonce[:])
	f, err := s.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return errors.New("cannot prepare atomic test-access write")
	}
	defer func() {
		_ = f.Close()
		if err := s.root.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Error("test-access temporary-file cleanup failed")
			resultErr = errors.Join(resultErr, errors.New("test-access temporary-file cleanup failed; remove the unfinished local temporary file before retrying"))
		}
	}()
	if _, err := f.Write(data); err != nil {
		return errors.New("cannot write test-access file")
	}
	if err := f.Chmod(mode); err != nil {
		return errors.New("cannot set test-access file permissions")
	}
	if err := f.Sync(); err != nil {
		return errors.New("cannot sync test-access file")
	}
	if err := f.Close(); err != nil {
		return errors.New("cannot close test-access file")
	}
	if precommit != nil {
		if err := precommit(); err != nil {
			return err
		}
	}
	if createOnly {
		// Link is atomic and refuses an intervening manually created file. The
		// temporary name is removed by the deferred cleanup before returning.
		if err := s.root.Link(tmp, target); err != nil {
			if errors.Is(err, os.ErrExist) {
				return ErrStaleDraft
			}
			return errors.New("cannot atomically create test-access file")
		}
	} else if err := s.root.Rename(tmp, target); err != nil {
		return errors.New("cannot atomically replace test-access file")
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return errors.New("test-access file replaced but directory sync failed; Reload before retrying")
	}
	defer func() { _ = dir.Close() }()
	if err := dir.Sync(); err != nil {
		return errors.New("test-access file replaced but directory sync failed; Reload before retrying")
	}
	return nil
}

// Format prevents reflection-based formatting from traversing secret fields.
func (Draft) Format(state fmt.State, verb rune) {
	_, _ = io.WriteString(state, "test-access draft (redacted)")
}
func (CredentialSnapshot) Format(state fmt.State, verb rune) {
	_, _ = io.WriteString(state, "test-access snapshot (redacted)")
}
