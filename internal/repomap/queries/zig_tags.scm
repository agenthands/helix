; Zig tag extraction query
; tree-sitter-grammars/tree-sitter-zig uses lowercase node types

(function_declaration
  name: (identifier) @name) @definition.function

(variable_declaration
  (identifier) @name) @definition.variable
