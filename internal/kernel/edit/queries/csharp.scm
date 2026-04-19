; C# declaration types for body extraction
; Declaration types: method_declaration, constructor_declaration
; Body field: body (block)

(method_declaration
  name: (identifier) @name
  body: (block) @body)

(constructor_declaration
  name: (identifier) @name
  body: (block) @body)
