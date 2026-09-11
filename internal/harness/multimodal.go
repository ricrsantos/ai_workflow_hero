package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// MediaKind identifies the kind of user or harness-produced media. C14 only
// transports images, but keeping the kind explicit leaves the boundary
// extensible without making adapters infer it from MIME types.
type MediaKind string

const (
	// MediaKindImage is an image attachment or image asset.
	MediaKindImage MediaKind = "image"
)

// AssetSource identifies who produced an asset during a turn.
type AssetSource string

const (
	AssetSourceUser  AssetSource = "user"
	AssetSourceModel AssetSource = "model"
	AssetSourceTool  AssetSource = "tool"
)

// Attachment is the immutable, disk-backed image reference shared by the TUI,
// conversation service, and harness adapters. Path points to a materialized
// session file; Name is metadata only and must never be used as a write path.
type Attachment struct {
	ID       string    `json:"id"`
	Kind     MediaKind `json:"kind"`
	Name     string    `json:"name"`
	MIMEType string    `json:"mime_type"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	// ContentHash is the SHA-256 of the materialized bytes. It is used for
	// session-scoped deduplication and stream/final asset repair.
	ContentHash string `json:"content_hash,omitempty"`
}

// Asset is an image produced or echoed by a harness. It embeds only metadata
// and a materialized path; image bytes never enter Bubble Tea model state.
type Asset struct {
	Attachment
	Source    AssetSource `json:"source"`
	SessionID string      `json:"session_id"`
	TurnID    string      `json:"turn_id"`
	Saved     bool        `json:"saved"`
	// Status and RevisedPrompt preserve optional native output metadata without
	// exposing provider-specific payloads to the TUI.
	Status        string `json:"status,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	SafeError     string `json:"safe_error,omitempty"`
}

// MediaCapability is one side of the capability intersection. A capability is
// supported only when both the adapter transport and selected model advertise
// the relevant flag; absent information remains unsupported.
type MediaCapability struct {
	ImageInputNative        bool     `json:"image_input_native"`
	ImageInputFileReference bool     `json:"image_input_file_reference"`
	ImageOutputNative       bool     `json:"image_output_native"`
	ImageOutputFile         bool     `json:"image_output_file"`
	SupportedImageMIMETypes []string `json:"supported_image_mime_types"`
	MaxAttachmentBytes      int64    `json:"max_attachment_bytes"`
}

// SupportsImageInput reports whether an image can be sent through this
// capability. An empty MIME type checks transport support only.
func (c MediaCapability) SupportsImageInput(mimeType string) bool {
	if !c.ImageInputNative && !c.ImageInputFileReference {
		return false
	}
	if mimeType == "" || len(c.SupportedImageMIMETypes) == 0 {
		return true
	}
	for _, supported := range c.SupportedImageMIMETypes {
		if strings.EqualFold(strings.TrimSpace(supported), strings.TrimSpace(mimeType)) {
			return true
		}
	}
	return false
}

// IntersectMediaCapabilities combines independent adapter and model facts.
// Boolean support uses intersection and MIME types use set intersection. A
// zero byte limit means unknown, so the stricter known limit wins when one
// source is missing a limit.
func IntersectMediaCapabilities(transport, model MediaCapability) MediaCapability {
	return MediaCapability{
		ImageInputNative:        transport.ImageInputNative && model.ImageInputNative,
		ImageInputFileReference: transport.ImageInputFileReference && model.ImageInputFileReference,
		ImageOutputNative:       transport.ImageOutputNative && model.ImageOutputNative,
		ImageOutputFile:         transport.ImageOutputFile && model.ImageOutputFile,
		SupportedImageMIMETypes: intersectMIMETypes(transport.SupportedImageMIMETypes, model.SupportedImageMIMETypes),
		MaxAttachmentBytes:      intersectMaxBytes(transport.MaxAttachmentBytes, model.MaxAttachmentBytes),
	}
}

func intersectMaxBytes(left, right int64) int64 {
	switch {
	case left <= 0:
		return maxPositive(right)
	case right <= 0:
		return left
	default:
		if left < right {
			return left
		}
		return right
	}
}

func maxPositive(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func intersectMIMETypes(left, right []string) []string {
	if len(left) == 0 {
		return uniqueMIMETypes(right)
	}
	if len(right) == 0 {
		return uniqueMIMETypes(left)
	}
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range right {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			rightSet[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(left))
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := rightSet[value]; ok {
			if _, duplicate := seen[value]; !duplicate {
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
	}
	return result
}

func uniqueMIMETypes(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
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

// ContentHash returns an asset's declared content hash. It is intentionally
// side-effect free; callers that own disk I/O should calculate hashes before
// constructing the immutable reference.
func ContentHash(asset Asset) string {
	if hash := strings.TrimSpace(asset.ContentHash); hash != "" {
		return hash
	}
	return strings.TrimSpace(asset.Attachment.ContentHash)
}

// HashBytes returns the lowercase SHA-256 digest used by asset stores and
// adapter output deduplication.
func HashBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// MergeAssetsByContentHash appends final assets missing from streamed assets.
// It preserves first-seen order and falls back to a stable identity when an
// older adapter did not provide a content hash.
func MergeAssetsByContentHash(streamed, final []Asset) []Asset {
	result := make([]Asset, 0, len(streamed)+len(final))
	seen := make(map[string]struct{}, len(streamed)+len(final))
	positions := make(map[string]int, len(streamed)+len(final))
	appendUnique := func(asset Asset) {
		key := assetIdentity(asset)
		if index, ok := positions[key]; ok {
			result[index] = mergeAssetMetadata(result[index], asset)
			return
		}
		seen[key] = struct{}{}
		positions[key] = len(result)
		result = append(result, asset)
	}
	for _, asset := range streamed {
		appendUnique(asset)
	}
	for _, asset := range final {
		appendUnique(asset)
	}
	return result
}

// mergeAssetMetadata repairs a streamed placeholder with fields that arrive
// only in the authoritative final event. Stable first-seen identity fields
// (ID, name, and path when already present) are retained, while status, saved
// state, provenance, and missing metadata are upgraded from the final value.
func mergeAssetMetadata(base, update Asset) Asset {
	if base.ID == "" {
		base.ID = update.ID
	}
	if base.Kind == "" {
		base.Kind = update.Kind
	}
	if base.Name == "" {
		base.Name = update.Name
	}
	if base.MIMEType == "" {
		base.MIMEType = update.MIMEType
	}
	if base.Path == "" || (update.Saved && update.Path != "") {
		base.Path = update.Path
	}
	if base.Size <= 0 {
		base.Size = update.Size
	}
	if base.Width <= 0 {
		base.Width = update.Width
	}
	if base.Height <= 0 {
		base.Height = update.Height
	}
	if base.ContentHash == "" {
		base.ContentHash = update.ContentHash
	}
	if base.Source == "" || update.Source == AssetSourceTool {
		base.Source = update.Source
	}
	if base.SessionID == "" {
		base.SessionID = update.SessionID
	}
	if base.TurnID == "" {
		base.TurnID = update.TurnID
	}
	base.Saved = base.Saved || update.Saved
	if update.Status != "" {
		base.Status = update.Status
	}
	if update.RevisedPrompt != "" {
		base.RevisedPrompt = update.RevisedPrompt
	}
	if update.SafeError != "" {
		base.SafeError = update.SafeError
	}
	return base
}

// RepairAssets is the concise repair alias used by conversation consumers.
func RepairAssets(streamed, final []Asset) []Asset {
	return MergeAssetsByContentHash(streamed, final)
}

func assetIdentity(asset Asset) string {
	if hash := ContentHash(asset); hash != "" {
		return "hash:" + strings.ToLower(hash)
	}
	if asset.ID != "" {
		return "id:" + asset.ID
	}
	if asset.Path != "" {
		return "path:" + asset.Path
	}
	return fmt.Sprintf("asset:%s:%s:%d:%d", asset.Name, asset.MIMEType, asset.Width, asset.Height)
}
