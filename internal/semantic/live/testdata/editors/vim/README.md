# Vim editor save fixture (LIVE-02)

`save.sh` reproduces Vim's default `backupcopy=auto` rename-on-save
sequence in pure POSIX shell. Source-of-truth for the event sequence:
`.planning/phases/60-live-update-pipeline/60-RESEARCH.md` "Editor
Save-Pattern Dossier — Vim — `backupcopy=auto` swap-rename".

## What it reproduces

```
CREATE   .${base}.tmp.$$
RENAME   ${base}        (original → backup)
RENAME   .${base}.tmp.$$ → ${base}
REMOVE   ${base}~
```

The watcher MUST observe at least one event resolving to `${base}`
after the rename completes. With directory-level watch + paths-only
signal (CONTEXT D-01), the events normalize correctly: the pending
path-set ends up `{${base}}` after debounce.

## Usage

```bash
./save.sh /path/to/auth.go "package main\n// modified\n"
```

## Platform support

Requires `bash` on PATH. The editor-tagged fixture test
(`editor_fixtures_test.go`, build tag `editor`) skips this fixture on
Windows where bash is unavailable. The JetBrains and VS Code fixtures
are pure Go and run on every platform.
