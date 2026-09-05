package tui

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/plugin"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

// telegramState is the TUI-side state for the optional Telegram plugin
// (telegram-tui R1–R4; ADR-059/060). It is a pointer-valued field on the model
// so the engine Notifier adapter installed at boot observes later connection
// changes. It never holds a token or chat id (ADR-062).
type telegramState struct {
	installed       bool
	pluginVersion   string
	protocolVersion int

	connected bool   // IPC connection is live and registered
	address   string // allocated instance address (e.g. "ai_workflow_2")
	paired    bool   // daemon reports an authorized chat is configured
	retrying  bool   // reconnecting after an unexpected drop
	daemonErr string // last non-secret connection error for Chat/Settings copy

	// Pairing modal state (telegram-tui R2). pairCode is the daemon-issued
	// single-use code; it is never a token or chat id. pairToken holds the bot
	// token only while it is being typed, is never rendered, and is cleared the
	// moment it is sent over the 0600 socket.
	pairing      bool
	pairState    string // "", "token", "waiting", "success", "expired"
	pairCode     string
	pairToken    string
	pairDeadline time.Time

	// abbrev is the editable project abbreviation (display-only live suffix is
	// in address).
	abbrev string

	client *telegramClient // nil when the plugin is not installed

	// recordOutbound captures conversation replies in tests before IPC send.
	recordOutbound func(string)
}

// telegramMsg is the base for all tea.Msg payloads delivered from the client
// goroutine into the Update loop.
type telegramMsg struct{}

type telegramRegisteredMsg struct {
	address string
	paired  bool
}

type telegramInboundMsg struct {
	inboundID string
	text      string
	isCommand bool
	address   string
}

type telegramEventMsg struct {
	eventType string
	data      string
}

type telegramDisconnectedMsg struct {
	err string
}

type telegramConnectedMsg struct{}

// telegramClient owns the daemon connection and runs its own goroutine. Writes
// are serialized under mu; the read loop is the single reader. Lifecycle
// (connect, register, reconnect with bounded backoff) runs entirely off the
// Bubble Tea Update loop (ADr-053; telegram-ipc R3).
type telegramClient struct {
	mu   sync.Mutex
	conn *ipc.Conn
	addr string

	projectDir    string
	mode          string
	abbrev        string
	pluginVersion string
	socketPath    string
	daemonPath    string

	msgCh chan<- tea.Msg
	quit  chan struct{}
	done  chan struct{}
}

// Send writes one frame to the daemon, returning an error when disconnected.
func (c *telegramClient) Send(m ipc.Message) error {
	if c == nil {
		return fmt.Errorf("telegram: not connected")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("telegram: not connected")
	}
	return c.conn.Send(m)
}

// setAbbrev updates the project abbreviation and forces a reconnect so the
// daemon allocates a fresh suffix (telegram-tui R1).
func (c *telegramClient) setAbbrev(a string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.abbrev = a
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()
}

// Close stops the client goroutine and releases the connection.
func (c *telegramClient) Close() {
	if c == nil {
		return
	}
	close(c.quit)
	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.mu.Unlock()
	<-c.done
}

const (
	telegramInitialBackoff = time.Second
	telegramMaxBackoff     = 30 * time.Second
)

func (c *telegramClient) run() {
	defer close(c.done)
	backoff := telegramInitialBackoff
	var lastSpawn time.Time
	for {
		err := c.connectOnce()
		select {
		case <-c.quit:
			return
		default:
		}
		if err == nil {
			backoff = telegramInitialBackoff
			select {
			case <-c.quit:
				return
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}

		c.emit(telegramDisconnectedMsg{err: err.Error()})

		if c.daemonPath != "" && (lastSpawn.IsZero() || time.Since(lastSpawn) >= 2*time.Second) {
			_ = spawnDaemon(c.daemonPath, c.socketPath)
			lastSpawn = time.Now()
			select {
			case <-c.quit:
				return
			case <-time.After(150 * time.Millisecond):
			}
			continue
		}

		select {
		case <-c.quit:
			return
		case <-time.After(backoff):
		}
		if backoff < telegramMaxBackoff {
			backoff *= 2
		}
	}
}

// connectOnce dials the daemon socket, registers, and then relays inbound and
// event frames into the Update loop until the connection drops.
func (c *telegramClient) connectOnce() error {
	conn, err := ipc.Dial(c.socketPath)
	if err != nil {
		return err
	}
	pc := ipc.NewConn(conn)
	if err := pc.Send(ipc.Message{
		Type:          ipc.TypeRegister,
		ProjectDir:    c.projectDir,
		Mode:          c.mode,
		ProjectAbbrev: c.abbrev,
		PluginVersion: c.pluginVersion,
		UID:           ipc.CurrentUID(),
	}); err != nil {
		_ = pc.Close()
		return err
	}

	reg, err := pc.Recv()
	if err != nil {
		_ = pc.Close()
		return err
	}
	if reg.Type == ipc.TypeError {
		_ = pc.Close()
		return fmt.Errorf("daemon rejected registration: %s", reg.ErrorText)
	}
	if reg.Type != ipc.TypeRegistered {
		_ = pc.Close()
		return fmt.Errorf("unexpected daemon response %q", reg.Type)
	}

	c.mu.Lock()
	c.conn = pc
	c.addr = reg.Address
	c.mu.Unlock()

	c.emit(telegramConnectedMsg{})
	c.emit(telegramRegisteredMsg{address: reg.Address, paired: reg.Paired})

	// Relay daemon-pushed frames until the connection ends.
	for {
		m, err := pc.Recv()
		if err != nil {
			c.mu.Lock()
			c.conn = nil
			c.mu.Unlock()
			return err
		}
		switch m.Type {
		case ipc.TypeInbound:
			c.emit(telegramInboundMsg{
				inboundID: m.InboundID,
				text:      m.Text,
				isCommand: m.IsCommand,
				address:   c.address(),
			})
		case ipc.TypeEvent:
			c.emit(telegramEventMsg{eventType: m.EventType, data: m.EventData})
		case ipc.TypeError:
			c.emit(telegramDisconnectedMsg{err: m.ErrorText})
		}
	}
}

func (c *telegramClient) emit(msg tea.Msg) {
	if c == nil || c.msgCh == nil {
		return
	}
	select {
	case <-c.quit:
	case c.msgCh <- msg:
	}
}

func (c *telegramClient) address() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.addr
}

// spawnDaemon starts the installed daemon binary in its own session so the TUI
// TTY does not own it. If a daemon is already listening, this is a no-op so a
// second process cannot steal Bot API getUpdates (ADR-059).
func spawnDaemon(daemonPath, socketPath string) error {
	if daemonListening(socketPath) {
		slog.Debug("telegram daemon already listening")
		return nil
	}
	if strings.TrimSpace(daemonPath) == "" {
		return fmt.Errorf("telegram: daemon binary path is empty")
	}
	if _, err := os.Stat(daemonPath); err != nil {
		return fmt.Errorf("telegram: daemon binary: %w", err)
	}
	cmd := exec.Command(daemonPath)
	cmd.Dir = filepath.Dir(daemonPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	dn, dnErr := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if dnErr == nil {
		cmd.Stdin = dn
		cmd.Stdout = dn
		cmd.Stderr = dn
		defer dn.Close()
	}
	if err := cmd.Start(); err != nil {
		slog.Warn("telegram daemon spawn failed", "path", daemonPath, "error", err)
		return err
	}
	go func() { _ = cmd.Wait() }()
	slog.Info("telegram daemon spawned", "path", daemonPath)
	return nil
}

func daemonListening(socketPath string) bool {
	if strings.TrimSpace(socketPath) == "" {
		return false
	}
	conn, err := ipc.Dial(socketPath)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func spawnTelegramDaemonCmd(daemonPath, socketPath string) tea.Cmd {
	return func() tea.Msg {
		if err := spawnDaemon(daemonPath, socketPath); err != nil {
			return telegramDisconnectedMsg{err: err.Error()}
		}
		return nil
	}
}

// telegramPluginInstalled resolves whether the Telegram plugin is installed and
// returns its manifest metadata. It never reads secret values.
func telegramPluginInstalled(binaryVersion string) (installed bool, version string, protocol int, daemonPath string) {
	h, err := plugin.CheckTelegramHealth(binaryVersion)
	if err != nil || !h.Installed {
		return false, "", 0, ""
	}
	return true, h.Version, h.ProtocolVersion, h.DaemonPath
}

// startTelegram wires the optional Telegram client into the model. It is a
// no-op in test mode or when the plugin is not installed (hero-tui R1).
func (m model) startTelegram(version string) model {
	if m.testMode || m.svc == nil {
		return m
	}
	installed, pluginVersion, protocol, daemonPath := telegramPluginInstalled(version)
	if !installed {
		return m
	}

	projectDir := m.svc.ProjectDir
	socketPath, err := telegram.SocketPath("telegram")
	if err != nil {
		slog.Debug("telegram socket path failed", "error", err)
		return m
	}

	mode := ipc.ModeCycle
	if m.freeChatMode {
		mode = ipc.ModeFree
	}

	abbrev := loadTelegramAbbrev(projectDir)
	st := &telegramState{
		installed:       true,
		pluginVersion:   pluginVersion,
		protocolVersion: protocol,
		abbrev:          abbrev,
	}
	m.telegram = st

	// Install the engine Notifier adapter so cycle/stage/approval/error/final
	// events flow to the daemon outbound path (conversation-service R3;
	// telegram-tui R3). The adapter filters locally and sends nothing when the
	// client is disconnected or unpaired.
	if m.svc.Engine != nil {
		m.svc.Engine.Notifier = conversation.NotifyFunc(st.notify)
	}

	ch := make(chan tea.Msg, 256)
	m.telegramMsgCh = ch
	client := &telegramClient{
		projectDir:    projectDir,
		mode:          mode,
		abbrev:        abbrev,
		pluginVersion: pluginVersion,
		socketPath:    socketPath,
		daemonPath:    daemonPath,
		msgCh:         ch,
		quit:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	st.client = client
	go client.run()
	return m
}

// stopTelegram unregisters and closes the client (TUI shutdown).
func (m model) stopTelegram() {
	if m.telegram == nil || m.telegram.client == nil {
		return
	}
	_ = m.telegram.client.Send(ipc.Message{Type: ipc.TypeUnregister})
	m.telegram.client.Close()
	m.telegram.client = nil
	if m.telegramMsgCh != nil {
		close(m.telegramMsgCh)
		m.telegramMsgCh = nil
	}
}

// relayTelegramMsgs pushes daemon frames into the Bubble Tea program so inbound
// does not depend on re-issuing waitTelegramMsg after every Update.
func relayTelegramMsgs(p *tea.Program, ch <-chan tea.Msg) {
	if p == nil || ch == nil {
		return
	}
	go func() {
		for msg := range ch {
			p.Send(msg)
		}
	}()
}

// loadTelegramAbbrev returns the persisted Settings Project ID, or the
// directory basename when hero.json has no telegram.project_abbrev.
func loadTelegramAbbrev(projectDir string) string {
	fallback := projectAbbrev(projectDir)
	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		return fallback
	}
	if saved := strings.TrimSpace(hero.Telegram.ProjectAbbrev); saved != "" {
		return normalizeTelegramAbbrev(saved)
	}
	return fallback
}

// projectAbbrev derives a project abbreviation from the project directory base
// name, lowercased to [a-z0-9_-] (matching the daemon allocator normalization).
func projectAbbrev(projectDir string) string {
	base := filepath.Base(filepath.Clean(projectDir))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "proj"
	}
	return normalizeTelegramAbbrev(base)
}

func normalizeTelegramAbbrev(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "proj"
	}
	return out
}

// notify forwards a lifecycle event to the daemon outbound path. Stream,
// thinking, and tool events never arrive here (the engine only publishes
// cycle/stage/approval/error/final events — PRD-C09-001 §3.3).
func (s *telegramState) notify(e conversation.Event) {
	if s == nil || s.client == nil || !s.connected {
		return
	}
	text := formatTelegramEvent(e)
	if text == "" {
		return
	}
	if err := s.client.Send(ipc.Message{Type: ipc.TypeOutbound, OutboundText: text}); err != nil {
		slog.Debug("telegram outbound notify failed", "error", err)
	}
}

// formatTelegramEvent renders a lifecycle event as a single outbound line.
// Pending/expired/cancel notices are muted by the caller, not here.
func formatTelegramEvent(e conversation.Event) string {
	switch e.Kind {
	case conversation.EventCycleStarted:
		title := strings.TrimSpace(e.CycleTitle)
		if title == "" {
			title = fmt.Sprintf("C%d", e.CycleID)
		}
		return "Cycle started: " + title
	case conversation.EventCycleFinished:
		return "Cycle finished."
	case conversation.EventStageStarted:
		return "Stage started: " + e.StageName
	case conversation.EventStageFinished:
		msg := "Stage finished: " + e.StageName
		if s := strings.TrimSpace(e.Message); s != "" {
			msg += " — " + s
		}
		return msg
	case conversation.EventApprovalRequired:
		return "Approval required: " + e.StageName
	case conversation.EventError:
		msg := "Error"
		if e.StageName != "" {
			msg += " in " + e.StageName
		}
		if s := strings.TrimSpace(e.Message); s != "" {
			msg += ": " + s
		}
		return msg
	case conversation.EventFinalResult:
		return strings.TrimSpace(e.Message)
	default:
		return ""
	}
}

// telegramListenCmd is a test helper that reads one daemon frame from the
// client channel. Production uses relayTelegramMsgs → tea.Program.Send.
func (m model) telegramListenCmd() tea.Cmd {
	if m.telegramMsgCh == nil {
		return nil
	}
	return waitTelegramMsg(m.telegramMsgCh)
}

// waitTelegramMsg relays a single message from the client goroutine into the
// Update loop. It is re-issued after each handled message.
func waitTelegramMsg(ch chan tea.Msg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		m, ok := <-ch
		if !ok {
			return nil
		}
		return m
	}
}
