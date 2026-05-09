// Package catalogs embeds per-language security-sensitive import pattern catalogs
// for the G-005 security-sensitive rule (Phase 66 Plan 03, D-22).
//
// Catalogs ship as YAML files embedded via embed.FS. Each file name is the
// language key (e.g., "go.yaml", "typescript.yaml"). Use Load to decode a
// single language catalog and merge with operator config overrides.
package catalogs

import "embed"

//go:embed *.yaml
var EmbeddedCatalogs embed.FS
