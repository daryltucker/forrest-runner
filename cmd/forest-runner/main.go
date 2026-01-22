/*
PURPOSE:
  Entry point for the Forest Runner application.
  Initializes the CLI root command and executes it.

REQUIREMENTS:
  User-specified:
  - Must serve as the single binary entry point.
  - Must handle top-level errors gracefully.

  Implementation-discovered:
  - Uses cobra for CLI command management.
  - Needs to pass OS args / context to the CLI.

ARCHITECTURE INTEGRATION:
  - Calls: internal/cli.Execute()
  - Depends on: internal/cli package

ERROR HANDLING:
  - Top-level panic recovery (though discouraged in favor of explicit error handling).
  - Explicit error check on Execute(); exit code 1 on failure.

IMPLEMENTATION RULES:
  - Critical: Keep main() minimal. All logic belongs in internal/ packages.
  - Do not put business logic here.
  - Do not use global variables for state here.

USAGE:
  go build -o forest-runner ./cmd/forest-runner
  ./forest-runner [command] [flags]

SELF-HEALING INSTRUCTIONS:
  - If CLI fails to start, check internal/cli/root.go definition.
  - If imports fail, run `go mod tidy`.

RELATED FILES:
  - internal/cli/root.go - The actual root command definition.

MAINTENANCE:
  - Update when changing the CLI framework or high-level signal handling.
*/

package main

import (
	"fmt"
	"os"

	"github.com/daryltucker/forest-runner/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
