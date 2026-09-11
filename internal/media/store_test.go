package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func writeFixture(t *testing.T, ext string, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 20), B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	switch ext {
	case ".png":
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
	case ".jpg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
			t.Fatal(err)
		}
	case ".gif":
		if err := gif.Encode(&buf, img, nil); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unsupported fixture %q", ext)
	}
	path := filepath.Join(t.TempDir(), "fixture"+ext)
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateImageUsesMagicBytesAndHeaders(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		mime string
	}{
		{name: "png", ext: ".png", mime: "image/png"},
		{name: "jpeg", ext: ".jpg", mime: "image/jpeg"},
		{name: "gif", ext: ".gif", mime: "image/gif"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFixture(t, tt.ext, 2, 3)
			result, err := ValidateImage(context.Background(), path, ValidationOptions{Limits: DefaultLimits()})
			if err != nil {
				t.Fatal(err)
			}
			if result.MIMEType != tt.mime || result.Width != 2 || result.Height != 3 {
				t.Fatalf("metadata=%+v", result)
			}
		})
	}
	spoof := filepath.Join(t.TempDir(), "spoof.png")
	if err := os.WriteFile(spoof, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateImage(context.Background(), spoof, ValidationOptions{Limits: DefaultLimits()}); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("spoof error=%v", err)
	}
}

func TestValidateImageRejectsDimensionBombBeforeDecode(t *testing.T) {
	data := make([]byte, 24)
	copy(data[:8], []byte("\x89PNG\r\n\x1a\n"))
	data[16], data[17], data[18], data[19] = 0x00, 0x00, 0x20, 0x00
	data[20], data[21], data[22], data[23] = 0x00, 0x00, 0x20, 0x00
	path := filepath.Join(t.TempDir(), "bomb.dat")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ValidateImage(context.Background(), path, ValidationOptions{Limits: DefaultLimits()})
	if !errors.Is(err, ErrDimensionsExceeded) && !errors.Is(err, ErrPixelLimitExceeded) {
		t.Fatalf("bomb error=%v", err)
	}
}

func TestValidateTurnLimitsAndExternalWarning(t *testing.T) {
	workspace := t.TempDir()
	outside := writeFixture(t, ".png", 2, 2)
	result, err := ValidateImage(context.Background(), outside, ValidationOptions{Workspace: workspace, Limits: DefaultLimits()})
	if err != nil || !result.ExternalPathWarning {
		t.Fatalf("external result=%+v err=%v", result, err)
	}
	if _, err := ValidateImage(context.Background(), outside, ValidationOptions{Workspace: workspace, Limits: DefaultLimits()}); err != nil {
		t.Fatalf("external warning should not fail validation: %v", err)
	}
}

func TestValidateImageCanonicalizesSymlink(t *testing.T) {
	original := writeFixture(t, ".png", 2, 2)
	link := filepath.Join(t.TempDir(), "link.png")
	if err := os.Symlink(original, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	result, err := ValidateImage(context.Background(), link, ValidationOptions{Limits: DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := filepath.EvalSymlinks(original)
	if result.CanonicalPath != canonical {
		t.Fatalf("canonical=%q want %q", result.CanonicalPath, canonical)
	}
}

func TestStoreMaterializesWithDedupeAndManifest(t *testing.T) {
	store, err := New(StoreOptions{DataHome: t.TempDir(), SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	path := writeFixture(t, ".png", 3, 4)
	first, err := store.Materialize(context.Background(), MaterializeRequest{SourcePath: path, TurnID: "turn-1", AllowExternalPath: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Materialize(context.Background(), MaterializeRequest{SourcePath: path, TurnID: "turn-2", AllowExternalPath: true})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.Path != second.Path || first.ContentHash == "" || first.ContentHash != second.ContentHash {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	assetInfo, err := os.Stat(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	if assetInfo.Mode().Perm() != 0o600 {
		t.Fatalf("asset mode=%o", assetInfo.Mode().Perm())
	}
	dirInfo, err := os.Stat(store.SessionDir())
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("session mode=%o", dirInfo.Mode().Perm())
	}
	manifest := store.Manifest()
	if len(manifest.Assets) != 2 || manifest.Assets[0].Path != manifest.Assets[1].Path {
		t.Fatalf("manifest=%+v", manifest)
	}
}

func TestStoreMaterializeOutputAssetAndBytesUsesSessionPath(t *testing.T) {
	store, err := New(StoreOptions{DataHome: t.TempDir(), SessionID: "output-session"})
	if err != nil {
		t.Fatal(err)
	}
	path := writeFixture(t, ".png", 2, 2)
	asset, err := store.MaterializeAsset(context.Background(), harness.Asset{
		Attachment: harness.Attachment{Path: path, Name: "generated.png"},
		Source:     harness.AssetSourceTool,
		SessionID:  "output-session",
		TurnID:     "turn-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if asset.Source != harness.AssetSourceTool || asset.Path == path || asset.SessionID != "output-session" || !asset.Saved {
		t.Fatalf("materialized output=%+v", asset)
	}
	if !strings.Contains(asset.Path, filepath.Join("output-session", "assets")) {
		t.Fatalf("output path=%q is not session scoped", asset.Path)
	}
	bytesAsset, err := store.MaterializeBytes(context.Background(), mustReadFile(t, path), "generated.png", "turn-2", string(harness.AssetSourceModel))
	if err != nil {
		t.Fatal(err)
	}
	if bytesAsset.Path != asset.Path || bytesAsset.ContentHash != asset.ContentHash {
		t.Fatalf("byte materialization did not dedupe: asset=%+v bytes=%+v", asset, bytesAsset)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCleanupSessionsRemovesExpiredOnly(t *testing.T) {
	dataHome := t.TempDir()
	root := filepath.Join(dataHome, "hero", "sessions")
	old := filepath.Join(root, "old")
	fresh := filepath.Join(root, "fresh")
	if err := os.MkdirAll(filepath.Join(old, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fresh, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(old, now.Add(-8*24*time.Hour), now.Add(-8*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := CleanupExpiredSessions(context.Background(), CleanupOptions{DataHome: dataHome, Retention: 7 * 24 * time.Hour, Now: func() time.Time { return now }}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old session still exists: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh session missing: %v", err)
	}
}
