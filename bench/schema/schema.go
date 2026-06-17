// Package schema owns the versioned bench result contract
// (result.v2.schema.json) and exposes its bytes for validate-on-write callers
// that must not depend on the process working directory.
//
// The schema file is embedded at build time so any package (e.g.
// bench/runtime's result.v2 builder) can compile + validate against the exact
// committed contract without a cwd-relative file read.
package schema

import _ "embed"

// ResultV2SchemaBytes is the embedded result.v2.schema.json contract document.
// It is the single source of truth for result.v2 validation; consumers compile
// it with jsonschema/v6 under Draft 2020-12 (see result.v2_test.go for the
// canonical compile pattern).
//
//go:embed result.v2.schema.json
var ResultV2SchemaBytes []byte

// ResultV2SchemaID is the logical resource id used when adding the embedded
// schema to a jsonschema compiler. It mirrors the schema's own $id so error
// messages reference a stable name rather than a transient path.
const ResultV2SchemaID = "result.v2.schema.json"
