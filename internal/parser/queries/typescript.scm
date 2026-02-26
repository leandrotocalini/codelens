; TypeScript tree-sitter query for symbol extraction

; Function declarations
(function_declaration
  name: (identifier) @function.name) @function

; Export function declarations
(export_statement
  declaration: (function_declaration
    name: (identifier) @export_function.name)) @export_function

; Class declarations
(class_declaration
  name: (type_identifier) @class.name) @class

; Interface declarations
(interface_declaration
  name: (type_identifier) @interface.name) @interface

; Type alias declarations
(type_alias_declaration
  name: (type_identifier) @type.name) @type

; Variable declarations (const/let/var)
(lexical_declaration
  (variable_declarator
    name: (identifier) @variable.name)) @variable

; Arrow functions assigned to variables
(lexical_declaration
  (variable_declarator
    name: (identifier) @arrow_function.name
    value: (arrow_function))) @arrow_function

; Import declarations
(import_statement
  source: (string) @import.path) @import

; Export default
(export_statement
  value: (identifier) @export_default.name) @export_default
