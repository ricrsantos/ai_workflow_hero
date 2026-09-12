package todos

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const (
	tmpRelDir = ".workflow-hero/tmp"
)

// ErrProjectionFailed indicates current-state.md could not be reconciled.
var ErrProjectionFailed = errors.New("todo projection failed")

// FormatProjectedLine renders one Pending Features bullet (design D6).
func FormatProjectedLine(id, status, origin, summary string) string {
	return fmt.Sprintf("- `%s` · %s · %s · %s", id, status, origin, summary)
}

// CandidatePath returns the fsynced candidate path for an idempotency key.
func CandidatePath(projectDir, idempotencyKey string) string {
	name := "todo-projection-" + sanitizeIdempotencyKey(idempotencyKey) + ".md"
	return filepath.Join(projectDir, tmpRelDir, name)
}

// ReconcileProjection runs the recoverable D6 protocol for one projection op.
func ReconcileProjection(projectDir string, st *store.Store, idempotencyKey string) error {
	if st == nil {
		return fmt.Errorf("store is required")
	}
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return fmt.Errorf("idempotency key is required")
	}
	op, err := st.GetTodoProjectionOpByKey(key)
	if err != nil {
		slog.Error("todo projection failed", "idempotency_key", key, "error", err)
		return fmt.Errorf("%w: load projection op: %v", ErrProjectionFailed, err)
	}
	if op.Status == store.ProjectionStatusVerified {
		return nil
	}

	targetPath := filepath.Join(projectDir, CurrentStateRelPath)
	candidatePath := CandidatePath(projectDir, key)

	switch op.Status {
	case store.ProjectionStatusIntentPersisted:
		if err := ensureCandidateReady(projectDir, st, key, targetPath, candidatePath); err != nil {
			return err
		}
		fallthrough
	case store.ProjectionStatusCandidateReady:
		if err := installCandidate(projectDir, st, key, targetPath, candidatePath); err != nil {
			return err
		}
		fallthrough
	case store.ProjectionStatusInstalled:
		return verifyInstalled(st, key, targetPath, candidatePath)
	default:
		slog.Error("todo projection failed", "idempotency_key", key, "status", op.Status)
		return fmt.Errorf("%w: unknown projection status %q", ErrProjectionFailed, op.Status)
	}
}

func ensureCandidateReady(projectDir string, st *store.Store, key, targetPath, candidatePath string) error {
	candidate, err := BuildCurrentStateCandidate(projectDir, st)
	if err != nil {
		slog.Error("todo projection failed", "idempotency_key", key, "error", err)
		return fmt.Errorf("%w: build candidate: %v", ErrProjectionFailed, err)
	}
	hash := hashBytes(candidate)
	slog.Debug("todo projection candidate hash", "idempotency_key", key, "sha256", hash)
	if err := writeFsyncCandidate(candidatePath, candidate); err != nil {
		slog.Error("todo projection failed", "idempotency_key", key, "error", err)
		return fmt.Errorf("%w: write candidate: %v", ErrProjectionFailed, err)
	}
	if err := st.UpdateTodoProjectionOpStatus(key, store.ProjectionStatusCandidateReady, hash); err != nil {
		slog.Error("todo projection failed", "idempotency_key", key, "error", err)
		return fmt.Errorf("%w: persist candidate status: %v", ErrProjectionFailed, err)
	}
	return nil
}

func installCandidate(projectDir string, st *store.Store, key, targetPath, candidatePath string) error {
	op, err := st.GetTodoProjectionOpByKey(key)
	if err != nil {
		slog.Error("todo projection failed", "idempotency_key", key, "error", err)
		return fmt.Errorf("%w: reload projection op: %v", ErrProjectionFailed, err)
	}
	candidate, err := os.ReadFile(candidatePath)
	if err != nil {
		if op.Status == store.ProjectionStatusCandidateReady {
			if err := ensureCandidateReady(projectDir, st, key, targetPath, candidatePath); err != nil {
				return err
			}
			candidate, err = os.ReadFile(candidatePath)
		}
		if err != nil {
			slog.Error("todo projection failed", "idempotency_key", key, "error", err)
			return fmt.Errorf("%w: read candidate: %v", ErrProjectionFailed, err)
		}
	}
	if op.CandidateSHA256 != "" && hashBytes(candidate) != op.CandidateSHA256 {
		if err := ensureCandidateReady(projectDir, st, key, targetPath, candidatePath); err != nil {
			return err
		}
		candidate, err = os.ReadFile(candidatePath)
		if err != nil {
			slog.Error("todo projection failed", "idempotency_key", key, "error", err)
			return fmt.Errorf("%w: read rebuilt candidate: %v", ErrProjectionFailed, err)
		}
	}
	if err := installCurrentStateAtomically(targetPath, candidate); err != nil {
		slog.Error("todo projection failed", "idempotency_key", key, "error", err)
		return fmt.Errorf("%w: install projection: %v", ErrProjectionFailed, err)
	}
	slog.Info("todo projection installed", "idempotency_key", key, "path", CurrentStateRelPath)
	if err := st.UpdateTodoProjectionOpStatus(key, store.ProjectionStatusInstalled, op.CandidateSHA256); err != nil {
		slog.Error("todo projection failed", "idempotency_key", key, "error", err)
		return fmt.Errorf("%w: persist installed status: %v", ErrProjectionFailed, err)
	}
	return nil
}

func verifyInstalled(st *store.Store, key, targetPath, candidatePath string) error {
	op, err := st.GetTodoProjectionOpByKey(key)
	if err != nil {
		slog.Error("todo projection failed", "idempotency_key", key, "error", err)
		return fmt.Errorf("%w: reload projection op: %v", ErrProjectionFailed, err)
	}
	installed, err := os.ReadFile(targetPath)
	if err != nil {
		slog.Error("todo projection failed", "idempotency_key", key, "error", err)
		return fmt.Errorf("%w: read installed file: %v", ErrProjectionFailed, err)
	}
	candidate, err := os.ReadFile(candidatePath)
	if err == nil {
		if hashBytes(installed) != hashBytes(candidate) {
			slog.Error("todo projection failed", "idempotency_key", key, "reason", "installed bytes mismatch candidate")
			return fmt.Errorf("%w: installed file does not match candidate", ErrProjectionFailed)
		}
	} else if op.CandidateSHA256 != "" && hashBytes(installed) != op.CandidateSHA256 {
		slog.Error("todo projection failed", "idempotency_key", key, "reason", "installed hash mismatch")
		return fmt.Errorf("%w: installed file hash mismatch", ErrProjectionFailed)
	}
	if err := verifyProjectedTodoIDs(installed, op.OpKind, op.TodoIDsJSON); err != nil {
		slog.Error("todo projection failed", "idempotency_key", key, "error", err)
		return fmt.Errorf("%w: %v", ErrProjectionFailed, err)
	}
	slog.Info("todo projection verified", "idempotency_key", key, "path", CurrentStateRelPath)
	if err := st.UpdateTodoProjectionOpStatus(key, store.ProjectionStatusVerified, op.CandidateSHA256); err != nil {
		slog.Error("todo projection failed", "idempotency_key", key, "error", err)
		return fmt.Errorf("%w: persist verified status: %v", ErrProjectionFailed, err)
	}
	return nil
}

// BuildCurrentStateCandidate rebuilds current-state.md from SQLite plus unmatched prose.
func BuildCurrentStateCandidate(projectDir string, st *store.Store) ([]byte, error) {
	if st == nil {
		return nil, fmt.Errorf("store is required")
	}
	path := filepath.Join(projectDir, CurrentStateRelPath)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", CurrentStateRelPath, err)
	}
	projected, err := st.ListTodosForProjection()
	if err != nil {
		return nil, err
	}
	structured, err := formatProjectedLines(st, projected)
	if err != nil {
		return nil, err
	}
	suppress, err := legacyProseSuppressions(st, projected)
	if err != nil {
		return nil, err
	}
	return rewritePendingSection(content, structured, suppress)
}

func formatProjectedLines(st *store.Store, projected []store.Todo) ([]string, error) {
	cache := make(map[int64]int)
	lines := make([]string, 0, len(projected))
	for _, t := range projected {
		origin, err := originLabel(st, t, cache)
		if err != nil {
			return nil, err
		}
		lines = append(lines, FormatProjectedLine(t.ID, t.Status, origin, t.Summary))
	}
	return lines, nil
}

func originLabel(st *store.Store, t store.Todo, cache map[int64]int) (string, error) {
	if t.Status == store.TodoStatusAdopted {
		n, err := cycleNumber(st, t.AdoptedCycleID, cache)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("C%d", n), nil
	}
	if t.OriginType == store.TodoOriginLegacy {
		return "legacy", nil
	}
	n, err := cycleNumber(st, t.OriginCycleID, cache)
	if err != nil {
		return "", err
	}
	stage := strings.TrimSpace(t.OriginSourceStage)
	if stage == "" {
		return fmt.Sprintf("C%d", n), nil
	}
	return fmt.Sprintf("C%d/%s", n, strings.ToUpper(stage)), nil
}

func cycleNumber(st *store.Store, id int64, cache map[int64]int) (int, error) {
	if id == 0 {
		return 0, fmt.Errorf("cycle id is required")
	}
	if n, ok := cache[id]; ok {
		return n, nil
	}
	c, err := st.GetCycle(id)
	if err != nil {
		return 0, err
	}
	cache[id] = c.Number
	return c.Number, nil
}

func legacyProseSuppressions(st *store.Store, projected []store.Todo) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	for _, t := range projected {
		if n := NormalizeLegacyLine(t.Summary); n != "" {
			out[n] = struct{}{}
		}
	}
	resolved, err := st.ListResolvedLegacySummaries()
	if err != nil {
		return nil, err
	}
	for _, summary := range resolved {
		if n := NormalizeLegacyLine(summary); n != "" {
			out[n] = struct{}{}
		}
	}
	return out, nil
}

func rewritePendingSection(content []byte, structured []string, suppress map[string]struct{}) ([]byte, error) {
	var out []string
	inPending := false
	var unmatched []string

	flushPending := func() {
		if !inPending {
			return
		}
		out = append(out, unmatched...)
		out = append(out, structured...)
		unmatched = nil
		inPending = false
	}

	for line := range strings.Lines(string(content)) {
		line = strings.TrimRight(line, "\r\n")
		if h, ok := parseHeading(line); ok {
			if inPending {
				flushPending()
			}
			out = append(out, line)
			inPending = pendingSectionSet()[h]
			continue
		}
		if !inPending {
			out = append(out, line)
			continue
		}
		if text, ok := parseListItem(line); ok {
			if IsStructuredProjectedLine(text) {
				continue
			}
			if _, skip := suppress[NormalizeLegacyLine(text)]; skip {
				continue
			}
			unmatched = append(unmatched, line)
			continue
		}
		out = append(out, line)
	}
	if inPending {
		flushPending()
	}
	result := strings.Join(out, "\n")
	if len(content) > 0 && content[len(content)-1] == '\n' {
		result += "\n"
	}
	return []byte(result), nil
}

func verifyProjectedTodoIDs(content []byte, opKind, todoIDsJSON string) error {
	ids, err := parseTodoIDsFromJSON(todoIDsJSON)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	items, err := Parse(content)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if sl, ok := ParseStructuredProjectedLine(item.Text); ok {
			seen[sl.ID] = true
		}
	}
	// Complete/resolve removes IDs from Pending; defer/adopt/release must keep them visible.
	mustBeAbsent := opKind == store.ProjectionOpComplete
	for _, id := range ids {
		present := seen[id]
		if mustBeAbsent {
			if present {
				return fmt.Errorf("projected todo id %q still present in pending section after complete", id)
			}
			continue
		}
		if !present {
			return fmt.Errorf("projected todo id %q missing from pending section", id)
		}
	}
	return nil
}

func parseTodoIDsFromJSON(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, fmt.Errorf("parse todo_ids_json: %w", err)
	}
	return ids, nil
}

func writeFsyncCandidate(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir tmp: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".candidate-*.md")
	if err != nil {
		return fmt.Errorf("create temp candidate: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp candidate: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp candidate: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp candidate: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("install candidate tmp file: %w", err)
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("open tmp dir: %w", err)
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close()
		return fmt.Errorf("sync tmp dir: %w", err)
	}
	_ = dir.Close()
	return nil
}

func installCurrentStateAtomically(path string, content []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat current-state: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".current-state.hero-*")
	if err != nil {
		return fmt.Errorf("create temp current-state: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp current-state: %w", err)
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp current-state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp current-state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp current-state: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace current-state: %w", err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open context dir: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return fmt.Errorf("sync context dir: %w", err)
	}
	return directory.Close()
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sanitizeIdempotencyKey(key string) string {
	var b strings.Builder
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "key"
	}
	return b.String()
}
