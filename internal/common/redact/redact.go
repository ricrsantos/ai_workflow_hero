// Package redact provides shared credential redaction for Hero logs and
// diagnostics (ADR-064; PRD-C09-001 §3.4). Telegram bot tokens and authorized
// chat ids must never reach a log line, error, or debug payload.
package redact

import (
	"io"
	"regexp"
	"strings"
)

// RedactedValue replaces any secret occurrence in redacted output.
const RedactedValue = "[REDACTED]"

// botTokenPattern matches Telegram bot tokens of the form "<bot_id>:<token>",
// where the token is the base64url secret returned by BotFather. The leading
// word boundary is deliberately omitted so tokens embedded in URLs
// (…/bot<token>/…) are matched even though the digits abut "bot".
var botTokenPattern = regexp.MustCompile(`\d{6,12}:[A-Za-z0-9_-]{30,}`)

// Redact masks Telegram bot tokens and every explicit secret value in s.
// Explicit secrets are typically the authorized chat id (a bare numeric id) and
// the full token string when the caller holds them (daemon only). The token
// pattern is applied unconditionally so even an echoed request URL is masked.
//
// Redact is safe to call with an empty secret list and is not secret-aware
// beyond the values it is given: it never emits a secret, only removes one.
func Redact(s string, secrets ...string) string {
	out := botTokenPattern.ReplaceAllString(s, RedactedValue)
	for _, sec := range secrets {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}
		out = strings.ReplaceAll(out, sec, RedactedValue)
	}
	return out
}

// HasToken reports whether s appears to contain a Telegram bot token.
// It is a diagnostic helper for tests and sanitizers.
func HasToken(s string) bool {
	return botTokenPattern.MatchString(s)
}

// Writer wraps an io.Writer and redacts every Write. It is line-oriented in the
// sense that it applies Redact to each Write call, which matches how slog and
// the daemon logger emit one line per Write.
type Writer struct {
	W       io.Writer
	Secrets []string
}

// Write redacts p and forwards the result to the wrapped writer. It reports the
// full length of p as consumed (satisfying the io.Writer contract) even though
// the on-disk bytes may differ after redaction.
func (w Writer) Write(p []byte) (int, error) {
	out := Redact(string(p), w.Secrets...)
	if _, err := io.WriteString(w.W, out); err != nil {
		return 0, err
	}
	return len(p), nil
}
