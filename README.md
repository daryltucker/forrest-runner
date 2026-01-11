# Forest Runner

**Forest Runner** is a production-grade benchmarking tool for Ollama fleets. It validates model availability, streaming capabilities, and performs detailed latency/throughput analysis (VRAM, Token Speed, Load Time).

## Quick Start

### Install via Go

```bash
go install github.com/daryltucker/forest-runner/cmd/forest-runner@latest
```

### Build from Source

```bash
git clone https://github.com/daryltucker/forest-runner
cd forest-runner
go build -o forest-runner ./cmd/forest-runner
```

```bash
go install ./cmd/forest-runner
```


### Run Benchmark (with exclusions)

```bash
./forest-runner run \
  --urls http://localhost:11434 \
  --output-dir ./results \
  --exclude "embed,rerank"
```

### Automated Result Versioning
Result output is automatically versioned to prevent data-loss (e.g., `model_results.json.1`).

## Configuration File

**./runner.yml** (loaded by default)

```yaml
# Forest Runner Configuration (Example)
urls:
  - "http://localhost:11434"

prompt: "Explain quantum entanglement to a 5-year-old."

output_dir: "./results"
output_file: "benchmark_results.csv"

# Timeouts & Retries
max_retries: 3
retry_delay: 2s
stream_timeout: 60s
load_timeout: 10m  # Time allowed for initial model load into VRAM

# Strict Hardware Guards
gpu_only: true           # If true, abort if model spills into System RAM (CPU)
cpu_only_allowed: false  # If false, abort if model loads 100% on CPU
keep_alive: 0            # "0" (immediate unload), "5m", "1h", etc.

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

## Finding Errors for Re-Tests

```bash
MODELS=$(vecq -s results/model_results.json -f failed_models.jq -r | paste -sd "," -)
./forest-runner run --models "$MODELS"
```

> Result output is automatically incremented to prevent data-loss


## Development

**Philosophy**: Config-as-Code, Hermetic Design, Auditability.

**Primary Agent**: [Cruiser](file:///.agent/AGENT_CRUISER.md) (Model Auditor)

```bash
go test ./...
```


---

## Examples


```bash
 [ main ✭ | ✔  ]
✔ 17:22 daryl@Sleipnir ~/Projects/NRG/forest_runner $ ./forest-runner run
time=2026-01-10T17:22:32.027-08:00 level=INFO msg="Discovering models..." url=http://localhost:11434
time=2026-01-10T17:22:32.088-08:00 level=INFO msg="Found models" url=http://localhost:11434 count=13
time=2026-01-10T17:22:32.088-08:00 level=INFO msg="Skipping model (excluded)" model=snowflake-arctic-embed:m-long-2K filter=embed
time=2026-01-10T17:22:32.088-08:00 level=INFO msg="Skipping model (excluded)" model=snowflake-arctic-embed:m-long-8K filter=embed
time=2026-01-10T17:22:32.088-08:00 level=INFO msg="Skipping model (excluded)" model=snowflake-arctic-embed:m-long filter=embed
time=2026-01-10T17:22:32.088-08:00 level=INFO msg="Skipping model (excluded)" model=qwen3-embedding:0.6b-q8_0-32K filter=embed
time=2026-01-10T17:22:32.088-08:00 level=INFO msg="Skipping model (excluded)" model=qwen3-embedding:0.6b-q8_0-8K filter=embed
time=2026-01-10T17:22:32.088-08:00 level=INFO msg="Skipping model (excluded)" model=qwen3-embedding:0.6b-q8_0 filter=embed
time=2026-01-10T17:22:32.088-08:00 level=INFO msg="Skipping model (excluded)" model=snowflake-arctic-embed2:568m-l-fp16-8K filter=embed
time=2026-01-10T17:22:32.088-08:00 level=INFO msg="Skipping model (excluded)" model=snowflake-arctic-embed2:568m-l-fp16 filter=embed
time=2026-01-10T17:22:32.088-08:00 level=INFO msg="Testing Model" model=huggingface.co/Qwen/Qwen2.5-Coder-0.5B-Instruct-GGUF:latest url=http://localhost:11434
time=2026-01-10T17:22:32.088-08:00 level=INFO msg="Network: Connected" remote=[::1]:11434 reused=true
time=2026-01-10T17:22:32.089-08:00 level=INFO msg="Network: Request Sent. Waiting for server..." model=huggingface.co/Qwen/Qwen2.5-Coder-0.5B-Instruct-GGUF:latest
time=2026-01-10T17:22:37.659-08:00 level=INFO msg="Network: First Byte Received" model=huggingface.co/Qwen/Qwen2.5-Coder-0.5B-Instruct-GGUF:latest
time=2026-01-10T17:22:39.832-08:00 level=INFO msg="Stream Inference Success" model=huggingface.co/Qwen/Qwen2.5-Coder-0.5B-Instruct-GGUF:latest
time=2026-01-10T17:22:39.832-08:00 level=INFO msg="Running Inference Config" model=huggingface.co/Qwen/Qwen2.5-Coder-0.5B-Instruct-GGUF:latest config=map[num_ctx:2048]
time=2026-01-10T17:22:41.625-08:00 level=INFO msg="Inference Success" model=huggingface.co/Qwen/Qwen2.5-Coder-0.5B-Instruct-GGUF:latest duration=1.79285393s tokens_gen=303 vram_pct=100.0%
time=2026-01-10T17:22:42.626-08:00 level=INFO msg="Running Inference Config" model=huggingface.co/Qwen/Qwen2.5-Coder-0.5B-Instruct-GGUF:latest config=map[num_ctx:4096]
time=2026-01-10T17:22:44.861-08:00 level=INFO msg="Inference Success" model=huggingface.co/Qwen/Qwen2.5-Coder-0.5B-Instruct-GGUF:latest duration=2.235099217s tokens_gen=331 vram_pct=100.0%
time=2026-01-10T17:22:45.865-08:00 level=INFO msg="Testing Model" model=huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF:latest url=http://localhost:11434
time=2026-01-10T17:22:45.865-08:00 level=INFO msg="Network: Connected" remote=[::1]:11434 reused=true
time=2026-01-10T17:22:45.866-08:00 level=INFO msg="Network: Request Sent. Waiting for server..." model=huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF:latest
time=2026-01-10T17:22:49.169-08:00 level=INFO msg="Network: First Byte Received" model=huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF:latest
time=2026-01-10T17:22:49.804-08:00 level=INFO msg="Stream Inference Success" model=huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF:latest
time=2026-01-10T17:22:49.804-08:00 level=INFO msg="Running Inference Config" model=huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF:latest config=map[num_ctx:2048]
time=2026-01-10T17:22:50.566-08:00 level=INFO msg="Inference Success" model=huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF:latest duration=761.401821ms tokens_gen=112 vram_pct=100.0%
time=2026-01-10T17:22:51.566-08:00 level=INFO msg="Running Inference Config" model=huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF:latest config=map[num_ctx:4096]
time=2026-01-10T17:22:52.653-08:00 level=INFO msg="Inference Success" model=huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF:latest duration=1.086676579s tokens_gen=181 vram_pct=100.0%
time=2026-01-10T17:22:53.654-08:00 level=INFO msg="Testing Model" model=gemma3n:e2b url=http://localhost:11434
time=2026-01-10T17:22:53.654-08:00 level=INFO msg="Network: Connected" remote=[::1]:11434 reused=true
time=2026-01-10T17:22:53.654-08:00 level=INFO msg="Network: Request Sent. Waiting for server..." model=gemma3n:e2b
time=2026-01-10T17:23:07.230-08:00 level=INFO msg="Network: First Byte Received" model=gemma3n:e2b
time=2026-01-10T17:23:36.788-08:00 level=INFO msg="Stream Inference Success" model=gemma3n:e2b
time=2026-01-10T17:23:36.789-08:00 level=INFO msg="Running Inference Config" model=gemma3n:e2b config=map[num_ctx:2048]
time=2026-01-10T17:24:09.117-08:00 level=INFO msg="Inference Success" model=gemma3n:e2b duration=32.323790609s tokens_gen=326 vram_pct=21.5%
time=2026-01-10T17:24:10.121-08:00 level=INFO msg="Running Inference Config" model=gemma3n:e2b config=map[num_ctx:4096]
time=2026-01-10T17:24:37.314-08:00 level=INFO msg="Inference Success" model=gemma3n:e2b duration=27.192841116s tokens_gen=280 vram_pct=21.5%
time=2026-01-10T17:24:38.316-08:00 level=INFO msg="Testing Model" model=qwen3:4b url=http://localhost:11434
time=2026-01-10T17:24:38.322-08:00 level=INFO msg="Network: Connected" remote=[::1]:11434 reused=true
time=2026-01-10T17:24:38.323-08:00 level=INFO msg="Network: Request Sent. Waiting for server..." model=qwen3:4b
time=2026-01-10T17:24:45.725-08:00 level=INFO msg="Network: First Byte Received" model=qwen3:4b
time=2026-01-10T17:25:10.008-08:00 level=INFO msg="Stream Inference Success" model=qwen3:4b
time=2026-01-10T17:25:10.008-08:00 level=INFO msg="Running Inference Config" model=qwen3:4b config=map[num_ctx:2048]
time=2026-01-10T17:25:30.925-08:00 level=INFO msg="Inference Success" model=qwen3:4b duration=20.916556519s tokens_gen=945 vram_pct=100.0%
time=2026-01-10T17:25:31.929-08:00 level=INFO msg="Running Inference Config" model=qwen3:4b config=map[num_ctx:4096]
time=2026-01-10T17:25:52.262-08:00 level=INFO msg="Inference Success" model=qwen3:4b duration=20.324023875s tokens_gen=914 vram_pct=100.0%
time=2026-01-10T17:25:53.265-08:00 level=INFO msg="Testing Model" model=gemma3:4b-it-q8_0 url=http://localhost:11434
time=2026-01-10T17:25:53.266-08:00 level=INFO msg="Network: Connected" remote=[::1]:11434 reused=true
time=2026-01-10T17:25:53.266-08:00 level=INFO msg="Network: Request Sent. Waiting for server..." model=gemma3:4b-it-q8_0
time=2026-01-10T17:26:02.238-08:00 level=INFO msg="Network: First Byte Received" model=gemma3:4b-it-q8_0
time=2026-01-10T17:26:42.834-08:00 level=INFO msg="Stream Inference Success" model=gemma3:4b-it-q8_0
time=2026-01-10T17:26:42.834-08:00 level=INFO msg="Running Inference Config" model=gemma3:4b-it-q8_0 config=map[num_ctx:2048]
time=2026-01-10T17:27:01.306-08:00 level=INFO msg="Inference Success" model=gemma3:4b-it-q8_0 duration=18.470814434s tokens_gen=283 vram_pct=54.3%
time=2026-01-10T17:27:02.308-08:00 level=INFO msg="Running Inference Config" model=gemma3:4b-it-q8_0 config=map[num_ctx:4096]
time=2026-01-10T17:27:19.331-08:00 level=INFO msg="Inference Success" model=gemma3:4b-it-q8_0 duration=17.021946078s tokens_gen=270 vram_pct=54.3%
```

```bash
 [ main ✭ | ✔  ]
✔ 17:27 daryl@Sleipnir ~/Projects/NRG/forest_runner $ vecq -s -r -f summary.jq ./results/model_results.json | column -t
Model                      VRAM(MB)  GPU%               Tk/s   Options
-----                      --------  ----               ----   -------
Qwen/Qwen2.5-Coder-0.5B-I  720.0     100                273.0  map[num_ctx:2048]
Qwen/Qwen2.5-Coder-0.5B-I  720.0     100                271.0  map[num_ctx:4096]
Qwen/Qwen2.5-0.5B-Instruc  720.0     100                266.0  map[num_ctx:2048]
Qwen/Qwen2.5-0.5B-Instruc  720.0     100                275.0  map[num_ctx:4096]
gemma3n:e2b                1292.0    21.50509128001501  35.0   map[num_ctx:2048]
gemma3n:e2b                1292.0    21.50509128001501  34.0   map[num_ctx:4096]
qwen3:4b                   3406.0    100                55.0   map[num_ctx:2048]
qwen3:4b                   3406.0    100                55.0   map[num_ctx:4096]
gemma3:4b-it-q8_0          3196.0    54.32037352889163  15.0   map[num_ctx:2048]
gemma3:4b-it-q8_0          3196.0    54.32037352889163  15.0   map[num_ctx:4096]
```
