# Research — SWE-bench Verified as a confirming secondary grader (`swebench==4.1.0` on Podman)

**Bottom line:** The `princeton-nlp/SWE-bench_Verified` pin still resolves (500 instances, `test` split; a canonical copy also lives at `SWE-bench/SWE-bench_Verified`). Always pass `--dataset_name` explicitly — v4.1.0's *default* moved to `SWE-bench/SWE-bench_Lite`. For a cheap confirming run, sample **K = 25–50** Verified instances via `--instance_ids`, stratified across repos/difficulty. **CRITICAL grader footgun:** the harness scores **0 tests evaluated as `resolved=True`** — our wrapper must independently enforce the 0-tests-is-hard-error rule.

## Dataset facts
- `princeton-nlp/SWE-bench_Verified`: 500 rows, single `test` split, ~2.1 MB, no deprecation banner. Mirrored at `SWE-bench/SWE-bench_Verified` (same 500 rows).
- Required fields: `instance_id, repo, base_commit, problem_statement, FAIL_TO_PASS, PASS_TO_PASS` (+ `patch, test_patch, version, environment_setup_commit, difficulty`). F2P/P2P are JSON-list-of-string test ids.
- **No "Verified-lite."** Verified IS already the curated 500-instance human-validated subset. `SWE-bench_Lite` (300) is a *different, non-overlapping* set — do NOT switch to it for a confirming run; subsample Verified instead.

## Harness logistics (v4.1.0, read from source)
- argv: `--dataset_name` (default `SWE-bench/SWE-bench_Lite` → override), `--split test`, `--instance_ids id1 id2…` (default = all), `--predictions_path` (required; `"gold"` to test gold patches), `--run_id` (required), `--max_workers` (4), `--cache_level env|none|base|instance` (env), `--timeout 1800`, `--namespace swebench` (pre-built image registry), `--report_dir .`.
- predictions.jsonl keys: `instance_id`, `model_name_or_path`, `model_patch` (unified diff string).
- Final run report → `report_dir`/`<model__name>.<run_id>.json` (keys incl. `resolved_ids`, `resolved_instances`, `empty_patch_ids`, `error_ids`).
- Per-instance report → `logs/run_evaluation/<run_id>/<model>/<instance_id>/report.json` (the F2P/P2P buckets) + `run_instance.log`, `test_output.txt`.

## `resolved` semantics + the zero-test gap (CRITICAL)
- `resolved=True` iff `compute_fail_to_pass()==1.0 AND compute_pass_to_pass()==1.0` (matches our contract: all FAIL_TO_PASS pass, no PASS_TO_PASS regression).
- **BUT** both metrics `return 1` when `total==0`. Empty/None patch → safe (`resolved=False`); unparseable log → safe (`found=False`). **A log that parses but contains zero expected tests → `total==0` → resolved=True.** This is the opposite of our "0 tests ⇒ hard error."
- **Our `grade_swebench.py` MUST** assert `len(F2P.success)+len(F2P.failure) == len(expected FAIL_TO_PASS)` and `>0` from the per-instance `report.json`, hard-erroring otherwise — NOT trust the harness `resolved` flag. (Phase 109 already recomputes from `tests_status` with a "0 tests ⇒ GradeError" rule; re-verify it holds against this footgun at corpus scale.)

## Container cost/time
- Disk dominates: `cache_level=env` ≈ 100 GB across all 500; optimized public registry ≈ 30 GiB total. Per-instance pulls tens-to-low-hundreds of MB.
- Warm compute ≈ 8 s/instance; full 500 run ≈ 62–73 min on 32c/128 GB. Per-instance hard `--timeout` 1800 s. Official min: 120 GB disk, 16 GB RAM, 8 cores x86_64.
- **Recommended K = 25–50** Verified instances (image pull ≈ 3–5 GiB for 50, a few min compute). Below ~15 binomial noise too high. **Stratify across distinct repos/difficulty** (same repo shares env image → diversifies signal + amortizes pulls).

## Podman specifics
- `podman system service --time=0 &` then `DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock` (our pin is correct; `--time=0` = no idle timeout for long runs).
- Gotchas: rootless `graphroot` must be on an exec-capable, roomy mount (100 GB+ images); `/etc/subuid`+`/etc/subgid` entries required; fuse-overlayfs vs native overlay driver mismatch is a common "won't start" cause. `--network=none` eval is smoother under rootless Podman.
- **Smoke first:** 1–2 instances with `predictions_path=gold`, K=2 on the real Podman socket before any batch.

## Unknowns
- No swebench-team Podman runbook exists — gotchas inferred from Podman docs applied to the harness's Docker-SDK usage; settle with the gold-patch smoke run.
- Per-instance pull sizes from Epoch optimized-registry numbers may differ from default `namespace=swebench` images.
- `--report_dir` interplay (top-level report) vs the always-`logs/run_evaluation/...` per-instance report — read both locations.

## Sources
- https://huggingface.co/datasets/princeton-nlp/SWE-bench_Verified · https://huggingface.co/datasets/SWE-bench/SWE-bench_Verified
- v4.1.0 source: `run_evaluation.py`, `reporting.py`, `grading.py`, `constants/__init__.py` (github.com/SWE-bench/SWE-bench @ v4.1.0)
- https://www.swebench.com/SWE-bench/faq/ · https://www.swebench.com/SWE-bench/guides/docker_setup/ · https://epoch.ai/blog/swebench-docker
- Podman: troubleshooting.md, SUSE rootless-podman, ArchWiki Podman
