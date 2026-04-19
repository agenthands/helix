; Swift tag extraction query
; Adapted from tree-sitter-language-pack swift-tags.scm

; Definitions
(class_declaration
  name: (type_identifier) @name) @definition.class

(protocol_declaration
  name: (type_identifier) @name) @definition.interface

(function_declaration
    name: (simple_identifier) @name) @definition.function

(property_declaration
    (pattern (simple_identifier) @name)) @definition.property
