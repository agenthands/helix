; Kotlin tag queries for Serena
; Adapted for tree-sitter-grammars/tree-sitter-kotlin grammar

; Definitions

(class_declaration
  (identifier) @name) @definition.class

(function_declaration
  (identifier) @name) @definition.function

(object_declaration
  (identifier) @name) @definition.object

; References

(call_expression
  (identifier) @name) @reference.call
