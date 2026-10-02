package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/screenshots"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

const screenshotWatchInterval = 2 * time.Second

type screenshotWatchTickMsg struct{ generation uint64 }

type screenshotWatchSnapshotMsg struct {
	generation uint64
	stage      string
	attempt    int
	cycleID    int64
	assets     []store.ScreenshotManifest
	active     bool
	enabled    bool
	err        bool
}

func screenshotWatchTickCmd(generation uint64) tea.Cmd {
	return tea.Tick(screenshotWatchInterval, func(time.Time) tea.Msg {
		return screenshotWatchTickMsg{generation: generation}
	})
}

func screenshotWatchSnapshotCmd(projectDir string, st *store.Store, stageName string, attempt int, generation uint64) tea.Cmd {
	return func() tea.Msg {
		msg := screenshotWatchSnapshotMsg{generation: generation, stage: stageName, attempt: attempt}
		if st == nil || strings.TrimSpace(projectDir) == "" || attempt <= 0 {
			msg.err = true
			return msg
		}
		doc, err := workflowconfig.LoadCurrentDocument(projectDir)
		if err != nil {
			msg.err = true
			return msg
		}
		stageConfig, ok := doc.Config.Stages[stageName]
		if !ok || !stageConfig.Enabled {
			return msg
		}
		if !stageConfig.Screenshots.Enabled || (stageName == stageQAEndToEnd && !stageConfig.UsePlaywright) {
			msg.active = true
			return msg
		}
		cycle, err := st.GetActiveCycle()
		if err != nil {
			return msg
		}
		stages, err := st.ListStages(cycle.ID)
		if err != nil {
			msg.err = true
			return msg
		}
		for _, stage := range stages {
			if stage.Name == stageName && stage.Status == store.StageRunning && stage.Iteration == attempt {
				msg.active = true
				break
			}
		}
		if !msg.active {
			return msg
		}
		msg.enabled = true
		msg.cycleID = cycle.ID
		service, err := screenshots.NewService(projectDir, st)
		if err != nil {
			msg.err = true
			return msg
		}
		assets, err := service.ReadySet(context.Background(), cycle.ID)
		if err != nil {
			msg.err = true
			return msg
		}
		for _, asset := range assets {
			if asset.StageName == stageName && asset.Attempt == attempt {
				msg.assets = append(msg.assets, asset)
			}
		}
		return msg
	}
}

func (m model) beginScreenshotAutoWatch(stageName string, attempt int) model {
	state := m.screenshots
	state.autoWatchGen++
	state.autoWatchActive = stageName == stageBrowserUI || stageName == stageQAEndToEnd
	state.autoWatchStage = stageName
	state.autoWatchAttempt = attempt
	state.autoWatchCycleID = 0
	state.autoWatchSeen = nil
	state.autoWatchWarning = false
	m.screenshots = state
	if state.autoWatchActive {
		slog.Info("tui automatic screenshot observation started", "stage", stageName, "attempt", attempt)
	}
	return m
}

func (m model) handleScreenshotWatchTick(msg screenshotWatchTickMsg) (model, tea.Cmd) {
	state := m.screenshots
	if !state.autoWatchActive || msg.generation != state.autoWatchGen || m.svc == nil || m.svc.Store == nil {
		return m, nil
	}
	return m, screenshotWatchSnapshotCmd(m.svc.ProjectDir, m.svc.Store, state.autoWatchStage, state.autoWatchAttempt, state.autoWatchGen)
}

func (m model) handleScreenshotWatchSnapshot(msg screenshotWatchSnapshotMsg) (model, tea.Cmd) {
	state := m.screenshots
	if !state.autoWatchActive || msg.generation != state.autoWatchGen || msg.stage != state.autoWatchStage || msg.attempt != state.autoWatchAttempt {
		return m, nil
	}
	if msg.err {
		state.autoWatchActive = false
		m.screenshots = state
		slogScreenshotWatchFailure()
		return m, nil
	}
	if !msg.active || !msg.enabled {
		state.autoWatchActive = false
		m.screenshots = state
		return m, nil
	}
	if state.autoWatchCycleID != msg.cycleID {
		state.autoWatchCycleID = msg.cycleID
		state.autoWatchSeen = nil
		state.assets = nil
	}
	for _, asset := range msg.assets {
		if !hasScreenshotManifest(state.assets, asset.ID) {
			state.assets = append(state.assets, asset)
		}
	}
	state.cycleID = msg.cycleID
	newAssets := make([]store.ScreenshotManifest, 0, len(msg.assets))
	for _, asset := range msg.assets {
		if !containsScreenshotID(state.autoWatchSeen, asset.ID) {
			newAssets = append(newAssets, asset)
		}
	}
	if m.screenshots.open {
		state = m.ensureScreenshotSelectionVisibleWithState(state)
	}
	m.screenshots = state

	var delivery tea.Cmd
	if len(newAssets) > 0 && m.telegram != nil && m.telegram.alwaysSend {
		switch {
		case !m.telegram.connected || !m.telegram.paired || strings.TrimSpace(m.telegram.address) == "":
			m = m.warnScreenshotAutoForward("Telegram is disconnected or unpaired; local screenshot cards remain available.")
		case !hasString(m.telegram.daemonCaps, ipc.CapabilityImageDelivery):
			m = m.warnScreenshotAutoForward("Telegram daemon lacks image-delivery support; local screenshot cards remain available.")
		default:
			for _, asset := range newAssets {
				state = m.screenshots
				state.autoWatchSeen = append(state.autoWatchSeen, asset.ID)
				m.screenshots = state
			}
			m = m.appendTelegramNotice(fmt.Sprintf("Sending %d newly captured screenshot(s) to the paired chat…", len(newAssets)))
			slog.Info("tui automatic screenshot delivery queued", "stage", msg.stage, "attempt", msg.attempt, "count", len(newAssets))
			delivery = telegramScreenshotDeliveryCmd(m.telegram.client, newAssets, 0)
		}
	}
	if m.screenshots.autoWatchActive {
		return m, tea.Batch(delivery, screenshotWatchTickCmd(msg.generation))
	}
	return m, delivery
}

func (m model) warnScreenshotAutoForward(message string) model {
	if m.screenshots.autoWatchWarning {
		return m
	}
	m.screenshots.autoWatchWarning = true
	m = m.appendTelegramNotice("⚠ " + message)
	return m.setStatusWarning("telegram", message)
}

func (m model) ensureScreenshotSelectionVisibleWithState(state screenshotCollectionState) screenshotCollectionState {
	current := m.screenshots
	m.screenshots = state
	m = m.ensureScreenshotSelectionVisible()
	state = m.screenshots
	m.screenshots = current
	return state
}

func hasScreenshotManifest(assets []store.ScreenshotManifest, id string) bool {
	for _, asset := range assets {
		if asset.ID == id {
			return true
		}
	}
	return false
}

func containsScreenshotID(ids []string, id string) bool {
	for _, value := range ids {
		if value == id {
			return true
		}
	}
	return false
}

func (m model) handleTelegramImageSendError(msg telegramImageSendErrorMsg) (model, tea.Cmd) {
	for _, id := range msg.failed {
		m.screenshots.autoWatchSeen = removeScreenshotID(m.screenshots.autoWatchSeen, id)
	}
	message := "Screenshot transfer could not reach the daemon; local cards remain available."
	if len(msg.failed) > 0 {
		message = "Screenshot transfer could not reach the daemon; failed IDs: " + strings.Join(msg.failed, ", ") + ". Local cards remain available."
	}
	m = m.appendTelegramNotice("⚠ " + message)
	m = m.setStatusWarning("telegram", message)
	if m.screenshots.autoWatchActive {
		return m, screenshotWatchTickCmd(m.screenshots.autoWatchGen)
	}
	return m, nil
}

func removeScreenshotID(ids []string, id string) []string {
	filtered := make([]string, 0, len(ids))
	for _, value := range ids {
		if value != id {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func slogScreenshotWatchFailure() {
	// Deliberately omit underlying errors: database and filesystem diagnostics
	// must not add unmanaged paths or other project data to routine UI notices.
	slog.Error("tui automatic screenshot observation stopped after validation failed")
}
