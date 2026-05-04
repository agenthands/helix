;; Python tree-sitter queries — NET-NEW for Phase 59.
;; Compiled against the daemon-injected GrammarRegistry's "python" grammar.
;; Per Phase 59 D-01 these are NOT copied from
;; internal/repomap/queries/python_tags.scm.
;;
;; Decorator add/remove preserves stable ID per CONTEXT.md D-03 +
;; RESEARCH.md line 491: decorator.name is captured separately and is
;; NOT part of the canonicalization input. The decorated function/class's
;; name, signature, and OwnerPath remain unchanged when a decorator is
;; added or removed.

;; -- Function definitions ----------------------------------------------------
(function_definition
  name: (identifier) @definition.function)

;; -- Decorated definitions ---------------------------------------------------
;; Bare decorator: @dec
(decorator
  (identifier) @decorator.name)

;; Call decorator: @dec(args)
(decorator
  (call
    function: (identifier) @decorator.name))

;; Attribute decorator: @module.dec
(decorator
  (attribute
    attribute: (identifier) @decorator.name))

;; -- Class definitions -------------------------------------------------------
(class_definition
  name: (identifier) @definition.class)

;; Class inheritance: superclasses argument list contains identifier(s)
(class_definition
  superclasses: (argument_list
    (identifier) @heritage.extends))

(class_definition
  superclasses: (argument_list
    (attribute
      attribute: (identifier) @heritage.extends)))

;; -- Imports -----------------------------------------------------------------
;; import os
(import_statement
  name: (dotted_name) @import.source)

;; import os as o
(import_statement
  name: (aliased_import
    name: (dotted_name) @import.source
    alias: (identifier) @import.alias))

;; from os import path
(import_from_statement
  module_name: (dotted_name) @import.source
  name: (dotted_name) @import.symbol)

;; from os import path as p
(import_from_statement
  module_name: (dotted_name) @import.source
  name: (aliased_import
    name: (dotted_name) @import.symbol
    alias: (identifier) @import.alias))

;; -- Function calls ----------------------------------------------------------
(call
  function: (identifier) @reference.call)

(call
  function: (attribute
    attribute: (identifier) @reference.call))

;; -- Attribute access (field references) -------------------------------------
(attribute
  attribute: (identifier) @reference.field)

;; -- Type annotations on parameters and return ------------------------------
(typed_parameter
  type: (type) @type.annotation)

(typed_default_parameter
  type: (type) @type.annotation)

(function_definition
  return_type: (type) @type.return)

;; -- Parameter names ---------------------------------------------------------
(parameters
  (identifier) @definition.parameter)

(typed_parameter
  (identifier) @definition.parameter)

(typed_default_parameter
  name: (identifier) @definition.parameter)

;; -- Top-level (module-level) variable assignment ---------------------------
;; module_var = ...
(module
  (expression_statement
    (assignment
      left: (identifier) @definition.variable)))
