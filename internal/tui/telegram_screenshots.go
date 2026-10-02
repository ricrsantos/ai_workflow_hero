package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

const telegramScreenshotBatchSize = 4 // Hero's bounded IPC batch; each image uses sendPhoto.

type telegramScreenshotSnapshotMsg struct {
	address  string
	selector string
	imageID  string
	snapshot screenshotSnapshotMsg
}

func (m model) telegramScreenshotRequest(address, selector, screenshotID string) (model, tea.Cmd) {
	if selector == "invalid" {
		return m, m.telegramOutboundCmd("Usage: /hero-screenshot [latest|list|<id>|todos].")
	}
	if m.telegram == nil || !m.telegram.alwaysSend {
		return m, m.telegramOutboundCmd("Screenshots are available in the TUI. Enable Always send reply in Telegram settings to receive images here.")
	}
	if m.telegram == nil || !m.telegram.connected || !m.telegram.paired || m.telegram.address == "" || address != m.telegram.address {
		return m, nil
	}
	if selector != screenshotSelectorList && !hasString(m.telegram.daemonCaps, ipc.CapabilityImageDelivery) {
		m = m.appendTelegramNotice("⚠ Screenshot forwarding unavailable: update the Telegram daemon to enable image delivery. Local screenshot cards remain available.")
		m = m.setStatusWarning("telegram", "Telegram daemon lacks image-delivery capability; update Hero to enable screenshot forwarding.")
		return m, m.telegramOutboundCmd("Screenshot forwarding is unavailable because the Telegram daemon needs an update. The local TUI screenshot collection remains available.")
	}
	if m.svc == nil || m.svc.Store == nil {
		return m, m.telegramOutboundCmd("No active cycle is available for screenshots.")
	}
	return m, telegramScreenshotSnapshotCmd(m.svc.ProjectDir, m.svc.Store, address, selector, screenshotID)
}

func telegramScreenshotSnapshotCmd(projectDir string, st *store.Store, address, selector, screenshotID string) tea.Cmd {
	base := loadScreenshotSnapshotCmd(projectDir, st, 0)
	return func() tea.Msg {
		message, ok := base().(screenshotSnapshotMsg)
		if !ok {
			return telegramScreenshotSnapshotMsg{address: address, selector: selector, imageID: screenshotID, snapshot: screenshotSnapshotMsg{err: true}}
		}
		return telegramScreenshotSnapshotMsg{address: address, selector: selector, imageID: screenshotID, snapshot: message}
	}
}

func (m model) handleTelegramScreenshotSnapshot(msg telegramScreenshotSnapshotMsg) (model, tea.Cmd) {
	if m.telegram == nil || !m.telegram.connected || !m.telegram.paired || m.telegram.address != msg.address || !m.telegram.alwaysSend {
		return m, nil
	}
	m.screenshots.generation++
	m.screenshots.open = true
	m.screenshots.loading = false
	m.screenshots.selector = msg.selector
	m.screenshots.requestedID = msg.imageID
	m.screenshots.cycleID = 0
	m.screenshots.assets = nil
	m.screenshots.selected = 0
	m.screenshots.offset = 0
	m.screenshots.message = ""
	m.screenshots.errorMessage = false
	msg.snapshot.generation = m.screenshots.generation
	m = m.handleScreenshotSnapshot(msg.snapshot)
	if msg.selector == screenshotSelectorList {
		return m, m.telegramOutboundCmd(screenshotListText(m.screenshots.assets, m.screenshots.message))
	}
	if msg.snapshot.noCycle || msg.snapshot.err || len(m.screenshots.assets) == 0 || strings.Contains(m.screenshots.message, "not found") {
		return m, m.telegramOutboundCmd(m.screenshots.message)
	}
	manifests := m.screenshots.assets
	if msg.selector == screenshotSelectorLatest || msg.selector == "id" {
		selected := m.screenshots.selected
		if selected < 0 || selected >= len(manifests) {
			return m, m.telegramOutboundCmd(m.screenshots.message)
		}
		manifests = []store.ScreenshotManifest{manifests[selected]}
	}
	return m, telegramScreenshotDeliveryCmd(m.telegram.client, manifests, 0)
}

func screenshotListText(assets []store.ScreenshotManifest, emptyMessage string) string {
	if len(assets) == 0 {
		if strings.TrimSpace(emptyMessage) != "" {
			return emptyMessage
		}
		return "No screenshot has been captured yet."
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("Ready screenshots (%d):", len(assets)))
	for _, asset := range assets {
		coverage := asset.CoverageID
		if coverage == "" {
			coverage = "no coverage ID"
		}
		profile := asset.ProfileID
		if profile == "" {
			profile = "no profile"
		}
		lines = append(lines, fmt.Sprintf("%s · %s · %s · attempt %d · %s · %s · %s",
			asset.ID, coverage, strings.ReplaceAll(asset.StageName, "_", " "), asset.Attempt, profile, asset.CapturedAt, asset.Result))
	}
	return strings.Join(lines, "\n")
}

func (m model) forwardScreenshotSnapshot(snapshot screenshotSnapshotMsg) (model, tea.Cmd) {
	if m.telegram == nil || !m.telegram.alwaysSend || snapshot.err || snapshot.noCycle || len(m.screenshots.assets) == 0 {
		return m, nil
	}
	if m.screenshots.selector == screenshotSelectorList {
		return m, nil
	}
	if !m.telegram.connected || !m.telegram.paired || m.telegram.address == "" {
		m = m.appendTelegramNotice("⚠ Screenshot delivery is unavailable while Telegram is disconnected or unpaired; local cards remain available.")
		m = m.setStatusWarning("telegram", "Screenshot delivery unavailable; local cards remain available.")
		return m, nil
	}
	if !hasString(m.telegram.daemonCaps, ipc.CapabilityImageDelivery) {
		m = m.appendTelegramNotice("⚠ Screenshot forwarding unavailable: update the Telegram daemon to enable image delivery. Local screenshot cards remain available.")
		m = m.setStatusWarning("telegram", "Telegram daemon lacks image-delivery capability; update Hero to enable screenshot forwarding.")
		return m, nil
	}
	manifests := m.screenshots.assets
	if m.screenshots.selector == screenshotSelectorLatest || m.screenshots.selector == "id" {
		if m.screenshots.selected < 0 || m.screenshots.selected >= len(manifests) {
			return m, nil
		}
		manifests = []store.ScreenshotManifest{manifests[m.screenshots.selected]}
	}
	return m, telegramScreenshotDeliveryCmd(m.telegram.client, manifests, 0)
}

func telegramScreenshotDeliveryCmd(client *telegramClient, manifests []store.ScreenshotManifest, retryAttempt int) tea.Cmd {
	if client == nil || len(manifests) == 0 {
		return nil
	}
	snapshot := append([]store.ScreenshotManifest(nil), manifests...)
	return func() tea.Msg {
		for start := 0; start < len(snapshot); start += telegramScreenshotBatchSize {
			end := min(start+telegramScreenshotBatchSize, len(snapshot))
			chunk := snapshot[start:end]
			refs := make([]ipc.ScreenshotImageRef, 0, len(chunk))
			ids := make([]string, 0, len(chunk))
			for _, manifest := range chunk {
				ids = append(ids, manifest.ID)
				refs = append(refs, ipc.ScreenshotImageRef{
					ScreenshotID: manifest.ID,
					CycleID:      manifest.CycleID,
					StageName:    manifest.StageName,
					Attempt:      manifest.Attempt,
					CoverageID:   manifest.CoverageID,
					UserID:       manifest.UserID,
					ProfileID:    manifest.ProfileID,
					CapturedAt:   manifest.CapturedAt,
					Path:         manifest.Path,
					Result:       manifest.Result,
				})
			}
			if err := client.Send(ipc.Message{
				Type:              ipc.TypeOutboundImageBatch,
				ImageBatchID:      screenshotBatchID(ids),
				ImageRetryAttempt: retryAttempt,
				Images:            refs,
			}); err != nil {
				failed := make([]string, 0, len(snapshot)-start)
				for _, pending := range snapshot[start:] {
					failed = append(failed, pending.ID)
				}
				return telegramImageSendErrorMsg{batchID: screenshotBatchID(ids), failed: failed}
			}
		}
		return nil
	}
}

func screenshotBatchID(ids []string) string {
	hash := sha256.Sum256([]byte("hero-image-batch-v1\x00" + strings.Join(ids, "\x00")))
	return "batch-" + hex.EncodeToString(hash[:16])
}

func hasString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func (m model) handleTelegramImageDeliveryResult(msg telegramImageDeliveryResultMsg) (model, tea.Cmd) {
	if len(msg.failed) == 0 && msg.errorCode == "" {
		m = m.appendTelegramNotice(fmt.Sprintf("✓ Telegram screenshot delivery complete (%d image(s)).", len(msg.delivered)))
		m = m.setStatusResult(true, "telegram", fmt.Sprintf("Delivered %d screenshot(s) to the paired chat.", len(msg.delivered)))
		return m, nil
	}
	if len(msg.failed) == 0 {
		m = m.appendTelegramNotice("⚠ Telegram screenshot batch was not delivered; local cards remain available.")
		m = m.setStatusWarning("telegram", "Screenshot batch was not delivered; local cards remain available.")
		return m, nil
	}
	m = m.appendTelegramNotice(fmt.Sprintf("⚠ Telegram screenshot delivery incomplete; failed IDs: %s. Local cards remain available.", strings.Join(msg.failed, ", ")))
	m = m.setStatusWarning("telegram", "Screenshot delivery incomplete; local cards remain available.")
	if msg.retryAttempt > 0 || msg.errorCode != "bot_api_delivery_failed" || m.telegram == nil || !m.telegram.connected || !m.telegram.alwaysSend {
		return m, nil
	}
	failed := make(map[string]struct{}, len(msg.failed))
	for _, id := range msg.failed {
		failed[id] = struct{}{}
	}
	var retry []store.ScreenshotManifest
	for _, manifest := range m.screenshots.assets {
		if _, ok := failed[manifest.ID]; ok {
			retry = append(retry, manifest)
		}
	}
	if len(retry) == 0 {
		return m, nil
	}
	m = m.appendTelegramNotice(fmt.Sprintf("Retrying %d failed screenshot(s) once…", len(retry)))
	return m, telegramScreenshotDeliveryCmd(m.telegram.client, retry, 1)
}
