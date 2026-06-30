;; Ruby tree-sitter queries — NET-NEW for polyglot expansion.
;; Node types verified against repomap/queries/ruby_tags.scm and tree-sitter-ruby grammar.

;; -- Methods -----------------------------------------------------------------
(method
  name: (_) @definition.method) @def.method.body

;; -- Singleton methods (class methods) ---------------------------------------
(singleton_method
  name: (_) @definition.method) @def.method.body

;; -- Classes -----------------------------------------------------------------
(class
  name: (constant) @definition.class) @def.class.body

;; -- Modules -----------------------------------------------------------------
(module
  name: (constant) @definition.module) @def.module.body

;; -- Method calls ------------------------------------------------------------
(call
  method: (identifier) @reference.call)

;; -- Type references (constants) --------------------------------------------
(constant) @reference.type

;; -- Variable assignments ---------------------------------------------------
(assignment
  left: (identifier) @definition.variable)

;; -- Parameters (method params) ---------------------------------------------
(method_parameters
  (identifier) @definition.parameter)

;; -- Block parameters --------------------------------------------------------
(block_parameters
  (identifier) @definition.parameter)
