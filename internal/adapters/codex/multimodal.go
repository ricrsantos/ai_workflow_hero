package codex

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const (
	// CodexLocalImageInputType is the app-server input union member for a
	// materialized image. It is intentionally kept separate from the inline
	// image variant: localImage does not put bytes or data URLs on the wire.
	CodexLocalImageInputType = "localImage"
	CodexImageInputType      = "image"

	CodexImageGenerationItemType = "imageGeneration"
	CodexImageViewItemType       = "imageView"

	maxCodexMetadataText = 400
	maxCodexIdentifier   = 160
)

var codexSensitivePathPattern = regexp.MustCompile(`(^|[\s"'(])((?:/|[A-Za-z]:[\\/])[^ \t\r\n"'<>)]*)`)

var (
	// ErrCodexMultimodalDegraded identifies a safe, explicit fallback boundary.
	// Callers can surface the diagnostic without treating an unknown app-server
	// schema as permission to silently remove attachments.
	ErrCodexMultimodalDegraded = errors.New("codex multimodal support degraded")

	codexImageMIMEs = map[string]string{
		"gif":  "image/gif",
		"jpeg": "image/jpeg",
		"jpg":  "image/jpeg",
		"png":  "image/png",
		"webp": "image/webp",
	}
)

// CodexMultimodalSchema records the image union members advertised by an
// installed Codex app-server schema. A zero value is deliberately not
// optimistic: schema absence means that native attachment support is unknown.
type CodexMultimodalSchema struct {
	SchemaPresent bool

	InputLocalImage bool
	InputImage      bool

	OutputImageGeneration bool
	OutputImageView       bool

	// Degraded is true when the schema was absent or did not describe all
	// multimodal members needed by this adapter. Diagnostic is safe to display;
	// it never contains the raw schema or local image paths.
	Degraded   bool
	Diagnostic string
}

// CodexSchemaProbe is kept as a short alias for callers that do not need the
// longer type name.
type CodexSchemaProbe = CodexMultimodalSchema

// ProbeCodexAppServerSchema inspects an initialize/schema fixture without
// assuming one exact Codex release layout. Codex has moved capability details
// between nested maps and generated union schemas, so known discriminators are
// recognized wherever they occur in the JSON tree.
func ProbeCodexAppServerSchema(raw json.RawMessage) CodexMultimodalSchema {
	var schema CodexMultimodalSchema
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		schema.Degraded = true
		schema.Diagnostic = "Codex app-server schema is absent; multimodal support is degraded"
		return schema
	}

	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		schema.Degraded = true
		schema.Diagnostic = "Codex app-server schema could not be read; multimodal support is degraded"
		return schema
	}
	if !nonEmptyJSONValue(value) {
		schema.Degraded = true
		schema.Diagnostic = "Codex app-server schema is absent; multimodal support is degraded"
		return schema
	}
	schema.SchemaPresent = true

	var walk func(any, []string)
	walk = func(node any, path []string) {
		switch typed := node.(type) {
		case map[string]any:
			for key, child := range typed {
				normalizedKey := compactCodexToken(key)
				// Generated JSON schemas may expose a discriminator as a
				// property whose value is another schema object rather than a
				// boolean. The key itself is therefore meaningful regardless of
				// its child shape.
				if normalizedKey == "localimage" {
					schema.InputLocalImage = true
				}
				if normalizedKey == "image" && (boolValue(child) || pathSuggestsInput(path)) {
					schema.InputImage = true
				}
				if normalizedKey == "imagegeneration" {
					schema.OutputImageGeneration = true
				}
				if normalizedKey == "imageview" {
					schema.OutputImageView = true
				}
				walk(child, append(path, normalizedKey))
			}
		case []any:
			for _, child := range typed {
				walk(child, path)
			}
		case string:
			token := compactCodexToken(typed)
			switch token {
			case "localimage":
				schema.InputLocalImage = true
			case "imagegeneration":
				schema.OutputImageGeneration = true
			case "imageview":
				schema.OutputImageView = true
			case "image":
				if pathSuggestsInput(path) || lastCodexPathToken(path) == "type" || lastCodexPathToken(path) == "enum" {
					schema.InputImage = true
				}
			}
		}
	}
	walk(value, nil)

	missing := make([]string, 0, 3)
	if !schema.InputLocalImage {
		missing = append(missing, "localImage input")
	}
	if !schema.OutputImageGeneration {
		missing = append(missing, "imageGeneration output")
	}
	if !schema.OutputImageView {
		missing = append(missing, "imageView output")
	}
	if len(missing) > 0 {
		schema.Degraded = true
		schema.Diagnostic = "Codex app-server schema does not advertise " + strings.Join(missing, ", ")
	}
	return schema
}

// ProbeCodexSchema is the map/value-friendly form of ProbeCodexAppServerSchema.
// It is useful for fake app-server peers that already decoded JSON fixtures.
func ProbeCodexSchema(value any) CodexMultimodalSchema {
	raw, err := json.Marshal(value)
	if err != nil {
		return CodexMultimodalSchema{
			Degraded:   true,
			Diagnostic: "Codex app-server schema could not be encoded; multimodal support is degraded",
		}
	}
	return ProbeCodexAppServerSchema(raw)
}

// SupportsNativeInput reports whether a local materialized attachment can be
// sent without an inline data URL or a text-only fallback.
func (s CodexMultimodalSchema) SupportsNativeInput() bool {
	return s.SchemaPresent && s.InputLocalImage
}

// SupportsImageOutput reports whether at least one native image output item is
// described by the probed schema.
func (s CodexMultimodalSchema) SupportsImageOutput() bool {
	return s.SchemaPresent && (s.OutputImageGeneration || s.OutputImageView)
}

// CodexMultimodalDegradedError is actionable without exposing a raw schema,
// user path, image bytes, or a provider error payload.
type CodexMultimodalDegradedError struct {
	Model  string
	Reason string
}

func (e *CodexMultimodalDegradedError) Error() string {
	if e == nil {
		return ErrCodexMultimodalDegraded.Error()
	}
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		reason = "the installed app-server schema does not expose localImage"
	}
	model := strings.TrimSpace(e.Model)
	if model == "" {
		return fmt.Sprintf("%s: %s", ErrCodexMultimodalDegraded, reason)
	}
	return fmt.Sprintf("%s for model %q: %s", ErrCodexMultimodalDegraded, sanitizeIdentifier(model), reason)
}

func (e *CodexMultimodalDegradedError) Unwrap() error { return ErrCodexMultimodalDegraded }

// CodexInputItem is the JSON shape accepted by turn/start.input for the
// multimodal subset used by Hero. Images are deliberately before text.
type CodexInputItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Path string `json:"path,omitempty"`
}

// WireMap returns the compact map form used by the existing turn/start
// parameter builder.
func (i CodexInputItem) WireMap() map[string]string {
	item := map[string]string{"type": i.Type}
	switch i.Type {
	case CodexLocalImageInputType:
		item["path"] = i.Path
	case CodexImageInputType:
		// The adapter currently sends validated local paths only. Keep the URL
		// field available for future schema-wired inline-image support.
		if i.Path != "" {
			item["url"] = i.Path
		}
	default:
		if i.Text != "" {
			item["text"] = i.Text
		}
	}
	return item
}

// BuildCodexTurnInput maps validated local attachments to ordered native
// input items. It fails closed when localImage is not advertised; the
// attachment slice is never silently discarded.
func BuildCodexTurnInput(schema CodexMultimodalSchema, attachments []harness.Attachment, prompt string) ([]CodexInputItem, error) {
	if len(attachments) > 0 && !schema.SupportsNativeInput() {
		reason := schema.Diagnostic
		if reason == "" {
			reason = "the installed app-server schema does not expose localImage"
		}
		return nil, &CodexMultimodalDegradedError{Reason: reason}
	}

	items := make([]CodexInputItem, 0, len(attachments)+1)
	for index, attachment := range attachments {
		if attachment.Kind != "" && attachment.Kind != harness.MediaKindImage {
			return nil, fmt.Errorf("codex attachment %d: unsupported media kind %q", index, attachment.Kind)
		}
		path, err := codexLocalImagePath(attachment.Path)
		if err != nil {
			return nil, fmt.Errorf("codex attachment %d: %w", index, err)
		}
		items = append(items, CodexInputItem{Type: CodexLocalImageInputType, Path: path})
	}
	if strings.TrimSpace(prompt) != "" {
		items = append(items, CodexInputItem{Type: "text", Text: prompt})
	}
	return items, nil
}

// BuildCodexTurnStartInput returns the map representation that can be put
// directly in turn/start params["input"].
func BuildCodexTurnStartInput(schema CodexMultimodalSchema, attachments []harness.Attachment, prompt string) ([]map[string]string, error) {
	items, err := BuildCodexTurnInput(schema, attachments, prompt)
	if err != nil {
		return nil, err
	}
	wire := make([]map[string]string, 0, len(items))
	for _, item := range items {
		wire = append(wire, item.WireMap())
	}
	return wire, nil
}

// CodexTurnInputs is the small legacy-compatible constructor used by the
// current turn/start builder. It only shapes already-admitted attachments;
// schema-aware callers must use BuildCodexTurnStartInput so an absent schema
// produces an explicit degradation error before this compatibility helper is
// reached.
func CodexTurnInputs(attachments []harness.Attachment, prompt string) ([]map[string]string, error) {
	return BuildCodexTurnStartInput(CodexMultimodalSchema{
		SchemaPresent:   true,
		InputLocalImage: true,
	}, attachments, prompt)
}

// CodexAssetBatch is the adapter-neutral result of normalizing one stream or
// final app-server payload. Diagnostics are bounded and safe to display.
type CodexAssetBatch struct {
	Assets      []harness.Asset
	Deltas      []harness.StreamDelta
	Diagnostics []string
	Degraded    bool
}

// NormalizeCodexNotification accepts either an item notification, a final
// turn wrapper, or a fixture containing an items array. It normalizes native
// image items and tool-written image paths without requiring a live process.
func NormalizeCodexNotification(method string, raw json.RawMessage, sessionID, turnID, baseDir string) (CodexAssetBatch, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return CodexAssetBatch{}, fmt.Errorf("decode Codex multimodal payload: %w", err)
	}
	candidates, diagnostics := collectCodexAssetCandidates(method, value, sessionID, turnID, baseDir)
	assets := make([]harness.Asset, 0, len(candidates))
	phases := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		assets = append(assets, candidate.asset)
		phases = append(phases, candidate.phase)
	}
	assets, phases = dedupeCodexCandidates(assets, phases)
	deltas := make([]harness.StreamDelta, 0, len(assets))
	for index, asset := range assets {
		phase := ""
		if index < len(phases) {
			phase = phases[index]
		}
		deltas = append(deltas, CodexAssetStreamDelta(asset, method, phase))
	}
	return CodexAssetBatch{
		Assets:      assets,
		Deltas:      deltas,
		Diagnostics: uniqueDiagnostics(diagnostics),
		Degraded:    len(diagnostics) > 0,
	}, nil
}

// NormalizeCodexOutput is a semantic alias for callers processing the final
// turn payload rather than an individual notification.
func NormalizeCodexOutput(method string, raw json.RawMessage, sessionID, turnID, baseDir string) (CodexAssetBatch, error) {
	return NormalizeCodexNotification(method, raw, sessionID, turnID, baseDir)
}

// NormalizeCodexImageItem normalizes the first imageGeneration/imageView item
// in raw. A metadata-only asset is returned for a failed generation with no
// saved path so safe status/error information is not lost.
func NormalizeCodexImageItem(raw json.RawMessage, sessionID, turnID, baseDir string) (harness.Asset, error) {
	batch, err := NormalizeCodexNotification("item/completed", raw, sessionID, turnID, baseDir)
	if err != nil {
		return harness.Asset{}, err
	}
	if len(batch.Assets) == 0 {
		return harness.Asset{}, errors.New("Codex payload contains no image item")
	}
	return batch.Assets[0], nil
}

// NormalizeCodexToolWrittenImages extracts tool-result/file-change paths and
// normalizes each readable image as a tool-sourced asset.
func NormalizeCodexToolWrittenImages(raw json.RawMessage, sessionID, turnID, baseDir string) ([]harness.Asset, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode Codex tool payload: %w", err)
	}
	paths := extractCodexToolImagePaths(value, baseDir)
	itemID := sanitizeIdentifier(firstString(value, "id", "itemId", "item_id", "callId", "call_id"))
	assets := make([]harness.Asset, 0, len(paths))
	for _, path := range paths {
		asset, err := normalizeCodexImagePath(path, codexImageMetadata{
			ID:     itemID,
			Source: harness.AssetSourceTool,
		}, sessionID, turnID, baseDir)
		if err != nil {
			continue
		}
		assets = append(assets, asset)
	}
	return DedupeCodexAssets(assets), nil
}

// ExtractCodexToolWrittenImagePaths returns candidate image paths from
// structured tool/file items. Callers that need assets should prefer
// NormalizeCodexToolWrittenImages, which also verifies magic bytes and hashes.
func ExtractCodexToolWrittenImagePaths(raw json.RawMessage, baseDir string) ([]string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode Codex tool payload: %w", err)
	}
	return extractCodexToolImagePaths(value, baseDir), nil
}

// CodexAssetStreamDelta creates the shared asset stream event. The pointer is
// a copy of the immutable value so a caller may safely reuse its Asset slice.
func CodexAssetStreamDelta(asset harness.Asset, harnessType, phase string) harness.StreamDelta {
	assetCopy := asset
	metadata := map[string]string{}
	if asset.Source != "" {
		metadata["source"] = string(asset.Source)
	}
	if asset.Status != "" {
		metadata["status"] = asset.Status
	}
	return harness.StreamDelta{
		Kind:        harness.StreamKindAsset,
		HarnessType: strings.TrimSpace(harnessType),
		SessionID:   asset.SessionID,
		Phase:       phase,
		Metadata:    metadata,
		Asset:       &assetCopy,
	}
}

// RepairCodexAssets merges streamed and final assets in first-seen order.
// Final metadata repairs a partial stream asset; equal content hashes never
// produce a duplicate card, even when the paths or native item IDs differ.
func RepairCodexAssets(streamed, final []harness.Asset) []harness.Asset {
	result := make([]harness.Asset, 0, len(streamed)+len(final))
	for _, asset := range streamed {
		appendOrMergeCodexAsset(&result, asset)
	}
	for _, asset := range final {
		appendOrMergeCodexAsset(&result, asset)
	}
	return result
}

// MergeCodexAssets is an alternate name for the stream/final repair operation.
func MergeCodexAssets(streamed, final []harness.Asset) []harness.Asset {
	return RepairCodexAssets(streamed, final)
}

// DedupeCodexAssets applies the same content-hash/identity rules to one batch.
func DedupeCodexAssets(assets []harness.Asset) []harness.Asset {
	result := make([]harness.Asset, 0, len(assets))
	for _, asset := range assets {
		appendOrMergeCodexAsset(&result, asset)
	}
	return result
}

// HashCodexImage computes a SHA-256 digest after verifying that path contains
// a supported image signature. It is intentionally independent of the session
// asset store so adapters can deduplicate tool paths before parent wiring.
func HashCodexImage(path string) (string, error) {
	info, err := inspectCodexImage(path, "")
	if err != nil {
		return "", err
	}
	return info.contentHash, nil
}

type codexAssetCandidate struct {
	asset harness.Asset
	phase string
}

type codexImageMetadata struct {
	ID            string
	Name          string
	MIMEType      string
	Status        string
	RevisedPrompt string
	SafeError     string
	Source        harness.AssetSource
	Width         int
	Height        int
	Size          int64
}

type codexImageFileInfo struct {
	path        string
	name        string
	mimeType    string
	size        int64
	width       int
	height      int
	contentHash string
}

func collectCodexAssetCandidates(method string, value any, sessionID, turnID, baseDir string) ([]codexAssetCandidate, []string) {
	var candidates []codexAssetCandidate
	var diagnostics []string
	var walk func(any, string, string, string)
	walk = func(node any, currentSession, currentTurn, currentBase string) {
		switch typed := node.(type) {
		case []any:
			for _, child := range typed {
				walk(child, currentSession, currentTurn, currentBase)
			}
		case map[string]any:
			if value, ok := typed["sessionId"]; ok {
				currentSession = firstNonEmpty(stringValue(value), currentSession)
			}
			if value, ok := typed["session_id"]; ok {
				currentSession = firstNonEmpty(stringValue(value), currentSession)
			}
			if value, ok := typed["threadId"]; ok {
				currentSession = firstNonEmpty(stringValue(value), currentSession)
			}
			if value, ok := typed["thread_id"]; ok {
				currentSession = firstNonEmpty(stringValue(value), currentSession)
			}
			if value, ok := typed["turnId"]; ok {
				currentTurn = firstNonEmpty(stringValue(value), currentTurn)
			}
			if value, ok := typed["turn_id"]; ok {
				currentTurn = firstNonEmpty(stringValue(value), currentTurn)
			}
			if cwd := firstString(typed, "cwd", "workingDirectory", "working_directory"); cwd != "" {
				if resolved, err := codexPath(cwd, currentBase); err == nil {
					currentBase = resolved
				}
			}

			if item, ok := typed["item"]; ok {
				walk(item, currentSession, currentTurn, currentBase)
				return
			}
			if turn, ok := typed["turn"]; ok {
				walk(turn, currentSession, currentTurn, currentBase)
				return
			}
			if items, ok := typed["items"]; ok {
				walk(items, currentSession, currentTurn, currentBase)
				return
			}

			typ := compactCodexToken(firstString(typed, "type", "itemType", "item_type", "kind"))
			if typ == "" || typ == "extension" {
				typ = compactCodexToken(firstString(typed, "name"))
			}
			switch typ {
			case "imagegeneration", "imagegenerationitem", "imagegenerationcall":
				asset, diagnostic := normalizeCodexImageMap(typed, harness.AssetSourceModel, currentSession, currentTurn, currentBase)
				if diagnostic != "" {
					diagnostics = append(diagnostics, diagnostic)
				}
				candidates = append(candidates, codexAssetCandidate{asset: asset, phase: codexAssetPhase(method, asset.Status)})
			case "imageview", "imageviewitem":
				asset, diagnostic := normalizeCodexImageMap(typed, harness.AssetSourceTool, currentSession, currentTurn, currentBase)
				if diagnostic != "" {
					diagnostics = append(diagnostics, diagnostic)
				}
				candidates = append(candidates, codexAssetCandidate{asset: asset, phase: codexAssetPhase(method, asset.Status)})
			default:
				if isCodexToolItem(typ, typed) {
					paths := extractCodexToolImagePaths(typed, currentBase)
					itemID := sanitizeIdentifier(firstString(typed, "id", "itemId", "item_id", "callId", "call_id"))
					for _, path := range paths {
						asset, err := normalizeCodexImagePath(path, codexImageMetadata{
							ID:     itemID,
							Source: harness.AssetSourceTool,
						}, currentSession, currentTurn, currentBase)
						if err != nil {
							diagnostics = append(diagnostics, safeCodexDiagnostic(err))
							continue
						}
						candidates = append(candidates, codexAssetCandidate{asset: asset, phase: codexAssetPhase(method, asset.Status)})
					}
				}
			}
		}
	}
	walk(value, strings.TrimSpace(sessionID), strings.TrimSpace(turnID), strings.TrimSpace(baseDir))
	return candidates, diagnostics
}

func normalizeCodexImageMap(item map[string]any, source harness.AssetSource, sessionID, turnID, baseDir string) (harness.Asset, string) {
	metadata := codexImageMetadata{
		ID:            sanitizeIdentifier(firstString(item, "id", "itemId", "item_id", "callId", "call_id")),
		Name:          safeMetadataName(firstString(item, "name", "filename", "fileName", "file_name")),
		MIMEType:      safeImageMIME(firstString(item, "mimeType", "mime_type", "mime")),
		Status:        safeMetadataText(firstString(item, "status", "state")),
		RevisedPrompt: safeMetadataText(firstString(item, "revisedPrompt", "revised_prompt")),
		SafeError:     safeCodexError(item),
		Source:        source,
		Width:         boundedDimension(firstNumber(item, "width")),
		Height:        boundedDimension(firstNumber(item, "height")),
		Size:          boundedSize(firstNumber(item, "size", "sizeBytes", "size_bytes")),
	}
	path := firstString(item, "savedPath", "saved_path", "path", "filePath", "file_path", "imagePath", "image_path")
	asset, err := normalizeCodexImagePath(path, metadata, sessionID, turnID, baseDir)
	if err == nil {
		return asset, ""
	}
	if metadata.SafeError == "" {
		metadata.SafeError = safeCodexDiagnostic(err)
	}
	if path == "" {
		metadata.SafeError = firstNonEmpty(metadata.SafeError, "Codex image item did not provide a saved path")
	}
	return metadataOnlyCodexAsset(metadata, sessionID, turnID, path), safeCodexDiagnostic(err)
}

func normalizeCodexImagePath(rawPath string, metadata codexImageMetadata, sessionID, turnID, baseDir string) (harness.Asset, error) {
	metadata.Source = firstAssetSource(metadata.Source, harness.AssetSourceModel)
	path := strings.TrimSpace(rawPath)
	if path == "" {
		asset := metadataOnlyCodexAsset(metadata, sessionID, turnID, "")
		if asset.SafeError == "" {
			asset.SafeError = "Codex image item did not provide a saved path"
		}
		return asset, errors.New(asset.SafeError)
	}
	resolved, err := codexPath(path, baseDir)
	if err != nil {
		return harness.Asset{}, err
	}
	info, err := inspectCodexImage(resolved, baseDir)
	if err != nil {
		return harness.Asset{}, err
	}

	name := metadata.Name
	if name == "" {
		name = info.name
	}
	mimeType := info.mimeType
	if mimeType == "" {
		mimeType = metadata.MIMEType
	}
	asset := harness.Asset{
		Attachment: harness.Attachment{
			ID:          firstNonEmpty(metadata.ID, "codex:image:"+info.contentHash[:minInt(16, len(info.contentHash))]),
			Kind:        harness.MediaKindImage,
			Name:        name,
			MIMEType:    mimeType,
			Path:        info.path,
			Size:        info.size,
			Width:       info.width,
			Height:      info.height,
			ContentHash: info.contentHash,
		},
		Source:        metadata.Source,
		SessionID:     strings.TrimSpace(sessionID),
		TurnID:        strings.TrimSpace(turnID),
		Saved:         true,
		Status:        metadata.Status,
		RevisedPrompt: metadata.RevisedPrompt,
		SafeError:     metadata.SafeError,
	}
	return asset, nil
}

func metadataOnlyCodexAsset(metadata codexImageMetadata, sessionID, turnID, rawPath string) harness.Asset {
	name := metadata.Name
	if name == "" {
		name = safeMetadataName(filepath.Base(strings.TrimSpace(rawPath)))
	}
	asset := harness.Asset{
		Attachment: harness.Attachment{
			ID:       metadata.ID,
			Kind:     harness.MediaKindImage,
			Name:     name,
			MIMEType: metadata.MIMEType,
			Path:     safeAssetPath(rawPath),
			Size:     metadata.Size,
			Width:    metadata.Width,
			Height:   metadata.Height,
		},
		Source:        firstAssetSource(metadata.Source, harness.AssetSourceModel),
		SessionID:     strings.TrimSpace(sessionID),
		TurnID:        strings.TrimSpace(turnID),
		Saved:         false,
		Status:        metadata.Status,
		RevisedPrompt: metadata.RevisedPrompt,
		SafeError:     metadata.SafeError,
	}
	if asset.ID == "" {
		asset.ID = firstNonEmpty("codex:image:"+sanitizeIdentifier(name), "codex:image")
	}
	return asset
}

func inspectCodexImage(rawPath, baseDir string) (codexImageFileInfo, error) {
	path, err := codexPath(rawPath, baseDir)
	if err != nil {
		return codexImageFileInfo{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return codexImageFileInfo{}, fmt.Errorf("Codex image %q cannot be read: %w", safeMetadataName(filepath.Base(path)), err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return codexImageFileInfo{}, fmt.Errorf("Codex image metadata unavailable: %w", err)
	}
	if !stat.Mode().IsRegular() {
		return codexImageFileInfo{}, fmt.Errorf("Codex image %q is not a regular file", safeMetadataName(filepath.Base(path)))
	}
	header := make([]byte, 512)
	n, readErr := file.Read(header)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return codexImageFileInfo{}, fmt.Errorf("Codex image header unavailable: %w", readErr)
	}
	header = header[:n]
	mimeType := codexImageMIMEFromHeader(header)
	if mimeType == "" {
		return codexImageFileInfo{}, fmt.Errorf("Codex output is not a supported image")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return codexImageFileInfo{}, fmt.Errorf("Codex image cannot be hashed: %w", err)
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return codexImageFileInfo{}, fmt.Errorf("Codex image cannot be hashed: %w", err)
	}
	contentHash := hex.EncodeToString(digest.Sum(nil))
	width, height := codexImageDimensions(path, mimeType, header)
	return codexImageFileInfo{
		path:        path,
		name:        safeMetadataName(filepath.Base(path)),
		mimeType:    mimeType,
		size:        stat.Size(),
		width:       width,
		height:      height,
		contentHash: contentHash,
	}, nil
}

func codexImageDimensions(path, mimeType string, header []byte) (int, int) {
	switch mimeType {
	case "image/png":
		if len(header) >= 24 {
			return int(readUint32BE(header[16:20])), int(readUint32BE(header[20:24]))
		}
	case "image/gif":
		if len(header) >= 10 {
			return int(readUint16LE(header[6:8])), int(readUint16LE(header[8:10]))
		}
	case "image/webp":
		if width, height, ok := webpDimensions(header); ok {
			return width, height
		}
	case "image/jpeg":
		file, err := os.Open(path)
		if err == nil {
			defer file.Close()
			if config, _, err := image.DecodeConfig(file); err == nil {
				return config.Width, config.Height
			}
		}
	}
	return 0, 0
}

func webpDimensions(header []byte) (int, int, bool) {
	if len(header) < 30 || string(header[0:4]) != "RIFF" || string(header[8:12]) != "WEBP" {
		return 0, 0, false
	}
	chunk := string(header[12:16])
	switch chunk {
	case "VP8X":
		if len(header) < 30 {
			return 0, 0, false
		}
		width := 1 + int(header[24]) + int(header[25])<<8 + int(header[26])<<16
		height := 1 + int(header[27]) + int(header[28])<<8 + int(header[29])<<16
		return width, height, true
	case "VP8 ":
		if len(header) >= 30 && header[23] == 0x9d && header[24] == 0x01 && header[25] == 0x2a {
			return int(readUint16LE(header[26:28]) & 0x3fff), int(readUint16LE(header[28:30]) & 0x3fff), true
		}
	}
	return 0, 0, false
}

func codexImageMIMEFromHeader(header []byte) string {
	switch {
	case len(header) >= 8 && bytes.Equal(header[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return "image/png"
	case len(header) >= 3 && header[0] == 0xff && header[1] == 0xd8 && header[2] == 0xff:
		return "image/jpeg"
	case len(header) >= 6 && (bytes.Equal(header[:6], []byte("GIF87a")) || bytes.Equal(header[:6], []byte("GIF89a"))):
		return "image/gif"
	case len(header) >= 12 && bytes.Equal(header[:4], []byte("RIFF")) && bytes.Equal(header[8:12], []byte("WEBP")):
		return "image/webp"
	default:
		return ""
	}
}

func codexLocalImagePath(rawPath string) (string, error) {
	path, err := codexPath(rawPath, "")
	if err != nil {
		return "", fmt.Errorf("localImage path is invalid: %w", err)
	}
	return path, nil
}

func codexPath(rawPath, baseDir string) (string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return "", errors.New("image path is empty")
	}
	if strings.IndexByte(path, 0) >= 0 {
		return "", errors.New("image path contains a NUL byte")
	}
	if parsed, err := url.Parse(path); err == nil && parsed.Scheme != "" {
		if !strings.EqualFold(parsed.Scheme, "file") {
			return "", errors.New("image path must be local")
		}
		path = parsed.Path
		if decoded, err := url.PathUnescape(path); err == nil {
			path = decoded
		}
	}
	path = strings.Trim(path, "\"'`")
	if !filepath.IsAbs(path) {
		if strings.TrimSpace(baseDir) != "" {
			base, err := filepath.Abs(baseDir)
			if err != nil {
				return "", fmt.Errorf("image base directory is invalid: %w", err)
			}
			path = filepath.Join(base, path)
		} else {
			absolute, err := filepath.Abs(path)
			if err != nil {
				return "", fmt.Errorf("image path is invalid: %w", err)
			}
			path = absolute
		}
	}
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = filepath.Clean(resolved)
	}
	return path, nil
}

func extractCodexToolImagePaths(value any, baseDir string) []string {
	paths := make([]string, 0)
	seen := make(map[string]struct{})
	var walk func(any, string, string)
	walk = func(node any, currentBase, parentKey string) {
		switch typed := node.(type) {
		case []any:
			for _, child := range typed {
				walk(child, currentBase, parentKey)
			}
		case map[string]any:
			if cwd := firstString(typed, "cwd", "workingDirectory", "working_directory"); cwd != "" {
				if resolved, err := codexPath(cwd, currentBase); err == nil {
					currentBase = resolved
				}
			}
			for key, child := range typed {
				normalizedKey := compactCodexToken(key)
				if stringValue, ok := child.(string); ok {
					if isCodexPathKey(normalizedKey) || parentKey == "changes" || looksLikeCodexImagePath(stringValue) {
						addCodexCandidatePath(stringValue, currentBase, seen, &paths)
					}
					if normalizedKey == "command" || normalizedKey == "stdout" || normalizedKey == "stderr" || normalizedKey == "aggregatedoutput" || normalizedKey == "result" {
						for _, candidate := range codexImagePathsInText(stringValue) {
							addCodexCandidatePath(candidate, currentBase, seen, &paths)
						}
					}
					if (strings.HasPrefix(strings.TrimSpace(stringValue), "{") || strings.HasPrefix(strings.TrimSpace(stringValue), "[")) && json.Valid([]byte(stringValue)) {
						var nested any
						if json.Unmarshal([]byte(stringValue), &nested) == nil {
							walk(nested, currentBase, normalizedKey)
						}
					}
				}
				if normalizedKey == "changes" {
					if changes, ok := child.(map[string]any); ok {
						for path := range changes {
							if looksLikeCodexImagePath(path) {
								addCodexCandidatePath(path, currentBase, seen, &paths)
							}
						}
					}
				}
				walk(child, currentBase, normalizedKey)
			}
		case string:
			for _, candidate := range codexImagePathsInText(typed) {
				addCodexCandidatePath(candidate, currentBase, seen, &paths)
			}
		}
	}
	walk(value, strings.TrimSpace(baseDir), "")
	return paths
}

func addCodexCandidatePath(rawPath, baseDir string, seen map[string]struct{}, paths *[]string) {
	path := strings.Trim(strings.TrimSpace(rawPath), "\"'`()[]{}<>,;:")
	if !looksLikeCodexImagePath(path) {
		return
	}
	resolved, err := codexPath(path, baseDir)
	if err != nil {
		return
	}
	key := resolved
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*paths = append(*paths, resolved)
}

func codexImagePathsInText(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("\"'`()[]{}<>,;", r)
	})
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if looksLikeCodexImagePath(field) {
			paths = append(paths, field)
		}
	}
	return paths
}

func isCodexToolItem(typ string, item map[string]any) bool {
	if strings.Contains(typ, "tool") || strings.Contains(typ, "command") || strings.Contains(typ, "filechange") || strings.Contains(typ, "shell") || strings.Contains(typ, "functioncall") {
		return true
	}
	for key := range item {
		normalized := compactCodexToken(key)
		if isCodexPathKey(normalized) || normalized == "changes" || normalized == "command" || normalized == "result" {
			return true
		}
	}
	return false
}

func isCodexPathKey(key string) bool {
	switch compactCodexToken(key) {
	case "path", "savedpath", "filepath", "filename", "imagepath", "outputpath", "artifactpath", "targetpath", "destinationpath", "outputfile":
		return true
	default:
		return false
	}
}

func looksLikeCodexImagePath(value string) bool {
	value = strings.Trim(strings.TrimSpace(value), "\"'`()[]{}<>,;:")
	if value == "" || strings.Contains(value, "data:") {
		return false
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && strings.EqualFold(parsed.Scheme, "file") {
		value = parsed.Path
	}
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(value)), ".")
	_, ok := codexImageMIMEs[extension]
	return ok
}

func appendOrMergeCodexAsset(result *[]harness.Asset, asset harness.Asset) {
	for index := range *result {
		if codexAssetsMatch((*result)[index], asset) {
			(*result)[index] = mergeCodexAsset((*result)[index], asset)
			return
		}
	}
	*result = append(*result, asset)
}

func codexAssetsMatch(left, right harness.Asset) bool {
	leftHash := strings.ToLower(strings.TrimSpace(harness.ContentHash(left)))
	rightHash := strings.ToLower(strings.TrimSpace(harness.ContentHash(right)))
	if leftHash != "" && rightHash != "" {
		return leftHash == rightHash
	}
	leftID := strings.TrimSpace(left.ID)
	rightID := strings.TrimSpace(right.ID)
	if leftID != "" && rightID != "" && leftID == rightID {
		return true
	}
	leftPath := strings.TrimSpace(left.Path)
	rightPath := strings.TrimSpace(right.Path)
	return leftPath != "" && rightPath != "" && filepath.Clean(leftPath) == filepath.Clean(rightPath)
}

func mergeCodexAsset(primary, secondary harness.Asset) harness.Asset {
	merged := primary
	if merged.ID == "" {
		merged.ID = secondary.ID
	}
	if merged.Kind == "" {
		merged.Kind = secondary.Kind
	}
	if merged.Name == "" {
		merged.Name = secondary.Name
	}
	if merged.MIMEType == "" {
		merged.MIMEType = secondary.MIMEType
	}
	if merged.Path == "" {
		merged.Path = secondary.Path
	}
	if merged.Size == 0 {
		merged.Size = secondary.Size
	}
	if merged.Width == 0 {
		merged.Width = secondary.Width
	}
	if merged.Height == 0 {
		merged.Height = secondary.Height
	}
	if merged.ContentHash == "" {
		merged.ContentHash = secondary.ContentHash
	}
	if merged.SessionID == "" {
		merged.SessionID = secondary.SessionID
	}
	if merged.TurnID == "" {
		merged.TurnID = secondary.TurnID
	}
	if merged.Status == "" || codexStatusRank(secondary.Status) > codexStatusRank(merged.Status) {
		merged.Status = secondary.Status
	}
	if merged.RevisedPrompt == "" {
		merged.RevisedPrompt = secondary.RevisedPrompt
	}
	if merged.SafeError == "" {
		merged.SafeError = secondary.SafeError
	}
	merged.Saved = merged.Saved || secondary.Saved
	if codexSourceRank(secondary.Source) > codexSourceRank(merged.Source) {
		merged.Source = secondary.Source
	}
	return merged
}

func dedupeCodexCandidates(assets []harness.Asset, phases []string) ([]harness.Asset, []string) {
	result := make([]harness.Asset, 0, len(assets))
	resultPhases := make([]string, 0, len(assets))
	for index, asset := range assets {
		phase := ""
		if index < len(phases) {
			phase = phases[index]
		}
		found := -1
		for existingIndex := range result {
			if codexAssetsMatch(result[existingIndex], asset) {
				found = existingIndex
				break
			}
		}
		if found < 0 {
			result = append(result, asset)
			resultPhases = append(resultPhases, phase)
			continue
		}
		result[found] = mergeCodexAsset(result[found], asset)
		if resultPhases[found] == "" {
			resultPhases[found] = phase
		}
	}
	return result, resultPhases
}

func codexAssetPhase(method, status string) string {
	method = strings.ToLower(strings.TrimSpace(method))
	status = strings.ToLower(strings.TrimSpace(status))
	if strings.Contains(method, "started") || status == "inprogress" || status == "generating" {
		return harness.StreamPhaseStarted
	}
	if strings.Contains(method, "completed") || status == "completed" || status == "failed" {
		return harness.StreamPhaseCompleted
	}
	return ""
}

func safeCodexError(item map[string]any) string {
	for _, key := range []string{"error", "failure", "safeError", "safe_error", "errorMessage", "error_message", "message", "detail"} {
		if value, ok := item[key]; ok {
			if message := safeCodexErrorValue(value); message != "" {
				return message
			}
		}
	}
	return ""
}

func safeCodexErrorValue(value any) string {
	switch typed := value.(type) {
	case string:
		return safeMetadataText(typed)
	case map[string]any:
		for _, key := range []string{"message", "reason", "detail", "description", "code"} {
			if nested, ok := typed[key]; ok {
				if message := safeCodexErrorValue(nested); message != "" {
					return message
				}
			}
		}
	case []any:
		for _, nested := range typed {
			if message := safeCodexErrorValue(nested); message != "" {
				return message
			}
		}
	}
	return ""
}

func safeCodexDiagnostic(err error) string {
	if err == nil {
		return ""
	}
	return safeMetadataText(err.Error())
}

func safeMetadataText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = codexSensitivePathPattern.ReplaceAllString(value, `${1}[path]`)
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) {
			builder.WriteByte(' ')
			continue
		}
		builder.WriteRune(r)
	}
	value = strings.Join(strings.Fields(builder.String()), " ")
	if looksLikeEncodedImageData(value) {
		return "[redacted image data]"
	}
	if len(value) > maxCodexMetadataText {
		value = value[:maxCodexMetadataText] + "…"
	}
	return value
}

func safeMetadataName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = filepath.Base(strings.ReplaceAll(value, "\\", "/"))
	value = safeMetadataText(value)
	if value == "." || value == string(filepath.Separator) {
		return ""
	}
	return value
}

func safeImageMIME(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(value, "image/") {
		for _, allowed := range codexImageMIMEs {
			if value == allowed {
				return value
			}
		}
	}
	return ""
}

func safeAssetPath(value string) string {
	value = strings.TrimSpace(value)
	if strings.IndexByte(value, 0) >= 0 {
		return ""
	}
	return value
}

func sanitizeIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._:-", r) {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
		if builder.Len() >= maxCodexIdentifier {
			break
		}
	}
	return strings.Trim(builder.String(), "._:-")
}

func looksLikeEncodedImageData(value string) bool {
	if len(value) < 128 || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	for _, r := range value {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || strings.ContainsRune("+/=_-", r)) {
			return false
		}
	}
	return true
}

func compactCodexToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func pathSuggestsInput(path []string) bool {
	for _, token := range path {
		switch compactCodexToken(token) {
		case "input", "inputs", "turnstart", "turninput", "userinput", "content", "capabilities", "supportedinput":
			return true
		}
	}
	return false
}

func lastCodexPathToken(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return compactCodexToken(path[len(path)-1])
}

func nonEmptyJSONValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case map[string]any:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	default:
		return true
	}
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}

func firstString(value any, keys ...string) string {
	if typed, ok := value.(map[string]any); ok {
		for _, key := range keys {
			if candidate, ok := typed[key]; ok {
				if result := stringValue(candidate); result != "" {
					return result
				}
			}
		}
	}
	return ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func firstNumber(value map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if candidate, ok := value[key]; ok {
			switch typed := candidate.(type) {
			case float64:
				return typed
			case json.Number:
				if parsed, err := typed.Float64(); err == nil {
					return parsed
				}
			case string:
				if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
					return parsed
				}
			}
		}
	}
	return 0
}

func boundedDimension(value float64) int {
	if value <= 0 || value > 100_000 {
		return 0
	}
	return int(value)
}

func boundedSize(value float64) int64 {
	if value <= 0 || value > float64(^uint64(0)>>1) {
		return 0
	}
	return int64(value)
}

func firstAssetSource(primary, fallback harness.AssetSource) harness.AssetSource {
	if primary != "" {
		return primary
	}
	return fallback
}

func codexSourceRank(source harness.AssetSource) int {
	switch source {
	case harness.AssetSourceTool:
		return 3
	case harness.AssetSourceModel:
		return 2
	case harness.AssetSourceUser:
		return 1
	default:
		return 0
	}
}

func codexStatusRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "success", "succeeded":
		return 3
	case "failed", "error":
		return 2
	case "inprogress", "generating", "started":
		return 1
	default:
		return 0
	}
}

func uniqueDiagnostics(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = safeMetadataText(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func readUint16LE(value []byte) uint16 {
	if len(value) < 2 {
		return 0
	}
	return uint16(value[0]) | uint16(value[1])<<8
}

func readUint32BE(value []byte) uint32 {
	if len(value) < 4 {
		return 0
	}
	return uint32(value[0])<<24 | uint32(value[1])<<16 | uint32(value[2])<<8 | uint32(value[3])
}
