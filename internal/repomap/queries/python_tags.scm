; Definitions
(function_definition
  name: (identifier) @name) @definition.function

(class_definition
  name: (identifier) @name) @definition.class

; References
(call
  function: [
    (identifier) @name
    (attribute attribute: (identifier) @name)
  ]) @reference.call
