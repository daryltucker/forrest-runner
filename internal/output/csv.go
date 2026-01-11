/*
PURPOSE:
  Writes benchmark results to a CSV file.
  Ensures data integrity by flushing writes immediately.

REQUIREMENTS:
  User-specified:
  - Output to CSV.
  - Keep file handle open for flushing (per original Python script).

  Implementation-discovered:
  - Needs to create file if not exists, or truncate if new run?
  - Original script used `unlink` then `open("w")`, implying overwrite.

ARCHITECTURE INTEGRATION:
  - Called by: internal/engine
  - Consumes: internal/model.Result

ERROR HANDLING:
  - Returns error on file creation or write failure.

IMPLEMENTATION RULES:
  - Use encoding/csv.
  - Flush() after every write (critical for crash resilience).
  - Use Mutex if concurrent writes are expected (Engine might be parallel).

USAGE:
  w, err := output.NewCSVWriter("results.csv")
  w.Write(result)
  w.Close()

SELF-HEALING INSTRUCTIONS:
  - If CSV format changes, update header and record conversion.

RELATED FILES:
  - internal/model/types.go

MAINTENANCE:
  - Update Write() mapping when Result struct changes.
*/

package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/daryltucker/forest-runner/internal/model"
)

// CSVWriter handles writing results to a CSV file.
type CSVWriter struct {
	file   *os.File
	writer *csv.Writer
	mu     sync.Mutex
}

// NewCSVWriter creates a new CSVWriter.
// It overwrites the file if it exists.
func NewCSVWriter(path string) (*CSVWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	w := csv.NewWriter(f)

	// Write Header
	// Write Header
	header := []string{
		"model", "url", "config", "timestamp", "client_duration_s",
		"total_duration_s", "load_duration_s", "prompt_eval_s", "eval_duration_s",
		"prompt_tokens", "gen_tokens", "tokens_returned",
		"vram_usage_mb", "vram_gpu_pct",
		"response", "error",
	}
	if err := w.Write(header); err != nil {
		f.Close()
		return nil, err
	}
	w.Flush()

	return &CSVWriter{
		file:   f,
		writer: w,
	}, nil
}

// Write writes a single result to the CSV file.
// It is thread-safe.
func (cw *CSVWriter) Write(r model.Result) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	configBytes, _ := json.Marshal(r.Config)
	configStr := string(configBytes)

	record := []string{
		r.Model,
		r.URL,
		configStr,
		r.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		fmt.Sprintf("%.4f", r.Duration.Seconds()),
		fmt.Sprintf("%.4f", r.TotalDuration.Seconds()),
		fmt.Sprintf("%.4f", r.LoadDuration.Seconds()),
		fmt.Sprintf("%.4f", r.PromptEvalDuration.Seconds()),
		fmt.Sprintf("%.4f", r.EvalDuration.Seconds()),
		fmt.Sprintf("%d", r.PromptEvalCount),
		fmt.Sprintf("%d", r.TokensGenerated),
		fmt.Sprintf("%d", r.TokensReturned),
		fmt.Sprintf("%.2f", float64(r.VRAMUsage)/1024/1024), // MB
		fmt.Sprintf("%.1f", r.VRAMPercentage),
		r.Response,
		r.Error,
	}

	if err := cw.writer.Write(record); err != nil {
		return err
	}
	cw.writer.Flush()
	return cw.writer.Error()
}

// Close closes the underlying file.
func (cw *CSVWriter) Close() error {
	cw.writer.Flush()
	return cw.file.Close()
}
