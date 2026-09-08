package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const (
	// PermissionTokenEnv is passed only to the short-lived MCP helper. It is
	// never inherited by Claude itself or written to application logs.
	PermissionTokenEnv = "HERO_CLAUDE_PERMISSION_TOKEN"
	// PermissionBridgeAddressEnv identifies the per-turn loopback listener for
	// the helper. The listener accepts one connection and validates the token.
	PermissionBridgeAddressEnv = "HERO_CLAUDE_PERMISSION_BRIDGE_ADDRESS"
)

// PermissionBridgeConfig contains only the data needed by one Claude turn.
// In particular, it has no credential, plugin, or arbitrary MCP configuration.
type PermissionBridgeConfig struct {
	Token     string
	SessionID string
	Request   func(context.Context, harness.PermissionRequest) (harness.PermissionResponse, error)
	Logger    *slog.Logger
}

// PermissionBridgeHandle owns one execution-scoped bridge listener. Run must
// return when the listener is closed or its context is cancelled.
type PermissionBridgeHandle interface {
	Run(context.Context) error
	Close() error
}

// PermissionBridgeStarter creates a listener for one Execute call. Keeping
// this seam injectable lets protocol and lifecycle tests use a fake listener
// without starting a helper process or Claude account.
type PermissionBridgeStarter interface {
	StartPermissionBridge(context.Context, PermissionBridgeConfig) (PermissionBridgeHandle, error)
}

// PermissionBridgeStarterFunc adapts a function to PermissionBridgeStarter.
type PermissionBridgeStarterFunc func(context.Context, PermissionBridgeConfig) (PermissionBridgeHandle, error)

// StartPermissionBridge implements PermissionBridgeStarter.
func (f PermissionBridgeStarterFunc) StartPermissionBridge(ctx context.Context, config PermissionBridgeConfig) (PermissionBridgeHandle, error) {
	return f(ctx, config)
}

// permissionMCPConfigProvider proves that the production bridge has a live,
// execution-scoped helper configuration. Test doubles do not need this detail.
type permissionMCPConfigProvider interface {
	permissionMCPConfig() permissionMCPConfig
}

type permissionMCPConfig struct {
	Address string
	Token   string
}

func writePermissionMCPConfig(config permissionMCPConfig) (string, error) {
	if strings.TrimSpace(config.Address) == "" || strings.TrimSpace(config.Token) == "" {
		return "", errors.New("Claude permission bridge has no live transport")
	}
	helper, err := os.Executable()
	if err != nil || strings.TrimSpace(helper) == "" {
		if err == nil {
			err = errors.New("empty executable path")
		}
		return "", fmt.Errorf("resolve Hero permission helper: %w", err)
	}
	payload, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"hero_permissions": map[string]any{
		"type":    "stdio",
		"command": helper,
		"args":    []string{"internal", "claude-permission-bridge"},
		"env": map[string]string{
			PermissionTokenEnv:         config.Token,
			PermissionBridgeAddressEnv: config.Address,
		},
	}}})
	if err != nil {
		return "", fmt.Errorf("encode Claude permission MCP config: %w", err)
	}
	file, err := os.CreateTemp("", "hero-claude-permission-*.json")
	if err != nil {
		return "", fmt.Errorf("create Claude permission MCP config: %w", err)
	}
	path := file.Name()
	if err := file.Chmod(0o600); err == nil {
		_, err = file.Write(payload)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("write Claude permission MCP config: %w", err)
	}
	return path, nil
}

type defaultPermissionBridgeStarter struct{}

func (defaultPermissionBridgeStarter) StartPermissionBridge(_ context.Context, config PermissionBridgeConfig) (PermissionBridgeHandle, error) {
	if strings.TrimSpace(config.Token) == "" || config.Request == nil {
		return nil, errors.New("Claude permission bridge requires a token and callback")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for Claude permission bridge: %w", err)
	}
	return &socketPermissionBridge{
		listener: listener,
		bridge: PermissionBridge{
			Token:     NewOneTimeToken(config.Token),
			Request:   config.Request,
			SessionID: config.SessionID,
			Logger:    config.Logger,
		},
		done: make(chan struct{}),
	}, nil
}

// socketPermissionBridge accepts exactly one localhost helper connection. The
// Claude child starts that helper as a stdio MCP server via --mcp-config; the
// helper forwards only approval_prompt calls to this socket.
type socketPermissionBridge struct {
	listener  net.Listener
	bridge    PermissionBridge
	done      chan struct{}
	closeOnce sync.Once
}

func (b *socketPermissionBridge) permissionMCPConfig() permissionMCPConfig {
	if b == nil || b.listener == nil || b.bridge.Token == nil {
		return permissionMCPConfig{}
	}
	return permissionMCPConfig{Address: b.listener.Addr().String(), Token: b.bridge.Token.value}
}

func (b *socketPermissionBridge) Run(ctx context.Context) error {
	if b == nil || b.listener == nil {
		return errors.New("Claude permission bridge listener is not configured")
	}
	accepted := make(chan struct {
		conn net.Conn
		err  error
	}, 1)
	go func() {
		conn, err := b.listener.Accept()
		accepted <- struct {
			conn net.Conn
			err  error
		}{conn: conn, err: err}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-b.done:
		return net.ErrClosed
	case result := <-accepted:
		if result.err != nil {
			if errors.Is(result.err, net.ErrClosed) {
				return net.ErrClosed
			}
			return fmt.Errorf("accept Claude permission helper: %w", result.err)
		}
		defer result.conn.Close()
		b.bridge.Reader = result.conn
		b.bridge.Writer = result.conn
		return b.bridge.Run(ctx)
	}
}

func (b *socketPermissionBridge) Close() error {
	if b == nil {
		return nil
	}
	var err error
	b.closeOnce.Do(func() {
		close(b.done)
		b.bridge.Token.Invalidate()
		if b.listener != nil {
			err = b.listener.Close()
		}
	})
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

// RunPermissionMCPHelper serves the sole stdio MCP endpoint exposed to Claude
// for a single turn. It handles MCP negotiation locally, injects the private
// one-time token, and forwards only validated approval_prompt calls to the
// parent bridge. It does not implement resources, prompts, credentials, or
// general MCP tool forwarding.
func RunPermissionMCPHelper(ctx context.Context, reader io.Reader, writer io.Writer, address, token string) error {
	if strings.TrimSpace(address) == "" || strings.TrimSpace(token) == "" {
		return errors.New("Claude permission MCP helper is not configured")
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxNDJSONLineBytes)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw := append([]byte(nil), scanner.Bytes()...)
		response, forward, err := permissionMCPResponse(raw, token)
		if err != nil {
			return err
		}
		if !forward {
			if len(response) != 0 {
				if _, err := writer.Write(append(response, '\n')); err != nil {
					return fmt.Errorf("write Claude permission MCP response: %w", err)
				}
			}
			continue
		}
		conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
		if err != nil {
			return fmt.Errorf("connect Claude permission bridge: %w", err)
		}
		if _, err := conn.Write(append(response, '\n')); err != nil {
			_ = conn.Close()
			return fmt.Errorf("forward Claude permission request: %w", err)
		}
		decision, err := bufio.NewReader(conn).ReadBytes('\n')
		_ = conn.Close()
		if err != nil {
			return fmt.Errorf("read Claude permission decision: %w", err)
		}
		if err := ValidatePermissionToolDecision(decision); err != nil {
			return fmt.Errorf("validate Claude permission decision: %w", err)
		}
		if _, err := writer.Write(decision); err != nil {
			return fmt.Errorf("write Claude permission decision: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Claude permission MCP request: %w", err)
	}
	return nil
}

func permissionMCPResponse(raw []byte, token string) ([]byte, bool, error) {
	var request struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, false, fmt.Errorf("decode Claude permission MCP request: %w", err)
	}
	if request.JSONRPC != "2.0" || strings.TrimSpace(request.Method) == "" {
		return nil, false, errors.New("Claude permission MCP request is invalid")
	}
	if len(request.ID) == 0 || string(request.ID) == "null" {
		return nil, false, nil
	}
	switch request.Method {
	case "initialize":
		return marshalMCPResult(request.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": "hero_permissions", "version": "1"},
		}), false, nil
	case "tools/list":
		return marshalMCPResult(request.ID, map[string]any{"tools": []any{map[string]any{
			"name":        "approval_prompt",
			"description": "Forwards one Claude permission decision to the supervised Hero TUI.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"tool_name": map[string]string{"type": "string"},
				"input":     map[string]string{"type": "object"},
			}, "required": []string{"tool_name", "input"}},
		}}}), false, nil
	case "tools/call":
		var call struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(request.Params, &call); err != nil {
			return nil, false, errors.New("Claude permission MCP tool parameters are invalid")
		}
		if call.Name != "approval_prompt" {
			return nil, false, errors.New("Claude permission MCP tool is not allowed")
		}
		var arguments map[string]json.RawMessage
		if err := json.Unmarshal(call.Arguments, &arguments); err != nil {
			return nil, false, errors.New("Claude permission MCP tool arguments are invalid")
		}
		encodedToken, _ := json.Marshal(token)
		arguments["token"] = encodedToken
		encodedArguments, _ := json.Marshal(arguments)
		forward := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(request.ID), "method": "tools/call", "params": map[string]any{"name": call.Name, "arguments": json.RawMessage(encodedArguments)}}
		encoded, err := json.Marshal(forward)
		if err != nil {
			return nil, false, fmt.Errorf("encode Claude permission bridge request: %w", err)
		}
		if err := ValidatePermissionToolRequest(encoded); err != nil {
			return nil, false, err
		}
		return encoded, true, nil
	default:
		return marshalMCPError(request.ID, -32601, "method not found"), false, nil
	}
}

func marshalMCPResult(id json.RawMessage, result any) []byte {
	encoded, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result})
	return encoded
}

func marshalMCPError(id json.RawMessage, code int, message string) []byte {
	encoded, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "error": map[string]any{"code": code, "message": message}})
	return encoded
}
