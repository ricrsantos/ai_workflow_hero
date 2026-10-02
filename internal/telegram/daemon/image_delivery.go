package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle/screenshots"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/media"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

const (
	// Telegram's Bot API sendPhoto documentation sets a 10 MB upload limit
	// (https://core.telegram.org/bots/api, checked 2026-10-01). Hero applies a
	// conservative local cap below that service limit to leave multipart headroom.
	telegramPhotoMaxBytes int64 = 9 * 1024 * 1024
	// Hero sends individual sendPhoto calls, not sendMediaGroup. Four is Hero's
	// bounded IPC/retry batch size; it is not a Bot API group-size assumption.
	telegramImageBatchMax = 4
)

var imageBatchIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,96}$`)

var errImageReference = errors.New("invalid managed screenshot reference")

// imageDeliveryMu serializes all batches so IPC arrival order is preserved
// across registered clients, and each bounded batch is delivered in order.
func (d *Daemon) sendImageBatch(ctx context.Context, conn interface{ Send(ipc.Message) error }, reg *client, message ipc.Message) {
	d.imageDeliveryMu.Lock()
	defer d.imageDeliveryMu.Unlock()
	result := d.deliverImageBatch(ctx, reg, message)
	if err := conn.Send(result); err != nil {
		// Avoid logging transport errors here: implementations may include
		// request details. The static event is enough to identify the failed edge.
		d.log.Error("telegram image delivery result could not be returned")
	}
}

func (d *Daemon) deliverImageBatch(ctx context.Context, reg *client, message ipc.Message) ipc.Message {
	result := ipc.Message{
		Type:              ipc.TypeImageDeliveryResult,
		ImageBatchID:      message.ImageBatchID,
		ImageRetryAttempt: message.ImageRetryAttempt,
	}
	defer func() {
		d.log.Debug("telegram image delivery batch processed",
			"requested_count", len(message.Images),
			"delivered_count", len(result.DeliveredScreenshotIDs),
			"failed_count", len(result.FailedScreenshotIDs),
		)
		if result.ImageDeliveryErrorCode == "" {
			if len(result.DeliveredScreenshotIDs) > 0 {
				d.log.Info("telegram image delivery batch completed",
					"delivered_count", len(result.DeliveredScreenshotIDs),
				)
			}
			return
		}
		if expectedImageDeliverySkip(result.ImageDeliveryErrorCode) {
			d.log.Info("telegram image delivery batch was not sent",
				"reason", result.ImageDeliveryErrorCode,
				"requested_count", len(message.Images),
			)
			return
		}
		d.log.Error("telegram image delivery batch failed",
			"error_code", result.ImageDeliveryErrorCode,
			"delivered_count", len(result.DeliveredScreenshotIDs),
			"failed_count", len(result.FailedScreenshotIDs),
		)
	}()
	ids := safeScreenshotIDs(message.Images)
	failAll := func(code string) ipc.Message {
		result.FailedScreenshotIDs = append([]string(nil), ids...)
		result.ImageDeliveryErrorCode = code
		return result
	}
	if reg == nil || reg.mode != ipc.ModeCycle || reg.projectDir == "" {
		return failAll("address_not_available")
	}
	if !clientHasCapability(reg, ipc.CapabilityImageDelivery) {
		return failAll("image_capability_unavailable")
	}
	if !imageBatchIDPattern.MatchString(message.ImageBatchID) || len(message.Images) == 0 || len(message.Images) > telegramImageBatchMax {
		return failAll("invalid_batch")
	}
	if ctx == nil || ctx.Err() != nil || !d.clientStillRegistered(reg) {
		return failAll("disconnected")
	}
	hero, err := install.LoadHeroJSON(reg.projectDir)
	if err != nil || !hero.Telegram.AlwaysSend {
		return failAll("always_send_disabled")
	}
	_, chatID, bot := d.creds()
	if chatID == "" {
		return failAll("not_paired")
	}
	if d.store == nil {
		return failAll("delivery_store_unavailable")
	}
	imageBot, ok := bot.(ImageBotAPI)
	if !ok || imageBot == nil {
		return failAll("image_transport_unavailable")
	}
	projectStore, err := store.OpenProject(reg.projectDir)
	if err != nil {
		return failAll("project_store_unavailable")
	}
	defer func() { _ = projectStore.Close() }()
	active, err := projectStore.GetActiveCycle()
	if err != nil {
		return failAll("screenshot_not_ready")
	}
	service, err := screenshots.NewService(reg.projectDir, projectStore)
	if err != nil {
		return failAll("screenshot_not_ready")
	}
	ready, err := service.ReadySet(ctx, active.ID)
	if err != nil {
		return failAll("screenshot_not_ready")
	}
	manifestByID := make(map[string]store.ScreenshotManifest, len(ready))
	for _, manifest := range ready {
		manifestByID[manifest.ID] = manifest
	}
	seen := make(map[string]struct{}, len(message.Images))
	for _, ref := range message.Images {
		if ctx.Err() != nil || !d.clientStillRegistered(reg) {
			result.FailedScreenshotIDs = append(result.FailedScreenshotIDs, ref.ScreenshotID)
			appendFailedRemainder(&result, message.Images, ref.ScreenshotID)
			if result.ImageDeliveryErrorCode == "" {
				result.ImageDeliveryErrorCode = "disconnected"
			}
			break
		}
		if _, duplicate := seen[ref.ScreenshotID]; duplicate {
			result.FailedScreenshotIDs = append(result.FailedScreenshotIDs, ref.ScreenshotID)
			setFirstImageError(&result, "invalid_batch")
			continue
		}
		seen[ref.ScreenshotID] = struct{}{}
		manifest, exists := manifestByID[ref.ScreenshotID]
		if !exists || !sameScreenshotReference(manifest, ref) {
			result.FailedScreenshotIDs = append(result.FailedScreenshotIDs, ref.ScreenshotID)
			setFirstImageError(&result, "invalid_reference")
			continue
		}
		deliveryID := screenshotDeliveryID(ref.ScreenshotID)
		claimed, delivered, claimErr := d.store.ClaimImageDelivery(deliveryID, d.now())
		if claimErr != nil {
			result.FailedScreenshotIDs = append(result.FailedScreenshotIDs, ref.ScreenshotID)
			setFirstImageError(&result, "delivery_store_unavailable")
			continue
		}
		if delivered {
			result.DeliveredScreenshotIDs = append(result.DeliveredScreenshotIDs, ref.ScreenshotID)
			continue
		}
		if !claimed {
			result.FailedScreenshotIDs = append(result.FailedScreenshotIDs, ref.ScreenshotID)
			setFirstImageError(&result, "delivery_in_progress")
			continue
		}
		photo, photoErr := loadManagedTelegramPhoto(ctx, reg.projectDir, active.ID, manifest)
		if photoErr != nil {
			_ = d.store.FinishImageDelivery(deliveryID, false, d.now())
			result.FailedScreenshotIDs = append(result.FailedScreenshotIDs, ref.ScreenshotID)
			setFirstImageError(&result, "image_validation_failed")
			continue
		}
		if !d.clientStillRegistered(reg) || !d.Paired() {
			_ = d.store.FinishImageDelivery(deliveryID, false, d.now())
			result.FailedScreenshotIDs = append(result.FailedScreenshotIDs, ref.ScreenshotID)
			appendFailedRemainder(&result, message.Images, ref.ScreenshotID)
			setFirstImageError(&result, "disconnected")
			break
		}
		sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		sendErr := imageBot.SendPhoto(sendCtx, chatID, photo)
		cancel()
		if sendErr != nil {
			_ = d.store.FinishImageDelivery(deliveryID, false, d.now())
			result.FailedScreenshotIDs = append(result.FailedScreenshotIDs, ref.ScreenshotID)
			setFirstImageError(&result, "bot_api_delivery_failed")
			continue
		}
		if err := d.store.FinishImageDelivery(deliveryID, true, d.now()); err != nil {
			// Report failure so the TUI can warn, but keep retryable persistence
			// failure visible rather than claiming durable deduplication.
			result.FailedScreenshotIDs = append(result.FailedScreenshotIDs, ref.ScreenshotID)
			setFirstImageError(&result, "delivery_store_unavailable")
			continue
		}
		result.DeliveredScreenshotIDs = append(result.DeliveredScreenshotIDs, ref.ScreenshotID)
	}
	return result
}

func expectedImageDeliverySkip(reason string) bool {
	switch reason {
	case "address_not_available", "image_capability_unavailable", "always_send_disabled", "not_paired", "disconnected":
		return true
	default:
		return false
	}
}

func safeScreenshotIDs(refs []ipc.ScreenshotImageRef) []string {
	ids := make([]string, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if validScreenshotID(ref.ScreenshotID) {
			if _, exists := seen[ref.ScreenshotID]; exists {
				continue
			}
			seen[ref.ScreenshotID] = struct{}{}
			ids = append(ids, ref.ScreenshotID)
		}
	}
	return ids
}

func clientHasCapability(reg *client, capability string) bool {
	for _, value := range reg.capabilities {
		if value == capability {
			return true
		}
	}
	return false
}

func (d *Daemon) clientStillRegistered(reg *client) bool {
	if reg == nil {
		return false
	}
	current, ok := d.registry.lookup(reg.address)
	return ok && current == reg
}

func appendFailedRemainder(result *ipc.Message, refs []ipc.ScreenshotImageRef, currentID string) {
	currentSeen := false
	for _, ref := range refs {
		if ref.ScreenshotID == currentID {
			currentSeen = true
			continue
		}
		if currentSeen && !containsString(result.FailedScreenshotIDs, ref.ScreenshotID) {
			result.FailedScreenshotIDs = append(result.FailedScreenshotIDs, ref.ScreenshotID)
		}
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func setFirstImageError(result *ipc.Message, code string) {
	if result.ImageDeliveryErrorCode == "" {
		result.ImageDeliveryErrorCode = code
	}
}

func validScreenshotID(value string) bool {
	if len(value) == 0 || len(value) > 96 {
		return false
	}
	for i, r := range value {
		if isASCIIAlphaNumeric(r) {
			continue
		}
		if i == 0 || (r != '_' && r != '-') {
			return false
		}
	}
	return true
}

func isASCIIAlphaNumeric(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

func sameScreenshotReference(manifest store.ScreenshotManifest, ref ipc.ScreenshotImageRef) bool {
	return manifest.CycleID == ref.CycleID && manifest.CycleID > 0 &&
		manifest.StageName == ref.StageName && manifest.Attempt == ref.Attempt &&
		manifest.CoverageID == ref.CoverageID && manifest.UserID == ref.UserID &&
		manifest.ProfileID == ref.ProfileID && manifest.CapturedAt == ref.CapturedAt &&
		manifest.Path == ref.Path && manifest.Result == ref.Result &&
		manifest.CaptureStatus == store.ScreenshotCaptureReady
}

func screenshotDeliveryID(screenshotID string) string {
	digest := sha256.Sum256([]byte("hero-telegram-screenshot-v1\x00" + screenshotID))
	return hex.EncodeToString(digest[:])
}

func loadManagedTelegramPhoto(ctx context.Context, projectDir string, cycleID int64, manifest store.ScreenshotManifest) (Photo, error) {
	if manifest.CycleID != cycleID || manifest.CaptureStatus != store.ScreenshotCaptureReady || !strings.HasPrefix(manifest.Path, "screenshots/") {
		return Photo{}, errImageReference
	}
	name := strings.TrimPrefix(manifest.Path, "screenshots/")
	if name == "" || filepath.Base(name) != name || strings.Contains(name, "/") || !strings.HasPrefix(name, manifest.ID+".") {
		return Photo{}, errImageReference
	}
	cycleRoot := filepath.Join(projectDir, ".workflow-hero", "cycles", "current")
	source := filepath.Join(cycleRoot, filepath.FromSlash(manifest.Path))
	limits := media.DefaultLimits()
	limits.MaxFileBytes = telegramPhotoMaxBytes
	validation, err := media.ValidateImage(ctx, source, media.ValidationOptions{
		Limits:    limits,
		Workspace: filepath.Join(cycleRoot, "screenshots"),
	})
	if err != nil || validation.Size > telegramPhotoMaxBytes || validation.ExternalPath {
		return Photo{}, errImageReference
	}
	shortSide := minInt(validation.Width, validation.Height)
	if shortSide <= 0 || validation.Width+validation.Height > 10_000 || maxInt(validation.Width, validation.Height) > 20*shortSide {
		return Photo{}, errImageReference
	}
	data, err := os.ReadFile(validation.CanonicalPath)
	if err != nil || int64(len(data)) != validation.Size {
		return Photo{}, errImageReference
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != validation.SHA256 {
		return Photo{}, errImageReference
	}
	var ext string
	switch validation.Format {
	case media.ImageFormatPNG:
		ext = ".png"
	case media.ImageFormatJPEG:
		ext = ".jpg"
	case media.ImageFormatGIF:
		ext = ".gif"
	case media.ImageFormatWebP:
		ext = ".webp"
	default:
		return Photo{}, errImageReference
	}
	caption := safeScreenshotCaption(manifest)
	return Photo{Filename: "screenshot" + ext, Caption: caption, Data: data}, nil
}

func safeScreenshotCaption(manifest store.ScreenshotManifest) string {
	parts := []string{strings.ReplaceAll(manifest.StageName, "_", " ")}
	if manifest.CoverageID != "" {
		parts = append(parts, "coverage "+manifest.CoverageID)
	}
	parts = append(parts, fmt.Sprintf("attempt %d", manifest.Attempt), manifest.Result)
	return strings.Join(parts, " · ")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
