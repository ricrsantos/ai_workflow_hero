package codex

import (
	"context"
	"fmt"
	"strings"
)

// AuthError is returned when Codex is not authenticated (ADR-047).
// Hero never prompts for an API key and never embeds login in the TUI.
type AuthError struct {
	Err error
}

func (e *AuthError) Error() string {
	// UI-C06-001 §6 — user-facing copy only; unwrap Err for diagnostics.
	return "Codex is not authenticated.\n\n  Suggestion: run `codex login` in a terminal, then retry."
}

func (e *AuthError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// requireAuth checks account/read after handshake. Unauthenticated → AuthError.
func (a *Adapter) requireAuth(ctx context.Context) error {
	var result struct {
		Account            any  `json:"account"`
		RequiresOpenaiAuth bool `json:"requiresOpenaiAuth"`
	}
	if err := a.rpcCall(ctx, "account/read", map[string]any{"refreshToken": false}, &result); err != nil {
		// Older/incompatible servers may lack account/read; surface as app-server error
		// only when the message clearly indicates auth, otherwise continue.
		if isAuthMessage(err.Error()) {
			return &AuthError{Err: err}
		}
		a.log().Debug("codex account/read unavailable", "error", err)
		return nil
	}
	if result.Account == nil && result.RequiresOpenaiAuth {
		return &AuthError{Err: fmt.Errorf("account/read: no account")}
	}
	if result.Account == nil && !result.RequiresOpenaiAuth {
		// Local/dev modes that do not require OpenAI auth.
		return nil
	}
	return nil
}

func isAuthMessage(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "unauthor") ||
		strings.Contains(lower, "not authenticated") ||
		strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "login required") ||
		strings.Contains(lower, "sign in") ||
		strings.Contains(lower, "codex login")
}
