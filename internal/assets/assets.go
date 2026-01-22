package assets

import "embed"

// Functions contains the embedded JQ scripts for forest-runner
//
//go:embed functions/*.jq
var Functions embed.FS
