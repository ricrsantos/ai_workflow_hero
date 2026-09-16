package tui

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/store"
)

// sessionPersistPipelines provide per-session FIFO ordering so a slow earlier
// drain cannot commit after a later stream or final-result drain (find-qa-28).
var sessionPersistPipelines sync.Map // map[string]*sessionPersistPipeline

type sessionPersistPipeline struct {
	mu      sync.Mutex
	queue   []sessionPersistWork
	running bool
}

type sessionPersistWork struct {
	fn     func() tea.Msg
	result chan tea.Msg
}

func sessionPersistPipelineFor(sessionID string) *sessionPersistPipeline {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = "_"
	}
	v, _ := sessionPersistPipelines.LoadOrStore(sessionID, &sessionPersistPipeline{})
	return v.(*sessionPersistPipeline)
}

func (p *sessionPersistPipeline) submit(fn func() tea.Msg) tea.Cmd {
	if p == nil {
		return func() tea.Msg { return fn() }
	}
	w := sessionPersistWork{fn: fn, result: make(chan tea.Msg, 1)}
	p.mu.Lock()
	p.queue = append(p.queue, w)
	if !p.running {
		p.running = true
		go p.loop()
	}
	p.mu.Unlock()
	return func() tea.Msg {
		return <-w.result
	}
}

func (p *sessionPersistPipeline) loop() {
	for {
		p.mu.Lock()
		if len(p.queue) == 0 {
			p.running = false
			p.mu.Unlock()
			return
		}
		w := p.queue[0]
		p.queue = p.queue[1:]
		p.mu.Unlock()
		w.result <- w.fn()
	}
}

func (p *sessionPersistPipeline) run(fn func() error) error {
	if p == nil {
		return fn()
	}
	errCh := make(chan error, 1)
	cmd := p.submit(func() tea.Msg {
		errCh <- fn()
		return nil
	})
	_ = cmd()
	return <-errCh
}

type heroSessionBoundMsg struct {
	executeID string
	sessionID string
}

type sessionPersistErrMsg struct {
	err           error
	sessionID     string
	events        []store.AppendSessionEventInput
	assets        []sessionAssetPersistItem
	bindings      []sessionBindingPersistItem
	nativeBinds   []sessionNativeBindPersistItem
	serialization bool
}

type sessionPersistOKMsg struct {
	sessionID     string
	leaseAcquired bool
}

type sessionLeaseReleaseResultMsg struct {
	sessionID string
	err       error
}

type sessionLeaseReleaseRetryMsg struct {
	sessionID string
}

// sessionTurnPersistError carries retryable first-turn / attachment failure state
// so partial durable sessions can be resumed without dropping the payload (find-qa-25).
type sessionTurnPersistError struct {
	err           error
	sessionID     string
	events        []store.AppendSessionEventInput
	assets        []sessionAssetPersistItem
	bindings      []sessionBindingPersistItem
	nativeBinds   []sessionNativeBindPersistItem
	serialization bool
}

func (e *sessionTurnPersistError) Error() string {
	if e == nil || e.err == nil {
		return "session turn persistence failed"
	}
	return e.err.Error()
}

func (e *sessionTurnPersistError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

type sessionBindingPersistItem struct {
	orchestration bool
	stage         string
	sessionID     string
	harnessID     string
}

type sessionNativeBindPersistItem struct {
	heroID    string
	harnessID string
	nativeID  string
	modelSlug string
	propsJSON string
}

type sessionAssetPersistItem struct {
	sessionID string
	asset     harness.Asset
}

type sessionRecoverPollTickMsg struct {
	sessionID string
	nativeID  string
	harnessID string
}

type sessionRecoverDismissedMsg struct{}

type sessionLeaseLostMsg struct {
	sessionID string
}

type sessionInterruptFinalizeDoneMsg struct{}

type sessionRecoverAttachDoneMsg struct {
	sessionID string
	err       error
	result    *harness.ExecutionResult
}

type chatTranscriptRestoreMsg struct {
	sessionID              string
	transcript             []convMessage
	assets                 []harness.Asset
	occupancy              map[string]int64
	nativeSessionID        string
	harnessID              string
	model                  string
	title                  string
	lifecycle              string
	session                store.Session
	historicalContinuation bool
	err                    error
}

type restoredTranscriptEntry struct {
	message   convMessage
	eventType string
}

func newTuiOwnerID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "tui-owner-fallback"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (m model) heroChatSessionActive() bool {
	return m.sessionService != nil && strings.TrimSpace(m.heroChatSessionID) != ""
}

func (m *model) heroSessionIDForExecute(executeID string) string {
	executeID = strings.TrimSpace(executeID)
	if executeID != "" {
		if ex, ok := m.executes[executeID]; ok {
			if id := strings.TrimSpace(ex.HeroSessionID); id != "" {
				return id
			}
		}
	}
	return strings.TrimSpace(m.heroChatSessionID)
}

func (m model) persistTargetHeroID() string {
	if id := strings.TrimSpace(m.streamPersistHeroID); id != "" {
		return id
	}
	return strings.TrimSpace(m.heroChatSessionID)
}

func routeSessionPersistPayload(heroID string, events []store.AppendSessionEventInput, assets []sessionAssetPersistItem) {
	heroID = strings.TrimSpace(heroID)
	if heroID == "" {
		return
	}
	for i := range events {
		events[i].SessionID = heroID
		events[i].BoundSessionID = heroID
	}
	for i := range assets {
		assets[i].sessionID = heroID
	}
}

func (m *model) queueSessionPersist(in store.AppendSessionEventInput) {
	m.queueSessionPersistOn("", in)
}

func (m *model) queueSessionPersistOn(heroID string, in store.AppendSessionEventInput) {
	if m.sessionService == nil {
		return
	}
	heroID = strings.TrimSpace(heroID)
	if heroID == "" {
		heroID = strings.TrimSpace(in.SessionID)
	}
	if heroID == "" {
		heroID = strings.TrimSpace(in.BoundSessionID)
	}
	if heroID == "" {
		heroID = strings.TrimSpace(m.heroChatSessionID)
	}
	if heroID == "" {
		return
	}
	in.BoundSessionID = heroID
	in.SessionID = heroID
	m.sessionPersistQueue = append(m.sessionPersistQueue, in)
}

func (m *model) queueSessionBindingPersist(orchestration bool, stage, sessionID, harnessID string) {
	if m.svc == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	stage = strings.TrimSpace(stage)
	if sessionID == "" || harnessID == "" {
		return
	}
	if orchestration {
		m.sessionBindingQueue = append(m.sessionBindingQueue, sessionBindingPersistItem{
			orchestration: true,
			sessionID:     sessionID,
			harnessID:     harnessID,
		})
		return
	}
	if stage == "" {
		return
	}
	m.sessionBindingQueue = append(m.sessionBindingQueue, sessionBindingPersistItem{
		stage:     stage,
		sessionID: sessionID,
		harnessID: harnessID,
	})
}

func (m *model) queueSessionNativeBindPersist(heroID, harnessID, nativeID, modelSlug string, props map[string]string) {
	heroID = strings.TrimSpace(heroID)
	nativeID = strings.TrimSpace(nativeID)
	if m.sessionService == nil || heroID == "" || nativeID == "" {
		return
	}
	m.sessionNativeBindQueue = append(m.sessionNativeBindQueue, sessionNativeBindPersistItem{
		heroID:    heroID,
		harnessID: strings.TrimSpace(harnessID),
		nativeID:  nativeID,
		modelSlug: strings.TrimSpace(modelSlug),
		propsJSON: marshalChatSessionProps(props),
	})
}

func (m model) drainSessionPersistCmd() (model, tea.Cmd) {
	hasEvents := len(m.sessionPersistQueue) > 0
	hasAssets := len(m.sessionAssetUpsertQueue) > 0
	hasBindings := len(m.sessionBindingQueue) > 0
	hasNative := len(m.sessionNativeBindQueue) > 0
	if !hasEvents && !hasAssets && !hasBindings && !hasNative {
		return m, nil
	}
	if (hasEvents || hasAssets || hasNative) && m.sessionService == nil {
		return m, nil
	}
	if hasBindings && m.svc == nil && !hasEvents && !hasAssets && !hasNative {
		return m, nil
	}
	eventBatch := append([]store.AppendSessionEventInput(nil), m.sessionPersistQueue...)
	assetBatch := append([]sessionAssetPersistItem(nil), m.sessionAssetUpsertQueue...)
	bindingBatch := append([]sessionBindingPersistItem(nil), m.sessionBindingQueue...)
	nativeBatch := append([]sessionNativeBindPersistItem(nil), m.sessionNativeBindQueue...)
	leaseRetryID := strings.TrimSpace(m.sessionLeaseRetryID)
	leaseRetryOwner := strings.TrimSpace(m.tuiOwnerID)
	m.sessionPersistQueue = nil
	m.sessionAssetUpsertQueue = nil
	m.sessionBindingQueue = nil
	m.sessionNativeBindQueue = nil
	svc := m.sessionService
	cycleSvc := m.svc
	heroKey := persistBatchSessionID(eventBatch, assetBatch, nativeBatch)
	pipe := sessionPersistPipelineFor(heroKey)
	delay := m.testPersistDelay
	return m, pipe.submit(func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay)
		}
		msg := persistSessionSuffix(context.Background(), svc, cycleSvc, eventBatch, assetBatch, bindingBatch, nativeBatch)
		if leaseRetryID == "" {
			return msg
		}
		if errMsg, ok := msg.(sessionPersistErrMsg); ok {
			if strings.TrimSpace(errMsg.sessionID) == "" {
				errMsg.sessionID = leaseRetryID
			}
			return errMsg
		}
		if svc == nil {
			return sessionPersistErrMsg{
				err:         sessionPersistenceError("reacquire session lease", fmt.Errorf("session service is required")),
				sessionID:   leaseRetryID,
				events:      eventBatch,
				assets:      assetBatch,
				bindings:    bindingBatch,
				nativeBinds: nativeBatch,
			}
		}
		if _, err := svc.AcquireLease(context.Background(), leaseRetryID, leaseRetryOwner); err != nil {
			return sessionPersistErrMsg{
				err:         sessionPersistenceError("reacquire session lease", err),
				sessionID:   leaseRetryID,
				events:      eventBatch,
				assets:      assetBatch,
				bindings:    bindingBatch,
				nativeBinds: nativeBatch,
			}
		}
		return sessionPersistOKMsg{sessionID: leaseRetryID, leaseAcquired: true}
	})
}

func persistBatchSessionID(events []store.AppendSessionEventInput, assets []sessionAssetPersistItem, native []sessionNativeBindPersistItem) string {
	if len(events) > 0 {
		heroKey := strings.TrimSpace(events[0].SessionID)
		if heroKey == "" {
			heroKey = strings.TrimSpace(events[0].BoundSessionID)
		}
		if heroKey != "" {
			return heroKey
		}
	}
	if len(assets) > 0 {
		if heroKey := strings.TrimSpace(assets[0].sessionID); heroKey != "" {
			return heroKey
		}
	}
	if len(native) > 0 {
		return strings.TrimSpace(native[0].heroID)
	}
	return ""
}

func persistBatchSessionIDWithBindings(
	events []store.AppendSessionEventInput,
	assets []sessionAssetPersistItem,
	bindings []sessionBindingPersistItem,
	native []sessionNativeBindPersistItem,
) string {
	if id := persistBatchSessionID(events, assets, native); id != "" {
		return id
	}
	if len(bindings) > 0 {
		return strings.TrimSpace(bindings[0].sessionID)
	}
	return ""
}

func persistSessionSuffix(
	ctx context.Context,
	svc *conversation.SessionService,
	cycleSvc *cycle.Service,
	eventBatch []store.AppendSessionEventInput,
	assetBatch []sessionAssetPersistItem,
	bindingBatch []sessionBindingPersistItem,
	nativeBatch []sessionNativeBindPersistItem,
) tea.Msg {
	fail := func(err error, serialization bool, events []store.AppendSessionEventInput, assets []sessionAssetPersistItem, bindings []sessionBindingPersistItem, natives []sessionNativeBindPersistItem) tea.Msg {
		return sessionPersistErrMsg{
			err:           err,
			sessionID:     persistBatchSessionIDWithBindings(events, assets, bindings, natives),
			events:        events,
			assets:        assets,
			bindings:      bindings,
			nativeBinds:   natives,
			serialization: serialization,
		}
	}
	if _, err := storeAssetsFromPersistItems(assetBatch); err != nil {
		return fail(sessionPersistenceError("convert session asset batch", err), true, eventBatch, assetBatch, bindingBatch, nativeBatch)
	}
	if svc != nil {
		groups := groupPersistSuffix(eventBatch, assetBatch, nativeBatch)
		for i, group := range groups {
			var bind *store.NativeSessionBind
			if group.bind != nil {
				bind = &store.NativeSessionBind{
					SessionID:           group.bind.heroID,
					HarnessID:           group.bind.harnessID,
					NativeSessionID:     group.bind.nativeID,
					Model:               group.bind.modelSlug,
					ModelPropertiesJSON: group.bind.propsJSON,
				}
			}
			groupAssets, convErr := storeAssetsFromPersistItems(group.assets)
			if convErr != nil {
				return fail(sessionPersistenceError("convert session asset group", convErr), true, remainingPersistEvents(groups, i), remainingPersistAssets(groups, i), bindingBatch, remainingPersistNatives(groups, i))
			}
			if len(group.events) == 0 && len(groupAssets) == 0 && bind == nil {
				continue
			}
			if err := svc.PersistTranscriptSuffix(ctx, group.events, groupAssets, bind); err != nil {
				op := "persist transcript suffix"
				if bind != nil {
					op = "persist native session bind"
				}
				return fail(sessionPersistenceError(op, err), false, remainingPersistEvents(groups, i), remainingPersistAssets(groups, i), bindingBatch, remainingPersistNatives(groups, i))
			}
		}
	}
	for i, item := range bindingBatch {
		if cycleSvc == nil {
			continue
		}
		if item.orchestration {
			if err := cycleSvc.SetOrchestrationSession(item.sessionID, item.harnessID); err != nil {
				return fail(sessionPersistenceError("set orchestration session binding", err), false, nil, nil, append([]sessionBindingPersistItem(nil), bindingBatch[i:]...), nil)
			}
			continue
		}
		if err := cycleSvc.SetStageSessionBinding(item.stage, item.harnessID, item.sessionID); err != nil {
			return fail(sessionPersistenceError("set stage session binding", err), false, nil, nil, append([]sessionBindingPersistItem(nil), bindingBatch[i:]...), nil)
		}
	}
	return sessionPersistOKMsg{}
}

type persistSuffixGroup struct {
	sessionID string
	events    []store.AppendSessionEventInput
	assets    []sessionAssetPersistItem
	bind      *sessionNativeBindPersistItem
}

func persistEventSessionID(ev store.AppendSessionEventInput) string {
	id := strings.TrimSpace(ev.SessionID)
	if id == "" {
		id = strings.TrimSpace(ev.BoundSessionID)
	}
	return id
}

func groupPersistSuffix(events []store.AppendSessionEventInput, assets []sessionAssetPersistItem, natives []sessionNativeBindPersistItem) []persistSuffixGroup {
	usedEvents := make([]bool, len(events))
	usedAssets := make([]bool, len(assets))
	takeFor := func(sessionID string) ([]store.AppendSessionEventInput, []sessionAssetPersistItem) {
		sessionID = strings.TrimSpace(sessionID)
		var evs []store.AppendSessionEventInput
		var as []sessionAssetPersistItem
		if sessionID == "" {
			return evs, as
		}
		for i, ev := range events {
			if usedEvents[i] || persistEventSessionID(ev) != sessionID {
				continue
			}
			usedEvents[i] = true
			evs = append(evs, ev)
		}
		for i, asset := range assets {
			if usedAssets[i] || strings.TrimSpace(asset.sessionID) != sessionID {
				continue
			}
			usedAssets[i] = true
			as = append(as, asset)
		}
		return evs, as
	}

	groups := make([]persistSuffixGroup, 0, len(natives)+1)
	for i := range natives {
		item := natives[i]
		sid := strings.TrimSpace(item.heroID)
		evs, as := takeFor(sid)
		bind := item
		groups = append(groups, persistSuffixGroup{sessionID: sid, events: evs, assets: as, bind: &bind})
	}

	leftoverIdx := map[string]int{}
	for i, ev := range events {
		if usedEvents[i] {
			continue
		}
		sid := persistEventSessionID(ev)
		if idx, ok := leftoverIdx[sid]; ok {
			groups[idx].events = append(groups[idx].events, ev)
			continue
		}
		leftoverIdx[sid] = len(groups)
		groups = append(groups, persistSuffixGroup{sessionID: sid, events: []store.AppendSessionEventInput{ev}})
	}
	for i, asset := range assets {
		if usedAssets[i] {
			continue
		}
		sid := strings.TrimSpace(asset.sessionID)
		if idx, ok := leftoverIdx[sid]; ok {
			groups[idx].assets = append(groups[idx].assets, asset)
			continue
		}
		leftoverIdx[sid] = len(groups)
		groups = append(groups, persistSuffixGroup{sessionID: sid, assets: []sessionAssetPersistItem{asset}})
	}
	return groups
}

func remainingPersistEvents(groups []persistSuffixGroup, from int) []store.AppendSessionEventInput {
	var out []store.AppendSessionEventInput
	for _, group := range groups[from:] {
		out = append(out, group.events...)
	}
	return out
}

func remainingPersistAssets(groups []persistSuffixGroup, from int) []sessionAssetPersistItem {
	var out []sessionAssetPersistItem
	for _, group := range groups[from:] {
		out = append(out, group.assets...)
	}
	return out
}

func remainingPersistNatives(groups []persistSuffixGroup, from int) []sessionNativeBindPersistItem {
	var out []sessionNativeBindPersistItem
	for _, group := range groups[from:] {
		if group.bind != nil {
			out = append(out, *group.bind)
		}
	}
	return out
}

func storeAssetsFromPersistItems(items []sessionAssetPersistItem) ([]store.SessionAsset, error) {
	out := make([]store.SessionAsset, 0, len(items))
	for _, item := range items {
		heroID := strings.TrimSpace(item.sessionID)
		asset := item.asset
		cardMeta, err := json.Marshal(asset)
		if err != nil {
			return nil, err
		}
		assetID := strings.TrimSpace(asset.ContentHash)
		if assetID == "" {
			assetID = strings.TrimSpace(asset.ID)
		}
		if assetID == "" {
			continue
		}
		out = append(out, store.SessionAsset{
			SessionID:    heroID,
			AssetID:      assetID,
			Ownership:    store.AssetOwnershipManagedCopy,
			Path:         asset.Path,
			Mime:         asset.MIMEType,
			OriginalName: asset.Name,
			CardMetaJSON: string(cardMeta),
		})
	}
	return out, nil
}

func (m model) releaseHeroChatLeaseCmd(sessionID string) tea.Cmd {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || (m.sessionService == nil && m.leaseReleaseFn == nil) {
		return nil
	}
	owner := strings.TrimSpace(m.tuiOwnerID)
	svc := m.sessionService
	releaseFn := m.leaseReleaseFn
	epoch := m.leaseEpoch
	captured := uint64(0)
	if epoch != nil {
		captured = epoch.current(sessionID)
	}
	return func() tea.Msg {
		var err error
		if releaseFn != nil {
			err = releaseFn(context.Background(), sessionID, owner)
		} else if svc != nil {
			err = svc.ReleaseLease(context.Background(), sessionID, owner)
		}
		if err != nil {
			slog.Error("tui session lease release failed", "error", redact.Error(err))
			return sessionLeaseReleaseResultMsg{sessionID: sessionID, err: err}
		}
		if epoch != nil && epoch.current(sessionID) != captured && svc != nil {
			if _, acquireErr := svc.AcquireLease(context.Background(), sessionID, owner); acquireErr != nil {
				slog.Error("tui stale lease cleanup restore failed", "error", redact.Error(acquireErr))
				return sessionLeaseReleaseResultMsg{sessionID: sessionID, err: acquireErr}
			}
			slog.Info("tui restored lease after stale cleanup")
			return sessionLeaseReleaseResultMsg{sessionID: sessionID}
		}
		slog.Info("tui session lease released")
		return sessionLeaseReleaseResultMsg{sessionID: sessionID}
	}
}

const sessionLeaseReleaseRetryInterval = 500 * time.Millisecond

type leaseReleaseEpoch struct {
	mu  sync.Mutex
	gen map[string]uint64
}

func newLeaseReleaseEpoch() *leaseReleaseEpoch {
	return &leaseReleaseEpoch{gen: make(map[string]uint64)}
}

func (e *leaseReleaseEpoch) bump(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if e == nil || sessionID == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.gen == nil {
		e.gen = make(map[string]uint64)
	}
	e.gen[sessionID]++
}

func (e *leaseReleaseEpoch) current(sessionID string) uint64 {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.gen == nil {
		return 0
	}
	return e.gen[strings.TrimSpace(sessionID)]
}

func (m model) heroLeaseOwnedByCurrentChat(sessionID string) bool {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false
	}
	return strings.TrimSpace(m.heroLeasedSessionID) == sessionID || strings.TrimSpace(m.heroChatSessionID) == sessionID
}

func (m model) restoreHeroLeaseAfterStaleCleanup(sessionID string) tea.Cmd {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || m.sessionService == nil {
		return nil
	}
	owner := strings.TrimSpace(m.tuiOwnerID)
	svc := m.sessionService
	return func() tea.Msg {
		if _, err := svc.AcquireLease(context.Background(), sessionID, owner); err != nil {
			slog.Error("tui stale lease cleanup restore failed", "error", redact.Error(err))
			return sessionLeaseReleaseResultMsg{sessionID: sessionID, err: err}
		}
		slog.Info("tui restored lease after stale cleanup")
		return nil
	}
}

func (m *model) bumpLeaseReleaseEpoch(sessionID string) {
	if m.leaseEpoch == nil {
		m.leaseEpoch = newLeaseReleaseEpoch()
	}
	m.leaseEpoch.bump(sessionID)
}

func (m model) scheduleLeaseReleaseRetry(sessionID string) tea.Cmd {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	return tea.Tick(sessionLeaseReleaseRetryInterval, func(time.Time) tea.Msg {
		return sessionLeaseReleaseRetryMsg{sessionID: sessionID}
	})
}

func (m *model) trackPendingLeaseRelease(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	m.pendingLeaseReleaseID = sessionID
	for _, id := range m.pendingLeaseReleaseIDs {
		if id == sessionID {
			return
		}
	}
	m.pendingLeaseReleaseIDs = append(m.pendingLeaseReleaseIDs, sessionID)
}

func (m *model) clearPendingLeaseRelease(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	out := m.pendingLeaseReleaseIDs[:0]
	for _, id := range m.pendingLeaseReleaseIDs {
		if id != sessionID {
			out = append(out, id)
		}
	}
	m.pendingLeaseReleaseIDs = out
	if strings.TrimSpace(m.pendingLeaseReleaseID) == sessionID {
		m.pendingLeaseReleaseID = ""
		if len(m.pendingLeaseReleaseIDs) > 0 {
			m.pendingLeaseReleaseID = m.pendingLeaseReleaseIDs[0]
		}
	}
}

func (m model) leaseReleasePending(sessionID string) bool {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false
	}
	if strings.TrimSpace(m.pendingLeaseReleaseID) == sessionID {
		return true
	}
	for _, id := range m.pendingLeaseReleaseIDs {
		if id == sessionID {
			return true
		}
	}
	return false
}

func (m model) scheduleLeaseReleaseCleanups(sessionIDs []string) (model, tea.Cmd) {
	var cmds []tea.Cmd
	for _, id := range sessionIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		m.trackPendingLeaseRelease(id)
		if cmd := m.releaseHeroChatLeaseCmd(id); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func marshalChatSessionProps(props map[string]string) string {
	if len(props) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(props)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func (m model) chatSessionCreateMeta(ex convExecute, pairHarness, pairModel string, props map[string]string) conversation.CreateSessionParams {
	kind := store.SessionKindFreechat
	stage := strings.TrimSpace(ex.StageName)
	agent := strings.TrimSpace(ex.AgentName)
	var cycleID *int64
	cycleNumber := 0
	// Refresh-snapshotted cycle metadata only (no store I/O on Update/execute paths).
	if m.status.CycleNumber > 0 {
		cycleNumber = m.status.CycleNumber
	}
	if m.sessionTimer.cycleID > 0 {
		id := m.sessionTimer.cycleID
		cycleID = &id
		if cycleNumber == 0 && m.sessionTimer.cycleNumber > 0 {
			cycleNumber = m.sessionTimer.cycleNumber
		}
	}
	title := ""
	switch {
	case m.orchestrationLive || agent == agentOrchestration:
		kind = store.SessionKindOrchestration
		title = conversation.TitleOrchestration(cycleNumber)
	case m.researchLive || stage == stageResearch:
		kind = store.SessionKindResearch
		title = conversation.TitleResearch(cycleNumber)
	case stage != "" && agent != "" && agent != agentOrchestration:
		kind = store.SessionKindStageAgent
		title = conversation.TitleStageAgent(cycleNumber, stage, agent)
	default:
		// Leave empty so EnsureFirstTurn derives TitleFreeChat from the first turn text.
		title = ""
	}
	origin := store.SessionOriginLocal
	if addr, ok := telegramAddressOf(ex.Origin); ok {
		origin = store.SessionOriginTelegram
		_ = addr
	}
	return conversation.CreateSessionParams{
		Kind:                kind,
		Title:               title,
		HarnessID:           strings.TrimSpace(pairHarness),
		Model:               strings.TrimSpace(pairModel),
		ModelPropertiesJSON: marshalChatSessionProps(props),
		CycleID:             cycleID,
		StageName:           stage,
		AgentName:           agent,
		TranscriptState:     store.TranscriptAvailable,
		LastOrigin:          origin,
	}
}

func (m model) chatFirstTurnContent(userLabel string, labelRole convRole, ex convExecute) conversation.FirstTurnContent {
	text := strings.TrimSpace(userLabel)
	attachmentCount := len(ex.Attachments)
	turn := conversation.FirstTurnContent{
		Text:            text,
		AttachmentCount: attachmentCount,
		Origin:          conversation.OriginLocal,
	}
	if addr, ok := telegramAddressOf(ex.Origin); ok {
		turn.Origin = conversation.OriginTelegram
		turn.OriginAddress = addr
	}
	if labelRole == convRoleSystem {
		turn.StageAgentPrompt = text
		turn.Text = ""
	}
	if text == "" && attachmentCount > 0 {
		turn.Text = ""
	}
	return turn
}

// sessionPersistenceError wraps a session persistence failure with the
// specific operation that failed, so a single boundary log line names the
// exact write that broke instead of a generic "persistence failed".
func sessionPersistenceError(op string, err error) error {
	if err == nil {
		return fmt.Errorf("session persistence failed: %s", op)
	}
	return fmt.Errorf("session persistence failed: %s: %w", op, err)
}

func attachmentAsset(att harness.Attachment) harness.Asset {
	asset := harness.Asset{
		Source:     harness.AssetSourceUser,
		Attachment: att,
	}
	if strings.TrimSpace(asset.ContentHash) == "" {
		asset.ContentHash = harness.ContentHash(asset)
	}
	return asset
}

func attachmentPersistBatch(heroID string, attachments []harness.Attachment) ([]store.AppendSessionEventInput, []sessionAssetPersistItem, error) {
	heroID = strings.TrimSpace(heroID)
	events := make([]store.AppendSessionEventInput, 0, len(attachments))
	assets := make([]sessionAssetPersistItem, 0, len(attachments))
	for _, att := range attachments {
		asset := attachmentAsset(att)
		raw, err := json.Marshal(asset)
		if err != nil {
			return nil, nil, err
		}
		assetID := strings.TrimSpace(asset.ContentHash)
		if assetID == "" {
			assetID = strings.TrimSpace(asset.ID)
		}
		events = append(events, store.AppendSessionEventInput{
			BoundSessionID:  heroID,
			SessionID:       heroID,
			EventType:       store.SessionEventAttachment,
			Origin:          store.SessionOriginLocal,
			PayloadJSON:     string(raw),
			ProviderEventID: "tui:attachment:" + assetID,
		})
		assets = append(assets, sessionAssetPersistItem{sessionID: heroID, asset: asset})
	}
	return events, assets, nil
}

func (m model) persistAcceptedAttachments(ctx context.Context, heroID string, attachments []harness.Attachment) error {
	heroID = strings.TrimSpace(heroID)
	if m.sessionService == nil || heroID == "" || len(attachments) == 0 {
		return nil
	}
	events, items, err := attachmentPersistBatch(heroID, attachments)
	if err != nil {
		return sessionPersistenceError("marshal attachment assets", err)
	}
	storeAssets, err := storeAssetsFromPersistItems(items)
	if err != nil {
		return sessionPersistenceError("convert attachment assets", err)
	}
	pipe := sessionPersistPipelineFor(heroID)
	if err := pipe.run(func() error {
		return m.sessionService.PersistTranscriptSuffix(ctx, events, storeAssets, nil)
	}); err != nil {
		return &sessionTurnPersistError{err: sessionPersistenceError("persist attachment transcript suffix", err), sessionID: heroID, events: events, assets: items}
	}
	return nil
}

func firstTurnUserEvent(heroID string, userLabel string, labelRole convRole, turn conversation.FirstTurnContent, providerEventID string) (store.AppendSessionEventInput, error) {
	payload, err := buildUserPersistPayload(userLabel, labelRole, turn.AttachmentCount)
	if err != nil {
		return store.AppendSessionEventInput{}, err
	}
	origin := store.SessionOriginLocal
	addr := ""
	if turn.Origin == conversation.OriginTelegram {
		origin = store.SessionOriginTelegram
		addr = turn.OriginAddress
	}
	heroID = strings.TrimSpace(heroID)
	return store.AppendSessionEventInput{
		BoundSessionID:  heroID,
		SessionID:       heroID,
		EventType:       store.SessionEventUser,
		Origin:          origin,
		OriginAddress:   addr,
		PayloadJSON:     payload,
		ProviderEventID: strings.TrimSpace(providerEventID),
	}, nil
}

func (m model) persistUserTurnBeforeExecute(
	ctx context.Context,
	ex convExecute,
	userLabel string,
	labelRole convRole,
	pairHarness, pairModel string,
	props map[string]string,
) (string, error) {
	if m.sessionService == nil {
		return strings.TrimSpace(m.heroChatSessionID), nil
	}
	persistCtx := context.WithoutCancel(ctx)
	turn := m.chatFirstTurnContent(userLabel, labelRole, ex)
	if !conversation.HasAcceptedTurn(turn) {
		return strings.TrimSpace(m.heroChatSessionID), nil
	}
	heroID := strings.TrimSpace(m.heroChatSessionID)
	meta := m.chatSessionCreateMeta(ex, pairHarness, pairModel, props)
	if meta.Title == "" {
		imageOnly := turn.AttachmentCount > 0 && strings.TrimSpace(turn.Text) == "" && strings.TrimSpace(turn.StageAgentPrompt) == ""
		meta.Title = conversation.TitleFreeChat(turn.Text, imageOnly)
	}
	providerID := ""
	if heroID == "" {
		providerID = "tui:first-user"
	}
	userEvent, err := firstTurnUserEvent(heroID, userLabel, labelRole, turn, providerID)
	if err != nil {
		return "", sessionPersistenceError("build first-turn user event", err)
	}
	attachEvents, attachItems, err := attachmentPersistBatch(heroID, ex.Attachments)
	if err != nil {
		return "", &sessionTurnPersistError{err: sessionPersistenceError("marshal first-turn attachment assets", err), sessionID: heroID, serialization: true}
	}
	storeAssets, err := storeAssetsFromPersistItems(attachItems)
	if err != nil {
		return "", &sessionTurnPersistError{err: sessionPersistenceError("convert first-turn attachment assets", err), sessionID: heroID, serialization: true, events: append([]store.AppendSessionEventInput{userEvent}, attachEvents...), assets: attachItems}
	}
	events := append([]store.AppendSessionEventInput{userEvent}, attachEvents...)
	if heroID == "" {
		if provisional := strings.TrimSpace(m.mediaSessionID); provisional != "" {
			meta.ID = provisional
		}
		sess, err := m.sessionService.CreateSessionWithTranscript(persistCtx, meta, events, storeAssets)
		if err != nil {
			return "", &sessionTurnPersistError{err: sessionPersistenceError("create session with transcript", err), events: events, assets: attachItems}
		}
		heroID = strings.TrimSpace(sess.ID)
		if heroID == "" {
			return "", nil
		}
		routeSessionPersistPayload(heroID, events, attachItems)
		slog.Info("tui hero chat session created")
		owner := strings.TrimSpace(m.tuiOwnerID)
		if _, err := m.sessionService.AcquireLease(persistCtx, heroID, owner); err != nil {
			return "", &sessionTurnPersistError{err: sessionPersistenceError("acquire first-turn session lease", err), sessionID: heroID, events: events, assets: attachItems}
		}
		return heroID, nil
	}
	pipe := sessionPersistPipelineFor(heroID)
	if err := pipe.run(func() error {
		return m.sessionService.PersistTranscriptSuffix(persistCtx, events, storeAssets, nil)
	}); err != nil {
		return "", &sessionTurnPersistError{err: sessionPersistenceError("persist transcript suffix", err), sessionID: heroID, events: events, assets: attachItems}
	}
	return heroID, nil
}

func buildUserPersistPayload(userLabel string, labelRole convRole, attachmentCount int) (string, error) {
	body := map[string]any{
		"text": strings.TrimSpace(userLabel),
	}
	if labelRole == convRoleSystem {
		body["label_role"] = "system"
	}
	if attachmentCount > 0 {
		body["attachment_count"] = attachmentCount
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (m *model) queuePersistForMessage(msg convMessage) error {
	return m.queuePersistForMessageOn("", msg)
}

func (m *model) queuePersistForMessageOn(heroID string, msg convMessage) error {
	eventType, ok := sessionEventTypeForRole(msg.role)
	if !ok {
		return nil
	}
	payload, err := marshalTranscriptPayload(msg)
	if err != nil {
		return err
	}
	origin, addr := sessionOriginFromConv(msg.origin)
	in := store.AppendSessionEventInput{
		EventType:       eventType,
		Origin:          origin,
		OriginAddress:   addr,
		PayloadJSON:     payload,
		ProviderEventID: strings.TrimSpace(msg.persistProviderID),
		ReplaceExisting: strings.TrimSpace(msg.persistProviderID) != "",
	}
	m.queueSessionPersistOn(heroID, in)
	return nil
}

func (m *model) queuePersistTranscriptIndex(heroID, executeID string, idx int) error {
	if idx < 0 || idx >= len(m.transcript) {
		return nil
	}
	if id := strings.TrimSpace(m.transcript[idx].persistProviderID); id == "" {
		key := strings.TrimSpace(executeID)
		if key == "" {
			key = strings.TrimSpace(heroID)
		}
		if key == "" {
			key = strings.TrimSpace(m.heroChatSessionID)
		}
		if key == "" {
			key = "local"
		}
		m.transcript[idx].persistProviderID = fmt.Sprintf("tui:turn:%s:%s:%d", key, m.transcript[idx].role, idx)
	}
	return m.queuePersistForMessageOn(heroID, m.transcript[idx])
}

func sessionEventTypeForRole(role convRole) (string, bool) {
	switch role {
	case convRoleUser, convRoleSystem:
		return store.SessionEventUser, true
	case convRoleAgent:
		return store.SessionEventAssistant, true
	case convRoleThinking:
		return store.SessionEventThinking, true
	case convRoleTool:
		return store.SessionEventTool, true
	case convRoleWarning:
		return store.SessionEventWarning, true
	case convRoleActivity:
		return store.SessionEventTool, true
	default:
		return "", false
	}
}

func marshalTranscriptPayload(msg convMessage) (string, error) {
	body := map[string]any{
		"text": msg.content,
	}
	if msg.agentName != "" {
		body["agent_name"] = msg.agentName
	}
	if msg.modelSlug != "" {
		body["model"] = msg.modelSlug
	}
	if msg.harnessID != "" {
		body["harness_id"] = msg.harnessID
	}
	if msg.callID != "" {
		body["call_id"] = msg.callID
	}
	if msg.interrupted {
		body["interrupted"] = true
	}
	if msg.failed {
		body["failed"] = true
	}
	if msg.role == convRoleSystem {
		body["label_role"] = "system"
	}
	if msg.occupancyKey != "" {
		body["occupancy_key"] = msg.occupancyKey
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func sessionOriginFromConv(origin string) (string, string) {
	if addr, ok := telegramAddressOf(origin); ok {
		return store.SessionOriginTelegram, addr
	}
	return store.SessionOriginLocal, ""
}

func assistantPersistOrigin(ex convExecute) (string, string) {
	if addr, ok := telegramAddressOf(ex.Origin); ok {
		return store.SessionOriginTelegram, addr
	}
	return store.SessionOriginLocal, ""
}

func (m *model) queueSessionInterruption() {
	m.queueSessionPersist(store.AppendSessionEventInput{
		EventType:   store.SessionEventInterruption,
		Origin:      store.SessionOriginLocal,
		PayloadJSON: `{"text":"interrupted"}`,
	})
}

func (m *model) queueSessionAsset(asset harness.Asset) error {
	return m.queueSessionAssetOn("", asset)
}

func (m *model) queueSessionAssetOn(heroID string, asset harness.Asset) error {
	payload, err := json.Marshal(asset)
	if err != nil {
		return err
	}
	m.queueSessionPersistOn(heroID, store.AppendSessionEventInput{
		EventType:   store.SessionEventAsset,
		Origin:      store.SessionOriginLocal,
		PayloadJSON: string(payload),
	})
	if m.sessionService == nil || m.sessionService.Store == nil {
		return nil
	}
	heroID = strings.TrimSpace(heroID)
	if heroID == "" {
		heroID = strings.TrimSpace(m.heroChatSessionID)
	}
	if heroID == "" {
		return nil
	}
	m.sessionAssetUpsertQueue = append(m.sessionAssetUpsertQueue, sessionAssetPersistItem{
		sessionID: heroID,
		asset:     asset,
	})
	return nil
}

func (m model) bindNativeHeroSessionCmd(harnessID, nativeID, modelSlug string, props map[string]string) tea.Cmd {
	heroID := strings.TrimSpace(m.heroChatSessionID)
	if heroID == "" || m.sessionService == nil || strings.TrimSpace(nativeID) == "" {
		return nil
	}
	pm := &m
	pm.queueSessionNativeBindPersist(heroID, harnessID, nativeID, modelSlug, props)
	_, cmd := pm.drainSessionPersistCmd()
	return cmd
}

func (m model) loadChatTranscriptCmd(sessionID string) tea.Cmd {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || m.sessionService == nil {
		return nil
	}
	svc := m.sessionService
	storeDB := svc.Store
	return func() tea.Msg {
		ctx := context.Background()
		events, err := loadChatTranscriptEvents(ctx, svc, sessionID)
		if err != nil {
			return chatTranscriptRestoreMsg{sessionID: sessionID, err: err}
		}
		assetRows, err := storeDB.ListSessionAssets(sessionID)
		if err != nil {
			return chatTranscriptRestoreMsg{sessionID: sessionID, err: err}
		}
		transcript, assets, occupancy := eventsToTranscript(events, assetRows)
		sess, err := svc.GetSession(ctx, sessionID)
		if err != nil {
			return chatTranscriptRestoreMsg{sessionID: sessionID, err: err}
		}
		historical := false
		if sess.Kind == store.SessionKindStageAgent && sess.CycleID.Valid && svc.Store != nil {
			if stage, stageErr := svc.Store.GetStage(sess.CycleID.Int64, sess.StageName); stageErr == nil {
				historical = stage.Status == store.StageCompleted
			}
		}
		return chatTranscriptRestoreMsg{
			sessionID:              sessionID,
			transcript:             transcript,
			assets:                 assets,
			occupancy:              occupancy,
			nativeSessionID:        sess.NativeSessionID,
			harnessID:              sess.HarnessID,
			model:                  sess.Model,
			title:                  sess.Title,
			lifecycle:              sess.Lifecycle,
			session:                sess,
			historicalContinuation: historical,
		}
	}
}

// loadChatTranscriptEvents reads bounded event pages newest-first and keeps
// walking backwards until the newest turn has its user boundary. A turn can
// contain hundreds of tool/thinking events, while its user and parent
// assistant rows are persisted near the beginning of the turn. Restoring only
// the newest page in that case produces a transcript made exclusively of
// details, losing the prompt, response, and occupancy metadata needed by Chat.
func loadChatTranscriptEvents(ctx context.Context, svc *conversation.SessionService, sessionID string) ([]store.SessionEvent, error) {
	var all []store.SessionEvent
	before := int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page, err := svc.ListEventsNewest(ctx, sessionID, before, store.DefaultSessionEventPage)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		all = append(page, all...)
		if containsSessionEventType(page, store.SessionEventUser) || len(page) < store.DefaultSessionEventPage {
			break
		}
		before = page[0].Seq
	}
	return all, nil
}

func containsSessionEventType(events []store.SessionEvent, eventType string) bool {
	for _, ev := range events {
		if ev.EventType == eventType {
			return true
		}
	}
	return false
}

func eventsToTranscript(events []store.SessionEvent, assetRows []store.SessionAsset) ([]convMessage, []harness.Asset, map[string]int64) {
	assetByHash := map[string]harness.Asset{}
	for _, row := range assetRows {
		var asset harness.Asset
		if err := json.Unmarshal([]byte(row.CardMetaJSON), &asset); err != nil || asset.ContentHash == "" {
			asset = harness.Asset{
				Attachment: harness.Attachment{
					Path:        row.Path,
					MIMEType:    row.Mime,
					Name:        row.OriginalName,
					ContentHash: row.AssetID,
				},
				SessionID: row.SessionID,
			}
		}
		if asset.Path == "" {
			asset.Path = row.Path
		}
		if asset.Name == "" {
			asset.Name = row.OriginalName
		}
		if asset.ContentHash == "" {
			asset.ContentHash = row.AssetID
		}
		assetByHash[asset.ContentHash] = asset
	}
	entries := make([]restoredTranscriptEntry, 0, len(events))
	occupancy := make(map[string]int64)
	var cardAssets []harness.Asset
	for _, ev := range events {
		msg, asset, ok := eventToConvMessage(ev, assetByHash)
		if !ok {
			continue
		}
		entries = append(entries, restoredTranscriptEntry{message: msg, eventType: ev.EventType})
		if asset != nil {
			cardAssets = append(cardAssets, *asset)
		}
	}
	entries = normalizeRestoredTranscript(entries)
	transcript := make([]convMessage, 0, len(entries))
	for _, entry := range entries {
		transcript = append(transcript, entry.message)
	}
	occupancy = restoredTranscriptOccupancy(entries)
	return transcript, cardAssets, occupancy
}

// normalizeRestoredTranscript restores the same per-turn layout used by the
// live Chat. The parent assistant row is kept after thinking/tool/activity
// rows, while its persisted payload is updated in-place as stream snapshots
// arrive. That replacement preserves the original seq, so raw seq order can
// otherwise put the parent response before details that were shown before it.
func normalizeRestoredTranscript(entries []restoredTranscriptEntry) []restoredTranscriptEntry {
	if len(entries) < 2 {
		return entries
	}

	out := make([]restoredTranscriptEntry, 0, len(entries))
	turnStart := 0
	for i := 1; i <= len(entries); i++ {
		if i < len(entries) && entries[i].eventType != store.SessionEventUser {
			continue
		}
		out = append(out, normalizeRestoredTranscriptTurn(entries[turnStart:i])...)
		turnStart = i
	}
	return out
}

func normalizeRestoredTranscriptTurn(turn []restoredTranscriptEntry) []restoredTranscriptEntry {
	if len(turn) < 2 {
		return turn
	}

	var parent []restoredTranscriptEntry
	for _, entry := range turn {
		if entry.eventType == store.SessionEventAssistant && strings.TrimSpace(entry.message.callID) == "" {
			parent = append(parent, entry)
		}
	}
	if len(parent) == 0 {
		return turn
	}

	leading := make([]restoredTranscriptEntry, 0, len(turn)-len(parent))
	trailing := make([]restoredTranscriptEntry, 0)
	for _, entry := range turn {
		if entry.eventType == store.SessionEventAssistant && strings.TrimSpace(entry.message.callID) == "" {
			continue
		}
		// Interruption markers are emitted after the parent row in the live
		// transcript, so keep them after the restored response. Asset cards
		// render with the turn's pre-response detail rows (thinking/tool/
		// activity) in the live view, so restore them before the parent.
		if entry.eventType == store.SessionEventInterruption {
			trailing = append(trailing, entry)
			continue
		}
		leading = append(leading, entry)
	}

	ordered := make([]restoredTranscriptEntry, 0, len(turn))
	ordered = append(ordered, leading...)
	ordered = append(ordered, parent...)
	ordered = append(ordered, trailing...)
	return ordered
}

// restoredTranscriptOccupancy reconstructs the best available window estimate
// from the selected transcript. Persisted assistant snapshots carry the
// occupancy key, while the corresponding user event may not (especially the
// first turn), so associate an unkeyed user row with its turn's parent key.
// This mirrors estimateTranscriptOccupancy without treating billed totals as
// context-window usage.
func restoredTranscriptOccupancy(entries []restoredTranscriptEntry) map[string]int64 {
	textByKey := make(map[string]*strings.Builder)
	appendTurn := func(turn []restoredTranscriptEntry) {
		turnKey := ""
		for _, entry := range turn {
			if entry.message.role != convRoleAgent {
				continue
			}
			if key := strings.TrimSpace(entry.message.occupancyKey); key != "" {
				turnKey = key
			}
		}
		for _, entry := range turn {
			msg := entry.message
			if msg.role != convRoleUser && msg.role != convRoleAgent {
				continue
			}
			key := strings.TrimSpace(msg.occupancyKey)
			if key == "" && msg.role == convRoleUser {
				key = turnKey
				if key == "" {
					key = occupancyKeyFreechat
				}
			}
			if key == "" || msg.content == "" {
				continue
			}
			if textByKey[key] == nil {
				textByKey[key] = &strings.Builder{}
			}
			textByKey[key].WriteString(msg.content)
		}
	}

	turnStart := 0
	for i := 1; i <= len(entries); i++ {
		if i < len(entries) && entries[i].eventType != store.SessionEventUser {
			continue
		}
		appendTurn(entries[turnStart:i])
		turnStart = i
	}

	occupancy := make(map[string]int64, len(textByKey))
	for key, text := range textByKey {
		if occ := harness.EstimateUsage(text.String(), "").Occupancy(); occ > 0 {
			occupancy[key] = occ
		}
	}
	return occupancy
}

func eventToConvMessage(ev store.SessionEvent, assetByHash map[string]harness.Asset) (convMessage, *harness.Asset, bool) {
	switch ev.EventType {
	case store.SessionEventInterruption:
		return convMessage{role: convRoleAgent, content: "", interrupted: true}, nil, true
	case store.SessionEventAttachment:
		asset, ok := restoreImportedAttachmentAsset(ev.PayloadJSON)
		if !ok {
			return convMessage{}, nil, false
		}
		if asset.ContentHash != "" {
			if merged, ok := assetByHash[asset.ContentHash]; ok {
				asset = merged
			}
		}
		return convMessage{role: convRoleAgent, content: "", assets: []harness.Asset{asset}}, &asset, true
	case store.SessionEventAsset:
		var asset harness.Asset
		if err := json.Unmarshal([]byte(ev.PayloadJSON), &asset); err != nil {
			return convMessage{}, nil, false
		}
		if asset.ContentHash != "" {
			if merged, ok := assetByHash[asset.ContentHash]; ok {
				asset = merged
			}
		}
		return convMessage{role: convRoleAgent, content: "", assets: []harness.Asset{asset}}, &asset, true
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err != nil {
		return convMessage{}, nil, false
	}
	text := jsonStringField(payload, "text")
	role := convRoleAgent
	switch ev.EventType {
	case store.SessionEventUser:
		if jsonStringField(payload, "label_role") == "system" {
			role = convRoleSystem
		} else {
			role = convRoleUser
		}
	case store.SessionEventAssistant:
		role = convRoleAgent
	case store.SessionEventThinking:
		role = convRoleThinking
	case store.SessionEventTool:
		role = convRoleTool
	case store.SessionEventWarning, store.SessionEventPermission, store.SessionEventQuestion:
		role = convRoleWarning
	case store.SessionEventNote:
		role = convRoleSystem
	default:
		role = convRoleActivity
	}
	origin := ""
	if ev.Origin == store.SessionOriginTelegram {
		addr := strings.TrimSpace(ev.OriginAddress)
		if addr != "" {
			origin = "telegram:" + addr
		}
	}
	msg := convMessage{
		role:         role,
		content:      text,
		agentName:    jsonStringField(payload, "agent_name"),
		modelSlug:    jsonStringField(payload, "model"),
		harnessID:    jsonStringField(payload, "harness_id"),
		callID:       jsonStringField(payload, "call_id"),
		origin:       origin,
		occupancyKey: jsonStringField(payload, "occupancy_key"),
		interrupted:  jsonBoolField(payload, "interrupted"),
		failed:       jsonBoolField(payload, "failed"),
	}
	return msg, nil, true
}

func restoreImportedAttachmentAsset(payloadJSON string) (harness.Asset, bool) {
	raw := strings.TrimSpace(payloadJSON)
	if raw == "" {
		return harness.Asset{}, false
	}
	var asset harness.Asset
	if err := json.Unmarshal([]byte(raw), &asset); err == nil {
		if harness.ContentHash(asset) != "" || strings.TrimSpace(asset.Name) != "" || strings.TrimSpace(asset.Path) != "" {
			if asset.ContentHash == "" {
				asset.ContentHash = harness.ContentHash(asset)
			}
			return asset, true
		}
	}
	var part map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &part); err != nil {
		return harness.Asset{}, false
	}
	name := jsonStringField(part, "name", "filename", "fileName", "file_name")
	path := jsonStringField(part, "path", "filepath", "filePath", "file_path", "savedPath", "saved_path", "file")
	if path == "" {
		path = jsonStringField(part, "url", "uri", "href", "src")
	}
	mime := jsonStringField(part, "mime", "mimeType", "mime_type", "mediaType", "contentType", "content_type")
	id := jsonStringField(part, "id", "assetID", "assetId")
	if name == "" && path != "" {
		name = filepath.Base(path)
	}
	if name == "" && path == "" {
		return harness.Asset{}, false
	}
	contentHash := harness.HashBytes([]byte(raw))
	return harness.Asset{
		Source: harness.AssetSourceModel,
		Attachment: harness.Attachment{
			ID:          id,
			Name:        name,
			Path:        path,
			MIMEType:    mime,
			ContentHash: contentHash,
		},
	}, true
}

func jsonStringField(payload map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func jsonBoolField(payload map[string]json.RawMessage, key string) bool {
	raw, ok := payload[key]
	if !ok {
		return false
	}
	var b bool
	return json.Unmarshal(raw, &b) == nil && b
}

func restoredSessionOccupancyKey(sess store.Session, transcript []convMessage, occupancy map[string]int64) string {
	candidates := make([]string, 0, 2)
	switch sess.Kind {
	case store.SessionKindFreechat:
		candidates = append(candidates, occupancyKeyFreechat)
	case store.SessionKindResearch:
		stage := strings.TrimSpace(sess.StageName)
		if stage == "" {
			stage = stageResearch
		}
		agent := strings.TrimSpace(sess.AgentName)
		if agent == "" {
			agent = agentDiscover
		}
		candidates = append(candidates, cycleOccupancyKey(stage, agent))
	case store.SessionKindOrchestration:
		agent := strings.TrimSpace(sess.AgentName)
		if agent == "" {
			agent = agentOrchestration
		}
		candidates = append(candidates, cycleOccupancyKey(sess.StageName, agent))
	case store.SessionKindStageAgent:
		candidates = append(candidates, cycleOccupancyKey(sess.StageName, sess.AgentName))
	}
	for _, key := range candidates {
		if key != "" {
			if _, ok := occupancy[key]; ok {
				return key
			}
		}
	}
	for i := len(transcript) - 1; i >= 0; i-- {
		key := strings.TrimSpace(transcript[i].occupancyKey)
		if key == "" {
			continue
		}
		if _, ok := occupancy[key]; ok {
			return key
		}
	}
	if len(occupancy) == 1 {
		for key := range occupancy {
			return key
		}
	}
	return ""
}

func (m model) applyChatTranscriptRestore(msg chatTranscriptRestoreMsg) model {
	if msg.err != nil {
		m.convError = msg.err.Error()
		return m
	}
	m = m.applyHeroSessionBinding(msg.session)
	m.heroSessionHistorical = msg.historicalContinuation
	m.heroChatSessionID = msg.sessionID
	m.heroLeasedSessionID = msg.sessionID
	m.sessionLeaseLost = false
	pm := &m
	pm.syncMediaSessionFromHero()
	m = *pm
	m.heroLeaseNextHeartbeat = time.Now().Add(sessionLeaseHeartbeatInterval)
	m.transcript = append([]convMessage(nil), msg.transcript...)
	m.bumpTranscriptLayout()
	m.assets = harness.MergeAssetsByContentHash(nil, msg.assets)
	m.contextOccupancy = msg.occupancy
	if key := restoredSessionOccupancyKey(msg.session, m.transcript, msg.occupancy); key != "" {
		m = m.showOccupancyKey(key)
	}
	if slug := strings.TrimSpace(msg.model); slug != "" {
		m.chatModelSlug = slug
	}
	if sid := strings.TrimSpace(msg.nativeSessionID); sid != "" {
		switch msg.session.Kind {
		case store.SessionKindFreechat:
			m = m.persistFreechatSession(sid, msg.harnessID)
		default:
			m = m.persistHarnessSession(sid, msg.harnessID)
		}
	}
	m.transcriptFollowBottom = true
	// Restore opens the same way a live stream is rendered: follow the newest
	// transcript rows immediately instead of waiting for the next delta/resize
	// to calculate the offset.
	return m.maybeFollowTranscriptBottom()
}

func (m model) submitBlockedBySessionPersist() bool {
	return m.sessionPersistBlocked || m.sessionLeaseLost
}

func syncPersistExecuteResult(
	ctx context.Context,
	svc *conversation.SessionService,
	heroID string,
	ex convExecute,
	result *harness.ExecutionResult,
	harnessID, modelSlug string,
	props map[string]string,
) error {
	heroID = strings.TrimSpace(heroID)
	if svc == nil || heroID == "" {
		return nil
	}
	if result == nil {
		return nil
	}
	var events []store.AppendSessionEventInput
	var items []sessionAssetPersistItem
	text := strings.TrimSpace(result.Output)
	if text != "" {
		payload, err := marshalTranscriptPayload(convMessage{
			role:         convRoleAgent,
			content:      text,
			agentName:    ex.AgentName,
			modelSlug:    modelSlug,
			harnessID:    harnessID,
			occupancyKey: ex.OccupancyKey,
		})
		if err != nil {
			return sessionPersistenceError("marshal assistant transcript payload", err)
		}
		origin, addr := assistantPersistOrigin(ex)
		events = append(events, store.AppendSessionEventInput{
			BoundSessionID:  heroID,
			SessionID:       heroID,
			EventType:       store.SessionEventAssistant,
			Origin:          origin,
			OriginAddress:   addr,
			PayloadJSON:     payload,
			ProviderEventID: "tui:execute-result:assistant:" + heroID,
		})
	}
	for _, asset := range result.Assets {
		raw, err := json.Marshal(asset)
		if err != nil {
			return sessionPersistenceError("marshal execute result asset", err)
		}
		assetID := strings.TrimSpace(asset.ContentHash)
		if assetID == "" {
			assetID = strings.TrimSpace(asset.ID)
		}
		events = append(events, store.AppendSessionEventInput{
			BoundSessionID:  heroID,
			SessionID:       heroID,
			EventType:       store.SessionEventAsset,
			Origin:          store.SessionOriginLocal,
			PayloadJSON:     string(raw),
			ProviderEventID: "tui:execute-result:asset:" + assetID,
		})
		items = append(items, sessionAssetPersistItem{sessionID: heroID, asset: asset})
	}
	storeAssets, err := storeAssetsFromPersistItems(items)
	if err != nil {
		return sessionPersistenceError("convert execute result assets", err)
	}
	var bind *store.NativeSessionBind
	if sid := strings.TrimSpace(result.SessionID); sid != "" {
		bind = &store.NativeSessionBind{
			SessionID:           heroID,
			HarnessID:           harnessID,
			NativeSessionID:     sid,
			Model:               modelSlug,
			ModelPropertiesJSON: marshalChatSessionProps(props),
		}
	}
	pipe := sessionPersistPipelineFor(heroID)
	return pipe.run(func() error {
		if err := svc.PersistTranscriptSuffix(ctx, events, storeAssets, bind); err != nil {
			return sessionPersistenceError("persist execute result transcript", err)
		}
		return nil
	})
}
