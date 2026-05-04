;; TypeScript + JavaScript shared queries — NET-NEW for Phase 59.
;; Compiled against the daemon-injected GrammarRegistry's "typescript"
;; (and "tsx") grammars. Per Phase 59 D-01 these queries are NOT copied
;; from internal/repomap/queries/typescript_tags.scm.
;;
;; The provider claims .ts, .tsx, .js, .jsx, .mjs, .cjs. The TS-only
;; constructs (interface, decorator-applied-to-class, generic type
;; parameters, namespace) simply don't match in pure JS source.

;; -- Function declarations ---------------------------------------------------
(function_declaration
  name: (identifier) @definition.function)

;; -- Arrow function assigned to const/let/var --------------------------------
;; const f = (x: number) => x  — normalizer collapses to Kind=function so
;; the stable ID survives a const-arrow ↔ function-decl swap (TS recipe).
(variable_declarator
  name: (identifier) @definition.function
  value: (arrow_function))

;; -- Class declarations ------------------------------------------------------
(class_declaration
  name: (type_identifier) @definition.class)

;; Heritage clauses on a class (extends / implements)
(class_declaration
  (class_heritage
    (extends_clause
      value: (identifier) @heritage.extends)))

(class_declaration
  (class_heritage
    (implements_clause
      (type_identifier) @heritage.implements)))

;; -- Method definitions inside classes ---------------------------------------
(method_definition
  name: (property_identifier) @definition.method)

;; -- Public field definition -------------------------------------------------
(public_field_definition
  name: (property_identifier) @definition.field)

;; -- Interface declarations (TS-only) ----------------------------------------
(interface_declaration
  name: (type_identifier) @definition.interface)

(interface_declaration
  (extends_type_clause
    (type_identifier) @heritage.extends))

;; -- Type aliases (TS) -------------------------------------------------------
(type_alias_declaration
  name: (type_identifier) @definition.type)

;; -- Enums (TS) --------------------------------------------------------------
(enum_declaration
  name: (identifier) @definition.enum)

;; -- Decorators --------------------------------------------------------------
;; @MyDecorator
(decorator (identifier) @decorator.name)
;; @MyDecorator(...)
(decorator (call_expression function: (identifier) @decorator.name))

;; -- Import statements -------------------------------------------------------
;; import "./module"
(import_statement
  source: (string) @import.source)

;; import default from "..." → captures default name as alias
(import_statement
  (import_clause (identifier) @import.alias)
  source: (string) @import.source)

;; import { Named } from "..." — named imports under named_imports
(import_specifier
  name: (identifier) @import.symbol)

;; -- Reference: function/method calls ----------------------------------------
(call_expression
  function: (identifier) @reference.call)

(call_expression
  function: (member_expression
    property: (property_identifier) @reference.call))

;; -- Reference: type identifiers --------------------------------------------
(type_identifier) @reference.type

;; -- Property access (member reads) -----------------------------------------
(member_expression
  property: (property_identifier) @reference.field)

;; -- Type annotations on params/returns -------------------------------------
(required_parameter
  pattern: (identifier) @definition.parameter
  type: (type_annotation) @type.annotation)

(optional_parameter
  pattern: (identifier) @definition.parameter
  type: (type_annotation) @type.annotation)

;; Function return type
(function_declaration
  return_type: (type_annotation) @type.return)

(method_definition
  return_type: (type_annotation) @type.return)
