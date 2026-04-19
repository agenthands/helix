; Adapted from borrow/aider/aider/queries/tree-sitter-languages/java-tags.scm
; Converted @name.definition.X -> @name + @definition.X (Serena convention)

(class_declaration
  name: (identifier) @name) @definition.class

(method_declaration
  name: (identifier) @name) @definition.method

(interface_declaration
  name: (identifier) @name) @definition.interface

; References

(method_invocation
  name: (identifier) @name
  arguments: (argument_list)) @reference.call

(object_creation_expression
  type: (type_identifier) @name) @reference.class

(superclass (type_identifier) @name) @reference.class
