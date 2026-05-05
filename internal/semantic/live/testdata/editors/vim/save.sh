#!/usr/bin/env bash
# save.sh — simulates Vim's default `backupcopy=auto` rename-on-save
# sequence (60-RESEARCH.md "Editor Save-Pattern Dossier" — Vim section).
#
# Verbatim file ops (POSIX rename branch):
#   1. write content to a sibling temp file (.${name}.tmp.$$)
#   2. rename the original target → ${name}~ (backup; if target exists)
#   3. rename the temp file → target
#   4. delete the backup
#
# fsnotify event sequence on a directory watch:
#   CREATE  .${name}.tmp.$$
#   RENAME  ${name}        (original moves to backup)
#   CREATE  ${name}~       (the backup, if retained)
#   RENAME  .${name}.tmp.$$ → ${name} (manifests as RENAME on tmp + CREATE on target)
#   REMOVE  ${name}~       (cleanup)
#
# Usage: save.sh <target-file> <new-content>
#
# Skipped on Windows CI (bash not on PATH); the JetBrains and VS Code
# fixtures are pure Go and run on every platform.

set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: save.sh <target-file> <new-content>" >&2
  exit 2
fi

target="$1"
content="$2"
dir="$(dirname "$target")"
base="$(basename "$target")"
tmp="$dir/.${base}.tmp.$$"
backup="$dir/${base}~"

printf '%s' "$content" > "$tmp"

if [[ -f "$target" ]]; then
  mv "$target" "$backup"
fi

mv "$tmp" "$target"

# Cleanup the backup; some Vim configs retain it, others remove it.
# We remove it to mirror the most common `:set nobackup` workflow.
rm -f "$backup"
