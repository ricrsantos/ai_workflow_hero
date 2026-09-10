package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
	"github.com/spf13/cobra"
)

const autoUpdateRestartTimeout = 5 * time.Second

func newAutoUpdateRestartCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "auto-update-restart",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return requestAutoUpdateRestartContext(cmd.Context())
		},
	}
}

// requestAutoUpdateRestart keeps the historical no-argument helper available
// to callers that do not have a Cobra context.
func requestAutoUpdateRestart() error {
	return requestAutoUpdateRestartContext(context.Background())
}

func requestAutoUpdateRestartContext(ctx context.Context) error {
	return requestAutoUpdateRestartWithTimeout(ctx, autoUpdateRestartTimeout)
}

// requestAutoUpdateRestartWithTimeout asks the daemon to broadcast a restart
// event. The deadline is deliberately local to this control-plane request: a
// daemon from an older release may accept the frame and never answer it.
func requestAutoUpdateRestartWithTimeout(ctx context.Context, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		return fmt.Errorf("request TUI restart: timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	socketPath, err := telegram.SocketPath("telegram")
	if err != nil {
		return fmt.Errorf("resolve Telegram IPC socket: %w", err)
	}
	conn, err := ipc.DialContext(ctx, socketPath)
	if err != nil {
		return fmt.Errorf("connect to Telegram daemon: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set Telegram IPC deadline: %w", err)
		}
	}

	client := ipc.NewConn(conn)
	if err := client.Send(ipc.Message{
		Type: ipc.TypeRequestUpdateRestart,
		UID:  ipc.CurrentUID(),
	}); err != nil {
		return fmt.Errorf("request TUI restart: %w", err)
	}
	response, err := client.Recv()
	if err != nil {
		return fmt.Errorf("read TUI restart response: %w", err)
	}
	if response.Type == ipc.TypeError {
		return fmt.Errorf("request TUI restart: %s", response.ErrorText)
	}
	if response.Type != ipc.TypeUpdateRestartAck {
		return fmt.Errorf("request TUI restart: unexpected response %q", response.Type)
	}
	sent, err := strconv.Atoi(response.EventData)
	if err != nil {
		return fmt.Errorf("request TUI restart: invalid client count %q", response.EventData)
	}
	if sent == 0 {
		return fmt.Errorf("request TUI restart: no registered Hero TUI instances")
	}
	return nil
}
