package opencode

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const (
	openCodeDefaultMIME = "application/octet-stream"
	openCodeAssetIDSize = 16
)

// OpenCodeInputPart is the protocol-neutral shape of an OpenCode prompt part.
// OpenCode expects image/file parts before the optional text part. The adapter
// can marshal this value directly into the /message or /prompt_async payload.
type OpenCodeInputPart struct {
	Type     string `json:"type"`
	MIME     string `json:"mime,omitempty"`
	URL      string `json:"url,omitempty"`
	Filename string `json:"filename,omitempty"`
	Text     string `json:"text,omitempty"`
}

// OpenCodeInputOptions supplies filesystem operations for prompt translation.
// FileReadable models whether the OpenCode serve process can read a path; the
// local ReadFile function is used only for the data-URI fallback.
type OpenCodeInputOptions struct {
	FileReadable func(path string) bool
	ReadFile     func(path string) ([]byte, error)
	Limits       OpenCodeLimits
}

// OpenCodeLimits contains provider/model image-input constraints discovered
// from OpenCode metadata. A zero limit means that OpenCode did not advertise
// that particular constraint. ImageInputKnown distinguishes an explicit
// unsupported value from missing capability metadata.
type OpenCodeLimits struct {
	ProviderID              string
	ModelID                 string
	MaxAttachments          int
	MaxAttachmentBytes      int64
	MaxTotalAttachmentBytes int64
	SupportedImageMIMETypes []string
	ImageInputKnown         bool
	ImageInputSupported     bool
}

// OpenCodeAssetMaterializer stores decoded output bytes and returns the
// immutable local path that should be placed in a harness.Asset. It is
// injected so adapter tests never need a real session store or provider.
type OpenCodeAssetMaterializer func(name, mimeType string, data []byte) (string, error)

// OpenCodeAssetNormalizerOptions configures SSE asset normalization.
// ReadFile and Materialize are injectable; when omitted, ReadFile uses the
// local filesystem and data-only parts use a private temporary file.
type OpenCodeAssetNormalizerOptions struct {
	SessionID    string
	TurnID       string
	WorkspaceDir string
	ReadFile     func(path string) ([]byte, error)
	Materialize  OpenCodeAssetMaterializer
}

// OpenCodeAssetResult contains newly discovered assets and their stream
// deltas. FinalAssets returns the complete deduplicated turn result.
type OpenCodeAssetResult struct {
	Assets []harness.Asset
	Deltas []harness.StreamDelta
}

// OpenCodeAssetNormalizer converts OpenCode SSE file/image and tool-result
// parts into immutable harness assets. It is deliberately event-driven: a
// file.watcher event can complete a pending tool-result path without starting
// an unbounded filesystem watcher or delaying turn completion.
type OpenCodeAssetNormalizer struct {
	options OpenCodeAssetNormalizerOptions
	seen    map[string]int
	pending map[string]openCodeAssetCandidate
	assets  []harness.Asset
}

// NewOpenCodeAssetNormalizer creates a turn-scoped OpenCode asset normalizer.
func NewOpenCodeAssetNormalizer(options OpenCodeAssetNormalizerOptions) *OpenCodeAssetNormalizer {
	return &OpenCodeAssetNormalizer{
		options: options,
		seen:    make(map[string]int),
		pending: make(map[string]openCodeAssetCandidate),
	}
}

// BuildOpenCodeInputParts validates limits and creates ordered OpenCode parts.
// A readable absolute path is sent as a file: URI. If the serve host cannot
// read that path, the bytes are read locally and sent as a data: URI instead.
func BuildOpenCodeInputParts(req harness.ExecuteRequest, text string, options OpenCodeInputOptions) ([]OpenCodeInputPart, error) {
	if err := ValidateOpenCodeAttachmentLimits(req.Model, req.Attachments, options.Limits); err != nil {
		return nil, err
	}

	readFile := options.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	fileReadable := options.FileReadable
	if fileReadable == nil {
		fileReadable = defaultOpenCodeFileReadable
	}

	parts := make([]OpenCodeInputPart, 0, len(req.Attachments)+1)
	admitted := append([]harness.Attachment(nil), req.Attachments...)
	for index, attachment := range req.Attachments {
		path, err := absoluteOpenCodePath(attachment.Path)
		if err != nil {
			return nil, fmt.Errorf("opencode image attachment %d: %w", index, err)
		}
		part := OpenCodeInputPart{
			Type:     "file",
			MIME:     normalizedOpenCodeMIME(attachment.MIMEType, path),
			Filename: strings.TrimSpace(attachment.Name),
		}
		if fileReadable(path) {
			if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
				admitted[index].Size = info.Size()
			}
			part.URL, err = openCodeFileURI(path)
			if err != nil {
				return nil, fmt.Errorf("opencode image attachment %d: %w", index, err)
			}
		} else {
			data, readErr := readFile(path)
			if readErr != nil {
				return nil, fmt.Errorf("opencode image attachment %d is not readable by serve and data fallback failed: %w", index, readErr)
			}
			admitted[index].Size = int64(len(data))
			part.URL = openCodeDataURI(part.MIME, data)
		}
		parts = append(parts, part)
	}
	if err := ValidateOpenCodeAttachmentLimits(req.Model, admitted, options.Limits); err != nil {
		return nil, err
	}

	// An image-only turn is valid. When text is present, it is deliberately
	// appended after every image to preserve the Free Chat ordering contract.
	if text != "" {
		parts = append(parts, OpenCodeInputPart{Type: "text", Text: text})
	}
	return parts, nil
}

// BuildOpenCodePromptBody creates a complete OpenCode prompt payload while
// keeping provider-specific part construction inside the adapter package.
func BuildOpenCodePromptBody(req harness.ExecuteRequest, text string, options OpenCodeInputOptions) ([]byte, error) {
	parts, err := BuildOpenCodeInputParts(req, text, options)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"parts": parts}
	if model := modelPayload(strings.TrimSpace(req.Model)); model != nil {
		payload["model"] = model
	}
	if optionsPayload := nativePropertyOptions(req.Properties); optionsPayload != nil {
		payload["options"] = optionsPayload
	}
	if strings.TrimSpace(req.AgentName) != "" {
		payload["agent"] = req.AgentName
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal opencode multimodal prompt: %w", err)
	}
	return body, nil
}

// ValidateOpenCodeAttachmentLimits applies the selected OpenCode provider /
// model limits. Unknown limits are left unconstrained; capability admission
// remains responsible for treating unknown image support as unsupported.
func ValidateOpenCodeAttachmentLimits(model string, attachments []harness.Attachment, limits OpenCodeLimits) error {
	model = strings.TrimSpace(model)
	if model == "" {
		model = strings.TrimSpace(limits.ModelID)
	}
	if model == "" {
		model = "unknown"
	}
	if limits.ImageInputKnown && !limits.ImageInputSupported && len(attachments) > 0 {
		return fmt.Errorf("opencode model %q does not support image input", model)
	}
	if limits.MaxAttachments > 0 && len(attachments) > limits.MaxAttachments {
		return fmt.Errorf("opencode model %q accepts at most %d image attachments (got %d)", model, limits.MaxAttachments, len(attachments))
	}
	var total int64
	for index, attachment := range attachments {
		if attachment.Size < 0 {
			return fmt.Errorf("opencode model %q received invalid image attachment %d size", model, index)
		}
		if limits.MaxAttachmentBytes > 0 && attachment.Size > limits.MaxAttachmentBytes {
			return fmt.Errorf("opencode model %q accepts image attachments up to %d bytes (attachment %d is %d bytes)", model, limits.MaxAttachmentBytes, index, attachment.Size)
		}
		total += attachment.Size
		if len(limits.SupportedImageMIMETypes) > 0 && !openCodeMIMEAllowed(attachment.MIMEType, limits.SupportedImageMIMETypes) {
			return fmt.Errorf("opencode model %q does not accept image MIME type %q (attachment %d)", model, attachment.MIMEType, index)
		}
	}
	if limits.MaxTotalAttachmentBytes > 0 && total > limits.MaxTotalAttachmentBytes {
		return fmt.Errorf("opencode model %q accepts image attachments totaling at most %d bytes (got %d)", model, limits.MaxTotalAttachmentBytes, total)
	}
	return nil
}

// ValidateOpenCodeModelLimits is an explicit alias for callers performing
// admission by model rather than by the more general attachment name.
func ValidateOpenCodeModelLimits(model string, attachments []harness.Attachment, limits OpenCodeLimits) error {
	return ValidateOpenCodeAttachmentLimits(model, attachments, limits)
}

// MergeOpenCodeLimits combines provider and model metadata, retaining the
// stricter known numeric limit and the intersection of known MIME types.
func MergeOpenCodeLimits(provider, model OpenCodeLimits) OpenCodeLimits {
	merged := OpenCodeLimits{
		ProviderID:              firstNonEmpty(model.ProviderID, provider.ProviderID),
		ModelID:                 firstNonEmpty(model.ModelID, provider.ModelID),
		MaxAttachments:          minimumPositiveInt(provider.MaxAttachments, model.MaxAttachments),
		MaxAttachmentBytes:      minimumPositiveInt64(provider.MaxAttachmentBytes, model.MaxAttachmentBytes),
		MaxTotalAttachmentBytes: minimumPositiveInt64(provider.MaxTotalAttachmentBytes, model.MaxTotalAttachmentBytes),
		SupportedImageMIMETypes: mergeOpenCodeMIMEs(provider.SupportedImageMIMETypes, model.SupportedImageMIMETypes),
	}
	merged.ImageInputKnown = provider.ImageInputKnown || model.ImageInputKnown
	merged.ImageInputSupported = true
	if provider.ImageInputKnown {
		merged.ImageInputSupported = merged.ImageInputSupported && provider.ImageInputSupported
	}
	if model.ImageInputKnown {
		merged.ImageInputSupported = merged.ImageInputSupported && model.ImageInputSupported
	}
	return merged
}

// ParseOpenCodeLimits reads the provider/model response returned by an
// OpenCode catalog endpoint. It accepts the current providers[]/models map
// shape and also tolerates a direct provider or model fixture.
func ParseOpenCodeLimits(data []byte, providerID, modelID string) (OpenCodeLimits, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return OpenCodeLimits{}, fmt.Errorf("decode opencode provider/model limits: %w", err)
	}
	return OpenCodeLimitsFromMetadata(root, providerID, modelID)
}

// OpenCodeLimitsFromMetadata extracts provider/model constraints from decoded
// OpenCode catalog metadata. Missing fields intentionally remain unknown.
func OpenCodeLimitsFromMetadata(root map[string]any, providerID, modelID string) (OpenCodeLimits, error) {
	providerID = strings.TrimSpace(providerID)
	modelID = strings.TrimSpace(modelID)
	if slash := strings.Index(modelID, "/"); slash >= 0 {
		if providerID == "" {
			providerID = strings.TrimSpace(modelID[:slash])
		}
		modelID = strings.TrimSpace(modelID[slash+1:])
	}
	if root == nil {
		return OpenCodeLimits{}, errors.New("opencode provider/model limits metadata is empty")
	}

	providerMeta := findOpenCodeProvider(root, providerID)
	modelMeta := findOpenCodeModel(providerMeta, modelID)
	if modelMeta == nil {
		modelMeta = findOpenCodeModel(root, modelID)
	}
	providerLimits := parseOpenCodeLimitMap(providerMeta)
	modelLimits := parseOpenCodeLimitMap(modelMeta)
	providerLimits.ProviderID = providerID
	modelLimits.ProviderID = providerID
	modelLimits.ModelID = modelID
	return MergeOpenCodeLimits(providerLimits, modelLimits), nil
}

// ConsumeSSEEvent normalizes any file/image part or tool-result path in one
// OpenCode event. Newly materialized assets are returned in stream order.
func (n *OpenCodeAssetNormalizer) ConsumeSSEEvent(evt map[string]any) (OpenCodeAssetResult, error) {
	if n == nil {
		return OpenCodeAssetResult{}, errors.New("nil opencode asset normalizer")
	}
	eventType := normalizeVersionedEventType(evtTypeString(evt))
	ctx := openCodeCandidateContext{source: harness.AssetSourceModel}
	lowerType := strings.ToLower(eventType)
	if strings.Contains(lowerType, "tool") || strings.HasPrefix(lowerType, "file.watcher") {
		ctx.source = harness.AssetSourceTool
	}
	var candidates []openCodeAssetCandidate
	collectOpenCodeCandidates(evt, ctx, &candidates)
	candidateSeen := make(map[string]struct{}, len(candidates))
	before := len(n.assets)
	for _, candidate := range candidates {
		if key := openCodeCandidateKey(candidate, n.options.WorkspaceDir); key != "" {
			if _, duplicate := candidateSeen[key]; duplicate {
				continue
			}
			candidateSeen[key] = struct{}{}
		}
		if candidate.sessionID == "" {
			candidate.sessionID = strings.TrimSpace(n.options.SessionID)
		}
		if candidate.turnID == "" {
			candidate.turnID = strings.TrimSpace(n.options.TurnID)
		}
		asset, ready, err := n.assetFromCandidate(candidate)
		if err != nil {
			return OpenCodeAssetResult{}, err
		}
		if !ready {
			continue
		}
		n.addAsset(asset, eventType)
	}
	return n.newAssetResult(before, eventType), nil
}

// ConsumeSSEEventResult is the result-returning variant of ConsumeSSEEvent.
// It exists to keep the mutation-oriented method small while providing a
// convenient stream/final API to adapters.
func (n *OpenCodeAssetNormalizer) ConsumeSSEEventResult(evt map[string]any) (OpenCodeAssetResult, error) {
	return n.ConsumeSSEEvent(evt)
}

func (n *OpenCodeAssetNormalizer) newAssetResult(before int, eventType string) OpenCodeAssetResult {
	if before >= len(n.assets) {
		return OpenCodeAssetResult{}
	}
	result := OpenCodeAssetResult{Assets: append([]harness.Asset(nil), n.assets[before:]...)}
	for index := before; index < len(n.assets); index++ {
		asset := n.assets[index]
		assetCopy := asset
		result.Deltas = append(result.Deltas, harness.StreamDelta{
			Kind:        harness.StreamKindAsset,
			Asset:       &assetCopy,
			HarnessType: eventType,
			SessionID:   asset.SessionID,
		})
	}
	return result
}

// FinalAssets returns the complete ordered, path/hash-deduplicated result for
// the turn. The returned slice does not alias normalizer state.
func (n *OpenCodeAssetNormalizer) FinalAssets() []harness.Asset {
	if n == nil {
		return nil
	}
	return append([]harness.Asset(nil), n.assets...)
}

// NormalizeOpenCodeSSEEvents processes fake or real decoded SSE events and
// returns both stream asset deltas and the repaired final asset list.
func NormalizeOpenCodeSSEEvents(events []map[string]any, options OpenCodeAssetNormalizerOptions) (OpenCodeAssetResult, error) {
	normalizer := NewOpenCodeAssetNormalizer(options)
	var deltas []harness.StreamDelta
	for _, event := range events {
		result, err := normalizer.ConsumeSSEEventResult(event)
		if err != nil {
			return OpenCodeAssetResult{}, err
		}
		deltas = append(deltas, result.Deltas...)
	}
	return OpenCodeAssetResult{
		Assets: normalizer.FinalAssets(),
		Deltas: deltas,
	}, nil
}

// DeduplicateOpenCodeAssets removes repeated output cards by content hash or
// canonical path while preserving first-seen order. A later tool observation
// upgrades a duplicate's source to tool without emitting another card.
func DeduplicateOpenCodeAssets(assets []harness.Asset) []harness.Asset {
	result := make([]harness.Asset, 0, len(assets))
	seen := make(map[string]int, len(assets)*2)
	for _, asset := range assets {
		keys := openCodeAssetKeys(asset)
		index := -1
		for _, key := range keys {
			if existing, ok := seen[key]; ok {
				index = existing
				break
			}
		}
		if index < 0 {
			index = len(result)
			result = append(result, asset)
		}
		for _, key := range keys {
			seen[key] = index
		}
		if asset.Source == harness.AssetSourceTool && result[index].Source != harness.AssetSourceTool {
			result[index].Source = harness.AssetSourceTool
		}
	}
	return result
}

func (n *OpenCodeAssetNormalizer) addAsset(asset harness.Asset, eventType string) {
	keys := openCodeAssetKeys(asset)
	index := -1
	for _, key := range keys {
		if existing, ok := n.seen[key]; ok {
			index = existing
			break
		}
	}
	if index >= 0 {
		if asset.Source == harness.AssetSourceTool && n.assets[index].Source != harness.AssetSourceTool {
			n.assets[index].Source = harness.AssetSourceTool
		}
		for _, key := range keys {
			n.seen[key] = index
		}
		return
	}
	index = len(n.assets)
	n.assets = append(n.assets, asset)
	for _, key := range keys {
		n.seen[key] = index
	}
	_ = eventType
}

func (n *OpenCodeAssetNormalizer) assetFromCandidate(candidate openCodeAssetCandidate) (harness.Asset, bool, error) {
	path := strings.TrimSpace(candidate.path)
	var data []byte
	var err error
	if len(candidate.data) > 0 {
		data = append([]byte(nil), candidate.data...)
	} else if path != "" {
		path, err = canonicalOpenCodePath(path, n.options.WorkspaceDir)
		if err != nil {
			return harness.Asset{}, false, err
		}
		readFile := n.options.ReadFile
		if readFile == nil {
			readFile = os.ReadFile
		}
		data, err = readFile(path)
		if err != nil {
			// Tool-result paths can precede the actual write. Keep a bounded,
			// turn-scoped pending correlation and let file.watcher.updated retry.
			n.pending[path] = mergeOpenCodeCandidates(n.pending[path], candidate)
			return harness.Asset{}, false, nil
		}
		if pending, ok := n.pending[path]; ok {
			candidate = mergeOpenCodeCandidates(pending, candidate)
			delete(n.pending, path)
		}
	}

	if candidate.uri != "" {
		uri := strings.TrimSpace(candidate.uri)
		switch {
		case strings.HasPrefix(strings.ToLower(uri), "data:"):
			candidateMIME, decoded, decodeErr := decodeOpenCodeDataURI(uri)
			if decodeErr != nil {
				return harness.Asset{}, false, decodeErr
			}
			if candidate.mime == "" {
				candidate.mime = candidateMIME
			}
			data = decoded
		case strings.HasPrefix(strings.ToLower(uri), "file:"):
			path, err = openCodePathFromURI(uri)
			if err != nil {
				return harness.Asset{}, false, err
			}
			candidate.uri = ""
			candidate.path = path
			return n.assetFromCandidate(candidate)
		default:
			// Remote URLs are intentionally not fetched by an adapter helper.
			return harness.Asset{}, false, nil
		}
	}
	if len(data) == 0 {
		return harness.Asset{}, false, nil
	}

	mimeType := normalizedOpenCodeMIME(candidate.mime, path)
	if mimeType == openCodeDefaultMIME {
		if detected := http.DetectContentType(data); strings.HasPrefix(detected, "image/") {
			mimeType = detected
		}
	}
	if !candidate.imageHint && !strings.HasPrefix(strings.ToLower(mimeType), "image/") && !isOpenCodeImagePath(path) {
		return harness.Asset{}, false, nil
	}
	if candidate.imageHint && !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		mimeType = imageMIMEFromPath(path, mimeType)
	}

	name := strings.TrimSpace(candidate.name)
	if name == "" && path != "" {
		name = filepath.Base(path)
	}
	hash := harness.HashBytes(data)
	if name == "" {
		name = "opencode-" + hash[:openCodeAssetIDSize]
	}
	materializedPath := path
	if n.options.Materialize != nil {
		materializedPath, err = n.options.Materialize(name, mimeType, data)
		if err != nil {
			return harness.Asset{}, false, fmt.Errorf("materialize opencode asset: %w", err)
		}
		materializedPath = strings.TrimSpace(materializedPath)
	}
	if materializedPath == "" {
		materializedPath, err = writeOpenCodeTemporaryAsset(name, mimeType, data)
		if err != nil {
			return harness.Asset{}, false, err
		}
	}

	id := strings.TrimSpace(candidate.id)
	if id == "" {
		id = "opencode-" + hash[:openCodeAssetIDSize]
	}
	return harness.Asset{
		Attachment: harness.Attachment{
			ID:          id,
			Kind:        harness.MediaKindImage,
			Name:        name,
			MIMEType:    mimeType,
			Path:        materializedPath,
			Size:        int64(len(data)),
			Width:       candidate.width,
			Height:      candidate.height,
			ContentHash: hash,
		},
		Source:        candidate.source,
		SessionID:     candidate.sessionID,
		TurnID:        candidate.turnID,
		Saved:         materializedPath != "",
		Status:        candidate.status,
		RevisedPrompt: candidate.revisedPrompt,
		SafeError:     candidate.safeError,
	}, true, nil
}

type openCodeAssetCandidate struct {
	source        harness.AssetSource
	sessionID     string
	turnID        string
	id            string
	name          string
	mime          string
	path          string
	uri           string
	data          []byte
	width         int
	height        int
	status        string
	revisedPrompt string
	safeError     string
	imageHint     bool
}

type openCodeCandidateContext struct {
	source    harness.AssetSource
	imageHint bool
	toolDepth int
}

func collectOpenCodeCandidates(value any, context openCodeCandidateContext, output *[]openCodeAssetCandidate) {
	switch current := value.(type) {
	case map[string]any:
		typ := strings.ToLower(strings.TrimSpace(openCodeStringField(current, "type", "kind", "partType")))
		local := context
		if strings.Contains(typ, "tool") || typ == "tool-result" || typ == "tool_result" {
			local.source = harness.AssetSourceTool
			local.toolDepth++
		}
		if typ == "image" || typ == "file" || typ == "image-generation" || typ == "image_generation" {
			local.imageHint = true
		}
		if candidate, ok := openCodeCandidateFromMap(current, local); ok {
			*output = append(*output, candidate)
		}
		for key, nested := range current {
			next := local
			if isOpenCodeToolKey(key) {
				next.source = harness.AssetSourceTool
				next.toolDepth++
			}
			if isOpenCodeImageKey(key) {
				next.imageHint = true
			}
			if s, ok := nested.(string); ok && isOpenCodePathKey(key) {
				if candidate, ok := openCodeCandidateFromString(s, next); ok {
					*output = append(*output, candidate)
				}
				continue
			}
			collectOpenCodeCandidates(nested, next, output)
		}
	case []any:
		for _, nested := range current {
			collectOpenCodeCandidates(nested, context, output)
		}
	case string:
		if context.toolDepth > 0 {
			if candidate, ok := openCodeCandidateFromString(current, context); ok {
				*output = append(*output, candidate)
			}
		}
	}
}

func openCodeCandidateFromMap(values map[string]any, context openCodeCandidateContext) (openCodeAssetCandidate, bool) {
	typ := strings.ToLower(strings.TrimSpace(openCodeStringField(values, "type", "kind", "partType")))
	uri := openCodeStringField(values, "url", "uri", "href", "src")
	path := openCodeStringField(values, "path", "filepath", "filePath", "file_path", "savedPath", "saved_path")
	if path == "" && typ == "file" {
		path = openCodeStringField(values, "file")
	}
	if uri == "" && strings.HasPrefix(strings.ToLower(path), "data:") {
		uri, path = path, ""
	}
	dataURI := openCodeStringField(values, "data")
	if uri == "" && strings.HasPrefix(strings.ToLower(dataURI), "data:") {
		uri = dataURI
	}
	mimeType := openCodeStringField(values, "mime", "mimeType", "mime_type", "mediaType", "contentType", "content_type")
	source := context.source
	if strings.EqualFold(openCodeStringField(values, "source", "origin"), string(harness.AssetSourceTool)) {
		source = harness.AssetSourceTool
	}
	candidate := openCodeAssetCandidate{
		source:        source,
		id:            openCodeStringField(values, "id", "assetID", "assetId"),
		name:          openCodeStringField(values, "name", "filename", "fileName", "file_name"),
		mime:          mimeType,
		path:          path,
		uri:           uri,
		width:         openCodeIntField(values, "width"),
		height:        openCodeIntField(values, "height"),
		status:        openCodeStringField(values, "status"),
		revisedPrompt: openCodeStringField(values, "revisedPrompt", "revised_prompt"),
		safeError:     openCodeStringField(values, "safeError", "safe_error", "error"),
		imageHint:     context.imageHint || typ == "image" || typ == "file" || strings.HasPrefix(strings.ToLower(mimeType), "image/"),
	}
	if turnID := openCodeStringField(values, "turnID", "turnId", "turn_id"); turnID != "" {
		candidate.turnID = turnID
	}
	if sessionID := openCodeStringField(values, "sessionID", "sessionId", "session_id"); sessionID != "" {
		candidate.sessionID = sessionID
	}
	if raw, ok := values["data"].([]byte); ok {
		candidate.data = append([]byte(nil), raw...)
	}
	if candidate.path == "" && candidate.uri == "" && len(candidate.data) == 0 {
		return openCodeAssetCandidate{}, false
	}
	if candidate.path != "" && !candidate.imageHint && !isOpenCodeImagePath(candidate.path) {
		return openCodeAssetCandidate{}, false
	}
	return candidate, true
}

func openCodeCandidateFromString(value string, context openCodeCandidateContext) (openCodeAssetCandidate, bool) {
	value = strings.Trim(strings.TrimSpace(value), "`\"'(),;[]{}")
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		return openCodeAssetCandidate{source: context.source, uri: value, imageHint: true}, true
	}
	if strings.HasPrefix(strings.ToLower(value), "file:") {
		return openCodeAssetCandidate{source: context.source, uri: value, imageHint: true}, true
	}
	for _, token := range strings.FieldsFunc(value, unicode.IsSpace) {
		token = strings.Trim(token, "`\"'(),;[]{}")
		if isOpenCodeImagePath(token) {
			return openCodeAssetCandidate{source: context.source, path: token, imageHint: true}, true
		}
	}
	return openCodeAssetCandidate{}, false
}

func openCodeCandidateKey(candidate openCodeAssetCandidate, workspace string) string {
	if path := strings.TrimSpace(candidate.path); path != "" {
		if canonical, err := canonicalOpenCodePath(path, workspace); err == nil {
			return "path:" + canonical
		}
		return "path:" + path
	}
	if uri := strings.TrimSpace(candidate.uri); uri != "" {
		if strings.HasPrefix(strings.ToLower(uri), "file:") {
			if path, err := openCodePathFromURI(uri); err == nil {
				return "path:" + path
			}
		}
		return "uri:" + uri
	}
	if len(candidate.data) > 0 {
		return "hash:" + harness.HashBytes(candidate.data)
	}
	return ""
}

func mergeOpenCodeCandidates(first, second openCodeAssetCandidate) openCodeAssetCandidate {
	merged := first
	if merged.source == "" {
		merged.source = second.source
	}
	if second.source == harness.AssetSourceTool {
		merged.source = second.source
	}
	if merged.sessionID == "" {
		merged.sessionID = second.sessionID
	}
	if merged.turnID == "" {
		merged.turnID = second.turnID
	}
	if merged.id == "" {
		merged.id = second.id
	}
	if merged.name == "" {
		merged.name = second.name
	}
	if merged.mime == "" {
		merged.mime = second.mime
	}
	if merged.path == "" {
		merged.path = second.path
	}
	if merged.uri == "" {
		merged.uri = second.uri
	}
	if len(merged.data) == 0 {
		merged.data = second.data
	}
	if merged.width == 0 {
		merged.width = second.width
	}
	if merged.height == 0 {
		merged.height = second.height
	}
	if merged.status == "" {
		merged.status = second.status
	}
	if merged.revisedPrompt == "" {
		merged.revisedPrompt = second.revisedPrompt
	}
	if merged.safeError == "" {
		merged.safeError = second.safeError
	}
	merged.imageHint = merged.imageHint || second.imageHint
	return merged
}

func openCodeAssetKeys(asset harness.Asset) []string {
	var keys []string
	if hash := strings.ToLower(strings.TrimSpace(harness.ContentHash(asset))); hash != "" {
		keys = append(keys, "hash:"+hash)
	}
	if path := strings.TrimSpace(asset.Path); path != "" {
		if canonical, err := canonicalOpenCodePath(path, ""); err == nil {
			keys = append(keys, "path:"+canonical)
		}
	}
	if len(keys) == 0 && strings.TrimSpace(asset.ID) != "" {
		keys = append(keys, "id:"+strings.TrimSpace(asset.ID))
	}
	return keys
}

func findOpenCodeProvider(root map[string]any, providerID string) map[string]any {
	if providers, ok := root["providers"]; ok {
		if provider := findNamedOpenCodeMap(providers, providerID); provider != nil {
			return provider
		}
	}
	rootID := openCodeStringField(root, "id", "name", "slug", "providerID", "providerId")
	if (providerID == "" || rootID == providerID) && (root["models"] != nil || root["limit"] != nil || root["limits"] != nil) {
		return root
	}
	return nil
}

func findOpenCodeModel(provider map[string]any, modelID string) map[string]any {
	if provider == nil {
		return nil
	}
	models, ok := provider["models"]
	if !ok {
		modelRootID := openCodeStringField(provider, "id", "name", "slug", "modelID", "modelId")
		if modelID == "" || modelRootID == modelID {
			return provider
		}
		return nil
	}
	return findNamedOpenCodeMap(models, modelID)
}

func findNamedOpenCodeMap(value any, name string) map[string]any {
	name = strings.TrimSpace(name)
	switch current := value.(type) {
	case map[string]any:
		if name != "" {
			if direct, ok := current[name].(map[string]any); ok {
				return direct
			}
		}
		for key, item := range current {
			if entry, ok := item.(map[string]any); ok {
				id := openCodeStringField(entry, "id", "name", "slug", "modelID", "modelId", "providerID", "providerId")
				if name == "" || key == name || id == name {
					return entry
				}
			}
		}
	case []any:
		for _, item := range current {
			if entry, ok := item.(map[string]any); ok {
				id := openCodeStringField(entry, "id", "name", "slug", "modelID", "modelId", "providerID", "providerId")
				if name == "" || id == name {
					return entry
				}
			}
		}
	}
	return nil
}

func parseOpenCodeLimitMap(values map[string]any) OpenCodeLimits {
	if values == nil {
		return OpenCodeLimits{}
	}
	limits := OpenCodeLimits{
		ProviderID: openCodeStringField(values, "providerID", "providerId", "provider", "provider_id"),
		ModelID:    openCodeStringField(values, "modelID", "modelId", "model", "model_id"),
	}
	limits.MaxAttachments = firstPositiveInt(values, "maxAttachments", "max_attachments", "maxFiles", "max_files")
	limits.MaxAttachmentBytes = firstPositiveInt64(values, "maxAttachmentBytes", "max_attachment_bytes", "maxImageBytes", "max_image_bytes", "maxFileBytes", "max_file_bytes")
	limits.MaxTotalAttachmentBytes = firstPositiveInt64(values, "maxTotalAttachmentBytes", "max_total_attachment_bytes", "maxTotalImageBytes", "max_total_image_bytes")
	limits.SupportedImageMIMETypes = firstStringList(values, "supportedImageMIMETypes", "supported_image_mime_types", "imageMIMETypes", "image_mime_types")
	if value, ok := boolField(values, "imageInput", "image_input", "supportsImage", "supports_image", "vision", "attachment"); ok {
		limits.ImageInputKnown = true
		limits.ImageInputSupported = value
	}
	if modalities, ok := values["modalities"].(map[string]any); ok {
		if input := stringListValue(modalities["input"]); len(input) > 0 {
			limits.ImageInputKnown = true
			limits.ImageInputSupported = containsImageModality(input)
		}
	}
	for _, key := range []string{"limit", "limits", "constraints", "capabilities", "image", "input", "attachments"} {
		if nested, ok := values[key].(map[string]any); ok {
			limits = MergeOpenCodeLimits(limits, parseOpenCodeLimitMap(nested))
		}
	}
	return limits
}

func defaultOpenCodeFileReadable(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	return err == nil && !info.IsDir()
}

func absoluteOpenCodePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("image attachment path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve image attachment path: %w", err)
	}
	return filepath.Clean(abs), nil
}

func canonicalOpenCodePath(path, workspace string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("asset path is empty")
	}
	if strings.HasPrefix(strings.ToLower(path), "file:") {
		decoded, err := openCodePathFromURI(path)
		if err != nil {
			return "", err
		}
		path = decoded
	}
	if !filepath.IsAbs(path) {
		if workspace != "" {
			path = filepath.Join(workspace, path)
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve asset path: %w", err)
	}
	clean := filepath.Clean(abs)
	if resolved, resolveErr := filepath.EvalSymlinks(clean); resolveErr == nil {
		return filepath.Clean(resolved), nil
	}
	return clean, nil
}

func openCodeFileURI(path string) (string, error) {
	abs, err := absoluteOpenCodePath(path)
	if err != nil {
		return "", err
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String(), nil
}

func openCodePathFromURI(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse OpenCode file URI: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "file") {
		return "", fmt.Errorf("unsupported OpenCode asset URI scheme %q", u.Scheme)
	}
	if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return "", errors.New("OpenCode asset file URI has a non-local host")
	}
	path, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", fmt.Errorf("decode OpenCode file URI path: %w", err)
	}
	return absoluteOpenCodePath(filepath.FromSlash(path))
}

func openCodeDataURI(mimeType string, data []byte) string {
	if strings.TrimSpace(mimeType) == "" {
		mimeType = openCodeDefaultMIME
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func decodeOpenCodeDataURI(raw string) (string, []byte, error) {
	comma := strings.IndexByte(raw, ',')
	if comma < 0 {
		return "", nil, errors.New("OpenCode data URI has no payload")
	}
	metadata := strings.TrimPrefix(raw[:comma], "data:")
	parts := strings.Split(metadata, ";")
	mimeType := openCodeDefaultMIME
	base64Payload := false
	for _, part := range parts {
		if strings.EqualFold(part, "base64") {
			base64Payload = true
			continue
		}
		if strings.Contains(part, "/") {
			mimeType = strings.TrimSpace(part)
		}
	}
	payload := raw[comma+1:]
	if base64Payload {
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return "", nil, fmt.Errorf("decode OpenCode data URI: %w", err)
		}
		return mimeType, data, nil
	}
	decoded, err := url.PathUnescape(payload)
	if err != nil {
		return "", nil, fmt.Errorf("decode OpenCode data URI: %w", err)
	}
	return mimeType, []byte(decoded), nil
}

func writeOpenCodeTemporaryAsset(name, mimeType string, data []byte) (string, error) {
	ext := filepath.Ext(name)
	if ext == "" {
		ext = openCodeMIMEExtension(mimeType)
	}
	file, err := os.CreateTemp("", "hero-opencode-asset-*"+ext)
	if err != nil {
		return "", fmt.Errorf("create temporary OpenCode asset: %w", err)
	}
	path := file.Name()
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("secure temporary OpenCode asset: %w", err)
	}
	if _, err := io.Copy(file, bytes.NewReader(data)); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write temporary OpenCode asset: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close temporary OpenCode asset: %w", err)
	}
	return path, nil
}

func openCodeMIMEExtension(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tiff"
	default:
		return ""
	}
}

func normalizedOpenCodeMIME(value, path string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		if parsed, _, err := mime.ParseMediaType(value); err == nil {
			return parsed
		}
		return value
	}
	if inferred := mime.TypeByExtension(filepath.Ext(path)); inferred != "" {
		return inferred
	}
	return openCodeDefaultMIME
}

func imageMIMEFromPath(path, fallback string) string {
	if inferred := mime.TypeByExtension(filepath.Ext(path)); strings.HasPrefix(strings.ToLower(inferred), "image/") {
		return inferred
	}
	if strings.HasPrefix(strings.ToLower(fallback), "image/") {
		return fallback
	}
	return "image/*"
}

func isOpenCodeImagePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff", ".svg":
		return true
	default:
		return false
	}
}

func openCodeMIMEAllowed(value string, allowed []string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if parsed, _, err := mime.ParseMediaType(value); err == nil {
		value = parsed
	}
	for _, item := range allowed {
		item = strings.ToLower(strings.TrimSpace(item))
		if parsed, _, err := mime.ParseMediaType(item); err == nil {
			item = parsed
		}
		if item == value || item == "image/*" && strings.HasPrefix(value, "image/") {
			return true
		}
	}
	return false
}

func mergeOpenCodeMIMEs(provider, model []string) []string {
	provider = normalizeOpenCodeMIMEs(provider)
	model = normalizeOpenCodeMIMEs(model)
	if len(provider) == 0 {
		return model
	}
	if len(model) == 0 {
		return provider
	}
	result := make([]string, 0, len(provider))
	for _, providerMIME := range provider {
		for _, modelMIME := range model {
			switch {
			case providerMIME == modelMIME:
				result = append(result, providerMIME)
			case providerMIME == "image/*" && strings.HasPrefix(modelMIME, "image/"):
				result = append(result, modelMIME)
			case modelMIME == "image/*" && strings.HasPrefix(providerMIME, "image/"):
				result = append(result, providerMIME)
			}
		}
	}
	return normalizeOpenCodeMIMEs(result)
}

func normalizeOpenCodeMIMEs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if parsed, _, err := mime.ParseMediaType(value); err == nil {
			value = parsed
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func openCodeStringField(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func openCodeIntField(values map[string]any, key string) int {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch current := value.(type) {
	case int:
		return current
	case int64:
		return int(current)
	case float64:
		return int(current)
	case json.Number:
		parsed, _ := current.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(current))
		return parsed
	default:
		return 0
	}
}

func firstPositiveInt(values map[string]any, keys ...string) int {
	for _, key := range keys {
		if value := openCodeIntField(values, key); value > 0 {
			return value
		}
	}
	return 0
}

func firstPositiveInt64(values map[string]any, keys ...string) int64 {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		var parsed int64
		switch current := value.(type) {
		case int:
			parsed = int64(current)
		case int64:
			parsed = current
		case float64:
			parsed = int64(current)
		case json.Number:
			parsed, _ = current.Int64()
		case string:
			parsed, _ = strconv.ParseInt(strings.TrimSpace(current), 10, 64)
		}
		if parsed > 0 {
			return parsed
		}
	}
	return 0
}

func firstStringList(values map[string]any, keys ...string) []string {
	for _, key := range keys {
		if list := stringListValue(values[key]); len(list) > 0 {
			return normalizeOpenCodeMIMEs(list)
		}
	}
	return nil
}

func stringListValue(value any) []string {
	switch current := value.(type) {
	case []string:
		return append([]string(nil), current...)
	case []any:
		result := make([]string, 0, len(current))
		for _, item := range current {
			if stringValue, ok := item.(string); ok && strings.TrimSpace(stringValue) != "" {
				result = append(result, strings.TrimSpace(stringValue))
			}
		}
		return result
	default:
		return nil
	}
}

func boolField(values map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		switch value := values[key].(type) {
		case bool:
			return value, true
		case string:
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "true", "yes", "supported":
				return true, true
			case "false", "no", "unsupported":
				return false, true
			}
		}
	}
	return false, false
}

func containsImageModality(values []string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), "image") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "image/") {
			return true
		}
	}
	return false
}

func isOpenCodeToolKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "tool", "tools", "toolresult", "tool_result", "tool-results", "tool_results", "result", "output":
		return true
	default:
		return false
	}
}

func isOpenCodeImageKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "image", "images", "file", "files", "asset", "assets", "attachment", "attachments":
		return true
	default:
		return false
	}
}

func isOpenCodePathKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "path", "filepath", "file_path", "filepathname", "filename", "file", "savedpath", "saved_path", "url", "uri", "src", "output", "result":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func minimumPositiveInt(first, second int) int {
	switch {
	case first <= 0:
		return maxInt(second, 0)
	case second <= 0:
		return first
	case first < second:
		return first
	default:
		return second
	}
}

func minimumPositiveInt64(first, second int64) int64 {
	switch {
	case first <= 0:
		return maxInt64(second, 0)
	case second <= 0:
		return first
	case first < second:
		return first
	default:
		return second
	}
}

func maxInt(value, fallback int) int {
	if value < fallback {
		return fallback
	}
	return value
}

func maxInt64(value, fallback int64) int64 {
	if value < fallback {
		return fallback
	}
	return value
}
