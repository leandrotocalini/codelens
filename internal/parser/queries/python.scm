; Python tree-sitter query for symbol extraction

; Function definitions
(function_definition
  name: (identifier) @function.name
  parameters: (parameters) @function.params) @function

; Class definitions
(class_definition
  name: (identifier) @class.name) @class

; Import statements
(import_statement
  name: (dotted_name) @import.name) @import

; Import from statements
(import_from_statement
  module_name: (dotted_name) @import_from.module) @import_from

; Decorated definitions
(decorated_definition
  (decorator
    (identifier) @decorator.name)
  definition: (function_definition
    name: (identifier) @decorated_function.name)) @decorated_function

; Assignment (top-level variables)
(expression_statement
  (assignment
    left: (identifier) @variable.name)) @variable
