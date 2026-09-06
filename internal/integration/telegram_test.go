package integration_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/plugin"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/daemon"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/vault"
)

// integrationFakeBot records sent messages for deterministic daemon tests.
type integrationFakeBot struct {
	mu      sync.Mutex
	sent    []string
	updates chan daemon.Update
}

func (f *integrationFakeBot) GetUpdates(ctx context.Context, _ int64) ([]daemon.Update, error) {
	if f.updates == nil {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	select {
	case update := <-f.updates:
		return []daemon.Update{update}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *integrationFakeBot) SendMessage(_ context.Context, chatID, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, chatID+"::"+text)
	return nil
}

func (f *integrationFakeBot) sentTexts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sent...)
}

func (f *integrationFakeBot) push(update daemon.Update) {
	f.updates <- update
}

func (f *integrationFakeBot) waitSent(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, got := range f.sentTexts() {
			if got == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("bot did not send %q; got %v", want, f.sentTexts())
}

func registerIntegrationClient(t *testing.T, socketPath, abbrev string) (net.Conn, *ipc.Conn, ipc.Message) {
	t.Helper()
	raw, err := ipc.Dial(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	conn := ipc.NewConn(raw)
	if err := conn.Send(ipc.Message{Type: ipc.TypeRegister, Mode: ipc.ModeCycle, ProjectAbbrev: abbrev, ProjectDir: "/p", UID: 1234}); err != nil {
		t.Fatal(err)
	}
	_ = raw.SetReadDeadline(time.Now().Add(2 * time.Second))
	registered, err := conn.Recv()
	if err != nil {
		t.Fatal(err)
	}
	_ = raw.SetReadDeadline(time.Time{})
	if registered.Type != ipc.TypeRegistered {
		t.Fatalf("register response=%+v", registered)
	}
	return raw, conn, registered
}

func recvIntegrationMessage(t *testing.T, raw net.Conn, conn *ipc.Conn) ipc.Message {
	t.Helper()
	_ = raw.SetReadDeadline(time.Now().Add(2 * time.Second))
	defer raw.SetReadDeadline(time.Time{})
	msg, err := conn.Recv()
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func TestIntegration_TelegramPluginInstallWritesManifest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "hero-telegram-daemon")
	if err := os.WriteFile(src, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m, err := plugin.InstallTelegram(filepath.Join(dir, "plugins", "telegram"), src, "2.9.2", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "telegram" || m.Version != "2.9.2" || m.ProtocolVersion != ipc.ProtocolVersion {
		t.Fatalf("manifest=%+v", m)
	}
	if _, err := os.Stat(m.DaemonPath); err != nil {
		t.Fatalf("daemon binary not written: %v", err)
	}
}

func TestIntegration_TelegramDaemonEndToEnd(t *testing.T) {
	bot := &integrationFakeBot{updates: make(chan daemon.Update, 16)}
	store, err := daemon.OpenStore(filepath.Join(t.TempDir(), "daemon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	v := vault.NewMemory()
	if err := v.Store("123456789:AAHq4K8xZyW0cN1pL9mR2tU5vX7wQ3sB6dF8gH0jK1", ""); err != nil {
		t.Fatal(err)
	}

	socketPath := filepath.Join(t.TempDir(), "telegram.sock")
	d := daemon.New(daemon.Options{
		Bot:        bot,
		Vault:      v,
		Store:      store,
		SocketPath: socketPath,
		Now:        time.Now,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		UID:        1234,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)

	raw1, c1, registration1 := registerIntegrationClient(t, socketPath, "proj")
	defer raw1.Close()
	if registration1.Address != "proj" {
		t.Fatalf("first address=%q want proj", registration1.Address)
	}

	// Pair through the daemon IPC path; the fake bot supplies the matching
	// Telegram /start update without contacting an external service.
	if err := c1.Send(ipc.Message{Type: ipc.TypePairStart}); err != nil {
		t.Fatal(err)
	}
	pairing := recvIntegrationMessage(t, raw1, c1)
	if pairing.Type != ipc.TypeEvent || pairing.EventType != ipc.EventPairingProgress || pairing.EventData == "" {
		t.Fatalf("pairing event=%+v", pairing)
	}
	bot.push(daemon.Update{UpdateID: 1, ChatID: "CHAT", Text: "/start " + pairing.EventData})
	bot.waitSent(t, "CHAT::Paired successfully.")
	paired := recvIntegrationMessage(t, raw1, c1)
	if paired.Type != ipc.TypeEvent || paired.EventType != ipc.EventPairingSuccess {
		t.Fatalf("pairing success event=%+v", paired)
	}

	raw2, c2, registration2 := registerIntegrationClient(t, socketPath, "proj")
	defer raw2.Close()
	if registration2.Address != "proj_2" || !registration2.Paired {
		t.Fatalf("second registration=%+v", registration2)
	}

	// Addressed plain text reaches its live target.
	bot.push(daemon.Update{UpdateID: 2, ChatID: "CHAT", Text: "proj: hello"})
	live := recvIntegrationMessage(t, raw1, c1)
	if live.Type != ipc.TypeInbound || live.Text != "hello" || live.IsCommand {
		t.Fatalf("live inbound=%+v", live)
	}
	bot.push(daemon.Update{UpdateID: 3, ChatID: "CHAT", Text: "proj_2: /hero-status"})
	command := recvIntegrationMessage(t, raw2, c2)
	if command.Type != ipc.TypeInbound || command.Text != "/hero-status" || !command.IsCommand {
		t.Fatalf("command inbound=%+v", command)
	}

	// Disconnect proj, queue an addressed message, then reconnect and receive it.
	if err := c1.Send(ipc.Message{Type: ipc.TypeUnregister}); err != nil {
		t.Fatal(err)
	}
	bot.waitSent(t, "CHAT::proj: disconnected.")
	_ = raw1.Close()
	time.Sleep(30 * time.Millisecond)
	bot.push(daemon.Update{UpdateID: 4, ChatID: "CHAT", Text: "proj: offline"})
	time.Sleep(30 * time.Millisecond)
	raw3, c3, registration3 := registerIntegrationClient(t, socketPath, "proj")
	defer raw3.Close()
	if registration3.Address != "proj" {
		t.Fatalf("reconnected address=%q", registration3.Address)
	}
	queued := recvIntegrationMessage(t, raw3, c3)
	if queued.Type != ipc.TypeInbound || queued.Text != "offline" {
		t.Fatalf("queued inbound=%+v", queued)
	}
	if queued.InboundID != "" {
		if err := c3.Send(ipc.Message{Type: ipc.TypeAckDelivery, AckID: queued.InboundID}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c3.Send(ipc.Message{Type: ipc.TypeUnregister}); err != nil {
		t.Fatal(err)
	}

	// A second offline message is cancelled by the daemon-owned command and is
	// not delivered on the following reconnect.
	_ = raw3.Close()
	time.Sleep(30 * time.Millisecond)
	bot.push(daemon.Update{UpdateID: 5, ChatID: "CHAT", Text: "proj: cancel me"})
	bot.push(daemon.Update{UpdateID: 6, ChatID: "CHAT", Text: "proj: /telegram-cancel-pending"})
	bot.waitSent(t, "CHAT::Telegram: 1 pending message(s) cancelled for proj.")
	pending, err := store.PendingForAddress("proj")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("cancelled pending messages remain=%+v", pending)
	}

	// Only explicit lifecycle/outbound text crosses the daemon boundary; the
	// prefixed result verifies the notification path used by the TUI notifier.
	if err := c2.Send(ipc.Message{Type: ipc.TypeOutbound, OutboundText: "Stage finished: qa"}); err != nil {
		t.Fatal(err)
	}
	bot.waitSent(t, "CHAT::proj_2: Stage finished: qa")
	if strings.Contains(strings.Join(bot.sentTexts(), "\n"), "thinking") {
		t.Fatal("stream/thinking content must not be emitted as a notification")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not exit on cancel")
	}
}

func TestIntegration_OpenSpecChangeTelegramPersistence(t *testing.T) {
	dir := makeGitRepo(t)
	doInstall(t, dir, "2.9.2")
	svc, err := cycle.OpenService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	if _, err := svc.NewCycle("Telegram", "integration lock"); err != nil {
		t.Fatal(err)
	}

	if err := svc.SetOpenspecChange("telegram-integration"); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.OpenspecChange != "telegram-integration" {
		t.Fatalf("openspec_change=%q", st.OpenspecChange)
	}
	if err := svc.ClearOpenspecChange(); err != nil {
		t.Fatal(err)
	}
}
