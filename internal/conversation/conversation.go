// Package conversation provides the transport-neutral conversation service
// shared by the Hero TUI and the Telegram remote interface (ADR-061). It owns
// input classification (slash vs plain text) and publishes lifecycle
// notifications through a narrow Notifier interface. It imports no Bubble Tea
// or lipgloss types, so harness adapters and the daemon ingress can both route
// through it without coupling.
//
// The TUI remains the view: it adapts Bubble Tea messages into Service calls and
// renders the structured results it receives. Telegram IPC ingress calls the
// same Service methods. Harness adapters never learn Telegram types.
package conversation

import (
	"context"
	"strings"
	"time"
)

// Origin identifies where a conversation turn came from.
type Origin string

const (
	// OriginLocal marks input typed in the TUI composer.
	OriginLocal Origin = "local"
	// OriginTelegram marks input delivered through the Telegram remote interface.
	OriginTelegram Origin = "telegram"
)

// Mode identifies the conversation context (cycle chat or free chat).
type Mode string

const (
	// ModeCycle is a project cycle conversation.
	ModeCycle Mode = "cycle"
	// ModeFree is a free chat conversation.
	ModeFree Mode = "free"
)

// Kind classifies a user input after dispatch.
type Kind string

const (
	// KindSlash is a recognized slash command (including control slashes).
	KindSlash Kind = "slash"
	// KindPlain is ordinary free-text input that starts a harness turn.
	KindPlain Kind = "plain"
)

// Input is a single user turn submitted to the Service.
type Input struct {
	// Text is the raw text (a slash command or plain text).
	Text string
	// Origin is where the turn came from.
	Origin Origin
	// Mode is the conversation context.
	Mode Mode
	// ProjectDir is the project root for the conversation (may be the user home
	// for free chat).
	ProjectDir string
	// Address is the allocated instance address for Telegram routing (e.g.
	// "ai_workflow_2"). Empty for local turns.
	Address string
}

// Dispatch is the result of classifying an Input.
type Dispatch struct {
	Kind     Kind
	Command  string // leading slash command word (e.g. "/hero-status"), empty for plain
	Argument string // text after the command word
}

// ClassifyInput classifies a user turn. A line whose first non-space character
// is '/' is a slash command; everything else is plain text. The returned
// Command includes the leading slash. Argument preserves the remainder
// (whitespace-trimmed).
func ClassifyInput(text string) Dispatch {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") {
		return Dispatch{Kind: KindPlain, Argument: text}
	}
	// Split on the first whitespace to separate command word from arguments.
	rest := trimmed
	idx := strings.IndexAny(rest, " \t\n")
	if idx == -1 {
		return Dispatch{Kind: KindSlash, Command: rest}
	}
	command := rest[:idx]
	arg := strings.TrimSpace(rest[idx:])
	return Dispatch{Kind: KindSlash, Command: command, Argument: arg}
}

// Session is the transport-neutral conversation session/context state.
type Session struct {
	ID         string
	ProjectDir string
	Mode       Mode
	AgentName  string
	StartedAt  time.Time
}

// Result is the outcome of a dispatched turn.
type Result struct {
	Output    string
	Summary   string
	SessionID string
	Duration  time.Duration
}

// Dispatcher runs a single classified turn against a harness. The TUI and the
// daemon ingress provide implementations; the Service owns classification and
// notification, not harness transport.
type Dispatcher interface {
	Execute(ctx context.Context, in Input) (Result, error)
}

// EventKind classifies lifecycle notifications published to the Notifier.
type EventKind string

const (
	EventCycleStarted     EventKind = "cycle_started"
	EventCycleFinished    EventKind = "cycle_finished"
	EventStageStarted     EventKind = "stage_started"
	EventStageFinished    EventKind = "stage_finished"
	EventApprovalRequired EventKind = "approval_required"
	EventError            EventKind = "error"
	EventFinalResult      EventKind = "final_result"
)

// Event is a lifecycle notification payload. It carries only cycle/stage
// identity and a message; it never carries stream, thinking, or tool content.
type Event struct {
	Kind       EventKind
	CycleID    int64
	CycleTitle string
	StageName  string
	Message    string
	Timestamp  time.Time
}

// Notifier receives lifecycle events. Subscribers (TUI transcript, Telegram
// outbound) filter locally; the Notifier never includes stream/thinking/tool
// events (PRD-C09-001 §3.3).
type Notifier interface {
	Notify(Event)
}

// NotifyFunc adapts a function to the Notifier interface.
type NotifyFunc func(Event)

// Notify implements Notifier.
func (f NotifyFunc) Notify(e Event) { f(e) }

// Service routes user input to a Dispatcher and publishes lifecycle events to a
// Notifier. It is transport-neutral and UI-free.
type Service struct {
	Dispatcher Dispatcher
	Notifier   Notifier
	Mode       Mode
	ProjectDir string
}

// New returns a Service with the given dispatcher and notifier.
func New(dispatcher Dispatcher, notifier Notifier) *Service {
	return &Service{Dispatcher: dispatcher, Notifier: notifier}
}

// Classify classifies text using ClassifyInput.
func (s *Service) Classify(text string) Dispatch {
	return ClassifyInput(text)
}

// Submit classifies in.Text and dispatches it. Plain text and non-control slash
// commands reach the Dispatcher; the Dispatcher is responsible for running the
// correct harness path. It returns the dispatch classification alongside the
// result so callers can render appropriately.
func (s *Service) Submit(ctx context.Context, in Input) (Dispatch, Result, error) {
	d := ClassifyInput(in.Text)
	if s.Dispatcher == nil {
		return d, Result{}, nil
	}
	res, err := s.Dispatcher.Execute(ctx, in)
	return d, res, err
}

// Publish forwards a lifecycle event to the configured Notifier, if any.
func (s *Service) Publish(e Event) {
	if s == nil || s.Notifier == nil {
		return
	}
	s.Notifier.Notify(e)
}
