package daemon

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/vault"
)

// fakeBot records sent messages and serves queued updates for deterministic tests.
type fakeBot struct {
	mu      sync.Mutex
	sent    []string
	updates []Update
	polls   int
}

func (f *fakeBot) GetUpdates(_ context.Context, _ int64) ([]Update, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.polls++
	u := f.updates
	f.updates = nil
	return u, nil
}

func (f *fakeBot) SendMessage(_ context.Context, chatID, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, chatID+"::"+text)
	return nil
}

func (f *fakeBot) queueUpdates(u ...Update) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, u...)
}

func (f *fakeBot) sentTexts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sent...)
}

func newTestDaemon(t *testing.T, bot BotAPI, store *Store) *Daemon {
	t.Helper()
	d := New(Options{
		Bot:        bot,
		Vault:      vault.NewMemory(),
		Store:      store,
		SocketPath: filepath.Join(t.TempDir(), "telegram.sock"),
		Now:        time.Now,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		UID:        1,
	})
	d.setCreds("123456789:AAHq4K8xZyW0cN1pL9mR2tU5vX7wQ3sB6dF8gH0jK1", "CHAT")
	return d
}

func TestProcessUpdateRoutesCommandToClient(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))

	out := make(chan ipc.Message, 16)
	_, addr := d.registry.register("/p", ipc.ModeCycle, "proj", out)
	if addr != "proj" {
		t.Fatalf("addr=%q", addr)
	}
	_ = d.store.MarkAddressKnown(addr, ipc.ModeCycle, "proj", time.Now())

	d.processUpdate(context.Background(), Update{UpdateID: 1, ChatID: "CHAT", Text: "proj: /hero-status"})

	select {
	case m := <-out:
		if m.Type != ipc.TypeInbound || !m.IsCommand || m.Text != "/hero-status" {
			t.Fatalf("unexpected inbound: %+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no inbound delivered")
	}
	if got := bot.sentTexts(); len(got) != 1 || got[0] != "CHAT::OK, Received." {
		t.Fatalf("confirmation=%v want OK, Received", got)
	}
}

func TestBroadcastUpdateRestartEvent(t *testing.T) {
	d := newTestDaemon(t, &fakeBot{}, openTestStore(t))
	out := make(chan ipc.Message, 1)
	d.registry.register("/p", ipc.ModeCycle, "proj", out)

	if got := d.broadcastEvent(ipc.EventUpdateRestart, ""); got != 1 {
		t.Fatalf("broadcast count=%d want 1", got)
	}
	select {
	case msg := <-out:
		if msg.Type != ipc.TypeEvent || msg.EventType != ipc.EventUpdateRestart {
			t.Fatalf("event=%+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("restart event was not broadcast")
	}
}

func TestDaemonUnknownIPCMessageReturnsError(t *testing.T) {
	d := newTestDaemon(t, &fakeBot{}, openTestStore(t))
	server, client := net.Pipe()
	done := make(chan struct{})
	go func() {
		d.handleConn(context.Background(), server)
		close(done)
	}()

	clientConn := ipc.NewConn(client)
	if err := clientConn.Send(ipc.Message{Type: "future_message"}); err != nil {
		t.Fatal(err)
	}
	response, err := clientConn.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != ipc.TypeError || !strings.Contains(response.ErrorText, "future_message") {
		t.Fatalf("response=%+v want an explicit unsupported-message error", response)
	}
	_ = client.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("daemon connection handler did not stop")
	}
}

func TestPIDFileRemovalDoesNotDeleteAReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", "telegram.pid")
	if err := writePIDFile(path, 101); err != nil {
		t.Fatal(err)
	}
	removePIDFile(path, 202)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("replacement pid file was removed: %v", err)
	}
	removePIDFile(path, 101)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("pid file still exists: %v", err)
	}
}

func TestProcessUpdateListSelectAndRouteSelectedInstance(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))
	first := make(chan ipc.Message, 1)
	second := make(chan ipc.Message, 1)
	_, _ = d.registry.register("/p1", ipc.ModeCycle, "alpha", first)
	_, _ = d.registry.register("/p2", ipc.ModeCycle, "beta", second)

	d.processUpdate(context.Background(), Update{UpdateID: 1, ChatID: "CHAT", Text: "/list"})
	d.processUpdate(context.Background(), Update{UpdateID: 2, ChatID: "CHAT", Text: "/select 2"})
	d.processUpdate(context.Background(), Update{UpdateID: 3, ChatID: "CHAT", Text: "hello"})

	select {
	case m := <-second:
		if m.Text != "hello" || m.IsCommand {
			t.Fatalf("selected inbound=%+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("selected instance did not receive input")
	}
	select {
	case m := <-first:
		t.Fatalf("unselected instance received input: %+v", m)
	default:
	}
	if got, err := d.store.SelectedAddress(); err != nil || got != "beta" {
		t.Fatalf("selected address=%q err=%v", got, err)
	}
	want := []string{
		"CHAT::Connected instances:\n1. alpha\n2. beta",
		"CHAT::Selected instance: beta.",
		"CHAT::OK, Received.",
	}
	if got := bot.sentTexts(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("replies=%v want %v", got, want)
	}
}

func TestProcessUpdateSelectedDisconnectedInstanceReturnsError(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))
	if err := d.store.SetSelectedAddress("proj"); err != nil {
		t.Fatal(err)
	}

	d.processUpdate(context.Background(), Update{UpdateID: 1, ChatID: "CHAT", Text: "hello"})

	want := "CHAT::Selected instance is disconnected. Send /list, then /select <number>."
	if got := bot.sentTexts(); len(got) != 1 || got[0] != want {
		t.Fatalf("replies=%v want %q", got, want)
	}
}

func TestProcessUpdateColonProseRoutesToSelectedInstance(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))
	out := make(chan ipc.Message, 2)
	_, addr := d.registry.register("/p", ipc.ModeCycle, "aiwkhero", out)
	if addr != "aiwkhero" {
		t.Fatalf("addr=%q", addr)
	}
	d.processUpdate(context.Background(), Update{UpdateID: 1, ChatID: "CHAT", Text: "/select 1"})

	prose := "Verifique porque está dando este erro na chamada do orchestration agente: aiwkhero: cursor agent execute failed: exit status 1 (Failed to claim persistent session for chat \"ses_f810a8dc9ffeO6nD2Xb69Dnunp\": Persistent-session chat ID must be a UUID)"
	d.processUpdate(context.Background(), Update{UpdateID: 2, ChatID: "CHAT", Text: prose})

	select {
	case m := <-out:
		if m.Text != prose || m.IsCommand {
			t.Fatalf("selected inbound=%+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("selected instance did not receive colon prose")
	}

	prefixedPayload := "cursor agent execute failed: exit status 1"
	d.processUpdate(context.Background(), Update{
		UpdateID: 3, ChatID: "CHAT", Text: "aiwkhero: " + prefixedPayload,
	})
	select {
	case m := <-out:
		if m.Text != prefixedPayload || m.IsCommand {
			t.Fatalf("addressed inbound=%+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("addressed instance did not receive payload")
	}

	want := []string{
		"CHAT::Selected instance: aiwkhero.",
		"CHAT::OK, Received.",
		"CHAT::OK, Received.",
	}
	if got := bot.sentTexts(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("replies=%v want %v", got, want)
	}
}

func TestProcessUpdateHelpWithoutSelection(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))

	d.processUpdate(context.Background(), Update{UpdateID: 1, ChatID: "CHAT", Text: "/help"})

	got := bot.sentTexts()
	if len(got) != 1 {
		t.Fatalf("replies=%v", got)
	}
	if !strings.Contains(got[0], "CHAT::Hero Telegram commands") {
		t.Fatalf("help reply=%q", got[0])
	}
	if !strings.Contains(got[0], "/interrupt") || !strings.Contains(got[0], "/kill") {
		t.Fatalf("help missing control commands: %q", got[0])
	}
}

func TestProcessUpdateHelpDoesNotForwardToSelectedInstance(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))
	out := make(chan ipc.Message, 1)
	_, _ = d.registry.register("/p", ipc.ModeCycle, "proj", out)
	if err := d.store.SetSelectedAddress("proj"); err != nil {
		t.Fatal(err)
	}

	d.processUpdate(context.Background(), Update{UpdateID: 1, ChatID: "CHAT", Text: "/help"})
	d.processUpdate(context.Background(), Update{UpdateID: 2, ChatID: "CHAT", Text: "proj: /help"})

	select {
	case m := <-out:
		t.Fatalf("help must not reach TUI: %+v", m)
	default:
	}
	got := bot.sentTexts()
	if len(got) != 2 {
		t.Fatalf("replies=%v", got)
	}
	for i, reply := range got {
		if !strings.Contains(reply, "Hero Telegram commands") {
			t.Fatalf("reply[%d]=%q", i, reply)
		}
		if strings.Contains(reply, "OK, Received.") {
			t.Fatalf("help must not send delivery ack: %q", reply)
		}
	}
}

func TestProcessUpdateUnknownAddressRejectedGenerically(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))

	d.processUpdate(context.Background(), Update{UpdateID: 1, ChatID: "CHAT", Text: "unknown: hello"})

	for _, s := range bot.sentTexts() {
		if s == "CHAT::Unknown address. Prefix your message with a configured project address." {
			return
		}
	}
	t.Fatalf("expected generic reply, got %v", bot.sentTexts())
}

func TestProcessUpdateQueuesOfflineKnownTarget(t *testing.T) {
	bot := &fakeBot{}
	s := openTestStore(t)
	d := newTestDaemon(t, bot, s)
	_ = s.MarkAddressKnown("proj", ipc.ModeCycle, "proj", time.Now())

	d.processUpdate(context.Background(), Update{UpdateID: 7, ChatID: "CHAT", Text: "proj: hello"})

	rows, err := s.PendingForAddress("proj")
	if err != nil || len(rows) != 1 {
		t.Fatalf("pending rows=%d err=%v", len(rows), err)
	}
	if rows[0].Text != "hello" || rows[0].UpdateID != 7 {
		t.Fatalf("unexpected pending: %+v", rows[0])
	}
}

func TestProcessUpdateDedupIgnoresDuplicate(t *testing.T) {
	bot := &fakeBot{}
	s := openTestStore(t)
	d := newTestDaemon(t, bot, s)
	out := make(chan ipc.Message, 16)
	_, _ = d.registry.register("/p", ipc.ModeCycle, "proj", out)

	u := Update{UpdateID: 9, ChatID: "CHAT", Text: "proj: once"}
	d.processUpdate(context.Background(), u)
	d.processUpdate(context.Background(), u) // duplicate

	select {
	case <-out:
		// one delivery
	default:
		t.Fatal("first delivery missing")
	}
	select {
	case m := <-out:
		t.Fatalf("duplicate delivered again: %+v", m)
	case <-time.After(200 * time.Millisecond):
		// good: no duplicate
	}
}

func TestProcessUpdateCancelPending(t *testing.T) {
	bot := &fakeBot{}
	s := openTestStore(t)
	d := newTestDaemon(t, bot, s)
	_ = s.MarkAddressKnown("proj", ipc.ModeCycle, "proj", time.Now())
	_ = s.EnqueuePending(PendingMessage{Address: "proj", Text: "a", CreatedAt: time.Now()})
	_ = s.EnqueuePending(PendingMessage{Address: "proj", Text: "b", CreatedAt: time.Now()})

	d.processUpdate(context.Background(), Update{UpdateID: 10, ChatID: "CHAT", Text: "proj: /telegram-cancel-pending"})

	rows, _ := s.PendingForAddress("proj")
	if len(rows) != 0 {
		t.Fatalf("expected all pending cancelled, got %d", len(rows))
	}
	found := false
	for _, s := range bot.sentTexts() {
		if s == "CHAT::Telegram: 2 pending message(s) cancelled for proj." {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cancel confirmation, got %v", bot.sentTexts())
	}
}

func TestUnregisterClientAnnouncesDisconnection(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))

	c, address := d.registry.register("/p", ipc.ModeCycle, "proj", make(chan ipc.Message, 1))
	d.unregisterClient(context.Background(), c)

	if address != "proj" {
		t.Fatalf("address=%q want proj", address)
	}
	if got := d.registry.count(); got != 0 {
		t.Fatalf("live clients=%d want 0", got)
	}
	if got := bot.sentTexts(); fmt.Sprint(got) != "[CHAT::proj: disconnected.]" {
		t.Fatalf("notifications=%v want disconnected announcement", got)
	}
}

func TestDaemonConnectionDropAnnouncesDisconnection(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("daemon did not stop")
		}
	})

	deadline := time.Now().Add(2 * time.Second)
	var raw net.Conn
	for time.Now().Before(deadline) {
		conn, err := ipc.Dial(d.socketPath)
		if err == nil {
			raw = conn
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if raw == nil {
		t.Fatal("daemon socket did not become available")
	}
	conn := ipc.NewConn(raw)
	if err := conn.Send(ipc.Message{Type: ipc.TypeRegister, Mode: ipc.ModeCycle, ProjectAbbrev: "proj", ProjectDir: "/p", UID: 1}); err != nil {
		t.Fatal(err)
	}
	if registered, err := conn.Recv(); err != nil || registered.Type != ipc.TypeRegistered {
		t.Fatalf("registration=%+v err=%v", registered, err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := bot.sentTexts(); fmt.Sprint(got) == "[CHAT::proj: registered CHAT::proj: disconnected.]" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("notifications=%v want registration and disconnection", bot.sentTexts())
}

func TestDaemonRegisterTwoClientsViaSocket(t *testing.T) {
	bot := &fakeBot{}
	s := openTestStore(t)
	runDir := t.TempDir()
	pidPath := filepath.Join(runDir, "telegram.pid")
	d := New(Options{
		Bot:        bot,
		Vault:      vault.NewMemory(),
		Store:      s,
		SocketPath: filepath.Join(t.TempDir(), "telegram.sock"),
		PIDPath:    pidPath,
		Now:        time.Now,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		UID:        1,
		Version:    "3.1.1-test",
	})
	_ = d.vault.Store("123456789:AAHq4K8xZyW0cN1pL9mR2tU5vX7wQ3sB6dF8gH0jK1", "CHAT")
	d.setCreds("123456789:AAHq4K8xZyW0cN1pL9mR2tU5vX7wQ3sB6dF8gH0jK1", "CHAT")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	pidDeadline := time.Now().Add(time.Second)
	for time.Now().Before(pidDeadline) {
		data, err := os.ReadFile(pidPath)
		if err == nil && strings.TrimSpace(string(data)) == strconv.Itoa(os.Getpid()) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	data, err := os.ReadFile(pidPath)
	if err != nil || strings.TrimSpace(string(data)) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("pid file=%q err=%v want current process", data, err)
	}

	reg := func(abbrev string) ipc.Message {
		t.Helper()
		conn, err := ipc.Dial(d.socketPath)
		if err != nil {
			t.Fatal(err)
		}
		c := ipc.NewConn(conn)
		if err := c.Send(ipc.Message{Type: ipc.TypeRegister, Mode: ipc.ModeCycle, ProjectAbbrev: abbrev, UID: 1, ProjectDir: "/p"}); err != nil {
			t.Fatal(err)
		}
		m, err := c.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if m.Type != ipc.TypeRegistered {
			t.Fatalf("type=%q", m.Type)
		}
		return m
	}

	m1 := reg("proj")
	m2 := reg("proj")
	if m1.Address != "proj" || m2.Address != "proj_2" {
		t.Fatalf("addresses %q %q", m1.Address, m2.Address)
	}
	if m1.DaemonVersion != "3.1.1-test" || !m1.HasCapability(ipc.CapabilityUpdateRestart) {
		t.Fatalf("registration metadata=%+v", m1)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not exit on context cancel")
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("pid file still exists after daemon exit: %v", err)
	}
}

func TestPollLoopSkipsWhenNoClients(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	time.Sleep(300 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not exit")
	}
	bot.mu.Lock()
	polls := bot.polls
	bot.mu.Unlock()
	if polls != 0 {
		t.Fatalf("idle daemon polled Bot API %d times", polls)
	}
}

func TestProcessUpdateProjectControlRejectedOnFreeChat(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))
	out := make(chan ipc.Message, 1)
	_, addr := d.registry.register("", ipc.ModeFree, "", out)
	if err := d.store.SetSelectedAddress(addr); err != nil {
		t.Fatal(err)
	}

	d.processUpdate(context.Background(), Update{UpdateID: 1, ChatID: "CHAT", Text: "/hero-add-todo find-qa-1"})

	select {
	case m := <-out:
		t.Fatalf("free chat must not receive project control: %+v", m)
	default:
	}
	got := bot.sentTexts()
	if len(got) != 1 || !strings.Contains(got[0], "project instance") {
		t.Fatalf("replies=%v", got)
	}
}

func TestProcessUpdateProjectControlForwardsToProjectTUI(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))
	out := make(chan ipc.Message, 1)
	_, addr := d.registry.register("/p", ipc.ModeCycle, "proj", out)
	if err := d.store.SetSelectedAddress(addr); err != nil {
		t.Fatal(err)
	}

	d.processUpdate(context.Background(), Update{UpdateID: 2, ChatID: "CHAT", Text: "/hero-complete-todo todo-1"})

	select {
	case m := <-out:
		if m.Text != "/hero-complete-todo todo-1" || !m.IsCommand {
			t.Fatalf("inbound=%+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("project TUI did not receive command")
	}
}

func TestProcessUpdateProjectControlAttachmentRejected(t *testing.T) {
	bot := &fakeBot{}
	d := newTestDaemon(t, bot, openTestStore(t))
	out := make(chan ipc.Message, 1)
	_, addr := d.registry.register("/p", ipc.ModeCycle, "proj", out)
	if err := d.store.SetSelectedAddress(addr); err != nil {
		t.Fatal(err)
	}

	d.processUpdate(context.Background(), Update{
		UpdateID:      3,
		ChatID:        "CHAT",
		Text:          "/hero-add-todo find-qa-1",
		HasAttachment: true,
	})

	select {
	case m := <-out:
		t.Fatalf("attachment command must not forward: %+v", m)
	default:
	}
	got := bot.sentTexts()
	if len(got) != 1 || !strings.Contains(got[0], "Attachments are not supported") {
		t.Fatalf("replies=%v", got)
	}
}
