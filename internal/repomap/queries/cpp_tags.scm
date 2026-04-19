; Adapted from borrow/aider/aider/queries/tree-sitter-languages/cpp-tags.scm
; Converted @name.definition.X -> @name + @definition.X (Serena convention)
; Stripped @scope capture and qualified_identifier pattern

(struct_specifier
  name: (type_identifier) @name
  body: (_)) @definition.class

(function_declarator
  declarator: (identifier) @name) @definition.function

(function_declarator
  declarator: (field_identifier) @name) @definition.function

(type_definition
  declarator: (type_identifier) @name) @definition.type

(enum_specifier
  name: (type_identifier) @name) @definition.type

(class_specifier
  name: (type_identifier) @name) @definition.class
