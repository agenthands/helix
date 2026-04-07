; TypeScript declaration types for body extraction
; Declaration types: function_declaration, method_definition, arrow_function
; Body field: body (statement_block or expression)

(function_declaration
  name: (identifier) @name
  body: (statement_block) @body)

(method_definition
  name: (property_identifier) @name
  body: (statement_block) @body)

(arrow_function
  body: (_) @body)
