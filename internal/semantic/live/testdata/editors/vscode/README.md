# VS Code editor save fixtures (LIVE-02)

Two pure-Go programs cover the two paths the VS Code editor takes on
file save. Source-of-truth:
`.planning/phases/60-live-update-pipeline/60-RESEARCH.md` "Editor
Save-Pattern Dossier — VS Code".

## `save_atomic.go`

Mode triggered by `files.atomicSave: true` (default on remote-FS
extensions). Writes content to a sibling temp file (`.vsctmp~XXXXXX`)
then renames it onto the target.

```
CREATE  .vsctmp~XXXXXX
WRITE   .vsctmp~XXXXXX
RENAME  .vsctmp~XXXXXX → target
```

This is the canonical Pitfall 1 mitigation evidence — directory-level
watch survives the rename onto the target file.

## `save_truncate.go`

Default save mode: open the target with `O_TRUNC|O_WRONLY|O_CREATE`,
write, close. Simplest possible path — no rename, no temp file.

```
WRITE  target
```

## Usage

```bash
go run save_atomic.go   /path/to/auth.go "package main\n// modified\n"
go run save_truncate.go /path/to/auth.go "package main\n// modified\n"
```

## Platform support

Pure Go; both programs run on every platform.
