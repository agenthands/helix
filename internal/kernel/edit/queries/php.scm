; PHP declaration types for body extraction
; Declaration types: function_definition, method_declaration
; Body field: body (compound_statement)

(function_definition
  name: (name) @name
  body: (compound_statement) @body)

(method_declaration
  name: (name) @name
  body: (compound_statement) @body)
