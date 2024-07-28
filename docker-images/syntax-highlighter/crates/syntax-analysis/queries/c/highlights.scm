"--" @identifier.operator
"-" @identifier.operator
"-=" @identifier.operator
"->" @identifier.operator
"=" @identifier.operator
"!=" @identifier.operator
"*" @identifier.operator
"&" @identifier.operator
"&&" @identifier.operator
"+" @identifier.operator
"++" @identifier.operator
"+=" @identifier.operator
"<" @identifier.operator
"==" @identifier.operator
">" @identifier.operator
"||" @identifier.operator
"!" @identifier.operator

; "." @delimiter
; ";" @delimiter

(string_literal) @string
(system_lib_string) @string

(null) @constant.null
(number_literal) @number
(char_literal) @character
[(true) (false)] @boolean

(call_expression
  function: (identifier) @identifier.function)
(call_expression
  function: (field_expression
             field: (field_identifier) @identifier.function))
(function_declarator
  declarator: (identifier) @identifier.function)
(preproc_function_def
  name: (identifier) @identifier.function)

(field_identifier) @identifier ;; TODO: something better
(statement_identifier) @identifier
(type_identifier) @type
(primitive_type) @type.builtin
(sized_type_specifier) @type

((identifier) @constant
 (#match? @constant "^[A-Z][A-Z\\d_]*$"))

(identifier) @identifier

(comment) @comment

"break" @keyword
"case" @keyword
"const" @keyword
"continue" @keyword
"default" @keyword
"do" @keyword
"else" @keyword
"enum" @keyword
"extern" @keyword
"for" @keyword
"if" @keyword
"inline" @keyword
"return" @keyword
"sizeof" @keyword
"static" @keyword
"struct" @keyword
"switch" @keyword
"typedef" @keyword
"union" @keyword
"volatile" @keyword
"while" @keyword

"#define" @keyword
"#elif" @keyword
"#else" @keyword
"#endif" @keyword
"#if" @keyword
"#ifdef" @keyword
"#ifndef" @keyword
"#include" @keyword
(preproc_directive) @keyword
