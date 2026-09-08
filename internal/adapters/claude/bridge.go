package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// PermissionBridge serves the one permitted MCP method for one Claude turn.
// It deliberately has no tool discovery, credentials, plugins, or resource
// operations: all it can do is relay a validated approval decision.
type PermissionBridge struct {
	Reader  io.Reader
	Writer  io.Writer
	Token   *OneTimeToken
	Request func(context.Context, harness.PermissionRequest) (harness.PermissionResponse, error)
	// SessionID is attached to normalized callbacks when the Claude stream has
	// already announced its native session. It is optional because the bridge
	// may receive a permission request before the first stream frame.
	SessionID string
	Logger    *slog.Logger
}

func (b PermissionBridge) logger() *slog.Logger {
	if b.Logger != nil {
		return b.Logger
	}
	return slog.Default()
}

// Run handles one JSON-RPC permission request. A token is consumed before the
// TUI callback runs, so replay cannot create an additional approval prompt.
func (b PermissionBridge) Run(ctx context.Context) error {
	if b.Reader == nil || b.Writer == nil || b.Token == nil || b.Request == nil {
		return errors.New("Claude permission bridge is not configured")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(b.Reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxNDJSONLineBytes)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("read Claude permission bridge: %w", err)
		}
		return io.EOF
	}
	raw := append([]byte(nil), scanner.Bytes()...)
	if err := ValidatePermissionToolRequest(raw); err != nil {
		b.logger().Error("claude permission bridge rejected request", "error", err)
		return err
	}
	request, err := decodeBridgeRequest(raw)
	if err != nil {
		return err
	}
	if err := b.Token.Consume(request.Token); err != nil {
		b.logger().Error("claude permission bridge rejected token", "error", err)
		return err
	}
	b.logger().Info("claude permission decision requested", "tool", request.ToolName)
	response, callbackErr := b.Request(ctx, harness.PermissionRequest{
		ID:          request.ID,
		Title:       "Claude permission: " + request.ToolName,
		Description: "Claude requests permission to use " + request.ToolName + ".",
		HarnessType: "claude.permission_prompt",
		SessionID:   b.SessionID,
	})
	if callbackErr != nil {
		response = harness.PermissionResponse{Approved: false, Reason: "permission decision unavailable"}
	}
	decision, err := encodeBridgeDecision(request.ID, request.Input, response)
	if err != nil {
		return err
	}
	if _, err := b.Writer.Write(append(decision, '\n')); err != nil {
		return fmt.Errorf("write Claude permission decision: %w", err)
	}
	if callbackErr != nil {
		return fmt.Errorf("Claude permission decision: %w", callbackErr)
	}
	b.logger().Info("claude permission decision forwarded", "approved", response.Approved)
	return nil
}

// Close invalidates the execution token and closes any closable transport.
// Execute calls this on every exit path, including a cancelled turn, so a
// stale request cannot be replayed after the Claude child is gone.
func (b *PermissionBridge) Close() error {
	if b == nil {
		return nil
	}
	if b.Token != nil {
		b.Token.Invalidate()
	}
	var firstErr error
	for _, value := range []any{b.Reader, b.Writer} {
		closer, ok := value.(io.Closer)
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type bridgeRequest struct {
	ID       string
	Token    string
	ToolName string
	Input    json.RawMessage
}

func decodeBridgeRequest(raw []byte) (bridgeRequest, error) {
	var wire struct {
		ID     any `json:"id"`
		Params struct {
			Arguments struct {
				Token    string          `json:"token"`
				ToolName string          `json:"tool_name"`
				Input    json.RawMessage `json:"input"`
			} `json:"arguments"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return bridgeRequest{}, fmt.Errorf("decode Claude permission bridge request: %w", err)
	}
	identifier, ok := wire.ID.(string)
	if !ok || strings.TrimSpace(identifier) == "" {
		return bridgeRequest{}, errors.New("Claude permission bridge request id must be a string")
	}
	return bridgeRequest{ID: identifier, Token: wire.Params.Arguments.Token, ToolName: wire.Params.Arguments.ToolName, Input: wire.Params.Arguments.Input}, nil
}

func encodeBridgeDecision(id string, input json.RawMessage, response harness.PermissionResponse) ([]byte, error) {
	decision := map[string]any{"behavior": "deny", "message": firstNonEmpty(response.Reason, "permission denied")}
	if response.Approved {
		var updated any
		if err := json.Unmarshal(input, &updated); err != nil {
			return nil, fmt.Errorf("decode approved Claude permission input: %w", err)
		}
		decision = map[string]any{"behavior": "allow", "updatedInput": updated}
	}
	payload, err := json.Marshal(decision)
	if err != nil {
		return nil, fmt.Errorf("encode Claude permission decision payload: %w", err)
	}
	wire := map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"content": []map[string]string{{"type": "text", "text": string(payload)}}}}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode Claude permission bridge response: %w", err)
	}
	if err := ValidatePermissionToolDecision(encoded); err != nil {
		return nil, fmt.Errorf("validate Claude permission bridge response: %w", err)
	}
	return encoded, nil
}
