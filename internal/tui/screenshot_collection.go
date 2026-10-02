package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/envhygiene"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/screenshots"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

const (
	screenshotSelectorLatest = "latest"
	screenshotSelectorList   = "list"
	screenshotSelectorTodos  = "todos"
	screenshotCurrentDir     = ".workflow-hero/cycles/current"
	screenshotOpenDelay      = 150 * time.Millisecond
)

var screenshotCollectionKey = key.NewBinding(
	key.WithKeys("alt+b"),
	key.WithHelp("Alt+B", "Screenshots"),
)

type screenshotCollectionState struct {
	open             bool
	loading          bool
	generation       uint64
	selector         string
	requestedID      string
	cycleID          int64
	assets           []store.ScreenshotManifest
	selected         int
	offset           int
	message          string
	errorMessage     bool
	batchRunning     bool
	batchGeneration  uint64
	batchIndex       int
	batchFailures    []string
	savePending      bool
	saveOverwrite    bool
	saveDestination  string
	saveInputDirty   bool
	saveScreenshotID string
	autoWatchActive  bool
	autoWatchGen     uint64
	autoWatchStage   string
	autoWatchAttempt int
	autoWatchCycleID int64
	autoWatchSeen    []string
	autoWatchWarning bool
}

type screenshotSnapshotMsg struct {
	generation uint64
	cycleID    int64
	assets     []store.ScreenshotManifest
	noCycle    bool
	err        bool
}

type screenshotActionMsg struct {
	generation      uint64
	screenshotID    string
	action          string
	err             bool
	overwriteNeeded bool
}

type screenshotBatchNextMsg struct{ generation uint64 }

type screenshotViewerResultMsg struct {
	generation   uint64
	screenshotID string
	err          bool
	batch        bool
}

func parseScreenshotSlash(input string) (selector, screenshotID string, ok bool) {
	fields := strings.Fields(strings.TrimSpace(input))
	if len(fields) == 0 || fields[0] != "/hero-screenshot" {
		return "", "", false
	}
	if len(fields) == 1 {
		return screenshotSelectorLatest, "", true
	}
	if len(fields) > 2 {
		return "invalid", "", true
	}
	argument := fields[1]
	switch argument {
	case screenshotSelectorLatest, screenshotSelectorList, screenshotSelectorTodos:
		return argument, "", true
	default:
		return "id", argument, true
	}
}

func (m model) openScreenshotCollection(selector, screenshotID string) (model, tea.Cmd) {
	if selector == "" {
		selector = screenshotSelectorLatest
	}
	state := m.screenshots
	state.generation++
	state.open = true
	state.selector = selector
	state.requestedID = screenshotID
	state.cycleID = 0
	state.assets = nil
	state.selected = 0
	state.offset = 0
	state.loading = false
	state.message = ""
	state.errorMessage = false
	state.batchRunning = false
	state.batchGeneration++
	state.batchIndex = 0
	state.batchFailures = nil
	state.savePending = false
	state.saveOverwrite = false
	state.saveDestination = ""
	state.saveInputDirty = false
	state.saveScreenshotID = ""
	m.screenshots = state

	if selector != screenshotSelectorLatest && selector != screenshotSelectorList && selector != screenshotSelectorTodos && selector != "id" {
		m.screenshots.message = "Usage: /hero-screenshot [latest|list|<id>|todos]"
		m.screenshots.errorMessage = true
		return m, nil
	}
	if m.svc == nil || m.svc.Store == nil {
		m.screenshots.message = "No active cycle is available for screenshots."
		m.screenshots.errorMessage = true
		return m, nil
	}
	m.screenshots.loading = true
	return m, loadScreenshotSnapshotCmd(m.svc.ProjectDir, m.svc.Store, state.generation)
}

func loadScreenshotSnapshotCmd(projectDir string, st *store.Store, generation uint64) tea.Cmd {
	return func() tea.Msg {
		if st == nil || strings.TrimSpace(projectDir) == "" {
			return screenshotSnapshotMsg{generation: generation, noCycle: true}
		}
		active, err := st.GetActiveCycle()
		if errors.Is(err, store.ErrNoActiveCycle) {
			return screenshotSnapshotMsg{generation: generation, noCycle: true}
		}
		if err != nil {
			return screenshotSnapshotMsg{generation: generation, err: true}
		}
		service, err := screenshots.NewService(projectDir, st)
		if err != nil {
			return screenshotSnapshotMsg{generation: generation, err: true}
		}
		assets, err := service.ReadySet(context.Background(), active.ID)
		if err != nil {
			return screenshotSnapshotMsg{generation: generation, err: true}
		}
		return screenshotSnapshotMsg{generation: generation, cycleID: active.ID, assets: assets}
	}
}

func (m model) handleScreenshotSnapshot(msg screenshotSnapshotMsg) model {
	if !m.screenshots.open || msg.generation != m.screenshots.generation {
		return m
	}
	m.screenshots.loading = false
	m.screenshots.cycleID = msg.cycleID
	if msg.noCycle {
		m.screenshots.message = "No active cycle is available for screenshots."
		m.screenshots.errorMessage = true
		return m
	}
	if msg.err {
		m.screenshots.message = "Ready screenshots could not be validated safely."
		m.screenshots.errorMessage = true
		slog.Error("tui screenshot snapshot validation failed")
		return m
	}
	m.screenshots.assets = append([]store.ScreenshotManifest(nil), msg.assets...)
	if len(msg.assets) == 0 {
		m.screenshots.message = "No screenshot has been captured yet."
		m.screenshots.errorMessage = m.screenshots.selector == screenshotSelectorLatest || m.screenshots.selector == "id"
		return m
	}
	switch m.screenshots.selector {
	case screenshotSelectorLatest:
		m.screenshots.selected = len(msg.assets) - 1
	case "id":
		m.screenshots.selected = -1
		for i := range msg.assets {
			if msg.assets[i].ID == m.screenshots.requestedID {
				m.screenshots.selected = i
				break
			}
		}
		if m.screenshots.selected < 0 {
			m.screenshots.message = "Screenshot ID was not found."
			m.screenshots.errorMessage = true
			return m
		}
	case screenshotSelectorTodos:
		m.screenshots.message = fmt.Sprintf("Snapshot loaded: %d ready screenshots. Use Open all for local viewers.", len(msg.assets))
	}
	m = m.ensureScreenshotSelectionVisible()
	slog.Info("tui screenshot snapshot loaded", "capture_count", len(msg.assets), "cycle_id", msg.cycleID)
	return m
}

func (m model) ensureScreenshotSelectionVisible() model {
	if len(m.screenshots.assets) == 0 {
		m.screenshots.selected = 0
		m.screenshots.offset = 0
		return m
	}
	if m.screenshots.selected < 0 {
		m.screenshots.selected = 0
	}
	if m.screenshots.selected >= len(m.screenshots.assets) {
		m.screenshots.selected = len(m.screenshots.assets) - 1
	}
	visible := m.screenshotVisibleCards()
	if m.screenshots.selected < m.screenshots.offset {
		m.screenshots.offset = m.screenshots.selected
	}
	if m.screenshots.selected >= m.screenshots.offset+visible {
		m.screenshots.offset = m.screenshots.selected - visible + 1
	}
	maxOffset := max(0, len(m.screenshots.assets)-visible)
	m.screenshots.offset = min(max(0, m.screenshots.offset), maxOffset)
	return m
}

func (m model) screenshotVisibleCards() int {
	return max(1, (m.frameContentHeight()-8)/3)
}

func (m model) handleScreenshotCollectionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.screenshots.savePending {
		return m.handleScreenshotSaveKey(msg)
	}
	switch strings.ToLower(msg.String()) {
	case "esc", "alt+b":
		m.screenshots.open = false
		m.screenshots.batchRunning = false
		m.screenshots.batchGeneration++
		return m, nil
	case "ctrl+c":
		if m.screen == screenConversation {
			return m.handleConversationKey(msg)
		}
		return m, nil
	case "alt+q":
		return m.handleKey(msg)
	case "up", "k":
		if m.screenshots.selected > 0 {
			m.screenshots.selected--
		}
		return m.ensureScreenshotSelectionVisible(), nil
	case "down", "j":
		if m.screenshots.selected < len(m.screenshots.assets)-1 {
			m.screenshots.selected++
		}
		return m.ensureScreenshotSelectionVisible(), nil
	case "pgup":
		m.screenshots.selected -= m.screenshotVisibleCards()
		if m.screenshots.selected < 0 {
			m.screenshots.selected = 0
		}
		return m.ensureScreenshotSelectionVisible(), nil
	case "pgdown":
		m.screenshots.selected += m.screenshotVisibleCards()
		if m.screenshots.selected >= len(m.screenshots.assets) {
			m.screenshots.selected = len(m.screenshots.assets) - 1
		}
		return m.ensureScreenshotSelectionVisible(), nil
	case "home":
		m.screenshots.selected = 0
		return m.ensureScreenshotSelectionVisible(), nil
	case "end":
		m.screenshots.selected = len(m.screenshots.assets) - 1
		return m.ensureScreenshotSelectionVisible(), nil
	case "enter", "o":
		return m.openSelectedScreenshot()
	case "c":
		return m.copySelectedScreenshotPath()
	case "s":
		return m.beginScreenshotSave()
	case "a":
		return m.beginOpenAllScreenshots()
	default:
		return m, nil
	}
}

func (m model) openSelectedScreenshot() (model, tea.Cmd) {
	manifest, ok := m.selectedScreenshot()
	if !ok {
		return m.setScreenshotMessage("No screenshot is selected.", true), nil
	}
	projectDir, st := m.screenshotProjectStore()
	return m, screenshotViewerCmd(projectDir, st, m.screenshots.cycleID, manifest.ID, m.screenshots.generation, false)
}

func (m model) copySelectedScreenshotPath() (model, tea.Cmd) {
	manifest, ok := m.selectedScreenshot()
	if !ok {
		return m.setScreenshotMessage("No screenshot is selected.", true), nil
	}
	projectDir, st := m.screenshotProjectStore()
	return m, screenshotActionCmd(projectDir, st, m.screenshots.cycleID, manifest.ID, m.screenshots.generation, "copy", "", false)
}

func (m model) selectedScreenshot() (store.ScreenshotManifest, bool) {
	if m.screenshots.selected < 0 || m.screenshots.selected >= len(m.screenshots.assets) {
		return store.ScreenshotManifest{}, false
	}
	return m.screenshots.assets[m.screenshots.selected], true
}

func (m model) screenshotProjectStore() (string, *store.Store) {
	if m.svc == nil {
		return "", nil
	}
	return m.svc.ProjectDir, m.svc.Store
}

func (m model) beginScreenshotSave() (model, tea.Cmd) {
	manifest, ok := m.selectedScreenshot()
	if !ok {
		return m.setScreenshotMessage("No screenshot is selected.", true), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return m.setScreenshotMessage("Save destination is unavailable; enter a path manually.", true), nil
	}
	m.screenshots.savePending = true
	m.screenshots.saveOverwrite = false
	m.screenshots.saveScreenshotID = manifest.ID
	m.screenshots.saveDestination = filepath.Join(home, "Downloads", filepath.Base(manifest.Path))
	m.screenshots.saveInputDirty = false
	m.screenshots.message = "Enter a destination path, then press Enter to save."
	m.screenshots.errorMessage = false
	return m, nil
}

func (m model) handleScreenshotSaveKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.screenshots.saveOverwrite {
		switch strings.ToLower(msg.String()) {
		case "y":
			m.screenshots.saveOverwrite = false
			projectDir, st := m.screenshotProjectStore()
			return m, screenshotActionCmd(projectDir, st, m.screenshots.cycleID, m.screenshots.saveScreenshotID, m.screenshots.generation, "save", m.screenshots.saveDestination, true)
		case "n":
			m.screenshots.saveOverwrite = false
			m.screenshots.message = "Choose another destination, then press Enter."
			return m, nil
		case "esc":
			return m.cancelScreenshotSave(), nil
		default:
			return m, nil
		}
	}
	switch msg.String() {
	case "esc":
		return m.cancelScreenshotSave(), nil
	case "enter":
		if strings.TrimSpace(m.screenshots.saveDestination) == "" {
			m.screenshots.message = "Save destination is required."
			m.screenshots.errorMessage = true
			return m, nil
		}
		projectDir, st := m.screenshotProjectStore()
		return m, screenshotActionCmd(projectDir, st, m.screenshots.cycleID, m.screenshots.saveScreenshotID, m.screenshots.generation, "save", m.screenshots.saveDestination, false)
	case "backspace", "delete":
		if !m.screenshots.saveInputDirty {
			m.screenshots.saveDestination = ""
			m.screenshots.saveInputDirty = true
		} else {
			runes := []rune(m.screenshots.saveDestination)
			if len(runes) > 0 {
				m.screenshots.saveDestination = string(runes[:len(runes)-1])
			}
		}
		return m, nil
	default:
		if len(msg.Runes) > 0 && !msg.Alt {
			if m.screenshots.saveInputDirty {
				m.screenshots.saveDestination += string(msg.Runes)
			} else {
				m.screenshots.saveDestination = string(msg.Runes)
				m.screenshots.saveInputDirty = true
			}
		}
		return m, nil
	}
}

func (m model) cancelScreenshotSave() model {
	m.screenshots.savePending = false
	m.screenshots.saveOverwrite = false
	m.screenshots.saveDestination = ""
	m.screenshots.saveInputDirty = false
	m.screenshots.saveScreenshotID = ""
	m.screenshots.message = "Save cancelled."
	m.screenshots.errorMessage = false
	return m
}

func (m model) handleScreenshotAction(msg screenshotActionMsg) model {
	if !m.screenshots.open || msg.generation != m.screenshots.generation {
		return m
	}
	if msg.action == "save" && msg.overwriteNeeded {
		m.screenshots.saveOverwrite = true
		m.screenshots.message = "Destination exists; press Y to overwrite or N to choose another."
		m.screenshots.errorMessage = false
		return m
	}
	if msg.action == "save" && !msg.err {
		m.screenshots.savePending = false
		m.screenshots.saveOverwrite = false
		m.screenshots.saveDestination = ""
		m.screenshots.saveInputDirty = false
		m.screenshots.saveScreenshotID = ""
	}
	if msg.err {
		m.screenshots.message = "Screenshot action failed for ID " + safeScreenshotID(msg.screenshotID) + ". The ready image may have changed; reload the collection and retry."
		m.screenshots.errorMessage = true
		slog.Error("tui screenshot action failed", "action", msg.action, "screenshot_id", safeScreenshotID(msg.screenshotID))
		return m
	}
	var message string
	switch msg.action {
	case "copy":
		message = "Copied the validated screenshot path."
	case "save":
		message = "Screenshot saved."
	default:
		return m
	}
	m.screenshots.message = message
	m.screenshots.errorMessage = false
	slog.Info("tui screenshot action completed", "action", msg.action, "screenshot_id", safeScreenshotID(msg.screenshotID))
	return m
}

func (m model) setScreenshotMessage(message string, isError bool) model {
	m.screenshots.message = message
	m.screenshots.errorMessage = isError
	return m
}

func (m model) beginOpenAllScreenshots() (model, tea.Cmd) {
	if len(m.screenshots.assets) == 0 {
		return m.setScreenshotMessage("No screenshot has been captured yet.", true), nil
	}
	m.screenshots.batchRunning = true
	m.screenshots.batchGeneration++
	m.screenshots.batchIndex = 0
	m.screenshots.batchFailures = nil
	m.screenshots.message = fmt.Sprintf("Opening 0/%d screenshots…", len(m.screenshots.assets))
	m.screenshots.errorMessage = false
	slog.Info("tui screenshot open-all started", "capture_count", len(m.screenshots.assets))
	return m, m.launchNextScreenshotCmd()
}

func (m model) launchNextScreenshotCmd() tea.Cmd {
	if !m.screenshots.batchRunning || m.screenshots.batchIndex >= len(m.screenshots.assets) {
		return nil
	}
	manifest := m.screenshots.assets[m.screenshots.batchIndex]
	projectDir, st := m.screenshotProjectStore()
	return screenshotViewerCmd(projectDir, st, m.screenshots.cycleID, manifest.ID, m.screenshots.batchGeneration, true)
}

func (m model) handleScreenshotViewerResult(msg screenshotViewerResultMsg) (tea.Model, tea.Cmd) {
	if !m.screenshots.open {
		return m, nil
	}
	if msg.batch {
		if !m.screenshots.batchRunning || msg.generation != m.screenshots.batchGeneration {
			return m, nil
		}
		if msg.err {
			m.screenshots.batchFailures = append(m.screenshots.batchFailures, safeScreenshotID(msg.screenshotID))
		}
		m.screenshots.batchIndex++
		if m.screenshots.batchIndex >= len(m.screenshots.assets) {
			m.screenshots.batchRunning = false
			m.screenshots.message = screenshotBatchSummary(m.screenshots.batchIndex, m.screenshots.batchFailures)
			m.screenshots.errorMessage = len(m.screenshots.batchFailures) > 0
			if len(m.screenshots.batchFailures) > 0 {
				slog.Error("tui screenshot open-all completed with failures", "failure_count", len(m.screenshots.batchFailures))
			} else {
				slog.Info("tui screenshot open-all completed", "capture_count", m.screenshots.batchIndex)
			}
			return m, nil
		}
		m.screenshots.message = fmt.Sprintf("Opening %d/%d screenshots…", m.screenshots.batchIndex, len(m.screenshots.assets))
		return m, tea.Tick(screenshotOpenDelay, func(time.Time) tea.Msg {
			return screenshotBatchNextMsg{generation: msg.generation}
		})
	}
	if msg.generation != m.screenshots.generation {
		return m, nil
	}
	if msg.err {
		m.screenshots.message = "Could not open screenshot ID " + safeScreenshotID(msg.screenshotID) + "."
		m.screenshots.errorMessage = true
		slog.Error("tui screenshot viewer launch failed", "screenshot_id", safeScreenshotID(msg.screenshotID))
	} else {
		m.screenshots.message = "Opened screenshot ID " + safeScreenshotID(msg.screenshotID) + "."
		m.screenshots.errorMessage = false
		slog.Info("tui screenshot viewer launched", "screenshot_id", safeScreenshotID(msg.screenshotID))
	}
	return m, nil
}

func screenshotBatchSummary(total int, failures []string) string {
	if len(failures) == 0 {
		return fmt.Sprintf("Opened all %d screenshots.", total)
	}
	return fmt.Sprintf("Opened %d/%d; failed IDs: %s", total-len(failures), total, strings.Join(failures, ", "))
}

func safeScreenshotID(id string) string {
	if id == "" || len(id) > 96 {
		return "unknown"
	}
	for _, r := range id {
		isAlphaNumeric := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		if !isAlphaNumeric && r != '_' && r != '-' {
			return "unknown"
		}
	}
	return id
}

func screenshotReadyPath(projectDir string, st *store.Store, cycleID int64, screenshotID string) (string, error) {
	service, err := screenshots.NewService(projectDir, st)
	if err != nil {
		return "", errors.New("screenshot service unavailable")
	}
	ready, err := service.ReadySet(context.Background(), cycleID)
	if err != nil {
		return "", errors.New("ready screenshot could not be validated")
	}
	for _, manifest := range ready {
		if manifest.ID != screenshotID {
			continue
		}
		path := filepath.Join(projectDir, screenshotCurrentDir, filepath.FromSlash(manifest.Path))
		relative, err := filepath.Rel(projectDir, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return "", errors.New("ready screenshot path is unsafe")
		}
		return path, nil
	}
	return "", errors.New("screenshot ID is no longer ready")
}

func screenshotViewerCmd(projectDir string, st *store.Store, cycleID int64, screenshotID string, generation uint64, batch bool) tea.Cmd {
	return func() tea.Msg {
		if st == nil || projectDir == "" {
			return screenshotViewerResultMsg{generation: generation, screenshotID: screenshotID, err: true, batch: batch}
		}
		path, err := screenshotReadyPath(projectDir, st, cycleID, screenshotID)
		if err != nil {
			return screenshotViewerResultMsg{generation: generation, screenshotID: screenshotID, err: true, batch: batch}
		}
		command := openAssetCommand(path)
		if err := command.Start(); err != nil {
			return screenshotViewerResultMsg{generation: generation, screenshotID: screenshotID, err: true, batch: batch}
		}
		if err := command.Process.Release(); err != nil {
			return screenshotViewerResultMsg{generation: generation, screenshotID: screenshotID, err: true, batch: batch}
		}
		return screenshotViewerResultMsg{generation: generation, screenshotID: screenshotID, batch: batch}
	}
}

func screenshotActionCmd(projectDir string, st *store.Store, cycleID int64, screenshotID string, generation uint64, action, destination string, overwrite bool) tea.Cmd {
	return func() tea.Msg {
		if st == nil || projectDir == "" {
			return screenshotActionMsg{generation: generation, screenshotID: screenshotID, action: action, err: true}
		}
		path, err := screenshotReadyPath(projectDir, st, cycleID, screenshotID)
		if err != nil {
			return screenshotActionMsg{generation: generation, screenshotID: screenshotID, action: action, err: true}
		}
		switch action {
		case "copy":
			if cmd := copyToClipboardCmd(path); cmd != nil {
				_ = cmd()
			}
			return screenshotActionMsg{generation: generation, screenshotID: screenshotID, action: action}
		case "save":
			needed, saveErr := saveScreenshotCopy(path, destination, overwrite, projectDir)
			return screenshotActionMsg{generation: generation, screenshotID: screenshotID, action: action, err: saveErr != nil, overwriteNeeded: needed}
		default:
			return screenshotActionMsg{generation: generation, screenshotID: screenshotID, action: action, err: true}
		}
	}
}

func saveScreenshotCopy(source, destination string, overwrite bool, projectDir string) (overwriteNeeded bool, resultErr error) {
	destination = strings.TrimSpace(destination)
	if source == "" || destination == "" {
		return false, errors.New("source and destination are required")
	}
	if screenshotCredentialDestination(projectDir, destination) {
		return false, errors.New("refusing to save a screenshot over a credential file")
	}
	sourceInfo, err := os.Lstat(source)
	if err != nil || !sourceInfo.Mode().IsRegular() || sourceInfo.Mode()&os.ModeSymlink != 0 {
		return false, errors.New("screenshot source is unsafe")
	}
	if info, err := os.Lstat(destination); err == nil {
		if os.SameFile(sourceInfo, info) {
			return false, errors.New("refusing to overwrite the managed screenshot source")
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return false, errors.New("destination is unsafe")
		}
		if !overwrite {
			return true, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, errors.New("destination cannot be checked")
	}
	in, err := os.Open(source)
	if err != nil {
		return false, errors.New("screenshot source could not be opened")
	}
	defer func() {
		if closeErr := in.Close(); resultErr == nil && closeErr != nil {
			resultErr = errors.New("screenshot source could not be closed")
		}
	}()
	dir := filepath.Dir(destination)
	tmp, err := os.CreateTemp(dir, ".hero-screenshot-*.tmp")
	if err != nil {
		return false, errors.New("save destination directory is unavailable")
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return false, errors.New("temporary screenshot file could not be closed")
		}
		return false, errors.New("could not secure saved screenshot")
	}
	const maxSavedImageBytes = 32 << 20
	written, err := io.Copy(tmp, io.LimitReader(in, maxSavedImageBytes+1))
	if err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return false, errors.New("temporary screenshot file could not be closed")
		}
		return false, errors.New("screenshot copy failed")
	}
	if written > maxSavedImageBytes {
		if closeErr := tmp.Close(); closeErr != nil {
			return false, errors.New("temporary screenshot file could not be closed")
		}
		return false, errors.New("screenshot exceeds the save size limit")
	}
	if err := tmp.Sync(); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return false, errors.New("temporary screenshot file could not be closed")
		}
		return false, errors.New("screenshot copy could not be synchronized")
	}
	if err := tmp.Close(); err != nil {
		return false, errors.New("screenshot copy could not be closed")
	}
	if overwrite {
		if err := os.Rename(tmpPath, destination); err != nil {
			return false, errors.New("screenshot could not be saved")
		}
		return false, nil
	}
	if err := os.Link(tmpPath, destination); err != nil {
		if errors.Is(err, os.ErrExist) {
			return true, nil
		}
		return false, errors.New("screenshot could not be saved")
	}
	return false, nil
}

func screenshotCredentialDestination(projectDir, destination string) bool {
	destination = filepath.Clean(destination)
	if envhygiene.IsSensitivePath(filepath.Base(destination)) || isPrivateKeyFilename(destination) {
		return true
	}
	if strings.TrimSpace(projectDir) == "" {
		return false
	}
	projectAbs, err := filepath.Abs(projectDir)
	if err != nil {
		return false
	}
	destinationAbs, err := filepath.Abs(destination)
	if err != nil {
		return false
	}
	if realProject, err := filepath.EvalSymlinks(projectAbs); err == nil {
		projectAbs = realProject
	}
	if realParent, err := filepath.EvalSymlinks(filepath.Dir(destinationAbs)); err == nil {
		destinationAbs = filepath.Join(realParent, filepath.Base(destinationAbs))
	}
	relative, err := filepath.Rel(projectAbs, destinationAbs)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	return envhygiene.IsSensitivePath(relative) || isPrivateKeyFilename(relative)
}

func isPrivateKeyFilename(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if base == "id_rsa" || base == "id_ed25519" || base == "id_ecdsa" || base == "id_dsa" {
		return true
	}
	return strings.HasSuffix(base, ".key") || strings.HasSuffix(base, ".p12") || strings.HasSuffix(base, ".pfx")
}

func (m model) renderScreenshotCollectionFrame() string {
	width := max(1, m.width)
	contentHeight := m.frameContentHeight()
	var b strings.Builder
	title := "Screenshots"
	if m.screenshots.cycleID > 0 {
		title += fmt.Sprintf(" · cycle %d", m.screenshots.cycleID)
	}
	if m.screenshots.selector == screenshotSelectorTodos {
		title += " · ready-set snapshot"
	}
	b.WriteString(truncateDisplayWidth(headerStyle.Render(title), width))
	b.WriteByte('\n')
	if m.screenshots.loading {
		b.WriteString(infoStyle.Render("Loading and validating ready screenshots…"))
		b.WriteByte('\n')
	} else if m.screenshots.message != "" {
		for _, line := range wrapStatusPlain(m.screenshots.message, screenshotMessageStyle(m.screenshots.errorMessage), width) {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if m.screenshots.savePending {
		label := m.screenshots.saveDestination
		if label == "" {
			label = "<destination path>"
		}
		prompt := "Save path: " + label
		if m.screenshots.saveOverwrite {
			prompt += " · Y overwrite · N choose another · Esc cancel"
		} else {
			prompt += " · Enter save · Esc cancel"
		}
		b.WriteString(truncateDisplayWidth(prompt, width))
		b.WriteByte('\n')
	}
	if len(m.screenshots.assets) == 0 {
		if m.screenshots.message == "" && !m.screenshots.loading {
			b.WriteString(mutedStyle.Render("No screenshot has been captured yet."))
			b.WriteByte('\n')
		}
	} else {
		visible := m.screenshotVisibleCards()
		start := min(max(0, m.screenshots.offset), len(m.screenshots.assets)-1)
		end := min(len(m.screenshots.assets), start+visible)
		for i := start; i < end; i++ {
			manifest := m.screenshots.assets[i]
			prefix := "  "
			if i == m.screenshots.selected {
				prefix = "▸ "
			}
			line1 := fmt.Sprintf("%s%s · coverage: %s", prefix, safeScreenshotID(manifest.ID), safeDisplayValue(manifest.CoverageID))
			line2 := fmt.Sprintf("  stage: %s · attempt: %d · user: %s · profile: %s", safeDisplayValue(manifest.StageName), manifest.Attempt, safeDisplayValue(manifest.UserID), safeDisplayValue(manifest.ProfileID))
			line3 := fmt.Sprintf("  time: %s · result: %s", formatScreenshotTimestamp(manifest.CapturedAt), safeDisplayValue(manifest.Result))
			if i == m.screenshots.selected {
				b.WriteString(selectedStyle.Render(truncateDisplayWidth(line1, width)))
			} else {
				b.WriteString(truncateDisplayWidth(line1, width))
			}
			b.WriteByte('\n')
			b.WriteString(truncateDisplayWidth(line2, width))
			b.WriteByte('\n')
			b.WriteString(truncateDisplayWidth(line3, width))
			b.WriteByte('\n')
		}
		if start > 0 || end < len(m.screenshots.assets) {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("Showing %d–%d of %d", start+1, end, len(m.screenshots.assets))))
			b.WriteByte('\n')
		}
	}
	b.WriteString(mutedStyle.Render("↑/↓ select · Enter/O view · C copy path · S save · A open all · Esc close"))
	content := fitContentHeight(strings.TrimRight(b.String(), "\n"), contentHeight, false)
	return content + "\n" + m.renderStatusBar() + "\n" + m.renderBorderRule() + "\n" + m.renderFooter()
}

func formatScreenshotTimestamp(timestamp string) string {
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return safeDisplayValue(timestamp)
	}
	return parsed.Local().Format("2006-01-02 15:04:05 MST")
}

func screenshotMessageStyle(isError bool) lipgloss.Style {
	if isError {
		return warnStyle
	}
	return infoStyle
}

func safeDisplayValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "—"
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, value)
}
