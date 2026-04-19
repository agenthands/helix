; Adapted from aider julia-tags.scm for Serena capture convention
; Adjusted for tree-sitter-julia v0.25.0 node types

(module_definition
  name: (identifier) @name) @definition.module

(struct_definition
  (type_head
    (identifier) @name)) @definition.class

(function_definition
  (signature
    (call_expression
      (identifier) @name))) @definition.function

(macro_definition
  (signature
    (call_expression
      (identifier) @name))) @definition.macro

(call_expression
  (identifier) @name) @reference.call
