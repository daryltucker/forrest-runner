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
	"sync"
	"time"

	"github.com/daryltucker/forest-runner/internal/config"
	"github.com/daryltucker/forest-runner/internal/output"
)

// nextAvailablePath returns the original path if it doesn't exist,
// otherwise appends .1, .2, etc. until an available path is found.
func nextAvailablePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	for i := 1; ; i++ {
		newPath := fmt.Sprintf("%s.%d", path, i)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
}

// Run executes the full benchmark suite.
func Run(cfg *config.Config) error {
	e := New(cfg)

	// Ensure output directory exists
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", cfg.OutputDir, err)
	}

	// Setup Outputs with Versioning
	csvPath := nextAvailablePath(filepath.Join(cfg.OutputDir, cfg.OutputFile))
	csvWriter, err := output.NewCSVWriter(csvPath)
	if err != nil {
		return fmt.Errorf("failed to init CSV writer at %s: %w", csvPath, err)
	}
	defer csvWriter.Close()

	jsonPath := nextAvailablePath(filepath.Join(cfg.OutputDir, "model_results.json"))
	jsonWriter, err := output.NewJSONWriter(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to init JSON writer at %s: %w", jsonPath, err)
	}
	defer jsonWriter.Close()

	// Handle Concurrency
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(cfg.URLs) {
		concurrency = len(cfg.URLs)
	}

	urlChan := make(chan string, len(cfg.URLs))
	for _, url := range cfg.URLs {
		urlChan <- url
	}
	close(urlChan)

	var wg sync.WaitGroup
	output.Logger.Info("Starting Fleet Cruise", "backends", len(cfg.URLs), "concurrency", concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range urlChan {
				runForURL(e, cfg, url, csvWriter, jsonWriter)
			}
		}()
	}

	wg.Wait()
	output.Logger.Info("Fleet Cruise Completed", "results_csv", csvPath, "results_json", jsonPath)
	return nil
}

// runForURL handles the full benchmark cycle for a single backend URL.
func runForURL(e *Engine, cfg *config.Config, url string, csvWriter *output.CSVWriter, jsonWriter *output.JSONWriter) {
	// 1. Discovery Phase
	var models []string
	var err error

	if len(cfg.Models) > 0 {
		output.Logger.Info("Using explicit model list", "url", url, "count", len(cfg.Models))
		models = cfg.Models
	} else {
		output.Logger.Info("Discovering models...", "url", url)
		models, err = e.GetModels(url)
		if err != nil {
			output.Logger.Error("Failed to discover models", "url", url, "error", err)
			return
		}
		output.Logger.Info("Found models", "url", url, "count", len(models))
	}

	// 2. Execution Phase
	for _, modelName := range models {
		// Check Exclusions
		shouldSkip := false
		for _, ex := range cfg.Exclude {
			if strings.Contains(strings.ToLower(modelName), strings.ToLower(ex)) {
				output.Logger.Info("Skipping model (excluded)", "model", modelName, "url", url, "filter", ex)
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
			output.Logger.Error("Stream Inference Failed", "model", modelName, "url", url, "error", err)
		} else {
			output.Logger.Info("Stream Inference Success", "model", modelName, "url", url)
		}

		// B. Metric Tests (Configs)
		for _, inferCfg := range cfg.InferConfigs {
			output.Logger.Info("Running Inference Config", "model", modelName, "url", url, "config", inferCfg)

			res, err := e.Inference(url, modelName, cfg.Prompt, inferCfg)
			if err != nil {
				output.Logger.Error("Inference Benchmark Failed. Skipping remaining configs for this model.", "model", modelName, "url", url, "config", inferCfg, "error", err)
				res.Error = err.Error()

				// Attempt to capture VRAM Stats even on error (robustness)
				size, vram, vramErr := e.GetRunningModelInfo(url, modelName)
				if vramErr == nil && size > 0 {
					res.MemoryUsage = size
					res.VRAMUsage = vram
					res.VRAMPercentage = float64(vram) / float64(size) * 100.0
				}

				// Write partial result
				if err := csvWriter.Write(res); err != nil {
					output.Logger.Error("Failed to write partial result to CSV", "error", err)
				}
				if err := jsonWriter.Write(res); err != nil {
					output.Logger.Error("Failed to write partial result to JSON", "error", err)
				}
				break // Cruiser Protocol: Don't keep testing if the tree is rotting
			}

			// Capture VRAM Stats (Model is likely still loaded)
			size, vram, err := e.GetRunningModelInfo(url, modelName)
			if err == nil && size > 0 {
				res.MemoryUsage = size
				res.VRAMUsage = vram
				res.VRAMPercentage = float64(vram) / float64(size) * 100.0
			}

			if res.TokensGenerated == 0 {
				output.Logger.Warn("Model returned success but generated 0 tokens. Context limit exceeded?", "model", modelName)
			}

			output.Logger.Info("Inference Success",
				"model", modelName,
				"url", url,
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
