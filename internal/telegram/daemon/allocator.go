package daemon

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

// client is a live registered TUI connection.
type client struct {
	address    string
	projectDir string
	mode       string
	abbrev     string
	outbound   chan ipc.Message
}

// registry tracks live clients and owns instance suffix allocation. A single
// daemon process serializes registration under one mutex, making allocation
// atomic under concurrent TUI launches (ADR-060; telegram-ipc R2).
type registry struct {
	mu      sync.Mutex
	clients map[*client]struct{}
	byAddr  map[string]*client
}

func newRegistry() *registry {
	return &registry{
		clients: make(map[*client]struct{}),
		byAddr:  make(map[string]*client),
	}
}

// register allocates a stable address and records the client. The address is
// the project base abbreviation for the first live instance, _2+ while any
// sibling is connected, or free_N for free chat (PRD-C09-001 §3.2).
func (r *registry) register(projectDir, mode, abbrev string, outbound chan ipc.Message) (*client, string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var addr string
	if mode == ipc.ModeFree {
		for i := 1; ; i++ {
			cand := fmt.Sprintf("free_%d", i)
			if _, taken := r.byAddr[cand]; !taken {
				addr = cand
				break
			}
		}
	} else {
		base := normalizeAbbrev(abbrev)
		if _, taken := r.byAddr[base]; !taken {
			addr = base
		} else {
			for i := 2; ; i++ {
				cand := fmt.Sprintf("%s_%d", base, i)
				if _, taken := r.byAddr[cand]; !taken {
					addr = cand
					break
				}
			}
		}
	}

	c := &client{address: addr, projectDir: projectDir, mode: mode, abbrev: abbrev, outbound: outbound}
	r.clients[c] = struct{}{}
	r.byAddr[addr] = c
	return c, addr
}

// unregister removes a client and frees its address.
func (r *registry) unregister(c *client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clients[c]; !ok {
		return
	}
	delete(r.clients, c)
	if r.byAddr[c.address] == c {
		delete(r.byAddr, c.address)
	}
}

// lookup returns the live client for address, if any.
func (r *registry) lookup(address string) (*client, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.byAddr[address]
	return c, ok
}

// count returns the number of live clients.
func (r *registry) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients)
}

// addresses returns the sorted set of live addresses.
func (r *registry) addresses() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.byAddr))
	for a := range r.byAddr {
		out = append(out, a)
	}
	return out
}

// normalizeAbbrev lowercases and restricts an abbreviation to [a-z0-9_-], with
// a safe default when the result is empty.
func normalizeAbbrev(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "proj"
	}
	return out
}
