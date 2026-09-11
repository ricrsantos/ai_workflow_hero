package opencode

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestBuildOpenCodeInputPartsPrefersFileURIAndFallsBackToDataURI(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "first image.png")
	fallbackPath := filepath.Join(dir, "second image.png")
	firstBytes := []byte("first-image-fixture")
	secondBytes := []byte("second-image-fixture")
	if err := os.WriteFile(filePath, firstBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fallbackPath, secondBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	req := harness.ExecuteRequest{
		Model: "provider/vision-model",
		Attachments: []harness.Attachment{
			{ID: "a1", Name: "first image.png", MIMEType: "image/png", Path: filePath, Size: int64(len(firstBytes))},
			{ID: "a2", Name: "second image.png", MIMEType: "image/png", Path: fallbackPath, Size: int64(len(secondBytes))},
		},
	}
	parts, err := BuildOpenCodeInputParts(req, "compare these", OpenCodeInputOptions{
		FileReadable: func(path string) bool { return path == filePath },
		ReadFile:     os.ReadFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("parts=%d, want 3", len(parts))
	}
	if parts[0].Type != "file" || parts[1].Type != "file" || parts[2].Type != "text" {
		t.Fatalf("part order=%+v", parts)
	}
	if want := "file://"; !strings.HasPrefix(parts[0].URL, want) {
		t.Fatalf("file URI=%q, want prefix %q", parts[0].URL, want)
	}
	if !strings.HasPrefix(parts[1].URL, "data:image/png;base64,") {
		t.Fatalf("fallback URI=%q, want image data URI", parts[1].URL)
	}
	encoded := strings.TrimPrefix(parts[1].URL, "data:image/png;base64,")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode fallback URI: %v", err)
	}
	if string(decoded) != string(secondBytes) {
		t.Fatalf("fallback bytes=%q, want %q", decoded, secondBytes)
	}
	if parts[2].Text != "compare these" {
		t.Fatalf("text=%q", parts[2].Text)
	}
}

func TestBuildOpenCodePromptBodyKeepsImagesBeforeText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.png")
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := BuildOpenCodePromptBody(harness.ExecuteRequest{
		Model:     "provider/model",
		AgentName: "build",
		Attachments: []harness.Attachment{{
			Name: "fixture.png", MIMEType: "image/png", Path: path, Size: 7,
		}},
	}, "caption", OpenCodeInputOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Model map[string]string   `json:"model"`
		Parts []OpenCodeInputPart `json:"parts"`
		Agent string              `json:"agent"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Model["providerID"] != "provider" || payload.Model["modelID"] != "model" {
		t.Fatalf("model=%v", payload.Model)
	}
	if payload.Agent != "build" || len(payload.Parts) != 2 || payload.Parts[0].Type != "file" || payload.Parts[1].Type != "text" {
		t.Fatalf("payload=%+v", payload)
	}
}

func TestOpenCodeLimitsParseAndValidateProviderModel(t *testing.T) {
	fixture := []byte(`{
  "providers": [{
    "id": "fixture-provider",
    "limits": {"max_attachments": 5, "max_attachment_bytes": 4096, "max_total_attachment_bytes": 9000},
    "models": {
      "fixture-model": {
        "modalities": {"input": ["text", "image"]},
        "limit": {"max_attachments": 2, "max_attachment_bytes": 1024},
        "supported_image_mime_types": ["image/png", "image/jpeg"]
      }
    }
  }]
}`)
	limits, err := ParseOpenCodeLimits(fixture, "fixture-provider", "fixture-model")
	if err != nil {
		t.Fatal(err)
	}
	if limits.MaxAttachments != 2 || limits.MaxAttachmentBytes != 1024 || limits.MaxTotalAttachmentBytes != 9000 {
		t.Fatalf("limits=%+v", limits)
	}
	if !limits.ImageInputKnown || !limits.ImageInputSupported {
		t.Fatalf("image capability=%+v", limits)
	}
	if got := strings.Join(limits.SupportedImageMIMETypes, ","); got != "image/jpeg,image/png" {
		t.Fatalf("MIME limits=%q", got)
	}

	attachments := []harness.Attachment{
		{MIMEType: "image/png", Size: 1024},
		{MIMEType: "image/jpeg", Size: 1024},
	}
	if err := ValidateOpenCodeAttachmentLimits("fixture-provider/fixture-model", attachments, limits); err != nil {
		t.Fatalf("valid attachments rejected: %v", err)
	}
	tooMany := append(append([]harness.Attachment(nil), attachments...), harness.Attachment{MIMEType: "image/png", Size: 1})
	if err := ValidateOpenCodeModelLimits("fixture-model", tooMany, limits); err == nil || !strings.Contains(err.Error(), "at most 2") {
		t.Fatalf("too many error=%v", err)
	}
	tooLarge := []harness.Attachment{{MIMEType: "image/png", Size: 1025}}
	if err := ValidateOpenCodeAttachmentLimits("fixture-model", tooLarge, limits); err == nil || !strings.Contains(err.Error(), "1024 bytes") {
		t.Fatalf("too large error=%v", err)
	}
	badMIME := []harness.Attachment{{MIMEType: "image/gif", Size: 1}}
	if err := ValidateOpenCodeAttachmentLimits("fixture-model", badMIME, limits); err == nil || !strings.Contains(err.Error(), "image/gif") {
		t.Fatalf("bad MIME error=%v", err)
	}
}

func TestNormalizeOpenCodeSSEFileAndDataPartsToAsset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model-output.png")
	fixture := []byte("model-output-fixture")
	if err := os.WriteFile(path, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("second-model-output"))
	materialized := filepath.Join(dir, "materialized")
	if err := os.Mkdir(materialized, 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := NormalizeOpenCodeSSEEvents([]map[string]any{
		{
			"type": "message.part.updated",
			"properties": map[string]any{
				"part": map[string]any{
					"type": "file", "id": "part-file", "url": mustFileURI(t, path),
					"mime": "image/png", "name": "model-output.png", "width": 12, "height": 8,
				},
			},
		},
		{
			"type": "message.part.updated",
			"properties": map[string]any{
				"part": map[string]any{
					"type": "image", "url": dataURI, "mime": "image/png", "name": "second.png",
				},
			},
		},
	}, OpenCodeAssetNormalizerOptions{
		SessionID: "session-1", TurnID: "turn-1", ReadFile: os.ReadFile,
		Materialize: func(name, mimeType string, data []byte) (string, error) {
			destination := filepath.Join(materialized, name)
			if err := os.WriteFile(destination, data, 0o600); err != nil {
				return "", err
			}
			return destination, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Assets) != 2 || len(result.Deltas) != 2 {
		t.Fatalf("assets=%d deltas=%d", len(result.Assets), len(result.Deltas))
	}
	if result.Assets[0].Source != harness.AssetSourceModel || result.Assets[0].TurnID != "turn-1" {
		t.Fatalf("first asset=%+v", result.Assets[0])
	}
	if result.Assets[0].Width != 12 || result.Assets[0].Height != 8 || result.Assets[0].MIMEType != "image/png" {
		t.Fatalf("first metadata=%+v", result.Assets[0])
	}
	if result.Deltas[0].Kind != harness.StreamKindAsset || result.Deltas[0].Asset == nil {
		t.Fatalf("first delta=%+v", result.Deltas[0])
	}
	if result.Assets[1].Name != "second.png" || result.Assets[1].Path == "" || result.Assets[1].ContentHash == "" {
		t.Fatalf("second asset=%+v", result.Assets[1])
	}
}

func TestOpenCodeToolResultAndWatcherCorrelationDeduplicatesByPathAndHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generated.png")
	normalizer := NewOpenCodeAssetNormalizer(OpenCodeAssetNormalizerOptions{
		SessionID: "session-tool", TurnID: "turn-tool", ReadFile: os.ReadFile,
	})
	toolResult := map[string]any{
		"type": "message.part.updated",
		"properties": map[string]any{
			"part": map[string]any{
				"type":  "tool",
				"state": map[string]any{"status": "completed", "output": path},
			},
		},
	}
	first, err := normalizer.ConsumeSSEEvent(toolResult)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Assets) != 0 {
		t.Fatalf("tool result before write produced assets=%+v", first.Assets)
	}
	fixture := []byte("tool-generated-image")
	if err := os.WriteFile(path, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	watcher := map[string]any{
		"type":       "file.watcher.updated",
		"properties": map[string]any{"path": path, "mime": "image/png", "name": "generated.png"},
	}
	second, err := normalizer.ConsumeSSEEvent(watcher)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Assets) != 1 || len(second.Deltas) != 1 {
		t.Fatalf("watcher result=%+v", second)
	}
	duplicate, err := normalizer.ConsumeSSEEvent(toolResult)
	if err != nil {
		t.Fatal(err)
	}
	if len(duplicate.Assets) != 0 || len(normalizer.FinalAssets()) != 1 {
		t.Fatalf("duplicate assets=%+v final=%+v", duplicate.Assets, normalizer.FinalAssets())
	}
	asset := normalizer.FinalAssets()[0]
	if asset.Source != harness.AssetSourceTool || asset.ContentHash != harness.HashBytes(fixture) {
		t.Fatalf("correlated asset=%+v", asset)
	}

	pathDuplicate := asset
	pathDuplicate.Source = harness.AssetSourceModel
	hashDuplicate := asset
	hashDuplicate.Path = filepath.Join(dir, "different-name.png")
	hashDuplicate.Source = harness.AssetSourceTool
	if got := DeduplicateOpenCodeAssets([]harness.Asset{pathDuplicate, hashDuplicate}); len(got) != 1 || got[0].Source != harness.AssetSourceTool {
		t.Fatalf("path/hash dedupe=%+v", got)
	}
}

func mustFileURI(t *testing.T, path string) string {
	t.Helper()
	uri, err := openCodeFileURI(path)
	if err != nil {
		t.Fatal(err)
	}
	return uri
}
