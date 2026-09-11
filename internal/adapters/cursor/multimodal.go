package cursor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const (
	// cursorDetectedImageLimit bounds the work performed by a best-effort
	// output detector. Input validation and session materialization own the
	// user-facing attachment limits; this limit prevents a tool-written file
	// from making turn completion unbounded.
	cursorDetectedImageLimit = 20 << 20
	maxCursorStreamLineBytes = 10 << 20
)

var cursorToolImageTokenPattern = regexp.MustCompile(`(?i)(?:file://)?(?:[a-z]:[\\/]|/|\.{0,2}[\\/])[^\s"'<>]+\.(?:png|jpe?g|gif|webp)(?:\?[^\s"'<>]+)?`)

// CursorFileReferenceOptions contains the active model capability facts used
// while composing a Cursor prompt. Unknown capabilities intentionally fail
// closed because Cursor's file-reference path is the only image transport
// implemented by this adapter.
type CursorFileReferenceOptions struct {
	Model      string
	Capability harness.MediaCapability
}

// ComposeCursorFileReferencePrompt adds materialized image paths to a Cursor
// prompt. It returns an error before any prompt is sent when the active model
// cannot read file references or an attachment is not readable. Attachments
// are never silently removed.
func ComposeCursorFileReferencePrompt(prompt string, attachments []harness.Attachment, options CursorFileReferenceOptions) (string, error) {
	if len(attachments) == 0 {
		return prompt, nil
	}
	model := displayCursorModel(options.Model)
	if !options.Capability.ImageInputFileReference {
		return "", fmt.Errorf("cursor cannot read image attachments for model %q: image_input_file_reference is unavailable; choose an image-capable model or remove the attachments", model)
	}

	validated := make([]harness.Attachment, len(attachments))
	for i, attachment := range attachments {
		canonical, size, err := validateCursorAttachment(attachment, options.Capability)
		if err != nil {
			return "", fmt.Errorf("cursor cannot send image attachment %d for model %q: %w", i+1, model, err)
		}
		validated[i] = attachment
		validated[i].Path = canonical
		if validated[i].Size <= 0 {
			validated[i].Size = size
		}
	}

	var out strings.Builder
	out.WriteString("Hero image attachments (Cursor file-reference path):\n")
	out.WriteString("Read every listed materialized image before answering. These are user-provided attachments; if the active model or harness cannot read an image, report that failure instead of ignoring it.\n")
	for i, attachment := range validated {
		out.WriteString(fmt.Sprintf("%d. %s", i+1, attachment.Path))
		if metadata := cursorAttachmentMetadata(attachment); metadata != "" {
			out.WriteString(" (" + metadata + ")")
		}
		out.WriteByte('\n')
	}
	if strings.TrimSpace(prompt) != "" {
		out.WriteString("\nUser message:\n")
		out.WriteString(prompt)
	}
	return out.String(), nil
}

// ComposeFileReferencePrompt is a compact compatibility wrapper for callers
// that already have the model and capability values separately.
func ComposeFileReferencePrompt(prompt string, attachments []harness.Attachment, model string, capability harness.MediaCapability) (string, error) {
	return ComposeCursorFileReferencePrompt(prompt, attachments, CursorFileReferenceOptions{Model: model, Capability: capability})
}

func displayCursorModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "<unknown>"
	}
	return model
}

func validateCursorAttachment(attachment harness.Attachment, capability harness.MediaCapability) (string, int64, error) {
	if attachment.Kind != "" && attachment.Kind != harness.MediaKindImage {
		return "", 0, fmt.Errorf("attachment kind %q is not an image", attachment.Kind)
	}
	if mimeType := strings.TrimSpace(attachment.MIMEType); mimeType != "" && !cursorMIMESupported(mimeType, capability.SupportedImageMIMETypes) {
		return "", 0, fmt.Errorf("MIME type %q is not supported by the file-reference capability", mimeType)
	}
	path := strings.TrimSpace(attachment.Path)
	if path == "" {
		return "", 0, errors.New("materialized image path is empty")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", 0, fmt.Errorf("resolve materialized image path: %w", err)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", 0, fmt.Errorf("resolve materialized image path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", 0, fmt.Errorf("stat materialized image: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", 0, errors.New("materialized image path is not a regular file")
	}
	if capability.MaxAttachmentBytes > 0 && info.Size() > capability.MaxAttachmentBytes {
		return "", 0, fmt.Errorf("image is %d bytes but the model limit is %d bytes", info.Size(), capability.MaxAttachmentBytes)
	}
	file, err := os.Open(abs)
	if err != nil {
		return "", 0, fmt.Errorf("open materialized image: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", 0, fmt.Errorf("close materialized image: %w", err)
	}
	return abs, info.Size(), nil
}

func cursorMIMESupported(mimeType string, supported []string) bool {
	if len(supported) == 0 {
		return strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "image/")
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	for _, candidate := range supported {
		if mimeType == strings.ToLower(strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func cursorAttachmentMetadata(attachment harness.Attachment) string {
	var values []string
	if mimeType := strings.TrimSpace(attachment.MIMEType); mimeType != "" {
		values = append(values, mimeType)
	}
	if attachment.Width > 0 && attachment.Height > 0 {
		values = append(values, fmt.Sprintf("%dx%d", attachment.Width, attachment.Height))
	}
	if attachment.Size > 0 {
		values = append(values, fmt.Sprintf("%d bytes", attachment.Size))
	}
	return strings.Join(values, ", ")
}

// CursorWorkspaceImageWatch takes one bounded snapshot at the start of a
// turn and compares it with one bounded snapshot at turn completion. This is
// deliberately finite: a tool cannot keep Hero waiting on an unbounded file
// watcher after the harness has finished.
type CursorWorkspaceImageWatch struct {
	root     string
	fs       fs.FS
	baseline map[string]cursorImageRecord
	active   bool
}

// NewCursorWorkspaceImageWatch creates a watch over an operating-system
// workspace and captures the pre-turn image snapshot immediately.
func NewCursorWorkspaceImageWatch(root string) (*CursorWorkspaceImageWatch, error) {
	return NewCursorWorkspaceImageWatchFS(root, nil)
}

// NewCursorWorkspaceImageWatchFS is the fixture-friendly constructor. When
// filesystem is nil, the real workspace directory is used through os.DirFS.
func NewCursorWorkspaceImageWatchFS(root string, filesystem fs.FS) (*CursorWorkspaceImageWatch, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("cursor workspace root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve cursor workspace root: %w", err)
	}
	if filesystem == nil {
		info, statErr := os.Stat(abs)
		if statErr != nil {
			return nil, fmt.Errorf("stat cursor workspace root: %w", statErr)
		}
		if !info.IsDir() {
			return nil, errors.New("cursor workspace root is not a directory")
		}
		filesystem = os.DirFS(abs)
	}
	watch := &CursorWorkspaceImageWatch{root: filepath.Clean(abs), fs: filesystem}
	if err := watch.StartTurn(); err != nil {
		return nil, err
	}
	return watch, nil
}

// StartTurn resets the watch baseline. Call it before each harness turn when
// a watcher is reused.
func (w *CursorWorkspaceImageWatch) StartTurn() error {
	if w == nil || w.fs == nil {
		return errors.New("cursor image watch is not initialized")
	}
	snapshot, err := w.snapshot()
	if err != nil {
		return fmt.Errorf("snapshot cursor workspace images: %w", err)
	}
	w.baseline = snapshot
	w.active = true
	return nil
}

// Scan returns image paths whose content differs from the start-of-turn
// snapshot. The result is sorted by workspace-relative path for stable card
// ordering.
func (w *CursorWorkspaceImageWatch) Scan() ([]string, error) {
	if w == nil || !w.active {
		return nil, errors.New("cursor image watch is not active")
	}
	current, err := w.snapshot()
	if err != nil {
		return nil, fmt.Errorf("scan cursor workspace images: %w", err)
	}
	paths := make([]string, 0)
	for rel, record := range current {
		before, existed := w.baseline[rel]
		if !existed || before.Hash != record.Hash {
			paths = append(paths, record.Path)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// FinishTurn combines changed workspace files with image paths mentioned in
// Cursor stream-json tool events. Explicit tool paths are retained even when a
// file existed before the turn, because the tool result is authoritative that
// it wrote or produced the file. Assets are deduplicated by content hash.
func (w *CursorWorkspaceImageWatch) FinishTurn(sessionID, turnID string, streamJSON []byte) ([]harness.Asset, error) {
	if w == nil || !w.active {
		return nil, errors.New("cursor image watch is not active")
	}
	return w.FinishTurnPaths(sessionID, turnID, ExtractCursorToolImagePaths(streamJSON))
}

// FinishTurnPaths closes the finite watch using paths collected while an
// NDJSON stream was consumed. It avoids requiring the adapter to retain raw
// provider payloads until process completion.
func (w *CursorWorkspaceImageWatch) FinishTurnPaths(sessionID, turnID string, toolPaths []string) ([]harness.Asset, error) {
	if w == nil || !w.active {
		return nil, errors.New("cursor image watch is not active")
	}
	changed, err := w.Scan()
	if err != nil {
		return nil, err
	}
	w.active = false

	paths := append([]string(nil), toolPaths...)
	paths = append(paths, changed...)
	return w.assetsForPaths(sessionID, turnID, paths)
}

// DetectCursorToolImageAssets is a small functional entry point for adapter
// orchestration code that owns the turn-scoped watcher.
func DetectCursorToolImageAssets(watch *CursorWorkspaceImageWatch, sessionID, turnID string, streamJSON []byte) ([]harness.Asset, error) {
	if watch == nil {
		return nil, errors.New("cursor image watch is required")
	}
	return watch.FinishTurn(sessionID, turnID, streamJSON)
}

func (w *CursorWorkspaceImageWatch) snapshot() (map[string]cursorImageRecord, error) {
	result := make(map[string]cursorImageRecord)
	err := fs.WalkDir(w.fs, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		rel := filepath.ToSlash(filepath.Clean(name))
		if rel == "." || !looksLikeCursorImagePath(rel) {
			return nil
		}
		record, ok, err := w.readImageRecord(rel)
		if err != nil {
			// Detection is best effort. A file that disappears or is still being
			// written must not turn a completed harness turn into a failure.
			return nil
		}
		if ok {
			result[rel] = record
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (w *CursorWorkspaceImageWatch) assetsForPaths(sessionID, turnID string, paths []string) ([]harness.Asset, error) {
	assets := make([]harness.Asset, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, rawPath := range paths {
		rel, ok := w.relativePath(rawPath)
		if !ok {
			continue
		}
		record, valid, err := w.readImageRecord(rel)
		if err != nil {
			continue
		}
		if !valid {
			continue
		}
		if _, duplicate := seen[record.Hash]; duplicate {
			continue
		}
		seen[record.Hash] = struct{}{}
		assets = append(assets, cursorAssetFromRecord(record, sessionID, turnID))
	}
	return assets, nil
}

func (w *CursorWorkspaceImageWatch) readImageRecord(rel string) (cursorImageRecord, bool, error) {
	file, err := w.fs.Open(rel)
	if err != nil {
		return cursorImageRecord{}, false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, cursorDetectedImageLimit+1))
	if err != nil {
		return cursorImageRecord{}, false, err
	}
	if len(data) > cursorDetectedImageLimit {
		return cursorImageRecord{}, false, nil
	}
	mimeType, width, height, ok := cursorImageMetadata(data, rel)
	if !ok {
		return cursorImageRecord{}, false, nil
	}
	return cursorImageRecord{
		Path:     filepath.Join(w.root, filepath.FromSlash(rel)),
		Name:     filepath.Base(filepath.FromSlash(rel)),
		MIMEType: mimeType,
		Size:     int64(len(data)),
		Width:    width,
		Height:   height,
		Hash:     harness.HashBytes(data),
	}, true, nil
}

func (w *CursorWorkspaceImageWatch) relativePath(rawPath string) (string, bool) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", false
	}
	if parsed, err := url.Parse(rawPath); err == nil && strings.EqualFold(parsed.Scheme, "file") {
		rawPath = parsed.Path
		if decoded, err := url.PathUnescape(rawPath); err == nil {
			rawPath = decoded
		}
	}

	var candidate string
	if filepath.IsAbs(rawPath) {
		candidate = filepath.Clean(rawPath)
	} else {
		candidate = filepath.Join(w.root, filepath.FromSlash(rawPath))
	}
	if filepath.IsAbs(candidate) {
		if canonical, err := filepath.EvalSymlinks(candidate); err == nil {
			candidate = canonical
		}
	}
	rel, err := filepath.Rel(w.root, candidate)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if !looksLikeCursorImagePath(rel) {
		return "", false
	}
	return rel, true
}

type cursorImageRecord struct {
	Path     string
	Name     string
	MIMEType string
	Size     int64
	Width    int
	Height   int
	Hash     string
}

func cursorAssetFromRecord(record cursorImageRecord, sessionID, turnID string) harness.Asset {
	return harness.Asset{
		Attachment: harness.Attachment{
			ID:          "tool-" + record.Hash,
			Kind:        harness.MediaKindImage,
			Name:        record.Name,
			MIMEType:    record.MIMEType,
			Path:        record.Path,
			Size:        record.Size,
			Width:       record.Width,
			Height:      record.Height,
			ContentHash: record.Hash,
		},
		Source:    harness.AssetSourceTool,
		SessionID: sessionID,
		TurnID:    turnID,
		Saved:     true,
	}
}

func cursorImageMetadata(data []byte, path string) (string, int, int, bool) {
	if len(data) == 0 {
		return "", 0, 0, false
	}
	contentType := http.DetectContentType(data[:minInt(len(data), 512)])
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp", 0, 0, true
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", 0, 0, false
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		// The magic-byte detector is still useful while a tool is finishing a
		// write; preserve the file as an image asset with unknown dimensions.
		return contentType, 0, 0, true
	}
	return contentType, config.Width, config.Height, true
}

func looksLikeCursorImagePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	default:
		return false
	}
}

// ExtractCursorToolImagePaths extracts image paths from Cursor tool-related
// stream-json events. It preserves first-seen order and ignores user/assistant
// text so an old attachment cannot be reclassified as a tool output.
func ExtractCursorToolImagePaths(streamJSON []byte) []string {
	if len(bytes.TrimSpace(streamJSON)) == 0 {
		return nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(streamJSON))
	scanner.Buffer(make([]byte, 0, 64*1024), maxCursorStreamLineBytes)
	seen := make(map[string]struct{})
	var paths []string
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event any
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		object, ok := event.(map[string]any)
		if !ok || !cursorToolEvent(object) {
			continue
		}
		var candidates []string
		collectCursorToolPaths(object, "", &candidates)
		for _, candidate := range candidates {
			candidate = normalizeCursorStreamPath(candidate)
			if candidate == "" {
				continue
			}
			key := filepath.Clean(candidate)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			paths = append(paths, candidate)
		}
	}
	return paths
}

// ExtractImagePathsFromStreamJSON is an adapter-neutral spelling retained for
// orchestration code that selects a Cursor parser by stream format.
func ExtractImagePathsFromStreamJSON(streamJSON []byte) []string {
	return ExtractCursorToolImagePaths(streamJSON)
}

func cursorToolEvent(object map[string]any) bool {
	for _, key := range []string{"type", "subtype", "event_type"} {
		if value, ok := object[key].(string); ok && strings.Contains(strings.ToLower(value), "tool") {
			return true
		}
	}
	for _, key := range []string{"tool_call", "tool_result", "tool_use", "tool_output"} {
		if _, ok := object[key]; ok {
			return true
		}
	}
	return containsCursorToolMarker(object)
}

func containsCursorToolMarker(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "tool_result") || strings.Contains(lower, "tool_use") || strings.Contains(lower, "tool_call") {
				return true
			}
			if (lower == "type" || lower == "subtype" || lower == "event_type") && strings.Contains(strings.ToLower(fmt.Sprint(child)), "tool") {
				return true
			}
			if containsCursorToolMarker(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsCursorToolMarker(child) {
				return true
			}
		}
	}
	return false
}

func collectCursorToolPaths(value any, key string, paths *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for _, childKey := range orderedCursorToolKeys(typed) {
			collectCursorToolPaths(typed[childKey], childKey, paths)
		}
	case []any:
		for _, child := range typed {
			collectCursorToolPaths(child, key, paths)
		}
	case string:
		if !cursorPathBearingKey(key) {
			return
		}
		if cursorJSONArgumentKey(key) {
			var nested any
			if json.Unmarshal([]byte(typed), &nested) == nil {
				collectCursorToolPaths(nested, key, paths)
				return
			}
		}
		if strings.EqualFold(key, "path") || strings.HasSuffix(strings.ToLower(key), "_path") || strings.EqualFold(key, "file") {
			if looksLikeCursorImagePath(typed) {
				*paths = append(*paths, typed)
			}
			return
		}
		appendCursorImageTokens(typed, paths)
	}
}

func orderedCursorToolKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		left, right := cursorToolKeyRank(keys[i]), cursorToolKeyRank(keys[j])
		if left != right {
			return left < right
		}
		return keys[i] < keys[j]
	})
	return keys
}

func cursorToolKeyRank(key string) int {
	lower := strings.ToLower(strings.TrimSpace(key))
	if lower == "path" || strings.HasSuffix(lower, "_path") || lower == "file" {
		return 0
	}
	if strings.Contains(lower, "tool") || cursorJSONArgumentKey(lower) {
		return 1
	}
	return 2
}

func cursorJSONArgumentKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "arguments", "args", "input", "payload":
		return true
	default:
		return false
	}
}

func appendCursorImageTokens(text string, paths *[]string) {
	for _, token := range strings.Fields(text) {
		for _, match := range cursorToolImageTokenPattern.FindAllString(token, -1) {
			*paths = append(*paths, match)
		}
		candidate := strings.Trim(token, "\"'`;:()[]{}<>")
		if looksLikeCursorImagePath(candidate) {
			*paths = append(*paths, candidate)
		}
	}
}

func cursorPathBearingKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}
	if cursorJSONArgumentKey(key) {
		return true
	}
	if strings.Contains(key, "path") || strings.Contains(key, "file") || strings.Contains(key, "image") {
		return true
	}
	switch key {
	case "content", "text", "message", "output", "stdout", "stderr", "result", "command":
		return true
	default:
		return false
	}
}

func normalizeCursorStreamPath(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'`;:()[]{}<>")
	if parsed, err := url.Parse(value); err == nil && strings.EqualFold(parsed.Scheme, "file") {
		value = parsed.Path
		if decoded, err := url.PathUnescape(value); err == nil {
			value = decoded
		}
	}
	return strings.TrimSpace(value)
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
