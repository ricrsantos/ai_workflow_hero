package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/vault"
)

type imageFakeBot struct {
	mu          sync.Mutex
	photos      []Photo
	chatIDs     []string
	photoCalls  int
	failAtCalls map[int]error
}

func (b *imageFakeBot) GetUpdates(context.Context, int64) ([]Update, error) { return nil, nil }
func (b *imageFakeBot) SendMessage(context.Context, string, string) error   { return nil }

func (b *imageFakeBot) SendPhoto(_ context.Context, chatID string, photo Photo) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.photoCalls++
	b.photos = append(b.photos, Photo{Filename: photo.Filename, Caption: photo.Caption, Data: append([]byte(nil), photo.Data...)})
	b.chatIDs = append(b.chatIDs, chatID)
	return b.failAtCalls[b.photoCalls]
}

func (b *imageFakeBot) snapshot() (calls int, photos []Photo, chats []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.photoCalls, append([]Photo(nil), b.photos...), append([]string(nil), b.chatIDs...)
}

func TestImageDeliveryAlwaysSendAndAddressGates(t *testing.T) {
	bot := &imageFakeBot{}
	d, reg, refs := newImageDeliveryDaemonFixture(t, bot, false, 1)
	result := d.deliverImageBatch(context.Background(), reg, imageBatch("gate-test", refs))
	if result.ImageDeliveryErrorCode != "always_send_disabled" || len(result.FailedScreenshotIDs) != 1 {
		t.Fatalf("disabled Always-send result=%+v", result)
	}
	if calls, _, _ := bot.snapshot(); calls != 0 {
		t.Fatalf("sent %d photos while Always-send was disabled", calls)
	}
	reg.mode = ipc.ModeFree
	result = d.deliverImageBatch(context.Background(), reg, imageBatch("free-chat", refs))
	if result.ImageDeliveryErrorCode != "address_not_available" {
		t.Fatalf("free-chat image result=%+v", result)
	}
}

func TestImageDeliveryPartialBatchRetriesWithDurableDedup(t *testing.T) {
	bot := &imageFakeBot{failAtCalls: map[int]error{1: errors.New("injected Bot API failure")}}
	d, reg, refs := newImageDeliveryDaemonFixture(t, bot, true, 3)
	message := imageBatch("batch-retry-test", refs)
	first := d.deliverImageBatch(context.Background(), reg, message)
	if first.ImageDeliveryErrorCode != "bot_api_delivery_failed" || strings.Join(first.FailedScreenshotIDs, ",") != refs[0].ScreenshotID || len(first.DeliveredScreenshotIDs) != 2 {
		t.Fatalf("partial result=%+v", first)
	}
	second := d.deliverImageBatch(context.Background(), reg, message)
	if len(second.FailedScreenshotIDs) != 0 || len(second.DeliveredScreenshotIDs) != 3 {
		t.Fatalf("retry result=%+v", second)
	}
	calls, photos, chats := bot.snapshot()
	if calls != 4 || len(photos) != 4 || len(chats) != 4 {
		t.Fatalf("retry calls=%d photos=%d chats=%d, want one failed + two delivered + one retry", calls, len(photos), len(chats))
	}
	for _, chatID := range chats {
		if chatID != "CHAT" {
			t.Fatalf("image sent to unexpected chat address %q", chatID)
		}
	}
	for _, photo := range photos {
		if strings.Contains(photo.Caption, "/") || strings.Contains(photo.Caption, "Operator") || len(photo.Data) == 0 {
			t.Fatalf("unsafe caption or empty image data: %+v", photo)
		}
	}
	_, _, _ = bot.snapshot()
	third := d.deliverImageBatch(context.Background(), reg, message)
	if len(third.FailedScreenshotIDs) != 0 || len(third.DeliveredScreenshotIDs) != 3 {
		t.Fatalf("dedup result=%+v", third)
	}
	if calls, _, _ := bot.snapshot(); calls != 4 {
		t.Fatalf("durable dedup resent photos: calls=%d want 4", calls)
	}
}

func TestImageDeliveryRejectsUnmanagedReferenceAndBatchOverflow(t *testing.T) {
	bot := &imageFakeBot{}
	d, reg, refs := newImageDeliveryDaemonFixture(t, bot, true, 5)
	unsafe := append([]ipc.ScreenshotImageRef(nil), refs[:1]...)
	unsafe[0].Path = "../../credentials.json"
	bad := d.deliverImageBatch(context.Background(), reg, imageBatch("unsafe-ref", unsafe))
	if bad.ImageDeliveryErrorCode != "invalid_reference" || len(bad.DeliveredScreenshotIDs) != 0 {
		t.Fatalf("unmanaged ref result=%+v", bad)
	}
	large := d.deliverImageBatch(context.Background(), reg, imageBatch("too-many", refs))
	if large.ImageDeliveryErrorCode != "invalid_batch" || len(large.FailedScreenshotIDs) != 5 {
		t.Fatalf("oversized batch result=%+v", large)
	}
	if calls, _, _ := bot.snapshot(); calls != 0 {
		t.Fatalf("invalid batches sent %d photos", calls)
	}
}

func TestImageDeliveryHTTPUsesMultipartAndInjectedTransport(t *testing.T) {
	const token = "test-token-never-log"
	imageBytes := testPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/bot"+token+"/sendPhoto") {
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Errorf("content type=%q", r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			w.WriteHeader(400)
			return
		}
		if got := r.FormValue("chat_id"); got != "CHAT" {
			t.Errorf("chat_id=%q", got)
		}
		if got := r.FormValue("caption"); got != "screen result" {
			t.Errorf("caption=%q", got)
		}
		file, _, err := r.FormFile("photo")
		if err != nil {
			t.Errorf("photo part: %v", err)
		} else {
			defer func() { _ = file.Close() }()
			data, readErr := io.ReadAll(file)
			if readErr != nil || !bytes.Equal(data, imageBytes) {
				t.Errorf("photo payload differs: err=%v", readErr)
			}
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	t.Cleanup(server.Close)
	api := NewHTTPBotAPI(token)
	api.baseURL = server.URL
	api.client = server.Client()
	if err := api.SendPhoto(context.Background(), "CHAT", Photo{Filename: "shot.png", Caption: "screen result", Data: imageBytes}); err != nil {
		t.Fatalf("SendPhoto: %v", err)
	}
}

func TestImageDeliveryIPCReturnsAddressedResult(t *testing.T) {
	bot := &imageFakeBot{}
	d, originalReg, refs := newImageDeliveryDaemonFixture(t, bot, true, 1)
	server, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		d.handleConn(context.Background(), server)
		close(done)
	}()
	client := ipc.NewConn(clientConn)
	t.Cleanup(func() {
		_ = clientConn.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("image IPC connection did not stop")
		}
	})
	if err := client.Send(ipc.Message{
		Type: ipc.TypeRegister, ProjectDir: originalReg.projectDir,
		Mode: ipc.ModeCycle, ProjectAbbrev: "image-test", ClientCapabilities: []string{ipc.CapabilityImageDelivery}, UID: 1,
	}); err != nil {
		t.Fatal(err)
	}
	registered, err := client.Recv()
	if err != nil || registered.Type != ipc.TypeRegistered || !registered.HasCapability(ipc.CapabilityImageDelivery) {
		t.Fatalf("registration=%+v err=%v", registered, err)
	}
	if err := client.Send(imageBatch("ipc-image", refs)); err != nil {
		t.Fatal(err)
	}
	result, err := client.Recv()
	if err != nil || result.Type != ipc.TypeImageDeliveryResult || result.ImageBatchID != "ipc-image" || len(result.DeliveredScreenshotIDs) != 1 || len(result.FailedScreenshotIDs) != 0 {
		t.Fatalf("delivery result=%+v err=%v", result, err)
	}
	if err := client.Send(ipc.Message{Type: ipc.TypeUnregister}); err != nil {
		t.Fatal(err)
	}
}

func newImageDeliveryDaemonFixture(t *testing.T, bot *imageFakeBot, alwaysSend bool, count int) (*Daemon, *client, []ipc.ScreenshotImageRef) {
	t.Helper()
	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, ".workflow-hero", "config"), 0o700); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf(`{"telegram":{"always_send":%t}}`, alwaysSend)
	if err := os.WriteFile(filepath.Join(projectDir, ".workflow-hero", "config", "hero.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	heroStore, err := store.OpenProject(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	cycleID, err := heroStore.CreateCycle(store.Cycle{Number: 1, Title: "images", Status: store.CycleStatusActive, StartedAt: time.Now().UTC().Format(time.RFC3339), ConfigSnapshotJSON: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if err := heroStore.CreateStages([]store.Stage{{CycleID: cycleID, Name: "browser_ui_validation", Status: store.StageRunning, SortOrder: 0, MaxIterations: 1}}); err != nil {
		t.Fatal(err)
	}
	currentScreens := filepath.Join(projectDir, ".workflow-hero", "cycles", "current", "screenshots")
	if err := os.MkdirAll(currentScreens, 0o700); err != nil {
		t.Fatal(err)
	}
	refs := make([]ipc.ScreenshotImageRef, 0, count)
	for index := 0; index < count; index++ {
		id := fmt.Sprintf("shot-%03d", index+1)
		filename := id + ".png"
		imageData := testPNG(t)
		if err := os.WriteFile(filepath.Join(currentScreens, filename), imageData, 0o400); err != nil {
			t.Fatal(err)
		}
		manifest := store.ScreenshotManifest{
			ID: id, CycleID: cycleID, StageName: "browser_ui_validation", Attempt: 1,
			CoverageID: fmt.Sprintf("screen-%d", index+1), UserID: "operator", ProfileID: "Operator",
			CapturedAt: time.Date(2026, 10, 1, 12, index, 0, 0, time.UTC).Format(time.RFC3339Nano),
			Path:       "screenshots/" + filename, Result: store.ScreenshotResultPassed, CaptureStatus: store.ScreenshotCaptureReady,
		}
		if err := heroStore.InsertScreenshotManifest(context.Background(), manifest); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ipc.ScreenshotImageRef{
			ScreenshotID: manifest.ID, CycleID: manifest.CycleID, StageName: manifest.StageName,
			Attempt: manifest.Attempt, CoverageID: manifest.CoverageID, UserID: manifest.UserID,
			ProfileID: manifest.ProfileID, CapturedAt: manifest.CapturedAt, Path: manifest.Path, Result: manifest.Result,
		})
	}
	if err := heroStore.Close(); err != nil {
		t.Fatal(err)
	}
	daemonStore := openTestStore(t)
	t.Cleanup(func() { _ = daemonStore.Close() })
	d := New(Options{Bot: bot, Vault: vault.NewMemory(), Store: daemonStore, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), UID: 1})
	d.setCreds("test-token", "CHAT")
	reg, _ := d.registry.register(projectDir, ipc.ModeCycle, "proj", make(chan ipc.Message, 8))
	reg.capabilities = []string{ipc.CapabilityImageDelivery}
	return d, reg, refs
}

func imageBatch(id string, refs []ipc.ScreenshotImageRef) ipc.Message {
	return ipc.Message{Type: ipc.TypeOutboundImageBatch, ImageBatchID: id, Images: append([]ipc.ScreenshotImageRef(nil), refs...)}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 24, 16))
	img.Set(0, 0, color.RGBA{R: 0x48, G: 0x65, B: 0x72, A: 0xff})
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
