; Go declaration types for body extraction
; Declaration types: function_declaration, method_declaration
; Body field: body (block)

(function_declaration
  name: (identifier) @name
  body: (block) @body)

(method_declaration
  name: (field_identifier) @name
  body: (block) @body)
