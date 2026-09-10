package telegram

import "strings"

// HelpCommand is the daemon-owned Telegram command that lists remote commands.
const HelpCommand = "/help"

// IsHelpCommand reports whether text is the Telegram /help command.
func IsHelpCommand(text string) bool {
	return strings.EqualFold(strings.TrimSpace(text), HelpCommand)
}

// CommandHelpText returns the compact catalog of Telegram-available commands.
// It is daemon-owned in production so /help works before /select; the TUI keeps
// a matching handler as defense in depth for addressed delivery paths.
func CommandHelpText() string {
	return strings.TrimSpace(`Hero Telegram commands

Routing
/list — List connected TUI instances
/select <n> — Select instance n from /list
/help — Show this help

Selected instance
/status — Cycle/session status, timers, and context usage
/version — Show the Hero version
/interrupt — Cancel in-flight agent work (same as Chat Ctrl+C)
/kill — Force-kill the selected TUI (last resort; daemon stays up)
/auto-update — Commit source changes and queue a local Hero binary update
/model — Choose free-chat harness, model, and properties
/hero-config — Guided cycle configuration wizard
/hero-config-show — Show cycle config or the active draft
/hero-config cancel — Discard the in-progress config draft
/hero-permission <id> allow|deny — Answer a harness permission prompt

Queue
<address>: /telegram-cancel-pending — Cancel queued messages for that address

Hero slash commands
After /select, send the same /hero-* commands as in Chat, for example:
/hero-new, /hero-start, /hero-approve, /hero-reject, /hero-cancel, /hero-finish,
/hero-archive, /hero-resume, /hero-sync, /hero-status, /hero-continue, /hero-back,
/hero-cycles, /hero-todos, /hero-help

Ordinary text goes to the selected TUI as a harness turn.
Addressed form: <address>: <text or command>
Pairing: /start <code> (only while pairing is open in Settings)`)
}
