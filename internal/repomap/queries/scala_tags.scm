; Adapted from aider scala-tags.scm for Serena capture convention

(class_definition
  name: (identifier) @name) @definition.class

(trait_definition
  name: (identifier) @name) @definition.interface

(object_definition
  name: (identifier) @name) @definition.object

(function_definition
  name: (identifier) @name) @definition.function

(type_definition
  name: (type_identifier) @name) @definition.type

(call_expression
  (identifier) @name) @reference.call

(extends_clause
  (type_identifier) @name) @reference.class
