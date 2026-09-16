package tui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/media"
)

func TestDurableMediaPathsUseHeroSessionIDAndSurviveRetention(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	svc, h := newConversationTestService(t)
	m := newModel(svc)
	m.convSink = newRecordingSink()
	m.freeChatMode = true
	m = EnterConversationForTest(m)
	m = SetChatHarnessIDForTest(m, "streaming")
	m = SetChatModelSlugForTest(m, "vision")
	path := writeAcceptancePNG(t, t.TempDir(), "retention.png")
	m, cmd := m.queueAttachmentPath(path)
	msg, ok := cmd().(attachmentMaterializedMsg)
	if !ok || msg.err != nil {
		t.Fatalf("materialization=%T %+v", msg, msg)
	}
	m = m.handleAttachmentMaterialized(msg)
	provisional := strings.TrimSpace(m.mediaSessionID)
	if provisional == "" {
		t.Fatal("attachment should allocate provisional media session id")
	}
	mediaDir := filepath.Join(dataHome, "hero", "sessions", provisional)
	if !strings.HasPrefix(msg.attachment.Path, mediaDir) {
		t.Fatalf("attachment path=%q want under %q", msg.attachment.Path, mediaDir)
	}
	m = SetConversationInput(m, "keep media")
	m, cmd = SubmitConversationForTest(m)
	if cmd == nil {
		t.Fatal("submit did not start execute")
	}
	m = drainConversationStream(t, m, cmd)
	heroID := strings.TrimSpace(m.heroChatSessionID)
	if heroID == "" {
		t.Fatal("expected hero chat session after first turn")
	}
	if heroID != provisional {
		t.Fatalf("hero session %q != provisional media session %q", heroID, provisional)
	}
	if len(h.LastAttachments()) != 1 {
		t.Fatalf("attachments=%v", h.LastAttachments())
	}
	ids, err := svc.Store.ListRegisteredSessionIDs()
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(ids, heroID) {
		t.Fatalf("registered ids=%v missing hero %q", ids, heroID)
	}
	now := time.Now()
	oldTime := now.Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(mediaDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(dataHome, "hero", "sessions", "orphan-media-run")
	if err := os.MkdirAll(filepath.Join(orphan, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(orphan, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	result, err := media.CleanupExpiredSessions(context.Background(), media.CleanupOptionsFromRegistered(dataHome, func(context.Context) ([]string, error) {
		return svc.Store.ListRegisteredSessionIDs()
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.RemovedSessions < 1 {
		t.Fatalf("cleanup=%+v want orphan removed", result)
	}
	if _, err := os.Stat(mediaDir); err != nil {
		t.Fatalf("registered hero media dir removed: %v", err)
	}
	if _, err := os.Stat(orphan); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan media dir still exists: %v", err)
	}
}

func TestNewChatRotatesMediaSessionDirectory(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	svc, _ := newConversationTestService(t)
	m := newModel(svc)
	m.convSink = newRecordingSink()
	m.freeChatMode = true
	m = EnterConversationForTest(m)
	path := writeAcceptancePNG(t, t.TempDir(), "first.png")
	m, cmd := m.queueAttachmentPath(path)
	msg, ok := cmd().(attachmentMaterializedMsg)
	if !ok || msg.err != nil {
		t.Fatalf("materialization=%T %+v", msg, msg)
	}
	m = m.handleAttachmentMaterialized(msg)
	firstID := strings.TrimSpace(m.mediaSessionID)
	next, _ := RunPaletteItemForTest(m, "/new-chat")
	if strings.TrimSpace(next.mediaSessionID) != "" {
		t.Fatalf("new-chat should clear media session id, got %q", next.mediaSessionID)
	}
	path2 := writeAcceptancePNG(t, t.TempDir(), "second.png")
	next, cmd = next.queueAttachmentPath(path2)
	msg2, ok := cmd().(attachmentMaterializedMsg)
	if !ok || msg2.err != nil {
		t.Fatalf("second materialization=%T %+v", msg2, msg2)
	}
	secondID := strings.TrimSpace(next.mediaSessionID)
	if firstID == "" || secondID == "" || firstID == secondID {
		t.Fatalf("media session ids first=%q second=%q", firstID, secondID)
	}
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func TestChatAttachmentShortcutsAndSlashWorkInEveryConversationMode(t *testing.T) {
	modes := []struct {
		name         string
		freeChat     bool
		researchLive bool
		runtimeAgent string
	}{
		{name: "free chat", freeChat: true},
		{name: "research", researchLive: true},
		{name: "workflow agent", runtimeAgent: agentOrchestration},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			m := EnterConversationForTest(NewTestModel(nil))
			m.freeChatMode = mode.freeChat
			m.researchLive = mode.researchLive
			m.runtimeAgentName = mode.runtimeAgent

			next, cmd := HandleTestKey(m, "alt+a")
			if !next.attachmentPickerActive {
				t.Fatal("Alt+A should open the image picker")
			}
			if cmd == nil {
				t.Fatal("Alt+A should schedule an asynchronous directory load")
			}

			m = EnterConversationForTest(NewTestModel(nil))
			m.freeChatMode = mode.freeChat
			m.researchLive = mode.researchLive
			m.runtimeAgentName = mode.runtimeAgent
			next, cmd, handled := m.dispatchExactHeroSlash("/attach")
			if !handled || !next.attachmentPickerActive {
				t.Fatal("/attach should open the image picker")
			}
			if cmd == nil {
				t.Fatal("/attach should schedule an asynchronous directory load")
			}
		})
	}
}

func TestAttachmentPickerLoadsDirectoryEntriesAndSupportsSelection(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	svc, _ := newConversationTestService(t)
	svc.WorkDir = t.TempDir()
	nestedDir := filepath.Join(svc.WorkDir, "images")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAcceptancePNG(t, nestedDir, "nested.png")

	m := NewTestModel(svc)
	m.freeChatMode = true
	m = EnterConversationForTest(m)
	next, cmd := HandleTestKey(m, "alt+a")
	if cmd == nil {
		t.Fatal("Alt+A should schedule the initial directory load")
	}

	updated, _ := next.Update(cmd())
	next = updated.(model)
	if got := next.attachmentPicker.View(); !strings.Contains(got, "images") || strings.Contains(got, "Bummer. No Files Found.") {
		t.Fatalf("initial picker view=%q, want the workspace directory entry", got)
	}

	next, cmd = HandleTestKey(next, "enter")
	if cmd == nil {
		t.Fatal("opening a directory should schedule another asynchronous load")
	}
	updated, _ = next.Update(cmd())
	next = updated.(model)
	if got := next.attachmentPicker.View(); !strings.Contains(got, "nested.png") {
		t.Fatalf("nested picker view=%q, want the image entry", got)
	}
	if got := ViewForTest(next); !strings.Contains(got, "nested.png") {
		t.Fatalf("full TUI view=%q, want the picker entry", got)
	}

	for _, size := range []tea.WindowSizeMsg{
		{Width: 72, Height: 12},
		{Width: 120, Height: 40},
	} {
		resized, _ := next.Update(size)
		next = resized.(model)
		resizedView := ViewForTest(next)
		if !strings.Contains(resizedView, "nested.png") {
			t.Fatalf("resized TUI view (%dx%d)=%q, want the picker entry to remain visible", size.Width, size.Height, resizedView)
		}
		if got := len(strings.Split(stripANSI(resizedView), "\n")); got != size.Height {
			t.Fatalf("resized TUI frame has %d rows at height %d, want %d", got, size.Height, size.Height)
		}
	}

	next, cmd = HandleTestKey(next, "enter")
	if next.attachmentPickerActive {
		t.Fatal("selecting an image should close the picker")
	}
	if cmd == nil {
		t.Fatal("selecting an image should schedule asynchronous validation")
	}
	materialized, ok := cmd().(attachmentMaterializedMsg)
	if !ok || materialized.err != nil {
		t.Fatalf("materialization message=%T %+v", materialized, materialized)
	}
	updated, _ = next.Update(materialized)
	next = updated.(model)
	if len(next.attachments) != 1 || next.attachments[0].pending || next.attachments[0].name != "nested.png" {
		t.Fatalf("selected attachment=%+v, want a ready nested image", next.attachments)
	}
}

func TestAttachmentPickerKeepsEntriesVisibleWhenScrolledAndResized(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	svc, _ := newConversationTestService(t)
	svc.WorkDir = t.TempDir()
	imageDir := filepath.Join(svc.WorkDir, "images")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 24; i++ {
		name := fmt.Sprintf("file-%02d.png", i)
		if err := os.WriteFile(filepath.Join(imageDir, name), []byte("fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := NewTestModel(svc)
	m.freeChatMode = true
	m = EnterConversationForTest(m)
	next, cmd := HandleTestKey(m, "alt+a")
	if cmd == nil {
		t.Fatal("Alt+A should schedule the initial directory load")
	}
	updated, _ := next.Update(cmd())
	next = updated.(model)

	next, cmd = HandleTestKey(next, "enter")
	if cmd == nil {
		t.Fatal("opening a directory should schedule another asynchronous load")
	}
	updated, _ = next.Update(cmd())
	next = updated.(model)

	updated, _ = next.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	next = updated.(model)
	last := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	next, _ = HandleTestKeyMsg(next, last)
	if got := stripANSI(ViewForTest(next)); !strings.Contains(got, "file-23.png") {
		t.Fatalf("scrolled picker view=%q, want the last entry visible", got)
	}

	// Shrinking while the picker is scrolled must preserve a valid viewport;
	// forwarding the raw WindowSizeMsg to this Bubbles version would reset its
	// upper bound without its private lower bound and render an empty picker.
	updated, _ = next.Update(tea.WindowSizeMsg{Width: 100, Height: 8})
	next = updated.(model)
	resized := stripANSI(ViewForTest(next))
	if strings.Contains(resized, "Bummer. No Files Found.") || !strings.Contains(resized, "file-") {
		t.Fatalf("scrolled picker disappeared after resize: %q", resized)
	}
}

func TestBracketedImagePathPasteWorksInEveryConversationMode(t *testing.T) {
	modes := []struct {
		name         string
		freeChat     bool
		researchLive bool
		runtimeAgent string
	}{
		{name: "free chat", freeChat: true},
		{name: "research", researchLive: true},
		{name: "workflow agent", runtimeAgent: agentOrchestration},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			m := EnterConversationForTest(NewTestModel(nil))
			m.freeChatMode = mode.freeChat
			m.researchLive = mode.researchLive
			m.runtimeAgentName = mode.runtimeAgent
			path := "/tmp/" + strings.ReplaceAll(mode.name, " ", "-") + ".png"

			next, cmd := HandleTestKeyMsg(m, tea.KeyMsg{Paste: true, Runes: []rune(path)})
			if len(next.attachments) != 1 || next.attachments[0].attachment.Path != path {
				t.Fatalf("pasted path attachment=%+v, want %q", next.attachments, path)
			}
			if cmd == nil {
				t.Fatal("bracketed image-path paste should start asynchronous validation")
			}
		})
	}
}

func TestDraggedImagePathPasteNormalizesTerminalPayloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "design with spaces.png")
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	escaped := strings.ReplaceAll(path, " ", `\ `)
	cases := []struct {
		name    string
		payload string
	}{
		{name: "plain path", payload: path},
		{name: "single quoted path", payload: "'" + path + "'"},
		{name: "double quoted path", payload: `"` + path + `"`},
		{name: "shell escaped path", payload: escaped},
		{name: "local file URI", payload: uri},
		{name: "quoted local file URI", payload: `"` + uri + `"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := imagePathFromTerminalPaste(tc.payload)
			if !ok || got != path {
				t.Fatalf("payload %q normalized to %q, recognized=%v; want %q", tc.payload, got, ok, path)
			}
		})
	}
}

func TestDraggedImagePathPasteQueuesAttachmentAndKeepsTextPasteText(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	svc, _ := newConversationTestService(t)
	path := writeAcceptancePNG(t, t.TempDir(), "dropped image.png")

	for _, tc := range []struct {
		name    string
		payload string
	}{
		{name: "quoted path", payload: "'" + path + "'"},
		{name: "file URI", payload: (&url.URL{Scheme: "file", Path: path}).String()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewTestModel(svc)
			m.freeChatMode = true
			m = EnterConversationForTest(m)

			next, cmd := HandleTestKeyMsg(m, tea.KeyMsg{Type: tea.KeyRunes, Paste: true, Runes: []rune(tc.payload)})
			if len(next.attachments) != 1 || next.attachments[0].attachment.Path != path {
				t.Fatalf("attachments=%+v, want source path %q", next.attachments, path)
			}
			if next.input != "" {
				t.Fatalf("dragged path leaked into composer input=%q", next.input)
			}
			if cmd == nil {
				t.Fatal("dragged image should start asynchronous validation")
			}
			materialized, ok := cmd().(attachmentMaterializedMsg)
			if !ok || materialized.err != nil {
				t.Fatalf("dragged image materialization=%T %+v", materialized, materialized)
			}
			next = next.handleAttachmentMaterialized(materialized)
			if next.attachments[0].pending || next.attachments[0].err != "" {
				t.Fatalf("dragged image chip after materialization=%+v", next.attachments[0])
			}
			if next.assetFocus || next.attachmentFocus || !next.chatInputFocused {
				t.Fatalf("dragged image should leave the composer focused: asset=%v attachment=%v input=%v", next.assetFocus, next.attachmentFocus, next.chatInputFocused)
			}
			next, cmd = HandleTestKey(next, "enter")
			if cmd != nil || next.input != "\n" {
				t.Fatalf("enter after drag should create a new line: input=%q cmd=%v", next.input, cmd != nil)
			}
		})
	}

	unbracketedDrop := NewTestModel(svc)
	unbracketedDrop.freeChatMode = true
	unbracketedDrop = EnterConversationForTest(unbracketedDrop)
	next, cmd := HandleTestKeyMsg(unbracketedDrop, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("'" + path + "'")})
	if len(next.attachments) != 1 || next.attachments[0].attachment.Path != path || next.input != "" || cmd == nil {
		t.Fatalf("quoted unbracketed drop was not promoted: attachments=%+v input=%q cmd=%v", next.attachments, next.input, cmd != nil)
	}

	splitDrop := NewTestModel(svc)
	splitDrop.freeChatMode = true
	splitDrop = EnterConversationForTest(splitDrop)
	for _, r := range []rune("'" + path + "'") {
		splitDrop, cmd = HandleTestKeyMsg(splitDrop, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(splitDrop.attachments) != 1 || splitDrop.attachments[0].attachment.Path != path || splitDrop.input != "" || cmd == nil {
		t.Fatalf("split quoted unbracketed drop was not promoted: attachments=%+v input=%q cmd=%v", splitDrop.attachments, splitDrop.input, cmd != nil)
	}

	plainPaste := NewTestModel(svc)
	plainPaste.freeChatMode = true
	plainPaste = EnterConversationForTest(plainPaste)
	next, cmd = HandleTestKeyMsg(plainPaste, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(path)})
	if len(next.attachments) != 0 || next.input != path || cmd != nil {
		t.Fatalf("ordinary unbracketed path paste changed attachment state: attachments=%+v input=%q cmd=%v", next.attachments, next.input, cmd != nil)
	}
}

func TestTerminalImagePathPasteRejectsNonLocalFileURIs(t *testing.T) {
	for _, payload := range []string{
		"file://remote-host/tmp/image.png",
		"file:///tmp/image.png?download=1",
		"file:///tmp/image.png#fragment",
	} {
		if path, ok := imagePathFromTerminalPaste(payload); ok {
			t.Fatalf("remote or qualified URI %q recognized as local path %q", payload, path)
		}
	}
}

func TestHeroControlSlashRejectsStagedAttachments(t *testing.T) {
	m := EnterConversationForTest(NewTestModel(nil))
	m = SetAttachmentsForTest(m, []harness.Attachment{{
		Kind: harness.MediaKindImage,
		Name: "control.png",
		Path: "/session/control.png",
	}})
	m = SetConversationInput(m, "/hero-start")

	next, cmd := SubmitConversationForTest(m)
	if cmd != nil || IsConversationStreaming(next) {
		t.Fatal("Hero control command with an attachment must not execute")
	}
	if len(next.attachments) != 1 {
		t.Fatalf("control rejection should preserve the chip: %+v", next.attachments)
	}
	if !strings.Contains(StatusTextForTest(next), "control commands") {
		t.Fatalf("status=%q", StatusTextForTest(next))
	}
}

func TestWorkflowAttachmentReachesHarnessAndClearsAfterSuccess(t *testing.T) {
	m, h, _ := newConversationTestModel(t)
	m = EnterConversationForTest(m)
	m.runtimeAgentName = agentOrchestration
	m.runtimeHarnessID = "streaming"
	m.runtimeModelSlug = "vision"
	m = SetAttachmentsForTest(m, []harness.Attachment{{
		Kind: harness.MediaKindImage,
		Name: "workflow.png",
		Path: "/session/workflow.png",
	}})
	m = SetConversationInput(m, "describe this workflow image")

	next, cmd := SubmitConversationForTest(m)
	if !IsConversationStreaming(next) || cmd == nil {
		t.Fatal("workflow attachment submission should start an asynchronous turn")
	}
	next = drainConversationStream(t, next, cmd)

	got := h.LastAttachments()
	if len(got) != 1 || got[0].Name != "workflow.png" {
		t.Fatalf("harness attachments=%+v", got)
	}
	if len(next.attachments) != 0 {
		t.Fatalf("successful workflow turn left composer chips behind: %+v", next.attachments)
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
	if len(next.attachments) != 0 {
		t.Fatalf("successful image turn left composer chips behind: %+v", next.attachments)
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

func TestAssetCardsUseExternalActionsWithoutInlinePreview(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated.png")
	if err := os.WriteFile(path, []byte("fixture image bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := EnterConversationForTest(NewTestModel(nil))
	m.assets = []harness.Asset{{Attachment: harness.Attachment{Name: "generated.png", Path: path}, Source: harness.AssetSourceModel}}
	m.assetFocus = true
	m.assetCursor = 0
	rendered := strings.Join(m.renderAssetCardLines(80), "\n")
	if strings.Contains(strings.ToLower(rendered), "preview") || strings.Contains(strings.ToLower(rendered), "mosaic") {
		t.Fatalf("asset card still advertises inline preview: %q", rendered)
	}
	if !strings.Contains(rendered, "enter/o open") {
		t.Fatalf("asset card is missing the external open action: %q", rendered)
	}
	_, cmd, handled := m.handleAssetKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled || cmd == nil {
		t.Fatal("enter should open the selected asset asynchronously")
	}
}

func TestAltGFocusesAssetCardsAndDoesNotLeaveStaleNavbarFocus(t *testing.T) {
	m := EnterConversationForTest(NewTestModel(nil))
	m.assets = []harness.Asset{
		{Attachment: harness.Attachment{Name: "first.png", Path: "/tmp/first.png"}},
		{Attachment: harness.Attachment{Name: "second.png", Path: "/tmp/second.png"}},
	}
	m.shellFocus = shellFocusNavbar
	navCursor := m.navCursor

	next, _ := HandleTestKey(m, "alt+g")
	if !next.assetFocus || next.shellFocus != shellFocusContent || next.chatInputFocused {
		t.Fatalf("alt+g focus state=%+v", next)
	}
	next, _ = HandleTestKey(next, "down")
	if !next.assetFocus || next.assetCursor != 1 || next.shellFocus != shellFocusContent || next.navCursor != navCursor {
		t.Fatalf("card navigation leaked to navbar: asset=%v cursor=%d shell=%v nav=%d", next.assetFocus, next.assetCursor, next.shellFocus, next.navCursor)
	}
	next, _ = HandleTestKey(next, "esc")
	if next.assetFocus || next.shellFocus != shellFocusContent || !next.chatInputFocused {
		t.Fatalf("esc did not return to composer: asset=%v shell=%v input=%v", next.assetFocus, next.shellFocus, next.chatInputFocused)
	}
	next, _ = HandleTestKey(next, "tab")
	if next.shellFocus != shellFocusNavbar || next.assetFocus || next.attachmentFocus {
		t.Fatalf("tab should move to navbar without stale media focus: shell=%v asset=%v attachment=%v", next.shellFocus, next.assetFocus, next.attachmentFocus)
	}
	next, _ = HandleTestKey(next, "alt+g")
	if !next.assetFocus || next.shellFocus != shellFocusContent || next.chatInputFocused {
		t.Fatalf("alt+g should recover card focus from navbar: shell=%v asset=%v input=%v", next.shellFocus, next.assetFocus, next.chatInputFocused)
	}
}

func TestAttachmentChipsExposeOpenCopySaveAndRemove(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "attached.png")
	destination := filepath.Join(dir, "exported.png")
	if err := os.WriteFile(source, []byte("fixture image bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := EnterConversationForTest(NewTestModel(nil))
	m.attachments = []tuiAttachment{{
		token: "attachment-1",
		name:  "attached.png",
		attachment: harness.Attachment{
			Kind:     harness.MediaKindImage,
			Name:     "attached.png",
			MIMEType: "image/png",
			Path:     source,
		},
	}}
	m.attachmentFocus = true
	m.shellFocus = shellFocusContent

	rendered := strings.Join(m.renderAttachmentChipLines(120), "\n")
	for _, action := range []string{"enter/o open", "c copy path", "s save", "x remove"} {
		if !strings.Contains(rendered, action) {
			t.Fatalf("attachment chip missing %q: %q", action, rendered)
		}
	}
	_, cmd, handled := m.handleAttachmentKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled || cmd == nil {
		t.Fatal("enter should open the selected attachment asynchronously")
	}
	_, cmd, handled = m.handleAttachmentKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if !handled || cmd == nil {
		t.Fatal("c should copy the selected attachment path asynchronously")
	}
	m, _, handled = m.handleAttachmentKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !handled || !m.assetSavePending || m.assetSaveTarget != mediaSaveTargetAttachment {
		t.Fatalf("s should open the attachment save dialog: pending=%v target=%d", m.assetSavePending, m.assetSaveTarget)
	}
	m, _, handled = m.handleAttachmentKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(destination)})
	if !handled || m.assetSaveInput != destination {
		t.Fatalf("attachment save input=%q, want %q", m.assetSaveInput, destination)
	}
	m, cmd, handled = m.handleAttachmentKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled || cmd == nil {
		t.Fatal("enter should start the attachment save asynchronously")
	}
	saved, ok := cmd().(assetActionMsg)
	if !ok || saved.err != nil {
		t.Fatalf("attachment save result=%T %+v", saved, saved)
	}
	updated, _ := m.Update(saved)
	m = updated.(model)
	if m.assetSavePending {
		t.Fatal("attachment save dialog should close after completion")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "fixture image bytes" {
		t.Fatalf("saved attachment data=%q err=%v", data, err)
	}
	m, _, handled = m.handleAttachmentKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !handled || len(m.attachments) != 0 {
		t.Fatalf("x should remove the selected attachment: handled=%v attachments=%d", handled, len(m.attachments))
	}
}

func TestStreamAssetUsesProducingExecuteTurn(t *testing.T) {
	m := EnterConversationForTest(NewTestModel(nil))
	m.transcript = []convMessage{
		{role: convRoleUser, content: "first prompt"},
		{role: convRoleAgent, content: "first answer"},
		{role: convRoleUser, content: "second prompt"},
		{role: convRoleAgent, content: "second answer"},
	}
	m.executes = map[string]convExecute{
		"first":  {ID: "first", AgentMsgIndex: 1},
		"second": {ID: "second", AgentMsgIndex: 3},
	}
	m.agentMsgIndex = 3
	(&m).applyStreamDelta(streamDeltaMsg{
		executeID: "first",
		delta: harness.StreamDelta{
			Kind:  harness.StreamKindAsset,
			Asset: &harness.Asset{Attachment: harness.Attachment{Name: "first-stream.png", ContentHash: "first-stream"}},
		},
	})
	(&m).applyStreamDelta(streamDeltaMsg{
		executeID: "second",
		delta: harness.StreamDelta{
			Kind:  harness.StreamKindAsset,
			Asset: &harness.Asset{Attachment: harness.Attachment{Name: "second-stream.png", ContentHash: "second-stream"}},
		},
	})

	if len(m.transcript[1].assets) != 1 || m.transcript[1].assets[0].Name != "first-stream.png" {
		t.Fatalf("first execute assets=%+v", m.transcript[1].assets)
	}
	if len(m.transcript[3].assets) != 1 || m.transcript[3].assets[0].Name != "second-stream.png" {
		t.Fatalf("second execute assets=%+v", m.transcript[3].assets)
	}
	rendered := strings.Join(m.transcriptContentLines(70), "\n")
	if ai, pi := strings.Index(rendered, "first answer"), strings.Index(rendered, "first-stream.png"); ai < 0 || pi < 0 || pi > ai {
		t.Fatalf("first stream asset should render before its answer: %q", rendered)
	}
	if ai, pi := strings.Index(rendered, "second answer"), strings.Index(rendered, "second-stream.png"); ai < 0 || pi < 0 || pi > ai {
		t.Fatalf("second stream asset should render before its answer: %q", rendered)
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
	if ai, pi := strings.Index(rendered, "first answer"), strings.Index(rendered, "first.png"); ai < 0 || pi < 0 || pi > ai {
		t.Fatalf("first asset should render before its answer: %q", rendered)
	}
	if ai, pi := strings.Index(rendered, "second answer"), strings.Index(rendered, "second.png"); ai < 0 || pi < 0 || pi > ai {
		t.Fatalf("second asset should render before its answer: %q", rendered)
	}
}

func TestExternalAttachmentPolicyControlsMaterialization(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	svc, _ := newConversationTestService(t)
	configDir := filepath.Join(svc.ProjectDir, ".workflow-hero", "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	svc.WorkDir = filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(svc.WorkDir, 0o700); err != nil {
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
