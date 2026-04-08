---
phase: 03-multi-language-and-skills
plan: 01
subsystem: language-registry
tags: [lsp, language-server, registry, yaml, installer, multi-language]

requires:
  - phase: 02-code-intelligence-kernel
    provides: "LS pool with quirks.go pattern for language-specific settings"
provides:
  - "52 embedded language server entries compiled into binary"
  - "Registry with YAML overlay for user customization"
  - "Three-tier LS installer (PATH, download, error)"
  - "ByExtension file-type detection for language auto-selection"
affects: [03-02, 03-03, lspool, kernel]

tech-stack:
  added: [gopkg.in/yaml.v3]
  patterns: [embedded-registry-with-yaml-overlay, three-tier-resolution]

key-files:
  created:
    - internal/langregistry/entry.go
    - internal/langregistry/languages.go
    - internal/langregistry/registry.go
    - internal/langregistry/registry_test.go
    - internal/langregistry/installer.go
    - internal/langregistry/installer_test.go

key-decisions:
  - "Used gopkg.in/yaml.v3 for YAML override parsing (already in go.mod as indirect dep)"
  - "52 language entries ported from legacy Python adapters with accurate commands, args, and file extensions"
  - "Deep merge semantics for YAML overrides: only specified fields are replaced"
  - "pipx preferred over pip for Python LS installs (safer global installs)"

patterns-established:
  - "Embedded registry pattern: defaultEntries map + YAML overlay via NewRegistry(overridePaths...)"
  - "Three-tier resolution: PATH lookup -> managed download -> helpful error"
  - "Platform key format: runtime.GOOS + '-' + runtime.GOARCH"

requirements-completed: [LNG-01, LNG-02]

duration: 3min
completed: 2026-04-08
---

# Phase 03 Plan 01: Language Registry Summary

**52-language embedded registry with YAML deep-merge overlay and three-tier LS installer (PATH/download/error)**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-08T09:00:03Z
- **Completed:** 2026-04-08T09:03:57Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- 52 language server entries compiled into binary covering all legacy Python adapters
- Registry supports YAML override with deep-merge semantics (preserves unspecified fields)
- Three-tier installer: PATH lookup first, managed download fallback (npm/pip/cargo/gem/dotnet/binary), helpful error last
- ByExtension lookup enables file-type-based language auto-detection

## Task Commits

Each task was committed atomically:

1. **Task 1: LSEntry types, embedded 52-language registry, and YAML overlay** - `551ec547` (feat)
2. **Task 2: Three-tier LS installer (PATH, download, error)** - `31be3139` (feat)

## Files Created/Modified
- `internal/langregistry/entry.go` - LSEntry and InstallInfo types with InstallHint()
- `internal/langregistry/languages.go` - 52 embedded defaultEntries (LOW/MEDIUM/HIGH quirk groups)
- `internal/langregistry/registry.go` - Registry with NewRegistry(), Get(), Languages(), ByExtension(), YAML merge
- `internal/langregistry/registry_test.go` - Tests for defaults count, lookups, extensions, YAML deep merge
- `internal/langregistry/installer.go` - Installer with Resolve() implementing three-tier strategy
- `internal/langregistry/installer_test.go` - Tests for PATH resolution, disabled auto-install, install hints

## Decisions Made
- Used gopkg.in/yaml.v3 for YAML parsing (already available as indirect dependency)
- Ported all 52 legacy adapters to Go structs with accurate command/args/file extensions
- Deep merge for YAML overrides avoids Pitfall 6 (losing unspecified fields)
- pipx preferred over pip for Python LS managed installs

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None - all entries have complete command, args, and file extension data.

## Next Phase Readiness
- Registry ready for consumption by LS pool (03-02 quirk adapter expansion)
- Installer ready for lazy resolution when workspaces activate languages
- YAML override paths (.serena/languages.yaml, ~/.serena/languages.yaml) ready for integration with config system

---
*Phase: 03-multi-language-and-skills*
*Completed: 2026-04-08*
