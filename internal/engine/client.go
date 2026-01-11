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
	// Cruiser Note: We use a custom transport to differentiate between
	// connection timeout and the server hanging during headers (e.g., model loading).
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// ResponseHeaderTimeout covers the time until we receive the first response byte
	// (Step 3: Headers). This is where model loading happens.
	transport.ResponseHeaderTimeout = cfg.LoadTimeout

	return &Engine{
		Config: cfg,
		Client: &http.Client{
			Transport: transport,
			// The overall timeout must cover Loading + Generation
			Timeout: cfg.LoadTimeout + (cfg.StreamTimeout * 2),
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

// monitorLoading polls /api/ps during the loading phase to ensure model placement
// adheres to the configured GPU/CPU guards.
func (e *Engine) monitorLoading(ctx context.Context, baseURL, modelName string, abort chan<- error, cancel context.CancelFunc) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			size, sizeVRAM, err := e.GetRunningModelInfo(baseURL, modelName)
			if err != nil {
				// Don't fail the monitor just because ps failed once (race condition during load)
				continue
			}

			// Model not found yet or just started loading
			if size == 0 {
				continue
			}

			// 100% CPU Check
			if sizeVRAM == 0 && !e.Config.CPUOnlyAllowed {
				select {
				case abort <- fmt.Errorf("ABORT: Model loaded 100%% on CPU (cpu_only_allowed=false)"):
					cancel()
				default:
				}
				return
			}

			// Split Load Check (any part on CPU)
			if sizeVRAM < size && e.Config.GPUOnly {
				select {
				case abort <- fmt.Errorf("ABORT: Model is partially on CPU (gpu_only=true)"):
					cancel()
				default:
				}
				return
			}
		}
	}
}

// StreamInference runs a streaming inference request.
func (e *Engine) StreamInference(baseURL, modelName, prompt string) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":      modelName,
		"prompt":     prompt,
		"stream":     true,
		"keep_alive": e.Config.KeepAlive,
	})

	// Setup Trace
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			output.Logger.Info("Network: Connected", "remote", connInfo.Conn.RemoteAddr(), "reused", connInfo.Reused)
		},
		WroteRequest: func(w httptrace.WroteRequestInfo) {
			output.Logger.Info("Network: Request Sent. Waiting for model to load...", "model", modelName)
		},
		GotFirstResponseByte: func() {
			output.Logger.Info("Network: First Byte Received", "model", modelName)
		},
	}

	// The context timeout must cover both the Load phase and the Generation phase.
	ctx, cancel := context.WithCancel(context.Background())
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, e.Config.LoadTimeout+e.Config.StreamTimeout)
	defer cancel()
	defer timeoutCancel()

	ctx = timeoutCtx // Use the timeout-wrapped context

	// Launch Loading Monitor
	abort := make(chan error, 1)
	go e.monitorLoading(ctx, baseURL, modelName, abort, cancel)

	ctx = httptrace.WithClientTrace(ctx, trace)

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/generate", baseURL), bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// Retry loop
	var lastErr error
	for i := 0; i < e.Config.MaxRetries; i++ {
		// Check for specific abort error before retrying
		select {
		case err := <-abort:
			return err
		default:
		}

		if i > 0 {
			time.Sleep(e.Config.RetryDelay)
			output.Logger.Info("Retrying streaming...", "attempt", i+1)
		}

		// Re-create request body reader for retry
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))

		resp, err := e.Client.Do(req)
		if err != nil {
			// Check for specific abort error before classifying as network error
			select {
			case abortErr := <-abort:
				return abortErr
			default:
			}

			if strings.Contains(err.Error(), "awaiting headers") {
				lastErr = fmt.Errorf("Ollama Header Timeout (model loading?): %w", err)
			} else {
				lastErr = fmt.Errorf("Network/Connection Error: %w", err)
			}
			continue
		}

		// Process Stream
		success := e.processStream(resp.Body)
		resp.Body.Close()

		if success {
			return nil
		}
		lastErr = fmt.Errorf("stream incomplete or failed to start")
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
		"model":      modelName,
		"prompt":     prompt,
		"stream":     false,
		"options":    extraConfig,
		"keep_alive": e.Config.KeepAlive,
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
			output.Logger.Info("Retrying inference...", "attempt", i+1)
		}

		finished, resData, abortErr, loopErr := func() (bool, model.Result, error, error) {
			ctx, cancel := context.WithCancel(context.Background())
			timeoutCtx, timeoutCancel := context.WithTimeout(ctx, e.Config.LoadTimeout+e.Config.StreamTimeout)
			defer timeoutCancel()
			defer cancel()

			// Launch Loading Monitor
			abort := make(chan error, 1)
			go e.monitorLoading(timeoutCtx, baseURL, modelName, abort, cancel)

			req, err := http.NewRequestWithContext(timeoutCtx, "POST", fmt.Sprintf("%s/api/generate", baseURL), bytes.NewBuffer(reqBody))
			if err != nil {
				return false, model.Result{}, nil, err
			}
			req.Header.Set("Content-Type", "application/json")

			output.Logger.Info("Network: Request Sent. Waiting for model to load...", "model", modelName)
			resp, err := e.Client.Do(req)
			if err != nil {
				// Check for specific abort error before classifying as network error
				select {
				case abortErr := <-abort:
					return false, model.Result{}, abortErr, nil
				default:
				}

				// Cruiser Protocol: Classify specific network errors
				message := err.Error()
				if strings.Contains(message, "awaiting headers") {
					return false, model.Result{}, nil, fmt.Errorf("Ollama Header Timeout (model loading?): %w", err)
				}
				return false, model.Result{}, nil, fmt.Errorf("Network/Connection Error: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return false, model.Result{}, nil, fmt.Errorf("Ollama Server Error (%s): %s", resp.Status, string(body))
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
				Error              string `json:"error"`         // API-side error
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return false, model.Result{}, nil, fmt.Errorf("failed to read response body: %w", err)
			}

			if err := json.Unmarshal(bodyBytes, &data); err != nil {
				return false, model.Result{}, nil, fmt.Errorf("Ollama returned invalid JSON: %w (Body: %s)", err, string(bodyBytes))
			}

			if data.Error != "" {
				return false, model.Result{}, nil, fmt.Errorf("Ollama API Error: %s", data.Error)
			}

			// Success
			return true, model.Result{
				Model:              modelName,
				URL:                baseURL,
				Config:             extraConfig,
				Timestamp:          start,
				Response:           data.Response,
				TotalDuration:      time.Duration(data.TotalDuration),
				LoadDuration:       time.Duration(data.LoadDuration),
				PromptEvalCount:    data.PromptEvalCount,
				PromptEvalDuration: time.Duration(data.PromptEvalDuration),
				EvalCount:          data.EvalCount,
				EvalDuration:       time.Duration(data.EvalDuration),
			}, nil, nil
		}()

		if abortErr != nil {
			return model.Result{}, abortErr
		}
		if finished {
			resData.Duration = time.Since(start) // Calculate overall duration for the successful attempt
			resData.TokensGenerated = resData.EvalCount
			resData.TokensReturned = len(strings.Split(resData.Response, " "))
			return resData, nil
		}
		lastErr = loopErr
	}

	res.Error = lastErr.Error()
	return res, lastErr
}
