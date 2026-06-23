# RepoMap-Eval Gold Corpus

This directory's `testdata/` carries the committed corpus the stdlib-only
`repomapeval` leaf scores. The leaf NEVER dials the daemon and NEVER imports
`internal/repomap`; it scores **committed gold** against a **committed captured
ranking** (committed-vs-committed, the Phase 100 determinism contract). The live
`get-repo-map` / `get-context` capture is refreshed by the separate
`//go:build ignore` HELIX_BIN-gated regenerator
(`bench/runtime/repomap_eval_capture_regen.go`), which lives OUTSIDE the leaf.

## Per-language size floor (the documented minimum)

The corpus covers the exact set of languages Phase 99 vendored (py / go / rust;
Java is reserved — D-03). The floor is the number of vendored exercises under
`bench/datasets/aider-polyglot/fixtures/<lang>/exercises/practice/`:

| Language | Floor (vendored exercises) |
|----------|----------------------------|
| go       | 9                          |
| python   | 9                          |
| rust     | 10                         |

`TestCorpusFloor` fails if any language corpus drops below its floor. The floor
is the documented known minimum (D-04) — enough for recall@k / MRR / nDCG@10 and
the discriminator margin to bite, without heavy authoring cost.

## `file:symbol` identity convention (pinned ONCE)

Every gold ID and every captured-ranking ID is `relpath:SymbolName`:

- **relpath** is the exercise-relative path of the file that defines the symbol,
  taken from the exercise's `.meta/config.json` `files.solution` entry
  (e.g. `wordy.go`, `book_store.py`, `src/lib.rs`).
- **SymbolName** is the defined function / type / method name. Methods are
  **receiver-qualified** as `Receiver.Method` (e.g. `bowling.go:Game.Roll`,
  `react.go:cell.SetValue`). Free functions and types are bare
  (e.g. `wordy.go:Answer`, `poker.go:handValue`).
- The split is on the LAST colon (`SplitID`), so a `src/lib.rs:answer` ID keeps
  `src/lib.rs` as the relpath and `answer` as the symbol.

Gold is authored from each exercise's `.meta/example.*` **reference solution**
(the file that actually defines the symbols) keyed to the `files.solution`
relpath — NEVER from `get-repo-map` output (D-02). `TestGoldParseable` asserts
every gold ID is `relpath:symbol`-well-formed and that its relpath matches a
`files.solution` entry of that exercise (Pitfall 3 guard).

## Captured-ranking parse contract

`get-repo-map` returns rendered **tree text** (`{"tree": treeText}` envelope,
`internal/skill/repomap/skill.go:428`), NOT a structured symbol list. The
regenerator parses that tree into an ordered `file:symbol` list at capture time;
the leaf only ever reads the pre-parsed JSON. The parse contract is:

- **File order** = appearance order of files in the rendered PageRank-prefix tree
  (the `ranked[:mid]` prefix that `RenderBudgeted` emits).
- **Symbol order** within a file = in-file elided-def appearance order under that
  file in the rendered tree.

Each captured exercise carries BOTH orderings:

- `repo_map` — the uniform `get-repo-map` ordering.
- `context` — the personalized `get-context` ordering, seeded from the
  exercise's `files.solution` stub path.

`TestCapturedAligned` asserts every gold exercise has a captured entry with a
non-empty `repo_map` AND `context` ranking. The committed captured rankings
include a few non-gold "noise" IDs (test-file symbols) after the gold prefix so
the metric is non-trivial and the discriminator (below) bites.

## Discriminator margin

The anti-vacuity gate (D-09) ships a **reversed** ranker and a **seeded-random**
ranker (`math/rand/v2` `rand.NewPCG`). The forward ranking must beat
`max(reversed, random)` on mean nDCG@10 by a committed absolute margin, AND both
adversarial rankers must individually score below forward (the discriminator
BITES). The committed margin constant is `DiscriminatorMargin` in `rankers.go`:

- **`DiscriminatorMargin = 0.30`.** Chosen after observing the real forward-vs-
  adversarial spread on this authored corpus (n=28 exercises across py/go/rust):
  forward mean nDCG@10 ≈ 1.0 (gold is front-loaded), reversed ≈ 0.005 (the gold
  prefix is pushed entirely off the top-10 discount window — each captured
  ranking carries a >10-ID noise tail), and seeded-random ≈ 0.52 (a shuffle
  still keeps some gold inside the top-10 on these short rankings). The binding
  constraint is `forward - max(reversed, random) ≈ 1.0 - 0.52 = 0.48`. 0.30 sits
  comfortably under that observed spread (≥0.2 is the guide per D-09/A6) while
  leaving ≈0.18 of headroom so a near-tie on a small corpus cannot sneak a
  vacuous pass through.
