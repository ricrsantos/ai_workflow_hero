package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/muesli/termenv"
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

func (m model) multimodalFreeChat() bool {
	return m.freeChatMode && m.screen == screenConversation && !m.researchLive && !m.workflowAgentActive()
}

// attachmentPermissionProfile reads the persisted policy from the Hero
// configuration root. Free Chat intentionally executes in the current working
// directory, so using executeDir here would skip ~/.workflow-hero/config and
// silently fall back to Ask.
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
	if !m.multimodalFreeChat() {
		return m.setStatusResult(false, "attach", "image attachments are available in Free Chat only"), nil
	}
	picker := filepicker.New()
	picker.AllowedTypes = []string{".png", ".jpg", ".jpeg", ".gif", ".webp"}
	picker.CurrentDirectory = m.executeDir()
	if strings.TrimSpace(picker.CurrentDirectory) == "" {
		picker.CurrentDirectory = "."
	}
	picker.DirAllowed = false
	picker.FileAllowed = true
	picker.ShowHidden = false
	picker.SetHeight(maxInt(8, m.frameContentHeight()-4))
	m.attachmentPicker = picker
	m.attachmentPickerActive = true
	return m, picker.Init()
}

func (m model) handleAttachmentPickerKey(msg tea.KeyMsg) (model, tea.Cmd) {
	if !m.attachmentPickerActive {
		return m, nil
	}
	if msg.String() == "esc" {
		m.attachmentPickerActive = false
		m.chatInputFocused = true
		return m, nil
	}
	picker, cmd := m.attachmentPicker.Update(msg)
	m.attachmentPicker = picker
	if selected, path := picker.DidSelectFile(msg); selected {
		m.attachmentPickerActive = false
		return m.queueAttachmentPath(path)
	}
	return m, cmd
}

func (m model) queueAttachmentPath(path string) (model, tea.Cmd) {
	if !m.multimodalFreeChat() {
		return m.setStatusResult(false, "attach", "image attachments are available in Free Chat only"), nil
	}
	if len(m.attachments) >= media.DefaultMaxAttachmentsPerTurn {
		return m.setStatusResult(false, "attach", "a turn can contain at most 5 images"), nil
	}
	path = strings.TrimSpace(path)
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
	if !m.multimodalFreeChat() {
		return m.setStatusResult(false, "attach", "image attachments are available in Free Chat only"), nil
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

func isLikelyImagePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.ContainsAny(path, "\r\n") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return strings.HasPrefix(path, "/") || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || strings.Contains(path, string(filepath.Separator))
	default:
		return false
	}
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

func (m model) clearAttachmentChips() model {
	m.attachments = nil
	m.attachmentTurnID = uuid.NewString()
	m.attachmentFocus = false
	return m
}

func (m model) removeAttachment(index int) model {
	if index < 0 || index >= len(m.attachments) {
		return m
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
			label = "▸ " + label + " · x dismiss"
		} else {
			label = "• " + label
		}
		if width > 0 && len([]rune(label)) > width {
			label = truncateDisplayWidth(label, width)
		}
		lines = append(lines, label)
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
	if m.assetMosaics == nil {
		m.assetMosaics = make(map[string]media.MosaicResult)
	}
	if m.assetMosaicPending == nil {
		m.assetMosaicPending = make(map[string]bool)
	}
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

func assetKey(asset harness.Asset) string {
	if hash := strings.TrimSpace(asset.ContentHash); hash != "" {
		return "hash:" + hash
	}
	if asset.Path != "" {
		return "path:" + asset.Path
	}
	return "id:" + asset.ID
}

func (m model) toggleAssetMosaic() (model, tea.Cmd) {
	if m.assetCursor < 0 || m.assetCursor >= len(m.assets) {
		return m, nil
	}
	asset := m.assets[m.assetCursor]
	key := assetKey(asset)
	if m.assetMosaics == nil {
		m.assetMosaics = make(map[string]media.MosaicResult)
	}
	if m.assetMosaicPending == nil {
		m.assetMosaicPending = make(map[string]bool)
	}
	if _, expanded := m.assetMosaics[key]; expanded {
		delete(m.assetMosaics, key)
		return m, nil
	}
	if m.assetMosaicPending[key] {
		return m, nil
	}
	m.assetMosaicPending[key] = true
	return m, tea.Batch(media.MosaicCmd(asset.Path, m.assetMosaicConfig()), convWaitTickCmd())
}

func (m model) assetMosaicConfig() media.MosaicConfig {
	return media.MosaicConfig{
		Width:      minInt(64, maxInt(1, m.transcriptTextWidth()/2)),
		Height:     minInt(12, maxInt(1, m.frameContentHeight()/3)),
		ColorDepth: tuiMosaicColorDepth(),
	}
}

func (m model) hasPendingMosaic() bool {
	for _, pending := range m.assetMosaicPending {
		if pending {
			return true
		}
	}
	return false
}

// refreshExpandedMosaics schedules a fresh decode/render for every expanded
// card after a terminal resize. The command owns all filesystem/image work;
// Update only marks the cards pending so keys and View remain responsive.
func (m model) refreshExpandedMosaics() (model, tea.Cmd) {
	if len(m.assetMosaics) == 0 || len(m.assets) == 0 {
		return m, nil
	}
	if m.assetMosaicPending == nil {
		m.assetMosaicPending = make(map[string]bool)
	}
	config := m.assetMosaicConfig()
	cmds := make([]tea.Cmd, 0, len(m.assetMosaics))
	for _, asset := range m.assets {
		key := assetKey(asset)
		if _, expanded := m.assetMosaics[key]; !expanded || strings.TrimSpace(asset.Path) == "" {
			continue
		}
		m.assetMosaicPending[key] = true
		cmds = append(cmds, media.MosaicCmd(asset.Path, config))
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m model) handleMosaicMsg(msg media.MosaicMsg) model {
	if m.assetMosaicPending != nil {
		delete(m.assetMosaicPending, assetKeyByPath(m.assets, msg.Path))
	}
	if msg.Err != nil {
		m.bumpTranscriptLayout()
		return m.setStatusWarning("preview", firstStatusLine(msg.Err.Error()))
	}
	if m.assetMosaics == nil {
		m.assetMosaics = make(map[string]media.MosaicResult)
	}
	for _, asset := range m.assets {
		if asset.Path == msg.Path {
			m.assetMosaics[assetKey(asset)] = msg.Result
			break
		}
	}
	m.bumpTranscriptLayout()
	return m
}

func assetKeyByPath(assets []harness.Asset, path string) string {
	for _, asset := range assets {
		if asset.Path == path {
			return assetKey(asset)
		}
	}
	return "path:" + path
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
		lines = append(lines, "  Enter preview · o open · c copy path · a attach · s save")
		key := assetKey(asset)
		if m.assetMosaicPending[key] {
			_, expanded := m.assetMosaics[key]
			selected := i+offset == m.assetCursor && m.assetFocus
			if selected || expanded {
				frame := waitAnimFrames[m.waitAnimFrame%len(waitAnimFrames)]
				lines = append(lines, "  "+frame+" rendering mosaic…")
			}
		}
		if i+offset == m.assetCursor && m.assetFocus && m.assetSavePending {
			input := m.assetSaveInput
			if input == "" {
				input = "<destination path>"
			}
			lines = append(lines, "  save path: "+truncateDisplayWidth(input, maxInt(1, width-14))+" · enter save · esc cancel")
			if m.assetSaveOverwritePending {
				lines = append(lines, "  destination exists · y overwrite · n choose another · esc cancel")
			}
		}
		if preview, ok := m.assetMosaics[key]; ok {
			if preview.Available {
				for _, previewLine := range strings.Split(preview.Text, "\n") {
					lines = append(lines, "  "+truncateDisplayWidth(previewLine, maxInt(1, width-2)))
				}
			} else if preview.Message != "" {
				lines = append(lines, "  "+preview.Message)
			}
		}
	}
	return lines
}

func (m model) openSelectedAsset() tea.Cmd {
	if m.assetCursor < 0 || m.assetCursor >= len(m.assets) || strings.TrimSpace(m.assets[m.assetCursor].Path) == "" {
		return func() tea.Msg { return assetActionMsg{err: errors.New("asset has no saved path")} }
	}
	path := m.assets[m.assetCursor].Path
	return tea.ExecProcess(openAssetCommand(path), func(err error) tea.Msg {
		return assetActionMsg{status: "asset viewer closed", err: err}
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
		return nil
	}
	return copyToClipboardCmd(m.assets[m.assetCursor].Path)
}

func (m model) attachSelectedAsset() model {
	if m.assetCursor < 0 || m.assetCursor >= len(m.assets) || !m.multimodalFreeChat() {
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

func tuiMosaicColorDepth() media.ColorDepth {
	switch lipgloss.ColorProfile() {
	case termenv.TrueColor, termenv.ANSI256:
		return media.ColorDepth256
	case termenv.ANSI:
		return media.ColorDepthANSI
	default:
		return media.ColorDepthUnknown
	}
}

func defaultAssetSavePath(asset harness.Asset) string {
	name := strings.TrimSpace(asset.Name)
	if name == "" {
		name = filepath.Base(strings.TrimSpace(asset.Path))
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
	return m.setStatusResult(false, "save", "confirm the destination path, then press enter")
}

func (m model) cancelAssetSave() model {
	m.assetSavePending = false
	m.assetSaveOverwritePending = false
	m.assetSaveSource = ""
	m.assetSaveInput = ""
	m.assetSaveInputDirty = false
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

func (m model) handleAssetKey(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	if len(m.assets) == 0 || !m.assetFocus || m.streaming {
		return m, nil, false
	}
	if m.assetSavePending {
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
	switch msg.String() {
	case "up":
		if m.assetCursor > 0 {
			m.assetCursor--
		}
		return m, nil, true
	case "down":
		if m.assetCursor < len(m.assets)-1 {
			m.assetCursor++
		}
		return m, nil, true
	case "enter":
		next, cmd := m.toggleAssetMosaic()
		return next, cmd, true
	case "o":
		return m, m.openSelectedAsset(), true
	case "c":
		return m, m.assetCopyPathCmd(), true
	case "a":
		return m.attachSelectedAsset(), nil, true
	case "s":
		return m.beginAssetSave(), nil, true
	}
	return m, nil, false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
