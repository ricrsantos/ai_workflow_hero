package tui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/media"
)

func writeAcceptancePNG(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 1, color.RGBA{B: 255, A: 255})
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestC14AcceptancePickerChipToHarnessSend(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	svc, h := newConversationTestService(t)
	m := newModel(svc)
	m.freeChatMode = true
	m = EnterConversationForTest(m)
	m = SetChatHarnessIDForTest(m, "streaming")
	m = SetChatModelSlugForTest(m, "vision")
	path := writeAcceptancePNG(t, t.TempDir(), "picker.png")
	m, cmd := m.queueAttachmentPath(path)
	msg, ok := cmd().(attachmentMaterializedMsg)
	if !ok || msg.err != nil {
		t.Fatalf("picker materialization message=%T %+v", msg, msg)
	}
	m = m.handleAttachmentMaterialized(msg)
	m = SetConversationInput(m, "describe this")
	m, cmd = SubmitConversationForTest(m)
	if cmd == nil {
		t.Fatal("attachment chip submit did not start a command")
	}
	m = drainConversationStream(t, m, cmd)
	attachments := h.LastAttachments()
	if len(attachments) != 1 || attachments[0].Name != "picker.png" || attachments[0].ContentHash == "" {
		t.Fatalf("harness attachments=%+v", attachments)
	}
	if !strings.HasPrefix(attachments[0].Path, filepath.Join(dataHome, "hero", "sessions")) {
		t.Fatalf("attachment was not materialized under XDG data: %q", attachments[0].Path)
	}
}

func TestC14AcceptanceClipboardChipCapabilityBlockAndToolCard(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	svc, h := newConversationTestService(t)
	m := newModel(svc)
	m.freeChatMode = true
	m = EnterConversationForTest(m)
	m = SetChatHarnessIDForTest(m, "streaming")
	m = SetChatModelSlugForTest(m, "vision")
	m = SetAttachmentsForTest(m, []harness.Attachment{{Kind: harness.MediaKindImage, Name: "clipboard.png", Path: "/session/clipboard.png"}})
	chipText := strings.Join(m.renderAttachmentChipLines(100), "\n")
	if !strings.Contains(chipText, "clipboard.png") {
		t.Fatalf("clipboard chip was not rendered: %q", chipText)
	}

	registry := media.NewRegistry()
	registry.RegisterTransport("streaming", harness.MediaCapability{ImageInputNative: true})
	registry.RegisterModel("streaming", "vision", harness.MediaCapability{})
	m = SetMediaRegistryForTest(m, registry)
	m = SetConversationInput(m, "compare")
	m, cmd := SubmitConversationForTest(m)
	if cmd == nil {
		t.Fatal("capability-blocked turn did not return a completion command")
	}
	if len(m.attachments) != 1 || m.attachments[0].attachment.Name != "clipboard.png" {
		t.Fatalf("capability check removed composer chip before execution: %+v", m.attachments)
	}
	m = drainConversationStream(t, m, cmd)
	if h.ExecuteCount() != 0 || !strings.Contains(strings.ToLower(ConversationErrorForTest(m)), "does not support image input") {
		t.Fatalf("capability block execute_count=%d error=%q", h.ExecuteCount(), ConversationErrorForTest(m))
	}
	if len(m.attachments) != 1 || m.attachments[0].attachment.Name != "clipboard.png" {
		t.Fatalf("capability block removed composer chip: %+v", m.attachments)
	}

	m = m.addAsset(harness.Asset{
		Attachment: harness.Attachment{Name: "tool-output.png", MIMEType: "image/png", Path: "/session/tool-output.png", ContentHash: "tool-hash"},
		Source:     harness.AssetSourceTool,
	})
	card := strings.Join(m.renderAssetCardLines(100), "\n")
	if !strings.Contains(card, "[tool wrote]") {
		t.Fatalf("tool asset card=%q", card)
	}
	preview, err := media.RenderMosaic(image.NewRGBA(image.Rect(0, 0, 2, 2)), media.MosaicConfig{Width: 2, Height: 1, ColorDepth: media.ColorDepth256})
	if err != nil || !preview.Available || preview.Text == "" {
		t.Fatalf("mosaic preview=%+v err=%v", preview, err)
	}
}
