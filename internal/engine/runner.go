/*
PURPOSE:
  High-level runner that orchestrates the benchmarking process.
  Loops through URLs -> Models -> Configs and executes tests.

REQUIREMENTS:
  User-specified:
  - Run suite against all detected models.
  - Log results to CSV/JSON.

  Implementation-discovered:
  - Needs to report progress to CLI.

ARCHITECTURE INTEGRATION:
  - Called by: internal/cli
  - Uses: internal/engine, internal/output

ERROR HANDLING:
  - Logs errors but continues (resilience).

IMPLEMENTATION RULES:
  - Iterate URLs.
  - For each URL: GetModels.
  - For each Model: Stream Test (fast check).
  - For each Model: Infer Test (benchmarks with configs).

USAGE:
  engine.Run(cfg)

SELF-HEALING INSTRUCTIONS:
  - None.

RELATED FILES:
  - internal/engine/client.go

MAINTENANCE:
  - Update iteration logic if parallelism is introduced.
*/

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/daryltucker/forest-runner/internal/config"
	"github.com/daryltucker/forest-runner/internal/output"
)

// Run executes the full benchmark suite.
func Run(cfg *config.Config) error {
	e := New(cfg)

	// Ensure output directory exists
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", cfg.OutputDir, err)
	}

	// Setup Outputs
	csvPath := filepath.Join(cfg.OutputDir, cfg.OutputFile)
	csvWriter, err := output.NewCSVWriter(csvPath)
	if err != nil {
		return fmt.Errorf("failed to init CSV writer at %s: %w", csvPath, err)
	}
	defer csvWriter.Close()

	jsonPath := filepath.Join(cfg.OutputDir, "model_results.json")
	jsonWriter, err := output.NewJSONWriter(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to init JSON writer at %s: %w", jsonPath, err)
	}
	defer jsonWriter.Close()

	// 1. Discovery Phase
	targets := make(map[string][]string)
	for _, url := range cfg.URLs {
		output.Logger.Info("Discovering models...", "url", url)
		models, err := e.GetModels(url)
		if err != nil {
			output.Logger.Error("Failed to discover models", "url", url, "error", err)
			continue
		}
		output.Logger.Info("Found models", "url", url, "count", len(models))
		targets[url] = models
	}

	// 2. Execution Phase
	for url, models := range targets {
		for _, modelName := range models {
			// Check Exclusions
			shouldSkip := false
			for _, ex := range cfg.Exclude {
				if strings.Contains(strings.ToLower(modelName), strings.ToLower(ex)) {
					output.Logger.Info("Skipping model (excluded)", "model", modelName, "filter", ex)
					shouldSkip = true
					break
				}
			}
			if shouldSkip {
				continue
			}

			output.Logger.Info("Testing Model", "model", modelName, "url", url)

			// A. Stream Test (Health Check)
			err := e.StreamInference(url, modelName, cfg.Prompt)
			if err != nil {
				output.Logger.Error("Stream Inference Failed", "model", modelName, "error", err)
				// Determine if we should skip metrics if health check fails.
				// Python script behavior: Try streaming, then try configs anyway.
			} else {
				output.Logger.Info("Stream Inference Success", "model", modelName)
			}

			// B. Metric Tests (Configs)
			for _, inferCfg := range cfg.InferConfigs {
				output.Logger.Info("Running Inference Config", "model", modelName, "config", inferCfg)

				res, err := e.Inference(url, modelName, cfg.Prompt, inferCfg)
				if err != nil {
					output.Logger.Error("Inference Benchmark Failed", "model", modelName, "config", inferCfg, "error", err)
					// Log failed result with error?
					res.Error = err.Error()
					// Write partial result and continue to next config
					if err := csvWriter.Write(res); err != nil {
						output.Logger.Error("Failed to write partial result to CSV", "error", err)
					}
					if err := jsonWriter.Write(res); err != nil {
						output.Logger.Error("Failed to write partial result to JSON", "error", err)
					}
					continue
				}

				// Capture VRAM Stats (Model is likely still loaded)
				size, vram, err := e.GetRunningModelInfo(url, modelName)
				if err == nil && size > 0 {
					res.MemoryUsage = size
					res.VRAMUsage = vram
					res.VRAMPercentage = float64(vram) / float64(size) * 100.0
				}

				output.Logger.Info("Inference Success",
					"model", modelName,
					"duration", res.Duration,
					"tokens_gen", res.TokensGenerated,
					"vram_pct", fmt.Sprintf("%.1f%%", res.VRAMPercentage),
				)

				// Write Result
				if err := csvWriter.Write(res); err != nil {
					output.Logger.Error("Failed to write result to CSV", "error", err)
				}
				if err := jsonWriter.Write(res); err != nil {
					output.Logger.Error("Failed to write result to JSON", "error", err)
				}
				// Optional: Sleep between runs?
				time.Sleep(1 * time.Second)
			}
		}
	}

	return nil
}
