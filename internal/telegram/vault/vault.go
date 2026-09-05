// Package vault isolates Telegram credentials (bot token + authorized chat id)
// in the operating-system credential vault (ADR-062; PRD-C09-001 §3.4). Only the
// daemon resolves values through this abstraction; project/global SQLite never
// stores secrets. Tests inject a fake store; unsupported platforms fail setup
// explicitly rather than falling back to environment variables or plaintext.
package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/zalando/go-keyring"
)

// Service and Account identify the single vault entry.
const (
	Service = "ai-workflow-hero-telegram"
	Account = "telegram"
)

// ErrNotFound is returned by Load when no vault entry exists.
var ErrNotFound = errors.New("telegram vault: entry not found")

// ErrUnavailable is returned when the platform has no supported vault backend.
var ErrUnavailable = errors.New("telegram vault: no supported OS credential vault on this platform")

// Entry holds the bot token and authorized chat id together.
type Entry struct {
	Token  string `json:"token"`
	ChatID string `json:"chat_id"`
}

// Store is the credential vault abstraction (design D4). Implementations must
// never log or return secrets through error messages.
type Store interface {
	// Store persists the token and chat id together.
	Store(token, chatID string) error
	// Load returns the stored entry, or ErrNotFound when absent.
	Load() (Entry, error)
	// Clear removes the entry. Clearing a missing entry is not an error.
	Clear() error
	// Available reports whether the OS credential vault is usable.
	Available() bool
}

// MemoryStore is an in-memory fake used by tests and headless environments
// (PRD-C09-001 §5). It is not persisted and must never be used in production.
type MemoryStore struct {
	mu    sync.RWMutex
	entry *Entry
}

// NewMemory returns an empty in-memory Store.
func NewMemory() *MemoryStore {
	return &MemoryStore{}
}

func (m *MemoryStore) Store(token, chatID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entry = &Entry{Token: token, ChatID: chatID}
	return nil
}

func (m *MemoryStore) Load() (Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.entry == nil {
		return Entry{}, ErrNotFound
	}
	return *m.entry, nil
}

func (m *MemoryStore) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entry = nil
	return nil
}

func (m *MemoryStore) Available() bool { return true }

// KeyringStore stores credentials in the OS vault (Secret Service on Linux,
// Keychain on macOS) via github.com/zalando/go-keyring.
type KeyringStore struct{}

// NewKeyring returns the OS-backed Store.
func NewKeyring() *KeyringStore { return &KeyringStore{} }

// Available reports whether this platform has a supported backend.
func (k *KeyringStore) Available() bool {
	switch runtime.GOOS {
	case "linux", "darwin":
		return true
	default:
		return false
	}
}

func (k *KeyringStore) Store(token, chatID string) error {
	if !k.Available() {
		return fmt.Errorf("%w (GOOS=%s)", ErrUnavailable, runtime.GOOS)
	}
	blob, err := json.Marshal(Entry{Token: token, ChatID: chatID})
	if err != nil {
		return fmt.Errorf("encode vault entry: %w", err)
	}
	if err := keyring.Set(Service, Account, string(blob)); err != nil {
		return fmt.Errorf("store in OS vault: %w", err)
	}
	return nil
}

func (k *KeyringStore) Load() (Entry, error) {
	if !k.Available() {
		return Entry{}, fmt.Errorf("%w (GOOS=%s)", ErrUnavailable, runtime.GOOS)
	}
	blob, err := keyring.Get(Service, Account)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("load from OS vault: %w", err)
	}
	var e Entry
	if err := json.Unmarshal([]byte(blob), &e); err != nil {
		return Entry{}, fmt.Errorf("decode vault entry: %w", err)
	}
	return e, nil
}

func (k *KeyringStore) Clear() error {
	if !k.Available() {
		return fmt.Errorf("%w (GOOS=%s)", ErrUnavailable, runtime.GOOS)
	}
	if err := keyring.Delete(Service, Account); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("clear from OS vault: %w", err)
	}
	return nil
}
