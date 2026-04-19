; Kotlin declaration types for body extraction
; Declaration types: function_declaration
; Body field: function_body (contains block)
; Note: Kotlin uses function_body wrapping a block, not a direct body field

(function_declaration
  (identifier) @name
  (function_body) @body)
