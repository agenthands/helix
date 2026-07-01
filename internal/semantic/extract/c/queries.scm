;; C tree-sitter queries — NET-NEW for polyglot expansion.
;; Compiled at provider construction against the daemon-injected
;; GrammarRegistry's "c" grammar.
;;
;; Capture vocabulary:
;;   definition.{function,struct,enum,type,field,variable,parameter}
;;   reference.{call,identifier,field,type}
;;   import.{source}
;;   type.{return,annotation}

;; -- Functions ---------------------------------------------------------------
(function_definition
  declarator: (function_declarator
    declarator: (identifier) @definition.function)) @def.function.body

(function_definition
  declarator: (function_declarator
    declarator: (field_identifier) @definition.function)) @def.function.body

;; -- Structs -----------------------------------------------------------------
(struct_specifier
  name: (type_identifier) @definition.struct) @def.struct.body

;; -- Enums -------------------------------------------------------------------
(enum_specifier
  name: (type_identifier) @definition.enum) @def.enum.body

;; -- Typedefs ----------------------------------------------------------------
(type_definition
  declarator: (type_identifier) @definition.type)

;; -- Struct fields -----------------------------------------------------------
;; Co-capture the field's declared type node (@declared.type) alongside the
;; field identifier so the provider can attach SymbolFact.DeclaredType in ONE
;; match (in-memory only; @declared.type is NOT emitted as a TypeFact — the
;; standalone type.annotation captures below own that). v2.12 Phase 135 B3.
(field_declaration
  type: (_) @declared.type
  declarator: (field_identifier) @definition.field)

;; -- Global variables --------------------------------------------------------
(declaration
  type: (_) @declared.type
  declarator: (init_declarator
    declarator: (identifier) @definition.variable))

;; -- Parameters --------------------------------------------------------------
(parameter_declaration
  type: (_) @declared.type
  declarator: (identifier) @definition.parameter)

(parameter_declaration
  type: (_) @declared.type
  declarator: (pointer_declarator
    declarator: (identifier) @definition.parameter))

;; -- Includes (imports) ------------------------------------------------------
(preproc_include
  path: (_) @import.source)

;; -- Function calls ----------------------------------------------------------
(call_expression
  function: (identifier) @reference.call)

(call_expression
  function: (field_expression
    field: (field_identifier) @reference.call))

;; -- Type references ---------------------------------------------------------
(type_identifier) @reference.type

;; -- Field access ------------------------------------------------------------
(field_expression
  field: (field_identifier) @reference.field)

;; -- Return type annotations -------------------------------------------------
(function_definition
  type: (_) @type.return)

;; -- Variable type annotations -----------------------------------------------
(declaration
  type: (_) @type.annotation)
