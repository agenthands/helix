// Package patch contains fixups for LSP metamodel quirks.
//
// The metamodel marks RenameParams.newName as optional, but in practice
// it is always required by language servers. This patch documents the
// discrepancy; the generated types use the metamodel as-is, and callers
// should always provide newName when constructing RenameParams.
package patch

// RenameParamsNote documents the metamodel quirk:
// The "newName" field in RenameParams is marked optional in the metamodel
// but is actually required for all known language server implementations.
const RenameParamsNote = "RenameParams.newName is effectively required despite metamodel marking it optional"
