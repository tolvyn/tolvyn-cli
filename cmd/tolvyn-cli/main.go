package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const defaultAPIURL = "https://api.tolvyn.io"
const cliVersion = "1.0.0"

var (
	flagJSON    bool
	flagNoColor bool
)

const helpText = `TOLVYN — Financial Control Plane for AI Infrastructure

Usage:
  tolvyn <command> [flags]

Commands:
  init          Configure TOLVYN CLI
  login         Authenticate with your account
  logout        Clear stored credentials
  status        Check API connectivity and auth status
  tail          Stream live AI requests in real-time
  cost          Show spend summary
  requests      Show request history
  keys          Manage API keys (list, create, revoke)
  providers     Manage provider API keys (list, add)
  teams         Manage teams (list, create)
  budgets       Manage budgets (list, create)
  kill          Emergency spend kill switch
  models        List pricing models
  usage         Usage data commands (export CSV)

Flags:
  --api-url string   Override API URL (default from config)
  --json             Output raw JSON instead of formatted tables (all commands)
  --no-color         Disable colored output

Run 'tolvyn <command> --help' for command-specific flags.
`

var rootCmd = &cobra.Command{
	Use:           "tolvyn",
	Short:         "TOLVYN — Financial Control Plane for AI Infrastructure",
	SilenceUsage:  true,
	SilenceErrors: true,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(helpText)
		return nil
	},
}

func main() {
	rootCmd.PersistentFlags().StringVar(&apiURLOverride, "api-url", "", "Override API URL (default from config)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output raw JSON instead of formatted tables")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")

	// Use custom help function so --help prints our text without cobra's boilerplate.
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd == rootCmd {
			fmt.Print(helpText)
			return
		}
		// For subcommands, use default cobra help.
		cmd.Usage() //nolint:errcheck
	})

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if flagNoColor || !isTTY() {
			useColor = false
		}
	}

	rootCmd.AddCommand(
		cmdInit,
		cmdLogin,
		cmdLogout,
		cmdStatus,
		cmdTail,
		cmdCost,
		cmdRequests,
		cmdKeys,
		cmdProviders,
		cmdTeams,
		cmdBudgets,
		cmdModels,
		cmdKill,
		cmdUsage,
		cmdReconcile,
		&cobra.Command{
			Use:   "version",
			Short: "Show CLI version",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Printf("tolvyn version %s\n", cliVersion)
			},
		},
	)
	rootCmd.Version = cliVersion
	rootCmd.SetVersionTemplate("tolvyn version {{.Version}}\n")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		os.Exit(1)
	}
}
