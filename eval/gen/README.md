# eval/gen — corpus task generator

A small `package main` Go program that emits 20 generated corpus tasks under `eval/corpus/` to grow Phase 67's reference dataset toward the D-05 50-100 ceiling without hand-authoring each one.

## Regenerate

```
go run ./eval/gen
```

Re-running is **idempotent**: each task directory is wiped and rewritten. The 10 hand-authored tasks (`go-rename-public-001`, `go-delete-symbol-001`, `go-public-api-001`, `go-large-edit-001`, `go-security-001`, `ts-rename-001`, `ts-delete-001`, `ts-public-api-001`, `py-rename-001`, `py-public-api-001`) are NOT in `builtinSpecs()` and are NOT touched by the generator.

## Adding a task

Append a `TaskSpec` literal to the slice returned by `builtinSpecs()` in `specs.go`, then re-run `go run ./eval/gen`. Each spec carries the language-specific seed source as raw strings — no per-task template indirection — so adding a task is one struct literal append.

## Self-smoke

After writing each task, the generator copies `repo/` to a temp dir, applies the expected post-edit transformation programmatically (rename: text-replace `OldSymbol`→`NewSymbol`; delete: substitute `PostEditFull`; public_api: substitute `PostEditDecl` and optionally `PostEditCaller`), then runs `verify.sh` against the temp copy and asserts exit 0. If any task's `verify.sh` is non-functional against the post-edit copy, the generator exits non-zero — proving the verify scripts are real, not stubs.

## Layout per task

```
eval/corpus/{lang}-{family}-NNN/
  task.md                  # 1-2 sentence agent instruction
  expected_tools.yaml      # rename / delete / public_api rule shape
  budget.yaml              # uniform 200000/32000/300/50
  verify.sh                # bash; exits 0 on PASS
  repo/
    main.go + go.mod       # Go tasks
    index.ts + package.json  # TS tasks
    main.py                # Python tasks
```
