/*
PURPOSE:
  Core engine for interacting with Ollama APIs.
  Handles model discovery, streaming inference, and benchmarking.

REQUIREMENTS:
  User-specified:
  - Detect models.
  - Stream inference (with timeout and garbage resilience).
  - Non-stream inference (metrics).

  Implementation-discovered:
  - Needs http.Client with timeouts.
  - Resilience against "garbage" JSON (invalid chunks).

ARCHITECTURE INTEGRATION:
  - Called by: internal/cli
  - Uses: internal/config, internal/model, internal/output

ERROR HANDLING:
  - Retries are handled at a higher level (Runner) or here?
  - Plan says "retry logic" is needed. Let's put basic http handling here, Runner can do high-level retries.
  - Actually, the original script had retries *inside* the inference functions. I will stick to that pattern for robustness.

IMPLEMENTATION RULES:
  - Use net/http.
  - Enforce timeouts.
  - Parse streaming JSON line-by-line.

USAGE:
  e := engine.New(cfg)
  models, err := e.GetModels(url)
  err := e.StreamInference(url, model, prompt)

SELF-HEALING INSTRUCTIONS:
  - If Ollama API changes, update endpoints (/api/tags, /api/generate).

RELATED FILES:
  - internal/config/config.go
  - internal/model/types.go

MAINTENANCE:
  - Update for new Ollama API features.
*/

package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"

	"github.com/daryltucker/forest-runner/internal/config"
	"github.com/daryltucker/forest-runner/internal/model"
	"github.com/daryltucker/forest-runner/internal/output"
)

// Engine handles Ollama interactions.
type Engine struct {
	Config *config.Config
	Client *http.Client
}

// New creates a new Engine.
func New(cfg *config.Config) *Engine {
	return &Engine{
		Config: cfg,
		Client: &http.Client{
			Timeout: cfg.StreamTimeout + 10*time.Second, // Safety buffer
		},
	}
}

// GetModels returns a list of available models from an Ollama host.
func (e *Engine) GetModels(baseURL string) ([]string, error) {
	resp, err := e.Client.Get(fmt.Sprintf("%s/api/tags", baseURL))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	var names []string
	for _, m := range payload.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

// GetRunningModelInfo retrieves memory stats for a running model from /api/ps.
func (e *Engine) GetRunningModelInfo(baseURL, modelName string) (int64, int64, error) {
	resp, err := e.Client.Get(fmt.Sprintf("%s/api/ps", baseURL))
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("bad status: %s", resp.Status)
	}

	var payload struct {
		Models []struct {
			Name     string `json:"name"`
			Size     int64  `json:"size"`
			SizeVRAM int64  `json:"size_vram"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, 0, err
	}

	for _, m := range payload.Models {
		// Loosely match model name or exact match
		if m.Name == modelName || strings.HasPrefix(m.Name, modelName) {
			return m.Size, m.SizeVRAM, nil
		}
	}

	return 0, 0, nil // Not found (might have unloaded?)
}

// StreamInference runs a streaming inference request.
func (e *Engine) StreamInference(baseURL, modelName, prompt string) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":  modelName,
		"prompt": prompt,
		"stream": true,
	})

	// Setup Trace
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			output.Logger.Info("Network: Connected", "remote", connInfo.Conn.RemoteAddr(), "reused", connInfo.Reused)
		},
		WroteRequest: func(w httptrace.WroteRequestInfo) {
			output.Logger.Info("Network: Request Sent. Waiting for server...", "model", modelName)
		},
		GotFirstResponseByte: func() {
			output.Logger.Info("Network: First Byte Received", "model", modelName)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.Config.StreamTimeout)
	defer cancel()

	ctx = httptrace.WithClientTrace(ctx, trace)

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/generate", baseURL), bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Retry loop? The Python script did retries. Let's do it here.
	// Actually, let's keep this function simple: "Try once". The caller (Runner) can loop retries if desired,
	// OR we implement retry here. The logic in Python was: retry inside function.

	// Refactoring: Let's do a simple retry wrapper or loop here.
	var lastErr error
	for i := 0; i < e.Config.MaxRetries; i++ {
		if i > 0 {
			time.Sleep(e.Config.RetryDelay)
			output.Logger.Info("Retrying streaming...", "attempt", i+1)
		}

		// Re-create request body reader for retry
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))

		resp, err := e.Client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Process Stream
		success := e.processStream(resp.Body)
		resp.Body.Close()

		if success {
			return nil
		}
		lastErr = fmt.Errorf("stream incomplete")
	}

	return lastErr
}

func (e *Engine) processStream(body io.Reader) bool {
	scanner := bufio.NewScanner(body)
	gotDone := false

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}

		// Garbage resilience: Ignore JSON errors
		if err := json.Unmarshal(line, &chunk); err != nil {
			output.Logger.Warn("Skipping invalid JSON chunk", "chunk", string(line))
			continue
		}

		if chunk.Response != "" {
			// In interactive mode we might print, but for now we just verify flow
		}

		if chunk.Done {
			gotDone = true
			break // Successfully finished
		}
	}

	if err := scanner.Err(); err != nil {
		output.Logger.Warn("Stream scanning error", "err", err)
		return false
	}

	return gotDone
}

// Inference runs a non-streaming benchmark.
func (e *Engine) Inference(baseURL, modelName, prompt string, extraConfig map[string]interface{}) (model.Result, error) {
	start := time.Now()

	payload := map[string]interface{}{
		"model":  modelName,
		"prompt": prompt,
		"stream": false,
	}
	// Merge extra config
	for k, v := range extraConfig {
		payload[k] = v
	}

	reqBody, _ := json.Marshal(payload)
	// Result structure to populate
	res := model.Result{
		Model:     modelName,
		URL:       baseURL,
		Config:    extraConfig, // Assign directly
		Timestamp: start,
	}

	// Retry loop
	var lastErr error
	for i := 0; i < e.Config.MaxRetries; i++ {
		if i > 0 {
			time.Sleep(e.Config.RetryDelay)
		}

		resp, err := e.Client.Post(fmt.Sprintf("%s/api/generate", baseURL), "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("bad status: %s", resp.Status)
			continue
		}

		var data struct {
			Response           string `json:"response"`
			Done               bool   `json:"done"`
			TotalDuration      int64  `json:"total_duration"` // ns
			LoadDuration       int64  `json:"load_duration"`  // ns
			PromptEvalCount    int    `json:"prompt_eval_count"`
			PromptEvalDuration int64  `json:"prompt_eval_duration"` // ns
			EvalCount          int    `json:"eval_count"`
			EvalDuration       int64  `json:"eval_duration"` // ns
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &data); err != nil {
			lastErr = err
			continue
		}

		// Calculate metrics
		res.Duration = time.Since(start)

		res.TotalDuration = time.Duration(data.TotalDuration)
		res.LoadDuration = time.Duration(data.LoadDuration)
		res.PromptEvalCount = data.PromptEvalCount
		res.PromptEvalDuration = time.Duration(data.PromptEvalDuration)
		res.EvalCount = data.EvalCount
		res.EvalDuration = time.Duration(data.EvalDuration)

		res.TokensGenerated = data.EvalCount // Use official count
		res.TokensReturned = len(strings.Split(data.Response, " "))
		res.Response = data.Response

		return res, nil
	}

	res.Error = lastErr.Error()
	return res, lastErr
}
