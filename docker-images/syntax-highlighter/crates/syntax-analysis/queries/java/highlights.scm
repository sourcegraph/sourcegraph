; Methods

(method_declaration
  name: (identifier) @identifier.function)
(method_invocation
  name: (identifier) @identifier.function)
(super) @identifier.builtin

; Annotations

(annotation
  name: (identifier) @identifier.attribute)
(marker_annotation
  name: (identifier) @identifier.attribute)

"@" @operator

; Types

(type_identifier) @identifier.type

(interface_declaration
  name: (identifier) @identifier.type)
(class_declaration
  name: (identifier) @identifier.type)
(enum_declaration
  name: (identifier) @identifier.type)

((field_access
  object: (identifier) @identifier.type)
 (#match? @identifier.type "^[A-Z]"))
((scoped_identifier
  scope: (identifier) @identifier.type)
 (#match? @identifier.type "^[A-Z]"))
((method_invocation
  object: (identifier) @identifier.type)
 (#match? @identifier.type "^[A-Z]"))
((method_reference
  . (identifier) @identifier.type)
 (#match? @identifier.type "^[A-Z]"))

(record_pattern
  (identifier) @identifier.type
)

(constructor_declaration
  name: (identifier) @identifier.type)

[
  (boolean_type)
  (integral_type)
  (floating_point_type)
  (floating_point_type)
  (void_type)]
@identifier.builtin

; Variables

((identifier) @constant
 (#match? @constant "^_*[A-Z][A-Z\\d_]+$"))

(identifier) @identifier

(this) @identifier.builtin

; Literals

[
  (hex_integer_literal)
  (decimal_integer_literal)
  (octal_integer_literal)
  (decimal_floating_point_literal)
  (hex_floating_point_literal)]
@number

[
  (character_literal)
  (string_literal)]
@string

[(true) (false)] @boolean

(null_literal) @constant.null

[
  (line_comment)
  (block_comment)]
@comment

; Keywords

[
  "abstract"
  "assert"
  "break"
  "case"
  "catch"
  "class"
  "record"
  "continue"
  "default"
  "do"
  "else"
  "enum"
  "exports"
  "extends"
  "final"
  "finally"
  "for"
  "if"
  "implements"
  "import"
  "instanceof"
  "interface"
  "module"
  "native"
  "new"
  "non-sealed"
  "open"
  "opens"
  "package"
  "private"
  "protected"
  "provides"
  "public"
  "requires"
  "return"
  "sealed"
  "static"
  "strictfp"
  "switch"
  "synchronized"
  "throw"
  "throws"
  "to"
  "transient"
  "transitive"
  "try"
  "uses"
  "volatile"
  "while"
  "with"]
@keyword
