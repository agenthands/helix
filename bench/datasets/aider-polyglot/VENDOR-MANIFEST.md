# Aider Polyglot — Vendor Manifest (reproducibility + per-file digest record)

This file is the **reproducibility contract** for the vendored, offline,
mixed-license fixture tree under `bench/datasets/aider-polyglot/fixtures/`. It
records, for EVERY on-disk file in that tree: the file's relative path, a REAL
`crypto/sha256` content digest, its license disposition, and its upstream
provenance.

The `make verify-licenses` gate (Phase 99 Plan 02) recomputes each digest over
the committed bytes and HARD-FAILS (non-zero exit) on any mismatch, on any
on-disk file missing a manifest row, or on any manifest row with no on-disk
file (bidirectional manifest-vs-disk walk). The digests below are real — NOT
40-zero placeholders (unlike the deferred pins in `bench/LICENSES.md`).

## Covered tree root

- **`--tree` root:** `bench/datasets/aider-polyglot/fixtures/` (paths in the
  table below are relative to this root). Plan 02 wires the gate's `--tree`
  flag to this same root, and `--manifest` to this file.

## Pinned upstream sources

| Upstream repo | Pinned SHA (40-hex, immutable) | License | Vendored subtree |
|---------------|--------------------------------|---------|------------------|
| `github.com/Aider-AI/polyglot-benchmark` | `7e0611e77b54e2dea774cdc0aa00cf9f7ed6144f` | MIT (Exercism redistribution) | `{python,go,rust}/exercises/practice/<ex>/` |
| `github.com/Aider-AI/aider` | `5dc9490bb35f9729ef2c95d00a19ccd30c26339c` | Apache-2.0 | `_aider-edit-format/` |

The polyglot SHA matches `pin.go`'s `PinnedSHA`. MIT provenance is established
against the `exercism/<lang>@<sha>` source repos (the polyglot-benchmark repo
ships no `LICENSE` for 5 of 6 tracks — see `LICENSE-AUDIT.md`), NOT against
polyglot-benchmark itself. Both SHAs were validated against the 40-lowercase-hex
`isHexSHA1` shape before clone, and fetched by SHA + checkout (never a mutable
ref/branch/fork) — T-99-01.

## Vendored subset selection (MIT polyglot, VENDOR-01)

9 exercises × 3 languages, recorded so the Phase 100 baseline is reproducible:

```
book-store  bowling  forth  pig-latin  poker  react  two-bucket  variable-length-quantity  wordy
```

- **go:** all 9 exercises vendored from upstream.
- **rust:** all 9 exercises vendored from upstream.
- **python:** 8 exercises vendored from upstream. The `python/exercises/practice/wordy/`
  slot is occupied by a PRE-EXISTING repo-authored hermetic loader-test stub
  (`.meta/config.json` carries `authors:["fixture"]`); it is recorded below as
  `internal — repo license`, NOT Exercism MIT, and was left byte-unchanged so
  `loader_test.go`'s pinned `fixtures/python/.../wordy` assertions keep passing.

Per exercise the vendored files are exactly those the loader maps
(`.meta/config.json`, `.meta/example.<ext>`, the solution stub, the test
file(s), and go.mod for go / Cargo.toml for rust). `.docs/`, `.approaches/`,
`.articles/`, `.meta/template.j2`, `.meta/tests.toml`, `.meta/gen.go`,
`.meta/test_template.tera`, `.meta/additional*tests*.json`, and
`.meta/Cargo-example.toml` are intentionally excluded (lean tree).

## Pre-existing hermetic stubs (PRESERVED, NOT Exercism MIT)

- `python/exercises/practice/wordy/` — repo-authored hermetic stub.
- `rust/exercises/practice/leap/` — repo-authored hermetic stub.

Both predate this phase, carry `authors:["fixture"]` configs, and are recorded
below as `internal — repo license` / `(repo-authored hermetic stub)`.

## Apache-2.0 aider edit-format subset (VENDOR-02)

Under `_aider-edit-format/` (underscore-prefixed, loader-walk-safe):
`languages/{python,go,rust,java,javascript}/test.<ext>` (byte-identical), a
trimmed `search-replace-sample.txt` excerpt (two verbatim SEARCH/REPLACE blocks
from the 27810-line gold), `NOTICE`, and a byte-identical `LICENSE` (Apache-2.0).

## Per-file digest table

| path (relative to fixtures/) | sha256 | license | upstream_provenance |
|------------------------------|--------|---------|---------------------|
| `_aider-edit-format/LICENSE` | cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30 | Apache-2.0 | Aider-AI/aider@5dc9490bb35f9729ef2c95d00a19ccd30c26339c |
| `_aider-edit-format/NOTICE` | 03cd38f2af9e7f37953a7357599c94671ac47e385c8cfce70471d6d3452e6595 | Apache-2.0 | Aider-AI/aider@5dc9490bb35f9729ef2c95d00a19ccd30c26339c |
| `_aider-edit-format/languages/go/test.go` | 05bbd979a0cc1c0b740609bb5ed7e33b494a9e4c07b55b52e1f1140737fd646b | Apache-2.0 | Aider-AI/aider@5dc9490bb35f9729ef2c95d00a19ccd30c26339c |
| `_aider-edit-format/languages/java/test.java` | 816183cbaf0e95f5043007c595350855d44891b8b246f17afb5dff82fa173064 | Apache-2.0 | Aider-AI/aider@5dc9490bb35f9729ef2c95d00a19ccd30c26339c |
| `_aider-edit-format/languages/javascript/test.js` | 83ea8deb96446b487acc2a53354a9e711d58fd77c9f2c1114640428c6d9af7d6 | Apache-2.0 | Aider-AI/aider@5dc9490bb35f9729ef2c95d00a19ccd30c26339c |
| `_aider-edit-format/languages/python/test.py` | 3a8f8b5d4b8b04449981e81e5f6b445192021ed5809504511657bf4e5f73478e | Apache-2.0 | Aider-AI/aider@5dc9490bb35f9729ef2c95d00a19ccd30c26339c |
| `_aider-edit-format/languages/rust/test.rs` | 4043d3fa06aad5ef0eb5b467f44a8c27c7c1d5a65170e975b253a3d9aa2c260d | Apache-2.0 | Aider-AI/aider@5dc9490bb35f9729ef2c95d00a19ccd30c26339c |
| `_aider-edit-format/search-replace-sample.txt` | 952420910002ffbfb81e92724c1b7b2a2f80623c7146699c27c5a5d14265036a | Apache-2.0 | Aider-AI/aider@5dc9490bb35f9729ef2c95d00a19ccd30c26339c |
| `go/NOTICE` | 73ab71a82f591a0460543bf02abb5452f43f9558f8094f080fc79078edc5544e | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/book-store/.meta/config.json` | ff256c85dd589e19bb87548cc0b0781cd83d8ff895393d3b8a88810cd20d2744 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/book-store/.meta/example.go` | 652eb24e01b7e72f724c914e274c729c585e2f060b1cfc859bd634df0a48de27 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/book-store/book_store.go` | 1285dc9ce76e52b28e8f3099073ae21dd49b1d3b4dbcc53791b3f5b6448a047e | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/book-store/book_store_test.go` | 207455da5d52a619f945f56646adacb8306aaa5d1b9cf63b1ef074ba757c096a | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/book-store/cases_test.go` | 9745068fdf36ce85b5899dfe0048613824f1e7a41f96744ad6e4a10e0699d2e8 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/book-store/go.mod` | 2f890cdf7193187612bb89804a82e2c6d40fbb334a547443b65304e20c66031e | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/bowling/.meta/config.json` | 114f397a943fb5fe5fe2eaf3453999ca2a76753c847800953dbb04194b37da67 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/bowling/.meta/example.go` | 0afb526824852a07384de5757f9ec18764546548ab6e6439e7a8b52f476d110a | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/bowling/bowling.go` | 15d086d579c80e86d61772e70fa95fc37c04b525e99fcac5d172f35063c0b98b | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/bowling/bowling_test.go` | 893738a03dd38f3e077c528d6c2b3a12c43e1a8cd87d1243f523decd93d94eff | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/bowling/cases_test.go` | bf2634c3b180567b0a216cca6643a6576d9cb5e533b5631f510073f6fab6193e | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/bowling/go.mod` | 382c3433c6343998b4feff29c75984ef93d6b251ca203b80fe2958b608e4450e | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/forth/.meta/config.json` | 5f364634a885357b79eee2f954a695f2d2dd5516d08b31ae201ab513fcd25935 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/forth/.meta/example.go` | 4cdfaf4ab86f5cef9bbee8cf57ef429700901cce642a7ef6d715354e7458c3bb | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/forth/cases_test.go` | a7afe6846a81b35ad1a5a6a158b2504d91c609c5110f68aa16e258518b3fa5f5 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/forth/forth.go` | 14c13ff0588e31c9c9fdf1bb88d1760791f196562db5dce4b752cbf998fd6fd6 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/forth/forth_test.go` | b8a143444e617eb1e303a24e710829cf9e2ccd1cd3c64866752349fe4e615666 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/forth/go.mod` | b3dc9275606656e9266545d383983439a5451556752e7ae9da37a845aaf19caa | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/pig-latin/.meta/config.json` | f0254fec184f6eb9173b3d955ccc13686e8af3f0e61703e45b21089523bc133c | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/pig-latin/.meta/example.go` | 0afa53ce47ead0f63d7da5e9f2a7fa914678e66598a27bc6f7a9b0e94975393d | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/pig-latin/cases_test.go` | 4c83cfc40e48933bc3451c17cb02f665632cf90dfa22ffc61e912d8adcd1637d | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/pig-latin/go.mod` | ae41df9c918b27a595b45a1864ea4adf4f36f0a326951758a31d46bcaf45df85 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/pig-latin/pig_latin.go` | b370899f26d102e9d3a17d490d5d651cc6594bbd37ed5b906930efee06d03c07 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/pig-latin/pig_latin_test.go` | 38b79935b4e12c21b34c8f7e4532ddc0884c52bde1be9a73a684deae3111feb7 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/poker/.meta/config.json` | 9828109412f6d8b2b027e3eb9a41b4cb5e8c1ec74c5ef21b4f054c3e20e2d8a6 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/poker/.meta/example.go` | 64f032e280aafad8a4806b1336e8a3e59699b63ebb27a4a5b258a77d790b7278 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/poker/cases_test.go` | 1f11f38c416f5e1685ae2f0342a618fd8898798c328d8bc3eb5c30a8e0295b78 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/poker/go.mod` | ab2878b669da6251ab0a065e088fa0bb2aca62c2e1a9c44b08a994dcad416eb8 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/poker/poker.go` | bc8853adbd801340b18d420f259bdb7cb362efb0dc68787f9a3d3250dbe89f7f | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/poker/poker_test.go` | 1d7fa31b3bc34f799d689a88df3291224c5161fdbd49e203489393e716e7487f | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/react/.meta/config.json` | 237877296b92a525d94c23ce99e016a0be6c0edac6fec915ad188822dcd1e94b | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/react/.meta/example.go` | 9f526fa177d52f72de51234a47647322368557c43f3bff0fbd8e6a0e427dae30 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/react/go.mod` | f642b12a02f6e1457d2821dbec2af382cf085f93aebe9ed74c6e6cce383f3e5e | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/react/react.go` | cb10ee7fc2c6c851464421fb3ed70143a5257bad791f4d26102dae93dd58c4ac | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/react/react_test.go` | ea2a04763130bbccd432822ef72d3a1d9b9c985ce9f08d63ccc0941041745174 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/two-bucket/.meta/config.json` | 209d888e10bca728498ca780200d7ba3adc8c0b88a78e99e128377f166c0d804 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/two-bucket/.meta/example.go` | 653cf33e430514255ac45c8ea0b55e1693b945184cf3e38105b1712bbca196b5 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/two-bucket/cases_test.go` | 188f4e0838506a72ceadd1ebbad81614357baac1793c423e2b247e23f0229992 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/two-bucket/go.mod` | ab06dc3735b944fc1fe108bdea1a0340047b20c051ccbc3005c637a09e2c6a46 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/two-bucket/two_bucket.go` | d34182e90548f73fd15a127c2b8fd4501f7f39f825bdb376f039181fc7222b11 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/two-bucket/two_bucket_test.go` | 052686779e1b27bbaeb87956c64c56130a736791abe5137df9bced2cc7d8a999 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/variable-length-quantity/.meta/config.json` | dec8c55d75bbd131c3e56bc09bee2c0cef7189e4a364ca53625b54ec70019f7a | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/variable-length-quantity/.meta/example.go` | 8e4a2c69f2a3c938b33009802b1dd82fde8f2c853ed2e69bc8c62b2b7add065c | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/variable-length-quantity/cases_test.go` | d0a9d53159c30bd9bd37611ae0a229b48ed1820e2ef6262182308b46422b4580 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/variable-length-quantity/go.mod` | 482a243fcde346409cb5c09a79de8acb81cd9441bac9e615e0bef652010585cb | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/variable-length-quantity/variable_length_quantity.go` | 53b23c7a56a5eb46b866b0836194d87934fbf9aee59c8b9a1ac8f2d9b8bbb1af | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/variable-length-quantity/variable_length_quantity_test.go` | e9be3f4000077a8a46464d927d4113dccb5cb0c89c51db6db905d0d125431b2f | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/wordy/.meta/config.json` | 117694df056acb34ff614acf6325f6540d149262a8b0c871c9f0b85d87fa4469 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/wordy/.meta/example.go` | 8e492ec31252bc085b43cc0fed93c60616456acb3fa1f9e017e3929ca5114856 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/wordy/cases_test.go` | ed8fe405ac82b61aaf95d803fca036297ad6ccd12e164008d64091012b688bd1 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/wordy/go.mod` | b2824615a8369f9ff20e0767b95e72d7344f110bafb0f5e6db666d427c78d7fa | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/wordy/wordy.go` | 85380381db04b3ff7673b613bd3c705431b0bc7fc6549125115c8094c7c0a3c7 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `go/exercises/practice/wordy/wordy_test.go` | de2558df42a51dcb9914375fa9c8f7c94e0cb996b68f5bb29c944bb249c5e292 | MIT | exercism/go@68c309cef65b6140646270de0910581332a60f44 |
| `python/NOTICE` | f8f58884770abe52ac40d3875ad4e44eec404374a54f3885d7382ff23e64e3cb | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/book-store/.meta/config.json` | 6bae06984205573b52770a4af33dfccbdc31c6134e042397b6c2c1c69372ee08 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/book-store/.meta/example.py` | 728655581209a08f392aa2d6a5060bca855f53ea3e96f66a86db738dbf23d751 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/book-store/book_store.py` | 5c22fe209f90b16dbbacff457199d0b14152b48616c875085c8ff1f6f7188ec2 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/book-store/book_store_test.py` | fa4109f3378cceaff8bdc389b333bc23c65f8b1345bc907be5c09593be0ee1eb | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/bowling/.meta/config.json` | ff4e4a727258398458fc48e7382abc9fe7d403a3932f2a0fbe9c85e113eb07dd | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/bowling/.meta/example.py` | 7f634589daeb5c2974d3600d8eaf60a8fa157c0ceb37fe7341e47ea0b2e9ca8f | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/bowling/bowling.py` | a356c0682b6c0c04e9e30a7da603b2014c227426b43bbe37f34cd99913d368b6 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/bowling/bowling_test.py` | 6b9a80b6835828f575ce0aba66876850f75c59f764014ac509fe1df7e3a9ee20 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/forth/.meta/config.json` | f732bc7caeee8d59cc1dfb133f15c79c7f5b45a92539d67688718dc2e3eb1912 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/forth/.meta/example.py` | a90f6134d4f24b5abee77880c8eb4f8adf7a88d925c0cd3d9bbfcfba895a320b | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/forth/forth.py` | 9acd281e02feff4fc5ae766063a1ce1290b625bb930cc8f54c15a0109610e562 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/forth/forth_test.py` | 05f78a51ce5b3e18439088442925c7cea5b1fb1703bc10832dd048c057b7639c | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/pig-latin/.meta/config.json` | 3bb69ebccaf35175d75eceb1a7dd5db377f126c42ed18fa932b06db6c18e48d5 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/pig-latin/.meta/example.py` | 52a698b0db8c23e113b4346b1c41df69b56bbc7d9422dcab9cde942d06721019 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/pig-latin/pig_latin.py` | fbef923db1bc51436424e83ca92e8d5d4f2505c5796fee8daf3fe276b2259e63 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/pig-latin/pig_latin_test.py` | 2a2bec06a0389dad5636c83cc604a418e6f0ed79fbf8768f620c7d1101ec65c2 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/poker/.meta/config.json` | 4f909ffbc55d536e4e6d42831a64009a981e652b9e03c745e07701bc3fed55af | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/poker/.meta/example.py` | 9627361e2566290218239ed64ea4edf610f826e94ae7044d9eef9316d241ec43 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/poker/poker.py` | 6a4b5adab3ba9261fb4f9e2b09abc549b9ea37bf0774edc3c7759cec9a8b41fe | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/poker/poker_test.py` | b39f023208318973e18a341449aa076f7595ff98f07174082a2a1b05c84ad8fd | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/react/.meta/config.json` | 644df9df77c3d1f7d1317bae06ef62f358a7880a176736a79ac33dc8be241ffa | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/react/.meta/example.py` | a1198a835f44b3826e6bb2d6c2d0df81ede7e7541f9f85853aff08f1487039ff | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/react/react.py` | 49f06798292a5ae593375b8b080f4bee839b85a19827b06dd4c6c25583e5f5ce | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/react/react_test.py` | 3017017c1f0830d96ddedac16fb6945a6e7769c62fdc89d8b251082cf9d495aa | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/two-bucket/.meta/config.json` | c24f814713bd81df19f82b2d34be37ee5c856e3d4b32386b6ea77bb9ba7259de | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/two-bucket/.meta/example.py` | bbf49eb2dc84d6a161e07e11d5bee11f224ad3b93a1c9afcf8a1083c405aa269 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/two-bucket/two_bucket.py` | 6ed45b4d895c2e1ff15732719df5c832f1b67a610b069f71c1c0c8c528da1dd4 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/two-bucket/two_bucket_test.py` | 3dedaa5c25ff1b94112807ee25a488c66fc5f7241e1fd86c19a8be07c7597466 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/variable-length-quantity/.meta/config.json` | 418face1a939687aabf477975269bcb66ee69402a8df8530909fb22b166408be | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/variable-length-quantity/.meta/example.py` | a516b77e06bb9961d956494e5953708510efd37a65ba6996d453ca932f07319d | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/variable-length-quantity/variable_length_quantity.py` | c065d92722a1b8b9723bae2de3662583b65287c3fa2a459768f65e667a2ac695 | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/variable-length-quantity/variable_length_quantity_test.py` | e9ecd83a153958e4ef268faa83df2441ef7deb509e01040dc042af53b7aaf7ef | MIT | exercism/python@d8886cad965da61a2461d170b70647c7e2013894 |
| `python/exercises/practice/wordy/.meta/config.json` | 281f9e61ab93df47228cdd54bbec8e7a1c8d98b7fc0e7099342baab69da3204e | internal — repo license | (repo-authored hermetic stub) |
| `python/exercises/practice/wordy/.meta/example.py` | 5c4d993622749cc6f9f53ab173503b7c8380201a340b3db3c28609e65505b914 | internal — repo license | (repo-authored hermetic stub) |
| `python/exercises/practice/wordy/wordy.py` | 4733f008ca52070c599dcb3a25aeb1118b4b37d886950fb01acbd2083b3b461c | internal — repo license | (repo-authored hermetic stub) |
| `python/exercises/practice/wordy/wordy_test.py` | a1d9a5be01bc6020b7ac469062581df099064e819bd8720915daedb50b090ef6 | internal — repo license | (repo-authored hermetic stub) |
| `rust/NOTICE` | 74b0aef4637fea3d9b35c169d1ccbb4d905f2fccc17e49c185281ace4bb3403e | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/book-store/.meta/config.json` | ba57a764e283bbbf427ac3eeb0abda8d5e928e78ff7b91ca178c5399eda792d2 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/book-store/.meta/example.rs` | 45b9b49aad3673a599632ae262a73f0bec5770bd9b275a6984df151e30398c9f | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/book-store/Cargo.toml` | 13736df23fe8988b854e8bb0cd90ea4408550a3e5bb8cfe54b2831f1afe22408 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/book-store/src/lib.rs` | 9875ce1e36e56efda907ad21921fa761d649bf55a7daba23b14d7ab13e51eed6 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/book-store/tests/book-store.rs` | 18c4e094adcfcd41afce1e00b4dd2346b0b9a45770d46c6ff416e4fb5e33c626 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/bowling/.meta/config.json` | 9599de32e8170b213b6d576724d01e76921d903b3fc8059986bfed100d3f7595 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/bowling/.meta/example.rs` | e426376afc91758c2ab6f5bb394ccbf230cb3294e0e5df3b08f9d55327b34efe | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/bowling/Cargo.toml` | 0a4a14a4da54dc16536e8bba445d4ab2746f6504eb8914b4a23256beebd5b942 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/bowling/src/lib.rs` | ca767570a21510152cd41d2d6b0c33208ee59e553758542bb2c95ca102f751e7 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/bowling/tests/bowling.rs` | 59a07bb552e96b0aed1124bcf96eb84596db0990520676bc52a583f2197e145d | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/forth/.meta/config.json` | 96ceb4aaf974832b3dbc7f33b4f7719fee7d6cc0e7dad990fdb35141c1f8eadc | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/forth/.meta/example.rs` | ef704d2864519848f5030e3144c0e7ef3dd0f62e6380307e2b44fb4a1c8d90f6 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/forth/Cargo.toml` | 9141705cd71d21e13c47b87400632527b9e0caad93a99a0f67a5906fe90aba0b | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/forth/src/lib.rs` | 439ec43a6c67aba6ca003c079fd030d72d4c91a542c488436fe59ed28a5ea762 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/forth/tests/alloc-attack.rs` | 23242b129edba250de03033a7010395fac7d941b0ebc075be3b690626111baa3 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/forth/tests/forth.rs` | d55f197270ead3ca9417d301716e5cb3b2f187d4063fcd64ace46e69917e9385 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/leap/.meta/config.json` | ffa180c14b252d991fc7016cf10e193a21cab07762a96eb2087b664202936af8 | internal — repo license | (repo-authored hermetic stub) |
| `rust/exercises/practice/leap/.meta/example.rs` | fb364b8e244f51eaab9e7df90f2a2b73da28094209ddd24e23c1b43469d7c239 | internal — repo license | (repo-authored hermetic stub) |
| `rust/exercises/practice/leap/src/lib.rs` | 21487b797e9985101364967804556f0b22e3ef01f2546fdb2322c3b29a793772 | internal — repo license | (repo-authored hermetic stub) |
| `rust/exercises/practice/leap/tests/leap.rs` | 6eeed95c320557b4e8aff4f0754be6dcee0447f1115083851372b67d14823225 | internal — repo license | (repo-authored hermetic stub) |
| `rust/exercises/practice/pig-latin/.meta/config.json` | 7a7ab8a6d715bbb7d4d416324fb98f4f9da9f248d9b87f510aa6247fdb20eddb | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/pig-latin/.meta/example.rs` | 55e9f3e163cc1d9b981cd6aaaeda0d765c26928abba4552e28c930ebd40f60b4 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/pig-latin/Cargo.toml` | 5242eda142ce58314974e7d1fd69b06b6f083c268607d28c41650da1a5a8c0bd | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/pig-latin/src/lib.rs` | 7cbbd5e8c2aea23201d027e2ae258164717dc79e2d0276387686a66270870b5c | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/pig-latin/tests/pig-latin.rs` | bf14c7a17504d74751706ebc066a6c2df4abdbef0312e50ae7c5c0b02bf2f89d | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/poker/.meta/config.json` | 5a1131109dcecbdab1372eee628548b46ed3421161acc10e096c45cc0a5d0dd5 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/poker/.meta/example.rs` | 9b0f1528fc4759665e4cff1204107fd7788aa2a923c2ca30838ee520e45f9a6e | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/poker/Cargo.toml` | 9239eb8a72c85f881dcdb09fa6ac087bb5aeccbcba484c8ef866c49a12373f50 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/poker/src/lib.rs` | 25ad2cded77de195e65fe757b0be1577bd26f8b1cf2f036cfcbce534809c3801 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/poker/tests/poker.rs` | a2f2a79cfd5dd45d7fbb63d007a30afdd451bd0a64bb5e11e17d4e1f3bda1083 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/react/.meta/config.json` | d63c5019b2716e6012794a675c9bf90608993d1c4507eff1d967cdb7ac9c4d2d | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/react/.meta/example.rs` | 2164b2d739786c6ff8691db83372a9d74b53ed3d3100f1c7960fcfa46379706f | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/react/Cargo.toml` | a7e86259e695485bc33a20921a1d361221d6e2a624b03c47cfb323b516ea55e9 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/react/src/lib.rs` | 46547887d9457fcff323c996e59ad359e0444c04b070146c25e920c36dff0f69 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/react/tests/react.rs` | f53454021d91884b5dd7698be4dc1d80620f3c587fd09e760b851256e1db6c71 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/two-bucket/.meta/config.json` | 649b6efacd0613421cfaaf2f9e7c942b5d95a4f4dad25b7533022a9014908db9 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/two-bucket/.meta/example.rs` | c053a0852c1c99771b38d00300f3a476dcc2e2c413b73caec4387755911e4e68 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/two-bucket/Cargo.toml` | 8a3b387b233c55bb0ca5f32340c1ef8214063fb0da259ea66916d404e8b8632c | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/two-bucket/src/lib.rs` | ccb5816343303fed8499b59bca874d75e263b357d101ef12eea76bb672d84da9 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/two-bucket/tests/two-bucket.rs` | 98a70b77abfc85490cdf22f5e64ef7be2a60da531efae3e9211db98de488bbf0 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/variable-length-quantity/.meta/config.json` | 7edd621316c546efec052d73b44163c7f709e2dcfedd93bb02aa238ca571f305 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/variable-length-quantity/.meta/example.rs` | b8aa2ddc54a86daea977b515eae32ff0b1f78feb0f05a56b1875618fd296d0f4 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/variable-length-quantity/Cargo.toml` | dfcad2b4d36ddbe87abf80bc49789150b66e4e60523b8771ee4c4172b770f9d0 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/variable-length-quantity/src/lib.rs` | 913a249e3eabba912c3489c022d92b36c31b72945c348e43bd1e15d87590a077 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/variable-length-quantity/tests/variable-length-quantity.rs` | f0a1e684145b9addcfcfb860e441d4a96ba7346cffd59ef85988744627528de8 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/wordy/.meta/config.json` | 9723493a9023b128822361e927864c34c1b7b34cb6ba96f311dc45e11a0ec8db | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/wordy/.meta/example.rs` | b1f6912250a7d789f6e179280cb3620dd9a25815abd2ccb8a3b9846b8aa4b48b | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/wordy/Cargo.toml` | 2e7565b04fd6c408bfa39675b0fd3b1d073e2f2428c7be52c5c6d9871da32656 | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/wordy/src/lib.rs` | 95340d220ed9ce06a6c6d54c1fdbeb4f1ae3daa23b38e1f89b9af8234266862f | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |
| `rust/exercises/practice/wordy/tests/wordy.rs` | 4282b8255e331ba38de5586ea2e50bc9d476e3d7bbc2b432f99b02b20e0fbd9a | MIT | exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b |

## Summary counts

- MIT (Exercism polyglot, vendored): 134 files
- Apache-2.0 (aider edit-format): 8 files
- internal — repo license (pre-existing hermetic stubs): 8 files
- **Total covered:** 150 files (the entire `fixtures/` tree)
