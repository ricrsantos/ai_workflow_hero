// Package claude contains the Claude Code adapter and its protocol boundary.
package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	// MinimumCLIVersion is the oldest Claude Code release whose headless
	// stream-json protocol Hero supports (ADR-071).
	MinimumCLIVersion = "2.1.261"

	// PermissionPromptToolName is the sole MCP tool accepted by the temporary
	// permission bridge. Execute wires the bridge for one turn only after the
	// installed CLI has passed this protocol gate.
	PermissionPromptToolName = "mcp__hero_permissions__approval_prompt"

	maxNDJSONLineBytes = 1 << 20
)

var (
	// ErrAskUnsupported is returned when the installed CLI cannot prove the
	// permission-prompt protocol needed for the conservative ask profile.
	ErrAskUnsupported = errors.New("claude ask permission transport is unsupported")
	// ErrPermissionTokenInvalid is intentionally indistinguishable for an
	// unknown and a replayed token, so bridge callers do not disclose state.
	ErrPermissionTokenInvalid = errors.New("claude permission token is invalid or expired")

	versionPattern = regexp.MustCompile(`(?i)\b(?:claude(?:\s+code)?\s+)?v?(\d+)\.(\d+)\.(\d+)\b`)
)

// OneTimeToken accepts precisely one request for an execution-scoped bridge.
// It does not generate tokens: the adapter injects cryptographically secure
// randomness at its execution boundary in task 3/4.
type OneTimeToken struct {
	mu    sync.Mutex
	value string
	used  bool
}

// NewOneTimeToken creates a token gate. An empty input is never usable.
func NewOneTimeToken(value string) *OneTimeToken {
	return &OneTimeToken{value: strings.TrimSpace(value)}
}

// Consume verifies the presented token and expires it immediately. Callers
// must keep a gate private to one bridge execution.
func (t *OneTimeToken) Consume(candidate string) error {
	if t == nil {
		return ErrPermissionTokenInvalid
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.used || t.value == "" || strings.TrimSpace(candidate) != t.value {
		return ErrPermissionTokenInvalid
	}
	t.used = true
	return nil
}

// Invalidate permanently closes a token gate when its execution ends. This is
// deliberately separate from Consume so cleanup also protects a bridge that
// never received a request.
func (t *OneTimeToken) Invalidate() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.value = ""
	t.used = true
	t.mu.Unlock()
}

// Invocation describes one injected Claude CLI invocation. Future execution
// code uses the same shape, keeping command construction testable without
// launching a real Claude process.
type Invocation struct {
	Args                 []string
	WorkingDir           string
	Environment          []string
	StartNewProcessGroup bool
}

// Clone returns an isolated copy suitable for recording in test launchers.
func (i Invocation) Clone() Invocation {
	i.Args = append([]string(nil), i.Args...)
	i.Environment = append([]string(nil), i.Environment...)
	return i
}

// ProbeResult contains the observable result of a compatibility probe.
// Stderr is retained for classification but must never be rendered verbatim to
// users because it can include project or authentication details.
type ProbeResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Launcher is the process boundary used by the protocol gate. Task 3 extends
// this boundary for turn-scoped streaming processes; availability checks only
// call Run and never authenticate or start a Claude session.
type Launcher interface {
	Run(context.Context, Invocation) (ProbeResult, error)
}

// ProtocolGate validates the installed CLI protocol before an adapter enables
// a behavior that could otherwise weaken permission handling.
type ProtocolGate struct {
	Launcher Launcher
	Logger   *slog.Logger
}

func (g ProtocolGate) logger() *slog.Logger {
	if g.Logger != nil {
		return g.Logger
	}
	return slog.Default()
}

// Verify checks the minimum version and the flags used by a stream-json turn.
// It does not start a Claude session, contact a service, or inspect credentials.
func (g ProtocolGate) Verify(ctx context.Context) error {
	if g.Launcher == nil {
		return fmt.Errorf("verify claude protocol: launcher is required")
	}

	g.logger().Debug("probing claude protocol", "probe", "version")
	version, err := g.Launcher.Run(ctx, Invocation{Args: []string{"--version"}})
	if err != nil {
		g.logger().Error("claude version probe failed", "error", err)
		return fmt.Errorf("probe claude version: %w", err)
	}
	if version.ExitCode != 0 {
		err := fmt.Errorf("claude --version exited with status %d", version.ExitCode)
		g.logger().Error("claude version probe failed", "error", err)
		return err
	}
	if err := ValidateVersion(version.Stdout); err != nil {
		g.logger().Error("claude version is incompatible", "error", err)
		return err
	}

	g.logger().Debug("probing claude protocol", "probe", "help")
	help, err := g.Launcher.Run(ctx, Invocation{Args: []string{"--help"}})
	if err != nil {
		g.logger().Error("claude flag probe failed", "error", err)
		return fmt.Errorf("probe claude flags: %w", err)
	}
	if help.ExitCode != 0 {
		err := fmt.Errorf("claude --help exited with status %d", help.ExitCode)
		g.logger().Error("claude flag probe failed", "error", err)
		return err
	}
	if err := ValidateRequiredFlags(help.Stdout); err != nil {
		g.logger().Error("claude flags are incompatible", "error", err)
		return err
	}

	g.logger().Info("claude protocol verified", "minimum_version", MinimumCLIVersion)
	return nil
}

// VerifyAskTransport proves only that the CLI advertises the permission-prompt
// flag and that the captured MCP request/response shape is safe to bridge. It
// intentionally fails closed: callers must not substitute another profile.
func (g ProtocolGate) VerifyAskTransport(ctx context.Context, request, decision []byte) error {
	if g.Launcher == nil {
		return fmt.Errorf("verify claude ask transport: launcher is required")
	}
	help, err := g.Launcher.Run(ctx, Invocation{Args: []string{"--help"}})
	if err != nil {
		g.logger().Error("claude ask transport probe failed", "error", err)
		return fmt.Errorf("probe claude ask transport: %w", err)
	}
	if help.ExitCode != 0 || !strings.Contains(help.Stdout, "--permission-prompt-tool") {
		return fmt.Errorf("%w: Claude Code %s+ with --permission-prompt-tool is required", ErrAskUnsupported, MinimumCLIVersion)
	}
	if err := ValidatePermissionToolRequest(request); err != nil {
		return fmt.Errorf("%w: invalid permission request fixture: %v", ErrAskUnsupported, err)
	}
	if err := ValidatePermissionToolDecision(decision); err != nil {
		return fmt.Errorf("%w: invalid permission decision fixture: %v", ErrAskUnsupported, err)
	}
	g.logger().Info("claude ask permission transport verified")
	return nil
}

// StreamInvocation builds the non-interactive stream command. The process
// group requirement is captured here so the later supervisor can reliably send
// SIGINT to the turn and all children without affecting foreign processes.
func StreamInvocation(workDir, model string) Invocation {
	return Invocation{
		Args: []string{
			"-p",
			"--output-format", "stream-json",
			"--verbose",
			"--include-partial-messages",
			"--model", strings.TrimSpace(model),
		},
		WorkingDir:           strings.TrimSpace(workDir),
		StartNewProcessGroup: true,
	}
}

// ValidateVersion checks a Claude Code semantic version against the supported
// floor. It accepts common `claude --version` prefixes but rejects malformed
// strings rather than guessing compatibility.
func ValidateVersion(output string) error {
	match := versionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(match) != 4 {
		return fmt.Errorf("claude version output is malformed; require %s or newer", MinimumCLIVersion)
	}
	got := [3]int{}
	for i := range got {
		n, err := strconv.Atoi(match[i+1])
		if err != nil {
			return fmt.Errorf("parse claude version: %w", err)
		}
		got[i] = n
	}
	minimum := [3]int{2, 1, 261}
	for i := range minimum {
		if got[i] > minimum[i] {
			return nil
		}
		if got[i] < minimum[i] {
			return fmt.Errorf("Claude Code %d.%d.%d is unsupported; require %s or newer", got[0], got[1], got[2], MinimumCLIVersion)
		}
	}
	return nil
}

// ValidateRequiredFlags checks the command-line surface required by Hero's
// supervised streaming adapter. It deliberately includes the permission tool;
// ask mode remains unavailable unless VerifyAskTransport also succeeds.
func ValidateRequiredFlags(help string) error {
	for _, flag := range []string{
		"--output-format",
		"stream-json",
		"--verbose",
		"--include-partial-messages",
		"--model",
		"--resume",
		"--permission-prompt-tool",
	} {
		if !strings.Contains(help, flag) {
			return fmt.Errorf("Claude Code is missing required flag or value %q", flag)
		}
	}
	return nil
}

// ValidateLiveAskTransportFlags verifies the CLI flags required to attach the
// temporary, strict MCP server that carries Hero's permission callback. The
// permission-prompt-tool flag alone is not sufficient proof of a live bridge.
func ValidateLiveAskTransportFlags(help string) error {
	for _, flag := range []string{"--permission-prompt-tool", "--mcp-config", "--strict-mcp-config"} {
		if !strings.Contains(help, flag) {
			return fmt.Errorf("Claude Code is missing ask transport flag %q", flag)
		}
	}
	return nil
}

// RawEvent is one validated NDJSON frame. Normalization into harness deltas is
// intentionally deferred to the execution adapter, preserving the protocol
// fixture gate as an independent compatibility check.
type RawEvent struct {
	Line    int
	Type    string
	Subtype string
	Raw     json.RawMessage
}

// DecodeNDJSON reads one JSON object per physical line. Line numbers make an
// upstream protocol regression actionable without exposing raw payloads in a
// normal user-facing error.
func DecodeNDJSON(r io.Reader) ([]RawEvent, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxNDJSONLineBytes)
	var events []RawEvent
	for line := 1; scanner.Scan(); line++ {
		raw := append([]byte(nil), scanner.Bytes()...)
		if len(strings.TrimSpace(string(raw))) == 0 {
			continue
		}
		var envelope struct {
			Type    string `json:"type"`
			Subtype string `json:"subtype"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, fmt.Errorf("decode Claude NDJSON line %d: %w", line, err)
		}
		if strings.TrimSpace(envelope.Type) == "" {
			return nil, fmt.Errorf("decode Claude NDJSON line %d: event type is required", line)
		}
		events = append(events, RawEvent{Line: line, Type: envelope.Type, Subtype: envelope.Subtype, Raw: raw})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Claude NDJSON: %w", err)
	}
	return events, nil
}

// ValidatePermissionToolRequest validates the MCP tools/call request that the
// future bridge receives. The random token is checked for presence here; the
// bridge enforces one-time use when it owns execution state.
func ValidatePermissionToolRequest(raw []byte) error {
	var request struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Method  string `json:"method"`
		Params  struct {
			Name      string `json:"name"`
			Arguments struct {
				Token    string          `json:"token"`
				ToolName string          `json:"tool_name"`
				Input    json.RawMessage `json:"input"`
			} `json:"arguments"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return fmt.Errorf("decode MCP permission request: %w", err)
	}
	if request.JSONRPC != "2.0" || request.ID == nil || request.Method != "tools/call" {
		return errors.New("request is not an MCP tools/call message")
	}
	if request.Params.Name != "approval_prompt" {
		return errors.New("request is not the approval_prompt tool")
	}
	if strings.TrimSpace(request.Params.Arguments.Token) == "" || strings.TrimSpace(request.Params.Arguments.ToolName) == "" {
		return errors.New("request token and tool_name are required")
	}
	if len(request.Params.Arguments.Input) == 0 || !json.Valid(request.Params.Arguments.Input) {
		return errors.New("request input must be JSON")
	}
	return nil
}

// ValidatePermissionToolDecision validates Claude's documented JSON-string
// response convention for permission-prompt tools. Only allow and deny are
// accepted, preventing accidental permissive fallbacks.
func ValidatePermissionToolDecision(raw []byte) error {
	var response struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Result  struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return fmt.Errorf("decode MCP permission decision: %w", err)
	}
	if response.JSONRPC != "2.0" || response.ID == nil || len(response.Result.Content) != 1 || response.Result.Content[0].Type != "text" {
		return errors.New("decision is not a single MCP text result")
	}
	var decision struct {
		Behavior     string          `json:"behavior"`
		UpdatedInput json.RawMessage `json:"updatedInput"`
		Message      string          `json:"message"`
	}
	if err := json.Unmarshal([]byte(response.Result.Content[0].Text), &decision); err != nil {
		return fmt.Errorf("decode stringified permission decision: %w", err)
	}
	switch decision.Behavior {
	case "allow":
		if len(decision.UpdatedInput) == 0 || !json.Valid(decision.UpdatedInput) {
			return errors.New("allow decision requires JSON updatedInput")
		}
	case "deny":
		if strings.TrimSpace(decision.Message) == "" {
			return errors.New("deny decision requires a message")
		}
	default:
		return errors.New("decision behavior must be allow or deny")
	}
	return nil
}
