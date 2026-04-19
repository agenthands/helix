; C++ declaration types for body extraction
; Declaration types: function_definition
; Body field: body (compound_statement)

(function_definition
  declarator: (function_declarator
    declarator: (identifier) @name)
  body: (compound_statement) @body)

(function_definition
  declarator: (function_declarator
    declarator: (field_identifier) @name)
  body: (compound_statement) @body)
