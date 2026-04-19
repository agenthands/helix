; Adapted from aider ocaml-tags.scm for Serena capture convention
; Stripped #strip! predicates and @doc captures

(module_definition
  (module_binding
    (module_name) @name)) @definition.module

(value_definition
  (let_binding
    pattern: (value_name) @name)) @definition.function

(class_definition
  (class_binding
    (class_name) @name)) @definition.class

(method_definition
  (method_name) @name) @definition.method

(application_expression
  function: (value_path
    (value_name) @name)) @reference.call
