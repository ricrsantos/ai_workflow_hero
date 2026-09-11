package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestFreeChatAttachmentShortcutsAndSlashBoundary(t *testing.T) {
	free := EnterConversationForTest(NewTestModel(nil))
	free.freeChatMode = true
	next, _ := HandleTestKey(free, "alt+a")
	if !next.attachmentPickerActive {
		t.Fatal("Alt+A should open the image picker in Free Chat")
	}

	workflow := EnterConversationForTest(NewTestModel(nil))
	next, _ = HandleTestKey(workflow, "alt+a")
	if next.attachmentPickerActive || !strings.Contains(StatusTextForTest(next), "Free Chat only") {
		t.Fatalf("workflow Alt+A should be ignored with a diagnostic: picker=%v status=%q", next.attachmentPickerActive, StatusTextForTest(next))
	}
	if _, _, handled := workflow.dispatchExactHeroSlash("/attach"); !handled {
		t.Fatal("workflow /attach should be handled as a blocked command")
	}
}

func TestAttachmentChipsHaveIndependentFocusAndDismiss(t *testing.T) {
	m := EnterConversationForTest(NewTestModel(nil))
	m.freeChatMode = true
	m = SetAttachmentsForTest(m, []harness.Attachment{
		{Kind: harness.MediaKindImage, Name: "first.png", Path: "/session/first.png"},
		{Kind: harness.MediaKindImage, Name: "second.png", Path: "/session/second.png"},
	})
	m, _ = HandleTestKey(m, "alt+c")
	if !m.attachmentFocus || m.chatInputFocused {
		t.Fatalf("chip focus=%v input focus=%v", m.attachmentFocus, m.chatInputFocused)
	}
	m, _ = HandleTestKey(m, "x")
	if len(m.attachments) != 1 || m.attachments[0].name != "second.png" {
		t.Fatalf("attachments after dismiss=%+v", m.attachments)
	}
	m, _ = HandleTestKey(m, "alt+c")
	if m.attachmentFocus || !m.chatInputFocused {
		t.Fatalf("chip focus should return to composer: focus=%v input=%v", m.attachmentFocus, m.chatInputFocused)
	}
}

func TestImageOnlyAttachmentHandoffPreservesOrder(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	m.freeChatMode = true
	m = EnterConversationForTest(m)
	m = SetChatHarnessIDForTest(m, "cursor")
	m = SetChatModelSlugForTest(m, "composer-2.5")
	m = SetAttachmentsForTest(m, []harness.Attachment{
		{Kind: harness.MediaKindImage, Name: "first.png", Path: "/session/first.png"},
		{Kind: harness.MediaKindImage, Name: "second.png", Path: "/session/second.png"},
	})
	m = SetConversationInput(m, "")
	next, cmd := SubmitConversationForTest(m)
	if !IsConversationStreaming(next) || cmd == nil {
		t.Fatal("image-only submission should start an asynchronous turn")
	}
	next = drainConversationStream(t, next, cmd)
	got := h.LastAttachments()
	if len(got) != 2 || got[0].Name != "first.png" || got[1].Name != "second.png" {
		t.Fatalf("attachments=%+v, want images in composer order", got)
	}
}

func TestAssetStreamAndFinalRepairDoesNotDuplicateCards(t *testing.T) {
	m := EnterConversationForTest(NewTestModel(nil))
	m.streaming = true
	streamed := harness.Asset{Attachment: harness.Attachment{ID: "stream", Name: "preview.png", ContentHash: "same"}, Source: harness.AssetSourceModel}
	final := harness.Asset{Attachment: harness.Attachment{ID: "final", Name: "saved.png", Path: "/session/saved.png", ContentHash: "same"}, Source: harness.AssetSourceModel, Saved: true}
	m = m.appendStreamDelta(harness.StreamDelta{Kind: harness.StreamKindAsset, Asset: &streamed})
	m = m.addAsset(final)
	if len(m.assets) != 1 {
		t.Fatalf("asset cards=%d, want one hash-deduplicated card", len(m.assets))
	}
	if got := len(m.renderAssetCardLines(80)); got == 0 || !strings.Contains(strings.Join(m.renderAssetCardLines(80), "\n"), "preview.png") {
		t.Fatalf("asset card rendering=%v", m.renderAssetCardLines(80))
	}
}

func TestAssetSaveDialogCopiesWithPrivateMode(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "generated.png")
	destination := filepath.Join(dir, "saved.png")
	if err := os.WriteFile(source, []byte("fixture image bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := EnterConversationForTest(NewTestModel(nil))
	m.assets = []harness.Asset{{Attachment: harness.Attachment{Name: "generated.png", Path: source, MIMEType: "image/png"}, Source: harness.AssetSourceModel}}
	m.assetFocus = true
	m.streaming = false
	m, _, handled := m.handleAssetKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !handled || !m.assetSavePending {
		t.Fatal("s should open the asset save dialog")
	}
	m, _, handled = m.handleAssetKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(destination)})
	if !handled || m.assetSaveInput != destination {
		t.Fatalf("save input=%q", m.assetSaveInput)
	}
	m, cmd, handled := m.handleAssetKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled || cmd == nil {
		t.Fatal("enter should start the asynchronous save")
	}
	msg, ok := cmd().(assetActionMsg)
	if !ok || msg.err != nil {
		t.Fatalf("save result=%T %+v", msg, msg)
	}
	updated, _ := m.Update(msg)
	result := updated.(model)
	if result.assetSavePending {
		t.Fatal("save dialog should close after completion")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "fixture image bytes" {
		t.Fatalf("saved data=%q err=%v", data, err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("saved mode=%o, want 600", info.Mode().Perm())
	}
}

func TestAssetsRenderOnTheirProducingTranscriptTurn(t *testing.T) {
	m := EnterConversationForTest(NewTestModel(nil))
	m.transcript = []convMessage{
		{role: convRoleUser, content: "first prompt"},
		{role: convRoleAgent, content: "first answer"},
		{role: convRoleUser, content: "second prompt"},
		{role: convRoleAgent, content: "second answer"},
	}
	m.agentMsgIndex = 1
	m = m.addAsset(harness.Asset{Attachment: harness.Attachment{Name: "first.png", ContentHash: "first"}})
	m.agentMsgIndex = 3
	m = m.addAsset(harness.Asset{Attachment: harness.Attachment{Name: "second.png", ContentHash: "second"}})

	if len(m.transcript[1].assets) != 1 || m.transcript[1].assets[0].Name != "first.png" {
		t.Fatalf("first turn assets=%+v", m.transcript[1].assets)
	}
	if len(m.transcript[3].assets) != 1 || m.transcript[3].assets[0].Name != "second.png" {
		t.Fatalf("second turn assets=%+v", m.transcript[3].assets)
	}
	rendered := strings.Join(m.transcriptContentLines(70), "\n")
	if strings.Index(rendered, "first answer") > strings.Index(rendered, "first.png") {
		t.Fatalf("first asset was not rendered after its answer: %q", rendered)
	}
	if strings.Index(rendered, "second answer") > strings.Index(rendered, "second.png") {
		t.Fatalf("second asset was not rendered after its answer: %q", rendered)
	}
}

func TestExternalAttachmentPolicyControlsMaterialization(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	svc, _ := newConversationTestService(t)
	configDir := filepath.Join(svc.ProjectDir, ".workflow-hero", "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	externalPath := writeAcceptancePNG(t, t.TempDir(), "external.png")

	writePolicy := func(t *testing.T, profile string) {
		t.Helper()
		data := []byte(`{"harnesses":{"streaming":{"permission_profile":"` + profile + `"}}}`)
		if err := os.WriteFile(filepath.Join(configDir, "hero.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	writePolicy(t, "auto-project")
	m := NewTestModel(svc)
	m.freeChatMode = true
	m = EnterConversationForTest(m)
	m = SetChatHarnessIDForTest(m, "streaming")
	m, cmd := m.queueAttachmentPath(externalPath)
	denied, ok := cmd().(attachmentMaterializedMsg)
	if !ok || denied.err == nil {
		t.Fatalf("auto-project external attachment result=%T %+v, want denial", denied, denied)
	}

	writePolicy(t, "auto-all")
	m = NewTestModel(svc)
	m.freeChatMode = true
	m = EnterConversationForTest(m)
	m = SetChatHarnessIDForTest(m, "streaming")
	m, cmd = m.queueAttachmentPath(externalPath)
	allowed, ok := cmd().(attachmentMaterializedMsg)
	if !ok || allowed.err != nil {
		t.Fatalf("auto-all external attachment result=%T %+v, want success", allowed, allowed)
	}
	if !allowed.external {
		t.Fatal("auto-all external attachment should retain warning metadata")
	}
}

func TestAssetSaveDialogPrefillsDownloadsAndConfirmsOverwrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	downloads := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(downloads, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "generated.png")
	destination := filepath.Join(downloads, "original.png")
	if err := os.WriteFile(source, []byte("new image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old image"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := EnterConversationForTest(NewTestModel(nil))
	m.assets = []harness.Asset{{Attachment: harness.Attachment{Name: "original.png", Path: source}, Source: harness.AssetSourceModel}}
	m.assetFocus = true
	m, _, handled := m.handleAssetKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !handled || m.assetSaveInput != destination {
		t.Fatalf("save input=%q, want %q", m.assetSaveInput, destination)
	}
	m, cmd, handled := m.handleAssetKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled || cmd == nil {
		t.Fatal("enter should check for an existing destination asynchronously")
	}
	check, ok := cmd().(assetActionMsg)
	if !ok || !check.overwriteRequired {
		t.Fatalf("overwrite check=%T %+v", check, check)
	}
	updated, _ := m.Update(check)
	m = updated.(model)
	if !m.assetSaveOverwritePending {
		t.Fatal("existing destination should require explicit overwrite confirmation")
	}
	m, cmd, handled = m.handleAssetKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !handled || cmd == nil {
		t.Fatal("y should start overwrite asynchronously")
	}
	saved, ok := cmd().(assetActionMsg)
	if !ok || saved.err != nil {
		t.Fatalf("overwrite result=%T %+v", saved, saved)
	}
	updated, _ = m.Update(saved)
	result := updated.(model)
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "new image" {
		t.Fatalf("destination data=%q err=%v", data, err)
	}
	if result.assetSavePending {
		t.Fatal("save dialog should close after confirmed overwrite")
	}
}
