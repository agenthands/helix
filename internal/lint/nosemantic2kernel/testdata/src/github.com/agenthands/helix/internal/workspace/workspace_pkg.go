// Stub so importers under testdata can resolve "github.com/agenthands/helix/internal/workspace".
// Phase 61 ENRICH-01: internal/workspace is outside the forbidden-import prefix
// (internal/kernel/...) so it falls outside the analyzer's check entirely — no
// dedicated carve-out is required.
package workspace
