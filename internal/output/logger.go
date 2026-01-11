/*
PURPOSE:
  Provides a structured logger for Forest Runner.
  Wraps slog for consistent output.

REQUIREMENTS:
  User-specified:
  - "Sane" CLI output. Not spammy.

  Implementation-discovered:
  - Needs to support Info/Error levels.

ARCHITECTURE INTEGRATION:
  - Used everywhere.

ERROR HANDLING:
  - N/A

IMPLEMENTATION RULES:
  - Use `log/slog` (Go 1.21+).

USAGE:
  output.Logger.Info("message", "key", "value")

SELF-HEALING INSTRUCTIONS:
  - Ensure Go 1.21+ is used.

RELATED FILES:
  - All.

MAINTENANCE:
  - Configurable log levels?
*/

package output

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func init() {
	// Default generic logger.
	// In the future, we can configure this via CLI flags (e.g. JSON handler for non-interactive)
	Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
}

// SetLogger allows overriding the default logger (e.g. for testing or config changes)
func SetLogger(l *slog.Logger) {
	Logger = l
}
