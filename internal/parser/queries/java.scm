; Java tree-sitter query for symbol extraction

; Class declarations
(class_declaration
  name: (identifier) @class.name) @class

; Interface declarations
(interface_declaration
  name: (identifier) @interface.name) @interface

; Enum declarations
(enum_declaration
  name: (identifier) @enum.name) @enum

; Method declarations
(method_declaration
  name: (identifier) @method.name
  parameters: (formal_parameters) @method.params) @method

; Constructor declarations
(constructor_declaration
  name: (identifier) @constructor.name) @constructor

; Field declarations
(field_declaration
  declarator: (variable_declarator
    name: (identifier) @field.name)) @field

; Import declarations
(import_declaration
  (scoped_identifier) @import.path) @import

; Package declaration
(package_declaration
  (scoped_identifier) @package.name) @package

; Annotation
(marker_annotation
  name: (identifier) @annotation.name) @annotation
