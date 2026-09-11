package codex

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestProbeCodexAppServerSchemaRecognizesImageUnions(t *testing.T) {
	raw := json.RawMessage(`{
  "methods": {
    "turn/start": {
      "input": {"oneOf": [
        {"type": "text"},
        {"type": "image"},
        {"type": "localImage"}
      ]}
    }
  },
  "items": {"oneOf": [
    {"type": "imageGeneration"},
    {"type": "imageView"}
  ]}
}`)

	got := ProbeCodexAppServerSchema(raw)
	if !got.SchemaPresent || got.Degraded {
		t.Fatalf("schema=%+v, want complete non-degraded probe", got)
	}
	if !got.InputLocalImage || !got.InputImage || !got.OutputImageGeneration || !got.OutputImageView {
		t.Fatalf("schema=%+v, want all image union members", got)
	}
	if !got.SupportsNativeInput() || !got.SupportsImageOutput() {
		t.Fatalf("schema support methods false: %+v", got)
	}
}

func TestProbeCodexSchemaAbsentDegradesExplicitly(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, []byte(`{}`), []byte(`null`), []byte(`not-json`)} {
		got := ProbeCodexAppServerSchema(raw)
		if got.SchemaPresent || !got.Degraded {
			t.Fatalf("raw=%q probe=%+v, want absent degraded schema", raw, got)
		}
		if !strings.Contains(strings.ToLower(got.Diagnostic), "degraded") && !strings.Contains(strings.ToLower(got.Diagnostic), "absent") {
			t.Fatalf("raw=%q diagnostic=%q is not explicit", raw, got.Diagnostic)
		}
	}
}

func TestBuildCodexTurnInputOrdersImagesBeforeText(t *testing.T) {
	schema := CodexMultimodalSchema{SchemaPresent: true, InputLocalImage: true}
	attachments := []harness.Attachment{
		{Kind: harness.MediaKindImage, Path: "/tmp/first.png"},
		{Kind: harness.MediaKindImage, Path: "/tmp/second.jpg"},
	}

	got, err := BuildCodexTurnStartInput(schema, attachments, "describe both")
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]string{
		{"type": "localImage", "path": "/tmp/first.png"},
		{"type": "localImage", "path": "/tmp/second.jpg"},
		{"type": "text", "text": "describe both"},
	}
	if !equalStringMaps(got, want) {
		t.Fatalf("input=%v want %v", got, want)
	}

	imageOnly, err := BuildCodexTurnStartInput(schema, attachments, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(imageOnly) != 2 {
		t.Fatalf("image-only input=%v want two localImage entries", imageOnly)
	}

	wire, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(wire, []byte(`"type":"localImage"`)) || !bytes.Contains(wire, []byte(`"type":"text"`)) {
		t.Fatalf("wire=%s missing expected discriminators", wire)
	}
}

func TestBuildCodexTurnInputFailsClosedWhenSchemaIsAbsent(t *testing.T) {
	_, err := BuildCodexTurnInput(CodexMultimodalSchema{}, []harness.Attachment{{Path: "/tmp/image.png"}}, "look")
	if err == nil {
		t.Fatal("expected explicit multimodal degradation error")
	}
	if !errors.Is(err, ErrCodexMultimodalDegraded) {
		t.Fatalf("error=%v is not ErrCodexMultimodalDegraded", err)
	}
	if !strings.Contains(err.Error(), "localImage") {
		t.Fatalf("error=%q does not identify missing localImage support", err)
	}
}

func TestCodexTurnInputsCompatibilityHelperUsesNativeShape(t *testing.T) {
	got, err := CodexTurnInputs([]harness.Attachment{{Path: "/tmp/image.png"}}, "look")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0]["type"] != CodexLocalImageInputType || got[0]["path"] != "/tmp/image.png" || got[1]["type"] != "text" {
		t.Fatalf("input=%v", got)
	}
}

func TestNormalizeCodexImageGenerationAndImageView(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generated.png")
	data := writeTinyPNG(t, path, 3, 2)

	generation := mustJSON(t, map[string]any{
		"item": map[string]any{
			"type":           "imageGeneration",
			"id":             "image-1",
			"status":         "completed",
			"saved_path":     path,
			"revised_prompt": "a blue square",
		},
	})
	got, err := NormalizeCodexNotification("item/completed", generation, "thread-1", "turn-1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Assets) != 1 || len(got.Deltas) != 1 {
		t.Fatalf("batch=%+v, want one asset and delta", got)
	}
	asset := got.Assets[0]
	if asset.Source != harness.AssetSourceModel || asset.ID != "image-1" || asset.Status != "completed" || asset.RevisedPrompt != "a blue square" {
		t.Fatalf("asset=%+v", asset)
	}
	if !asset.Saved || asset.Path != path || asset.MIMEType != "image/png" || asset.Width != 3 || asset.Height != 2 || asset.Size != int64(len(data)) {
		t.Fatalf("asset metadata=%+v", asset)
	}
	if asset.ContentHash != harness.HashBytes(data) {
		t.Fatalf("hash=%q want %q", asset.ContentHash, harness.HashBytes(data))
	}
	if got.Deltas[0].Kind != harness.StreamKindAsset || got.Deltas[0].Asset == nil || got.Deltas[0].Asset.ContentHash != asset.ContentHash {
		t.Fatalf("delta=%+v", got.Deltas[0])
	}

	view := mustJSON(t, map[string]any{
		"type": "imageView",
		"id":   "view-1",
		"path": path,
	})
	viewBatch, err := NormalizeCodexNotification("item/completed", view, "thread-1", "turn-1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(viewBatch.Assets) != 1 || viewBatch.Assets[0].Source != harness.AssetSourceTool {
		t.Fatalf("imageView batch=%+v", viewBatch)
	}
}

func TestNormalizeCodexImageFailureKeepsSafeMetadata(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"type":           "imageGeneration",
		"id":             "image-failed",
		"status":         "failed",
		"revised_prompt": "make a harmless icon",
		"failure": map[string]any{
			"message": "quota exceeded",
		},
		// This is deliberately not copied into Asset or diagnostics.
		"result": strings.Repeat("ZmFrZS1pbWFnZS1ieXRlcw==", 20),
	})
	asset, err := NormalizeCodexImageItem(raw, "thread-1", "turn-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if asset.Status != "failed" || asset.SafeError != "quota exceeded" || asset.RevisedPrompt != "make a harmless icon" {
		t.Fatalf("asset=%+v", asset)
	}
	if asset.Saved || asset.Path != "" {
		t.Fatalf("failed metadata-only asset=%+v", asset)
	}
	if strings.Contains(asset.SafeError, "ZmFrZS") || strings.Contains(asset.SafeError, "result") {
		t.Fatalf("unsafe generation payload leaked into safe error: %q", asset.SafeError)
	}
}

func TestNormalizeCodexToolWrittenImagesUsesMagicHashDedupe(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "design-a.png")
	second := filepath.Join(dir, "design-b.png")
	data := writeTinyPNG(t, first, 2, 2)
	if err := os.WriteFile(second, data, 0o600); err != nil {
		t.Fatal(err)
	}

	raw := mustJSON(t, map[string]any{
		"type": "fileChange",
		"id":   "tool-1",
		"changes": map[string]any{
			first:  map[string]any{"kind": "add"},
			second: map[string]any{"kind": "add"},
		},
	})
	paths, err := ExtractCodexToolWrittenImagePaths(raw, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("paths=%v want two structured image paths", paths)
	}

	assets, err := NormalizeCodexToolWrittenImages(raw, "thread-1", "turn-1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 {
		t.Fatalf("assets=%+v want one SHA-256-deduped asset", assets)
	}
	if assets[0].Source != harness.AssetSourceTool || assets[0].ContentHash != harness.HashBytes(data) || !assets[0].Saved {
		t.Fatalf("asset=%+v", assets[0])
	}
}

func TestNormalizeCodexToolCommandPathAndNotificationWrapper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "render.webp")
	writeTinyWebP(t, path, 4, 5)
	raw := mustJSON(t, map[string]any{
		"threadId": "thread-1",
		"turnId":   "turn-1",
		"item": map[string]any{
			"type":    "commandExecution",
			"id":      "command-1",
			"cwd":     dir,
			"command": "write-image render.webp",
		},
	})
	batch, err := NormalizeCodexNotification("item/completed", raw, "", "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Assets) != 1 {
		t.Fatalf("batch=%+v want one command-written asset", batch)
	}
	asset := batch.Assets[0]
	if asset.SessionID != "thread-1" || asset.TurnID != "turn-1" || asset.MIMEType != "image/webp" || asset.Width != 4 || asset.Height != 5 {
		t.Fatalf("asset=%+v", asset)
	}
}

func TestRepairCodexAssetsRepairsStreamAndDedupesByHash(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "one.png")
	second := filepath.Join(dir, "two.png")
	data := writeTinyPNG(t, first, 2, 3)
	if err := os.WriteFile(second, data, 0o600); err != nil {
		t.Fatal(err)
	}
	hash := harness.HashBytes(data)

	streamed := []harness.Asset{{
		Attachment: harness.Attachment{ID: "image-1", Kind: harness.MediaKindImage},
		Source:     harness.AssetSourceModel,
		SessionID:  "thread-1",
		TurnID:     "turn-1",
		Status:     "generating",
	}}
	final := []harness.Asset{{
		Attachment: harness.Attachment{
			ID:          "image-1",
			Kind:        harness.MediaKindImage,
			Name:        "one.png",
			MIMEType:    "image/png",
			Path:        second,
			Size:        int64(len(data)),
			Width:       2,
			Height:      3,
			ContentHash: hash,
		},
		Source: harness.AssetSourceModel,
		Saved:  true,
		Status: "completed",
	}}
	toolDuplicate := final[0]
	toolDuplicate.ID = "tool-1"
	toolDuplicate.Source = harness.AssetSourceTool

	got := RepairCodexAssets(streamed, append(final, toolDuplicate))
	if len(got) != 1 {
		t.Fatalf("assets=%+v want one repaired asset", got)
	}
	if got[0].Path != second || got[0].ContentHash != hash || !got[0].Saved || got[0].Status != "completed" {
		t.Fatalf("repaired=%+v", got[0])
	}
	if got[0].Source != harness.AssetSourceTool {
		t.Fatalf("source=%q want tool source to survive duplicate detection", got[0].Source)
	}
}

func TestCodexAssetStreamDeltaDoesNotExposeRawPayload(t *testing.T) {
	asset := harness.Asset{
		Attachment: harness.Attachment{
			ID:          "image-1",
			Kind:        harness.MediaKindImage,
			Name:        "generated.png",
			ContentHash: strings.Repeat("a", 64),
		},
		Source:    harness.AssetSourceModel,
		SessionID: "thread-1",
		Status:    "completed",
	}
	delta := CodexAssetStreamDelta(asset, "item/completed", harness.StreamPhaseCompleted)
	if delta.Kind != harness.StreamKindAsset || delta.Asset == nil || delta.Asset == &asset {
		t.Fatalf("delta=%+v does not carry an immutable copy", delta)
	}
	if delta.Metadata["source"] != "model" || delta.Metadata["status"] != "completed" {
		t.Fatalf("metadata=%v", delta.Metadata)
	}
}

func equalStringMaps(got, want []map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if len(got[index]) != len(want[index]) {
			return false
		}
		for key, wantValue := range want[index] {
			if got[index][key] != wantValue {
				return false
			}
		}
	}
	return true
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func writeTinyPNG(t *testing.T, path string, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 40), G: uint8(y * 40), B: 180, A: 255})
		}
	}
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	data := buffer.Bytes()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return data
}

func writeTinyWebP(t *testing.T, path string, width, height int) {
	t.Helper()
	// Minimal VP8X container. The normalizer only needs a real RIFF/WEBP
	// signature and the bounded canvas dimensions for metadata tests.
	data := make([]byte, 30)
	copy(data[0:4], "RIFF")
	copy(data[8:12], "WEBP")
	copy(data[12:16], "VP8X")
	data[24] = byte(width - 1)
	data[25] = byte((width - 1) >> 8)
	data[26] = byte((width - 1) >> 16)
	data[27] = byte(height - 1)
	data[28] = byte((height - 1) >> 8)
	data[29] = byte((height - 1) >> 16)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
