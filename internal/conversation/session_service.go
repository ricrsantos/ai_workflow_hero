package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// ForkContextMaxBytes is the UTF-8 byte budget for fork context blobs (design D6).
const ForkContextMaxBytes = 32 * 1024

// SessionService owns durable Hero session lifecycle without UI dependencies (ADR-093).
type SessionService struct {
	Store   *store.Store
	Clock   store.Clock
	Remote  harness.RemoteHistoryReader
	Deleter harness.NativeSessionDeleter
	Log     *slog.Logger
	// setRemoteImportConfirmed overrides Store.SetRemoteImportConfirmed (tests only).
	setRemoteImportConfirmed func(id string, confirmed bool) error
}

// NewSessionService wires a session service with store and clock.
func NewSessionService(st *store.Store, clock store.Clock) *SessionService {
	if clock == nil {
		clock = store.DefaultClock()
	}
	return &SessionService{
		Store: st,
		Clock: clock,
		Log:   slog.Default(),
	}
}

func (svc *SessionService) log() *slog.Logger {
	if svc == nil || svc.Log == nil {
		return slog.Default()
	}
	return svc.Log
}

// FirstTurnContent describes an accepted user or stage-agent prompt for persistence.
type FirstTurnContent struct {
	Text             string
	AttachmentCount  int
	StageAgentPrompt string
	Origin           Origin
	OriginAddress    string
	UserPayloadJSON  string // optional; built when empty
	ProviderEventID  string
}

// HasAcceptedTurn reports whether persistence should create a session row.
func HasAcceptedTurn(c FirstTurnContent) bool {
	if strings.TrimSpace(c.StageAgentPrompt) != "" {
		return true
	}
	if strings.TrimSpace(c.Text) != "" {
		return true
	}
	return c.AttachmentCount > 0
}

// CreateSessionParams carries metadata for a new Hero session row.
type CreateSessionParams struct {
	ID                  string // optional predetermined primary key
	Kind                string
	Title               string
	HarnessID           string
	Model               string
	ModelPropertiesJSON string
	CycleID             *int64
	StageName           string
	AgentName           string
	TranscriptState     string
	LastOrigin          string
}

// EnsureFirstTurnResult is the outcome of create-on-first-turn persistence.
type EnsureFirstTurnResult struct {
	SessionID  string
	Session    store.Session
	FirstEvent store.SessionEvent
	Created    bool
}

// EnsureFirstTurn creates the session and first event atomically when heroSessionID is empty
// and the turn is accepted. Empty Chat surfaces return zero SessionID without error.
func (svc *SessionService) EnsureFirstTurn(ctx context.Context, heroSessionID string, meta CreateSessionParams, turn FirstTurnContent) (EnsureFirstTurnResult, error) {
	if err := ctx.Err(); err != nil {
		return EnsureFirstTurnResult{}, err
	}
	if svc == nil || svc.Store == nil {
		return EnsureFirstTurnResult{}, fmt.Errorf("session service store is required")
	}
	heroSessionID = strings.TrimSpace(heroSessionID)
	if heroSessionID != "" {
		return EnsureFirstTurnResult{SessionID: heroSessionID}, nil
	}
	if !HasAcceptedTurn(turn) {
		return EnsureFirstTurnResult{}, nil
	}

	origin := storeOrigin(turn.Origin)
	eventPayload := strings.TrimSpace(turn.UserPayloadJSON)
	if eventPayload == "" {
		built, err := buildUserEventPayload(turn)
		if err != nil {
			return EnsureFirstTurnResult{}, err
		}
		eventPayload = built
	}

	sessIn := store.CreateSessionInput{
		ID:                  strings.TrimSpace(meta.ID),
		Kind:                meta.Kind,
		Title:               meta.Title,
		HarnessID:           meta.HarnessID,
		Model:               meta.Model,
		ModelPropertiesJSON: meta.ModelPropertiesJSON,
		CycleID:             meta.CycleID,
		StageName:           meta.StageName,
		AgentName:           meta.AgentName,
		TranscriptState:     meta.TranscriptState,
		LastOrigin:          meta.LastOrigin,
	}
	if sessIn.Kind == "" {
		sessIn.Kind = store.SessionKindFreechat
	}
	if sessIn.Title == "" {
		sessIn.Title = TitleFreeChat(turn.Text, turn.AttachmentCount > 0 && strings.TrimSpace(turn.Text) == "" && strings.TrimSpace(turn.StageAgentPrompt) == "")
	}
	if sessIn.TranscriptState == "" {
		sessIn.TranscriptState = store.TranscriptAvailable
	}
	if sessIn.LastOrigin == "" {
		sessIn.LastOrigin = origin
	}

	eventType := store.SessionEventUser
	if p := strings.TrimSpace(turn.StageAgentPrompt); p != "" && strings.TrimSpace(turn.Text) == "" && turn.AttachmentCount == 0 {
		eventPayload = mustUserPayloadJSON(p)
	}

	sess, ev, err := svc.Store.CreateSessionWithFirstEvent(sessIn, store.AppendSessionEventInput{
		EventType:       eventType,
		Origin:          origin,
		OriginAddress:   strings.TrimSpace(turn.OriginAddress),
		PayloadJSON:     eventPayload,
		ProviderEventID: strings.TrimSpace(turn.ProviderEventID),
	})
	if err != nil {
		svc.log().Error("session first-turn persistence failed", "kind", sessIn.Kind)
		return EnsureFirstTurnResult{}, err
	}
	svc.log().Info("session created on first turn", logAttrKind(sess.Kind)...)
	return EnsureFirstTurnResult{
		SessionID:  sess.ID,
		Session:    sess,
		FirstEvent: ev,
		Created:    true,
	}, nil
}

// AppendEvent appends a transcript event bound to the active Execute session.
func (svc *SessionService) AppendEvent(ctx context.Context, in store.AppendSessionEventInput) (store.SessionEvent, error) {
	if err := ctx.Err(); err != nil {
		return store.SessionEvent{}, err
	}
	ev, err := svc.Store.AppendSessionEvent(in)
	if err != nil {
		svc.log().Error("session event append failed", "event_type", in.EventType)
		return store.SessionEvent{}, err
	}
	return ev, nil
}

// GetSession loads a Hero session aggregate.
func (svc *SessionService) GetSession(ctx context.Context, id string) (store.Session, error) {
	if err := ctx.Err(); err != nil {
		return store.Session{}, err
	}
	return svc.Store.GetSession(id)
}

// RenameSession trims and updates the session title.
func (svc *SessionService) RenameSession(ctx context.Context, id, title string) (store.Session, error) {
	if err := ctx.Err(); err != nil {
		return store.Session{}, err
	}
	sess, err := svc.Store.UpdateSessionTitle(id, title)
	if err != nil {
		if errors.Is(err, store.ErrEmptySessionTitle) {
			svc.log().Error("session rename rejected empty title")
		}
		return store.Session{}, err
	}
	svc.log().Info("session renamed")
	return sess, nil
}

// ListSessions lists History rows (read-only; no lease required).
func (svc *SessionService) ListSessions(ctx context.Context, filter store.ListSessionsFilter) ([]store.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return svc.Store.ListSessions(filter)
}

// ArchiveSession marks a session archived.
func (svc *SessionService) ArchiveSession(ctx context.Context, id string) (store.Session, error) {
	if err := ctx.Err(); err != nil {
		return store.Session{}, err
	}
	sess, err := svc.Store.UpdateSessionLifecycle(id, store.SessionLifecycleArchived, nil)
	if err != nil {
		return store.Session{}, err
	}
	svc.log().Info("session archived")
	return sess, nil
}

// RestoreSession returns an archived session to the active list.
func (svc *SessionService) RestoreSession(ctx context.Context, id string) (store.Session, error) {
	if err := ctx.Err(); err != nil {
		return store.Session{}, err
	}
	sess, err := svc.Store.UpdateSessionLifecycle(id, store.SessionLifecycleActive, nil)
	if err != nil {
		return store.Session{}, err
	}
	svc.log().Info("session restored")
	return sess, nil
}

// MarkInterrupted sets lifecycle=interrupted for crash mid-turn recovery.
func (svc *SessionService) MarkInterrupted(ctx context.Context, id string) (store.Session, error) {
	if err := ctx.Err(); err != nil {
		return store.Session{}, err
	}
	at := svc.Clock.Now().UTC().Format(time.RFC3339)
	sess, err := svc.Store.UpdateSessionLifecycle(id, store.SessionLifecycleInterrupted, &at)
	if err != nil {
		return store.Session{}, err
	}
	svc.log().Info("session marked interrupted")
	return sess, nil
}

// AcquireLease obtains the exclusive continuation lease for ownerID.
func (svc *SessionService) AcquireLease(ctx context.Context, sessionID, ownerID string) (store.AcquireLeaseResult, error) {
	if err := ctx.Err(); err != nil {
		return store.AcquireLeaseResult{}, err
	}
	res, err := svc.Store.AcquireSessionLease(sessionID, ownerID, svc.Clock)
	if err != nil {
		if errors.Is(err, store.ErrSessionBusy) {
			svc.log().Info("session lease busy")
		}
		return store.AcquireLeaseResult{}, err
	}
	return res, nil
}

// HeartbeatLease extends the lease TTL for ownerID.
func (svc *SessionService) HeartbeatLease(ctx context.Context, sessionID, ownerID string) (store.SessionLease, error) {
	if err := ctx.Err(); err != nil {
		return store.SessionLease{}, err
	}
	return svc.Store.HeartbeatSessionLease(sessionID, ownerID, svc.Clock)
}

// ReleaseLease drops the lease when owned by ownerID.
func (svc *SessionService) ReleaseLease(ctx context.Context, sessionID, ownerID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return svc.Store.ReleaseSessionLease(sessionID, ownerID)
}

// ResumeBinding returns harness/model/native attributes for exact resume (ADR-095).
type ResumeBinding struct {
	SessionID           string
	HarnessID           string
	NativeSessionID     string
	Model               string
	ModelPropertiesJSON string
	Kind                string
	TranscriptState     string
	Lifecycle           string
}

// ResumeBinding loads stored native identity for Chat continuation.
func (svc *SessionService) ResumeBinding(ctx context.Context, sessionID string) (ResumeBinding, error) {
	if err := ctx.Err(); err != nil {
		return ResumeBinding{}, err
	}
	sess, err := svc.Store.GetSession(sessionID)
	if err != nil {
		return ResumeBinding{}, err
	}
	return ResumeBinding{
		SessionID:           sess.ID,
		HarnessID:           sess.HarnessID,
		NativeSessionID:     sess.NativeSessionID,
		Model:               sess.Model,
		ModelPropertiesJSON: sess.ModelPropertiesJSON,
		Kind:                sess.Kind,
		TranscriptState:     sess.TranscriptState,
		Lifecycle:           sess.Lifecycle,
	}, nil
}

// BindNativeSession records harness/native identity after first Execute bind.
func (svc *SessionService) BindNativeSession(ctx context.Context, id, harnessID, nativeSessionID, model, props string) (store.Session, error) {
	if err := ctx.Err(); err != nil {
		return store.Session{}, err
	}
	sess, err := svc.Store.BindNativeSession(id, harnessID, nativeSessionID, model, props)
	if err != nil {
		return store.Session{}, err
	}
	svc.log().Info("session native bound", "harness_id", strings.TrimSpace(harnessID))
	return sess, nil
}

// ListEventsNewest loads a bounded transcript page for restore/fork helpers.
func (svc *SessionService) ListEventsNewest(ctx context.Context, sessionID string, beforeSeq int64, limit int) ([]store.SessionEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return svc.Store.ListSessionEventsNewest(sessionID, beforeSeq, limit)
}

// ForkSessionInput configures an explicit context fork (design D6).
type ForkSessionInput struct {
	SourceSessionID     string
	Kind                string
	Title               string
	HarnessID           string
	Model               string
	ModelPropertiesJSON string
	CycleID             *int64
	StageName           string
	AgentName           string
}

// ForkSession creates a new Hero session seeded with a deterministic context note; the source row is unchanged.
func (svc *SessionService) ForkSession(ctx context.Context, in ForkSessionInput) (store.Session, error) {
	if err := ctx.Err(); err != nil {
		return store.Session{}, err
	}
	source, err := svc.Store.GetSession(strings.TrimSpace(in.SourceSessionID))
	if err != nil {
		return store.Session{}, err
	}
	blob, legacyNote := svc.BuildForkContextBlob(ctx, source)
	noteBody := FormatForkNote(source.Title, blob, legacyNote)

	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = source.Kind
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = source.Title
	}
	harnessID := strings.TrimSpace(in.HarnessID)
	if harnessID == "" {
		harnessID = source.HarnessID
	}
	model := strings.TrimSpace(in.Model)
	if model == "" {
		model = source.Model
	}
	props := strings.TrimSpace(in.ModelPropertiesJSON)
	if props == "" {
		props = source.ModelPropertiesJSON
	}

	sess, _, err := svc.Store.CreateSessionWithFirstEvent(store.CreateSessionInput{
		Kind:                kind,
		Title:               title,
		HarnessID:           harnessID,
		Model:               model,
		ModelPropertiesJSON: props,
		CycleID:             in.CycleID,
		StageName:           coalesceNonEmpty(in.StageName, source.StageName),
		AgentName:           coalesceNonEmpty(in.AgentName, source.AgentName),
		TranscriptState:     store.TranscriptAvailable,
	}, store.AppendSessionEventInput{
		EventType:   store.SessionEventNote,
		Origin:      store.SessionOriginLocal,
		PayloadJSON: mustNotePayloadJSON(noteBody),
	})
	if err != nil {
		svc.log().Error("session fork persistence failed")
		return store.Session{}, err
	}
	svc.log().Info("session fork created")
	return sess, nil
}

// BuildForkContextBlob collects user/assistant text newest-first up to ForkContextMaxBytes, then chronological.
func (svc *SessionService) BuildForkContextBlob(ctx context.Context, source store.Session) (string, string) {
	if source.TranscriptState == store.TranscriptUnavailableLegacy {
		return "", "No local transcript is available for this legacy session."
	}
	events, err := svc.loadAllTextEvents(ctx, source.ID)
	if err != nil || len(events) == 0 {
		return "", ""
	}
	return buildForkBlobFromEvents(events), ""
}

func (svc *SessionService) loadAllTextEvents(ctx context.Context, sessionID string) ([]store.SessionEvent, error) {
	var all []store.SessionEvent
	before := int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page, err := svc.Store.ListSessionEventsNewest(sessionID, before, store.DefaultSessionEventPage)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		all = append(page, all...)
		before = page[0].Seq
		if len(page) < store.DefaultSessionEventPage {
			break
		}
	}
	return all, nil
}

// FormatForkNote renders the fork note body with prefix and optional legacy explanation.
func FormatForkNote(sourceTitle, contextBlob, legacyNote string) string {
	prefix := fmt.Sprintf("[Hero context fork from session %s]", strings.TrimSpace(sourceTitle))
	var b strings.Builder
	b.WriteString(prefix)
	if legacyNote != "" {
		b.WriteString("\n\n")
		b.WriteString(legacyNote)
	}
	if strings.TrimSpace(contextBlob) != "" {
		b.WriteString("\n\n")
		b.WriteString(contextBlob)
	}
	return b.String()
}

func buildForkBlobFromEvents(events []store.SessionEvent) string {
	type chunk struct {
		seq  int64
		line string
	}
	var chunks []chunk
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		if ev.EventType != store.SessionEventUser && ev.EventType != store.SessionEventAssistant {
			continue
		}
		text := extractPayloadText(ev.PayloadJSON)
		if text == "" {
			continue
		}
		chunks = append(chunks, chunk{seq: ev.Seq, line: ev.EventType + ": " + text})
	}
	var size int
	var picked []chunk
	for _, c := range chunks {
		add := len(c.line)
		if len(picked) > 0 {
			add++ // newline between lines when reversed
		}
		if size+add > ForkContextMaxBytes {
			break
		}
		picked = append(picked, c)
		size += add
	}
	if len(picked) == 0 {
		return ""
	}
	for i, j := 0, len(picked)-1; i < j; i, j = i+1, j-1 {
		picked[i], picked[j] = picked[j], picked[i]
	}
	var b strings.Builder
	for i, c := range picked {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(c.line)
	}
	return b.String()
}

// ImportRemoteHistory appends provider events after explicit user confirmation.
func (svc *SessionService) ImportRemoteHistory(ctx context.Context, sessionID string, userConfirmed bool) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if svc.Remote == nil || !svc.Remote.SupportsRemoteHistory() {
		return 0, fmt.Errorf("remote history import is not supported")
	}
	sess, err := svc.Store.GetSession(sessionID)
	if err != nil {
		return 0, err
	}
	if !userConfirmed && !sess.RemoteImportConfirmed {
		return 0, fmt.Errorf("remote history import requires user confirmation")
	}
	if sess.NativeSessionID == "" {
		return 0, fmt.Errorf("session has no native id for remote import")
	}
	events, err := svc.Remote.ReadRemoteHistory(ctx, sess.NativeSessionID)
	if err != nil {
		svc.log().Error("remote history read failed")
		return 0, err
	}
	appendInputs := make([]store.AppendSessionEventInput, 0, len(events))
	for _, ne := range events {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		payload := "{}"
		if len(ne.PayloadJSON) > 0 {
			payload = string(ne.PayloadJSON)
		}
		createdAt := ""
		if !ne.CreatedAt.IsZero() {
			createdAt = ne.CreatedAt.UTC().Format(time.RFC3339)
		}
		appendInputs = append(appendInputs, store.AppendSessionEventInput{
			BoundSessionID:  sess.ID,
			SessionID:       sess.ID,
			EventType:       ne.EventType,
			Origin:          ne.Origin,
			OriginAddress:   ne.OriginAddress,
			PayloadJSON:     payload,
			ProviderEventID: ne.ProviderEventID,
			CreatedAt:       createdAt,
		})
	}
	imported, err := svc.Store.ImportRemoteSessionEvents(store.ImportRemoteSessionEventsInput{
		SessionID:                sess.ID,
		Events:                   appendInputs,
		SetRemoteImportConfirmed: userConfirmed,
		SetTranscriptAvailable:   userConfirmed,
	})
	if err != nil {
		if userConfirmed {
			svc.log().Error("remote history import transaction failed", logAttrImported(imported)...)
		}
		return 0, err
	}
	svc.log().Info("remote history import completed", logAttrImported(imported)...)
	return imported, nil
}

func (svc *SessionService) persistRemoteImportConfirmed(id string, confirmed bool) error {
	if svc == nil || svc.Store == nil {
		return fmt.Errorf("session service store is required")
	}
	if svc.setRemoteImportConfirmed != nil {
		return svc.setRemoteImportConfirmed(id, confirmed)
	}
	return svc.Store.SetRemoteImportConfirmed(id, confirmed)
}

// DeleteLocalFirst removes the SQLite session row and returns managed paths to purge.
func (svc *SessionService) DeleteLocalFirst(ctx context.Context, sessionID string) (store.SessionDeleteOp, []string, error) {
	if err := ctx.Err(); err != nil {
		return store.SessionDeleteOp{}, nil, err
	}
	op, paths, err := svc.Store.DeleteSessionLocalFirst(sessionID)
	if err != nil {
		svc.log().Error("session local delete failed")
		return store.SessionDeleteOp{}, nil, err
	}
	svc.log().Info("session local delete completed")
	return op, paths, nil
}

// PurgeManagedAssetFiles removes managed_copy files after a successful local SQLite delete.
func PurgeManagedAssetFiles(paths []string) error {
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// ResumeIncompleteDelete finishes a recoverable delete op: purge managed files when still at intent, then advance status.
func (svc *SessionService) ResumeIncompleteDelete(ctx context.Context, op store.SessionDeleteOp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if svc.Store == nil {
		return fmt.Errorf("session service store is required")
	}
	warn := remoteDeleteWarning(svc, op)
	switch op.Status {
	case store.DeleteOpIntent:
		paths, err := svc.Store.ListManagedPathsForDeleteOp(op.ID)
		if err != nil {
			return err
		}
		if err := PurgeManagedAssetFiles(paths); err != nil {
			svc.log().Error("session delete managed purge failed")
			return err
		}
		return svc.CompleteDeleteAfterPurge(ctx, op, warn)
	case store.DeleteOpLocalPurged, store.DeleteOpRemoteAttempted:
		return svc.CompleteDeleteAfterPurge(ctx, op, warn)
	default:
		return fmt.Errorf("unexpected delete op status %q", op.Status)
	}
}

func remoteDeleteWarning(svc *SessionService, op store.SessionDeleteOp) string {
	warn := strings.TrimSpace(op.RemoteWarning)
	if warn == "" && strings.TrimSpace(op.NativeSessionID) != "" {
		if svc.Deleter == nil || !svc.Deleter.SupportsNativeDelete() {
			warn = "Provider-side data may remain."
		}
	}
	return warn
}

// CompleteDeleteAfterPurge advances the delete op after filesystem purge and optional native delete.
func (svc *SessionService) CompleteDeleteAfterPurge(ctx context.Context, op store.SessionDeleteOp, remoteWarning string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := svc.Store.UpdateSessionDeleteOpStatus(op.ID, store.DeleteOpLocalPurged, ""); err != nil {
		return err
	}
	warn := strings.TrimSpace(remoteWarning)
	if svc.Deleter != nil && svc.Deleter.SupportsNativeDelete() && strings.TrimSpace(op.NativeSessionID) != "" {
		if err := svc.Deleter.DeleteNativeSession(ctx, op.NativeSessionID); err != nil {
			warn = "Provider-side session data may remain."
			svc.log().Error("native session delete failed")
			if err := svc.Store.UpdateSessionDeleteOpStatus(op.ID, store.DeleteOpRemoteAttempted, warn); err != nil {
				return err
			}
			return svc.Store.UpdateSessionDeleteOpStatus(op.ID, store.DeleteOpCompleted, warn)
		}
	}
	status := store.DeleteOpCompleted
	if warn != "" {
		if err := svc.Store.UpdateSessionDeleteOpStatus(op.ID, store.DeleteOpRemoteAttempted, warn); err != nil {
			return err
		}
	} else if svc.Deleter != nil && svc.Deleter.SupportsNativeDelete() && op.NativeSessionID != "" {
		if err := svc.Store.UpdateSessionDeleteOpStatus(op.ID, store.DeleteOpRemoteAttempted, ""); err != nil {
			return err
		}
	}
	return svc.Store.UpdateSessionDeleteOpStatus(op.ID, status, warn)
}

func storeOrigin(o Origin) string {
	switch o {
	case OriginTelegram:
		return store.SessionOriginTelegram
	default:
		return store.SessionOriginLocal
	}
}

func buildUserEventPayload(turn FirstTurnContent) (string, error) {
	text := strings.TrimSpace(turn.Text)
	if text == "" && strings.TrimSpace(turn.StageAgentPrompt) != "" {
		text = strings.TrimSpace(turn.StageAgentPrompt)
	}
	body := map[string]any{
		"text": text,
	}
	if turn.AttachmentCount > 0 {
		body["attachment_count"] = turn.AttachmentCount
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal user event payload: %w", err)
	}
	return string(raw), nil
}

func mustUserPayloadJSON(text string) string {
	s, err := buildUserEventPayload(FirstTurnContent{Text: text})
	if err != nil {
		return `{"text":""}`
	}
	return s
}

func mustNotePayloadJSON(body string) string {
	raw, err := json.Marshal(map[string]string{"text": body})
	if err != nil {
		return `{"text":""}`
	}
	return string(raw)
}

func extractPayloadText(payload string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return ""
	}
	raw, ok := m["text"]
	if !ok {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return ""
	}
	return strings.TrimSpace(text)
}

func coalesceNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}
