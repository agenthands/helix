; Adapted from borrow/aider/aider/queries/tree-sitter-languages/c-tags.scm
; Converted @name.definition.X -> @name + @definition.X (Serena convention)

(struct_specifier
  name: (type_identifier) @name
  body: (_)) @definition.class

(function_declarator
  declarator: (identifier) @name) @definition.function

(type_definition
  declarator: (type_identifier) @name) @definition.type

(enum_specifier
  name: (type_identifier) @name) @definition.type
