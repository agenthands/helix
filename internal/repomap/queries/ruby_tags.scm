; Adapted from borrow/aider/aider/queries/tree-sitter-languages/ruby-tags.scm
; Stripped #strip!, #select-adjacent!, #is-not?, #not-match? predicates, @doc, @ignore

(method
  name: (_) @name) @definition.method

(singleton_method
  name: (_) @name) @definition.method

(class
  name: [(constant) @name
         (scope_resolution name: (_) @name)]) @definition.class

(module
  name: [(constant) @name
         (scope_resolution name: (_) @name)]) @definition.module

; References

(call method: (identifier) @name) @reference.call
