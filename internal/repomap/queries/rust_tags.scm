; Definitions
(function_item
  name: (identifier) @name) @definition.function

(struct_item
  name: (type_identifier) @name) @definition.type

(enum_item
  name: (type_identifier) @name) @definition.type

(trait_item
  name: (type_identifier) @name) @definition.type

(impl_item
  type: (type_identifier) @name) @definition.type

; References
(call_expression
  function: [
    (identifier) @name
    (scoped_identifier name: (identifier) @name)
    (field_expression field: (field_identifier) @name)
  ]) @reference.call
