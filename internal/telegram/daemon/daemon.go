// Package daemon implements the local Telegram Bot API daemon (ADR-059–063).
// One daemon per OS user owns the Bot API connection, multiplexes all registered
// Hero TUIs over versioned local IPC, handles pairing, addressed routing, the
// durable pending queue, and outbound notifications. It contains no LLM
// reasoning and is fully injectable for tests (Bot API, vault, clock, store,
// IPC transport).
package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/vault"
)

// Options configures a Daemon. All fields are injectable for tests.
type Options struct {
	Bot   BotAPI
	Vault vault.Store
	Store *Store
	// BotFactory builds the Bot API from a token when one is stored after the
	// daemon has already started (e.g. the TUI setup flow). May be nil.
	BotFactory func(token string) BotAPI
	SocketPath string
	Now        func() time.Time
	Logger     *slog.Logger
	// UID is the effective UID clients must declare. Defaults to ipc.CurrentUID().
	UID int
}

// Daemon owns Bot API connectivity and IPC multiplexing for one OS user.
type Daemon struct {
	vault      vault.Store
	store      *Store
	botFactory func(token string) BotAPI
	socketPath string
	now        func() time.Time
	log        *slog.Logger
	uid        int

	registry *registry
	pairing  *pairingManager

	// mu guards the cached credentials (token + authorized chat id) and the
	// current Bot API client. They are resolved by the daemon only (ADR-062).
	mu     sync.Mutex
	token  string
	chatID string
	bot    BotAPI

	shutdownMu sync.Mutex
	shutdownFn func()
	exitTimer  *time.Timer
}

const disconnectNotificationTimeout = 2 * time.Second

// New builds a Daemon from opts, applying defaults for nil fields.
func New(opts Options) *Daemon {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.UID == 0 {
		opts.UID = ipc.CurrentUID()
	}
	return &Daemon{
		vault:      opts.Vault,
		store:      opts.Store,
		botFactory: opts.BotFactory,
		bot:        opts.Bot,
		socketPath: opts.SocketPath,
		now:        opts.Now,
		log:        opts.Logger,
		uid:        opts.UID,
		registry:   newRegistry(),
		pairing:    newPairingManager(opts.Now),
	}
}

// Run starts the daemon: it loads cached credentials, listens on the OS-user
// socket, long-polls the Bot API (when a token is present), and exits when the
// context is cancelled or the last client unregisters (ADR-059).
func (d *Daemon) Run(ctx context.Context) error {
	if d.vault == nil {
		return fmt.Errorf("daemon: vault is required")
	}
	d.log.Info("telegram daemon starting", "socket", d.socketPath)
	if e, err := d.vault.Load(); err == nil {
		d.setCreds(e.Token, e.ChatID)
		if d.bot == nil && d.botFactory != nil && e.Token != "" {
			d.setBot(d.botFactory(e.Token))
		}
	}

	ln, err := ipc.Listen(d.socketPath)
	if err != nil {
		return fmt.Errorf("daemon listen: %w", err)
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(d.socketPath)
	}()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	d.setShutdown(cancel)

	go d.pollLoop(runCtx)

	var wg sync.WaitGroup
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-runCtx.Done():
					return
				default:
					d.log.Error("daemon accept failed", "error", err)
					return
				}
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				d.handleConn(runCtx, conn)
			}()
		}
	}()

	<-runCtx.Done()
	_ = ln.Close()
	<-acceptDone
	wg.Wait()
	d.log.Info("telegram daemon stopped")
	return nil
}

func (d *Daemon) setShutdown(fn func()) {
	d.shutdownMu.Lock()
	d.shutdownFn = fn
	d.shutdownMu.Unlock()
}

func (d *Daemon) requestShutdown() {
	d.shutdownMu.Lock()
	defer d.shutdownMu.Unlock()
	if d.exitTimer != nil {
		d.exitTimer.Stop()
	}
	fn := d.shutdownFn
	d.exitTimer = time.AfterFunc(3*time.Second, func() {
		if d.registry.count() > 0 {
			return
		}
		d.log.Info("last client disconnected, daemon will exit")
		if fn != nil {
			fn()
		}
	})
}

func (d *Daemon) cancelScheduledShutdown() {
	d.shutdownMu.Lock()
	defer d.shutdownMu.Unlock()
	if d.exitTimer != nil {
		d.exitTimer.Stop()
		d.exitTimer = nil
	}
}

func (d *Daemon) setCreds(token, chatID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.token = token
	d.chatID = chatID
}

func (d *Daemon) setBot(bot BotAPI) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bot = bot
}

func (d *Daemon) creds() (token, chatID string, bot BotAPI) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.token, d.chatID, d.bot
}

// Paired reports whether an authorized chat is configured.
func (d *Daemon) Paired() bool {
	_, chatID, _ := d.creds()
	return chatID != ""
}

// send sends a Bot API message using the current client, if any.
func (d *Daemon) send(ctx context.Context, chatID, text string) {
	_, _, bot := d.creds()
	if bot == nil {
		return
	}
	_ = bot.SendMessage(ctx, chatID, text)
}

// unregisterClient removes a TUI from the live registry, reports its departure
// to the paired chat, and schedules daemon shutdown when it was the last one.
// The notification has its own short deadline so a Telegram outage cannot
// indefinitely delay the last-client shutdown.
func (d *Daemon) unregisterClient(ctx context.Context, c *client) {
	if c == nil || !d.registry.unregister(c) {
		return
	}

	notifyCtx, cancel := context.WithTimeout(ctx, disconnectNotificationTimeout)
	d.announceDisconnection(notifyCtx, c)
	cancel()

	if d.registry.count() == 0 {
		d.requestShutdown()
	}
}

// handleConn serves one TUI IPC connection until it disconnects.
func (d *Daemon) handleConn(ctx context.Context, conn net.Conn) {
	c := ipc.NewConn(conn)
	defer c.Close()

	var reg *client
	outbound := make(chan ipc.Message, 256)
	defer func() {
		d.unregisterClient(ctx, reg)
	}()

	// Reader goroutine forwards frames so writes never deadlock behind a read.
	inbound := make(chan ipc.Message, 64)
	readErr := make(chan error, 1)
	go func() {
		for {
			m, err := c.Recv()
			if err != nil {
				readErr <- err
				return
			}
			select {
			case inbound <- m:
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-readErr:
			d.log.Debug("client read ended", "error", err)
			return
		case m := <-inbound:
			if !m.VersionOK() {
				_ = c.Send(ipc.Message{Type: ipc.TypeError, ErrorText: "incompatible protocol version; upgrade or reinstall the plugin"})
				return
			}
			switch m.Type {
			case ipc.TypeRegister:
				if m.UID != 0 && d.uid != 0 && m.UID != d.uid {
					_ = c.Send(ipc.Message{Type: ipc.TypeError, ErrorText: "access denied: socket owner mismatch"})
					return
				}
				reg, _ = d.registry.register(m.ProjectDir, m.Mode, m.ProjectAbbrev, outbound)
				d.cancelScheduledShutdown()
				if d.store != nil {
					_ = d.store.MarkAddressKnown(reg.address, m.Mode, m.ProjectAbbrev, d.now())
				}
				d.log.Info("client registered", "address", reg.address, "mode", m.Mode)
				if err := c.Send(ipc.Message{Type: ipc.TypeRegistered, Address: reg.address, Paired: d.Paired()}); err != nil {
					return
				}
				d.announceRegistration(ctx, reg)
				d.flushPending(ctx, reg)
			case ipc.TypeUnregister:
				if reg != nil {
					d.unregisterClient(ctx, reg)
					reg = nil
				}
				return
			case ipc.TypeSetCredentials:
				d.handleSetCredentials(ctx, m.Token)
			case ipc.TypePairStart:
				d.handlePairStart(ctx)
			case ipc.TypePairCancel:
				d.pairing.cancel()
				d.broadcastEvent(ipc.EventPairingExpired, "")
			case ipc.TypeClear:
				d.handleClear()
			case ipc.TypeTest:
				d.handleTest(ctx)
			case ipc.TypeAckDelivery:
				d.handleAck(m.AckID)
			case ipc.TypeOutbound:
				if reg != nil {
					addr := reg.address
					text := m.OutboundText
					go d.sendOutbound(ctx, addr, text)
				}
			}
		case m := <-outbound:
			if err := c.Send(m); err != nil {
				return
			}
		}
	}
}
