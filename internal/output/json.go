/*
PURPOSE:
  Writes benchmark results to a JSON Lines file (NDJSON).
  Optimized for machine parsing and `vecq` integration.

REQUIREMENTS:
  User-specified:
  - JSON output for easier parsing.

  Implementation-discovered:
  - JSON Lines is better for streaming/logging than a single large array (append-friendly).

ARCHITECTURE INTEGRATION:
  - Called by: internal/engine
  - Consumes: internal/model.Result

ERROR HANDLING:
  - Returns error on file creation or write failure.

IMPLEMENTATION RULES:
  - Use encoding/json.NewEncoder.
  - Thread-safe.

USAGE:
  w, err := output.NewJSONWriter("results.jsonl")
  w.Write(result)
  w.Close()

SELF-HEALING INSTRUCTIONS:
  - None specific.

RELATED FILES:
  - internal/model/types.go

MAINTENANCE:
  - Update if we switch to plain JSON array (not recommended for streaming).
*/

package output

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/daryltucker/forest-runner/internal/model"
)

// JSONWriter handles writing results to a JSON Lines file.
type JSONWriter struct {
	file    *os.File
	encoder *json.Encoder
	mu      sync.Mutex
}

// NewJSONWriter creates a new JSONWriter.
func NewJSONWriter(path string) (*JSONWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	return &JSONWriter{
		file:    f,
		encoder: json.NewEncoder(f),
	}, nil
}

// Write writes a single result as a JSON line.
func (jw *JSONWriter) Write(r model.Result) error {
	jw.mu.Lock()
	defer jw.mu.Unlock()

	return jw.encoder.Encode(r)
}

// Close closes the underlying file.
func (jw *JSONWriter) Close() error {
	return jw.file.Close()
}
