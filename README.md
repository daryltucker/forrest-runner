# Forest Runner

**Forest Runner** is a production-grade benchmarking tool for Ollama fleets. It validates model availability, streaming capabilities, and performs detailed latency/throughput analysis (VRAM, Token Speed, Load Time).

## Quick Start

### Install via Go

```bash
go install github.com/daryltucker/forest-runner/cmd/forest-runner@latest
```

### Build from Source

```bash
# 1. Clone & Build
git clone https://github.com/daryltucker/forest-runner
cd forest-runner
go build -o forest-runner ./cmd/forest-runner
```

### Run Benchmark (with exclusions)

```bash
./forest-runner run \
  --urls http://localhost:11434 \
  --output-dir ./results \
  --exclude "embed,rerank"
```

## Configuration File

**./runner.yml** (loaded by default)

```yaml
# Forest Runner Configuration (Example)
urls:
  - "http://localhost:11434"
  - "http://localhost:11435"

prompt: "Explain quantum entanglement to a 5-year-old."

output_dir: "./results"
output_file: "benchmark_results.csv"

max_retries: 3
retry_delay: 2s
stream_timeout: 60s

exclude:
  - "embed"
  - "rerank"

inference_configs:
  - num_ctx: 2048
  - num_ctx: 4096
    temperature: 0.7
```

## Viewing Results

Results are saved as **CSV** (for spreadsheets) and **JSON** (for programmatic analysis).

### Detailed Summary Table
Use `vecq` to generate a clean table of Virtual Memory (VRAM) and Token Generation Speed:

```bash
vecq -s -r -f summary.jq ./results/model_results.json | column -t
```
*Output Example:*
```text
Model                  VRAM(MB)  GPU%  Tk/s  Options
-----                  --------  ----  ----  -------
Qwen/Qwen2.5-Coder...  720       100   273   {"num_ctx":2048}
llama3:8b              5120      100   120   {"num_ctx":4096,"temp":0.7}
```

### Quick Checks
**Find Errors**:
```bash
vecq -s ./results/model_results.json -q 'map(select(.error != null and .error != ""))'
```

**Check VRAM Offload**:
```bash
vecq -s ./results/model_results.json -q '.[] | select(.vram_gpu_pct > 0) | {model: .model, mb: (.vram_usage_bytes/1024/1024)|floor}'
```


## Development

**Philosophy**: Config-as-Code, Hermetic Design, Auditability.

```bash
go test ./...
```
