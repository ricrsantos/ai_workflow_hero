package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
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
	// claudeDetectedImageLimit bounds best-effort tool output detection. The
	// session asset service remains responsible for validating user input.
	claudeDetectedImageLimit = 20 << 20
	maxClaudeStreamLineBytes = 10 << 20
)

var claudeToolImageTokenPattern = regexp.MustCompile(`(?i)(?:file://)?(?:[a-z]:[\\/]|/|\.{0,2}[\\/])[^\s"'<>]+\.(?:png|jpe?g|gif|webp)(?:\?[^\s"'<>]+)?`)

// ClaudeImageTransportMode identifies the proven transport selected for an
// attachment turn.
type ClaudeImageTransportMode string

const (
	ClaudeImageTransportNative          ClaudeImageTransportMode = "native-stream-json"
	ClaudeImageTransportDegradedFileRef ClaudeImageTransportMode = "degraded-file-reference"
)

// ClaudeFileReferenceOptions contains the active model capability facts for
// the explicitly labeled fallback path.
type ClaudeFileReferenceOptions struct {
	Model      string
	Capability harness.MediaCapability
}

// ClaudeImageSpikeResult records the account-free stream-json image-input
// compatibility spike. Native transport is selected only when Passed is true
// and the active model capability also advertises native image input.
type ClaudeImageSpikeResult struct {
	Passed     bool
	Mode       ClaudeImageTransportMode
	Diagnostic string
	Invocation Invocation
}

// ClaudeImageSpikeRunner is the process seam used by the fixture spike. The
// runner receives the exact invocation and NDJSON stdin that a real process
// would receive, but tests can answer without an account or network access.
type ClaudeImageSpikeRunner func(context.Context, Invocation, []byte) (ProbeResult, error)

// ClaudeAttachmentPlan is the adapter-owned translation result. Stdin is
// populated only for a proven native stream-json path; degraded plans carry a
// prompt with an explicit file-reference label.
type ClaudeAttachmentPlan struct {
	Mode       ClaudeImageTransportMode
	Prompt     string
	Invocation Invocation
	Stdin      []byte
	Diagnostic string
}

// ClaudeAttachmentOptions selects native versus degraded attachment input.
type ClaudeAttachmentOptions struct {
	WorkingDir string
	Model      string
	Capability harness.MediaCapability
	Spike      ClaudeImageSpikeResult
}

// ClaudeStreamJSONImageInvocation builds the candidate native invocation used
// by the spike and, only after the spike passes, by the native input plan.
func ClaudeStreamJSONImageInvocation(workDir, model string) Invocation {
	return Invocation{
		Args: []string{
			"-p",
			"--input-format", "stream-json",
			"--output-format", "stream-json",
			"--verbose",
			"--include-partial-messages",
			"--model", strings.TrimSpace(model),
		},
		WorkingDir:           strings.TrimSpace(workDir),
		StartNewProcessGroup: true,
	}
}

// RunClaudeStreamJSONImageSpike sends a candidate image content block through
// an injected process seam and returns a pass/fail result. A normal text-only
// stream, an exit error, or an output without an explicit image-input
// acknowledgement is a failed spike and therefore selects the degraded path.
func RunClaudeStreamJSONImageSpike(ctx context.Context, runner ClaudeImageSpikeRunner, workingDir, model string, attachment harness.Attachment) (ClaudeImageSpikeResult, error) {
	if runner == nil {
		return ClaudeImageSpikeResult{}, errors.New("Claude image spike runner is required")
	}
	invocation, input, err := BuildClaudeNativeImageInput(workingDir, model, "Hero image-input compatibility spike", []harness.Attachment{attachment})
	if err != nil {
		return ClaudeImageSpikeResult{}, err
	}
	result := ClaudeImageSpikeResult{
		Mode:       ClaudeImageTransportDegradedFileRef,
		Invocation: invocation,
	}
	probe, runErr := runner(ctx, invocation, input)
	if runErr != nil {
		result.Diagnostic = "Claude stream-json image input spike failed: process probe was unavailable; using degraded file-reference fallback"
		return result, nil
	}
	if probe.ExitCode != 0 {
		result.Diagnostic = fmt.Sprintf("Claude stream-json image input spike failed: probe exited with status %d; using degraded file-reference fallback", probe.ExitCode)
		return result, nil
	}
	if !claudeImageInputAcknowledged(probe.Stdout) {
		result.Diagnostic = "Claude stream-json image input spike failed: no explicit image-input acknowledgement was observed; using degraded file-reference fallback"
		return result, nil
	}
	result.Passed = true
	result.Mode = ClaudeImageTransportNative
	result.Diagnostic = "Claude stream-json image input spike passed: native image input is proven for this fixture"
	return result, nil
}

// BuildClaudeNativeImageInput creates the candidate user NDJSON frame with
// images before text. It is deliberately separate from Execute so the native
// path cannot be enabled until the fixture spike proves the wire shape.
func BuildClaudeNativeImageInput(workDir, model, prompt string, attachments []harness.Attachment) (Invocation, []byte, error) {
	if len(attachments) == 0 {
		return Invocation{}, nil, errors.New("Claude native image input requires at least one attachment")
	}
	content := make([]map[string]any, 0, len(attachments)+1)
	for i, attachment := range attachments {
		data, mimeType, err := readClaudeNativeAttachment(attachment)
		if err != nil {
			return Invocation{}, nil, fmt.Errorf("read Claude native image attachment %d: %w", i+1, err)
		}
		content = append(content, map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": mimeType,
				"data":       base64.StdEncoding.EncodeToString(data),
			},
		})
	}
	if strings.TrimSpace(prompt) != "" {
		content = append(content, map[string]any{"type": "text", "text": prompt})
	}
	frame := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": content,
		},
	}
	payload, err := json.Marshal(frame)
	if err != nil {
		return Invocation{}, nil, fmt.Errorf("encode Claude native image input: %w", err)
	}
	payload = append(payload, '\n')
	return ClaudeStreamJSONImageInvocation(workDir, model), payload, nil
}

// BuildClaudeAttachmentPlan selects the native path only after both the
// model capability and the fixture spike prove it. When native input is not
// proven, a supported file-reference capability gets a labeled degraded plan;
// otherwise the attachment turn fails explicitly.
func BuildClaudeAttachmentPlan(prompt string, attachments []harness.Attachment, options ClaudeAttachmentOptions) (ClaudeAttachmentPlan, error) {
	if len(attachments) == 0 {
		return ClaudeAttachmentPlan{Prompt: prompt}, nil
	}
	model := displayClaudeModel(options.Model)
	native := options.Spike.Passed && options.Capability.ImageInputNative
	if native {
		invocation, input, err := BuildClaudeNativeImageInput(options.WorkingDir, options.Model, prompt, attachments)
		if err != nil {
			return ClaudeAttachmentPlan{}, fmt.Errorf("Claude native image input for model %q: %w", model, err)
		}
		return ClaudeAttachmentPlan{
			Mode:       ClaudeImageTransportNative,
			Invocation: invocation,
			Stdin:      input,
			Diagnostic: "Claude native stream-json image input selected after a passing fixture spike",
		}, nil
	}
	if options.Capability.ImageInputFileReference {
		filePrompt, err := ComposeClaudeFileReferencePrompt(prompt, attachments, ClaudeFileReferenceOptions{Model: options.Model, Capability: options.Capability})
		if err != nil {
			return ClaudeAttachmentPlan{}, err
		}
		return ClaudeAttachmentPlan{
			Mode:       ClaudeImageTransportDegradedFileRef,
			Prompt:     filePrompt,
			Diagnostic: "Claude native stream-json image input was not proven; using degraded file-reference fallback",
		}, nil
	}
	return ClaudeAttachmentPlan{}, fmt.Errorf("Claude cannot accept image attachments for model %q: native stream-json image input is not proven and degraded file-reference input is unavailable; choose a capable model or remove the attachments", model)
}

// ComposeClaudeFileReferencePrompt composes the temporary Claude fallback.
// The label is intentional: a file reference is not native image transport,
// and callers must not present it to users as if Claude received an image
// content block.
func ComposeClaudeFileReferencePrompt(prompt string, attachments []harness.Attachment, options ClaudeFileReferenceOptions) (string, error) {
	if len(attachments) == 0 {
		return prompt, nil
	}
	model := displayClaudeModel(options.Model)
	if !options.Capability.ImageInputFileReference {
		return "", fmt.Errorf("Claude cannot read image attachments for model %q: degraded file-reference input is unavailable; native stream-json image input must be proven before sending", model)
	}
	validated := make([]harness.Attachment, len(attachments))
	for i, attachment := range attachments {
		canonical, size, err := validateClaudeAttachment(attachment, options.Capability)
		if err != nil {
			return "", fmt.Errorf("Claude cannot send image attachment %d for model %q: %w", i+1, model, err)
		}
		validated[i] = attachment
		validated[i].Path = canonical
		if validated[i].Size <= 0 {
			validated[i].Size = size
		}
	}

	var out strings.Builder
	out.WriteString("Hero image attachments (DEGRADED Claude file-reference fallback):\n")
	out.WriteString("The native Claude stream-json image-input wire format was not proven for this CLI/model. Read every listed materialized image directly from disk before answering. This is a degraded file-reference path, not a native image attachment; report any unreadable image instead of ignoring it.\n")
	for i, attachment := range validated {
		out.WriteString(fmt.Sprintf("%d. %s", i+1, attachment.Path))
		if metadata := claudeAttachmentMetadata(attachment); metadata != "" {
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

// ComposeDegradedFileReferencePrompt is a concise alias for adapter callers
// that already know they are on Claude's degraded path.
func ComposeDegradedFileReferencePrompt(prompt string, attachments []harness.Attachment, model string, capability harness.MediaCapability) (string, error) {
	return ComposeClaudeFileReferencePrompt(prompt, attachments, ClaudeFileReferenceOptions{Model: model, Capability: capability})
}

func displayClaudeModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "<unknown>"
	}
	return model
}

func validateClaudeAttachment(attachment harness.Attachment, capability harness.MediaCapability) (string, int64, error) {
	if attachment.Kind != "" && attachment.Kind != harness.MediaKindImage {
		return "", 0, fmt.Errorf("attachment kind %q is not an image", attachment.Kind)
	}
	if mimeType := strings.TrimSpace(attachment.MIMEType); mimeType != "" && !claudeMIMESupported(mimeType, capability.SupportedImageMIMETypes) {
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

func claudeMIMESupported(mimeType string, supported []string) bool {
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

func claudeAttachmentMetadata(attachment harness.Attachment) string {
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

func readClaudeNativeAttachment(attachment harness.Attachment) ([]byte, string, error) {
	path := strings.TrimSpace(attachment.Path)
	if path == "" {
		return nil, "", errors.New("materialized image path is empty")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open materialized image: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, claudeDetectedImageLimit+1))
	if err != nil {
		return nil, "", fmt.Errorf("read materialized image: %w", err)
	}
	if len(data) > claudeDetectedImageLimit {
		return nil, "", fmt.Errorf("image exceeds the %d-byte native input limit", claudeDetectedImageLimit)
	}
	mimeType := strings.TrimSpace(attachment.MIMEType)
	if !claudeMIMESupported(mimeType, nil) {
		detectedMIME, _, _, ok := claudeImageMetadata(data, path)
		if !ok {
			return nil, "", errors.New("image MIME type is missing or unsupported")
		}
		mimeType = detectedMIME
	}
	return data, mimeType, nil
}

func claudeImageInputAcknowledged(output string) bool {
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), maxClaudeStreamLineBytes)
	for scanner.Scan() {
		var event map[string]any
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		if accepted, ok := event["accepted"].(bool); ok && accepted {
			typ, _ := event["type"].(string)
			subtype, _ := event["subtype"].(string)
			label := strings.ToLower(typ + " " + subtype)
			if strings.Contains(label, "image") || strings.Contains(label, "input") {
				return true
			}
		}
		if result, ok := event["result"].(map[string]any); ok {
			if accepted, ok := result["image_input_accepted"].(bool); ok && accepted {
				return true
			}
		}
	}
	return false
}

// ClaudeWorkspaceImageWatch uses a finite before/after workspace scan for the
// degraded file-reference path. Tool-result paths are merged at turn end, so
// a path can be detected even when the filesystem mtime is coarse.
type ClaudeWorkspaceImageWatch struct {
	root     string
	fs       fs.FS
	baseline map[string]claudeImageRecord
	active   bool
}

// NewClaudeWorkspaceImageWatch creates and starts an operating-system watch.
func NewClaudeWorkspaceImageWatch(root string) (*ClaudeWorkspaceImageWatch, error) {
	return NewClaudeWorkspaceImageWatchFS(root, nil)
}

// NewClaudeWorkspaceImageWatchFS accepts an fs.FS for deterministic fixtures.
func NewClaudeWorkspaceImageWatchFS(root string, filesystem fs.FS) (*ClaudeWorkspaceImageWatch, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("Claude workspace root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve Claude workspace root: %w", err)
	}
	if filesystem == nil {
		info, statErr := os.Stat(abs)
		if statErr != nil {
			return nil, fmt.Errorf("stat Claude workspace root: %w", statErr)
		}
		if !info.IsDir() {
			return nil, errors.New("Claude workspace root is not a directory")
		}
		filesystem = os.DirFS(abs)
	}
	watch := &ClaudeWorkspaceImageWatch{root: filepath.Clean(abs), fs: filesystem}
	if err := watch.StartTurn(); err != nil {
		return nil, err
	}
	return watch, nil
}

// StartTurn resets the before-turn image snapshot.
func (w *ClaudeWorkspaceImageWatch) StartTurn() error {
	if w == nil || w.fs == nil {
		return errors.New("Claude image watch is not initialized")
	}
	snapshot, err := w.snapshot()
	if err != nil {
		return fmt.Errorf("snapshot Claude workspace images: %w", err)
	}
	w.baseline = snapshot
	w.active = true
	return nil
}

// Scan returns files created or content-modified since StartTurn.
func (w *ClaudeWorkspaceImageWatch) Scan() ([]string, error) {
	if w == nil || !w.active {
		return nil, errors.New("Claude image watch is not active")
	}
	current, err := w.snapshot()
	if err != nil {
		return nil, fmt.Errorf("scan Claude workspace images: %w", err)
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

// FinishTurn merges tool_result path extraction with the finite filesystem
// scan and returns one tool-sourced asset per content hash.
func (w *ClaudeWorkspaceImageWatch) FinishTurn(sessionID, turnID string, streamJSON []byte) ([]harness.Asset, error) {
	if w == nil || !w.active {
		return nil, errors.New("Claude image watch is not active")
	}
	paths := ExtractClaudeToolResultImagePaths(streamJSON)
	return w.FinishTurnPaths(sessionID, turnID, paths)
}

// FinishTurnPaths closes the finite watch and merges caller-collected
// tool-result paths with files changed during the turn. Callers that stream
// NDJSON can collect paths line by line, avoiding retention of raw provider
// payloads in the adapter.
func (w *ClaudeWorkspaceImageWatch) FinishTurnPaths(sessionID, turnID string, toolPaths []string) ([]harness.Asset, error) {
	if w == nil || !w.active {
		return nil, errors.New("Claude image watch is not active")
	}
	changed, err := w.Scan()
	if err != nil {
		return nil, err
	}
	w.active = false
	paths := append([]string(nil), toolPaths...)
	paths = append(paths, changed...)
	return w.assetsForPaths(sessionID, turnID, paths), nil
}

// DetectClaudeToolImageAssets is the orchestration entry point for the
// degraded Claude turn watcher.
func DetectClaudeToolImageAssets(watch *ClaudeWorkspaceImageWatch, sessionID, turnID string, streamJSON []byte) ([]harness.Asset, error) {
	if watch == nil {
		return nil, errors.New("Claude image watch is required")
	}
	return watch.FinishTurn(sessionID, turnID, streamJSON)
}

func (w *ClaudeWorkspaceImageWatch) snapshot() (map[string]claudeImageRecord, error) {
	result := make(map[string]claudeImageRecord)
	err := fs.WalkDir(w.fs, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		rel := filepath.ToSlash(filepath.Clean(name))
		if rel == "." || !looksLikeClaudeImagePath(rel) {
			return nil
		}
		record, ok, err := w.readImageRecord(rel)
		if err != nil {
			// Tool output detection is best effort and must not delay or fail
			// completion because a file is concurrently being replaced.
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

func (w *ClaudeWorkspaceImageWatch) assetsForPaths(sessionID, turnID string, paths []string) []harness.Asset {
	assets := make([]harness.Asset, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, rawPath := range paths {
		rel, ok := w.relativePath(rawPath)
		if !ok {
			continue
		}
		record, valid, err := w.readImageRecord(rel)
		if err != nil || !valid {
			continue
		}
		if _, duplicate := seen[record.Hash]; duplicate {
			continue
		}
		seen[record.Hash] = struct{}{}
		assets = append(assets, claudeAssetFromRecord(record, sessionID, turnID))
	}
	return assets
}

func (w *ClaudeWorkspaceImageWatch) readImageRecord(rel string) (claudeImageRecord, bool, error) {
	file, err := w.fs.Open(rel)
	if err != nil {
		return claudeImageRecord{}, false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, claudeDetectedImageLimit+1))
	if err != nil {
		return claudeImageRecord{}, false, err
	}
	if len(data) > claudeDetectedImageLimit {
		return claudeImageRecord{}, false, nil
	}
	mimeType, width, height, ok := claudeImageMetadata(data, rel)
	if !ok {
		return claudeImageRecord{}, false, nil
	}
	return claudeImageRecord{
		Path:     filepath.Join(w.root, filepath.FromSlash(rel)),
		Name:     filepath.Base(filepath.FromSlash(rel)),
		MIMEType: mimeType,
		Size:     int64(len(data)),
		Width:    width,
		Height:   height,
		Hash:     harness.HashBytes(data),
	}, true, nil
}

func (w *ClaudeWorkspaceImageWatch) relativePath(rawPath string) (string, bool) {
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
	if canonical, err := filepath.EvalSymlinks(candidate); err == nil {
		candidate = canonical
	}
	rel, err := filepath.Rel(w.root, candidate)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if !looksLikeClaudeImagePath(rel) {
		return "", false
	}
	return rel, true
}

type claudeImageRecord struct {
	Path     string
	Name     string
	MIMEType string
	Size     int64
	Width    int
	Height   int
	Hash     string
}

func claudeAssetFromRecord(record claudeImageRecord, sessionID, turnID string) harness.Asset {
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

func claudeImageMetadata(data []byte, path string) (string, int, int, bool) {
	if len(data) == 0 {
		return "", 0, 0, false
	}
	contentType := http.DetectContentType(data[:minClaudeInt(len(data), 512)])
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp", 0, 0, true
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", 0, 0, false
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return contentType, 0, 0, true
	}
	return contentType, config.Width, config.Height, true
}

func looksLikeClaudeImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	default:
		return false
	}
}

// ExtractClaudeToolResultImagePaths extracts image paths from Claude
// tool_result/tool-related NDJSON events in first-seen order. Assistant/user
// text outside a tool result is ignored so user attachments cannot become
// tool-sourced assets.
func ExtractClaudeToolResultImagePaths(streamJSON []byte) []string {
	if len(bytes.TrimSpace(streamJSON)) == 0 {
		return nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(streamJSON))
	scanner.Buffer(make([]byte, 0, 64*1024), maxClaudeStreamLineBytes)
	seen := make(map[string]struct{})
	var paths []string
	for scanner.Scan() {
		var event any
		if json.Unmarshal(bytes.TrimSpace(scanner.Bytes()), &event) != nil {
			continue
		}
		object, ok := event.(map[string]any)
		if !ok || !claudeToolEvent(object) {
			continue
		}
		var candidates []string
		collectClaudeToolPaths(object, "", &candidates)
		for _, candidate := range candidates {
			candidate = normalizeClaudeStreamPath(candidate)
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

// ExtractClaudeToolImagePaths is a shorter alias for callers that do not need
// to mention the exact Claude event shape.
func ExtractClaudeToolImagePaths(streamJSON []byte) []string {
	return ExtractClaudeToolResultImagePaths(streamJSON)
}

func claudeToolEvent(object map[string]any) bool {
	for _, key := range []string{"type", "subtype", "event_type"} {
		if value, ok := object[key].(string); ok {
			lower := strings.ToLower(value)
			if strings.Contains(lower, "tool") {
				return true
			}
		}
	}
	for _, key := range []string{"tool_result", "tool_use", "tool_call", "tool_output"} {
		if _, ok := object[key]; ok {
			return true
		}
	}
	return containsClaudeToolMarker(object)
}

func containsClaudeToolMarker(value any) bool {
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
			if containsClaudeToolMarker(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsClaudeToolMarker(child) {
				return true
			}
		}
	}
	return false
}

func collectClaudeToolPaths(value any, key string, paths *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for _, childKey := range orderedClaudeToolKeys(typed) {
			collectClaudeToolPaths(typed[childKey], childKey, paths)
		}
	case []any:
		for _, child := range typed {
			collectClaudeToolPaths(child, key, paths)
		}
	case string:
		if !claudePathBearingKey(key) {
			return
		}
		if claudeJSONArgumentKey(key) {
			var nested any
			if json.Unmarshal([]byte(typed), &nested) == nil {
				collectClaudeToolPaths(nested, key, paths)
				return
			}
		}
		if strings.EqualFold(key, "path") || strings.HasSuffix(strings.ToLower(key), "_path") || strings.EqualFold(key, "file") {
			if looksLikeClaudeImagePath(typed) {
				*paths = append(*paths, typed)
			}
			return
		}
		appendClaudeImageTokens(typed, paths)
	}
}

func orderedClaudeToolKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		left, right := claudeToolKeyRank(keys[i]), claudeToolKeyRank(keys[j])
		if left != right {
			return left < right
		}
		return keys[i] < keys[j]
	})
	return keys
}

func claudeToolKeyRank(key string) int {
	lower := strings.ToLower(strings.TrimSpace(key))
	if lower == "path" || strings.HasSuffix(lower, "_path") || lower == "file" {
		return 0
	}
	if strings.Contains(lower, "tool") || claudeJSONArgumentKey(lower) {
		return 1
	}
	return 2
}

func claudeJSONArgumentKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "arguments", "args", "input", "payload":
		return true
	default:
		return false
	}
}

func appendClaudeImageTokens(text string, paths *[]string) {
	for _, token := range strings.Fields(text) {
		for _, match := range claudeToolImageTokenPattern.FindAllString(token, -1) {
			*paths = append(*paths, match)
		}
		candidate := strings.Trim(token, "\"'`;:()[]{}<>")
		if looksLikeClaudeImagePath(candidate) {
			*paths = append(*paths, candidate)
		}
	}
}

func claudePathBearingKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}
	if claudeJSONArgumentKey(key) {
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

func normalizeClaudeStreamPath(value string) string {
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

func minClaudeInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
