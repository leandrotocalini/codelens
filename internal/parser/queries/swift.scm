; Swift tree-sitter query for symbol extraction

; Import declarations
(import_declaration
  (identifier) @import.name) @import

; Class declarations
(class_declaration
  name: (type_identifier) @class.name) @class

; Struct declarations
(struct_declaration
  name: (type_identifier) @struct.name) @struct

; Enum declarations
(enum_declaration
  name: (type_identifier) @enum.name) @enum

; Protocol declarations
(protocol_declaration
  name: (type_identifier) @protocol.name) @protocol

; Function declarations
(function_declaration
  name: (simple_identifier) @function.name) @function

; Init declarations
(init_declaration) @init

; Property declarations
(property_declaration
  (pattern
    (simple_identifier) @property.name)) @property

; Typealias declarations
(typealias_declaration
  name: (type_identifier) @typealias.name) @typealias

; Extension declarations
(extension_declaration
  name: (type_identifier) @extension.name) @extension
