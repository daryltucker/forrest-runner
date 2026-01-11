/*
PURPOSE:
  Defines the root Cobra command for the Forest Runner CLI.
  Handles global flags and command initialization.

REQUIREMENTS:
  User-specified:
  - Provide a CLI interface.
  - Support global flags like --config.

  Implementation-discovered:
  - Needs to expose an Execute() function for main.go.

ARCHITECTURE INTEGRATION:
  - Called by: cmd/forest-runner/main.go
  - Calls: Child commands (run, list-models)
  - Modifies: Global configuration state (temporarily, until passed down).

ERROR HANDLING:
  - Returns error to main.go for exit code handling.

IMPLEMENTATION RULES:
  - Use `PersistentFlags()` for flags available to all subcommands.
  - Keep Run logic in subcommands, Root is usually empty or helps.

USAGE:
  Called by main.go.

SELF-HEALING INSTRUCTIONS:
  - If adding new global flags, add them to init().

RELATED FILES:
  - cmd/forest-runner/main.go

MAINTENANCE:
  - Update when adding global configuration options.
*/

package cli

import (
	"github.com/spf13/cobra"
)

var (
	// cfgFile stores the path to the config file (if specified via flag)
	cfgFile string

	rootCmd = &cobra.Command{
		Use:   "forest-runner",
		Short: "Benchmarking and testing tool for Ollama fleets",
		Long:  `A systematic auditing tool for Ollama models. Use 'run --help' for benchmark options.`,
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./forest_runner.yaml)")
}
