# pytest config for the dev-time tuning harness (tools/dspy-tune/).
#
# testdata/ holds synthetic FIXTURE files for the corpus-materializer tests —
# including an Exercism-shaped `*_test.py` stub (testdata/fake_corpus_repo/.../
# greeter_test.py) that FAILS by design (the stub is unimplemented). pytest's
# default `python_files` glob matches `*_test.py`, so a bare `pytest` would
# collect that fixture and report a spurious failure. Exclude the whole testdata
# tree from collection — those files are inputs to tests, not tests themselves.
collect_ignore = ["testdata"]
