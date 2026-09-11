package media

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeWebPHeaderFixture(t *testing.T, width, height uint32) string {
	t.Helper()
	data := make([]byte, 30)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], 22)
	copy(data[8:12], "WEBP")
	copy(data[12:16], "VP8X")
	binary.LittleEndian.PutUint32(data[16:20], 10)
	data[20] = 0
	data[24] = byte(width - 1)
	data[25] = byte((width - 1) >> 8)
	data[26] = byte((width - 1) >> 16)
	data[27] = byte(height - 1)
	data[28] = byte((height - 1) >> 8)
	data[29] = byte((height - 1) >> 16)
	path := filepath.Join(t.TempDir(), "header-with-wrong-extension.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateImageSupportsWebPHeaderAndIgnoresExtension(t *testing.T) {
	path := writeWebPHeaderFixture(t, 3, 2)
	result, err := ValidateImage(context.Background(), path, ValidationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.MIMEType != MIMETypeWebP || result.Width != 3 || result.Height != 2 {
		t.Fatalf("metadata=%+v", result)
	}
}

func TestMaterializeWritesSafeManifestMetadata(t *testing.T) {
	dataHome := t.TempDir()
	store, err := New(StoreOptions{DataHome: dataHome, SessionID: "session-safe"})
	if err != nil {
		t.Fatal(err)
	}
	source := writeFixture(t, ".png", 2, 3)
	attachment, err := store.Materialize(context.Background(), MaterializeRequest{
		SourcePath:        source,
		OriginalName:      `../nested\\secret.png`,
		TurnID:            "turn-1",
		Origin:            "user",
		AllowExternalPath: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(attachment.Path) != store.AssetsDir() {
		t.Fatalf("asset path=%q is outside asset directory", attachment.Path)
	}
	if strings.ContainsAny(filepath.Base(attachment.Path), `/\\`) || len(filepath.Base(attachment.Path)) != 36 {
		t.Fatalf("asset filename=%q is not a UUID", filepath.Base(attachment.Path))
	}
	if strings.ContainsAny(attachment.Name, `/\\`) || strings.Contains(attachment.Name, "..") {
		t.Fatalf("unsafe metadata name=%q", attachment.Name)
	}

	manifestData, err := os.ReadFile(store.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		SessionID string          `json:"session_id"`
		Assets    []ManifestEntry `json:"assets"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SessionID != "session-safe" || len(manifest.Assets) != 1 {
		t.Fatalf("manifest=%+v", manifest)
	}
	entry := manifest.Assets[0]
	if entry.AssetID != attachment.ID || entry.SessionID != "session-safe" || entry.TurnID != "turn-1" || entry.Origin != "user" || entry.MIMEType != MIMETypePNG || entry.Path != attachment.Path {
		t.Fatalf("manifest entry=%+v attachment=%+v", entry, attachment)
	}
	if info, err := os.Stat(store.ManifestPath()); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("manifest mode=%o", info.Mode().Perm())
	}
}

func TestMaterializeEnforcesPerTurnLimits(t *testing.T) {
	source := writeFixture(t, ".png", 2, 2)
	store, err := New(StoreOptions{
		DataHome:  t.TempDir(),
		SessionID: "limited",
		Limits:    Limits{MaxAttachmentsPerTurn: 1, MaxPixelsPerTurn: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := MaterializeRequest{SourcePath: source, TurnID: "turn"}
	if _, err := store.Materialize(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Materialize(context.Background(), request); !errors.Is(err, ErrTooManyAttachments) {
		t.Fatalf("second attachment error=%v", err)
	}
}

func TestNewStoreHonorsXDGDataHome(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	store, err := NewStore("xdg-session")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dataHome, "hero", "sessions", "xdg-session", "assets")
	if store.AssetsDir() != want {
		t.Fatalf("assets=%q want %q", store.AssetsDir(), want)
	}
}
