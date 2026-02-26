; Go tree-sitter query for symbol extraction
; Used when tree-sitter bindings are available

; Function declarations
(function_declaration
  name: (identifier) @function.name) @function

; Method declarations
(method_declaration
  receiver: (parameter_list
    (parameter_declaration
      type: (_) @method.receiver))
  name: (field_identifier) @method.name) @method

; Type declarations (struct)
(type_declaration
  (type_spec
    name: (type_identifier) @type.name
    type: (struct_type))) @type

; Type declarations (interface)
(type_declaration
  (type_spec
    name: (type_identifier) @interface.name
    type: (interface_type))) @interface

; Import declarations
(import_spec
  path: (interpreted_string_literal) @import.path) @import

; Constant declarations
(const_declaration
  (const_spec
    name: (identifier) @constant.name)) @constant
