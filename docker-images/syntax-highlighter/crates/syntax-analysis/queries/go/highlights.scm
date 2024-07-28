;; Builtin types

((type_identifier) @type.builtin
  (#match? @type.builtin
            "^(bool|byte|complex128|complex64|error|float32|float64|int|int16|int32|int64|int8|rune|string|uint|uint16|uint32|uint64|uint8|uintptr)$"))


;; Builtin functions

((identifier) @function.builtin
  (#match? @function.builtin "^(append|cap|close|complex|copy|delete|imag|len|make|new|panic|print|println|real|recover)$"))

; Function calls

(parameter_declaration (identifier) @variable.parameter)
(variadic_parameter_declaration (identifier) @variable.parameter)

(call_expression
  function: (identifier) @identifer.function)

(call_expression
  function: (selector_expression
             field: (field_identifier) @identifier.function))

; Function definitions

(method_spec
 name: (field_identifier) @identifier.function)
(function_declaration
 name: (identifier) @identifier.function)

(method_declaration
 name: (field_identifier) @identifier.function)

; Constants

(const_spec
 name: (identifier) @constant)

; Operators

[
 "--"
 "-"
 "-="
 ":="
 "!"
 "!="
 "..."
 "*"
 "*"
 "*="
 "/"
 "/="
 "&"
 "&&"
 "&="
 "%"
 "%="
 "^"
 "^="
 "+"
 "++"
 "+="
 "<-"
 "<"
 "<<"
 "<<="
 "<="
 "="
 "=="
 ">"
 ">="
 ">>"
 ">>="
 "|"
 "|="
 "||"] @operator

; Keywords

[
 "break"
 "chan"
 "const"
 "continue"
 "default"
 "defer"
 "go"
 "goto"
 "interface"
 "map"
 "range"
 "select"
 "struct"
 "type"
 "var"
 "fallthrough"] @keyword

"func" @keyword.function
"return" @keyword.return

"for" @keyword.repeat

[
 "import"
 "package"] @include

[
 "else"
 "case"
 "switch"
 "if"] @conditional



; Literals

(interpreted_string_literal) @string
(raw_string_literal) @string
(rune_literal) @string
(escape_sequence) @string.escape

(int_literal) @number
(float_literal) @float
(imaginary_literal) @number

[(true) (false)] @boolean
(nil) @constant.null

(comment) @comment

;;
; Identifiers

(package_identifier) @variable.module
(type_identifier) @type
(keyed_element . (literal_element (identifier) @identifier.attribute))
((identifier) @constant (#match? @constant "^[A-Z][A-Z\\d_]+$"))
((identifier) @constant (#eq? @constant "_"))
(identifier) @variable
(field_identifier) @identifier.property


