package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/media"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// tuiAttachment is UI-only state. The image bytes stay in the media store;
// Bubble Tea messages and the model carry references and safe metadata only.
type tuiAttachment struct {
	token      string
	name       string
	attachment harness.Attachment
	pending    bool
	external   bool
	err        string
}

type attachmentMaterializedMsg struct {
	token      string
	attachment harness.Attachment
	external   bool
	err        error
}

type assetActionMsg struct {
	status            string
	err               error
	clearSave         bool
	overwriteRequired bool
}

type mediaSaveTarget uint8

const (
	mediaSaveTargetNone mediaSaveTarget = iota
	mediaSaveTargetAsset
	mediaSaveTargetAttachment
)

type mediaCleanupMsg struct {
	err error
}

func newMultimodalState() (string, string) {
	return "", uuid.NewString()
}

// durableMediaSessionID returns the Hero session id used for on-disk media paths.
// It prefers the bound Hero chat session; otherwise it allocates a provisional
// id once into mediaSessionID so first-turn SQLite creation can reuse it.
func (m *model) durableMediaSessionID() string {
	if hero := strings.TrimSpace(m.heroChatSessionID); hero != "" {
		return hero
	}
	if provisional := strings.TrimSpace(m.mediaSessionID); provisional != "" {
		return provisional
	}
	id, err := store.NewSessionID()
	if err != nil {
		id = uuid.NewString()
	}
	m.mediaSessionID = id
	return id
}

func (m *model) syncMediaSessionFromHero() {
	if id := strings.TrimSpace(m.heroChatSessionID); id != "" {
		m.mediaSessionID = id
	}
}

func (m model) mediaStartupCleanupCmd() tea.Cmd {
	opts := media.CleanupOptions{}
	if m.svc != nil && m.svc.Store != nil {
		st := m.svc.Store
		opts = media.CleanupOptionsFromRegistered("", func(context.Context) ([]string, error) {
			return st.ListRegisteredSessionIDs()
		})
	}
	return func() tea.Msg {
		_, err := media.CleanupExpiredSessions(context.Background(), opts)
		return mediaCleanupMsg{err: err}
	}
}

func (m model) multimodalConversation() bool {
	return m.screen == screenConversation && !m.todoControlActive() && !m.awaitingRejectReason
}

// attachmentPermissionProfile reads the persisted policy from the Hero
// configuration root. Chat execution may use a different workspace, so using
// executeDir here would skip the project configuration and silently fall back
// to Ask.
func (m model) attachmentPermissionProfile() harness.PermissionProfile {
	profile := harness.PermissionProfileAsk
	configRoot := ""
	if m.svc != nil {
		configRoot = m.svc.ProjectDir
	}
	if strings.TrimSpace(configRoot) == "" {
		configRoot = m.executeDir()
	}
	if strings.TrimSpace(configRoot) != "" {
		if hero, err := install.LoadHeroJSON(configRoot); err == nil {
			profile = install.HarnessPermissionProfile(hero, m.conversationHarnessTool())
		}
	}
	return harness.NormalizePermissionProfile(profile)
}

// allowExternalAttachmentPaths maps Hero's file-permission policy to the
// attachment boundary. An explicit attachment selection satisfies the Ask
// profile; Auto-project is the only profile that denies paths outside the
// workspace, while Auto-all permits them.
func (m model) allowExternalAttachmentPaths() bool {
	return m.attachmentPermissionProfile() != harness.PermissionProfileAutoProject
}

func (m model) openAttachmentPicker() (model, tea.Cmd) {
	if !m.multimodalConversation() {
		return m.setStatusResult(false, "attach", "image attachments are available in Chat only"), nil
	}
	m = m.clearMediaFocus()
	picker := filepicker.New()
	picker.AllowedTypes = []string{".png", ".jpg", ".jpeg", ".gif", ".webp"}
	picker.CurrentDirectory = m.executeDir()
	if strings.TrimSpace(picker.CurrentDirectory) == "" {
		picker.CurrentDirectory = "."
	}
	picker.DirAllowed = false
	picker.FileAllowed = true
	picker.ShowHidden = false
	picker.AutoHeight = false
	m.attachmentPicker = picker
	m.attachmentPickerActive = true
	m.chatInputFocused = false
	m = m.resizeAttachmentPicker()
	return m, m.attachmentPicker.Init()
}

func (m model) handleAttachmentPickerKey(msg tea.KeyMsg) (model, tea.Cmd) {
	if !m.attachmentPickerActive {
		return m, nil
	}
	if msg.String() == "esc" {
		m.attachmentPickerActive = false
		m = m.clearMediaFocus()
		return m, nil
	}
	m, cmd := m.handleAttachmentPickerMsg(msg)
	if selected, path := m.attachmentPicker.DidSelectFile(msg); selected {
		m.attachmentPickerActive = false
		return m.queueAttachmentPath(path)
	}
	return m, cmd
}

// handleAttachmentPickerMsg forwards asynchronous child messages to the
// mounted Bubbles picker. Its directory result type is intentionally private
// to Bubbles, so the root model must route otherwise-unhandled messages here.
func (m model) handleAttachmentPickerMsg(msg tea.Msg) (model, tea.Cmd) {
	if !m.attachmentPickerActive {
		return m, nil
	}
	picker, cmd := m.attachmentPicker.Update(msg)
	m.attachmentPicker = picker
	return m, cmd
}

// resizeAttachmentPicker keeps the focused picker inside the content pane.
// The parent owns the pane because the picker is rendered alongside the TUI
// footer rather than as a standalone Bubble Tea program.
func (m model) resizeAttachmentPicker() model {
	if !m.attachmentPickerActive {
		return m
	}
	height := m.frameContentHeight()
	if height < 1 {
		height = 1
	}
	m.attachmentPicker.SetHeight(height)
	return m
}

func (m model) queueAttachmentPath(path string) (model, tea.Cmd) {
	if !m.multimodalConversation() {
		return m.setStatusResult(false, "attach", "image attachments are available in Chat only"), nil
	}
	if len(m.attachments) >= media.DefaultMaxAttachmentsPerTurn {
		return m.setStatusResult(false, "attach", "a turn can contain at most 5 images"), nil
	}
	path = normalizeAttachmentPath(path)
	if path == "" {
		return m.setStatusResult(false, "attach", "image path is required"), nil
	}
	token := uuid.NewString()
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "image"
	}
	m.attachments = append(m.attachments, tuiAttachment{
		token: token,
		name:  name,
		attachment: harness.Attachment{
			Kind: harness.MediaKindImage,
			Name: name,
			Path: path,
		},
		pending: true,
	})
	m = m.clearMediaFocus()
	pm := &m
	sessionID := pm.durableMediaSessionID()
	return *pm, m.materializeAttachmentCmd(token, path, name, sessionID)
}

func (m model) materializeAttachmentCmd(token, sourcePath, name, sessionID string) tea.Cmd {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	turnID := strings.TrimSpace(m.attachmentTurnID)
	if turnID == "" {
		turnID = uuid.NewString()
	}
	workspace := m.executeDir()
	allowExternal := m.allowExternalAttachmentPaths()
	return func() tea.Msg {
		validation, err := media.ValidateImage(context.Background(), sourcePath, media.ValidationOptions{
			Workspace:         workspace,
			AllowExternalPath: allowExternal,
			Limits:            media.DefaultLimits(),
		})
		if err != nil {
			return attachmentMaterializedMsg{token: token, err: safeAttachmentError(err)}
		}
		st, err := media.New(media.StoreOptions{
			SessionID:          sessionID,
			Workspace:          workspace,
			AllowExternalPaths: allowExternal,
		})
		if err != nil {
			return attachmentMaterializedMsg{token: token, err: safeAttachmentError(err)}
		}
		attachment, err := st.Materialize(context.Background(), media.MaterializeRequest{
			SourcePath:        validation.CanonicalPath,
			OriginalName:      name,
			TurnID:            turnID,
			Origin:            string(harness.AssetSourceUser),
			Workspace:         workspace,
			AllowExternalPath: allowExternal,
		})
		if err != nil {
			return attachmentMaterializedMsg{token: token, err: safeAttachmentError(err)}
		}
		return attachmentMaterializedMsg{token: token, attachment: attachment, external: validation.ExternalPathWarning}
	}
}

func safeAttachmentError(err error) error {
	if err == nil {
		return nil
	}
	// Validation errors are already path/byte safe. Keep the UI message bounded
	// so a provider or filesystem diagnostic cannot consume the transcript.
	text := strings.TrimSpace(err.Error())
	if len(text) > 240 {
		text = text[:240] + "..."
	}
	return errors.New(text)
}

func (m model) startClipboardAttachment() (model, tea.Cmd) {
	if !m.multimodalConversation() {
		return m.setStatusResult(false, "attach", "image attachments are available in Chat only"), nil
	}
	if len(m.attachments) >= media.DefaultMaxAttachmentsPerTurn {
		return m.setStatusResult(false, "attach", "a turn can contain at most 5 images"), nil
	}
	token := uuid.NewString()
	m.attachments = append(m.attachments, tuiAttachment{
		token: token,
		name:  "clipboard.png",
		attachment: harness.Attachment{
			Kind: harness.MediaKindImage,
			Name: "clipboard.png",
		},
		pending: true,
	})
	m = m.clearMediaFocus()
	pm := &m
	sessionID := pm.durableMediaSessionID()
	turnID := pm.attachmentTurnID
	workspace := pm.executeDir()
	allowExternal := pm.allowExternalAttachmentPaths()
	return *pm, func() tea.Msg {
		data, err := readNativeClipboardPNG(context.Background())
		if err != nil {
			return attachmentMaterializedMsg{token: token, err: safeAttachmentError(err)}
		}
		tempDir := ""
		if workspace != "" {
			if info, statErr := os.Stat(workspace); statErr == nil && info.IsDir() {
				tempDir = workspace
			}
		}
		file, err := os.CreateTemp(tempDir, "hero-clipboard-*.png")
		if err != nil {
			return attachmentMaterializedMsg{token: token, err: safeAttachmentError(err)}
		}
		tempPath := file.Name()
		defer os.Remove(tempPath)
		_ = file.Chmod(0o600)
		if _, err := file.Write(data); err != nil {
			_ = file.Close()
			return attachmentMaterializedMsg{token: token, err: safeAttachmentError(err)}
		}
		if err := file.Close(); err != nil {
			return attachmentMaterializedMsg{token: token, err: safeAttachmentError(err)}
		}
		st, err := media.New(media.StoreOptions{SessionID: sessionID, Workspace: workspace, AllowExternalPaths: allowExternal})
		if err != nil {
			return attachmentMaterializedMsg{token: token, err: safeAttachmentError(err)}
		}
		attachment, err := st.Materialize(context.Background(), media.MaterializeRequest{
			SourcePath:        tempPath,
			OriginalName:      "clipboard.png",
			TurnID:            turnID,
			Origin:            string(harness.AssetSourceUser),
			Workspace:         workspace,
			AllowExternalPath: allowExternal,
		})
		if err != nil {
			return attachmentMaterializedMsg{token: token, err: safeAttachmentError(err)}
		}
		return attachmentMaterializedMsg{token: token, attachment: attachment}
	}
}

func readNativeClipboardPNG(ctx context.Context) ([]byte, error) {
	commands := make([][]string, 0, 3)
	switch runtime.GOOS {
	case "linux":
		commands = append(commands, []string{"wl-paste", "--no-newline", "--type", "image/png"}, []string{"xclip", "-selection", "clipboard", "-t", "image/png", "-o"})
	case "darwin":
		commands = append(commands, []string{"pngpaste", "-"})
	}
	for _, args := range commands {
		if len(args) == 0 {
			continue
		}
		data, err := exec.CommandContext(ctx, args[0], args[1:]...).Output()
		if err == nil && len(data) > 0 {
			return data, nil
		}
	}
	return nil, errors.New("native clipboard does not contain a readable PNG image")
}

// imagePathFromTerminalPaste recognizes the path payload emitted by terminal
// drag-and-drop. There is no portable drag event in a terminal: emulators
// normally send a bracketed paste containing either a filesystem path or a
// local file URI. Some emulators quote or shell-escape paths with spaces.
func imagePathFromTerminalPaste(raw string) (string, bool) {
	path, ok := normalizeTerminalPath(raw)
	if !ok || !isFilesystemImagePath(path) {
		return "", false
	}
	return path, true
}

// promoteUnbracketedTerminalImage recovers the common terminal fallback in
// which a drag-and-drop path is delivered as ordinary key input. A quoted
// path or a file URI is a strong enough boundary to promote when the terminal
// does not provide Paste=true; a bare absolute path is deliberately excluded
// because ordinary typing has no reliable end marker.
func (m model) promoteUnbracketedTerminalImage() (model, tea.Cmd, bool) {
	raw := strings.TrimSpace(m.input)
	if !isUnbracketedTerminalImageCandidate(raw) {
		return m, nil, false
	}
	path, ok := imagePathFromTerminalPaste(raw)
	if !ok {
		return m, nil, false
	}
	m = m.clearChatInput()
	next, cmd := m.queueAttachmentPath(path)
	return next, cmd, true
}

func isUnbracketedTerminalImageCandidate(raw string) bool {
	if len(raw) < 2 {
		return false
	}
	if raw[0] == '\'' || raw[0] == '"' {
		return raw[len(raw)-1] == raw[0]
	}
	return strings.HasPrefix(strings.ToLower(raw), "file:")
}

// normalizeAttachmentPath applies the same harmless terminal-path cleanup to
// explicit /attach paths and paths selected by the picker. If the input is not
// a safely recognizable local path (for example a remote file URI), preserve
// it so the normal media validator can return the user-facing error.
func normalizeAttachmentPath(raw string) string {
	path, ok := normalizeTerminalPath(raw)
	if ok {
		return path
	}
	return strings.TrimSpace(raw)
}

func normalizeTerminalPath(raw string) (string, bool) {
	path := strings.TrimSpace(raw)
	if path == "" || strings.ContainsAny(path, "\r\n") {
		return "", false
	}

	var ok bool
	path, ok = stripTerminalPathQuotes(path)
	if !ok || path == "" {
		return "", false
	}

	if strings.HasPrefix(strings.ToLower(path), "file:") {
		path, ok = localFileURIPath(path)
		if !ok {
			return "", false
		}
	}
	path = unescapeTerminalPath(path)
	if path == "" || strings.ContainsAny(path, "\r\n") {
		return "", false
	}
	return path, true
}

func stripTerminalPathQuotes(path string) (string, bool) {
	for len(path) >= 2 {
		quote := path[0]
		if quote != '\'' && quote != '"' {
			break
		}
		if path[len(path)-1] != quote {
			return "", false
		}
		path = path[1 : len(path)-1]
	}
	return path, true
}

func localFileURIPath(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "file") || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return "", false
	}
	path, err := url.PathUnescape(u.EscapedPath())
	if err != nil || path == "" {
		return "", false
	}
	return path, true
}

func unescapeTerminalPath(path string) string {
	if !strings.ContainsRune(path, '\\') {
		return path
	}

	var b strings.Builder
	runes := []rune(path)
	b.Grow(len(path))
	for i := 0; i < len(runes); i++ {
		if runes[i] != '\\' || i+1 >= len(runes) || !isTerminalEscapedPathRune(runes[i+1]) {
			b.WriteRune(runes[i])
			continue
		}
		i++
		b.WriteRune(runes[i])
	}
	return b.String()
}

func isTerminalEscapedPathRune(r rune) bool {
	if r == '\\' || r == ' ' || r == '\t' {
		return true
	}
	return strings.ContainsRune(`"'()[]{}&;|<>*$?!#`, r)
}

func isFilesystemImagePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.ContainsAny(path, "\r\n") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return filepath.IsAbs(path) || strings.HasPrefix(path, "."+string(filepath.Separator)) || strings.HasPrefix(path, ".."+string(filepath.Separator)) || strings.Contains(path, string(filepath.Separator))
	default:
		return false
	}
}

func isLikelyImagePath(path string) bool {
	_, ok := imagePathFromTerminalPaste(path)
	return ok
}

func (m model) handleAttachmentMaterialized(msg attachmentMaterializedMsg) model {
	for i := range m.attachments {
		if m.attachments[i].token != msg.token {
			continue
		}
		m.attachments[i].pending = false
		m.attachments[i].external = msg.external
		if msg.err != nil {
			m.attachments[i].err = msg.err.Error()
			return m.setStatusWarning("attach", firstStatusLine(msg.err.Error()))
		}
		m.attachments[i].attachment = msg.attachment
		m.attachments[i].name = msg.attachment.Name
		return m
	}
	return m
}

func (m model) readyAttachments() ([]harness.Attachment, error) {
	if len(m.attachments) == 0 {
		return nil, nil
	}
	ready := make([]harness.Attachment, 0, len(m.attachments))
	for _, chip := range m.attachments {
		if chip.pending {
			return nil, errors.New("wait for image validation to finish")
		}
		if strings.TrimSpace(chip.err) != "" {
			return nil, fmt.Errorf("image %q cannot be sent: %s", chip.name, chip.err)
		}
		if strings.TrimSpace(chip.attachment.Path) == "" {
			return nil, fmt.Errorf("image %q is not materialized", chip.name)
		}
		ready = append(ready, chip.attachment)
	}
	return ready, nil
}

func (m model) clearAttachmentChipsForExecute(ex convExecute) model {
	if len(ex.AttachmentChips) == 0 || len(m.attachments) != len(ex.AttachmentChips) {
		return m
	}
	for index, chip := range ex.AttachmentChips {
		if chip.token == "" || chip.token != m.attachments[index].token {
			return m
		}
	}
	return m.clearAttachmentChips()
}

func (m model) clearMediaFocus() model {
	m.assetFocus = false
	m.attachmentFocus = false
	if m.assetSavePending {
		m = m.clearMediaSaveState()
	}
	m.shellFocus = shellFocusContent
	m.chatInputFocused = true
	return m
}

func (m model) focusAssetCards() model {
	if len(m.assets) == 0 {
		if m.assetFocus || m.attachmentFocus {
			return m.clearMediaFocus()
		}
		return m
	}
	m.shellFocus = shellFocusContent
	m.assetFocus = true
	m.attachmentFocus = false
	m.chatInputFocused = false
	if m.assetCursor < 0 {
		m.assetCursor = 0
	}
	if m.assetCursor >= len(m.assets) {
		m.assetCursor = len(m.assets) - 1
	}
	return m
}

func (m model) focusAttachmentChips() model {
	if len(m.attachments) == 0 {
		if m.assetFocus || m.attachmentFocus {
			return m.clearMediaFocus()
		}
		return m
	}
	m.shellFocus = shellFocusContent
	m.assetFocus = false
	m.attachmentFocus = true
	m.chatInputFocused = false
	if m.attachmentCursor < 0 {
		m.attachmentCursor = 0
	}
	if m.attachmentCursor >= len(m.attachments) {
		m.attachmentCursor = len(m.attachments) - 1
	}
	return m
}

func (m model) clearMediaSaveState() model {
	m.assetSavePending = false
	m.assetSaveOverwritePending = false
	m.assetSaveSource = ""
	m.assetSaveInput = ""
	m.assetSaveInputDirty = false
	m.assetSaveTarget = mediaSaveTargetNone
	m.assetSaveAttachmentIndex = -1
	return m
}

func (m model) clearAttachmentChips() model {
	m.attachments = nil
	m.attachmentTurnID = uuid.NewString()
	m.attachmentFocus = false
	if m.assetSaveTarget == mediaSaveTargetAttachment {
		m = m.clearMediaSaveState()
	}
	return m
}

func (m model) removeAttachment(index int) model {
	if index < 0 || index >= len(m.attachments) {
		return m
	}
	if m.assetSaveTarget == mediaSaveTargetAttachment {
		switch {
		case m.assetSaveAttachmentIndex == index:
			m = m.clearMediaSaveState()
		case m.assetSaveAttachmentIndex > index:
			m.assetSaveAttachmentIndex--
		}
	}
	m.attachments = append(m.attachments[:index], m.attachments[index+1:]...)
	if len(m.attachments) == 0 {
		m.attachmentFocus = false
		m.chatInputFocused = true
	}
	if m.attachmentCursor >= len(m.attachments) {
		m.attachmentCursor = len(m.attachments) - 1
	}
	return m
}

func (m model) renderAttachmentChipLines(width int) []string {
	if len(m.attachments) == 0 {
		return nil
	}
	lines := make([]string, 0, len(m.attachments))
	for i, chip := range m.attachments {
		label := "[image] " + chip.name
		if chip.pending {
			label = "⠋ " + label + " · validating"
		} else if chip.err != "" {
			label = "✗ " + label + " · " + chip.err
		} else {
			meta := chip.attachment.MIMEType
			if chip.attachment.Width > 0 && chip.attachment.Height > 0 {
				meta += fmt.Sprintf(" · %dx%d", chip.attachment.Width, chip.attachment.Height)
			}
			if chip.attachment.Size > 0 {
				meta += fmt.Sprintf(" · %dB", chip.attachment.Size)
			}
			if chip.external {
				label = "[image] ⚠ external: " + chip.name
			}
			label += " · " + strings.TrimPrefix(meta, " · ")
		}
		if i == m.attachmentCursor && m.attachmentFocus {
			label = "▸ " + label + " · enter/o open · c copy path · s save · x remove"
		} else {
			label = "• " + label
		}
		if width > 0 {
			label = truncateDisplayWidth(label, width)
		}
		lines = append(lines, label)
		if i == m.assetSaveAttachmentIndex && m.attachmentFocus && m.assetSavePending {
			lines = append(lines, m.renderMediaSaveLines(width)...)
		}
	}
	return lines
}

func (m model) addAsset(asset harness.Asset) model {
	return m.addAssetToTurn(asset, m.agentMsgIndex)
}

func (m model) addAssetToTurn(asset harness.Asset, turnIndex int) model {
	if strings.TrimSpace(asset.ContentHash) == "" {
		asset.ContentHash = asset.Attachment.ContentHash
	}
	if turnIndex < 0 || turnIndex >= len(m.transcript) || m.transcript[turnIndex].role != convRoleAgent {
		for index := len(m.transcript) - 1; index >= 0; index-- {
			if m.transcript[index].role == convRoleAgent {
				turnIndex = index
				break
			}
		}
	}
	if turnIndex < 0 || turnIndex >= len(m.transcript) || m.transcript[turnIndex].role != convRoleAgent {
		m.transcript = append(m.transcript, convMessage{
			role:      convRoleAgent,
			agentName: strings.TrimSpace(m.runtimeAgentName),
			modelSlug: m.conversationModelSlug(),
			harnessID: m.conversationHarnessTool(),
		})
		turnIndex = len(m.transcript) - 1
		m.agentMsgIndex = turnIndex
	}
	msg := m.transcript[turnIndex]
	msg.assets = harness.MergeAssetsByContentHash(msg.assets, []harness.Asset{asset})
	m.transcript[turnIndex] = msg
	m.rebuildAssetIndex()
	if m.assetCursor < 0 {
		m.assetCursor = 0
	}
	m.bumpTranscriptLayout()
	return m
}

func (m *model) rebuildAssetIndex() {
	var assets []harness.Asset
	for _, msg := range m.transcript {
		assets = append(assets, msg.assets...)
	}
	m.assets = assets
}

func (m model) renderAssetCardLines(width int) []string {
	if len(m.assets) == 0 {
		return nil
	}
	return m.renderAssetCardLinesFor(width, m.assets, 0)
}

func (m model) renderAssetCardLinesFor(width int, assets []harness.Asset, offset int) []string {
	if len(assets) == 0 {
		return nil
	}
	lines := []string{"Assets"}
	for i, asset := range assets {
		name := asset.Name
		if name == "" {
			name = filepath.Base(asset.Path)
		}
		source := "[model]"
		if asset.Source == harness.AssetSourceTool {
			source = "[tool wrote]"
		}
		meta := strings.TrimSpace(asset.MIMEType)
		if asset.Width > 0 && asset.Height > 0 {
			meta += fmt.Sprintf(" · %dx%d", asset.Width, asset.Height)
		}
		if asset.Size > 0 {
			meta += fmt.Sprintf(" · %dB", asset.Size)
		}
		line := fmt.Sprintf("%s %s %s", source, name, meta)
		if i+offset == m.assetCursor && m.assetFocus {
			line = "▸ " + line
		}
		lines = append(lines, line)
		lines = append(lines, "  enter/o open · c copy path · a attach · s save")
		if i+offset == m.assetCursor && m.assetFocus && m.assetSavePending && m.assetSaveTarget == mediaSaveTargetAsset {
			lines = append(lines, m.renderMediaSaveLines(width)...)
		}
	}
	return lines
}

func (m model) openSelectedAsset() tea.Cmd {
	if m.assetCursor < 0 || m.assetCursor >= len(m.assets) {
		return mediaActionErrorCmd("asset has no saved path")
	}
	return openMediaPathCmd(m.assets[m.assetCursor].Path)
}

func openMediaPathCmd(path string) tea.Cmd {
	path = strings.TrimSpace(path)
	if path == "" {
		return mediaActionErrorCmd("image has no saved path")
	}
	return tea.ExecProcess(openAssetCommand(path), func(err error) tea.Msg {
		return assetActionMsg{status: "image viewer closed", err: err}
	})
}

func openAssetCommand(path string) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", path)
	}
	return exec.Command("xdg-open", path)
}

func (m model) assetCopyPathCmd() tea.Cmd {
	if m.assetCursor < 0 || m.assetCursor >= len(m.assets) {
		return mediaActionErrorCmd("asset has no saved path")
	}
	return copyMediaPathCmd(m.assets[m.assetCursor].Path)
}

func (m model) selectedAttachment() (tuiAttachment, bool) {
	if m.attachmentCursor < 0 || m.attachmentCursor >= len(m.attachments) {
		return tuiAttachment{}, false
	}
	return m.attachments[m.attachmentCursor], true
}

func (m model) openSelectedAttachment() tea.Cmd {
	chip, ok := m.selectedAttachment()
	if !ok {
		return mediaActionErrorCmd("image attachment is not selected")
	}
	if chip.pending {
		return mediaActionErrorCmd("wait for image validation to finish")
	}
	if strings.TrimSpace(chip.err) != "" {
		return mediaActionErrorCmd("image attachment cannot be opened: " + chip.err)
	}
	return openMediaPathCmd(chip.attachment.Path)
}

func (m model) attachmentCopyPathCmd() tea.Cmd {
	chip, ok := m.selectedAttachment()
	if !ok {
		return mediaActionErrorCmd("image attachment is not selected")
	}
	if chip.pending {
		return mediaActionErrorCmd("wait for image validation to finish")
	}
	if strings.TrimSpace(chip.err) != "" {
		return mediaActionErrorCmd("image attachment cannot be copied: " + chip.err)
	}
	return copyMediaPathCmd(chip.attachment.Path)
}

func copyMediaPathCmd(path string) tea.Cmd {
	path = strings.TrimSpace(path)
	if path == "" {
		return mediaActionErrorCmd("image has no saved path")
	}
	return copyToClipboardCmd(path)
}

func mediaActionErrorCmd(message string) tea.Cmd {
	return func() tea.Msg {
		return assetActionMsg{err: errors.New(message)}
	}
}

func (m model) attachSelectedAsset() model {
	if m.assetCursor < 0 || m.assetCursor >= len(m.assets) || !m.multimodalConversation() {
		return m
	}
	if len(m.attachments) >= media.DefaultMaxAttachmentsPerTurn {
		return m.setStatusWarning("attach", "a turn can contain at most 5 images")
	}
	asset := m.assets[m.assetCursor]
	if strings.TrimSpace(asset.Path) == "" {
		return m.setStatusWarning("attach", "asset has no saved path")
	}
	m.attachments = append(m.attachments, tuiAttachment{token: uuid.NewString(), name: asset.Name, attachment: asset.Attachment})
	return m
}

func defaultMediaSavePath(name, sourcePath string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(strings.TrimSpace(sourcePath))
	}
	name = filepath.Base(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "image"
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join("Downloads", name)
	}
	return filepath.Join(home, "Downloads", name)
}

func defaultAssetSavePath(asset harness.Asset) string {
	return defaultMediaSavePath(asset.Name, asset.Path)
}

func (m model) renderMediaSaveLines(width int) []string {
	if !m.assetSavePending {
		return nil
	}
	input := m.assetSaveInput
	if input == "" {
		input = "<destination path>"
	}
	lines := []string{
		"  save path: " + truncateDisplayWidth(input, maxInt(1, width-14)) + " · enter save · esc cancel",
	}
	if m.assetSaveOverwritePending {
		lines = append(lines, "  destination exists · y overwrite · n choose another · esc cancel")
	}
	return lines
}

func (m model) beginAssetSave() model {
	if m.assetCursor < 0 || m.assetCursor >= len(m.assets) {
		return m
	}
	path := strings.TrimSpace(m.assets[m.assetCursor].Path)
	if path == "" {
		return m.setStatusWarning("save", "asset has no saved path")
	}
	m.assetSavePending = true
	m.assetSaveOverwritePending = false
	m.assetSaveSource = path
	m.assetSaveInput = defaultAssetSavePath(m.assets[m.assetCursor])
	m.assetSaveInputDirty = false
	m.assetSaveTarget = mediaSaveTargetAsset
	m.assetSaveAttachmentIndex = -1
	return m.setStatusResult(false, "save", "confirm the destination path, then press enter")
}

func (m model) beginAttachmentSave() model {
	chip, ok := m.selectedAttachment()
	if !ok {
		return m.setStatusWarning("save", "image attachment is not selected")
	}
	if chip.pending {
		return m.setStatusWarning("save", "wait for image validation to finish")
	}
	if strings.TrimSpace(chip.err) != "" {
		return m.setStatusWarning("save", "image attachment cannot be saved: "+chip.err)
	}
	path := strings.TrimSpace(chip.attachment.Path)
	if path == "" {
		return m.setStatusWarning("save", "image has no saved path")
	}
	m.assetSavePending = true
	m.assetSaveOverwritePending = false
	m.assetSaveSource = path
	m.assetSaveInput = defaultMediaSavePath(chip.name, path)
	m.assetSaveInputDirty = false
	m.assetSaveTarget = mediaSaveTargetAttachment
	m.assetSaveAttachmentIndex = m.attachmentCursor
	return m.setStatusResult(false, "save", "confirm the destination path, then press enter")
}

func (m model) cancelAssetSave() model {
	m = m.clearMediaSaveState()
	return m.setStatusResult(false, "save", "asset save cancelled")
}

func (m model) saveAssetCmd(source, destination string, overwrite bool) tea.Cmd {
	return func() tea.Msg {
		source = strings.TrimSpace(source)
		destination = strings.TrimSpace(destination)
		if source == "" || destination == "" {
			return assetActionMsg{err: errors.New("asset save requires a source and destination path"), clearSave: true}
		}
		in, err := os.Open(source)
		if err != nil {
			return assetActionMsg{err: safeAssetError(err), clearSave: true}
		}
		defer in.Close()
		flags := os.O_WRONLY | os.O_CREATE
		if overwrite {
			flags |= os.O_TRUNC
		} else {
			flags |= os.O_EXCL
		}
		out, err := os.OpenFile(destination, flags, 0o600)
		if err != nil {
			if !overwrite && errors.Is(err, os.ErrExist) {
				return assetActionMsg{overwriteRequired: true}
			}
			return assetActionMsg{err: safeAssetError(err), clearSave: true}
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			_ = os.Remove(destination)
			return assetActionMsg{err: safeAssetError(copyErr), clearSave: true}
		}
		if closeErr != nil {
			_ = os.Remove(destination)
			return assetActionMsg{err: safeAssetError(closeErr), clearSave: true}
		}
		if err := os.Chmod(destination, 0o600); err != nil {
			return assetActionMsg{err: safeAssetError(err), clearSave: true}
		}
		return assetActionMsg{status: "asset saved", clearSave: true}
	}
}

func safeAssetError(err error) error {
	if err == nil {
		return nil
	}
	text := strings.TrimSpace(err.Error())
	if len(text) > 240 {
		text = text[:240] + "..."
	}
	return errors.New(text)
}

func (m model) handleMediaSaveKey(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	if !m.assetSavePending {
		return m, nil, false
	}
	if m.assetSaveOverwritePending {
		switch strings.ToLower(msg.String()) {
		case "y":
			m.assetSaveOverwritePending = false
			return m, m.saveAssetCmd(m.assetSaveSource, m.assetSaveInput, true), true
		case "n":
			m.assetSaveOverwritePending = false
			return m.setStatusWarning("save", "choose another destination, then press enter"), nil, true
		case "esc":
			return m.cancelAssetSave(), nil, true
		default:
			return m, nil, true
		}
	}
	switch msg.String() {
	case "esc":
		return m.cancelAssetSave(), nil, true
	case "enter":
		destination := strings.TrimSpace(m.assetSaveInput)
		if destination == "" {
			return m.setStatusWarning("save", "destination path is required"), nil, true
		}
		return m, m.saveAssetCmd(m.assetSaveSource, destination, false), true
	case "backspace", "delete":
		if !m.assetSaveInputDirty {
			m.assetSaveInput = ""
			m.assetSaveInputDirty = true
		} else {
			runes := []rune(m.assetSaveInput)
			if len(runes) > 0 {
				m.assetSaveInput = string(runes[:len(runes)-1])
			}
		}
		return m, nil, true
	}
	if len(msg.Runes) > 0 && !msg.Alt {
		if m.assetSaveInputDirty {
			m.assetSaveInput += string(msg.Runes)
		} else {
			m.assetSaveInput = string(msg.Runes)
			m.assetSaveInputDirty = true
		}
		return m, nil, true
	}
	return m, nil, true
}

func (m model) handleAssetKey(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	if len(m.assets) == 0 || !m.assetFocus || m.streaming {
		return m, nil, false
	}
	if m.assetSavePending {
		return m.handleMediaSaveKey(msg)
	}
	if key.Matches(msg, assetCardsFocusKey) {
		return m.clearMediaFocus(), nil, true
	}
	switch {
	case key.Matches(msg, navUpKey):
		if m.assetCursor > 0 {
			m.assetCursor--
		}
		return m, nil, true
	case key.Matches(msg, navDownKey):
		if m.assetCursor < len(m.assets)-1 {
			m.assetCursor++
		}
		return m, nil, true
	case key.Matches(msg, mediaOpenKey):
		return m, m.openSelectedAsset(), true
	case key.Matches(msg, mediaCopyPathKey):
		return m, m.assetCopyPathCmd(), true
	case key.Matches(msg, mediaAttachKey):
		return m.attachSelectedAsset(), nil, true
	case key.Matches(msg, mediaSaveKey):
		return m.beginAssetSave(), nil, true
	}
	return m, nil, false
}

func (m model) handleAttachmentKey(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	if len(m.attachments) == 0 || !m.attachmentFocus || m.streaming {
		return m, nil, false
	}
	if m.assetSavePending {
		return m.handleMediaSaveKey(msg)
	}
	if key.Matches(msg, attachmentChipsFocusKey) {
		return m.clearMediaFocus(), nil, true
	}
	switch {
	case key.Matches(msg, navUpKey):
		if m.attachmentCursor > 0 {
			m.attachmentCursor--
		}
		return m, nil, true
	case key.Matches(msg, navDownKey):
		if m.attachmentCursor < len(m.attachments)-1 {
			m.attachmentCursor++
		}
		return m, nil, true
	case key.Matches(msg, mediaOpenKey):
		return m, m.openSelectedAttachment(), true
	case key.Matches(msg, mediaCopyPathKey):
		return m, m.attachmentCopyPathCmd(), true
	case key.Matches(msg, mediaSaveKey):
		return m.beginAttachmentSave(), nil, true
	case key.Matches(msg, mediaRemoveKey):
		return m.removeAttachment(m.attachmentCursor), nil, true
	}
	return m, nil, false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
