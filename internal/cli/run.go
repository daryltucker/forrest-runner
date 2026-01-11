/*
PURPOSE:
  Defines the 'run' subcommand.
  Executes the full benchmark suite.

REQUIREMENTS:
  User-specified:
  - Run the benchmarks.
  - specific flags for overrides.

  Implementation-discovered:
  - Need to load config first.
  - Apply flag overrides to config.

ARCHITECTURE INTEGRATION:
  - Calls: internal/engine.Run()
  - Uses: internal/config

ERROR HANDLING:
  - Returns error if config load fails or engine run fails.

IMPLEMENTATION RULES:
  - Setup flags in init().
  - Logic: Load Config -> Override -> Engine.Run.

USAGE:
  forest-runner run --urls https://...

SELF-HEALING INSTRUCTIONS:
  - Check flag names match Config struct fields generally.

RELATED FILES:
  - internal/cli/root.go

MAINTENANCE:
  - Update when adding new CLI overrides.
*/

package cli

import (
	"fmt"
	"os"

	"github.com/daryltucker/forest-runner/internal/config"
	"github.com/daryltucker/forest-runner/internal/engine"
	"github.com/spf13/cobra"
)

var (
	urlsOverride    []string
	outputOverride  string
	promptFile      string
	excludeOverride []string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the benchmark suite",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Load Config
		cfg, err := config.Load(cfgFile)
		// If err != nil here, it means user specified a file that didn't load, OR
		// parsing failed. config.Load now handles "no file found" by returning defaults.
		if err != nil {
			return err
		}

		// 2. Overrides
		if len(urlsOverride) > 0 {
			cfg.URLs = urlsOverride
		}
		if outputOverride != "" {
			cfg.OutputDir = outputOverride
		}
		if promptFile != "" {
			data, err := os.ReadFile(promptFile)
			if err != nil {
				return fmt.Errorf("failed to read prompt file: %w", err)
			}
			cfg.Prompt = string(data)
		}
		if len(excludeOverride) > 0 {
			cfg.Exclude = excludeOverride
		}

		// 3. Execution
		return engine.Run(cfg)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringSliceVar(&urlsOverride, "urls", nil, "Comma-separated list of Ollama URLs")
	runCmd.Flags().StringVarP(&outputOverride, "output-dir", "o", "", "Output directory for results (CSV/JSON)")
	runCmd.Flags().StringVarP(&promptFile, "prompt-file", "p", "", "Path to a markdown/text file containing the prompt (overrides config)")
	runCmd.Flags().StringSliceVar(&excludeOverride, "exclude", nil, "Comma-separated list of substrings to exclude from model names")
}
