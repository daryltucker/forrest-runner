# Header row
(["Model", "VRAM(MB)", "GPU%", "Tk/s", "Options"] | (., map(length*"-"))),

# Data rows
(.[] | [
  # Trim model name for readability (e.g., huggingface.co/Qwen/... -> Qwen/...)
  (.model | sub(".*huggingface.co/"; "") | .[0:25]),
  
  # VRAM in MB
  (.vram_usage_bytes/1048576 | floor),
  
  # GPU Offload Percentage
  .vram_percentage,
  
  # Tokens per Second (Generative)
  ((.eval_count / (.eval_duration/1000000000)) | floor),

  # Config Options (Compact JSON)
  (.config | tostring)
]) | @tsv
