;; C# tree-sitter queries — minimal safe set.
;; Capture vocabulary:
;;   definition.{method,class,interface,struct,enum,field,variable,parameter}
;;   reference.{call,field,type}
;;   import.{source}
;;   heritage.{extends}

;; -- Methods + constructors --------------------------------------------------
(method_declaration
  name: (identifier) @definition.method) @def.method.body

(constructor_declaration
  name: (identifier) @definition.method) @def.constructor.body

;; -- Types -------------------------------------------------------------------
(class_declaration
  name: (identifier) @definition.class) @def.class.body

(interface_declaration
  name: (identifier) @definition.interface) @def.interface.body

(struct_declaration
  name: (identifier) @definition.struct) @def.struct.body

(enum_declaration
  name: (identifier) @definition.enum) @def.enum.body

;; -- Fields + properties + locals --------------------------------------------
(property_declaration
  name: (identifier) @definition.field)

(field_declaration
  (variable_declaration
    (variable_declarator
      name: (identifier) @definition.field)))

(local_declaration_statement
  (variable_declaration
    (variable_declarator
      name: (identifier) @definition.variable)))

;; -- Parameters --------------------------------------------------------------
(parameter
  name: (identifier) @definition.parameter)

;; -- Imports -----------------------------------------------------------------
(using_directive
  name: (_) @import.source)

;; -- Method calls ------------------------------------------------------------
(invocation_expression
  function: (identifier) @reference.call)

(invocation_expression
  function: (member_access_expression
    name: (identifier) @reference.call))

;; -- Type references ---------------------------------------------------------
(identifier) @reference.type

;; -- Member access -----------------------------------------------------------
(member_access_expression
  name: (identifier) @reference.field)

;; -- Heritage ----------------------------------------------------------------
(base_list
  (_) @heritage.extends)
