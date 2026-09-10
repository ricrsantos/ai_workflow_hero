package main

import (
	"fmt"
	"strconv"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
	"github.com/spf13/cobra"
)

func newAutoUpdateRestartCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "auto-update-restart",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return requestAutoUpdateRestart()
		},
	}
}

func requestAutoUpdateRestart() error {
	socketPath, err := telegram.SocketPath("telegram")
	if err != nil {
		return fmt.Errorf("resolve Telegram IPC socket: %w", err)
	}
	conn, err := ipc.Dial(socketPath)
	if err != nil {
		return fmt.Errorf("connect to Telegram daemon: %w", err)
	}
	defer conn.Close()

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
