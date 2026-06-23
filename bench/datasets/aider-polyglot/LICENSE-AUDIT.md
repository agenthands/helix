# Aider Polyglot — Per-Track License Audit (SC#4)

This file is the redistribution-compliance record for the Aider-Polyglot
benchmark dataset (`github.com/Aider-AI/polyglot-benchmark`), which redistributes
Exercism exercise content with attribution. The polyglot benchmark covers six
languages — C++, Go, Java, JavaScript, Python, Rust — each sourced from the
corresponding Exercism track (`github.com/exercism/<lang>`), which carries its
own `LICENSE`.

Each track block below records the track's **real** SPDX license, the **sha256
computed over the track's committed `LICENSE` text at the pinned commit sha**
(read, not assumed — every track was fetched at the pinned sha and verified),
and a short excerpt of the redistribution/permission clause.

All six Exercism tracks ship an **identical MIT `LICENSE`** (verified
byte-for-byte: all six `LICENSE` files hash to the same sha256
`e52f804e74f0fbd34e8927962319df346166b8194094309a0aee318693df44df`). MIT permits
redistribution provided the copyright notice and permission notice are included,
which the polyglot-benchmark repo satisfies via attribution.

This file is a HARD-FAIL gate: `make verify-licenses` strict-decodes every
track block (`KnownFields(true)`), asserts a non-empty SPDX license + sha256 per
track, and exits NON-ZERO on a missing/malformed/zero-track audit. It checks
structural validity and the presence of a recorded license + sha256 only — not
legal accuracy.

Provenance — each `license_sha256` is the sha256 of the `LICENSE` text fetched
from `https://raw.githubusercontent.com/exercism/<track>/<pinned_sha>/LICENSE`
at the pinned sha recorded in the `source_repo` field below.

## Dual disposition (Phase 99)

As of Phase 99 this audit records a **mixed-license** vendored fixture tree under
`fixtures/`:

- **MIT track blocks** (keyed by a top-level `track:` key) — the six Exercism
  tracks below; these are byte-unchanged from Phase 85. The vendored MIT
  polyglot subset under `{python,go,rust}/exercises/practice/` derives its
  provenance from these `exercism/<lang>@<sha>` blocks.
- **Apache-2.0 fixture block(s)** (keyed by a top-level `fixture:` key) — the
  aider edit-format fixtures under `_aider-edit-format/`, vendored from
  `Aider-AI/aider@5dc9490` under Apache-2.0, with a bundled `NOTICE` + `LICENSE`
  copy. See the `fixture:` block at the end of this file.

In addition to strict-decoding both block kinds, the Phase 99 gate runs a
**manifest-vs-disk sha256 walk**: every on-disk file under `fixtures/` must have
a matching real-sha256 row in `VENDOR-MANIFEST.md`, and vice-versa, failing
closed on any mismatch (tamper detection on individual vendored bytes). No inline
SPDX is injected into executed fixture bytes — disposition lives here, in the
manifest, and in the per-track/per-subtree NOTICE sidecars only.

---
track: cpp
source_repo: github.com/exercism/cpp@9ca0c11dabd771bfae6fe5d9c09c6dd3ac51c4b8
license: MIT
license_sha256: e52f804e74f0fbd34e8927962319df346166b8194094309a0aee318693df44df
redistribution_clause_excerpt: "Permission is hereby granted, free of charge, to any person obtaining a copy of this software ... to use, copy, modify, merge, publish, distribute, sublicense ... The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software."
---
track: go
source_repo: github.com/exercism/go@68c309cef65b6140646270de0910581332a60f44
license: MIT
license_sha256: e52f804e74f0fbd34e8927962319df346166b8194094309a0aee318693df44df
redistribution_clause_excerpt: "Permission is hereby granted, free of charge, to any person obtaining a copy of this software ... to use, copy, modify, merge, publish, distribute, sublicense ... The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software."
---
track: java
source_repo: github.com/exercism/java@0443ae57ff3d4842259bd30bc0b71aeb593fe39b
license: MIT
license_sha256: e52f804e74f0fbd34e8927962319df346166b8194094309a0aee318693df44df
redistribution_clause_excerpt: "Permission is hereby granted, free of charge, to any person obtaining a copy of this software ... to use, copy, modify, merge, publish, distribute, sublicense ... The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software."
---
track: javascript
source_repo: github.com/exercism/javascript@971f01d9afa678b4378b85e99ef4dfae9836f68b
license: MIT
license_sha256: e52f804e74f0fbd34e8927962319df346166b8194094309a0aee318693df44df
redistribution_clause_excerpt: "Permission is hereby granted, free of charge, to any person obtaining a copy of this software ... to use, copy, modify, merge, publish, distribute, sublicense ... The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software."
---
track: python
source_repo: github.com/exercism/python@d8886cad965da61a2461d170b70647c7e2013894
license: MIT
license_sha256: e52f804e74f0fbd34e8927962319df346166b8194094309a0aee318693df44df
redistribution_clause_excerpt: "Permission is hereby granted, free of charge, to any person obtaining a copy of this software ... to use, copy, modify, merge, publish, distribute, sublicense ... The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software."
---
track: rust
source_repo: github.com/exercism/rust@557695ebd2fbe2cf99d51e91a4c8a556b7e2133b
license: MIT
license_sha256: e52f804e74f0fbd34e8927962319df346166b8194094309a0aee318693df44df
redistribution_clause_excerpt: "Permission is hereby granted, free of charge, to any person obtaining a copy of this software ... to use, copy, modify, merge, publish, distribute, sublicense ... The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software."
---
fixture: _aider-edit-format
source_repo: Aider-AI/aider@5dc9490bb35f9729ef2c95d00a19ccd30c26339c
license: Apache-2.0
notice_path: fixtures/_aider-edit-format/NOTICE
license_path: fixtures/_aider-edit-format/LICENSE
---
