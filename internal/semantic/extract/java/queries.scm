;; Java tree-sitter queries — minimal safe set.
;; Capture vocabulary:
;;   definition.{method,class,interface,enum,field,variable,parameter,constructor}
;;   reference.{call,field,type}
;;   import.{source}
;;   heritage.{extends,implements}

;; -- Methods + constructors --------------------------------------------------
(method_declaration
  name: (identifier) @definition.method) @def.method.body

(constructor_declaration
  name: (identifier) @definition.constructor) @def.constructor.body

;; -- Classes / interfaces / enums --------------------------------------------
(class_declaration
  name: (identifier) @definition.class) @def.class.body

(interface_declaration
  name: (identifier) @definition.interface) @def.interface.body

(enum_declaration
  name: (identifier) @definition.enum) @def.enum.body

;; -- Fields + locals ---------------------------------------------------------
(field_declaration
  declarator: (variable_declarator
    name: (identifier) @definition.field))

(local_variable_declaration
  declarator: (variable_declarator
    name: (identifier) @definition.variable))

;; -- Parameters --------------------------------------------------------------
(formal_parameter
  name: (identifier) @definition.parameter)

;; -- Imports -----------------------------------------------------------------
(import_declaration
  (identifier) @import.source)

(import_declaration
  (scoped_identifier) @import.source)

;; -- Method calls ------------------------------------------------------------
(method_invocation
  name: (identifier) @reference.call)

(method_invocation
  object: (_)
  name: (identifier) @reference.call)

;; -- Type references ---------------------------------------------------------
(type_identifier) @reference.type

;; -- Field access ------------------------------------------------------------
(field_access
  field: (identifier) @reference.field)

;; -- Heritage ----------------------------------------------------------------
(superclass
  (_) @heritage.extends)

(super_interfaces
  (_) @heritage.implements)
