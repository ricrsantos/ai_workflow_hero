package daemon

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
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

func TestDaemonRegisterTwoClientsViaSocket(t *testing.T) {
	bot := &fakeBot{}
	s := openTestStore(t)
	d := New(Options{
		Bot:        bot,
		Vault:      vault.NewMemory(),
		Store:      s,
		SocketPath: filepath.Join(t.TempDir(), "telegram.sock"),
		Now:        time.Now,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		UID:        1,
	})
	_ = d.vault.Store("123456789:AAHq4K8xZyW0cN1pL9mR2tU5vX7wQ3sB6dF8gH0jK1", "CHAT")
	d.setCreds("123456789:AAHq4K8xZyW0cN1pL9mR2tU5vX7wQ3sB6dF8gH0jK1", "CHAT")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)

	reg := func(abbrev string) string {
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
		return m.Address
	}

	a1 := reg("proj")
	a2 := reg("proj")
	if a1 != "proj" || a2 != "proj_2" {
		t.Fatalf("addresses %q %q", a1, a2)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not exit on context cancel")
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
