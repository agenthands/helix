; Adapted from borrow/aider/aider/queries/tree-sitter-languages/c_sharp-tags.scm
; Converted @name.definition.X -> @name + @definition.X (Serena convention)

(class_declaration
  name: (identifier) @name) @definition.class

(interface_declaration
  name: (identifier) @name) @definition.interface

(method_declaration
  name: (identifier) @name) @definition.method

(namespace_declaration
  name: (identifier) @name) @definition.module

; References

(object_creation_expression
  type: (identifier) @name) @reference.class

(invocation_expression
  function: (member_access_expression
    name: (identifier) @name)) @reference.call
