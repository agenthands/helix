;; Kotlin tree-sitter queries — verified against actual grammar.
;; Node types confirmed via tree-sitter parse dump (2026-06-28).

;; -- Functions ---------------------------------------------------------------
(function_declaration
  name: (identifier) @definition.function) @def.function.body

;; -- Classes + interfaces ----------------------------------------------------
;; Kotlin uses `class_declaration` for both classes and interfaces
;; (interfaces are `class_declaration` with `interface` keyword)
(class_declaration
  name: (identifier) @definition.class) @def.class.body

;; -- Properties --------------------------------------------------------------
(property_declaration
  (variable_declaration
    (identifier) @definition.field))

;; -- Class parameters (constructor properties) ------------------------------
(class_parameter
  (identifier) @definition.field)

;; -- Parameters --------------------------------------------------------------
;; function_value_parameters contain the actual param list
(function_value_parameters
  (parameter
    (identifier) @definition.parameter))

;; -- Type references (user_type nests identifier) ---------------------------
(user_type
  (identifier) @reference.type)

;; -- Function calls ----------------------------------------------------------
;; Bare call: foo()
(call_expression
  (identifier) @reference.call)

;; Method calls: obj.foo() (callee is a navigation_expression whose last
;; identifier is the method name; the leading named child is the receiver).
(call_expression
  (navigation_expression
    (_)
    (identifier) @reference.call))

;; -- Navigation (field access: obj.prop) -------------------------------------
(navigation_expression
  (identifier) @reference.field)

