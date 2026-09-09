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
// The address prefix is case-sensitive and must match the allocated abbrev form
// (lowercase [a-z0-9_-], same charset as normalizeAbbrev). Prose that happens
// to contain ":" (e.g. "agente: ...") returns ok=false so /select routing can
// forward the full text (PRD-C09-001 §3.2; ADR-063).
func parseAddressed(text string) (address, payload string, ok bool) {
	text = strings.TrimSpace(text)
	idx := strings.Index(text, ":")
	if idx <= 0 {
		return "", "", false
	}
	address = text[:idx]
	payload = strings.TrimSpace(text[idx+1:])
	if payload == "" || !validAddressToken(address) {
		return "", "", false
	}
	return address, payload, true
}

// validAddressToken reports whether s looks like a daemon-allocated instance
// address (base abbrev, base_N, or free_N). Uppercase and spaces are rejected
// so ordinary sentences with colons fall through to selection routing.
func validAddressToken(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			continue
		case r == '_' || r == '-':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
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
