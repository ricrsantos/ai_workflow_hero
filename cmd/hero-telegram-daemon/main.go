// Command hero-telegram-daemon is the per-OS-user Telegram Bot API daemon
// (ADR-059). It owns the Bot API connection, serves versioned local IPC to Hero
// TUIs, and stores only non-sensitive state in its private SQLite store. It
// contains no LLM reasoning and is installed by `hero plugin install telegram`.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/logrotate"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/daemon"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/vault"
)

func main() {
	if err := run(); err != nil {
		slog.Error("telegram daemon exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logPath, err := telegram.DaemonLogPath()
	if err != nil {
		return err
	}
	logWriter := logrotate.New(logPath)
	defer logWriter.Close()
	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	root, err := telegram.UserHeroDir()
	if err != nil {
		return err
	}
	store, err := daemon.OpenStore(filepath.Join(root, "telegram-daemon.db"))
	if err != nil {
		return err
	}
	defer store.Close()

	socketPath, err := telegram.SocketPath("telegram")
	if err != nil {
		return err
	}

	v := vault.NewKeyring()
	var bot daemon.BotAPI
	if e, loadErr := v.Load(); loadErr == nil && e.Token != "" {
		bot = daemon.NewHTTPBotAPI(e.Token)
	}

	d := daemon.New(daemon.Options{
		Bot:        bot,
		Vault:      v,
		Store:      store,
		BotFactory: func(t string) daemon.BotAPI { return daemon.NewHTTPBotAPI(t) },
		SocketPath: socketPath,
		Logger:     logger,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return d.Run(ctx)
}
