;; Rust tree-sitter queries — NET-NEW for polyglot expansion.
;; Compiled at provider construction against the daemon-injected
;; GrammarRegistry's "rust" grammar.
;;
;; Capture vocabulary:
;;   definition.{function,struct,enum,trait,impl,type,variable,constant,field,parameter,module}
;;   reference.{call,identifier,field,type}
;;   import.{source}
;;   type.{annotation,return,parameter}
;;   heritage.{extends,implements}

;; -- Functions ---------------------------------------------------------------
(function_item
  name: (identifier) @definition.function) @def.function.body

;; -- Methods (in impl blocks) ------------------------------------------------
;; Method declarations inside impl_item
(impl_item
  type: (_)
  body: (declaration_list
    (function_item
      name: (identifier) @definition.method))) @def.method.body

;; -- Structs -----------------------------------------------------------------
(struct_item
  name: (type_identifier) @definition.struct) @def.struct.body

;; -- Enums -------------------------------------------------------------------
(enum_item
  name: (type_identifier) @definition.enum) @def.enum.body

;; -- Traits ------------------------------------------------------------------
(trait_item
  name: (type_identifier) @definition.interface) @def.trait.body

;; -- Type aliases ------------------------------------------------------------
(type_item
  name: (type_identifier) @definition.type)

;; -- Constants ---------------------------------------------------------------
(const_item
  name: (identifier) @definition.constant)

;; -- Static items (statics) --------------------------------------------------
(static_item
  name: (identifier) @definition.variable)

;; -- Struct fields -----------------------------------------------------------
(field_declaration
  name: (field_identifier) @definition.field)

;; -- Enum variants -----------------------------------------------------------
(enum_variant
  name: (identifier) @definition.field)

;; -- Let bindings (local variables) ------------------------------------------
(let_declaration
  pattern: (identifier) @definition.variable)

;; -- Parameters --------------------------------------------------------------
(parameters
  (parameter
    pattern: (identifier) @definition.parameter
    type: (_) @type.parameter))

(self_parameter) @definition.parameter

;; -- Use declarations (imports) ----------------------------------------------
(use_declaration
  argument: (_) @import.source)

;; -- Function calls ----------------------------------------------------------
(call_expression
  function: (identifier) @reference.call)

;; Method calls: foo.bar()
(call_expression
  function: (field_expression
    field: (field_identifier) @reference.call))

;; -- Type references ---------------------------------------------------------
(type_identifier) @reference.type

;; -- Field access ------------------------------------------------------------
(field_expression
  field: (field_identifier) @reference.field)

;; -- Return type annotations -------------------------------------------------
(function_item
  return_type: (_) @type.return)

;; -- Variable type annotations -----------------------------------------------
(let_declaration
  type: (_) @type.annotation)

;; -- Trait bounds (heritage) -------------------------------------------------
;; impl Foo for Bar — Bar implements Foo
(impl_item
  trait: (_) @heritage.implements)

;; trait Foo: Bar — Foo extends Bar  
(trait_bounds
  (_) @heritage.extends)
