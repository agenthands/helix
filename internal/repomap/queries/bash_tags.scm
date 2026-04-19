; Bash tag query - authored from scratch (no aider reference)

(function_definition
  name: (word) @name) @definition.function

(command
  name: (command_name (word) @name)) @reference.call
