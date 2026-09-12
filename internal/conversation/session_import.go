package conversation

import (
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// ShouldOfferRemoteImport reports whether the TUI should show the first-import
// confirmation dialog before reading provider history (PRD-C16-001 FR-11).
func (svc *SessionService) ShouldOfferRemoteImport(sess store.Session, hasLocalEvents bool) bool {
	if sess.RemoteImportConfirmed {
		return false
	}
	if strings.TrimSpace(sess.NativeSessionID) == "" {
		return false
	}
	if svc.Remote == nil || !svc.Remote.SupportsRemoteHistory() {
		return false
	}
	if hasLocalEvents && sess.TranscriptState != store.TranscriptUnavailableLegacy {
		return false
	}
	return true
}
