package daemon

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

// prefixAddress prefixes an outbound notification with the instance address so
// the recipient can attribute it (PRD-C09-001 §3.2; UI-C09-001 §4).
func prefixAddress(address, text string) string {
	return fmt.Sprintf("%s: %s", address, text)
}

func (d *Daemon) sleep(ctx context.Context, dur time.Duration) {
	t := time.NewTimer(dur)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// pollLoop long-polls the Bot API and feeds updates into processUpdate. It only
// polls when a token and a live client are present so a shutting-down or
// duplicate daemon cannot steal getUpdates from the process that owns the TUI
// (ADR-059).
func (d *Daemon) pollLoop(ctx context.Context) {
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		token, _, bot := d.creds()
		if token == "" || bot == nil || d.registry.count() == 0 {
			d.sleep(ctx, time.Second)
			continue
		}
		updates, err := bot.GetUpdates(ctx, offset)
		if err != nil {
			d.log.Error("bot getUpdates failed", "error", redact.Redact(err.Error(), token))
			d.sleep(ctx, 2*time.Second)
			continue
		}
		var maxID int64
		for _, u := range updates {
			if u.UpdateID > maxID {
				maxID = u.UpdateID
			}
			d.processUpdate(ctx, u)
		}
		if maxID > 0 {
			offset = maxID + 1
		}
		d.sleep(ctx, 200*time.Millisecond)
	}
}

// processUpdate handles one Bot API update: idempotent dedup, pairing, and
// selection-based routing (ADR-063).
func (d *Daemon) processUpdate(ctx context.Context, u Update) {
	if u.UpdateID <= 0 {
		return
	}
	if d.store != nil {
		if processed, err := d.store.UpdateProcessed(u.UpdateID); err == nil && processed {
			return // duplicate redelivery is a no-op
		}
		_ = d.store.MarkUpdateProcessed(u.UpdateID, d.now())
	}
	if strings.TrimSpace(u.Text) == "" {
		return
	}
	text := strings.TrimSpace(stripBotCommand(u.Text))

	if d.pairing.active() != "" {
		d.processPairing(ctx, u, text)
		return
	}

	_, chatID, _ := d.creds()
	if chatID == "" {
		d.log.Debug("inbound ignored before pairing")
		return
	}
	if u.ChatID != chatID {
		d.log.Info("unauthorized inbound rejected")
		d.send(ctx, u.ChatID, "Unauthorized.")
		return
	}
	d.routeInbound(ctx, u, text)
}

// processPairing validates a message against the active pairing code and binds
// the authorized chat on success (PRD-C09-001 §3.4).
func (d *Daemon) processPairing(ctx context.Context, u Update, text string) {
	matched, expired, _ := d.pairing.validate(text)
	if expired {
		d.broadcastEvent(ipc.EventPairingExpired, "")
		d.send(ctx, u.ChatID, "Pairing code expired.")
		return
	}
	if !matched {
		d.log.Info("pairing code mismatch rejected")
		d.send(ctx, u.ChatID, "Invalid pairing code.")
		return
	}
	token, _, _ := d.creds()
	if err := d.vault.Store(token, u.ChatID); err != nil {
		d.log.Error("vault store failed during pairing", "error", err)
		d.send(ctx, u.ChatID, "Pairing failed: credential storage error.")
		return
	}
	d.pairing.consume()
	d.setCreds(token, u.ChatID)
	if d.store != nil {
		if err := d.store.ClearSelectedAddress(); err != nil {
			d.log.Error("clear selected instance after pairing failed", "error", err)
		}
	}
	d.broadcastEvent(ipc.EventPairingSuccess, "")
	d.send(ctx, u.ChatID, "Paired successfully.")
	for _, addr := range d.registry.addresses() {
		d.send(ctx, u.ChatID, prefixAddress(addr, "registered"))
	}
}

// routeInbound handles daemon-owned instance selection commands, then routes
// ordinary input to the selected live TUI. Explicit addressing is retained for
// backwards-compatible pending-queue cancellation and delivery.
func (d *Daemon) routeInbound(ctx context.Context, u Update, text string) {
	if text == listCommand {
		d.listInstances(ctx, u.ChatID)
		return
	}
	if n, ok := parseSelect(text); ok {
		d.selectInstance(ctx, u.ChatID, n)
		return
	}
	if strings.HasPrefix(text, selectCommand) {
		d.send(ctx, u.ChatID, "Usage: /select <number>. Send /list to view connected instances.")
		return
	}

	address, payload, ok := parseAddressed(text)
	if ok {
		d.routeAddressed(ctx, u, address, payload)
		return
	}
	if d.store == nil {
		d.send(ctx, u.ChatID, "No instance selected. Send /list, then /select <number>.")
		return
	}
	address, err := d.store.SelectedAddress()
	if err != nil {
		d.log.Error("read selected instance failed", "error", err)
		d.send(ctx, u.ChatID, "Unable to read the selected instance. Send /list and try again.")
		return
	}
	if address == "" {
		d.send(ctx, u.ChatID, "No instance selected. Send /list, then /select <number>.")
		return
	}
	if _, found := d.registry.lookup(address); !found {
		d.send(ctx, u.ChatID, "Selected instance is disconnected. Send /list, then /select <number>.")
		return
	}
	d.routeAddressed(ctx, u, address, text)
}

func (d *Daemon) listInstances(ctx context.Context, chatID string) {
	addresses := d.registry.addresses()
	if len(addresses) == 0 {
		d.send(ctx, chatID, "No connected instances.")
		return
	}
	var b strings.Builder
	b.WriteString("Connected instances:\n")
	for i, address := range addresses {
		fmt.Fprintf(&b, "%d. %s\n", i+1, address)
	}
	d.send(ctx, chatID, strings.TrimSuffix(b.String(), "\n"))
}

func (d *Daemon) selectInstance(ctx context.Context, chatID string, n int) {
	addresses := d.registry.addresses()
	if n > len(addresses) {
		d.send(ctx, chatID, "Invalid selection. Send /list to view connected instances.")
		return
	}
	address := addresses[n-1]
	if d.store == nil {
		d.send(ctx, chatID, "Unable to save the selected instance.")
		return
	}
	if err := d.store.SetSelectedAddress(address); err != nil {
		d.log.Error("save selected instance failed", "error", err)
		d.send(ctx, chatID, "Unable to save the selected instance. Try again.")
		return
	}
	d.log.Info("telegram instance selected", "address", address)
	d.send(ctx, chatID, fmt.Sprintf("Selected instance: %s.", address))
}

func (d *Daemon) routeAddressed(ctx context.Context, u Update, address, payload string) {
	action, arg := classifyInbound(payload)

	if action == actionCancelPending {
		d.cancelPending(ctx, u.ChatID, address)
		return
	}

	cli, found := d.registry.lookup(address)
	if !found {
		// Known but offline → durable queue; unknown → generic, no disclosure.
		if d.store != nil {
			if known, err := d.store.AddressKnown(address); err == nil && known {
				d.log.Info("inbound queued; no live client", "address", address)
				d.enqueue(u, address, arg, action == actionCommand)
				return
			}
		}
		d.genericReply(ctx, u.ChatID)
		return
	}

	msg := ipc.Message{
		Type:      ipc.TypeInbound,
		InboundID: "", // live delivery has no queue row
		Text:      arg,
		IsCommand: action == actionCommand,
	}
	select {
	case cli.outbound <- msg:
		d.log.Info("inbound pushed to live client", "address", address)
		d.send(ctx, u.ChatID, "OK, Received.")
	default:
		// Channel full: treat as temporarily unavailable and queue.
		d.log.Info("inbound queued; live client backlog", "address", address)
		d.enqueue(u, address, arg, action == actionCommand)
	}
}

func (d *Daemon) cancelPending(ctx context.Context, chatID, address string) {
	if d.store == nil {
		return
	}
	n, err := d.store.CancelPendingForAddress(address)
	if err != nil {
		d.log.Error("cancel pending failed", "error", err)
		d.send(ctx, chatID, "Failed to cancel pending messages.")
		return
	}
	d.send(ctx, chatID, fmt.Sprintf("Telegram: %d pending message(s) cancelled for %s.", n, address))
}

func (d *Daemon) enqueue(u Update, address, arg string, isCommand bool) {
	if d.store == nil {
		return
	}
	if err := d.store.EnqueuePending(PendingMessage{
		Address:   address,
		Text:      arg,
		IsCommand: isCommand,
		UpdateID:  u.UpdateID,
		CreatedAt: d.now(),
	}); err != nil {
		d.log.Error("enqueue pending failed", "error", err)
	}
}

func (d *Daemon) genericReply(ctx context.Context, chatID string) {
	d.send(ctx, chatID, "Unknown address. Prefix your message with a configured project address.")
}

// flushPending pushes queued messages to a reconnected client and marks them
// delivered (pending → delivered; TUI ack moves them to processed).
func (d *Daemon) flushPending(ctx context.Context, c *client) {
	if d.store == nil {
		return
	}
	rows, err := d.store.PendingForAddress(c.address)
	if err != nil {
		d.log.Error("read pending failed", "error", err)
		return
	}
	for _, p := range rows {
		_ = d.store.Transition(p.ID, StatusDelivered)
		msg := ipc.Message{
			Type:      ipc.TypeInbound,
			InboundID: strconv.FormatInt(p.ID, 10),
			Text:      p.Text,
			IsCommand: p.IsCommand,
		}
		select {
		case c.outbound <- msg:
		default:
		}
	}
}

// handleAck processes a delivery acknowledgement (delivered → processed).
func (d *Daemon) handleAck(ackID string) {
	if d.store == nil || ackID == "" {
		return
	}
	id, err := strconv.ParseInt(ackID, 10, 64)
	if err != nil {
		return
	}
	_ = d.store.Transition(id, StatusProcessed)
}

// handleClear removes the stored credentials (token + authorized chat id) from
// the OS vault and notifies every registered client (ADR-062).
func (d *Daemon) handleClear() {
	if err := d.vault.Clear(); err != nil {
		d.log.Error("vault clear failed", "error", err)
		return
	}
	d.setCreds("", "")
	if d.store != nil {
		if err := d.store.ClearSelectedAddress(); err != nil {
			d.log.Error("clear selected instance failed", "error", err)
		}
	}
	d.pairing.cancel()
	d.log.Info("telegram credentials cleared")
	d.broadcastEvent(ipc.EventCleared, "")
}

// handleTest sends a test message to the authorized chat so the user can verify
// the outbound Bot API path works (UI-C09-001 §1).
func (d *Daemon) handleTest(ctx context.Context) {
	_, chatID, _ := d.creds()
	if chatID == "" {
		return
	}
	d.send(ctx, chatID, "Test message from Hero.")
}

// handleSetCredentials stores the bot token in the vault (preserving any
// existing authorized chat id) and refreshes the cached credential.
func (d *Daemon) handleSetCredentials(_ context.Context, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	_, chatID, bot := d.creds()
	if err := d.vault.Store(token, chatID); err != nil {
		d.log.Error("vault store token failed", "error", err)
		return
	}
	d.setCreds(token, chatID)
	if bot == nil && d.botFactory != nil {
		d.setBot(d.botFactory(token))
	}
	d.log.Info("bot token stored in OS vault")
}

// handlePairStart begins pairing and broadcasts the single-use code.
func (d *Daemon) handlePairStart(_ context.Context) {
	token, _, _ := d.creds()
	if token == "" {
		d.broadcastEvent(ipc.EventPairingProgress, "missing-token")
		return
	}
	code := d.pairing.begin()
	d.log.Info("pairing started")
	d.broadcastEvent(ipc.EventPairingProgress, code)
}

// sendOutbound sends an outbound notification to the authorized chat, prefixed
// with the source address. It sends nothing when unpaired (ADR-063).
func (d *Daemon) sendOutbound(ctx context.Context, address, text string) {
	_, chatID, _ := d.creds()
	if chatID == "" {
		d.log.Debug("outbound skipped: not paired")
		return
	}
	d.send(ctx, chatID, prefixAddress(address, text))
}

// announceRegistration announces a new instance to the authorized chat.
func (d *Daemon) announceRegistration(ctx context.Context, c *client) {
	_, chatID, _ := d.creds()
	if chatID == "" {
		return
	}
	label := "registered"
	if c.mode == ipc.ModeFree {
		label = "free chat connected"
	}
	d.send(ctx, chatID, prefixAddress(c.address, label))
}

// broadcastEvent pushes a lifecycle event frame to every registered client.
func (d *Daemon) broadcastEvent(eventType, data string) {
	for _, addr := range d.registry.addresses() {
		if c, ok := d.registry.lookup(addr); ok {
			select {
			case c.outbound <- ipc.Message{Type: ipc.TypeEvent, EventType: eventType, EventData: data}:
			default:
			}
		}
	}
}

// ExpirePending expires stale pending rows and returns how many expired.
func (d *Daemon) ExpirePending() (int64, error) {
	if d.store == nil {
		return 0, nil
	}
	return d.store.ExpirePending(d.now())
}
