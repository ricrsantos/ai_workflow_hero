package cursor

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestComposeCursorFileReferencePrompt_FailsClosed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.png")
	if err := os.WriteFile(path, tinyCursorPNG(t), 0o600); err != nil {
		t.Fatal(err)
	}
	attachment := harness.Attachment{Kind: harness.MediaKindImage, MIMEType: "image/png", Path: path}

	tests := []struct {
		name       string
		capability harness.MediaCapability
		want       string
	}{
		{name: "unknown capability", capability: harness.MediaCapability{}, want: "image_input_file_reference"},
		{name: "native only is not file reference", capability: harness.MediaCapability{ImageInputNative: true}, want: "file_reference"},
		{name: "unsupported MIME", capability: harness.MediaCapability{ImageInputFileReference: true, SupportedImageMIMETypes: []string{"image/jpeg"}}, want: "image/png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := ComposeCursorFileReferencePrompt("inspect this", []harness.Attachment{attachment}, CursorFileReferenceOptions{Model: "cursor-test-model", Capability: tt.capability})
			if err == nil {
				t.Fatal("expected a closed failure")
			}
			if prompt != "" {
				t.Fatalf("failed composition returned a prompt: %q", prompt)
			}
			if !strings.Contains(err.Error(), "cursor") || !strings.Contains(err.Error(), "cursor-test-model") || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not identify the closed failure", err)
			}
		})
	}
}

func TestComposeCursorFileReferencePrompt_ComposesEveryMaterializedAttachment(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.png")
	secondPath := filepath.Join(dir, "second.png")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, tinyCursorPNG(t), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	attachments := []harness.Attachment{
		{ID: "a1", Kind: harness.MediaKindImage, Name: "first.png", MIMEType: "image/png", Path: firstPath, Width: 1, Height: 1},
		{ID: "a2", Kind: harness.MediaKindImage, Name: "second.png", MIMEType: "image/png", Path: secondPath},
	}

	prompt, err := ComposeCursorFileReferencePrompt("compare the images", attachments, CursorFileReferenceOptions{
		Model: "vision-model",
		Capability: harness.MediaCapability{
			ImageInputFileReference: true,
			SupportedImageMIMETypes: []string{"image/png"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{firstPath, secondPath, "compare the images", "Cursor file-reference path"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("composed prompt does not contain %q: %s", want, prompt)
		}
	}
}

func TestExtractCursorToolImagePaths_IgnoresNonToolText(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"please inspect /workspace/user.png"}]}}`,
		`{"type":"tool_result","tool_result":{"path":"./outputs/first.png","content":"wrote /workspace/outputs/second.webp"}}`,
		`{"type":"tool_call","tool_call":{"path":"./outputs/first.png"}}`,
	}, "\n")

	paths := ExtractCursorToolImagePaths([]byte(stream))
	want := []string{"./outputs/first.png", "/workspace/outputs/second.webp"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestCursorWorkspaceImageWatch_IsTurnScopedAndDeduplicatesByHash(t *testing.T) {
	png := tinyCursorPNG(t)
	filesystem := fstest.MapFS{
		"before.png": &fstest.MapFile{Data: png},
	}
	watch, err := NewCursorWorkspaceImageWatchFS("/workspace", filesystem)
	if err != nil {
		t.Fatal(err)
	}
	filesystem["outputs/first.png"] = &fstest.MapFile{Data: png}
	filesystem["outputs/same-bytes.png"] = &fstest.MapFile{Data: png}
	stream := []byte(`{"type":"tool_result","tool_result":{"content":"saved /workspace/outputs/first.png and /workspace/outputs/same-bytes.png"}}`)

	assets, err := watch.FinishTurn("session-1", "turn-1", stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 {
		t.Fatalf("assets = %#v, want one hash-deduplicated asset", assets)
	}
	asset := assets[0]
	if asset.Source != harness.AssetSourceTool || asset.SessionID != "session-1" || asset.TurnID != "turn-1" {
		t.Fatalf("asset provenance = %+v", asset)
	}
	if asset.Path != "/workspace/outputs/first.png" || asset.ContentHash == "" {
		t.Fatalf("asset path/hash = %q/%q", asset.Path, asset.ContentHash)
	}
	if asset.Width != 1 || asset.Height != 1 {
		t.Fatalf("asset dimensions = %dx%d", asset.Width, asset.Height)
	}

	if _, err := watch.FinishTurn("session-1", "turn-2", nil); err == nil {
		t.Fatal("expected a finished watch to reject a second turn")
	}
}

func TestCursorWorkspaceImageWatch_DetectsModifiedImage(t *testing.T) {
	filesystem := fstest.MapFS{"output.png": &fstest.MapFile{Data: tinyCursorPNG(t)}}
	watch, err := NewCursorWorkspaceImageWatchFS("/workspace", filesystem)
	if err != nil {
		t.Fatal(err)
	}
	modified := append([]byte(nil), tinyCursorPNG(t)...)
	modified = append(modified, 0)
	filesystem["output.png"] = &fstest.MapFile{Data: modified}

	paths, err := watch.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "/workspace/output.png" {
		t.Fatalf("modified paths = %#v", paths)
	}
}

func tinyCursorPNG(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return data
}
