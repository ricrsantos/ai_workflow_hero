package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/clierr"
	"github.com/ricrsantos/ai_workflow_hero/internal/telegram"
	"github.com/spf13/cobra"
)

// NewCommand returns the `hero plugin` command with install/uninstall/list
// subcommands for official plugins (ADR-003; ADR-059).
func NewCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage optional official plugins (e.g. telegram)",
		Long:  `Install, list, and uninstall optional official plugins such as the Telegram remote interface. Plugins are opt-in and are never enabled by a normal hero install.`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.AddCommand(
		newInstallCommand(version),
		newUninstallCommand(),
		newListCommand(),
	)
	return cmd
}

func newInstallCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "install <name>",
		Short: "Install an official plugin",
		Args:  cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()
			name := args[0]
			if name != telegram.PluginName {
				return fail(stderr, ErrUnsupportedPlugin{Name: name})
			}

			fmt.Fprintf(stdout, "Downloading %s %s for %s/%s from GitHub releases...\n",
				telegram.DaemonBinaryName, releaseTag(version), runtime.GOOS, runtime.GOARCH)
			m, err := InstallTelegramFromRelease(context.Background(), version, time.Now())
			if err != nil {
				return fail(stderr, err)
			}
			fmt.Fprintf(stdout, "Installed %s v%s (protocol v%d)\nDaemon: %s\n",
				m.Name, m.Version, m.ProtocolVersion, m.DaemonPath)
			return nil
		},
	}
}

func newUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Uninstall an official plugin",
		Args:  cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()
			name := args[0]
			if name != telegram.PluginName {
				return fail(stderr, ErrUnsupportedPlugin{Name: name})
			}
			pluginDir, err := telegram.PluginDir(telegram.PluginName)
			if err != nil {
				return fail(stderr, err)
			}
			if !IsInstalled(pluginDir) {
				fmt.Fprintf(stdout, "Plugin %q is not installed.\n", name)
				return nil
			}
			if err := UninstallTelegram(pluginDir); err != nil {
				return fail(stderr, err)
			}
			fmt.Fprintf(stdout, "Uninstalled %s plugin.\n", name)
			return nil
		},
	}
}

func newListCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()
			base, err := telegram.PluginsDir()
			if err != nil {
				return fail(stderr, err)
			}
			plugins, err := List(base)
			if err != nil {
				return fail(stderr, err)
			}
			if asJSON {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(plugins)
			}
			if len(plugins) == 0 {
				fmt.Fprintln(stdout, "No plugins installed.")
				return nil
			}
			for _, p := range plugins {
				fmt.Fprintf(stdout, "%s\t%s\t%s\n", p.Name, p.Version, p.DaemonPath)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func fail(stderr io.Writer, err error) error {
	e := clierr.New(err.Error())
	clierr.Format(stderr, e)
	return e
}
