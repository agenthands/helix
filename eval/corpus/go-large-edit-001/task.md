The function `BuildReport` currently produces a plain-text report. Rewrite its
body to produce a structured JSON report instead. First use get_context or
get_repo_map to understand the codebase layout, then use replace_symbol_body
to replace the function body. Do not use replace_in_file for large-region edits.
