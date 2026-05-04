;; Go tree-sitter queries — NET-NEW for Phase 59.
;; Covers SPEC §13.4 capture vocabulary + Phase 59 receiver/heritage
;; additions per CONTEXT.md D-01a. Compiled at provider construction
;; against the daemon-injected GrammarRegistry's "go" grammar.
;;
;; Capture vocabulary used here:
;;   definition.{function,method,struct,class,interface,type,variable,
;;               constant,field,parameter}
;;   reference.{call,identifier,field,type}
;;   import.{source,alias}
;;   type.{annotation,return,parameter}
;;   heritage.embeds   (Go-specific: embedded fields/interfaces)
;;   receiver.{type,name}
;;
;; Reference shape only — the "go_tags.scm" file in internal/repomap/
;; informs query style (S-expression form, capture naming) but is NOT
;; copied; D-01 hard invariant.

;; -- Functions (top-level) ---------------------------------------------------
(function_declaration
  name: (identifier) @definition.function) @def.function.body

;; -- Methods (with receiver) -------------------------------------------------
;; Pointer receiver: (r *T) M(...)
(method_declaration
  receiver: (parameter_list
    (parameter_declaration
      name: (identifier) @receiver.name
      type: (pointer_type (type_identifier) @receiver.type)))
  name: (field_identifier) @definition.method) @def.method.body

;; Value receiver: (r T) M(...)
(method_declaration
  receiver: (parameter_list
    (parameter_declaration
      name: (identifier) @receiver.name
      type: (type_identifier) @receiver.type))
  name: (field_identifier) @definition.method) @def.method.body

;; -- Struct types ------------------------------------------------------------
(type_spec
  name: (type_identifier) @definition.struct
  type: (struct_type)) @def.struct.body

;; -- Interface types ---------------------------------------------------------
(type_spec
  name: (type_identifier) @definition.interface
  type: (interface_type)) @def.interface.body

;; -- Type aliases / generic type declarations --------------------------------
;; `type ID = int64` — alias_declaration in tree-sitter-go
(type_alias
  name: (type_identifier) @definition.type)

;; Generic type spec (Go 1.18+) — type T[X any] = ... is also covered by
;; type_spec; non-struct non-interface type_spec is captured here. (struct
;; and interface have their own queries above; tree-sitter will match all
;; that fit, so consumers de-duplicate by SelectionRange.)
(type_spec
  name: (type_identifier) @definition.type
  type: [
    (type_identifier)
    (qualified_type)
    (pointer_type)
    (slice_type)
    (array_type)
    (map_type)
    (channel_type)
    (function_type)
    (generic_type)
  ]) @def.type.body

;; -- Constants ---------------------------------------------------------------
(const_spec
  name: (identifier) @definition.constant)

;; -- Top-level variables -----------------------------------------------------
(var_spec
  name: (identifier) @definition.variable)

;; -- Struct fields -----------------------------------------------------------
(field_declaration
  name: (field_identifier) @definition.field)

;; -- Embedded struct field (Go-specific heritage) ----------------------------
;; Embedded fields have NO `name:` field — only `type:`. Use negative field
;; constraint to filter regular field declarations like `n int`.
(field_declaration
  !name
  type: (type_identifier) @heritage.embeds)
;; Embedded with pointer: type Foo struct { *Base }
(field_declaration
  !name
  type: (pointer_type (type_identifier) @heritage.embeds))

;; -- Imports -----------------------------------------------------------------
;; Bare import: import "fmt"
(import_spec
  path: (interpreted_string_literal) @import.source)

;; Aliased import: import f "fmt"
(import_spec
  name: (package_identifier) @import.alias
  path: (interpreted_string_literal) @import.source)

;; -- Calls (function invocation) ---------------------------------------------
;; Bare call:  Foo(...)
(call_expression
  function: (identifier) @reference.call)

;; Method/selector call:  pkg.Foo(...) or s.Method(...)
(call_expression
  function: (selector_expression
    field: (field_identifier) @reference.call))

;; -- Type references ---------------------------------------------------------
(type_identifier) @reference.type

;; -- Field access (selector reads) -------------------------------------------
(selector_expression
  field: (field_identifier) @reference.field)

;; -- Function parameters -----------------------------------------------------
(parameter_declaration
  name: (identifier) @definition.parameter
  type: (_) @type.parameter)

;; Variadic parameter:  args ...string
(variadic_parameter_declaration
  name: (identifier) @definition.parameter
  type: (_) @type.parameter)

;; -- Function return types ---------------------------------------------------
(function_declaration
  result: (_) @type.return)

(method_declaration
  result: (_) @type.return)
