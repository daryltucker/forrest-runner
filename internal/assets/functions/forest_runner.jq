# Forest Runner JQ Library
#
# PURPOSE:
#   A unified library of JQ functions for analyzing Forest Runner benchmarks.
#   Replaces the individual file approach for cleaner namespace management.
#
# USAGE:
#   vecq -q 'include "forest_runner"; forest_summary' ...
#   (Assuming installed to ~/.config/vecq/functions/forest_runner.jq)

# --- forest_merge_results ---
# Merges multiple benchmark result files (NDJSON outputs), deduplicating by
# picking the "best" (success > failure) and most recent record for each
# Model+URL+Config combination.
def forest_merge_results:
  group_by([.model, .url, (.config | tostring)]) |
  map(
    sort_by([
      (if .error == null or .error == "" then 1 else 0 end),
      .timestamp
    ]) |
    last
  );

# --- forest_failed_models ---
# Returns a plain list of model names that failed in the dataset.
# Useful for creating retry lists:
# forest-runner run --models $(vecq ... | paste -sd "," -)
def forest_failed_models:
  map(select(.error != null and .error != "")) |
  map(.model) |
  unique |
  .[] ;

# --- forest_summary ---
# Generates a TSV-formatted summary table of the results.
# Automatically detects if a "Backend" column is needed (Multi-backend support).
def forest_summary:
  flatten(1) as $rows |
  ($rows | map(.url) | unique | length) as $url_count |

  # Header row
  if $url_count > 1 then
    (["Model", "Backend", "VRAM(MB)", "GPU%", "Tk/s", "Options"] | (., map(length*"-")))
  else
    (["Model", "VRAM(MB)", "GPU%", "Tk/s", "Options"] | (., map(length*"-")))
  end,

  # Data rows
  ($rows | map(select(.model != null)) | sort_by(.model, (.config | tostring), .url) | .[] | [
    # Trim model name for readability
    (.model | sub(".*huggingface.co/"; "") | .[0:25]),

    # Backend URL (only if multiple exist)
    (if $url_count > 1 then (.url | sub("https?://";"") | sub(":[0-9]+$";"") | .[0:15]) else empty end),

    # VRAM in MB
    (.vram_usage_bytes/1048576 | floor),

    # GPU Offload Percentage
    .vram_percentage,

    # Tokens per Second (Generative)
    (if .error and .error != "" then "ERROR" elif .eval_duration > 0 then ((.eval_count / (.eval_duration/1e9)) | floor) else 0 end),

    # Config Options (Compact JSON)
    (.config | tostring)
  ]) | @tsv;

# --- forest_compare ---
# Strict side-by-side comparison view (A/B Testing).
# Groups models together strictly and highlights differences.
# Currently identical to forest_summary but enforces the multi-backend view if relevant.
def forest_compare:
  forest_summary;
