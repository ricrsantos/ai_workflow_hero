// Package lifecycle provides the private, process-local event bridge used by
// Hero's TUI and CLI-as-API child processes. It deliberately carries only the
// transport-neutral conversation.Event payload.
package lifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/conversation"
)

// EventSocketEnv is inherited by CLI-as-API and harness child processes.
const EventSocketEnv = "HERO_LIFECYCLE_EVENT_SOCKET"

const (
	relaySocketMode = 0o600
	relayDirMode    = 0o700
	notifyTimeout   = 500 * time.Millisecond
	readTimeout     = 1 * time.Second
)

// Relay receives lifecycle events for one owning TUI process.
type Relay struct {
	path     string
	listener net.Listener
	events   chan conversation.Event
	cancel   context.CancelFunc
	done     chan struct{}

	closeOnce sync.Once
	connWG    sync.WaitGroup
}

// NewRelay creates a private Unix-domain event endpoint. The endpoint is
// project-scoped where the path fits the platform's Unix socket limit; a short
// per-user temporary path is used for unusually deep project directories.
func NewRelay(projectDir string) (*Relay, error) {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return nil, fmt.Errorf("lifecycle relay: project directory is empty")
	}

	runDir := filepath.Join(projectDir, ".workflow-hero", "run")
	if err := os.MkdirAll(runDir, relayDirMode); err != nil {
		return nil, fmt.Errorf("lifecycle relay: create run directory: %w", err)
	}
	_ = os.Chmod(runDir, relayDirMode)

	path := filepath.Join(runDir, fmt.Sprintf("lifecycle-%d.sock", os.Getpid()))
	// Linux and macOS cap sockaddr_un paths at roughly 104–108 bytes. Keep
	// enough headroom for the kernel and use a short exact temp path if needed.
	if len(path) >= 100 {
		path = filepath.Join(os.TempDir(), fmt.Sprintf("hero-lifecycle-%d-%s.sock", os.Getuid(), shortProjectHash(projectDir)))
	}
	if err := removeStaleSocket(path); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("lifecycle relay: listen on %q: %w", path, err)
	}
	if err := os.Chmod(path, relaySocketMode); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("lifecycle relay: protect socket: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &Relay{
		path:     path,
		listener: listener,
		events:   make(chan conversation.Event, 64),
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	go r.acceptLoop(ctx)
	return r, nil
}

func shortProjectHash(projectDir string) string {
	sum := sha256.Sum256([]byte(projectDir))
	return hex.EncodeToString(sum[:])[:12]
}

func removeStaleSocket(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("lifecycle relay: inspect socket: %w", err)
	}
	if info.IsDir() || info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("lifecycle relay: socket path is not a Unix socket: %s", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("lifecycle relay: remove stale socket: %w", err)
	}
	return nil
}

// Path returns the endpoint path to pass to child processes.
func (r *Relay) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// Events returns the ordered stream of received lifecycle events.
func (r *Relay) Events() <-chan conversation.Event {
	if r == nil {
		return nil
	}
	return r.events
}

func (r *Relay) acceptLoop(ctx context.Context) {
	defer close(r.done)
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Debug("lifecycle relay accept failed", "error", err)
			continue
		}
		r.connWG.Add(1)
		go func() {
			defer r.connWG.Done()
			r.handleConnection(ctx, conn)
		}()
	}
}

func (r *Relay) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
	var event conversation.Event
	if err := json.NewDecoder(conn).Decode(&event); err != nil {
		slog.Debug("lifecycle relay decode failed", "error", err)
		return
	}
	if event.Kind == "" || event.CycleID <= 0 {
		slog.Debug("lifecycle relay ignored invalid event", "kind", event.Kind, "cycle_id", event.CycleID)
		return
	}
	select {
	case r.events <- event:
	case <-ctx.Done():
	}
}

// Close stops the endpoint and removes its exact socket path.
func (r *Relay) Close() error {
	if r == nil {
		return nil
	}
	var err error
	r.closeOnce.Do(func() {
		r.cancel()
		err = r.listener.Close()
		<-r.done
		r.connWG.Wait()
		close(r.events)
		if removeErr := os.Remove(r.path); removeErr != nil && !os.IsNotExist(removeErr) && err == nil {
			err = removeErr
		}
	})
	return err
}

// envNotifier is the child-side synchronous notifier. Notifications are
// best-effort: a lifecycle event must never make a CLI command fail or stall.
type envNotifier struct {
	path string
}

// NewEnvNotifier returns a notifier for EventSocketEnv, or nil when the
// process is not running under an owning TUI.
func NewEnvNotifier() conversation.Notifier {
	path := strings.TrimSpace(os.Getenv(EventSocketEnv))
	if path == "" {
		return nil
	}
	return envNotifier{path: path}
}

func (n envNotifier) Notify(event conversation.Event) {
	if strings.TrimSpace(n.path) == "" {
		return
	}
	conn, err := net.DialTimeout("unix", n.path, notifyTimeout)
	if err != nil {
		slog.Debug("lifecycle event relay unavailable", "error", err)
		return
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(notifyTimeout))
	if err := json.NewEncoder(conn).Encode(event); err != nil {
		slog.Debug("lifecycle event relay send failed", "error", err)
	}
}

// SocketEnv returns the child-process environment assignment for path.
// Keeping this helper here avoids duplicating the environment key in adapters.
func SocketEnv(path string) string {
	return EventSocketEnv + "=" + strings.TrimSpace(path)
}

// Compile-time guard for accidental notifier API drift.
var _ conversation.Notifier = envNotifier{}
