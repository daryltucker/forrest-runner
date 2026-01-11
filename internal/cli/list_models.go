/*
PURPOSE:
  Defines the 'list-models' subcommand.
  Helps debug connectivity and model discovery.

REQUIREMENTS:
  User-specified:
  - List available models.

  Implementation-discovered:
  - Useful validation step before full run.

ARCHITECTURE INTEGRATION:
  - Calls: internal/engine.GetModels() (via Client)

ERROR HANDLING:
  - Prints error if URL incorrect.

IMPLEMENTATION RULES:
  - Simple output to stdout.

USAGE:
  forest-runner list-models --urls ...

SELF-HEALING INSTRUCTIONS:
  - None.

RELATED FILES:
  - internal/engine/client.go

MAINTENANCE:
  - None.
*/

package cli

import (
	"fmt"
	"os"

	"github.com/daryltucker/forest-runner/internal/config"
	"github.com/daryltucker/forest-runner/internal/engine"
	"github.com/spf13/cobra"
)

var listModelsCmd = &cobra.Command{
	Use:   "list-models",
	Short: "List available models on target hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Reuse config loading/override logic?
		// Or just use flags?
		// Let's try to be consistent.
		cfg := config.DefaultConfig()
		// (Skipping file load for simplicity unless needed, or reuse loaded cfg if we abstracted it)

		if len(urlsOverride) > 0 {
			cfg.URLs = urlsOverride
		}

		e := engine.New(cfg)

		for _, url := range cfg.URLs {
			fmt.Printf("Querying %s...\n", url)
			models, err := e.GetModels(url)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				continue
			}
			for _, m := range models {
				fmt.Printf("- %s\n", m)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listModelsCmd)
	// Reuse overrides flags from run? Or define new ones?
	// If I defined them on runCmd, they belong to runCmd.
	// I should define them here too, or moving them to Persistent on root if shared.
	// `urls` seems common.
	listModelsCmd.Flags().StringSliceVar(&urlsOverride, "urls", nil, "Comma-separated list of URLs")
}
