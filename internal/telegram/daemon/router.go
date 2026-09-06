package daemon

import (
	"strconv"
	"strings"
)

// cancelPendingCommand is the daemon-owned queue cancellation command
// (ADR-063). It never maps to /hero-cancel or a harness interrupt.
const cancelPendingCommand = "/telegram-cancel-pending"

const (
	listCommand   = "/list"
	selectCommand = "/select"
)

// inboundAction classifies an addressed payload.
type inboundAction int

const (
	actionPlain         inboundAction = iota // ordinary text → one harness turn
	actionCommand                            // slash command → TUI command path
	actionCancelPending                      // daemon-owned queue cancellation
)

// parseAddressed splits an inbound message into "<address>:" and its payload.
// The address prefix is required and case-sensitive (PRD-C09-001 §3.2).
// Malformed input returns ok=false.
func parseAddressed(text string) (address, payload string, ok bool) {
	text = strings.TrimSpace(text)
	idx := strings.Index(text, ":")
	if idx <= 0 {
		return "", "", false
	}
	address = text[:idx]
	payload = strings.TrimSpace(text[idx+1:])
	if address == "" || payload == "" {
		return "", "", false
	}
	return address, payload, true
}

// classifyInbound classifies an addressed payload.
func classifyInbound(payload string) (inboundAction, string) {
	p := strings.TrimSpace(payload)
	if p == cancelPendingCommand {
		return actionCancelPending, p
	}
	if strings.HasPrefix(p, "/") {
		return actionCommand, p
	}
	return actionPlain, p
}

// parseSelect parses the daemon-owned /select command. It accepts only a
// one-based list position so addresses never need to be repeated by users.
func parseSelect(text string) (int, bool) {
	fields := strings.Fields(text)
	if len(fields) != 2 || fields[0] != selectCommand {
		return 0, false
	}
	n, err := strconv.Atoi(fields[1])
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}
