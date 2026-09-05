package daemon

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

// pairingTTL is the single-use pairing code validity window (10 minutes).
const pairingTTL = 10 * time.Minute

// pairingCodeDigits is the length of the numeric pairing code.
const pairingCodeDigits = 6

// pairing holds the active single-use code (PRD-C09-001 §3.4).
type pairing struct {
	code      string
	expiresAt time.Time
}

// valid reports whether the code is unexpired at now.
func (p *pairing) valid(now time.Time) bool {
	return p != nil && now.Before(p.expiresAt)
}

// pairingManager serializes pairing code lifecycle. The code is held only for
// the pending operation and never persisted (ADR-062).
type pairingManager struct {
	mu      sync.Mutex
	current *pairing
	now     func() time.Time
}

func newPairingManager(now func() time.Time) *pairingManager {
	if now == nil {
		now = time.Now
	}
	return &pairingManager{now: now}
}

// begin generates and stores a new code, returning it.
func (pm *pairingManager) begin() string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	code := randomCode(pairingCodeDigits)
	pm.current = &pairing{code: code, expiresAt: pm.now().Add(pairingTTL)}
	return code
}

// cancel invalidates the active code.
func (pm *pairingManager) cancel() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.current = nil
}

// active returns the active code, or empty when none.
func (pm *pairingManager) active() string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.current == nil || !pm.current.valid(pm.now()) {
		return ""
	}
	return pm.current.code
}

// validate checks whether text matches the active code ("/start <code>" or bare
// code). It returns (matched, expired). A nil return means no code is active.
func (pm *pairingManager) validate(text string) (matched, expired bool, code *pairing) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.current == nil {
		return false, false, nil
	}
	cur := pm.current
	if cur.valid(pm.now()) {
		text = strings.TrimSpace(stripBotCommand(text))
		text = strings.TrimPrefix(text, "/start")
		text = strings.TrimSpace(text)
		return text == cur.code, false, cur
	}
	// Code expired: invalidate it.
	pm.current = nil
	return false, true, nil
}

// consume removes the active code after a successful bind.
func (pm *pairingManager) consume() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.current = nil
}

func randomCode(digits int) string {
	max := big.NewInt(10)
	max.Exp(max, big.NewInt(int64(digits)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return fmt.Sprintf("%0*d", digits, 0)
	}
	return fmt.Sprintf("%0*d", digits, n.Int64())
}
