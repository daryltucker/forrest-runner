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
	modelsOverride  []string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the benchmark suite",
	Long: `Executes the full benchmark suite against one or more Ollama servers.
The process follows a strict protocol:
1. Discovery: Finds all available models on the target URLs.
2. Health Check: Runs a streaming inference test to ensure the model is responsive.
3. Benchmarking: Executes multiple inference configurations to collect performance metrics.

Results are automatically saved to CSV and JSON formats, with automatic file versioning
(e.g., results.json.1) to prevent overwriting previous data.`,
	Example: `  # Run with defaults (uses forest_runner.yaml)
  forest-runner run

  # Override target URLs and output directory
  forest-runner run --urls http://ollama-1:11434,http://ollama-2:11434 -o ./benchmarks

  # Run only specific models
  forest-runner run --models qwen2.5:7b,llama3.1:8b

  # Exclude specific models (e.g., vision models)
  forest-runner run --exclude vision,moe

  # Use a specific prompt file
  forest-runner run -p ./prompts/code_gen.md`,
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
		if len(modelsOverride) > 0 {
			cfg.Models = modelsOverride
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
	runCmd.Flags().StringSliceVar(&modelsOverride, "models", nil, "Comma-separated list of specific models to run (skips discovery)")
}
