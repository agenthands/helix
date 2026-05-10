// Package sandbox manages per-(task, mode) filesystem and environment isolation
// for the Phase 67 evaluation harness. Each mode receives a private HOME
// directory, daemon socket, and helix_config.yml so no cross-mode contamination
// occurs. Isolation mechanics follow the subprocess design in 67-RESEARCH.md
// §"Subprocess Isolation Design". Real implementation lands in Wave 1+.
package sandbox
