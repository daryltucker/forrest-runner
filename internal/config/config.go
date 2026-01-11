/*
PURPOSE:
  Defines the configuration structure and loading logic for Forest Runner.
  Adheres to "Config IS Code" philosophy.

REQUIREMENTS:
  User-specified:
  - Allow configuration of target URLs, timeouts, and prompts.

  Implementation-discovered:
  - Needs to support YAML parsing.
  - Needs to support Environment variables overrides (FOREST_...).

ARCHITECTURE INTEGRATION:
  - Used by: internal/cli, internal/engine
  - Dependencies: gopkg.in/yaml.v3 (standard for Go config)

ERROR HANDLING:
  - Returns explicit error if config file is invalid.
  - Returns optional error if config file is missing (might fall back to defaults).

IMPLEMENTATION RULES:
  - Config struct tags should support yaml.
  - Defaults should be sensible (e.g., 60s timeout).

USAGE:
  cfg, err := config.Load("forest_runner.yaml")

SELF-HEALING INSTRUCTIONS:
  - If new fields are needed, add to Config struct and update Load() defaults.

RELATED FILES:
  - internal/cli/root.go

MAINTENANCE:
  - Update when adding new tuning parameters.
*/

package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the full configuration for Forest Runner.
type Config struct {
	URLs          []string      `yaml:"urls"`
	Prompt        string        `yaml:"prompt"`
	OutputDir     string        `yaml:"output_dir"`
	OutputFile    string        `yaml:"output_file"` // Deprecated? Or just filename? Let's keep for filename base.
	MaxRetries    int           `yaml:"max_retries"`
	RetryDelay    time.Duration `yaml:"retry_delay"`
	StreamTimeout time.Duration `yaml:"stream_timeout"`
	// Exclude is a list of strings to filter model names (substring match)
	Exclude []string `yaml:"exclude"`
	// InferConfigs allows defining multiple inference configurations
	InferConfigs []map[string]interface{} `yaml:"inference_configs"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		URLs:          []string{"http://localhost:11434"},
		Prompt:        "What is the capital of France?",
		OutputDir:     ".",
		OutputFile:    "model_results.csv",
		MaxRetries:    3,
		RetryDelay:    2 * time.Second,
		StreamTimeout: 60 * time.Second,
		Exclude:       []string{"embed", "rerank"},
		InferConfigs: []map[string]interface{}{
			{"num_ctx": 2048},
			{"num_ctx": 4096},
		},
	}
}

// Load reads configuration from a file.
// If path is specified, it attempts to load that file.
// If path is empty, it searches for default files in order.
// If no file found, returns default config.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	var data []byte
	var err error

	if path != "" {
		data, err = os.ReadFile(path)
		if err != nil {
			return cfg, err
		}
	} else {
		// Search for defaults
		defaults := []string{"runner.yaml", "runner.conf", "forest_runner.yaml"}
		found := false
		for _, name := range defaults {
			data, err = os.ReadFile(name)
			if err == nil {
				path = name // record which file we loaded
				found = true
				break
			}
		}
		if !found {
			// No config file found, return default
			return cfg, nil
		}
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	return cfg, nil
}
