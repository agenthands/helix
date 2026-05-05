# JetBrains editor save fixture (LIVE-02)

`save.go` reproduces the JetBrains "safe write" save sequence verbatim.
Source-of-truth: `.planning/phases/60-live-update-pipeline/60-RESEARCH.md`
"Editor Save-Pattern Dossier — JetBrains — `___jb_tmp___` +
`___jb_old___` rename".

## What it reproduces

```
CREATE  ${target}___jb_tmp___
RENAME  ${target}        (original → ___jb_old___)
CREATE  ${target}___jb_old___
RENAME  ${target}___jb_tmp___ → ${target}
REMOVE  ${target}___jb_old___
```

The watcher MUST filter `___jb_tmp___` and `___jb_old___` suffixes
in `handleEvent` so the post-debounce pending path-set is `{target}`,
not `{target, target___jb_tmp___, target___jb_old___}`. This is the
load-bearing test for Pitfall 5 (60-RESEARCH.md).

## Usage

```bash
go run save.go /path/to/auth.go "package main\n// modified\n"
```

## Platform support

Pure Go; runs on every platform.
