/*
PURPOSE:
  Defines the core data structures used throughout Forest Runner.
  These models represent benchmark results and metrics.

REQUIREMENTS:
  User-specified:
  - Record duration, tokens generated, tokens returned.
  - Track model name, URL, config used.

  Implementation-discovered:
  - Need JSON tags for Vecq/API compatibility.
  - Need CSV tags or explicit mapping.

ARCHITECTURE INTEGRATION:
  - Used by: internal/engine, internal/output
  - Shared across boundaries.

ERROR HANDLING:
  - None (pure data structs).

IMPLEMENTATION RULES:
  - Keep structs simple and public.
  - Use time.Time and time.Duration for high precision.

USAGE:
  res := model.Result{...}

SELF-HEALING INSTRUCTIONS:
  - If new metrics are needed, add field and update CSV/JSON writers.

RELATED FILES:
  - internal/output/csv.go
  - internal/output/json.go

MAINTENANCE:
  - Update when adding new metrics to capture.
*/

package model

import (
	"time"
)

// Result represents the outcome of a single benchmark run.
type Result struct {
	Model              string                 `json:"model"`
	URL                string                 `json:"url"`
	Config             map[string]interface{} `json:"config"` // JSON object
	Timestamp          time.Time              `json:"timestamp"`
	Duration           time.Duration          `json:"duration"`
	TotalDuration      time.Duration          `json:"total_duration"` // Server-side
	LoadDuration       time.Duration          `json:"load_duration"`
	PromptEvalCount    int                    `json:"prompt_eval_count"`
	PromptEvalDuration time.Duration          `json:"prompt_eval_duration"`
	EvalCount          int                    `json:"eval_count"`
	EvalDuration       time.Duration          `json:"eval_duration"`

	// Resource Usage (from /api/ps)
	MemoryUsage    int64   `json:"memory_usage_bytes"` // Total size
	VRAMUsage      int64   `json:"vram_usage_bytes"`   // VRAM usage
	VRAMPercentage float64 `json:"vram_percentage"`    // VRAM / Total

	TokensGenerated int    `json:"tokens_generated"`
	TokensReturned  int    `json:"tokens_returned"`
	Response        string `json:"response,omitempty"` // Optional: full response text
	Error           string `json:"error,omitempty"`    // If the run failed
}
