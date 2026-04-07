; Rust declaration types for body extraction
; Declaration types: function_item, impl_item
; Body field: body (block)

(function_item
  name: (identifier) @name
  body: (block) @body)

(impl_item
  body: (declaration_list) @body)
