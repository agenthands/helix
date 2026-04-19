; JavaScript declaration types for body extraction
; Declaration types: function_declaration, method_definition
; Body field: body (statement_block)

(function_declaration
  name: (identifier) @name
  body: (statement_block) @body)

(method_definition
  name: (property_identifier) @name
  body: (statement_block) @body)
