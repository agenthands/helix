;; PHP tree-sitter queries — verified against actual grammar.
;; Node types confirmed via tree-sitter parse dump (2026-06-28).

;; -- Functions ---------------------------------------------------------------
(function_definition
  name: (name) @definition.function) @def.function.body

;; -- Methods -----------------------------------------------------------------
(method_declaration
  name: (name) @definition.method) @def.method.body

;; -- Classes -----------------------------------------------------------------
(class_declaration
  name: (name) @definition.class) @def.class.body

;; -- Interfaces --------------------------------------------------------------
(interface_declaration
  name: (name) @definition.interface) @def.interface.body

;; -- Traits ------------------------------------------------------------------
(trait_declaration
  name: (name) @definition.interface) @def.trait.body

;; -- Enums -------------------------------------------------------------------
(enum_declaration
  name: (name) @definition.enum) @def.enum.body

;; -- Properties (variable_name nests $ and name) -----------------------------
(property_declaration
  (property_element
    (variable_name
      (name) @definition.field)))

;; -- Constants ---------------------------------------------------------------
(const_declaration
  (const_element
    (name) @definition.constant))

;; -- Parameters (simple_parameter nests variable_name → $ → name) -----------
(simple_parameter
  (variable_name
    (name) @definition.parameter))

;; -- Use declarations (imports) ----------------------------------------------
(use_declaration
  (name) @import.source)

;; -- Function calls ----------------------------------------------------------
(function_call_expression
  function: (name) @reference.call)

;; Method calls: $obj->method()
(member_call_expression
  name: (name) @reference.call)

;; Static method calls: Foo::method()
(scoped_call_expression
  name: (name) @reference.call)

;; -- Type references (both named_type and primitive_type) --------------------
(named_type
  (name) @reference.type)

(primitive_type) @reference.type

;; -- Member access -----------------------------------------------------------
(member_access_expression
  name: (name) @reference.field)

;; -- Heritage: extends + implements -----------------------------------------
(class_declaration
  (base_clause
    (name) @heritage.extends))

(class_declaration
  (class_interface_clause
    (name) @heritage.implements))
