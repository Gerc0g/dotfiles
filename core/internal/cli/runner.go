package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Gerc0g/dotfiles/core/agentconfig"
	"github.com/Gerc0g/dotfiles/core/runner"
	"github.com/spf13/cobra"
)

func newRunnerCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "runner", Short: "Изолированные серверные задачи Codex"}
	var directory, preset, sessionOptions string
	start := &cobra.Command{Use: "app-server", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		var options []agentconfig.SessionOptions
		if cmd.Flags().Changed("session-options") {
			parsed, err := agentconfig.ParseSessionOptions(sessionOptions)
			if err != nil {
				return err
			}
			options = append(options, parsed)
		}
		return runner.Run(ctx, directory, preset, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), options...)
	}}
	start.Flags().StringVar(&directory, "directory", "", "Registered workspace directory")
	start.Flags().StringVar(&preset, "preset", "work", "work, research or ordinary")
	start.Flags().StringVar(&sessionOptions, "session-options", "", "Typed non-secret per-session preferences (JSON)")
	var inspectDirectory, inspectPreset, inspectSessionOptions string
	inspect := &cobra.Command{Use: "resolve", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		b, e := runner.Resolve(inspectDirectory, inspectPreset)
		if e != nil {
			return e
		}
		if cmd.Flags().Changed("session-options") {
			parsed, err := agentconfig.ParseSessionOptions(inspectSessionOptions)
			if err != nil {
				return err
			}
			if _, err = agentconfig.SessionEffectiveFor(b.Preset, parsed); err != nil {
				return err
			}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(b)
	}}
	inspect.Flags().StringVar(&inspectDirectory, "directory", "", "Registered workspace directory")
	inspect.Flags().StringVar(&inspectPreset, "preset", "work", "work, research or ordinary")
	inspect.Flags().StringVar(&inspectSessionOptions, "session-options", "", "Typed non-secret per-session preferences (JSON)")
	inside := &cobra.Command{Use: "inside", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return runner.Inside(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
	}}
	broker := &cobra.Command{Use: "broker OPERATION [JSON]", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		raw := json.RawMessage(`{}`)
		if len(args) == 2 {
			raw = json.RawMessage(args[1])
		}
		if !json.Valid(raw) {
			return fmt.Errorf("invalid JSON")
		}
		result, e := runner.Broker(cmd.Context(), args[0], raw)
		if e != nil {
			return e
		}
		_, e = fmt.Fprintln(cmd.OutOrStdout(), string(result))
		return e
	}}
	status := &cobra.Command{Use: "status", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		s, e := runner.List()
		if e != nil {
			return e
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(s)
	}}
	cmd.AddCommand(start, inspect, inside, broker, status)
	return cmd
}
