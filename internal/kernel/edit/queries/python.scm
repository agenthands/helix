; Python declaration types for body extraction
; Declaration types: function_definition, class_definition
; Body field: body (block)

(function_definition
  name: (identifier) @name
  body: (block) @body)

(class_definition
  name: (identifier) @name
  body: (block) @body)
